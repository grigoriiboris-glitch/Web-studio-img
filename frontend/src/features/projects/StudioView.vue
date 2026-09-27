<!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline -->
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  generationsApi,
  humanActionsApi,
  iterationsApi,
  projectEventsApi,
  projectsApi,
  promptsApi,
  provenanceApi,
  referencesApi,
  type Generation,
  type HumanAction,
  type Iteration,
  type IterationType,
  type Project,
  type Prompt,
  type Reference,
} from '../../api/client'

const route = useRoute()
const project = ref<Project | null>(null)
const iterations = ref<Iteration[]>([])
const generations = ref<Generation[]>([])
const prompts = ref<Prompt[]>([])
const references = ref<Reference[]>([])
const humanActions = ref<HumanAction[]>([])
const error = ref<string | null>(null)
const loading = ref(true)
const creating = ref(false)
const generating = ref(false)
const savingPrompt = ref(false)
const addingReference = ref(false)
const verifying = ref(false)
const provenanceVerified = ref<boolean | null>(null)
const provenanceMessage = ref('')
let eventAbort: AbortController | undefined

const types: IterationType[] = ['idea', 'sketch', 'generation', 'selection', 'composition', 'prompt', 'manual_edit', 'final']
const promptComponents = ['subject', 'composition', 'camera', 'lighting', 'material', 'texture', 'color', 'atmosphere', 'style', 'depth', 'detail', 'constraints', 'negative_constraints']

const form = ref<{ type: IterationType; title: string; description: string }>({
  type: 'idea',
  title: '',
  description: '',
})
const generationForm = ref({ prompt: '', negative_prompt: '', aspect_ratio: '1:1', seed: undefined as number | undefined })
const promptForm = ref({
  original_text: '',
  ai_suggestions: '',
  final_text: '',
  created_by: 'human' as Prompt['created_by'],
  components: {} as Record<string, string>,
})
const referenceForm = ref({
  source_url: '',
  source_type: 'reference' as Reference['source_type'],
  license: 'unknown',
  license_verified: false,
  user_owned: false,
  sha256: '',
  notes: '',
})

const projectId = () => String(route.params.projectId)

async function loadStudio() {
  const id = projectId()
  const [loadedProject, timeline, loadedPrompts, loadedReferences, loadedActions] = await Promise.all([
    projectsApi.get(id),
    iterationsApi.list(id),
    promptsApi.list(id),
    referencesApi.list(id),
    humanActionsApi.list(id),
  ])
  project.value = loadedProject
  iterations.value = timeline.iterations
  prompts.value = loadedPrompts.prompts
  references.value = loadedReferences.references
  humanActions.value = loadedActions.actions
}

async function refreshPrompts() {
  prompts.value = (await promptsApi.list(projectId())).prompts
}

async function refreshReferences() {
  references.value = (await referencesApi.list(projectId())).references
}

async function refreshActions() {
  humanActions.value = (await humanActionsApi.list(projectId())).actions
}

async function createIteration() {
  creating.value = true
  error.value = null
  try {
    const created = await iterationsApi.create(projectId(), {
      type: form.value.type,
      title: form.value.title.trim() || undefined,
      description: form.value.description.trim() || undefined,
    })
    iterations.value.push(created)
    form.value = { type: 'idea', title: '', description: '' }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not create iteration'
  } finally {
    creating.value = false
  }
}

async function createGeneration() {
  generating.value = true
  error.value = null
  try {
    const created = await generationsApi.create(projectId(), {
      prompt: generationForm.value.prompt.trim(),
      negative_prompt: generationForm.value.negative_prompt.trim() || undefined,
      aspect_ratio: generationForm.value.aspect_ratio,
      seed: generationForm.value.seed,
    }, crypto.randomUUID())
    generations.value = [created, ...generations.value.filter(item => item.id !== created.id)]
    generationForm.value = { prompt: '', negative_prompt: '', aspect_ratio: '1:1', seed: undefined }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not queue generation'
  } finally {
    generating.value = false
  }
}

