<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import {
  assetsApi,
  variantBoardApi,
  type Variant,
  type VariantSet,
  type VariantSource,
} from '../../api/client'

const route = useRoute()
const projectId = String(route.params.projectId)

const sources = ref<VariantSource[]>([])
const sets = ref<VariantSet[]>([])
const activeSet = ref<VariantSet | null>(null)
const variants = ref<Variant[]>([])
const error = ref('')
const loading = ref(false)
const creating = ref(false)
const setName = ref('')
const selectedSourceIds = ref<string[]>([])
const compareIds = ref<string[]>([])
const compareVariants = ref<Variant[]>([])
const imageUrls = ref<Record<string, string>>({})
const zoom = ref(1)
const offsetX = ref(0)
const offsetY = ref(0)
const dragging = ref(false)
const dragStart = ref({ x: 0, y: 0 })
const selectedVariantIds = computed(() =>
  variants.value
    .filter(item => item.decision === 'selected' || item.decision === 'kept')
    .map(item => item.id),
)

async function loadSources() {
  const response = await variantBoardApi.sources(projectId)
  sources.value = response.sources.filter(item => item.status === 'succeeded')
}

async function loadSets() {
  const response = await variantBoardApi.listSets(projectId)
  sets.value = response.variant_sets
  if (!activeSet.value && response.variant_sets.length) {
    await openSet(response.variant_sets[0])
  }
}

async function openSet(set: VariantSet) {
  activeSet.value = set
  const response = await variantBoardApi.getSet(projectId, set.id)
  variants.value = response.variants
  compareIds.value = variants.value
    .filter(item => item.compare_selected)
    .slice(0, 4)
    .map(item => item.id)
  await preloadImages(variants.value)
  await refreshCompare()
}

async function preloadImages(items: Variant[]) {
  const entries = await Promise.all(
    items
      .filter(item => item.asset_id)
      .map(async item => {
        try {
          const result = await assetsApi.downloadUrl(projectId, item.asset_id!)
          return [item.id, result.url] as const
        } catch {
          return null
        }
      }),
  )
  for (const entry of entries) {
    if (entry) imageUrls.value[entry[0]] = entry[1]
  }
}

async function createSet() {
  if (!setName.value.trim() || selectedSourceIds.value.length < 2) {
    error.value = 'Нужно выбрать минимум 2 завершённые генерации и задать имя набора.'
    return
  }
  creating.value = true
  error.value = ''
  try {
    const response = await variantBoardApi.createSet(projectId, {
      name: setName.value.trim(),
      generation_ids: selectedSourceIds.value,
    })
    setName.value = ''
    selectedSourceIds.value = []
    await loadSets()
    await openSet(response.variant_set)
  } catch (e: any) {
    error.value = e.message
  } finally {
    creating.value = false
  }
}

async function patch(
  item: Variant,
  input: Parameters<typeof variantBoardApi.patchVariant>[3],
) {
  try {
    const updated = await variantBoardApi.patchVariant(
      projectId,
      item.variant_set_id,
      item.id,
      input,
    )
    const index = variants.value.findIndex(value => value.id === updated.id)
    if (index >= 0) variants.value[index] = { ...variants.value[index], ...updated }
    if (input.compare_selected !== undefined) {
      if (
        input.compare_selected &&
        !compareIds.value.includes(item.id) &&
        compareIds.value.length < 4
      ) {
        compareIds.value.push(item.id)
      }
      if (!input.compare_selected) {
        compareIds.value = compareIds.value.filter(id => id !== item.id)
      }
    }
    if (input.compare_selected === undefined && input.favorite === undefined) {
      await refreshCompare()
    }
  } catch (e: any) {
    error.value = e.message
  }
}

async function refreshCompare() {
  if (!activeSet.value || compareIds.value.length < 2) {
    compareVariants.value = []
    return
  }
  compareVariants.value = (
    await variantBoardApi.compare(projectId, activeSet.value.id, compareIds.value)
  ).variants
  await preloadImages(compareVariants.value)
}

async function toggleCompare(item: Variant) {
  if (!compareIds.value.includes(item.id) && compareIds.value.length >= 4) {
    error.value = 'Сравнение поддерживает максимум 4 варианта.'
    return
  }
  const next = !compareIds.value.includes(item.id)
  await patch(item, { compare_selected: next })
  await refreshCompare()
}

async function createSelectionIteration() {
  if (!activeSet.value || selectedVariantIds.value.length === 0) {
    error.value = 'Сначала отметь хотя бы один вариант как Keep или Selected.'
    return
  }
  try {
    await variantBoardApi.createIteration(
      projectId,
      activeSet.value.id,
      selectedVariantIds.value,
      activeSet.value.name + ' selection',
    )
    error.value = ''
  } catch (e: any) {
    error.value = e.message
  }
}

