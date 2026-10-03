<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { cardTypesApi, type CardTypeDefinition as ServerCardType, type CardTypeVersion } from '../../api/client'
import { registerCardTypeDefinition, type CardFieldSchema, type CardTypeDefinition } from './cardBatchTypes'

const props = defineProps<{ projectId: string }>()
const types = ref<ServerCardType[]>([])
const selectedId = ref('')
const selectedVersion = ref<CardTypeVersion | null>(null)
const versions = ref<CardTypeVersion[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
const creating = ref(false)

const form = ref({
  key: '',
  name: '',
  description: '',
  fields: '[]',
  productionDefaults: '{}',
  promptRules: '{}',
  defaultRecipeId: '',
  defaultRecipeVersion: '',
  defaultTemplateId: '',
  defaultTemplateVersion: '',
})

const selectedType = computed(() => types.value.find(t => t.id === selectedId.value))

function parseObject(raw: string, label: string): Record<string, unknown> {
  const value = JSON.parse(raw || '{}')
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(label + ' must be a JSON object')
  return value
}

function parseFields(): CardFieldSchema[] {
  const value = JSON.parse(form.value.fields || '[]')
  if (!Array.isArray(value)) throw new Error('Fields must be a JSON array')
  return value as CardFieldSchema[]
}

function registerServerType(type: ServerCardType, version: CardTypeVersion) {
  const production = version.production_defaults || {}
  const rules = version.prompt_rules || {}
  const schema = version.schema || {}
  const fields = Array.isArray(schema.fields) ? schema.fields as CardFieldSchema[] : []
  const definition: CardTypeDefinition = {
    id: type.id,
    key: type.key,
    name: type.name,
    description: type.description,
    version: version.version,
    fields,
    promptInstructions: typeof rules.promptInstructions === 'string' ? rules.promptInstructions : '',
    recipeKeywords: Array.isArray(rules.recipeKeywords) ? rules.recipeKeywords.map(String) : [],
    defaultTemplateId: version.default_template_id || String(production.templateId || 'custom'),
    defaultWidth: Number(production.width || 0) || undefined,
    defaultHeight: Number(production.height || 0) || undefined,
    defaultAspectRatio: typeof production.aspectRatio === 'string' ? production.aspectRatio : undefined,
    defaultNegativePrompt: typeof production.negativePrompt === 'string' ? production.negativePrompt : undefined,
    defaultGenerationParameters: typeof production.generationParameters === 'object' && production.generationParameters ? production.generationParameters as Record<string, unknown> : undefined,
    defaultReferenceIds: Array.isArray(production.referenceIds) ? production.referenceIds.map(String) : undefined,
  }
  registerCardTypeDefinition(definition)
}

async function load() {
  if (!props.projectId) return
  loading.value = true; error.value = ''
  try {
    const response = await cardTypesApi.list(props.projectId)
    types.value = response.card_types
    if (!selectedId.value && types.value.length) selectedId.value = types.value[0].id
    if (selectedId.value) await selectType(selectedId.value)
  } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
  finally { loading.value = false }
}

async function selectType(id: string) {
  selectedId.value = id; error.value = ''
  try {
    const details = await cardTypesApi.get(props.projectId, id)
    selectedVersion.value = details.version
    const versionList = await cardTypesApi.versions(props.projectId, id)
    versions.value = versionList.versions
    form.value = {
      key: details.card_type.key,
      name: details.card_type.name,
      description: details.card_type.description,
      fields: JSON.stringify(details.version.schema?.fields || [], null, 2),
      productionDefaults: JSON.stringify(details.version.production_defaults || {}, null, 2),
      promptRules: JSON.stringify(details.version.prompt_rules || {}, null, 2),
      defaultRecipeId: details.version.default_recipe_id || '',
      defaultRecipeVersion: details.version.default_recipe_version?.toString() || '',
      defaultTemplateId: details.version.default_template_id || '',
      defaultTemplateVersion: details.version.default_template_version?.toString() || '',
    }
    registerServerType(details.card_type, details.version)
  } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
}

function payload() {
  const fields = parseFields()
  const production = parseObject(form.value.productionDefaults, 'Production defaults')
  const rules = parseObject(form.value.promptRules, 'Prompt rules')
  const recipeVersion = form.value.defaultRecipeVersion ? Number(form.value.defaultRecipeVersion) : undefined
  const templateVersion = form.value.defaultTemplateVersion ? Number(form.value.defaultTemplateVersion) : undefined
  if (recipeVersion !== undefined && (!Number.isInteger(recipeVersion) || recipeVersion < 1)) throw new Error('Default recipe version must be a positive integer')
  if (templateVersion !== undefined && (!Number.isInteger(templateVersion) || templateVersion < 1)) throw new Error('Default template version must be a positive integer')
  return {
    key: form.value.key.trim().toLowerCase(),
    name: form.value.name.trim(),
    description: form.value.description,
    schema: { fields },
    production_defaults: production,
    prompt_rules: rules,
    default_recipe_id: form.value.defaultRecipeId.trim() || undefined,
    default_recipe_version: recipeVersion,
    default_template_id: form.value.defaultTemplateId.trim() || undefined,
    default_template_version: templateVersion,
  }
}

async function createType() {
  saving.value = true; error.value = ''; message.value = ''
  try {
    const created = await cardTypesApi.create(props.projectId, payload())
    types.value.push(created.card_type)
    await selectType(created.card_type.id)
    creating.value = false
    message.value = 'Card Type created at v1.'
  } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
  finally { saving.value = false }
}

async function publishVersion() {
  if (!selectedId.value) return
  saving.value = true; error.value = ''; message.value = ''
  try {
    const updated = await cardTypesApi.update(props.projectId, selectedId.value, payload())
    const index = types.value.findIndex(t => t.id === selectedId.value)
    if (index >= 0) types.value[index] = updated.card_type
    await selectType(selectedId.value)
    message.value = 'Published immutable Card Type v' + updated.version.version + '.'
  } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
  finally { saving.value = false }
}

async function cloneType() {
  if (!selectedId.value) return
  saving.value = true; error.value = ''; message.value = ''
  try {
    const cloned = await cardTypesApi.clone(props.projectId, selectedId.value, {
      key: form.value.key.trim().toLowerCase() + '-copy',
      name: form.value.name.trim() + ' Copy',
    })
    types.value.push(cloned.card_type)
    await selectType(cloned.card_type.id)
    message.value = 'Card Type cloned as v1.'
  } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
  finally { saving.value = false }
}

async function archiveType() {
  if (!selectedId.value || !confirm('Archive this Card Type? Existing batches remain pinned to their versions.')) return
  saving.value = true; error.value = ''; message.value = ''
  try {
    await cardTypesApi.archive(props.projectId, selectedId.value)
    const item = types.value.find(t => t.id === selectedId.value)
    if (item) item.archived_at = new Date().toISOString()
    message.value = 'Card Type archived. Existing batches are unchanged.'
  } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
  finally { saving.value = false }
}

onMounted(load)
</script>

<template>
  <section class="card-type-manager panel">
    <div class="manager-head">
      <div>
        <h2>Card Type Manager</h2>
        <p class="muted">Card Type describes the card data model. Versions are immutable; batches can stay pinned to old versions.</p>
      </div>
      <div class="actions">
        <button type="button" @click="creating = !creating">{{ creating ? 'Cancel' : 'New type' }}</button>
        <button type="button" @click="load" :disabled="loading">Refresh</button>
      </div>
    </div>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="message" class="info">{{ message }}</div>

    <div class="manager-grid">
      <aside>
        <strong>Project Card Types</strong>
        <button v-for="type in types" :key="type.id" type="button" class="type-row" :class="{ active: type.id === selectedId }" @click="selectType(type.id)">
          <span>{{ type.name }}</span><small>{{ type.key }} · v{{ type.current_version }}{{ type.archived_at ? ' · archived' : '' }}</small>
        </button>
        <span v-if="!types.length" class="muted">No project types yet.</span>
      </aside>

      <form v-if="creating" @submit.prevent="createType">
        <h3>Create Card Type</h3>
        <label>Key<input v-model="form.key" placeholder="weapon"></label>
        <label>Name<input v-model="form.name" placeholder="Weapon"></label>
        <label>Description<textarea v-model="form.description" rows="2"></textarea></label>
        <label>Fields JSON<textarea v-model="form.fields" rows="8"></textarea></label>
        <label>Production defaults JSON<textarea v-model="form.productionDefaults" rows="6"></textarea></label>
        <label>Prompt rules JSON<textarea v-model="form.promptRules" rows="6"></textarea></label>
        <div class="actions"><button class="primary" type="submit" :disabled="saving">Create v1</button></div>
      </form>

      <form v-else-if="selectedType" @submit.prevent="publishVersion">
        <h3>{{ selectedType.name }} — publish v{{ selectedType.current_version + 1 }}</h3>
        <label>Key<input v-model="form.key" disabled></label>
        <label>Name<input v-model="form.name"></label>
        <label>Description<textarea v-model="form.description" rows="2"></textarea></label>
        <label>Fields JSON<textarea v-model="form.fields" rows="8"></textarea></label>
        <label>Production defaults JSON<textarea v-model="form.productionDefaults" rows="6"></textarea></label>
        <label>Prompt rules JSON<textarea v-model="form.promptRules" rows="6"></textarea></label>
        <div class="actions">
          <button class="primary" type="submit" :disabled="saving || !!selectedType.archived_at">Publish new immutable version</button>
          <button type="button" @click="cloneType" :disabled="saving">Clone</button>
          <button type="button" @click="archiveType" :disabled="saving || !!selectedType.archived_at">Archive</button>
        </div>
        <details v-if="versions.length">
          <summary>Version history ({{ versions.length }})</summary>
          <div v-for="version in versions" :key="version.id" class="version">
            <span>v{{ version.version }}</span><small>{{ new Date(version.created_at).toLocaleString() }}</small>
          </div>
        </details>
      </form>

      <div v-else class="muted">Select a Card Type or create one.</div>
    </div>
  </section>
</template>

<style scoped>
.card-type-manager { margin-bottom:18px; }
.manager-head { display:flex; justify-content:space-between; gap:16px; align-items:flex-start; }
.manager-grid { display:grid; grid-template-columns:minmax(180px,260px) 1fr; gap:18px; margin-top:14px; }
.manager-grid aside { display:grid; gap:6px; align-content:start; }
.type-row { display:flex; flex-direction:column; align-items:flex-start; text-align:left; gap:2px; }
.type-row.active { outline:2px solid #222; }
.manager-grid form { display:grid; gap:10px; }
.manager-grid label { display:flex; flex-direction:column; gap:5px; font-size:13px; }
.actions { display:flex; gap:8px; flex-wrap:wrap; align-items:center; }
.muted { color:#666; }
.error { margin:10px 0; padding:9px; background:#fee; color:#900; border-radius:7px; }
.info { margin:10px 0; padding:9px; background:#eef8ee; color:#174d17; border-radius:7px; }
input,textarea { width:100%; box-sizing:border-box; padding:8px; border:1px solid #ccc; border-radius:7px; font:inherit; }
button { padding:8px 12px; border:1px solid #bbb; border-radius:7px; background:#fff; cursor:pointer; }
button.primary { background:#222; color:#fff; border-color:#222; }
button:disabled { opacity:.5; cursor:not-allowed; }
.version { display:flex; justify-content:space-between; padding:6px; background:#f7f7f7; border-radius:6px; margin-top:5px; }
@media (max-width:800px) { .manager-grid { grid-template-columns:1fr; } }
</style>