async function cancelGeneration(item: Generation) {
  error.value = null
  try {
    const updated = await generationsApi.cancel(projectId(), item.id)
    const index = generations.value.findIndex(candidate => candidate.id === item.id)
    if (index >= 0) generations.value[index] = updated
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not cancel generation'
  }
}

async function savePrompt() {
  savingPrompt.value = true
  error.value = null
  try {
    const suggestions = promptForm.value.ai_suggestions
      .split('\n')
      .map(value => value.trim())
      .filter(Boolean)
    const created = await promptsApi.create(projectId(), {
      original_text: promptForm.value.original_text.trim(),
      ai_suggestions: suggestions,
      final_text: promptForm.value.final_text.trim() || undefined,
      created_by: promptForm.value.created_by,
      components: { ...promptForm.value.components },
    })
    prompts.value = [created, ...prompts.value]
    promptForm.value = {
      original_text: '',
      ai_suggestions: '',
      final_text: '',
      created_by: 'human',
      components: {},
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not save prompt version'
  } finally {
    savingPrompt.value = false
  }
}

async function approvePrompt(prompt: Prompt) {
  if (!prompt.final_text?.trim()) {
    error.value = 'Add approved final text before approval'
    return
  }
  try {
    const created = await promptsApi.approve(projectId(), prompt.id, {
      final_text: prompt.final_text,
      components: prompt.components,
    })
    prompts.value = [created, ...prompts.value]
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not approve prompt'
  }
}

async function addReference() {
  addingReference.value = true
  error.value = null
  try {
    const created = await referencesApi.create(projectId(), {
      source_url: referenceForm.value.source_url.trim() || undefined,
      source_type: referenceForm.value.source_type,
      license: referenceForm.value.license.trim(),
      license_verified: referenceForm.value.license_verified,
      user_owned: referenceForm.value.user_owned,
      sha256: referenceForm.value.sha256.trim() || undefined,
      notes: referenceForm.value.notes.trim() || undefined,
    })
    references.value = [created, ...references.value]
    referenceForm.value = {
      source_url: '',
      source_type: 'reference',
      license: 'unknown',
      license_verified: false,
      user_owned: false,
      sha256: '',
      notes: '',
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not add reference'
  } finally {
    addingReference.value = false
  }
}

async function removeReference(reference: Reference) {
  try {
    await referencesApi.remove(projectId(), reference.id)
    references.value = references.value.filter(item => item.id !== reference.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not remove reference'
  }
}

async function verifyProvenance() {
  verifying.value = true
  try {
    const result = await provenanceApi.verify(projectId())
    provenanceVerified.value = result.verified
    provenanceMessage.value = result.verified
      ? 'Hash chain is valid across ' + result.event_count + ' events.'
      : (result.reason ?? 'Hash chain verification failed.')
  } catch (err) {
    provenanceVerified.value = false
    provenanceMessage.value = err instanceof Error ? err.message : 'Could not verify provenance'
  } finally {
    verifying.value = false
  }
}

async function handleProjectEvent(event: {
  entity_type: string
  entity_id: string
  event_type: string
}) {
  if (event.entity_type === 'generation') {
    try {
      const updated = await generationsApi.get(projectId(), event.entity_id)
      const index = generations.value.findIndex(item => item.id === updated.id)
      if (index >= 0) generations.value[index] = updated
      else generations.value = [updated, ...generations.value]
    } catch {
      // Keep the local state; the next project event can refresh it.
    }
  }
  if (event.entity_type === 'prompt') await refreshPrompts()
  if (event.entity_type === 'reference') await refreshReferences()
  if (event.entity_type === 'human_action') await refreshActions()
  if (event.entity_type === 'iteration') {
    try {
      iterations.value = (await iterationsApi.list(projectId())).iterations
    } catch {
      // Keep the last known timeline.
    }
  }
}

async function startProjectEvents() {
  eventAbort?.abort()
  eventAbort = new AbortController()
  try {
    await projectEventsApi.stream(projectId(), handleProjectEvent, eventAbort.signal)
  } catch (err) {
    if (!eventAbort.signal.aborted) {
      error.value = err instanceof Error ? err.message : 'Project event stream disconnected'
    }
  }
}

async function restoreIteration(iteration: Iteration) {
  error.value = null
  try {
    const restored = await iterationsApi.restore(projectId(), iteration.id)
    iterations.value.push(restored)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not restore iteration'
  }
}

onMounted(async () => {
  try {
    await loadStudio()
    void startProjectEvents()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load studio'
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  eventAbort?.abort()
})
</script>

<template>
  <el-container class="studio">
    <el-header class="header">
      <RouterLink to="/projects"><el-button link>← Projects</el-button></RouterLink>
      <strong>{{ project?.name ?? 'Studio' }}</strong>
      <el-tag v-if="project">{{ project.status }}</el-tag>
    </el-header>

    <el-main>
      <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = null" />
      <el-skeleton v-if="loading" :rows="8" animated />

      <template v-else>
        <el-card class="create-card">
          <template #header>Generate image</template>
          <el-form label-position="top" @submit.prevent="createGeneration">
            <el-form-item label="Prompt">
              <el-input v-model="generationForm.prompt" type="textarea" maxlength="20000" show-word-limit />
            </el-form-item>
            <el-form-item label="Negative prompt">
              <el-input v-model="generationForm.negative_prompt" type="textarea" maxlength="10000" />
            </el-form-item>
            <el-form-item label="Aspect ratio">
              <el-select v-model="generationForm.aspect_ratio">
                <el-option label="1:1" value="1:1" />
                <el-option label="16:9" value="16:9" />
                <el-option label="9:16" value="9:16" />
                <el-option label="4:3" value="4:3" />
              </el-select>
            </el-form-item>
            <el-form-item label="Seed (optional)">
              <el-input-number v-model="generationForm.seed" :min="0" />
            </el-form-item>
            <el-button type="primary" :loading="generating" :disabled="!generationForm.prompt.trim()" @click="createGeneration">Generate</el-button>
          </el-form>
        </el-card>

        <el-card class="create-card">
          <template #header>Prompt Studio</template>
          <el-form label-position="top" @submit.prevent="savePrompt">
            <el-form-item label="Original input">
              <el-input v-model="promptForm.original_text" type="textarea" maxlength="20000" show-word-limit />
            </el-form-item>
            <el-form-item label="AI suggestions (one per line)">
              <el-input v-model="promptForm.ai_suggestions" type="textarea" maxlength="20000" />
            </el-form-item>
            <el-form-item label="Approved final text">
              <el-input v-model="promptForm.final_text" type="textarea" maxlength="20000" />
            </el-form-item>
            <el-form-item label="Structured prompt components">
              <el-row :gutter="12" class="component-grid">
                <el-col v-for="component in promptComponents" :key="component" :span="8">
                  <el-input v-model="promptForm.components[component]" :placeholder="component" :aria-label="component" />
                </el-col>
              </el-row>
            </el-form-item>
            <el-form-item label="Origin">
              <el-select v-model="promptForm.created_by">
                <el-option label="Human" value="human" />
                <el-option label="AI" value="ai" />
                <el-option label="Mixed" value="mixed" />
              </el-select>
            </el-form-item>
            <el-button type="primary" :loading="savingPrompt" :disabled="!promptForm.original_text.trim()" @click="savePrompt">Save new version</el-button>
          </el-form>
        </el-card>

        <el-card v-if="prompts.length" class="timeline-card">
          <template #header>Prompt versions</template>
          <el-timeline>
            <el-timeline-item v-for="prompt in prompts" :key="prompt.id" :timestamp="'v' + prompt.version + ' · ' + new Date(prompt.created_at).toLocaleString()" placement="top">
              <div class="prompt-version">
                <el-tag>{{ prompt.created_by }}</el-tag>
                <strong>Original</strong>
                <p>{{ prompt.original_text }}</p>
                <div v-if="prompt.ai_suggestions?.length">
                  <strong>AI suggestions</strong>
                  <ul><li v-for="suggestion in prompt.ai_suggestions" :key="suggestion">{{ suggestion }}</li></ul>
                </div>
                <div v-if="prompt.final_text">
                  <strong>Approved text</strong>
                  <p>{{ prompt.final_text }}</p>
                  <el-button size="small" type="success" @click="approvePrompt(prompt)">Approve as new version</el-button>
                </div>
                <div v-if="prompt.components">
                  <strong>DSL</strong>
                  <div class="chips">
                    <el-tag v-for="(value, key) in prompt.components" :key="key" size="small">{{ key }}: {{ value }}</el-tag>
                  </div>
                </div>
              </div>
            </el-timeline-item>
          </el-timeline>
        </el-card>

        <el-card class="create-card">
          <template #header>References</template>
          <el-form label-position="top" @submit.prevent="addReference">
            <el-form-item label="Source URL">
              <el-input v-model="referenceForm.source_url" placeholder="https://..." />
            </el-form-item>
            <el-form-item label="Source type">
              <el-select v-model="referenceForm.source_type">
                <el-option label="Inspiration" value="inspiration" />
                <el-option label="Reference" value="reference" />
                <el-option label="Direct source" value="direct_source" />
                <el-option label="User created" value="user_created" />
                <el-option label="Public domain" value="public_domain" />
                <el-option label="Unknown" value="unknown" />
              </el-select>
            </el-form-item>
            <el-form-item label="License">
              <el-input v-model="referenceForm.license" maxlength="500" />
            </el-form-item>
            <el-form-item label="SHA-256">
              <el-input v-model="referenceForm.sha256" maxlength="64" />
            </el-form-item>
            <el-form-item label="Notes">
              <el-input v-model="referenceForm.notes" type="textarea" maxlength="10000" />
            </el-form-item>
            <el-checkbox v-model="referenceForm.license_verified">License verified</el-checkbox>
            <el-checkbox v-model="referenceForm.user_owned">User owned</el-checkbox>
            <div class="form-actions">
              <el-button type="primary" :loading="addingReference" @click="addReference">Add reference</el-button>
            </div>
          </el-form>
        </el-card>

        <el-card v-if="references.length" class="timeline-card">
          <template #header>Reference library</template>
          <el-table :data="references" stripe>
            <el-table-column prop="source_type" label="Type" width="150" />
            <el-table-column prop="license" label="License" width="180" />
            <el-table-column prop="source_url" label="Source">
              <template #default="scope">
                <a v-if="scope.row.source_url" :href="scope.row.source_url" target="_blank" rel="noreferrer">{{ scope.row.source_url }}</a>
                <span v-else>Asset-backed reference</span>
              </template>
            </el-table-column>
            <el-table-column label="Verified" width="110">
              <template #default="scope">{{ scope.row.license_verified ? 'Yes' : 'No' }}</template>
            </el-table-column>
            <el-table-column width="100">
              <template #default="scope"><el-button link type="danger" @click="removeReference(scope.row)">Remove</el-button></template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card v-if="generations.length" class="timeline-card">
          <template #header>Generation Queue</template>
          <el-timeline>
            <el-timeline-item v-for="item in generations" :key="item.id" :timestamp="new Date(item.created_at).toLocaleString()" placement="top">
              <div class="generation">
                <div class="iteration-head">
                  <el-tag :type="item.status === 'succeeded' ? 'success' : item.status === 'failed' ? 'danger' : 'warning'">{{ item.status }}</el-tag>
                  <strong>{{ item.prompt }}</strong>
                  <el-button v-if="item.status === 'queued' || item.status === 'running'" size="small" @click="cancelGeneration(item)">Cancel</el-button>
                </div>
                <small v-if="item.model_version">Model version: {{ item.model_version }}</small>
                <el-alert v-if="item.error_message" :title="item.error_message" type="error" :closable="false" />
              </div>
            </el-timeline-item>
          </el-timeline>
        </el-card>

        <el-card class="timeline-card">
          <template #header>Human Contribution Map</template>
          <el-empty v-if="humanActions.length === 0" description="No explicit human actions recorded yet." />
          <el-timeline v-else>
            <el-timeline-item v-for="action in humanActions" :key="action.id" :timestamp="new Date(action.created_at).toLocaleString()" placement="top">
              <el-tag>{{ action.action_type }}</el-tag>
              <pre class="action-payload">{{ JSON.stringify(action.payload ?? {}, null, 2) }}</pre>
            </el-timeline-item>
          </el-timeline>
        </el-card>

        <el-card class="timeline-card">
          <template #header>Provenance integrity</template>
          <el-space wrap>
            <el-button :loading="verifying" @click="verifyProvenance">Verify hash chain</el-button>
            <el-tag v-if="provenanceVerified === true" type="success">Verified</el-tag>
            <el-tag v-else-if="provenanceVerified === false" type="danger">Failed</el-tag>
            <span v-if="provenanceMessage">{{ provenanceMessage }}</span>
          </el-space>
        </el-card>

        <el-card class="create-card">
          <template #header>Create iteration</template>
          <el-form label-position="top" @submit.prevent="createIteration">
            <el-form-item label="Type">
              <el-select v-model="form.type">
                <el-option v-for="type in types" :key="type" :label="type" :value="type" />
              </el-select>
            </el-form-item>
            <el-form-item label="Title">
              <el-input v-model="form.title" maxlength="200" show-word-limit />
            </el-form-item>
            <el-form-item label="Description">
              <el-input v-model="form.description" type="textarea" maxlength="5000" show-word-limit />
            </el-form-item>
            <el-button type="primary" :loading="creating" @click="createIteration">Create</el-button>
          </el-form>
        </el-card>

        <el-card class="timeline-card">
          <template #header>Creative Timeline</template>
          <el-empty v-if="iterations.length === 0" description="No iterations yet." />
          <el-timeline v-else>
            <el-timeline-item v-for="item in iterations" :key="item.id" :timestamp="new Date(item.created_at).toLocaleString()" placement="top">
              <div class="iteration">
                <div class="iteration-head">
                  <el-tag>{{ item.type }}</el-tag>
                  <strong>{{ item.title || 'Untitled iteration' }}</strong>
                  <el-button size="small" @click="restoreIteration(item)">Restore as new iteration</el-button>
                </div>
                <p v-if="item.description">{{ item.description }}</p>
                <small v-if="item.parent_iteration_id">Parent: {{ item.parent_iteration_id }}</small>
              </div>
            </el-timeline-item>
          </el-timeline>
        </el-card>
      </template>
    </el-main>
  </el-container>
</template>

<style scoped>
.studio { min-height: 100vh; }
.header { display: flex; align-items: center; justify-content: space-between; }
.create-card, .timeline-card { margin-bottom: 16px; }
.iteration, .prompt-version, .generation { display: grid; gap: 8px; }
.iteration-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.iteration p, .prompt-version p { margin: 0; }
.iteration small, .generation small { color: var(--el-text-color-secondary); }
.component-grid { width: 100%; row-gap: 12px; }
.chips { display: flex; flex-wrap: wrap; gap: 6px; }
.form-actions { margin-top: 12px; }
.action-payload { white-space: pre-wrap; word-break: break-word; margin: 8px 0 0; font: inherit; }
</style>
