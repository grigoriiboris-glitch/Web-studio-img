export interface ApiError {
  error: { code: string; message: string; request_id?: string }
}

export interface Project {
  id: string
  user_id: string
  name: string
  description?: string
  status: 'active' | 'archived' | 'deleted'
  created_at: string
  updated_at: string
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

export const healthApi = {
  get: () => apiRequest<{ status: string }>('/health'),
}

export const projectsApi = {
  list: () => apiRequest<{ projects: Project[] }>('/projects'),
  get: (id: string) => apiRequest<Project>(`/projects/${id}`),
  create: (input: { name: string; description?: string }) =>
    apiRequest<Project>('/projects', { method: 'POST', body: JSON.stringify(input) }),
  update: (id: string, input: { name: string; description?: string; status: Project['status'] }) =>
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
  parent_iteration_id?: string
  type: IterationType
  title?: string
  description?: string
  created_at: string
}

export const iterationsApi = {
  list: (projectId: string) =>
    apiRequest<{ iterations: Iteration[] }>(`/projects/${projectId}/iterations`),
  create: (
    projectId: string,
    input: {
      parent_iteration_id?: string
      type: IterationType
      title?: string
      description?: string
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
  status: GenerationStatus
  provider_job_id?: string
  error_code?: string
  error_message?: string
  cost?: number
  created_at: string
  started_at?: string
  completed_at?: string
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
  ) => apiRequest<AssistantToolResponse<T>>('/projects/' + projectId + '/assistant/tools/' + encodeURIComponent(tool), {
    method: 'POST',
    headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined,
    body: JSON.stringify(input),
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
