<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'

import {
  approvalGateApi,
  creativeBriefApi,
  type ApprovalGate,
  type CreativeBrief,
  type CreativeBriefInput,
} from '../../api/client'

const route = useRoute()
const projectId = String(route.params.projectId)

const mode = ref('develop')
const policy = ref<any>(null)
const lifecycle = ref<any>(null)
const usage = ref<any>(null)
const query = ref('')
const results = ref<any[]>([])
const error = ref('')
const brief = ref<CreativeBrief | null>(null)
const briefVersions = ref<CreativeBrief[]>([])
const briefLoading = ref(false)
const briefSaving = ref(false)
const briefApproving = ref(false)
const approvalGate = ref<ApprovalGate | null>(null)
const approvalLoading = ref(false)
const overrideReason = ref('')
const overrideChecks = ref<string[]>([])

const briefForm = reactive<CreativeBriefInput>({
  title: '',
  goal: '',
  audience: '',
  deliverable: '',
  aspect_ratio: '1:1',
  target_width: undefined,
  target_height: undefined,
  subject: '',
  must_have: [],
  avoid: [],
  mood: '',
  required_elements: [],
  constraints: [],
  success_criteria: [],
  deadline: undefined,
})

function csv(value: string) {
  return value
    .split(',')
    .map(item => item.trim())
    .filter(Boolean)
}

function applyBriefToForm(value: CreativeBrief | null) {
  if (!value) return
  briefForm.title = value.title
  briefForm.goal = value.goal
  briefForm.audience = value.audience ?? ''
  briefForm.deliverable = value.deliverable ?? ''
  briefForm.aspect_ratio = value.aspect_ratio ?? '1:1'
  briefForm.target_width = value.target_width
  briefForm.target_height = value.target_height
  briefForm.subject = value.subject ?? ''
  briefForm.must_have = [...value.must_have]
  briefForm.avoid = [...value.avoid]
  briefForm.mood = value.mood ?? ''
  briefForm.required_elements = [...value.required_elements]
  briefForm.constraints = [...value.constraints]
  briefForm.success_criteria = [...value.success_criteria]
  briefForm.deadline = value.deadline
}

async function loadApprovalGate() {
  try {
    approvalGate.value = await approvalGateApi.get(projectId)
    overrideChecks.value = approvalGate.value.warnings
  } catch (e: any) {
    error.value = e.message
  }
}

async function loadBrief() {
  briefLoading.value = true
  try {
    const [current, versions] = await Promise.all([
      creativeBriefApi.current(projectId).catch(() => null),
      creativeBriefApi.versions(projectId),
    ])
    brief.value = current
    briefVersions.value = versions.briefs
    applyBriefToForm(current)
  } catch (e: any) {
    error.value = e.message
  } finally {
    briefLoading.value = false
  }
}

async function saveBrief() {
  if (!briefForm.title.trim() || !briefForm.goal.trim()) {
    error.value = 'Brief title and goal are required'
    return
  }
  briefSaving.value = true
  error.value = ''
  try {
    const input: CreativeBriefInput = {
      ...briefForm,
      audience: String(briefForm.audience ?? '').trim() || undefined,
      deliverable: String(briefForm.deliverable ?? '').trim() || undefined,
      subject: String(briefForm.subject ?? '').trim() || undefined,
      mood: String(briefForm.mood ?? '').trim() || undefined,
      must_have: briefForm.must_have ?? [],
      avoid: briefForm.avoid ?? [],
      required_elements: briefForm.required_elements ?? [],
      constraints: briefForm.constraints ?? [],
      success_criteria: briefForm.success_criteria ?? [],
    }
    brief.value = await creativeBriefApi.create(projectId, input)
    briefVersions.value = (await creativeBriefApi.versions(projectId)).briefs
  } catch (e: any) {
    error.value = e.message
  } finally {
    briefSaving.value = false
  }
}

async function approveBrief() {
  if (!brief.value) return
  briefApproving.value = true
  error.value = ''
  try {
    brief.value = await creativeBriefApi.approve(projectId, brief.value.id)
    briefVersions.value = (await creativeBriefApi.versions(projectId)).briefs
  } catch (e: any) {
    error.value = e.message
  } finally {
    briefApproving.value = false
  }
}

