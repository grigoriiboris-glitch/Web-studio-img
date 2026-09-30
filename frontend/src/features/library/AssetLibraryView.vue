<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RouterLink } from 'vue-router'
import { assetLibraryApi, projectsApi, type AssetLibraryAssetType, type AssetLibraryItem, type AssetLibrarySource, type Project } from '../../api/client'

const types: AssetLibraryAssetType[] = ['character','object','product','logo','symbol','background','texture','material','mask','image','other']
const items = ref<AssetLibraryItem[]>([])
const projects = ref<Project[]>([])
const sources = ref<AssetLibrarySource[]>([])
const selectedProjectId = ref('')
const sourceAssetId = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const search = ref('')
const typeFilter = ref<AssetLibraryAssetType | ''>('')
const tagFilter = ref('')
const showArchived = ref(false)
const addDialog = ref(false)
const versionDialog = ref(false)
const useDialog = ref(false)
const detailsDialog = ref(false)
const selectedItem = ref<AssetLibraryItem | null>(null)
const name = ref('')
const description = ref('')
const assetType = ref<AssetLibraryAssetType>('image')
const tags = ref('')
const useProjectId = ref('')
const detailVersions = ref<AssetLibraryItem['current'][]>([])
const detailUsage = ref<Array<{ project_name?: string; rights_status: string; created_at: string }>>([])

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = (await assetLibraryApi.list({
      q: search.value.trim() || undefined,
      type: typeFilter.value || undefined,
      tag: tagFilter.value.trim() || undefined,
      status: showArchived.value ? 'archived' : 'active',
    })).items
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load asset library'
  } finally {
    loading.value = false
  }
}

async function loadSources(projectId: string) {
  selectedProjectId.value = projectId
  sourceAssetId.value = ''
  sources.value = projectId ? (await assetLibraryApi.sources(projectId)).sources : []
}

function openAdd() {
  name.value = ''
  description.value = ''
  assetType.value = 'image'
  tags.value = ''
  selectedProjectId.value = projects.value[0]?.id ?? ''
  addDialog.value = true
  void loadSources(selectedProjectId.value)
}

function openVersion(item: AssetLibraryItem) {
  selectedItem.value = item
  selectedProjectId.value = projects.value[0]?.id ?? ''
  versionDialog.value = true
  void loadSources(selectedProjectId.value)
}

function openUse(item: AssetLibraryItem) {
  selectedItem.value = item
  useProjectId.value = projects.value[0]?.id ?? ''
  useDialog.value = true
}

async function openDetails(item: AssetLibraryItem) {
  try {
    const data = await assetLibraryApi.get(item.id)
    selectedItem.value = data.item
    detailVersions.value = data.versions as AssetLibraryItem['current'][]
    detailUsage.value = data.usage
    detailsDialog.value = true
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not load asset details')
  }
}

async function addToLibrary() {
  if (!name.value.trim() || !sourceAssetId.value) {
    ElMessage.warning('Choose a source asset and name')
    return
  }
  saving.value = true
  try {
    await assetLibraryApi.create({
      source_asset_id: sourceAssetId.value,
      name: name.value.trim(),
      description: description.value.trim() || undefined,
      asset_type: assetType.value,
      tags: tags.value.split(',').map((tag) => tag.trim()).filter(Boolean),
    })
    addDialog.value = false
    ElMessage.success('Asset added to library')
    await load()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not add asset')
  } finally {
    saving.value = false
  }
}

async function addVersion() {
  if (!selectedItem.value || !sourceAssetId.value) {
    ElMessage.warning('Choose a source asset')
    return
  }
  saving.value = true
  try {
    await assetLibraryApi.createVersion(selectedItem.value.id, sourceAssetId.value)
    versionDialog.value = false
    ElMessage.success('New immutable version created')
    await load()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not create version')
  } finally {
    saving.value = false
  }
}

async function useAsset() {
  if (!selectedItem.value || !useProjectId.value) {
    ElMessage.warning('Choose a target project')
    return
  }
  saving.value = true
  try {
    await assetLibraryApi.useInProject(selectedItem.value.id, { project_id: useProjectId.value, rights_status: 'inherited' })
    useDialog.value = false
    ElMessage.success('Asset usage recorded for project')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not use asset in project')
  } finally {
    saving.value = false
  }
}

async function archive(item: AssetLibraryItem) {
  try {
    await ElMessageBox.confirm('Archive “' + item.name + '”? Original project assets stay unchanged.', 'Archive library asset', { type: 'warning' })
    await assetLibraryApi.archive(item.id)
    ElMessage.success('Asset archived')
    await load()
  } catch {
    // cancelled
  }
}

watch([search, typeFilter, tagFilter, showArchived], () => { void load() })
watch(selectedProjectId, (id) => {
  if (addDialog.value || versionDialog.value) void loadSources(id)
})
onMounted(async () => {
  try {
    projects.value = (await projectsApi.list()).projects
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not load projects')
  }
  await load()
})
</script>

