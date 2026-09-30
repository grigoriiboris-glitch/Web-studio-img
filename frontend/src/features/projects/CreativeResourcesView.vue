<!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline -->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RouterLink, useRoute } from 'vue-router'
import {
  creativeLibraryApi,
  styleApi,
  dnaApi,
  rightsApi,
  constraintsApi,
  layersApi,
  manualEditsApi,
  type CreativeLibraryItem,
  type StyleProfile,
  type AssetDNAProfile,
  type VisualLanguageProfile,
  type RightsItem,
  type DoNotUseConstraint,
  type Layer,
  type ManualEdit,
} from '../../api/creative'

const route = useRoute()
const projectId = () => String(route.params.projectId)

const loading = ref(false)
const error = ref<string | null>(null)
const libraryKind = ref<'material' | 'texture'>('material')
const libraryItems = ref<CreativeLibraryItem[]>([])
const libraryQuery = ref('')
const libraryDialog = ref(false)
const libraryEditing = ref<CreativeLibraryItem | null>(null)
const libraryForm = reactive({ category: '', name: '', description: '', tags: '', prompt_fragment: '' })

const styles = ref<StyleProfile[]>([])
const styleDialog = ref(false)
const styleEditing = ref<StyleProfile | null>(null)
const styleForm = reactive({ name: '', description: '', parameters: '{}', prompt_influence: false })

const assetDNA = ref<AssetDNAProfile[]>([])
const visualLanguages = ref<VisualLanguageProfile[]>([])
const dnaLoading = ref(false)

const rights = ref<RightsItem[]>([])
const rightsForm = reactive({
  target_type: 'asset' as RightsItem['target_type'],
  target_id: '',
  ownership: 'unknown' as RightsItem['ownership'],
  license: 'unknown',
  license_source: '',
  verification_state: 'unknown' as RightsItem['verification_state'],
  notes: '',
})
const constraints = ref<DoNotUseConstraint[]>([])
const constraintForm = reactive({ kind: 'artist' as DoNotUseConstraint['kind'], value: '', active: true })

const layers = ref<Layer[]>([])
const layerForm = reactive({
  name: '',
  layer_type: 'image' as Layer['layer_type'],
  order_index: 0,
  opacity: 1,
  visible: true,
  blend_mode: 'normal',
  source_kind: 'human' as Layer['source_kind'],
  asset_id: '',
  iteration_id: '',
})
const edits = ref<ManualEdit[]>([])
const editForm = reactive({
  source_asset_id: '',
  mask_asset_id: '',
  result_asset_id: '',
  iteration_id: '',
  operation: 'mask' as ManualEdit['operation'],
  prompt: '',
  parameters: '{}',
})

async function loadLibrary() {
  libraryItems.value = (await creativeLibraryApi.list(projectId(), libraryKind.value, libraryQuery.value)).items
}

async function loadAll() {
  loading.value = true
  error.value = null
  try {
    await Promise.all([
      loadLibrary(),
      loadStyles(),
      loadDNA(),
      loadRights(),
      loadConstraints(),
      loadLayers(),
      loadEdits(),
    ])
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load creative resources'
  } finally {
    loading.value = false
  }
}

async function loadStyles() {
  styles.value = (await styleApi.list()).profiles
}

async function loadDNA() {
  dnaLoading.value = true
  try {
    const [asset, language] = await Promise.all([
      dnaApi.listAsset(projectId()),
      dnaApi.listLanguage(projectId()),
    ])
    assetDNA.value = asset.profiles
    visualLanguages.value = language.profiles
  } finally {
    dnaLoading.value = false
  }
}

async function loadRights() {
  rights.value = (await rightsApi.list(projectId())).items
}

async function loadConstraints() {
  constraints.value = (await constraintsApi.listProject(projectId())).constraints
}

async function loadLayers() {
  layers.value = (await layersApi.list(projectId())).layers
}

async function loadEdits() {
  edits.value = (await manualEditsApi.list(projectId())).edits
}

function openLibraryCreate() {
  libraryEditing.value = null
  Object.assign(libraryForm, { category: '', name: '', description: '', tags: '', prompt_fragment: '' })
  libraryDialog.value = true
}

