export interface ApiError {
  error: { code: string; message: string; request_id?: string }
}

export type PrivacyMode = 'local_only' | 'provider_allowed' | 'project_default'

export interface Project {
  id: string
  user_id: string
  name: string
  description?: string
  status: 'active' | 'archived' | 'deleted'
  privacy_mode: PrivacyMode
  created_at: string
  updated_at: string
  card_type_id?: string
  card_type_version?: number
}

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '/api/v1').replace(/\/$/, '')

export async function apiRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const token = localStorage.getItem('web-studio-access-token')
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init.headers,
    },
  })

  if (!response.ok) {
    let payload: ApiError | undefined
    try {
      payload = await response.json() as ApiError
    } catch {
      // non-JSON error
    }
    throw new Error(payload?.error.message ?? `Request failed with status ${response.status}`)
  }

  if (response.status === 204) return undefined as T
  return await response.json() as T
}


async function buildIdempotencyKey(scope: string, input: unknown): Promise<string> {
  const bytes = new TextEncoder().encode(scope + ":" + JSON.stringify(input))
  const digest = await crypto.subtle.digest("SHA-256", bytes)
  const hex = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, "0")).join("")
  return scope + "-" + hex
}

export const healthApi = {
  get: () => apiRequest<{ status: string }>('/health'),
}