<template>
  <!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline, vue/attributes-order, vue/html-self-closing -->
  <el-container class="library-page">
    <el-header class="header">
      <div>
        <RouterLink to="/projects">Projects</RouterLink>
        <h2>Global Asset Library</h2>
        <span>Immutable cross-project assets, versions, usage and rights snapshots.</span>
      </div>
      <el-button type="primary" @click="openAdd">Add asset</el-button>
    </el-header>

    <el-main>
      <el-alert v-if="error" type="error" :title="error" show-icon class="error" />
      <el-card class="filters">
        <el-input v-model="search" placeholder="Search name or description" clearable />
        <el-select v-model="typeFilter" clearable placeholder="Type" style="width: 180px">
          <el-option v-for="type in types" :key="type" :label="type" :value="type" />
        </el-select>
        <el-input v-model="tagFilter" placeholder="Tag" clearable style="width: 180px" />
        <el-switch v-model="showArchived" active-text="Archived" />
      </el-card>

      <div v-loading="loading" class="grid">
        <el-card v-for="item in items" :key="item.id" class="item">
          <img v-if="item.current?.thumbnail_url || item.current?.preview_url" :src="item.current.thumbnail_url || item.current.preview_url" class="preview" alt="" />
          <div class="meta">
            <strong>{{ item.name }}</strong>
            <el-tag size="small">{{ item.asset_type }}</el-tag>
            <el-tag v-for="tag in item.tags" :key="tag" size="small" effect="plain">{{ tag }}</el-tag>
          </div>
          <p>{{ item.description || 'No description' }}</p>
          <div class="facts">
            <span>Version {{ item.current_version }}</span>
            <span>{{ item.status }}</span>
            <span>Checksum {{ item.current?.checksum?.slice(0, 10) }}</span>
          </div>
          <div class="actions">
            <el-button link @click="openDetails(item)">Details</el-button>
            <el-button link @click="openVersion(item)" :disabled="item.status === 'archived'">New version</el-button>
            <el-button link @click="openUse(item)" :disabled="item.status === 'archived'">Use in project</el-button>
            <el-button link type="danger" @click="archive(item)" :disabled="item.status === 'archived'">Archive</el-button>
          </div>
        </el-card>
      </div>
      <el-empty v-if="!loading && items.length === 0" description="No library assets match the filter" />
    </el-main>

    <el-dialog v-model="addDialog" title="Add asset to library" width="640px">
      <el-form label-position="top">
        <el-form-item label="Source project">
          <el-select v-model="selectedProjectId" filterable style="width: 100%">
            <el-option v-for="project in projects" :key="project.id" :label="project.name" :value="project.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Source asset">
          <el-select v-model="sourceAssetId" filterable style="width: 100%">
            <el-option v-for="source in sources" :key="source.id" :label="source.type + ' · ' + source.id.slice(0, 8) + ' · ' + source.width + '×' + source.height" :value="source.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Name"><el-input v-model="name" maxlength="200" /></el-form-item>
        <el-form-item label="Type">
          <el-select v-model="assetType" style="width: 100%">
            <el-option v-for="type in types" :key="type" :label="type" :value="type" />
          </el-select>
        </el-form-item>
        <el-form-item label="Tags"><el-input v-model="tags" placeholder="character, hero, product" /></el-form-item>
        <el-form-item label="Description"><el-input v-model="description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="addDialog = false">Cancel</el-button><el-button type="primary" :loading="saving" @click="addToLibrary">Add</el-button></template>
    </el-dialog>

    <el-dialog v-model="versionDialog" title="Create immutable version" width="560px">
      <el-form label-position="top">
        <el-form-item label="Source project">
          <el-select v-model="selectedProjectId" filterable style="width: 100%">
            <el-option v-for="project in projects" :key="project.id" :label="project.name" :value="project.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Source asset">
          <el-select v-model="sourceAssetId" filterable style="width: 100%">
            <el-option v-for="source in sources" :key="source.id" :label="source.type + ' · ' + source.id.slice(0, 8) + ' · ' + source.width + '×' + source.height" :value="source.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="versionDialog = false">Cancel</el-button><el-button type="primary" :loading="saving" @click="addVersion">Create</el-button></template>
    </el-dialog>

    <el-dialog v-model="useDialog" title="Use asset in project" width="480px">
      <el-select v-model="useProjectId" filterable style="width: 100%">
        <el-option v-for="project in projects" :key="project.id" :label="project.name" :value="project.id" />
      </el-select>
      <p class="hint">Usage is tracked separately per project and keeps the selected rights status.</p>
      <template #footer><el-button @click="useDialog = false">Cancel</el-button><el-button type="primary" :loading="saving" @click="useAsset">Use</el-button></template>
    </el-dialog>

    <el-dialog v-model="detailsDialog" :title="selectedItem?.name || 'Asset details'" width="760px">
      <div v-if="selectedItem">
        <p><strong>Type:</strong> {{ selectedItem.asset_type }}</p>
        <p><strong>Current version:</strong> {{ selectedItem.current_version }}</p>
        <p><strong>Tags:</strong> {{ selectedItem.tags.join(', ') || '—' }}</p>
        <h4>Version history</h4>
        <el-table :data="detailVersions" stripe>
          <el-table-column prop="version" label="Version" width="90" />
          <el-table-column prop="checksum" label="Checksum" min-width="190" />
          <el-table-column prop="mime_type" label="MIME" width="140" />
          <el-table-column prop="created_at" label="Created" min-width="180" />
        </el-table>
        <h4>Used in projects</h4>
        <el-table :data="detailUsage" stripe>
          <el-table-column prop="project_name" label="Project" />
          <el-table-column prop="rights_status" label="Rights" />
          <el-table-column prop="created_at" label="Used at" min-width="180" />
        </el-table>
      </div>
    </el-dialog>
  </el-container>
</template>

<style scoped>
.library-page { min-height: 100vh; }
.header { display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.filters { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; flex-wrap: wrap; }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
.item { min-height: 360px; }
.preview { width: 100%; height: 180px; object-fit: contain; background: var(--el-fill-color-light); border-radius: 6px; }
.meta, .actions, .facts { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.meta { margin-top: 10px; }
.facts { color: var(--el-text-color-secondary); font-size: 12px; }
.actions { margin-top: 12px; }
.error { margin-bottom: 16px; }
.hint { color: var(--el-text-color-secondary); margin-top: 12px; }
</style>