function openLibraryEdit(item: CreativeLibraryItem) {
  libraryEditing.value = item
  Object.assign(libraryForm, {
    category: item.category,
    name: item.name,
    description: item.description ?? '',
    tags: item.tags.join(', '),
    prompt_fragment: item.prompt_fragment,
  })
  libraryDialog.value = true
}

async function saveLibraryItem() {
  if (!libraryForm.name.trim() || !libraryForm.category.trim() || !libraryForm.prompt_fragment.trim()) {
    ElMessage.warning('Category, name and prompt fragment are required')
    return
  }
  const input = {
    category: libraryForm.category.trim(),
    name: libraryForm.name.trim(),
    description: libraryForm.description.trim() || undefined,
    tags: libraryForm.tags.split(',').map((value) => value.trim()).filter(Boolean),
    prompt_fragment: libraryForm.prompt_fragment.trim(),
  }
  try {
    if (libraryEditing.value) {
      await creativeLibraryApi.update(projectId(), libraryKind.value, libraryEditing.value.id, input)
    } else {
      await creativeLibraryApi.create(projectId(), libraryKind.value, input)
    }
    libraryDialog.value = false
    await loadLibrary()
    ElMessage.success('Library item saved')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not save library item')
  }
}

async function deleteLibraryItem(item: CreativeLibraryItem) {
  if (item.user_id === undefined) {
    ElMessage.warning('System library items cannot be deleted')
    return
  }
  try {
    await ElMessageBox.confirm('Delete this library item?', 'Delete', { type: 'warning' })
    await creativeLibraryApi.remove(projectId(), libraryKind.value, item.id)
    await loadLibrary()
    ElMessage.success('Library item deleted')
  } catch {
    // cancelled
  }
}

async function selectLibraryItem(item: CreativeLibraryItem) {
  try {
    await creativeLibraryApi.select(projectId(), libraryKind.value, item.id)
    ElMessage.success('Selection recorded in creative history')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not record selection')
  }
}

function openStyleCreate() {
  styleEditing.value = null
  Object.assign(styleForm, { name: '', description: '', parameters: '{}', prompt_influence: false })
  styleDialog.value = true
}

function openStyleEdit(profile: StyleProfile) {
  styleEditing.value = profile
  Object.assign(styleForm, {
    name: profile.name,
    description: profile.description ?? '',
    parameters: JSON.stringify(profile.parameters, null, 2),
    prompt_influence: profile.prompt_influence,
  })
  styleDialog.value = true
}

async function saveStyle() {
  let parameters: Record<string, unknown>
  try {
    parameters = JSON.parse(styleForm.parameters || '{}') as Record<string, unknown>
  } catch {
    ElMessage.warning('Style parameters must be valid JSON')
    return
  }
  try {
    if (styleEditing.value) {
      await styleApi.update(styleEditing.value.id, {
        name: styleForm.name.trim(),
        description: styleForm.description.trim() || undefined,
        parameters,
        prompt_influence: styleForm.prompt_influence,
      })
    } else {
      await styleApi.create({
        name: styleForm.name.trim(),
        description: styleForm.description.trim() || undefined,
        parameters,
        prompt_influence: styleForm.prompt_influence,
      })
    }
    styleDialog.value = false
    await loadStyles()
    ElMessage.success('Style profile saved')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not save style profile')
  }
}

async function inferStyle() {
  try {
    const inferred = await styleApi.infer(projectId())
    styles.value.unshift(inferred)
    ElMessage.success('Style profile inferred from recorded human choices')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not infer style profile')
  }
}

async function applyStyle(profile: StyleProfile) {
  try {
    const result = await styleApi.apply(projectId(), profile.id)
    await ElMessageBox.alert(JSON.stringify(result.prompt_suggestion ?? {}, null, 2), 'Prompt suggestion', { type: 'info' })
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not apply style profile')
  }
}

async function analyzeAssetDNA() {
  try {
    const result = await dnaApi.analyzeAsset(projectId())
    assetDNA.value.unshift(result)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not analyze Asset DNA')
  }
}

async function analyzeVisualLanguage() {
  try {
    const result = await dnaApi.analyzeLanguage(projectId())
    visualLanguages.value.unshift(result)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not analyze Personal Visual Language')
  }
}