async function load() {
  try {
    policy.value = await api('/projects/' + projectId + '/mode')
    mode.value = policy.value.mode
    lifecycle.value = await api('/projects/' + projectId + '/lifecycle')
    usage.value = await api('/projects/' + projectId + '/usage')
  } catch (e: any) {
    error.value = e.message
  }
  await Promise.all([loadBrief(), loadApprovalGate()])
}

async function api(path: string, init?: RequestInit) {
  const response = await fetch('/api/v1' + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  const data = await response.json()
  if (!response.ok) throw new Error(data?.error?.message || 'Request failed')
  return data
}

async function refreshApprovalGate() {
  approvalGate.value = await approvalGateApi.get(projectId)
}

async function requestReview() {
  approvalLoading.value = true
  error.value = ''
  try {
    approvalGate.value = await approvalGateApi.requestReview(projectId)
  } catch (e: any) {
    error.value = e.message
  } finally {
    approvalLoading.value = false
  }
}

async function approveProject() {
  approvalLoading.value = true
  error.value = ''
  try {
    approvalGate.value = await approvalGateApi.approve(projectId)
  } catch (e: any) {
    error.value = e.message
  } finally {
    approvalLoading.value = false
  }
}

async function approveComposition() {
  approvalLoading.value = true
  error.value = ''
  try {
    approvalGate.value = await approvalGateApi.approveComposition(projectId)
  } catch (e: any) {
    error.value = e.message
  } finally {
    approvalLoading.value = false
  }
}

async function finalizeProject() {
  approvalLoading.value = true
  error.value = ''
  try {
    approvalGate.value = await approvalGateApi.finalize(projectId, {
      override_reason: overrideReason.value.trim() || undefined,
      override_checks: overrideChecks.value,
    })
    mode.value = 'finalize'
  } catch (e: any) {
    error.value = e.message
  } finally {
    approvalLoading.value = false
  }
}

async function createRevision() {
  approvalLoading.value = true
  error.value = ''
  try {
    approvalGate.value = await approvalGateApi.revision(projectId)
    mode.value = 'develop'
  } catch (e: any) {
    error.value = e.message
  } finally {
    approvalLoading.value = false
  }
}

function checkClass(status: string) {
  return 'check-' + status
}
async function setMode(next: string) {
  try {
    policy.value = await api('/projects/' + projectId + '/mode', {
      method: 'PUT',
      body: JSON.stringify({ mode: next }),
    })
    mode.value = next
  } catch (e: any) {
    error.value = e.message
  }
}

async function lifecycleAction(action: string) {
  try {
    lifecycle.value = await api('/projects/' + projectId + '/lifecycle', {
      method: 'POST',
      body: JSON.stringify({ action }),
    })
  } catch (e: any) {
    error.value = e.message
  }
}

async function search() {
  try {
    const data = await api('/search?q=' + encodeURIComponent(query.value))
    results.value = data.results
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline -->
  <main class="workflow">
    <div class="workflow-header">
      <h1>Project workflow</h1>
      <a class="variant-link" :href="'/projects/' + projectId + '/variants'">Variant Board</a>
    </div>
    <p v-if="error" class="error">
      {{ error }}
    </p>

    <section class="panel">
      <div class="section-header">
        <div>
          <h2>Creative Brief</h2>
          <p>Цель проекта и критерии, по которым оценивается результат.</p>
        </div>
        <span v-if="brief" class="status">{{ brief.status }} · v{{ brief.version }}</span>
      </div>

      <div v-loading="briefLoading" class="brief-grid">
        <label>
          Title
          <input v-model="briefForm.title" maxlength="200">
        </label>
        <label>
          Goal
          <textarea v-model="briefForm.goal" rows="3" maxlength="5000" />
        </label>
        <label>
          Audience
          <input v-model="briefForm.audience">
        </label>
        <label>
          Deliverable
          <input v-model="briefForm.deliverable">
        </label>
        <label>
          Aspect ratio
          <input v-model="briefForm.aspect_ratio" placeholder="16:9">
        </label>
        <label>
          Subject
          <input v-model="briefForm.subject">
        </label>
        <label>
          Mood
          <input v-model="briefForm.mood">
        </label>
        <label>
          Target size
          <div class="inline-fields">
            <input v-model.number="briefForm.target_width" type="number" min="1" placeholder="Width">
            <input v-model.number="briefForm.target_height" type="number" min="1" placeholder="Height">
          </div>
        </label>
        <label class="wide">
          Must have
          <input
            :value="briefForm.must_have?.join(', ')"
            placeholder="product fully visible, negative space"
            @input="briefForm.must_have = csv(($event.target as HTMLInputElement).value)"
          >
        </label>
        <label class="wide">
          Avoid
          <input
            :value="briefForm.avoid?.join(', ')"
            placeholder="hands, text, watermark"
            @input="briefForm.avoid = csv(($event.target as HTMLInputElement).value)"
          >
        </label>
        <label class="wide">
          Required elements
          <input
            :value="briefForm.required_elements?.join(', ')"
            @input="briefForm.required_elements = csv(($event.target as HTMLInputElement).value)"
          >
        </label>
        <label class="wide">
          Constraints
          <input
            :value="briefForm.constraints?.join(', ')"
            @input="briefForm.constraints = csv(($event.target as HTMLInputElement).value)"
          >
        </label>
        <label class="wide">
          Success criteria
          <input
            :value="briefForm.success_criteria?.join(', ')"
            @input="briefForm.success_criteria = csv(($event.target as HTMLInputElement).value)"
          >
        </label>
      </div>

      <div class="actions">
        <button type="button" :disabled="briefSaving" @click="saveBrief">
          {{ brief ? 'Save new version' : 'Create brief' }}
        </button>
        <button
          v-if="brief && brief.status !== 'approved'"
          type="button"
          :disabled="briefApproving"
          @click="approveBrief"
        >
          Approve current version
        </button>
      </div>

      <div v-if="briefVersions.length" class="versions">
        <strong>Versions</strong>
        <div v-for="item in briefVersions" :key="item.id" class="version-row">
          <span>v{{ item.version }} — {{ item.title }}</span>
          <span>{{ item.status }}</span>
        </div>
      </div>
    </section>

    <section>
      <h2>Mode</h2>
      <button
        v-for="item in ['explore', 'develop']"
        :key="item"
        type="button"
        :disabled="approvalGate?.workflow_state === 'final'"
        @click="setMode(item)"
      >
        {{ item }}
      </button>
      <span v-if="approvalGate" class="workflow-state">
        {{ approvalGate.workflow_state }}
      </span>
      <p v-if="policy">
        {{ mode }} · max variants: {{ policy.max_variants }} · {{ policy.generation_priority }}
      </p>
    </section>

    <section class="panel approval-panel">
      <div class="section-header">
        <div>
          <h2>Approval / Finalize Gate</h2>
          <p>Каждый критический пункт должен быть закрыт до Approval. Предупреждения требуют явного override.</p>
        </div>
        <span v-if="approvalGate" class="status">{{ approvalGate.workflow_state }}</span>
      </div>

      <div v-if="approvalGate" class="approval-checklist">
        <article v-for="check in approvalGate.checks" :key="check.key" :class="['approval-check', checkClass(check.status)]">
          <div>
            <strong>{{ check.label }}</strong>
            <p>{{ check.message }}</p>
          </div>
          <span class="check-status">{{ check.status }}</span>
        </article>
      </div>

      <div v-if="approvalGate?.warnings.length" class="override-box">
        <strong>Warnings to override before Finalize</strong>
        <label v-for="warning in approvalGate.warnings" :key="warning" class="override-row">
          <input v-model="overrideChecks" type="checkbox" :value="warning">
          {{ warning }}
        </label>
        <label>
          Override reason
          <textarea v-model="overrideReason" rows="3" maxlength="2000" placeholder="Why is this warning acceptable for production?" />
        </label>
      </div>

      <div class="actions">
        <button v-if="approvalGate?.can_request_review" type="button" :disabled="approvalLoading" @click="requestReview">Request review</button>
        <button v-if="approvalGate?.can_approve" type="button" :disabled="approvalLoading" @click="approveProject">Approve</button>
        <button v-if="approvalGate?.checks.some(check => check.key === 'composition' && check.status === 'blocked')" type="button" :disabled="approvalLoading" @click="approveComposition">Approve composition</button>
        <button v-if="approvalGate?.can_finalize" type="button" :disabled="approvalLoading || (approvalGate.warnings.length > 0 && !overrideReason.trim()) || overrideChecks.length !== approvalGate.warnings.length" @click="finalizeProject">Finalize</button>
        <button v-if="approvalGate?.workflow_state === 'final'" type="button" :disabled="approvalLoading" @click="createRevision">Create new revision</button>
      </div>

      <p v-if="approvalGate?.final_iteration_id" class="final-info">
        Final revision: {{ approvalGate.final_iteration_id }}
        <span v-if="approvalGate.final_asset_id"> · asset {{ approvalGate.final_asset_id }}</span>
      </p>
    </section>
    <section>
      <h2>Lifecycle</h2>
      <p>Status: {{ lifecycle?.status }}</p>
      <button type="button" @click="lifecycleAction('archive')">Archive</button>
      <button type="button" @click="lifecycleAction('delete')">Delete</button>
      <button v-if="lifecycle?.status === 'deleted'" type="button" @click="lifecycleAction('restore')">Restore</button>
    </section>

    <section>
      <h2>Usage</h2>
      <p>Today: {{ usage?.daily_units }} / {{ usage?.daily_limit }}</p>
      <p>Month: {{ usage?.monthly_units }} / {{ usage?.monthly_limit }}</p>
      <p>Project: {{ usage?.project_units }} / {{ usage?.project_limit }}</p>
    </section>

    <section>
      <h2>Search</h2>
      <form @submit.prevent="search">
        <input v-model="query" minlength="2" placeholder="Search creative content">
        <button type="submit">Search</button>
      </form>
      <ul>
        <li v-for="item in results" :key="item.type + item.id">
          {{ item.type }} — {{ item.title }}
        </li>
      </ul>
    </section>
  </main>
</template>

<style scoped>
.workflow-state,
.check-status {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  background: #f2f4f7;
  font-size: 12px;
  font-weight: 600;
}

.approval-checklist {
  display: grid;
  gap: 8px;
  margin-bottom: 16px;
}

.approval-check {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 12px;
  align-items: start;
  padding: 12px;
  border-left: 4px solid #d0d5dd;
  border-radius: 6px;
  background: #fafafa;
}

.approval-check p {
  margin: 4px 0 0;
}

.approval-check.check-passed {
  border-left-color: #12b76a;
}

.approval-check.check-warning {
  border-left-color: #f79009;
}

.approval-check.check-blocked {
  border-left-color: #f04438;
}

.override-box {
  display: grid;
  gap: 8px;
  margin: 16px 0;
  padding: 14px;
  border-radius: 8px;
  background: #fff7ed;
}

.override-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.final-info {
  margin-top: 12px;
  font-size: 13px;
}

.workflow {
  padding: 24px;
  min-height: 100vh;
}

.panel {
  margin-bottom: 24px;
  padding: 20px;
  border: 1px solid #ddd;
  border-radius: 10px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.section-header h2 {
  margin-bottom: 4px;
}

.section-header p {
  margin-top: 0;
}

.status {
  font-weight: 600;
}

.brief-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.brief-grid label {
  display: grid;
  gap: 6px;
  font-weight: 600;
}

.brief-grid .wide {
  grid-column: 1 / -1;
}

.inline-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.brief-grid input,
.brief-grid textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 8px;
  border: 1px solid #bbb;
  border-radius: 6px;
  font: inherit;
}

.actions {
  display: flex;
  gap: 10px;
  margin-top: 16px;
}

.actions button,
.workflow section > button,
.workflow form button {
  padding: 8px 12px;
  cursor: pointer;
}

.versions {
  margin-top: 20px;
  display: grid;
  gap: 6px;
}

.version-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 0;
  border-top: 1px solid #eee;
}

.error {
  color: #b42318;
}

@media (max-width: 800px) {
  .brief-grid {
    grid-template-columns: 1fr;
  }

  .brief-grid .wide {
    grid-column: auto;
  }
}
</style>
