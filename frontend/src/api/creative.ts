import { apiRequest } from './client'

export interface CreativeLibraryItem {
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

function libraryPath(projectId: string, kind: 'material' | 'texture') {
  return '/projects/' + projectId + '/' + (kind === 'material' ? 'materials' : 'textures')
}

export const creativeLibraryApi = {
  list: (projectId: string, kind: 'material' | 'texture', q?: string, category?: string) => {
    const params = new URLSearchParams()
    if (q?.trim()) params.set('q', q.trim())
    if (category?.trim()) params.set('category', category.trim())
    const query = params.toString()
    return apiRequest<{ items: CreativeLibraryItem[] }>(libraryPath(projectId, kind) + (query ? '?' + query : ''))
  },
  create: (projectId: string, kind: 'material' | 'texture', input: { category: string; name: string; description?: string; tags?: string[]; prompt_fragment: string; preview_key?: string }) =>
    apiRequest<CreativeLibraryItem>(libraryPath(projectId, kind), { method: 'POST', body: JSON.stringify(input) }),
  update: (projectId: string, kind: 'material' | 'texture', id: string, input: { category: string; name: string; description?: string; tags?: string[]; prompt_fragment: string; preview_key?: string }) =>
    apiRequest<CreativeLibraryItem>(libraryPath(projectId, kind) + '/' + id, { method: 'PATCH', body: JSON.stringify(input) }),
  remove: (projectId: string, kind: 'material' | 'texture', id: string) =>
    apiRequest<void>(libraryPath(projectId, kind) + '/' + id, { method: 'DELETE' }),
  select: (projectId: string, kind: 'material' | 'texture', id: string) =>
    apiRequest<{ item: CreativeLibraryItem; selected: boolean }>(libraryPath(projectId, kind) + '/' + id + '/select', { method: 'POST' }),
}

export interface StyleProfile {
  id: string
  user_id: string
  name: string
  description?: string
  parameters: Record<string, unknown>
  version: number
  prompt_influence: boolean
  created_at: string
  updated_at: string
}

export const styleApi = {
  list: () => apiRequest<{ profiles: StyleProfile[] }>('/style-profiles'),
  create: (input: { name: string; description?: string; parameters?: Record<string, unknown>; prompt_influence?: boolean }) =>
    apiRequest<StyleProfile>('/style-profiles', { method: 'POST', body: JSON.stringify(input) }),
  update: (id: string, input: { name?: string; description?: string; parameters?: Record<string, unknown>; prompt_influence?: boolean }) =>
    apiRequest<StyleProfile>('/style-profiles/' + id, { method: 'PATCH', body: JSON.stringify(input) }),
  infer: (projectId: string) => apiRequest<StyleProfile>('/projects/' + projectId + '/style-profiles/infer', { method: 'POST' }),
  apply: (projectId: string, id: string) =>
    apiRequest<Record<string, unknown>>('/projects/' + projectId + '/style-profiles/' + id + '/apply', { method: 'POST' }),
}

export interface AssetDNAProfile {
  id: string
  user_id: string
  project_id: string
  name: string
  features: Record<string, unknown>
  source_iterations: string[]
  source_assets: string[]
  reusable: boolean
  created_at: string
  updated_at: string
}

export interface VisualLanguageProfile {
  id: string
  user_id: string
  project_id?: string
  name: string
  signals: Record<string, unknown>
  source_iterations: string[]
  version: number
  prompt_influence: boolean
  created_at: string
  updated_at: string
}

export const dnaApi = {
  listAsset: (projectId: string) => apiRequest<{ profiles: AssetDNAProfile[] }>('/projects/' + projectId + '/asset-dna'),
  analyzeAsset: (projectId: string) => apiRequest<AssetDNAProfile>('/projects/' + projectId + '/asset-dna/analyze', { method: 'POST' }),
  listLanguage: (projectId: string) => apiRequest<{ profiles: VisualLanguageProfile[] }>('/projects/' + projectId + '/visual-language'),
  analyzeLanguage: (projectId: string) => apiRequest<VisualLanguageProfile>('/projects/' + projectId + '/visual-language/analyze', { method: 'POST' }),
  applyLanguage: (projectId: string, id: string) =>
    apiRequest<Record<string, unknown>>('/projects/' + projectId + '/visual-language/' + id + '/apply', { method: 'POST' }),
}

export interface RightsItem {
  id: string
  user_id: string
  project_id: string
  target_type: 'asset' | 'reference'
  target_id: string
  ownership: 'user_owned' | 'third_party' | 'public_domain' | 'unknown'
  license: string
  license_source?: string
  verification_state: 'verified' | 'unverified' | 'unknown' | 'restricted'
  verification_date?: string
  notes?: string
  created_at: string
  updated_at: string
}

export interface DoNotUseConstraint {
  id: string
  user_id: string
  project_id?: string
  kind: 'artist' | 'image' | 'reference' | 'motif' | 'brand' | 'composition' | 'style'
  value: string
  active: boolean
  created_at: string
  updated_at: string
}

export const rightsApi = {
  list: (projectId: string) => apiRequest<{ items: RightsItem[] }>('/projects/' + projectId + '/rights'),
  create: (projectId: string, input: { target_type: RightsItem['target_type']; target_id: string; ownership: RightsItem['ownership']; license: string; license_source?: string; verification_state?: RightsItem['verification_state']; verification_date?: string; notes?: string }) =>
    apiRequest<RightsItem>('/projects/' + projectId + '/rights', { method: 'POST', body: JSON.stringify(input) }),
  update: (projectId: string, id: string, input: { ownership?: RightsItem['ownership']; license?: string; license_source?: string; verification_state?: RightsItem['verification_state']; verification_date?: string; notes?: string }) =>
    apiRequest<RightsItem>('/projects/' + projectId + '/rights/' + id, { method: 'PATCH', body: JSON.stringify(input) }),
  remove: (projectId: string, id: string) => apiRequest<void>('/projects/' + projectId + '/rights/' + id, { method: 'DELETE' }),
}

export const constraintsApi = {
  listGlobal: () => apiRequest<{ constraints: DoNotUseConstraint[] }>('/constraints'),
  listProject: (projectId: string) => apiRequest<{ constraints: DoNotUseConstraint[] }>('/projects/' + projectId + '/constraints'),
  createGlobal: (input: { kind: DoNotUseConstraint['kind']; value: string; active?: boolean }) =>
    apiRequest<DoNotUseConstraint>('/constraints', { method: 'POST', body: JSON.stringify(input) }),
  createProject: (projectId: string, input: { kind: DoNotUseConstraint['kind']; value: string; active?: boolean }) =>
    apiRequest<DoNotUseConstraint>('/projects/' + projectId + '/constraints', { method: 'POST', body: JSON.stringify(input) }),
  update: (projectId: string, id: string, input: { kind?: DoNotUseConstraint['kind']; value?: string; active?: boolean }) =>
    apiRequest<DoNotUseConstraint>('/projects/' + projectId + '/constraints/' + id, { method: 'PATCH', body: JSON.stringify(input) }),
  remove: (projectId: string, id: string) => apiRequest<void>('/projects/' + projectId + '/constraints/' + id, { method: 'DELETE' }),
}

export interface Layer {
  id: string
  project_id: string
  user_id: string
  iteration_id?: string
  asset_id?: string
  parent_layer_id?: string
  name: string
  layer_type: 'base' | 'mask' | 'image' | 'manual_edit' | 'adjustment' | 'group'
  order_index: number
  visible: boolean
  opacity: number
  blend_mode: string
  source_kind: 'human' | 'ai' | 'derived' | 'imported'
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface ManualEdit {
  id: string
  project_id: string
  user_id: string
  iteration_id?: string
  source_asset_id: string
  mask_asset_id?: string
  result_asset_id?: string
  operation: 'paint' | 'erase' | 'mask' | 'composite'
  prompt?: string
  parameters: Record<string, unknown>
  status: 'draft' | 'applied' | 'rejected'
  created_at: string
  updated_at: string
}

export const layersApi = {
  list: (projectId: string) => apiRequest<{ layers: Layer[] }>('/projects/' + projectId + '/layers'),
  create: (projectId: string, input: Partial<Omit<Layer, 'id' | 'project_id' | 'user_id' | 'created_at' | 'updated_at'>>) =>
    apiRequest<Layer>('/projects/' + projectId + '/layers', { method: 'POST', body: JSON.stringify(input) }),
  update: (projectId: string, id: string, input: Partial<Omit<Layer, 'id' | 'project_id' | 'user_id' | 'created_at' | 'updated_at'>>) =>
    apiRequest<Layer>('/projects/' + projectId + '/layers/' + id, { method: 'PATCH', body: JSON.stringify(input) }),
  remove: (projectId: string, id: string) => apiRequest<void>('/projects/' + projectId + '/layers/' + id, { method: 'DELETE' }),
}

export const manualEditsApi = {
  list: (projectId: string) => apiRequest<{ edits: ManualEdit[] }>('/projects/' + projectId + '/manual-edits'),
  create: (projectId: string, input: { iteration_id?: string; source_asset_id: string; mask_asset_id?: string; result_asset_id?: string; operation: ManualEdit['operation']; prompt?: string; parameters?: Record<string, unknown> }, idempotencyKey: string) =>
    apiRequest<ManualEdit>('/projects/' + projectId + '/manual-edits', { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(input) }),
  apply: (projectId: string, id: string) => apiRequest<ManualEdit>('/projects/' + projectId + '/manual-edits/' + id + '/apply', { method: 'POST' }),
  reject: (projectId: string, id: string) => apiRequest<ManualEdit>('/projects/' + projectId + '/manual-edits/' + id + '/reject', { method: 'POST' }),
}