async function applyVisualLanguage(profile: VisualLanguageProfile) {
  try {
    const result = await dnaApi.applyLanguage(projectId(), profile.id)
    await ElMessageBox.alert(JSON.stringify(result.prompt_suggestion ?? {}, null, 2), 'Visual language suggestion', { type: 'info' })
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not apply visual language')
  }
}

async function saveRights() {
  if (!rightsForm.target_id.trim()) {
    ElMessage.warning('Target ID is required')
    return
  }
  try {
    await rightsApi.create(projectId(), {
      target_type: rightsForm.target_type,
      target_id: rightsForm.target_id.trim(),
      ownership: rightsForm.ownership,
      license: rightsForm.license.trim() || 'unknown',
      license_source: rightsForm.license_source.trim() || undefined,
      verification_state: rightsForm.verification_state,
      notes: rightsForm.notes.trim() || undefined,
    })
    Object.assign(rightsForm, {
      target_id: '', license: 'unknown', license_source: '', notes: '',
      ownership: 'unknown', verification_state: 'unknown',
    })
    await loadRights()
    ElMessage.success('Rights record saved')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not save rights')
  }
}

async function removeRights(item: RightsItem) {
  try {
    await rightsApi.remove(projectId(), item.id)
    await loadRights()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not delete rights record')
  }
}

async function addConstraint() {
  if (!constraintForm.value.trim()) {
    ElMessage.warning('Constraint value is required')
    return
  }
  try {
    await constraintsApi.createProject(projectId(), { ...constraintForm, value: constraintForm.value.trim() })
    constraintForm.value = ''
    await loadConstraints()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not save constraint')
  }
}

async function removeConstraint(item: DoNotUseConstraint) {
  try {
    await constraintsApi.remove(projectId(), item.id)
    await loadConstraints()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not delete constraint')
  }
}

async function saveLayer() {
  if (!layerForm.name.trim()) {
    ElMessage.warning('Layer name is required')
    return
  }
  try {
    await layersApi.create(projectId(), {
      name: layerForm.name.trim(),
      layer_type: layerForm.layer_type,
      order_index: layerForm.order_index,
      opacity: layerForm.opacity,
      visible: layerForm.visible,
      blend_mode: layerForm.blend_mode.trim() || 'normal',
      source_kind: layerForm.source_kind,
      asset_id: layerForm.asset_id.trim() || undefined,
      iteration_id: layerForm.iteration_id.trim() || undefined,
    })
    layerForm.name = ''
    await loadLayers()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not create layer')
  }
}

async function removeLayer(layer: Layer) {
  try {
    await layersApi.remove(projectId(), layer.id)
    await loadLayers()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not delete layer')
  }
}

async function saveManualEdit() {
  if (!editForm.source_asset_id.trim()) {
    ElMessage.warning('Source asset ID is required')
    return
  }
  let parameters: Record<string, unknown>
  try {
    parameters = JSON.parse(editForm.parameters || '{}') as Record<string, unknown>
  } catch {
    ElMessage.warning('Edit parameters must be valid JSON')
    return
  }
  try {
    await manualEditsApi.create(projectId(), {
      source_asset_id: editForm.source_asset_id.trim(),
      mask_asset_id: editForm.mask_asset_id.trim() || undefined,
      result_asset_id: editForm.result_asset_id.trim() || undefined,
      iteration_id: editForm.iteration_id.trim() || undefined,
      operation: editForm.operation,
      prompt: editForm.prompt.trim() || undefined,
      parameters,
    }, crypto.randomUUID())
    await loadEdits()
    ElMessage.success('Manual edit recorded')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not create manual edit')
  }
}

async function applyEdit(edit: ManualEdit) {
  try {
    const updated = await manualEditsApi.apply(projectId(), edit.id)
    edits.value = edits.value.map((item) => item.id === updated.id ? updated : item)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not apply manual edit')
  }
}

async function rejectEdit(edit: ManualEdit) {
  try {
    const updated = await manualEditsApi.reject(projectId(), edit.id)
    edits.value = edits.value.map((item) => item.id === updated.id ? updated : item)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not reject manual edit')
  }
}

onMounted(loadAll)
</script>

