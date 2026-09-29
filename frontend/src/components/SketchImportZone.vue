<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { assetsApi, type Asset } from '../api/client'
import {
  extractImageFiles,
  extractImageFilesFromItems,
  extractImageUrls,
  extractImageUrlsFromDataTransfer,
  formatSketchFileSize,
  isEditableEventTarget,
  isImageLikeDataTransfer,
  MAX_SKETCH_FILE_SIZE,
} from './imageImport'

type QueueStatus = 'queued' | 'uploading' | 'uploaded' | 'error'

type QueueItem = {
  id: string
  name: string
  size: number
  mime: string
  file?: File
  sourceUrl?: string
  previewUrl: string
  source: 'file' | 'clipboard' | 'url'
  status: QueueStatus
  progress: number
  error?: string
  asset?: Asset
}

const props = withDefaults(defineProps<{
  projectId: string
  disabled?: boolean
  maxFiles?: number
}>(), {
  disabled: false,
  maxFiles: 20,
})

const emit = defineEmits<{
  imported: [payload: { asset: Asset; name: string; source: QueueItem['source'] }]
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const queue = ref<QueueItem[]>([])
const dragActive = ref(false)
const importing = ref(false)
let dragDepth = 0

const queuedItems = computed(() => queue.value.filter(item => item.status === 'queued' || item.status === 'error'))
const uploadedItems = computed(() => queue.value.filter(item => item.status === 'uploaded'))
const completedCount = computed(() => uploadedItems.value.length)
const overallProgress = computed(() => {
  if (!queue.value.length) return 0
  return Math.round(queue.value.reduce((sum, item) => sum + item.progress, 0) / queue.value.length)
})

function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim()) return error.message
  return 'Could not import sketch'
}

function addQueueError(name: string, message: string, source: QueueItem['source']) {
  queue.value.push({
    id: crypto.randomUUID(),
    name,
    size: 0,
    mime: '',
    previewUrl: '',
    source,
    status: 'error',
    progress: 0,
    error: message,
  })
}

function addFiles(files: File[], source: QueueItem['source']) {
  if (props.disabled) return
  const remaining = Math.max(0, props.maxFiles - queue.value.length)
  for (const file of files.slice(0, remaining)) {
    if (file.size > MAX_SKETCH_FILE_SIZE) {
      addQueueError(file.name || 'sketch', 'File is too large. Maximum size is 10 MB.', source)
      continue
    }
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      addQueueError(file.name || 'sketch', 'Unsupported image format. Use PNG, JPEG or WebP.', source)
      continue
    }
    const previewUrl = URL.createObjectURL(file)
    queue.value.push({
      id: crypto.randomUUID(),
      name: file.name || 'Pasted sketch',
      size: file.size,
      mime: file.type,
      file,
      previewUrl,
      source,
      status: 'queued',
      progress: 0,
    })
  }
}

function addUrls(raw: string, source: 'url' | 'clipboard') {
  if (props.disabled) return
  const remaining = Math.max(0, props.maxFiles - queue.value.length)
  for (const url of extractImageUrls(raw).slice(0, remaining)) {
    try {
      const parsed = new URL(url)
      const name = decodeURIComponent(parsed.pathname.split('/').pop() || 'web-image')
      queue.value.push({
        id: crypto.randomUUID(),
        name,
        size: 0,
        mime: 'image/*',
        sourceUrl: url,
        previewUrl: url,
        source,
        status: 'queued',
        progress: 0,
      })
    } catch {
      addQueueError('image URL', 'Invalid image URL.', source)
    }
  }
}

function addDataTransfer(dataTransfer: DataTransfer | null, source: QueueItem['source']) {
  if (!dataTransfer) return
  const files = extractImageFiles(dataTransfer.files)
  if (files.length) addFiles(files, source)
  else addUrls(
    extractImageUrlsFromDataTransfer(dataTransfer).join('\n'),
    source === 'clipboard' ? 'clipboard' : 'url',
  )
}

function chooseFiles() {
  if (!props.disabled) fileInput.value?.click()
}

function handleFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  addFiles(Array.from(input.files ?? []), 'file')
  input.value = ''
}

