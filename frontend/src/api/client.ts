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