export const projectsApi = {
  list: () => apiRequest<{ projects: Project[] }>('/projects'),
  get: (id: string) => apiRequest<Project>(`/projects/${id}`),
  create: (input: { name: string; description?: string }) =>
    apiRequest<Project>('/projects', { method: 'POST', body: JSON.stringify(input) }),
  update: (id: string, input: { name: string; description?: string; status: Project['status']; privacy_mode?: PrivacyMode }) =>
    apiRequest<Project>(`/projects/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
  archive: (id: string) =>
    apiRequest<Project>(`/projects/${id}`, { method: 'DELETE' }),
}

export interface CreativeBrief {
  id: string
  project_id: string
  user_id: string
  version: number
  title: string
  goal: string
  audience?: string
  deliverable?: string
  aspect_ratio?: string
  target_width?: number
  target_height?: number
  subject?: string
  must_have: string[]
  avoid: string[]
  mood?: string
  required_elements: string[]
  constraints: string[]
  success_criteria: string[]
  deadline?: string
  status: 'draft' | 'approved'
  created_at: string
  updated_at: string
}

export type CreativeBriefInput = Omit<CreativeBrief, 'id' | 'project_id' | 'user_id' | 'version' | 'status' | 'created_at' | 'updated_at'>

export type ApprovalCheckStatus = 'passed' | 'warning' | 'blocked'

export interface ApprovalCheck {
  key: string
  label: string
  status: ApprovalCheckStatus
  critical: boolean
  message: string
}

export interface ApprovalGate {
  workflow_state: 'draft' | 'explore' | 'develop' | 'ready_for_review' | 'approved' | 'final'
  final_iteration_id?: string
  final_asset_id?: string
  checks: ApprovalCheck[]
  warnings: string[]
  blocked: string[]
  can_request_review: boolean
  can_approve: boolean
  can_finalize: boolean
  override_required: boolean
  approved_at?: string
  finalized_at?: string
}

export const approvalGateApi = {
  get: (projectId: string) =>
    apiRequest<ApprovalGate>(`/projects/${projectId}/approval-gate`),
  requestReview: (projectId: string) =>
    apiRequest<ApprovalGate>(`/projects/${projectId}/approval-gate/review`, { method: 'POST' }),
  approve: (projectId: string) =>
    apiRequest<ApprovalGate>(`/projects/${projectId}/approval-gate/approve`, { method: 'POST' }),
  approveComposition: (projectId: string) =>
    apiRequest<ApprovalGate>(`/projects/${projectId}/approval-gate/composition/approve`, { method: 'POST' }),
  finalize: (projectId: string, input: { override_reason?: string; override_checks?: string[] }) =>
    apiRequest<ApprovalGate>(`/projects/${projectId}/approval-gate/finalize`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  revision: (projectId: string) =>
    apiRequest<ApprovalGate>(`/projects/${projectId}/approval-gate/revision`, { method: 'POST' }),
}


export interface CardTypeVersion {
  id: string
  card_type_id: string
  version: number
  schema: Record<string, unknown>
  production_defaults: Record<string, unknown>
  default_recipe_id?: string
  default_recipe_version?: number
  default_template_id?: string
  default_template_version?: number
  prompt_rules: Record<string, unknown>
  reject_reason_profile_id?: string
  created_at: string
}

export interface CardTypeDefinition {
  id: string
  project_id: string
  key: string
  name: string
  description: string
  current_version: number
  archived_at?: string
  created_at: string
  updated_at: string
}

export interface CardTypeDetails {
  card_type: CardTypeDefinition
  version: CardTypeVersion
}

export const cardTypesApi = {
  list: (projectId: string) => apiRequest<{ card_types: CardTypeDefinition[] }>(`/projects/${projectId}/card-types`),
  get: (projectId: string, id: string) => apiRequest<CardTypeDetails>(`/projects/${projectId}/card-types/${id}`),
  versions: (projectId: string, id: string) => apiRequest<{ versions: CardTypeVersion[] }>(`/projects/${projectId}/card-types/${id}/versions`),
  create: (projectId: string, input: Record<string, unknown>) =>
    apiRequest<CardTypeDetails>(`/projects/${projectId}/card-types`, { method: 'POST', body: JSON.stringify(input) }),
  update: (projectId: string, id: string, input: Record<string, unknown>) =>
    apiRequest<CardTypeDetails>(`/projects/${projectId}/card-types/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
  clone: (projectId: string, id: string, input: { key?: string; name?: string } = {}) =>
    apiRequest<CardTypeDetails>(`/projects/${projectId}/card-types/${id}/clone`, { method: 'POST', body: JSON.stringify(input) }),
  archive: (projectId: string, id: string) =>
    apiRequest<void>(`/projects/${projectId}/card-types/${id}/archive`, { method: 'POST' }),
}

export interface CardBatch {
  id: string
  project_id: string
  user_id: string
  name: string
  source_file: string
  sheet: string
  mapping: Record<string, unknown>
  state: Record<string, unknown>
  version: number
  card_type_id?: string
  card_type_version?: number
  created_at: string
  updated_at: string
}

export const cardBatchApi = {
  list: (projectId: string) => apiRequest<{ batches: CardBatch[] }>(`/projects/${projectId}/card-batches`),
  get: (projectId: string, batchId: string) => apiRequest<CardBatch>(`/projects/${projectId}/card-batches/${batchId}`),
  create: (projectId: string, input: { name: string; source_file?: string; sheet?: string; mapping?: Record<string, unknown>; state: Record<string, unknown>; card_type_id?: string; card_type_version?: number }) =>
    apiRequest<CardBatch>(`/projects/${projectId}/card-batches`, { method: 'POST', body: JSON.stringify(input) }),
  update: (projectId: string, batchId: string, input: { version: number; source_file?: string; sheet?: string; mapping?: Record<string, unknown>; state?: Record<string, unknown>; card_type_id?: string; card_type_version?: number }) =>
    apiRequest<CardBatch>(`/projects/${projectId}/card-batches/${batchId}`, { method: 'PATCH', body: JSON.stringify(input) }),
  remove: (projectId: string, batchId: string) => apiRequest<void>(`/projects/${projectId}/card-batches/${batchId}`, { method: 'DELETE' }),
}

export interface RecipeParameter {
  name: string
  type: 'string' | 'number' | 'integer' | 'boolean' | 'image' | 'mask'
  required: boolean
  path: string
  default?: unknown
  description?: string
}

export interface RecipeVersion {
  id: string
  recipe_id: string
  version: number
  workflow: Record<string, unknown>
  input_mappings: Record<string, string>
  exposed_parameters: RecipeParameter[]
  default_parameters: Record<string, unknown>
  workflow_hash: string
  created_by: string
  created_at: string
}

export interface Recipe {
  id: string
  user_id: string
  project_id?: string
  name: string
  description?: string
  provider: string
  model: string
  model_version?: string
  scope: 'project' | 'global'
  tags: string[]
  preview_asset_id?: string
  published: boolean
  current_version: number
  created_at: string
  updated_at: string
  version?: RecipeVersion
}

export type RecipeVersionInput = {
  workflow: Record<string, unknown>
  input_mappings: Record<string, string>
  exposed_parameters: RecipeParameter[]
  default_parameters: Record<string, unknown>
}

export const recipesApi = {
  list: (projectId: string) =>
    apiRequest<{ recipes: Recipe[] }>(`/projects/${projectId}/recipes`),
  get: (projectId: string, recipeId: string) =>
    apiRequest<Recipe>(`/projects/${projectId}/recipes/${recipeId}`),
  create: (
    projectId: string,
    input: Omit<Recipe, 'id' | 'user_id' | 'created_at' | 'updated_at' | 'current_version' | 'published' | 'version'> & { version: RecipeVersionInput },
  ) =>
    apiRequest<Recipe>(`/projects/${projectId}/recipes`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  addVersion: (projectId: string, recipeId: string, input: RecipeVersionInput) =>
    apiRequest<Recipe>(`/projects/${projectId}/recipes/${recipeId}/versions`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  publish: (projectId: string, recipeId: string) =>
    apiRequest<Recipe>(`/projects/${projectId}/recipes/${recipeId}/publish`, { method: 'POST' }),
  unpublish: (projectId: string, recipeId: string) =>
    apiRequest<Recipe>(`/projects/${projectId}/recipes/${recipeId}/unpublish`, { method: 'POST' }),
  duplicate: (projectId: string, recipeId: string) =>
    apiRequest<Recipe>(`/projects/${projectId}/recipes/${recipeId}/duplicate`, { method: 'POST' }),
  compatibility: (projectId: string, recipeId: string) =>
    apiRequest<{ compatible: boolean; issues: string[]; provider: string; model: string; version: number }>(
      `/projects/${projectId}/recipes/${recipeId}/compatibility`,
      { method: 'POST' },
    ),
  fromGeneration: (projectId: string, generationId: string) =>
    apiRequest<Recipe>(`/projects/${projectId}/recipes/from-generation/${generationId}`, { method: 'POST' }),
}

export const creativeBriefApi = {
  current: (projectId: string) =>
    apiRequest<CreativeBrief>('/projects/' + projectId + '/brief'),
  versions: (projectId: string) =>
    apiRequest<{ briefs: CreativeBrief[] }>('/projects/' + projectId + '/brief/versions'),
  approved: (projectId: string) =>
    apiRequest<CreativeBrief>('/projects/' + projectId + '/brief/approved'),
  create: (projectId: string, input: CreativeBriefInput) =>
    apiRequest<CreativeBrief>('/projects/' + projectId + '/brief', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  approve: (projectId: string, briefId: string) =>
    apiRequest<CreativeBrief>('/projects/' + projectId + '/brief/' + briefId + '/approve', {
      method: 'POST',
    }),
}

export type IterationType =
  | 'idea'
  | 'sketch'
  | 'generation'
  | 'selection'
  | 'composition'
  | 'prompt'
  | 'manual_edit'
  | 'final'

export interface Iteration {
  id: string
  project_id: string
  branch_id: string
  parent_iteration_id?: string
  type: IterationType
  title?: string
  description?: string
  decisions?: Record<string, unknown>
  merge_sources?: string[]
  created_at: string
}

export interface Branch {
  id: string
  project_id: string
  name: string
  parent_iteration_id?: string
  status: 'active' | 'archived'
  created_by: string
  created_at: string
  updated_at: string
}

export interface BranchDecision {
  source_branch_id: string
  value: unknown
}

export interface BranchDifference {
  dimension: string
  source?: unknown
  target?: unknown
  different: boolean
}

export interface BranchMergeInput {
  source_branch_ids: string[]
  decisions: Record<string, BranchDecision>
}

export const branchesApi = {
  list: (projectId: string) =>
    apiRequest<{ branches: Branch[] }>(`/projects/${projectId}/branches`),
  create: (projectId: string, input: { name: string; parent_iteration_id?: string }) =>
    apiRequest<Branch>(`/projects/${projectId}/branches`, { method: 'POST', body: JSON.stringify(input) }),
  update: (projectId: string, branchId: string, input: { name: string; status: Branch['status'] }) =>
    apiRequest<Branch>(`/projects/${projectId}/branches/${branchId}`, { method: 'PATCH', body: JSON.stringify(input) }),
  compare: (projectId: string, sourceBranchId: string, targetBranchId: string) =>
    apiRequest<{ differences: BranchDifference[] }>(`/projects/${projectId}/branches/compare?source_branch_id=${sourceBranchId}&target_branch_id=${targetBranchId}`),
  merge: (projectId: string, targetBranchId: string, input: BranchMergeInput) =>
    apiRequest<Iteration>(`/projects/${projectId}/branches/${targetBranchId}/merge`, { method: 'POST', body: JSON.stringify(input) }),
}

export const iterationsApi = {
  list: (projectId: string) =>
    apiRequest<{ iterations: Iteration[] }>(`/projects/${projectId}/iterations`),
  create: (
    projectId: string,
    input: {
      parent_iteration_id?: string
      branch_id?: string
      type: IterationType
      title?: string
      description?: string
      decisions?: Record<string, unknown>
    },
  ) =>
    apiRequest<Iteration>(`/projects/${projectId}/iterations`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  get: (projectId: string, iterationId: string) =>
    apiRequest<Iteration>(`/projects/${projectId}/iterations/${iterationId}`),
  restore: (projectId: string, iterationId: string) =>
    apiRequest<Iteration>(`/projects/${projectId}/iterations/${iterationId}/restore`, {
      method: 'POST',
    }),
}


export type GenerationStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'

export interface Generation {
  id: string
  project_id: string
  iteration_id?: string
  user_id: string
  provider: string
  model: string
  model_version?: string
  prompt: string
  negative_prompt?: string
  seed?: number
  aspect_ratio?: string
  parameters?: Record<string, unknown>
  reference_ids?: string[]
  resolved_reference_influence?: Record<string, unknown>
  status: GenerationStatus
  provider_job_id?: string
  error_code?: string
  error_message?: string
  cost?: number
  created_at: string
  started_at?: string
  completed_at?: string
  recipe_id?: string
  recipe_version?: number
  resolved_workflow_hash?: string
  final_parameters?: Record<string, unknown>
}

export const generationsApi = {
  create: (
    projectId: string,
    input: {
      iteration_id?: string
      prompt: string
      negative_prompt?: string
      seed?: number
      aspect_ratio?: string
      parameters?: Record<string, unknown>
      reference_ids?: string[]
      reference_influence?: Record<string, unknown>
      recipe_id?: string
      recipe_version?: number
    },
    idempotencyKey: string,
  ) =>
    apiRequest<Generation>(`/projects/${projectId}/generations`, {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(input),
    }),
  get: (projectId: string, generationId: string) =>
    apiRequest<Generation>(`/projects/${projectId}/generations/${generationId}`),
  cancel: (projectId: string, generationId: string) =>
    apiRequest<Generation>(`/projects/${projectId}/generations/${generationId}/cancel`, {
      method: 'POST',
    }),
}


export interface VariantSource {
  generation_id: string
  iteration_id?: string
  asset_id?: string
  status: GenerationStatus
  prompt: string
  provider: string
  model: string
  created_at: string
  width?: number
  height?: number
  context?: {
    seed?: number | null
    negative_prompt?: string
    reference_ids?: string[]
    recipe_id?: string | null
    recipe_version?: number | null
    resolved_workflow_hash?: string | null
    model_version?: string | null
    final_parameters?: Record<string, unknown>
    card_type?: string
    card_template_id?: string
  }
}

export interface Variant {
  id: string
  variant_set_id: string
  generation_id: string
  asset_id?: string
  ordinal: number
  decision: 'candidate' | 'kept' | 'rejected' | 'selected'
  favorite: boolean
  compare_selected: boolean
  reject_reason: string[]
  reject_comment?: string
  reject_severity?: 'low' | 'medium' | 'high'
  reject_reason_skipped: boolean
  created_at: string
  updated_at: string
  source?: VariantSource
}

export interface VariantSet {
  id: string
  project_id: string
  user_id: string
  name: string
  metadata: Record<string, unknown>
  created_at: string
}

export const variantBoardApi = {
  sources: (projectId: string) =>
    apiRequest<{ sources: VariantSource[] }>(`/projects/${projectId}/variant-sources`),
  listSets: (projectId: string) =>
    apiRequest<{ variant_sets: VariantSet[] }>(`/projects/${projectId}/variant-sets`),
  createSet: async (
    projectId: string,
    input: { name: string; generation_ids: string[] },
    idempotencyKey?: string,
  ) => {
    const key = idempotencyKey ?? await buildIdempotencyKey("variant-set-create", input)
    return apiRequest<{ variant_set: VariantSet; variants: Variant[] }>(
      `/projects/${projectId}/variant-sets`,
      { method: 'POST', body: JSON.stringify(input), headers: { "Idempotency-Key": key } },
    )
  },
  getSet: (projectId: string, setId: string) =>
    apiRequest<{ variant_set: VariantSet; variants: Variant[] }>(
      `/projects/${projectId}/variant-sets/${setId}`,
    ),
  compare: (projectId: string, setId: string, variantIds: string[]) => {
    const query = variantIds.map(id => 'variant_id=' + encodeURIComponent(id)).join('&')
    return apiRequest<{ variants: Variant[] }>(
      `/projects/${projectId}/variant-sets/${setId}/compare?${query}`,
    )
  },
  regenerate: async (projectId: string, setId: string, variantId: string, idempotencyKey?: string) => {
    const key = idempotencyKey ?? await buildIdempotencyKey("variant-regenerate-" + setId + "-" + variantId, { variantId })
    return apiRequest<{ generation: Generation; variant: Variant }>(
      `/projects/${projectId}/variant-sets/${setId}/variants/${variantId}/regenerate`,
      { method: 'POST', headers: { "Idempotency-Key": key } },
    )
  },
  patchVariant: async (
    projectId: string,
    setId: string,
    variantId: string,
    input: {
      decision?: Variant['decision']
      favorite?: boolean
      compare_selected?: boolean
      reject_reason?: string[]
      reject_comment?: string
      reject_severity?: '' | 'low' | 'medium' | 'high'
      skip_reason?: boolean
    },
    idempotencyKey?: string,
  ) => {
    const key = idempotencyKey ?? await buildIdempotencyKey("variant-patch-" + setId + "-" + variantId, input)
    return apiRequest<Variant>(
      `/projects/${projectId}/variant-sets/${setId}/variants/${variantId}`,
      { method: 'PATCH', body: JSON.stringify(input), headers: { "Idempotency-Key": key } },
    )
  },
  rejectionSummary: (projectId: string) =>
    apiRequest<RejectReasonSummary>(`/projects/${projectId}/variant-rejection-summary`),
  rejectionTimeline: (projectId: string) =>
    apiRequest<{ items: RejectTimelineItem[] }>(`/projects/${projectId}/variant-rejection-timeline`),
  createIteration: async (
    projectId: string,
    setId: string,
    variantIds: string[],
    title?: string,
    decisions?: Record<string, unknown>,
    idempotencyKey?: string,
  ) => {
    const input = { variant_ids: variantIds, title, decisions }
    const key = idempotencyKey ?? await buildIdempotencyKey("variant-iteration-" + setId, input)
    return apiRequest<{
      iteration_id: string
      title: string
      description: string
      decisions?: Record<string, unknown>
    }>(
      `/projects/${projectId}/variant-sets/${setId}/iterations`,
      { method: 'POST', body: JSON.stringify(input), headers: { "Idempotency-Key": key } },
    )
  },
}




export interface RejectReasonSummary {
  rejected_count: number
  reasons: Array<{ reason: string; count: number; percent: number }>
}

export interface RejectTimelineItem {
  id: string
  version: number
  action_type: string
  payload?: Record<string, unknown>
  old_state?: Record<string, unknown>
  new_state?: Record<string, unknown>
  created_at: string
}

export type ProjectEvent = {
  id: number
  project_id: string
  user_id: string
  event_type: string
  entity_type: string
  entity_id: string
  payload?: Record<string, unknown>
  created_at: string
}

export interface Prompt {
  id: string
  project_id: string
  iteration_id?: string
  parent_prompt_id?: string
  version: number
  original_text: string
  ai_suggestions?: string[]
  final_text?: string
  components?: Record<string, string>
  created_by: 'human' | 'ai' | 'mixed'
  created_at: string
}

export interface ReferenceInfluence {
  composition: number
  semantic: number
  color: number
  style: number
  material: number
  geometry: number
  mood: number
  warning?: string
}

export interface Reference {
  id: string
  project_id: string
  asset_id?: string
  source_url?: string
  source_type: 'inspiration' | 'reference' | 'direct_source' | 'user_created' | 'public_domain' | 'unknown'
  license: string
  license_verified: boolean
  user_owned: boolean
  sha256?: string
  notes?: string
  influence?: ReferenceInfluence
  created_at: string
  updated_at: string
}

export interface ReferenceUsage {
  id: string
  user_id: string
  project_id: string
  reference_id: string
  iteration_id?: string
  generation_id?: string
  usage_type: string
  role: 'inspiration' | 'composition' | 'subject' | 'color' | 'material' | 'mood'
  influence?: ReferenceInfluence
  created_at: string
}

export interface HumanAction {
  id: string
  project_id: string
  iteration_id?: string
  user_id: string
  version: number
  action_type: string
  payload?: Record<string, unknown>
  old_state?: Record<string, unknown>
  new_state?: Record<string, unknown>
  ai_influence?: Record<string, unknown>
  created_at: string
}

export interface ProvenanceVerification {
  valid: boolean
  events_checked: number
  broken_links: string[]
  verified_at: string
}

// eslint-disable-next-line no-unused-vars
type ProjectEventHandler = (...args: [ProjectEvent]) => void

export const projectEventsApi = {
  stream: async (
    projectId: string,
    onEvent: ProjectEventHandler,
    signal?: AbortSignal,
  ) => {
    let lastEventId = 0
    let backoffMs = 500

    while (!signal?.aborted) {
      let response: Response
      try {
        const token = localStorage.getItem('web-studio-access-token')
        response = await fetch(API_BASE_URL + '/projects/' + projectId + '/events', {
          headers: {
            Accept: 'text/event-stream',
            ...(lastEventId > 0 ? { 'Last-Event-ID': String(lastEventId) } : {}),
            ...(token ? { Authorization: 'Bearer ' + token } : {}),
          },
          signal,
        })
      } catch {
        if (signal?.aborted) return
        await new Promise(resolve => setTimeout(resolve, backoffMs))
        backoffMs = Math.min(backoffMs * 2, 10_000)
        continue
      }

      if (!response.ok) {
        if (response.status >= 400 && response.status < 500) {
          throw new Error('Event stream failed with status ' + response.status)
        }
        await new Promise(resolve => setTimeout(resolve, backoffMs))
        backoffMs = Math.min(backoffMs * 2, 10_000)
        continue
      }
      if (!response.body) throw new Error('Event stream is unavailable')

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let eventId = ''
      let eventName = ''
      let data = ''

      const emit = () => {
        if (!data) return
        try {
          const event = JSON.parse(data) as ProjectEvent
          if (eventName) event.event_type = eventName
          if (event.id > 0) lastEventId = event.id
          else if (eventId) lastEventId = Number(eventId) || lastEventId
          onEvent(event)
        } finally {
          eventId = ''
          eventName = ''
          data = ''
        }
      }

      while (!signal?.aborted) {
        const chunk = await reader.read()
        if (chunk.done) break
        buffer += decoder.decode(chunk.value, { stream: true })
        const frames = buffer.split('\n\n')
        buffer = frames.pop() ?? ''
        for (const frame of frames) {
          for (const line of frame.split('\n')) {
            if (line.startsWith('id:')) eventId = line.slice(3).trim()
            else if (line.startsWith('event:')) eventName = line.slice(6).trim()
            else if (line.startsWith('data:')) data += line.slice(5).trim()
          }
          emit()
        }
      }
      if (signal?.aborted) return

      await new Promise(resolve => setTimeout(resolve, backoffMs))
      backoffMs = Math.min(backoffMs * 2, 10_000)
    }
  },
}

export const promptsApi = {
  list: (projectId: string) => apiRequest<{ prompts: Prompt[] }>('/projects/' + projectId + '/prompts'),
  get: (projectId: string, promptId: string) => apiRequest<Prompt>('/projects/' + projectId + '/prompts/' + promptId),
  create: (projectId: string, input: {
    iteration_id?: string
    parent_prompt_id?: string
    original_text: string
    ai_suggestions?: string[]
    final_text?: string
    components?: Record<string, string>
    created_by?: Prompt['created_by']
  }) => apiRequest<Prompt>('/projects/' + projectId + '/prompts', { method: 'POST', body: JSON.stringify(input) }),
  patch: (projectId: string, promptId: string, input: {
    original_text?: string
    ai_suggestions?: string[]
    final_text?: string
    components?: Record<string, string>
    created_by?: Prompt['created_by']
  }) => apiRequest<Prompt>('/projects/' + projectId + '/prompts/' + promptId, { method: 'PATCH', body: JSON.stringify(input) }),
  approve: (projectId: string, promptId: string, input: {
    final_text: string
    components?: Record<string, string>
  }) => apiRequest<Prompt>('/projects/' + projectId + '/prompts/' + promptId + '/approve', { method: 'POST', body: JSON.stringify(input) }),
}

export const referencesApi = {
  list: (projectId: string) => apiRequest<{ references: Reference[] }>('/projects/' + projectId + '/references'),
  listUsage: (projectId: string, referenceId: string) => apiRequest<{ usage: ReferenceUsage[] }>('/projects/' + projectId + '/references/' + referenceId + '/usage'),
  createUsage: (projectId: string, referenceId: string, input: { iteration_id?: string; generation_id?: string; usage_type: string; role: ReferenceUsage['role']; influence?: ReferenceInfluence }) =>
    apiRequest<ReferenceUsage>('/projects/' + projectId + '/references/' + referenceId + '/usage', { method: 'POST', body: JSON.stringify(input) }),
  create: (projectId: string, input: {
    asset_id?: string
    source_url?: string
    source_type: Reference['source_type']
    license: string
    license_verified?: boolean
    user_owned?: boolean
    sha256?: string
    notes?: string
    influence?: ReferenceInfluence
  }) => apiRequest<Reference>('/projects/' + projectId + '/references', { method: 'POST', body: JSON.stringify(input) }),
  update: (projectId: string, referenceId: string, input: {
    asset_id?: string
    source_url?: string
    source_type: Reference['source_type']
    license: string
    license_verified?: boolean
    user_owned?: boolean
    sha256?: string
    notes?: string
    influence?: ReferenceInfluence
  }) => apiRequest<Reference>('/projects/' + projectId + '/references/' + referenceId, { method: 'PATCH', body: JSON.stringify(input) }),
  remove: (projectId: string, referenceId: string) =>
    apiRequest<void>('/projects/' + projectId + '/references/' + referenceId, { method: 'DELETE' }),
  analyzeInfluence: (projectId: string, referenceId: string, targetAssetId: string) =>
    apiRequest<Reference>('/projects/' + projectId + '/references/' + referenceId + '/influence', { method: 'POST', body: JSON.stringify({ target_asset_id: targetAssetId }) }),
}


export type AssetLibraryAssetType = 'character' | 'object' | 'product' | 'logo' | 'symbol' | 'background' | 'texture' | 'material' | 'mask' | 'image' | 'other'
export type AssetLibraryRightsStatus = 'inherited' | 'verified' | 'unverified' | 'restricted' | 'unknown'
export interface AssetLibraryVersion {
  id: string
  library_item_id: string
  version: number
  source_asset_id?: string
  source_project_id?: string
  mime_type: string
  size: number
  width: number
  height: number
  checksum: string
  rights_snapshot: Record<string, unknown>
  provenance: Record<string, unknown>
  original_url?: string
  preview_url?: string
  thumbnail_url?: string
  created_at: string
}
export interface AssetLibraryItem {
  id: string
  user_id: string
  name: string
  description?: string
  asset_type: AssetLibraryAssetType
  tags: string[]
  status: 'active' | 'archived'
  current_version: number
  created_at: string
  updated_at: string
  current?: AssetLibraryVersion
}
export interface AssetLibraryUsage {
  id: string
  library_version_id: string
  user_id: string
  project_id: string
  project_name?: string
  rights_status: AssetLibraryRightsStatus
  rights_notes?: string
  created_at: string
}
export interface AssetLibrarySource {
  id: string
  project_id: string
  type: string
  mime_type: string
  size: number
  width: number
  height: number
  checksum: string
  preview_url?: string
  created_at: string
}
export const assetLibraryApi = {
  list: (params: { q?: string; type?: AssetLibraryAssetType; tag?: string; status?: 'active' | 'archived' } = {}) => {
    const query = new URLSearchParams()
    if (params.q) query.set('q', params.q)
    if (params.type) query.set('type', params.type)
    if (params.tag) query.set('tag', params.tag)
    if (params.status) query.set('status', params.status)
    const suffix = query.toString() ? '?' + query.toString() : ''
    return apiRequest<{ items: AssetLibraryItem[] }>('/library/assets' + suffix)
  },
  get: (itemId: string) => apiRequest<{ item: AssetLibraryItem; versions: AssetLibraryVersion[]; usage: AssetLibraryUsage[] }>('/library/assets/' + itemId),
  create: (input: { source_asset_id: string; name: string; description?: string; asset_type: AssetLibraryAssetType; tags: string[] }) =>
    apiRequest<AssetLibraryItem>('/library/assets', { method: 'POST', body: JSON.stringify(input) }),
  archive: (itemId: string) => apiRequest<void>('/library/assets/' + itemId, { method: 'DELETE' }),
  versions: (itemId: string) => apiRequest<{ versions: AssetLibraryVersion[] }>('/library/assets/' + itemId + '/versions'),
  createVersion: (itemId: string, sourceAssetId: string) =>
    apiRequest<AssetLibraryVersion>('/library/assets/' + itemId + '/versions', { method: 'POST', body: JSON.stringify({ source_asset_id: sourceAssetId }) }),
  usage: (itemId: string) => apiRequest<{ usage: AssetLibraryUsage[] }>('/library/assets/' + itemId + '/usage'),
  useInProject: (itemId: string, input: { project_id: string; version?: number; rights_status?: AssetLibraryRightsStatus; rights_notes?: string }) =>
    apiRequest<{ usage: AssetLibraryUsage; version: AssetLibraryVersion }>('/library/assets/' + itemId + '/use', { method: 'POST', body: JSON.stringify(input) }),
  sources: (projectId: string) => apiRequest<{ sources: AssetLibrarySource[] }>('/library/sources?project_id=' + encodeURIComponent(projectId)),
}

export const humanActionsApi = {
  list: (projectId: string) => apiRequest<{ actions: HumanAction[] }>('/projects/' + projectId + '/human-actions'),
  create: (projectId: string, input: {
    iteration_id?: string
    action_type: string
    payload?: Record<string, unknown>
    old_state?: Record<string, unknown>
    new_state?: Record<string, unknown>
    ai_influence?: Record<string, unknown>
  }) => apiRequest<HumanAction>('/projects/' + projectId + '/human-actions', { method: 'POST', body: JSON.stringify(input) }),
}

export const provenanceApi = {
  list: (projectId: string) => apiRequest<{ events: ProjectEvent[] }>('/projects/' + projectId + '/provenance'),
  verify: (projectId: string) => apiRequest<ProvenanceVerification>('/projects/' + projectId + '/provenance/verify'),
}


export interface Asset {
  id: string
  project_id: string
  generation_id?: string
  type: string
  user_id: string
  storage_key: string
  preview_key?: string
  thumbnail_key?: string
  mime_type: string
  size: number
  width: number
  height: number
  checksum: string
  exif?: Record<string, unknown>
  lifecycle_status: string
  created_at: string
}

export const assetsApi = {
  get: (projectId: string, assetId: string) => apiRequest<Asset>('/projects/' + projectId + '/assets/' + assetId),
  downloadUrl: (projectId: string, assetId: string) => apiRequest<{ asset_id: string; url: string; expires_at: string }>('/projects/' + projectId + '/assets/' + assetId + '/download-url'),
  uploadMultipart: (projectId: string, file: File) => assetsApi.uploadMultipartWithProgress(projectId, file),
  // eslint-disable-next-line no-unused-vars
  uploadMultipartWithProgress: (projectId: string, file: File, onProgress?: (_percentage: number) => void) =>
    new Promise<Asset>((resolve, reject) => {
      const token = localStorage.getItem('web-studio-access-token')
      const form = new FormData()
      form.append('file', file)
      const xhr = new XMLHttpRequest()
      xhr.open('POST', API_BASE_URL + '/projects/' + projectId + '/assets')
      if (token) xhr.setRequestHeader('Authorization', 'Bearer ' + token)
      xhr.upload.onprogress = event => {
        if (event.lengthComputable) onProgress?.(Math.min(99, Math.round((event.loaded / event.total) * 100)))
      }
      xhr.onerror = () => reject(new Error('Asset upload failed: network error'))
      xhr.onabort = () => reject(new Error('Asset upload was cancelled'))
      xhr.onload = () => {
        let payload: unknown = null
        try {
          payload = xhr.responseText ? JSON.parse(xhr.responseText) : null
        } catch {
          payload = null
        }
        if (xhr.status >= 200 && xhr.status < 300 && payload && typeof payload === 'object') {
          onProgress?.(100)
          resolve(payload as Asset)
          return
        }
        const message = payload && typeof payload === 'object' && 'error' in payload
          ? String((payload as { error?: { message?: unknown } }).error?.message ?? '')
          : ''
        reject(new Error(message || 'Asset upload failed with status ' + xhr.status))
      }
      xhr.send(form)
    }),
  importFromUrl: (projectId: string, url: string) =>
    apiRequest<Asset>('/projects/' + projectId + '/assets/import-url', {
      method: 'POST',
      body: JSON.stringify({ url }),
    }),
}

export interface SimilarityCheck {
  id: string
  project_id: string
  user_id: string
  target_asset_id: string
  visual_score: number
  composition_score: number
  semantic_score: number
  style_score: number
  search_scope: string
  sources: string[]
  unavailable_sources: string[]
  algorithm: string
  algorithm_version: string
  metadata: Record<string, unknown>
  created_at: string
}

export const similarityApi = {
  create: (projectId: string, input: { target_asset_id: string; reference_ids?: string[]; search_scope?: string }, idempotencyKey: string) =>
    apiRequest<SimilarityCheck>('/projects/' + projectId + '/similarity-checks', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(input),
    }),
  get: (checkId: string) => apiRequest<SimilarityCheck>('/similarity-checks/' + checkId),
}

export interface CreationExport {
  id: string
  project_id: string
  user_id: string
  final_asset_id: string
  final_iteration_id?: string
  status: 'running' | 'completed' | 'failed'
  artifacts: Record<string, string>
  manifest: Record<string, unknown>
  error?: string
  created_at: string
  completed_at?: string
}

export const exportsApi = {
  create: (projectId: string, finalAssetId?: string, idempotencyKey?: string) =>
    apiRequest<CreationExport>('/projects/' + projectId + '/exports', {
      method: 'POST',
      headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined,
      body: JSON.stringify(finalAssetId ? { final_asset_id: finalAssetId } : {}),
    }),
  get: (exportId: string) => apiRequest<CreationExport>('/exports/' + exportId),
}

export interface CompositionSpec {
  id: string
  project_id: string
  iteration_id: string
  user_id: string
  focal_points: unknown[]
  bounding_boxes: unknown[]
  relative_positions: Record<string, unknown>
  horizon?: number
  camera_elevation?: number
  perspective?: string
  hierarchy: unknown[]
  negative_space: Record<string, unknown>
  dominant_geometry: Record<string, unknown>
  object_scale: Record<string, unknown>
  light_direction: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CompositionMutation {
  id: string
  project_id: string
  user_id: string
  composition_spec_id?: string
  source_similarity_check_id?: string
  suggestions: string[]
  status: 'proposed' | 'accepted' | 'rejected'
  accepted_iteration_id?: string
  created_at: string
}

export const compositionApi = {
  get: (projectId: string, iterationId: string) =>
    apiRequest<CompositionSpec>('/projects/' + projectId + '/iterations/' + iterationId + '/composition'),
  update: (projectId: string, iterationId: string, input: Partial<Omit<CompositionSpec, 'id' | 'project_id' | 'iteration_id' | 'user_id' | 'created_at' | 'updated_at'>>) =>
    apiRequest<CompositionSpec>('/projects/' + projectId + '/iterations/' + iterationId + '/composition', {
      method: 'PUT',
      body: JSON.stringify(input),
    }),
  analyze: (projectId: string, iterationId: string, assetId: string) =>
    apiRequest<{ spec: CompositionSpec; analysis: Record<string, unknown>; uncertainty: string }>(
      '/projects/' + projectId + '/iterations/' + iterationId + '/composition/analyze',
      { method: 'POST', body: JSON.stringify({ asset_id: assetId }) },
    ),
  suggest: (
    projectId: string,
    input: { composition_spec_id?: string; source_similarity_check_id?: string; composition_similarity: number },
    idempotencyKey: string,
  ) =>
    apiRequest<CompositionMutation>('/projects/' + projectId + '/composition-mutation-suggestions', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(input),
    }),
  accept: (projectId: string, mutationId: string) =>
    apiRequest<CompositionMutation>('/projects/' + projectId + '/composition-mutation-suggestions/' + mutationId + '/accept', { method: 'POST' }),
  reject: (projectId: string, mutationId: string) =>
    apiRequest<CompositionMutation>('/projects/' + projectId + '/composition-mutation-suggestions/' + mutationId + '/reject', { method: 'POST' }),
}

export interface LibraryItem {
  id: string
  user_id?: string
  kind: 'material' | 'texture'
  category: string
  name: string
  description?: string
  tags: string[]
  prompt_fragment: string
  preview_key?: string
  created_at: string
  updated_at: string
}

function queryString(values: Record<string, string | undefined>) {
  const params = new URLSearchParams()
  Object.entries(values).forEach(([key, value]) => { if (value?.trim()) params.set(key, value.trim()) })
  const query = params.toString()
  return query ? '?' + query : ''
}

export const materialsApi = {
  list: (projectId: string, q?: string, category?: string) =>
    apiRequest<{ items: LibraryItem[] }>('/projects/' + projectId + '/materials' + queryString({ q, category })),
  create: (projectId: string, input: { category: string; name: string; description?: string; tags?: string[]; prompt_fragment: string; preview_key?: string }) =>
    apiRequest<LibraryItem>('/projects/' + projectId + '/materials', { method: 'POST', body: JSON.stringify(input) }),
}

export const texturesApi = {
  list: (projectId: string, q?: string, category?: string) =>
    apiRequest<{ items: LibraryItem[] }>('/projects/' + projectId + '/textures' + queryString({ q, category })),
  create: (projectId: string, input: { category: string; name: string; description?: string; tags?: string[]; prompt_fragment: string; preview_key?: string }) =>
    apiRequest<LibraryItem>('/projects/' + projectId + '/textures', { method: 'POST', body: JSON.stringify(input) }),
}


export type ComfyFlowStepId = 'sketch' | 'reference' | 'structure' | 'final'

export interface ComfyFlowStep {
  id: ComfyFlowStepId
  enabled: boolean
  order: number
}

export interface ComfyFlowEvidence {
  kind: 'asset' | 'node'
  value: string
}

export interface ComfyArtistFlowStep extends ComfyFlowStep {
  evidence?: ComfyFlowEvidence[]
}

export interface ComfyFlowInput {
  id: string
  type: 'image' | 'mask' | 'string' | 'number' | 'integer' | 'boolean'
  label: string
  required: boolean
}

export interface ComfyFlowCompatibility {
  compatible: boolean
  errors: string[]
  warnings: string[]
  referenced_nodes: string[]
  referenced_models: string[]
}

export interface ComfyFlowPlan {
  name: string
  description: string
  prompt: string
  negative_prompt: string
  inputs: ComfyFlowInput[]
  parameters: Record<string, unknown>
  selected_model: Record<string, string>
  workflow: Record<string, unknown>
  reasoning: string
  artist_steps?: ComfyArtistFlowStep[]
  flow_fingerprint?: string
  artist_flow_validated?: boolean
  source?: 'ai_generated' | 'existing_recipe'
  recipe_id?: string
  recipe_version?: number
  compatibility: ComfyFlowCompatibility
  flow_steps?: ComfyFlowStep[]
  compiled_flow_steps?: ComfyFlowStep[]
}

export interface AssistantToolDescriptor {
  name: string
  description: string
  mutating: boolean
  recommendation: boolean
}

export interface AssistantRecommendation {
  recommendation: string
  reason: string
  evidence: Record<string, unknown>
  confidence: number
  affected_entity: Record<string, unknown>
  expected_effect: string
}

export interface AssistantAction {
  id: string
  project_id: string
  user_id: string
  tool: string
  kind: 'tool_execution' | 'recommendation'
  input: Record<string, unknown>
  output: Record<string, unknown>
  explanation: string
  confidence: number
  uncertainty: string
  decision?: 'apply' | 'edit' | 'ignore'
  created_at: string
  decided_at?: string
}

export interface AssistantToolResponse<T = Record<string, unknown>> {
  action: AssistantAction
  result: T
  explanation: string
  confidence: number
  uncertainty: string
}

export const assistantApi = {
  tools: (projectId: string) =>
    apiRequest<{ tools: AssistantToolDescriptor[] }>('/projects/' + projectId + '/assistant/tools'),
  actions: (projectId: string) =>
    apiRequest<{ actions: AssistantAction[] }>('/projects/' + projectId + '/assistant/actions'),
  execute: <T = Record<string, unknown>>(
    projectId: string,
    tool: string,
    input: Record<string, unknown> = {},
    idempotencyKey?: string,
    signal?: AbortSignal,
  ) => apiRequest<AssistantToolResponse<T>>('/projects/' + projectId + '/assistant/tools/' + encodeURIComponent(tool), {
    method: 'POST',
    headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined,
    body: JSON.stringify(input),
    signal,
  }),
  decide: (
    projectId: string,
    actionId: string,
    decision: 'apply' | 'edit' | 'ignore',
    input: { final_text?: string; components?: Record<string, string> } = {},
    idempotencyKey: string,
  ) => apiRequest<AssistantAction>('/projects/' + projectId + '/assistant/recommendations/' + actionId + '/decision', {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify({ decision, ...input }),
  }),
}