function handleDragOver(event: DragEvent) {
  if (!isImageLikeDataTransfer(event.dataTransfer)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function handleDragEnter(event: DragEvent) {
  if (!isImageLikeDataTransfer(event.dataTransfer)) return
  event.preventDefault()
  dragDepth += 1
  dragActive.value = true
}

function handleDragLeave(event: DragEvent) {
  if (!isImageLikeDataTransfer(event.dataTransfer)) return
  event.preventDefault()
  dragDepth = Math.max(0, dragDepth - 1)
  if (dragDepth === 0) dragActive.value = false
}

function handleDrop(event: DragEvent) {
  if (!isImageLikeDataTransfer(event.dataTransfer)) return
  event.preventDefault()
  dragDepth = 0
  dragActive.value = false
  addDataTransfer(event.dataTransfer, 'url')
}

function handleWindowDragOver(event: DragEvent) {
  if (!isImageLikeDataTransfer(event.dataTransfer)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function handleWindowDragEnter(event: DragEvent) {
  if (!isImageLikeDataTransfer(event.dataTransfer)) return
  event.preventDefault()
  dragDepth += 1
  dragActive.value = true
}

function handleWindowDragLeave(event: DragEvent) {
  if (!isImageLikeDataTransfer(event.dataTransfer)) return
  event.preventDefault()
  dragDepth = Math.max(0, dragDepth - 1)
  if (dragDepth === 0) dragActive.value = false
}

function handleWindowDrop(event: DragEvent) {
  if (!isImageLikeDataTransfer(event.dataTransfer)) return
  event.preventDefault()
  dragDepth = 0
  dragActive.value = false
  addDataTransfer(event.dataTransfer, 'url')
}

function handlePaste(event: ClipboardEvent) {
  if (props.disabled || isEditableEventTarget(event.target)) return
  const files = event.clipboardData ? extractImageFilesFromItems(event.clipboardData.items) : []
  const text = event.clipboardData
    ? [
        event.clipboardData.getData('text/uri-list'),
        event.clipboardData.getData('text/html'),
        event.clipboardData.getData('text/plain'),
      ].filter(Boolean).join('\n')
    : ''
  if (!files.length && !extractImageUrls(text).length) return
  event.preventDefault()
  if (files.length) addFiles(files, 'clipboard')
  else addUrls(text, 'clipboard')
}

function revoke(item: QueueItem) {
  if (item.file) URL.revokeObjectURL(item.previewUrl)
}

function removeItem(item: QueueItem) {
  revoke(item)
  queue.value = queue.value.filter(candidate => candidate.id !== item.id)
}

function clearUploaded() {
  for (const item of uploadedItems.value) revoke(item)
  queue.value = queue.value.filter(item => item.status !== 'uploaded')
}

function updateItem(item: QueueItem, patch: Partial<QueueItem>) {
  Object.assign(item, patch)
}

async function uploadItem(item: QueueItem) {
  if (item.status === 'uploaded') return
  updateItem(item, { status: 'uploading', progress: 0, error: undefined })
  try {
    const asset = item.file
      ? await assetsApi.uploadMultipartWithProgress(props.projectId, item.file, percentage => {
          updateItem(item, { progress: percentage })
        })
      : await (async () => {
          updateItem(item, { progress: 35 })
          const result = await assetsApi.importFromUrl(props.projectId, item.sourceUrl ?? '')
          updateItem(item, { progress: 100 })
          return result
        })()
    updateItem(item, { status: 'uploaded', progress: 100, asset })
    emit('imported', { asset, name: item.name, source: item.source })
  } catch (error) {
    updateItem(item, { status: 'error', progress: 0, error: errorMessage(error) })
  }
}

async function importAll() {
  if (importing.value || !queuedItems.value.length) return
  importing.value = true
  try {
    for (const item of [...queuedItems.value]) {
      await uploadItem(item)
    }
  } finally {
    importing.value = false
  }
}

onMounted(() => {
  window.addEventListener('paste', handlePaste)
  window.addEventListener('dragover', handleWindowDragOver)
  window.addEventListener('dragenter', handleWindowDragEnter)
  window.addEventListener('dragleave', handleWindowDragLeave)
  window.addEventListener('drop', handleWindowDrop)
})

onUnmounted(() => {
  window.removeEventListener('paste', handlePaste)
  window.removeEventListener('dragover', handleWindowDragOver)
  window.removeEventListener('dragenter', handleWindowDragEnter)
  window.removeEventListener('dragleave', handleWindowDragLeave)
  window.removeEventListener('drop', handleWindowDrop)
  for (const item of queue.value) revoke(item)
})
</script>

<template>
  <div
    class="sketch-import-zone"
    :class="{ 'sketch-import-zone--active': dragActive, 'sketch-import-zone--disabled': disabled }"
    @dragenter="handleDragEnter"
    @dragover="handleDragOver"
    @dragleave="handleDragLeave"
    @drop="handleDrop"
  >
    <input
      ref="fileInput"
      class="sketch-import-zone__input"
      type="file"
      multiple
      accept="image/jpeg,image/png,image/webp"
      :disabled="disabled"
      @change="handleFileInput"
    >

    <div class="sketch-import-zone__title">
      {{ dragActive ? 'Drop sketches here' : 'Drop, paste or choose sketches' }}
    </div>
    <div class="sketch-import-zone__hint">
      PNG, JPEG or WebP · up to 10 MB each · Ctrl+V / ⌘V works for images
    </div>
    <el-button type="primary" plain :disabled="disabled" @click="chooseFiles">Choose sketches</el-button>

    <div v-if="queue.length" class="sketch-import-zone__queue">
      <div v-for="item in queue" :key="item.id" class="sketch-import-item">
        <img v-if="item.previewUrl" :src="item.previewUrl" :alt="item.name" class="sketch-import-item__preview">
        <div class="sketch-import-item__meta">
          <strong>{{ item.name }}</strong>
          <small>{{ item.file ? formatSketchFileSize(item.size) : 'Web image URL' }}</small>
          <el-tag v-if="item.status === 'uploaded'" type="success">Imported · {{ item.asset?.width }}×{{ item.asset?.height }}</el-tag>
          <el-tag v-else-if="item.status === 'error'" type="danger">Error</el-tag>
          <el-progress
            v-else
            :percentage="item.progress"
            :status="item.status === 'uploading' ? undefined : 'exception'"
            :show-text="false"
          />
          <p v-if="item.error" class="sketch-import-item__error">{{ item.error }}</p>
        </div>
        <el-button v-if="item.status !== 'uploading'" text @click="removeItem(item)">Remove</el-button>
      </div>
    </div>

    <div v-if="queue.length" class="sketch-import-zone__footer">
      <span>{{ completedCount }}/{{ queue.length }} imported · {{ overallProgress }}%</span>
      <el-space>
        <el-button v-if="uploadedItems.length" text @click="clearUploaded">Clear completed</el-button>
        <el-button type="primary" :loading="importing" :disabled="!queuedItems.length" @click="importAll">
          Import {{ queuedItems.length }} sketch{{ queuedItems.length === 1 ? '' : 'es' }}
        </el-button>
      </el-space>
    </div>

    <Teleport to="body">
      <div v-if="dragActive" class="sketch-import-overlay">
        <div class="sketch-import-overlay__card">
          <strong>Release to add sketch</strong>
          <span>Images from files or public browser URLs are supported.</span>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.sketch-import-zone {
  border: 1px dashed var(--el-border-color);
  border-radius: 12px;
  padding: 24px;
  text-align: center;
  transition: border-color 0.15s ease, background-color 0.15s ease, transform 0.15s ease;
  position: relative;
  background: var(--el-bg-color);
}

.sketch-import-zone--active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  transform: translateY(-1px);
}

.sketch-import-zone--disabled {
  opacity: 0.65;
  pointer-events: none;
}

.sketch-import-zone__input {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}

.sketch-import-zone__title {
  font-weight: 700;
  font-size: 18px;
}

.sketch-import-zone__hint {
  margin: 8px 0 16px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.sketch-import-zone__queue {
  display: grid;
  gap: 10px;
  margin-top: 18px;
  text-align: left;
}

.sketch-import-item {
  display: grid;
  grid-template-columns: 72px 1fr auto;
  gap: 12px;
  align-items: center;
  padding: 10px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--el-bg-color-overlay);
}

.sketch-import-item__preview {
  width: 72px;
  height: 56px;
  object-fit: cover;
  border-radius: 6px;
  background: var(--el-fill-color-light);
}

.sketch-import-item__meta {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.sketch-import-item__meta strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sketch-import-item__meta small {
  color: var(--el-text-color-secondary);
}

.sketch-import-item__error {
  margin: 0;
  color: var(--el-color-danger);
  font-size: 12px;
}

.sketch-import-zone__footer {
  margin-top: 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  color: var(--el-text-color-secondary);
}

.sketch-import-overlay {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: grid;
  place-items: center;
  pointer-events: none;
  background: color-mix(in srgb, var(--el-color-primary) 12%, transparent);
}

.sketch-import-overlay__card {
  display: grid;
  gap: 6px;
  padding: 28px 36px;
  border: 2px solid var(--el-color-primary);
  border-radius: 16px;
  background: var(--el-bg-color);
  box-shadow: var(--el-box-shadow-dark);
  text-align: center;
}

@media (max-width: 700px) {
  .sketch-import-item {
    grid-template-columns: 56px 1fr;
  }

  .sketch-import-item .el-button {
    grid-column: 2;
    justify-self: start;
  }
}
</style>