<template>
  <el-container class="resources">
    <el-header class="header">
      <div>
        <strong>Creative Resources</strong>
        <span class="muted">Libraries, Visual DNA, Rights and Layers</span>
      </div>
      <el-space>
        <RouterLink :to="`/projects/${projectId()}/studio`">
          <el-button>Back to Studio</el-button>
        </RouterLink>
        <RouterLink :to="`/projects/${projectId()}/visual-dna-v2`">
          <el-button type="primary">Visual DNA v2</el-button>
        </RouterLink>
      </el-space>
    </el-header>

    <el-main>
      <el-alert v-if="error" :title="error" type="error" show-icon class="error" />

      <el-tabs v-loading="loading">
        <el-tab-pane label="Materials & Textures">
          <el-space wrap>
            <el-radio-group v-model="libraryKind" @change="loadLibrary">
              <el-radio-button value="material">Materials</el-radio-button>
              <el-radio-button value="texture">Textures</el-radio-button>
            </el-radio-group>
            <el-input v-model="libraryQuery" clearable placeholder="Search" @keyup.enter="loadLibrary" />
            <el-button type="primary" @click="openLibraryCreate">New item</el-button>
          </el-space>
          <el-table :data="libraryItems" stripe style="margin-top: 16px">
            <el-table-column prop="name" label="Name" />
            <el-table-column prop="category" label="Category" />
            <el-table-column prop="prompt_fragment" label="Prompt fragment" />
            <el-table-column prop="tags" label="Tags">
              <template #default="scope">{{ scope.row.tags.join(', ') }}</template>
            </el-table-column>
            <el-table-column label="Actions" width="280">
              <template #default="scope">
                <el-button size="small" @click="selectLibraryItem(scope.row)">Select</el-button>
                <el-button v-if="scope.row.user_id" size="small" @click="openLibraryEdit(scope.row)">Edit</el-button>
                <el-button v-if="scope.row.user_id" size="small" type="danger" @click="deleteLibraryItem(scope.row)">Delete</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="Style DNA">
          <el-space wrap>
            <el-button type="primary" @click="inferStyle">Infer from project</el-button>
            <el-button @click="openStyleCreate">New profile</el-button>
            <el-button @click="loadStyles">Refresh</el-button>
          </el-space>
          <el-table :data="styles" stripe style="margin-top: 16px">
            <el-table-column prop="name" label="Profile" />
            <el-table-column prop="version" label="Version" width="90" />
            <el-table-column prop="prompt_influence" label="Prompt influence" width="150">
              <template #default="scope">{{ scope.row.prompt_influence ? 'enabled' : 'disabled' }}</template>
            </el-table-column>
            <el-table-column label="Actions" width="260">
              <template #default="scope">
                <el-button size="small" @click="openStyleEdit(scope.row)">Edit</el-button>
                <el-button size="small" type="primary" :disabled="!scope.row.prompt_influence" @click="applyStyle(scope.row)">Suggest prompt</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="Asset DNA">
          <el-space wrap>
            <el-button type="primary" :loading="dnaLoading" @click="analyzeAssetDNA">Analyze Asset DNA</el-button>
            <el-button @click="analyzeVisualLanguage">Analyze Personal Visual Language</el-button>
          </el-space>
          <el-row :gutter="16" style="margin-top: 16px">
            <el-col :span="12">
              <el-card>
                <template #header>Asset DNA</template>
                <el-empty v-if="assetDNA.length === 0" description="No profiles" />
                <div v-for="profile in assetDNA" :key="profile.id" class="json-block">
                  <strong>{{ profile.name }}</strong>
                  <pre>{{ JSON.stringify(profile.features, null, 2) }}</pre>
                </div>
              </el-card>
            </el-col>
            <el-col :span="12">
              <el-card>
                <template #header>Personal Visual Language</template>
                <el-empty v-if="visualLanguages.length === 0" description="No profiles" />
                <div v-for="profile in visualLanguages" :key="profile.id" class="json-block">
                  <strong>{{ profile.name }}</strong>
                  <pre>{{ JSON.stringify(profile.signals, null, 2) }}</pre>
                  <el-button size="small" :disabled="!profile.prompt_influence" @click="applyVisualLanguage(profile)">Use as suggestion</el-button>
                </div>
              </el-card>
            </el-col>
          </el-row>
        </el-tab-pane>

        <el-tab-pane label="Rights & Constraints">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-card>
                <template #header>Rights Registry</template>
                <el-form label-position="top">
                  <el-form-item label="Target type">
                    <el-select v-model="rightsForm.target_type">
                      <el-option value="asset" label="Asset" />
                      <el-option value="reference" label="Reference" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Target ID">
                    <el-input v-model="rightsForm.target_id" />
                  </el-form-item>
                  <el-form-item label="Ownership">
                    <el-select v-model="rightsForm.ownership">
                      <el-option value="unknown" label="Unknown" />
                      <el-option value="user_owned" label="User owned" />
                      <el-option value="third_party" label="Third party" />
                      <el-option value="public_domain" label="Public domain" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="License"><el-input v-model="rightsForm.license" /></el-form-item>
                  <el-form-item label="License source"><el-input v-model="rightsForm.license_source" /></el-form-item>
                  <el-form-item label="Verification">
                    <el-select v-model="rightsForm.verification_state">
                      <el-option value="unknown" label="Unknown" />
                      <el-option value="unverified" label="Unverified" />
                      <el-option value="verified" label="Verified" />
                      <el-option value="restricted" label="Restricted" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Notes"><el-input v-model="rightsForm.notes" type="textarea" :rows="2" /></el-form-item>
                  <el-button type="primary" @click="saveRights">Save rights</el-button>
                </el-form>
              </el-card>
              <el-table :data="rights" size="small" style="margin-top: 16px">
                <el-table-column prop="target_type" label="Target" />
                <el-table-column prop="target_id" label="ID" />
                <el-table-column prop="ownership" label="Ownership" />
                <el-table-column prop="verification_state" label="Verification" />
                <el-table-column label="Actions" width="90">
                  <template #default="scope"><el-button size="small" type="danger" @click="removeRights(scope.row)">Delete</el-button></template>
                </el-table-column>
              </el-table>
            </el-col>

            <el-col :span="12">
              <el-card>
                <template #header>Do Not Use constraints</template>
                <el-form label-position="top">
                  <el-form-item label="Kind">
                    <el-select v-model="constraintForm.kind">
                      <el-option value="artist" label="Artist" />
                      <el-option value="image" label="Image" />
                      <el-option value="reference" label="Reference" />
                      <el-option value="motif" label="Motif" />
                      <el-option value="brand" label="Brand" />
                      <el-option value="composition" label="Composition" />
                      <el-option value="style" label="Style" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Value"><el-input v-model="constraintForm.value" /></el-form-item>
                  <el-form-item><el-switch v-model="constraintForm.active" active-text="Active" /></el-form-item>
                  <el-button type="primary" @click="addConstraint">Add constraint</el-button>
                </el-form>
              </el-card>
              <el-table :data="constraints" size="small" style="margin-top: 16px">
                <el-table-column prop="kind" label="Kind" width="130" />
                <el-table-column prop="value" label="Value" />
                <el-table-column prop="active" label="Active" width="80" />
                <el-table-column label="Actions" width="90">
                  <template #default="scope"><el-button size="small" type="danger" @click="removeConstraint(scope.row)">Delete</el-button></template>
                </el-table-column>
              </el-table>
            </el-col>
          </el-row>
        </el-tab-pane>

        <el-tab-pane label="Layers & Manual Edit">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-card>
                <template #header>Project Layers</template>
                <el-form label-position="top">
                  <el-form-item label="Name"><el-input v-model="layerForm.name" /></el-form-item>
                  <el-form-item label="Type">
                    <el-select v-model="layerForm.layer_type">
                      <el-option value="base" label="Base" />
                      <el-option value="mask" label="Mask" />
                      <el-option value="image" label="Image" />
                      <el-option value="manual_edit" label="Manual edit" />
                      <el-option value="adjustment" label="Adjustment" />
                      <el-option value="group" label="Group" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Source kind">
                    <el-select v-model="layerForm.source_kind">
                      <el-option value="human" label="Human" />
                      <el-option value="ai" label="AI" />
                      <el-option value="derived" label="Derived" />
                      <el-option value="imported" label="Imported" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Asset ID"><el-input v-model="layerForm.asset_id" /></el-form-item>
                  <el-form-item label="Iteration ID"><el-input v-model="layerForm.iteration_id" /></el-form-item>
                  <el-button type="primary" @click="saveLayer">Add layer</el-button>
                </el-form>
              </el-card>
              <el-table :data="layers" size="small" style="margin-top: 16px">
                <el-table-column prop="name" label="Layer" />
                <el-table-column prop="layer_type" label="Type" width="120" />
                <el-table-column prop="source_kind" label="Source" width="110" />
                <el-table-column label="Actions" width="90">
                  <template #default="scope"><el-button size="small" type="danger" @click="removeLayer(scope.row)">Delete</el-button></template>
                </el-table-column>
              </el-table>
            </el-col>

            <el-col :span="12">
              <el-card>
                <template #header>Manual Edit Record</template>
                <el-alert type="info" :closable="false" title="Actual brush/lasso AI inpainting is tracked separately in issue #70." />
                <el-form label-position="top">
                  <el-form-item label="Source asset ID"><el-input v-model="editForm.source_asset_id" /></el-form-item>
                  <el-form-item label="Mask asset ID"><el-input v-model="editForm.mask_asset_id" /></el-form-item>
                  <el-form-item label="Result asset ID"><el-input v-model="editForm.result_asset_id" /></el-form-item>
                  <el-form-item label="Iteration ID"><el-input v-model="editForm.iteration_id" /></el-form-item>
                  <el-form-item label="Operation">
                    <el-select v-model="editForm.operation">
                      <el-option value="mask" label="Mask" />
                      <el-option value="paint" label="Paint" />
                      <el-option value="erase" label="Erase" />
                      <el-option value="composite" label="Composite" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Prompt"><el-input v-model="editForm.prompt" type="textarea" :rows="2" /></el-form-item>
                  <el-form-item label="Parameters JSON"><el-input v-model="editForm.parameters" type="textarea" :rows="4" /></el-form-item>
                  <el-button type="primary" @click="saveManualEdit">Record edit</el-button>
                </el-form>
              </el-card>
              <el-table :data="edits" size="small" style="margin-top: 16px">
                <el-table-column prop="operation" label="Operation" width="110" />
                <el-table-column prop="status" label="Status" width="100" />
                <el-table-column prop="source_asset_id" label="Source asset" />
                <el-table-column label="Actions" width="180">
                  <template #default="scope">
                    <el-button size="small" type="success" :disabled="scope.row.status !== 'draft' || !scope.row.result_asset_id" @click="applyEdit(scope.row)">Apply</el-button>
                    <el-button size="small" type="danger" :disabled="scope.row.status !== 'draft'" @click="rejectEdit(scope.row)">Reject</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-col>
          </el-row>
        </el-tab-pane>
      </el-tabs>
    </el-main>

    <el-dialog v-model="libraryDialog" :title="libraryEditing ? 'Edit library item' : 'New library item'" width="560px">
      <el-form label-position="top">
        <el-form-item label="Category"><el-input v-model="libraryForm.category" /></el-form-item>
        <el-form-item label="Name"><el-input v-model="libraryForm.name" /></el-form-item>
        <el-form-item label="Description"><el-input v-model="libraryForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="Tags"><el-input v-model="libraryForm.tags" placeholder="stone, luxury" /></el-form-item>
        <el-form-item label="Prompt fragment"><el-input v-model="libraryForm.prompt_fragment" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="libraryDialog = false">Cancel</el-button>
        <el-button type="primary" @click="saveLibraryItem">Save</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="styleDialog" :title="styleEditing ? 'Edit style profile' : 'New style profile'" width="620px">
      <el-form label-position="top">
        <el-form-item label="Name"><el-input v-model="styleForm.name" /></el-form-item>
        <el-form-item label="Description"><el-input v-model="styleForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="Parameters JSON"><el-input v-model="styleForm.parameters" type="textarea" :rows="8" /></el-form-item>
        <el-form-item><el-switch v-model="styleForm.prompt_influence" active-text="Allow prompt influence (user approval still required)" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="styleDialog = false">Cancel</el-button>
        <el-button type="primary" @click="saveStyle">Save</el-button>
      </template>
    </el-dialog>
  </el-container>
</template>

<style scoped>
.resources {
  min-height: 100vh;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.muted {
  margin-left: 12px;
  color: var(--el-text-color-secondary);
}

.error {
  margin-bottom: 16px;
}

.json-block {
  margin-bottom: 16px;
}

.json-block pre {
  max-height: 280px;
  overflow: auto;
  padding: 12px;
  background: var(--el-fill-color-lighter);
  border-radius: 6px;
}

.el-table {
  width: 100%;
}
</style>
