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
  created_at: string
  updated_at: string
}

export interface HumanAction {
  id: string
  project_id: string
  iteration_id?: string
  user_id: string
  action_type: string
  payload?: Record<string, unknown>
  old_state?: Record<string, unknown>
  new_state?: Record<string, unknown>
  ai_influence?: Record<string, unknown>
  created_at: string
}

export interface ProvenanceVerification {
  verified: boolean
  event_count: number
  first_invalid_event_id?: string
  reason?: string
}

export const projectEventsApi = {
  stream: async (
    projectId: string,
    onEvent: (event: ProjectEvent) => void,
    signal?: AbortSignal,
  ) => {
    const token = localStorage.getItem('web-studio-access-token')
    const response = await fetch(API_BASE_URL + '/projects/' + projectId + '/events', {
      headers: {
        Accept: 'text/event-stream',
        ...(token ? { Authorization: 'Bearer ' + token } : {}),
      },
      signal,
    })
    if (!response.ok) throw new Error('Event stream failed with status ' + response.status)
    if (!response.body) throw new Error('Event stream is unavailable')

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let eventName = 'message'
    let data = ''
    const emit = () => {
      if (!data) return
      try {
        const event = JSON.parse(data) as ProjectEvent
        onEvent(event)
      } finally {
        eventName = 'message'
        data = ''
      }
    }

    while (true) {
      const chunk = await reader.read()
      if (chunk.done) break
      buffer += decoder.decode(chunk.value, { stream: true })
      const frames = buffer.split('\n\n')
      buffer = frames.pop() ?? ''
      for (const frame of frames) {
        for (const line of frame.split('\n')) {
          if (line.startsWith('event:')) eventName = line.slice(6).trim()
          else if (line.startsWith('data:')) data += line.slice(5).trim()
        }
        if (eventName === 'message' || eventName.startsWith('generation_') || eventName.startsWith('prompt_') || eventName.startsWith('reference_')) emit()
        else emit()
      }
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
  }) => apiRequest<Reference>('/projects/' + projectId + '/references/' + referenceId, { method: 'PATCH', body: JSON.stringify(input) }),
  remove: (projectId: string, referenceId: string) =>
    apiRequest<void>('/projects/' + projectId + '/references/' + referenceId, { method: 'DELETE' }),
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
  verify: (projectId: string) => apiRequest<ProvenanceVerification>('/projects/' + projectId + '/provenance/verify'),
}