function startDrag(event: PointerEvent) {
  dragging.value = true
  dragStart.value = {
    x: event.clientX - offsetX.value,
    y: event.clientY - offsetY.value,
  }
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function moveDrag(event: PointerEvent) {
  if (!dragging.value) return
  offsetX.value = event.clientX - dragStart.value.x
  offsetY.value = event.clientY - dragStart.value.y
}

function endDrag() {
  dragging.value = false
}

function wheelZoom(event: WheelEvent) {
  event.preventDefault()
  zoom.value = Math.min(
    3,
    Math.max(0.5, zoom.value + (event.deltaY > 0 ? -0.1 : 0.1)),
  )
}

onMounted(async () => {
  loading.value = true
  try {
    await Promise.all([loadSources(), loadSets()])
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <main class="variant-board">
    <header class="page-header">
      <div>
        <h1>Variant Board</h1>
        <p>Выбор, сравнение и фиксация решений по результатам генерации.</p>
      </div>
      <button type="button" class="primary" @click="createSelectionIteration">
        Create selection iteration
      </button>
    </header>

    <p v-if="error" class="error">{{ error }}</p>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>New Variant Set</h2>
          <p>Объедини завершённые генерации в одну рабочую серию.</p>
        </div>
      </div>

      <div class="create-grid">
        <label>
          Set name
          <input v-model="setName" maxlength="200" placeholder="Product hero — round 1">
        </label>
        <div class="source-list">
          <div v-for="source in sources" :key="source.generation_id" class="source-row">
            <label class="checkbox-row">
              <input
                v-model="selectedSourceIds"
                :value="source.generation_id"
                type="checkbox"
                :disabled="selectedSourceIds.length >= 50 && !selectedSourceIds.includes(source.generation_id)"
              >
              <span>
                <strong>{{ source.provider }} / {{ source.model }}</strong>
                <small>{{ new Date(source.created_at).toLocaleString() }}</small>
                <small>{{ source.prompt.slice(0, 120) }}</small>
              </span>
            </label>
          </div>
        </div>
      </div>

      <button type="button" :disabled="creating" @click="createSet">
        {{ creating ? 'Creating…' : 'Create Variant Set (' + selectedSourceIds.length + ')' }}
      </button>
    </section>

    <section class="layout">
      <aside class="panel sets-panel">
        <h2>Variant Sets</h2>
        <button
          v-for="set in sets"
          :key="set.id"
          type="button"
          class="set-button"
          :class="{ active: activeSet?.id === set.id }"
          @click="openSet(set)"
        >
          {{ set.name }}
        </button>
        <p v-if="!sets.length">Пока нет Variant Sets.</p>
      </aside>

      <section class="panel board-panel">
        <div class="panel-title">
          <div>
            <h2>{{ activeSet?.name || 'Select a Variant Set' }}</h2>
            <p v-if="activeSet">Выбери лучшие варианты, добавь reject/favorite и сравни до 4.</p>
          </div>
          <span v-if="variants.length" class="counter">{{ variants.length }} variants</span>
        </div>

        <div v-if="loading" class="empty">Loading…</div>
        <div v-else-if="!variants.length" class="empty">
          Выбери Variant Set слева.
        </div>

        <div v-else class="variant-grid">
          <article
            v-for="item in variants"
            :key="item.id"
            class="variant-card"
            :class="{ rejected: item.decision === 'rejected', selected: item.decision === 'selected' || item.decision === 'kept' }"
          >
            <div class="image-wrap">
              <img
                v-if="imageUrls[item.id]"
                :src="imageUrls[item.id]"
                :alt="'Variant ' + item.ordinal"
              >
              <div v-else class="image-placeholder">
                No image
              </div>
              <span class="ordinal">#{{ item.ordinal }}</span>
              <span v-if="item.favorite" class="favorite">★</span>
            </div>

            <div class="variant-body">
              <div class="decision">
                <strong>{{ item.decision }}</strong>
                <span v-if="item.source">{{ item.source.provider }} · {{ item.source.model }}</span>
              </div>

              <p v-if="item.source" class="prompt">
                {{ item.source.prompt }}
              </p>

              <div class="actions">
                <button type="button" @click="patch(item, { decision: 'kept' })">
                  Keep
                </button>
                <button type="button" @click="patch(item, { decision: 'selected' })">
                  Select
                </button>
                <button
                  type="button"
                  class="danger"
                  @click="patch(item, { decision: item.decision === 'rejected' ? 'candidate' : 'rejected', reject_reason: item.decision === 'rejected' ? [] : ['other'] })"
                >
                  {{ item.decision === 'rejected' ? 'Restore' : 'Reject' }}
                </button>
                <button type="button" @click="patch(item, { favorite: !item.favorite })">
                  {{ item.favorite ? 'Unfavorite' : 'Favorite' }}
                </button>
                <button
                  type="button"
                  :class="{ active: compareIds.includes(item.id) }"
                  @click="toggleCompare(item)"
                >
                  {{ compareIds.includes(item.id) ? 'Compare ✓' : 'Compare' }}
                </button>
              </div>
            </div>
          </article>
        </div>
      </section>
    </section>

    <section v-if="compareVariants.length >= 2" class="panel compare-panel">
      <div class="panel-title">
        <div>
          <h2>Compare</h2>
          <p>Синхронные zoom/pan для выбранных вариантов.</p>
        </div>
        <span>{{ Math.round(zoom * 100) }}%</span>
      </div>

      <div class="compare-grid">
        <div
          v-for="item in compareVariants"
          :key="item.id"
          class="compare-cell"
          @wheel="wheelZoom"
          @pointerdown="startDrag"
          @pointermove="moveDrag"
          @pointerup="endDrag"
          @pointercancel="endDrag"
          @pointerleave="endDrag"
        >
          <img
            v-if="imageUrls[item.id]"
            :src="imageUrls[item.id]"
            :alt="'Variant ' + item.ordinal"
            class="compare-image"
            :style="{ transform: 'translate(' + offsetX + 'px, ' + offsetY + 'px) scale(' + zoom + ')' }"
          >
          <span class="compare-label">#{{ item.ordinal }}</span>
        </div>
      </div>
    </section>
  </main>
</template>

<style scoped>
.variant-board {
  min-height: 100vh;
  padding: 24px;
  background: var(--el-bg-color-page, #f7f8fa);
}

.page-header,
.panel-title,
.layout {
  display: flex;
  gap: 16px;
}

.page-header,
.panel-title {
  align-items: flex-start;
  justify-content: space-between;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h1,
.panel h2 {
  margin: 0 0 6px;
}

.page-header p,
.panel-title p {
  margin: 0;
  color: #667085;
}

.panel {
  margin-bottom: 20px;
  padding: 18px;
  border: 1px solid #ddd;
  border-radius: 12px;
  background: #fff;
}

.primary {
  padding: 10px 14px;
}

.create-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(240px, 320px) 1fr;
  margin-bottom: 16px;
}

.create-grid label {
  display: grid;
  gap: 6px;
  font-weight: 600;
}

.create-grid input[type="text"],
.create-grid input:not([type]) {
  width: 100%;
  box-sizing: border-box;
  padding: 9px;
  border: 1px solid #bbb;
  border-radius: 8px;
}

.source-list {
  max-height: 220px;
  overflow: auto;
  border: 1px solid #eee;
  border-radius: 8px;
}

.source-row {
  padding: 8px 10px;
  border-bottom: 1px solid #eee;
}

.checkbox-row {
  display: flex !important;
  grid-template-columns: none !important;
  gap: 10px !important;
}

.checkbox-row span {
  display: grid;
  gap: 2px;
}

.checkbox-row small {
  color: #667085;
  font-weight: 400;
}

.layout {
  align-items: flex-start;
}

.sets-panel {
  flex: 0 0 240px;
}

.board-panel {
  flex: 1;
}

.set-button {
  display: block;
  width: 100%;
  margin-bottom: 8px;
  padding: 10px;
  text-align: left;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fff;
}

.set-button.active {
  border-color: #409eff;
}

.variant-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  margin-top: 16px;
}

.variant-card {
  overflow: hidden;
  border: 1px solid #ddd;
  border-radius: 10px;
  background: #fff;
}

.variant-card.selected {
  border-color: #67c23a;
}

.variant-card.rejected {
  opacity: 0.62;
}

.image-wrap {
  position: relative;
  aspect-ratio: 1;
  overflow: hidden;
  background: #f0f0f0;
}

.image-wrap img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.image-placeholder,
.empty {
  display: grid;
  min-height: 180px;
  place-items: center;
  color: #667085;
}

.ordinal,
.favorite {
  position: absolute;
  top: 8px;
  padding: 4px 7px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
}

.ordinal {
  left: 8px;
}

.favorite {
  right: 8px;
}

.variant-body {
  padding: 10px;
}

.decision {
  display: grid;
  gap: 3px;
}

.decision span {
  color: #667085;
  font-size: 12px;
}

.prompt {
  display: -webkit-box;
  overflow: hidden;
  margin: 8px 0;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  color: #475467;
  font-size: 12px;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.actions button {
  padding: 5px 8px;
}

.actions button.active {
  border-color: #409eff;
}

.actions .danger {
  color: #b42318;
}

.compare-panel {
  overflow: hidden;
}

.compare-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.compare-cell {
  position: relative;
  min-height: 320px;
  overflow: hidden;
  touch-action: none;
  border-radius: 8px;
  background: #111;
  cursor: grab;
}

.compare-cell:active {
  cursor: grabbing;
}

.compare-image {
  width: 100%;
  height: 320px;
  object-fit: contain;
  transform-origin: center;
  user-select: none;
}

.compare-label {
  position: absolute;
  left: 8px;
  top: 8px;
  padding: 4px 7px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
}

.counter {
  color: #667085;
}

.error {
  margin-bottom: 16px;
  color: #b42318;
}

@media (max-width: 900px) {
  .layout {
    flex-direction: column;
  }

  .create-grid {
    grid-template-columns: 1fr;
  }

  .sets-panel {
    flex-basis: auto;
    width: 100%;
  }
}
</style>
