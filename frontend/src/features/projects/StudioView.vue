<!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline -->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  RouterLink, useRoute } from 'vue-router'
import SketchImportZone from '../../components/SketchImportZone.vue'
import {
  generationsApi,
  humanActionsApi,
  iterationsApi,
  projectEventsApi,
  projectsApi,
  promptsApi,
  provenanceApi,
  referencesApi,
  similarityApi,
  exportsApi,
  compositionApi,
  assistantApi,
  materialsApi,
  texturesApi,
  variantBoardApi,
  type RejectReasonSummary,
  type Asset,
  type Generation,
  type HumanAction,
  type Iteration,
  type IterationType,
  type Project,
  type Prompt,
  type Reference,
  type SimilarityCheck,
  type CreationExport,
  type CompositionSpec,
  type CompositionMutation,
  type AssistantAction,
  type AssistantRecommendation,
  type LibraryItem,
  recipesApi,
  type Recipe,
} from '../../api/client'

const route = useRoute()
const project = ref<Project | null>(null)
const iterations = ref<Iteration[]>([])
const generations = ref<Generation[]>([])
const prompts = ref<Prompt[]>([])
const references = ref<Reference[]>([])
const humanActions = ref<HumanAction[]>([])
const selectedGenerationId = ref<string | null>(null)
const rejectedGenerationIds = ref<string[]>([])
const selectedReferenceId = ref<string | null>(null)
const error = ref<string | null>(null)
const loading = ref(true)
const creating = ref(false)
const generating = ref(false)
const savingPrompt = ref(false)
const addingReference = ref(false)
const lastUploadedAsset = ref('')
const verifying = ref(false)
const provenanceVerified = ref<boolean | null>(null)
const provenanceMessage = ref('')
const referenceAnalysisTarget = ref('')
const similarityTargetAsset = ref('')
const similarityResult = ref<SimilarityCheck | null>(null)
const creationExport = ref<CreationExport | null>(null)
const exporting = ref(false)
const compositionSpec = ref<CompositionSpec | null>(null)
const compositionIterationId = ref('')
const compositionSaving = ref(false)
const compositionMutation = ref<CompositionMutation | null>(null)
const materials = ref<LibraryItem[]>([])
const textures = ref<LibraryItem[]>([])
const materialSearch = ref('')
const textureSearch = ref('')
const selectedMaterial = ref('')
const selectedTexture = ref('')
const assistantTools = ref<{ name: string; description: string; mutating: boolean; recommendation: boolean }[]>([])
const assistantActions = ref<AssistantAction[]>([])
const assistantLoading = ref(false)
const assistantDecisionLoading = ref('')
const assistantEditAction = ref<AssistantAction | null>(null)
const assistantEditText = ref('')
const criticAIterationId = ref('')
const criticBIterationId = ref('')
const criticResult = ref<Record<string, unknown> | null>(null)
const criticLoading = ref(false)
const factClaim = ref('')
const factEvidenceJson = ref(JSON.stringify([
  {
    text: '',
    source: '',
    assessment: 'unknown',
    confidence: 0.5,
  },
], null, 2))
const factResult = ref<Record<string, unknown> | null>(null)
const factLoading = ref(false)
const rejectionSummary = ref<RejectReasonSummary | null>(null)
let eventAbort: AbortController | undefined

const types: IterationType[] = ['idea', 'sketch', 'generation', 'selection', 'composition', 'prompt', 'manual_edit', 'final']
const promptComponents = ['subject', 'composition', 'camera', 'lighting', 'material', 'texture', 'color', 'atmosphere', 'style', 'depth', 'detail', 'constraints', 'negative_constraints']

const form = ref<{ type: IterationType; title: string; description: string }>({
  type: 'idea',
  title: '',
  description: '',
})
const recipes = ref<Recipe[]>([])
const selectedRecipeId = ref('')
const recipeParameters = ref<Record<string, unknown>>({})
const selectedRecipe = computed(() => recipes.value.find(recipe => recipe.id === selectedRecipeId.value) ?? null)

const generationForm = ref({ prompt: '', negative_prompt: '', aspect_ratio: '1:1', seed: undefined as number | undefined })
const promptForm = ref({
  original_text: '',
  ai_suggestions: '',
  final_text: '',
  created_by: 'human' as Prompt['created_by'],
  components: {} as Record<string, string>,
})
const referenceForm = ref({
  asset_id: '',
  source_url: '',
  source_type: 'reference' as Reference['source_type'],
  license: 'unknown',
  license_verified: false,
  user_owned: false,
  sha256: '',
  notes: '',
  influence: {
    composition: 0,
    semantic: 0,
    color: 0,
    style: 0,
    material: 0,
    geometry: 0,
  },
})
const builderText = computed(() => {
  const ordered = promptComponents
    .map(component => promptForm.value.components[component]?.trim())
    .filter((value): value is string => Boolean(value))
  return ordered.join(', ')
})

const projectId = () => String(route.params.projectId)

async function loadStudio() {
  const id = projectId()
  const [loadedProject, timeline, loadedPrompts, loadedReferences, loadedActions, loadedAssistantTools, loadedAssistantActions] = await Promise.all([
    projectsApi.get(id),
    iterationsApi.list(id),
    promptsApi.list(id),
    referencesApi.list(id),
    humanActionsApi.list(id),
    assistantApi.tools(id),
    assistantApi.actions(id),
  ])
  project.value = loadedProject
  iterations.value = timeline.iterations
  prompts.value = loadedPrompts.prompts
  references.value = loadedReferences.references
  humanActions.value = loadedActions.actions
  assistantTools.value = loadedAssistantTools.tools
  assistantActions.value = loadedAssistantActions.actions
  try {
    rejectionSummary.value = await variantBoardApi.rejectionSummary(id)
  } catch {
    rejectionSummary.value = null
  }
  if (timeline.iterations.length >= 2 && !criticAIterationId.value && !criticBIterationId.value) {
    criticAIterationId.value = timeline.iterations[timeline.iterations.length - 2].id
    criticBIterationId.value = timeline.iterations[timeline.iterations.length - 1].id
  }
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

function criticObservations(): Array<Record<string, unknown>> {
  const value = criticResult.value?.observations
  if (!Array.isArray(value)) return []
  return value.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === 'object')
}

function criticPromptSimilarity(): number | null {
  const prompts = criticResult.value?.prompts
  if (!prompts || typeof prompts !== 'object') return null
  const value = (prompts as Record<string, unknown>).similarity
  return typeof value === 'number' ? value : null
}

function criticAlgorithm(): string {
  const comparison = criticResult.value?.image_comparison
  if (!comparison || typeof comparison !== 'object') return 'metadata-only'
  const value = (comparison as Record<string, unknown>).algorithm
  return typeof value === 'string' ? value : 'metadata-only'
}

function criticAssetAvailable(side: 'a' | 'b'): boolean | null {
  const assets = criticResult.value?.assets
  if (!assets || typeof assets !== 'object') return null
  const value = (assets as Record<string, unknown>)[side]
  if (!value || typeof value !== 'object') return null
  const available = (value as Record<string, unknown>).available
  return typeof available === 'boolean' ? available : null
}

function criticMetric(name: string): number | null {
  const comparison = criticResult.value?.image_comparison
  if (!comparison || typeof comparison !== 'object') return null
  const value = (comparison as Record<string, unknown>)[name]
  return typeof value === 'number' ? value : null
}

async function compareIterations() {
  if (!criticAIterationId.value || !criticBIterationId.value) {
    error.value = 'Select two iterations to compare'
    return
  }
  if (criticAIterationId.value === criticBIterationId.value) {
    error.value = 'Select two different iterations'
    return
  }
  criticLoading.value = true
  error.value = null
  try {
    const response = await assistantApi.execute<Record<string, unknown>>(
      projectId(),
      'compare_iterations',
      {
        iteration_a_id: criticAIterationId.value,
        iteration_b_id: criticBIterationId.value,
      },
    )
    criticResult.value = response.result
    assistantActions.value.push(response.action)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not compare iterations'
  } finally {
    criticLoading.value = false
  }
}

async function runFactCheck() {
  if (!factClaim.value.trim()) {
    error.value = 'Enter a claim to check'
    return
  }
  let evidence: unknown
  try {
    evidence = JSON.parse(factEvidenceJson.value)
  } catch {
    error.value = 'Evidence must be valid JSON'
    return
  }
  if (!Array.isArray(evidence) || evidence.length === 0) {
    error.value = 'Provide at least one evidence item'
    return
  }

  factLoading.value = true
  error.value = null
  try {
    const response = await assistantApi.execute<Record<string, unknown>>(
      projectId(),
      'fact_check',
      {
        claim: factClaim.value.trim(),
        evidence,
      },
    )
    factResult.value = response.result
    assistantActions.value.push(response.action)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not run fact checker'
  } finally {
    factLoading.value = false
  }
}

function factResultValue(name: string): unknown {
  return factResult.value?.[name]
}

function factEvidenceItems(): Array<Record<string, unknown>> {
  const value = factResult.value?.evidence
  if (!Array.isArray(value)) return []
  return value.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === 'object')
}

function responseUncertainty(status: unknown): string {
  switch (status) {
    case 'SUPPORTED': return 'Supporting evidence was recorded, but source contents were not independently verified.'
    case 'PARTIALLY_SUPPORTED': return 'Evidence is mixed or incomplete; review the supplied sources before relying on the claim.'
    default: return 'The available evidence does not establish the claim; review the supplied sources.'
  }
}

async function runAssistantRecommendation(tool: string) {
  assistantLoading.value = true
  error.value = null
  try {
    const input: Record<string, unknown> = {}
    if (tool === 'analyze_composition') {
      const assetId = lastUploadedAsset.value.trim()
      if (!assetId) {
        error.value = 'Upload an asset before asking the Assistant for composition analysis'
        return
      }
      input.asset_id = assetId
      if (compositionIterationId.value) input.iteration_id = compositionIterationId.value
    } else if (tool === 'suggest_materials') {
      input.kind = 'material'
      input.query = materialSearch.value.trim()
    }
    const response = await assistantApi.execute(projectId(), tool, input)
    assistantActions.value.push(response.action)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not run Assistant recommendation'
  } finally {
    assistantLoading.value = false
  }
}

function assistantRecommendation(action: AssistantAction): AssistantRecommendation | null {
  const value = action.output?.recommendation
  if (!value || typeof value !== 'object') return null
  return value as unknown as AssistantRecommendation
}

function openAssistantEdit(action: AssistantAction) {
  const recommendation = assistantRecommendation(action)
  if (!recommendation) return
  assistantEditAction.value = action
  assistantEditText.value = recommendation.recommendation
}

async function decideAssistant(action: AssistantAction, decision: 'apply' | 'edit' | 'ignore') {
  if (decision === 'edit') {
    openAssistantEdit(action)
    return
  }
  assistantDecisionLoading.value = action.id
  error.value = null
  try {
    const updated = await assistantApi.decide(projectId(), action.id, decision, {}, crypto.randomUUID())
    const index = assistantActions.value.findIndex(item => item.id === action.id)
    if (index >= 0) assistantActions.value[index] = updated
    else assistantActions.value.push(updated)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not record Assistant decision'
  } finally {
    assistantDecisionLoading.value = ''
  }
}

async function submitAssistantEdit() {
  const action = assistantEditAction.value
  const finalText = assistantEditText.value.trim()
  if (!action || !finalText) return
  assistantDecisionLoading.value = action.id
  error.value = null
  try {
    const updated = await assistantApi.decide(projectId(), action.id, 'edit', { final_text: finalText }, crypto.randomUUID())
    const index = assistantActions.value.findIndex(item => item.id === action.id)
    if (index >= 0) assistantActions.value[index] = updated
    assistantEditAction.value = null
    assistantEditText.value = ''
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not edit Assistant recommendation'
  } finally {
    assistantDecisionLoading.value = ''
  }
}

async function createIteration() {
  creating.value = true
  error.value = null
  if (form.value.type === 'manual_edit' && !form.value.description.trim()) {
    error.value = 'Describe the manual edit before recording it in the timeline'
    creating.value = false
    return
  }
  try {
    const created = await iterationsApi.create(projectId(), {
      type: form.value.type,
      title: form.value.title.trim() || undefined,
      description: form.value.description.trim() || undefined,
    })
    iterations.value.push(created)
    const actionType = created.type === 'idea'
      ? 'IDEA_CREATED'
      : created.type === 'composition'
        ? 'COMPOSITION_CHANGED'
        : created.type === 'manual_edit'
          ? 'MANUAL_EDIT'
          : ''
    if (created.type === 'composition') {
      try {
        compositionSpec.value = await compositionApi.update(projectId(), created.id, {})
        compositionIterationId.value = created.id
      } catch (err) {
        error.value = err instanceof Error ? err.message : 'Could not initialize composition editor'
      }
    }
    if (actionType) {
      await humanActionsApi.create(projectId(), {
        iteration_id: created.id,
        action_type: actionType,
        payload: {
          iteration_type: created.type,
          title: created.title,
          description: created.description,
          version: created.id,
        },
        old_state: { iteration_id: null, title: null, description: null },
        new_state: { iteration_id: created.id, title: created.title, description: created.description },
      })
      await refreshActions()
    }
    form.value = { type: 'idea', title: '', description: '' }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not create iteration'
  } finally {
    creating.value = false
  }
}

function applyAvoidConstraints() {
  if (!rejectionSummary.value?.reasons.length) return
  const constraints = rejectionSummary.value.reasons
    .slice(0, 5)
    .map(item => 'avoid ' + item.reason)
    .join(', ')
  const current = generationForm.value.negative_prompt.trim()
  generationForm.value.negative_prompt = current
    ? current + ', ' + constraints
    : constraints
}

async function loadRecipes() {
  try {
    const result = await recipesApi.list(projectId)
    recipes.value = result.recipes
  } catch {
    recipes.value = []
  }
}

function selectRecipe() {
  const defaults = selectedRecipe.value?.version?.default_parameters ?? {}
  recipeParameters.value = { ...defaults }
}

function recipeParameterValue(name: string): unknown {
  return recipeParameters.value[name]
}

function setRecipeParameter(name: string, value: unknown) {
  recipeParameters.value = { ...recipeParameters.value, [name]: value }
}

async function createGeneration() {
  generating.value = true
  error.value = null
  try {
    const generationPayload = {
    ...generationForm.value,
    prompt: generationForm.value.prompt.trim(),
    negative_prompt: generationForm.value.negative_prompt.trim() || undefined,
    parameters: { ...(generationForm.value.parameters ?? {}), ...recipeParameters.value },
    recipe_id: selectedRecipeId.value || undefined,
    recipe_version: selectedRecipe.value?.current_version,
  }
  const created = await generationsApi.create(projectId(), generationPayload, crypto.randomUUID())
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
    const generatedFinalText = promptForm.value.final_text.trim() || builderText.value.trim() || undefined
    const created = await promptsApi.create(projectId(), {
      original_text: promptForm.value.original_text.trim(),
      ai_suggestions: suggestions,
      final_text: generatedFinalText,
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

async function logHumanAction(input: Parameters<typeof humanActionsApi.create>[1]) {
  await humanActionsApi.create(projectId(), input)
  await refreshActions()
}

async function acceptSuggestion(prompt: Prompt, suggestion: string) {
  const current = promptForm.value.final_text.trim()
  const next = suggestion
  promptForm.value.final_text = next
  error.value = null
  try {
    await logHumanAction({
      action_type: 'PROMPT_EDITED',
      payload: { prompt_id: prompt.id, decision: 'accepted', suggestion, version: prompt.version },
      old_state: { final_text: current, components: prompt.components ?? {} },
      new_state: { final_text: next, components: prompt.components ?? {} },
      ai_influence: { source: 'prompt_suggestion', accepted: true },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not record prompt suggestion decision'
  }
}

async function partiallyApplySuggestion(prompt: Prompt, suggestion: string) {
  const current = promptForm.value.final_text.trim()
  const next = current ? current + ', ' + suggestion : suggestion
  promptForm.value.final_text = next
  try {
    await logHumanAction({
      action_type: 'PROMPT_EDITED',
      payload: { prompt_id: prompt.id, decision: 'partial', suggestion, version: prompt.version },
      old_state: { final_text: current },
      new_state: { final_text: next },
      ai_influence: { source: 'prompt_suggestion', partially_accepted: true },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not record partial prompt suggestion'
  }
}

async function rejectSuggestion(prompt: Prompt, suggestion: string) {
  const current = promptForm.value.final_text.trim()
  try {
    await logHumanAction({
      action_type: 'AI_RECOMMENDATION_REJECTED',
      payload: { prompt_id: prompt.id, decision: 'rejected', suggestion, version: prompt.version },
      old_state: { final_text: current, ai_suggestion: suggestion },
      new_state: { final_text: current, ai_suggestion: suggestion, disposition: 'rejected' },
      ai_influence: { source: 'prompt_suggestion', rejected: true },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not record AI recommendation rejection'
  }
}

async function selectVariant(item: Generation) {
  const previous = selectedGenerationId.value
  selectedGenerationId.value = item.id
  rejectedGenerationIds.value = rejectedGenerationIds.value.filter(id => id !== item.id)
  try {
    await logHumanAction({
      action_type: 'VARIANT_SELECTED',
      payload: { generation_id: item.id, prompt: item.prompt, version: item.id },
      old_state: { selected_generation_id: previous },
      new_state: { selected_generation_id: item.id },
      ai_influence: { source: 'generation' },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not record selected variant'
  }
}

async function rejectVariant(item: Generation) {
  const wasRejected = rejectedGenerationIds.value.includes(item.id)
  rejectedGenerationIds.value = wasRejected
    ? rejectedGenerationIds.value.filter(id => id !== item.id)
    : [...rejectedGenerationIds.value, item.id]
  try {
    await logHumanAction({
      action_type: 'VARIANT_REJECTED',
      payload: { generation_id: item.id, prompt: item.prompt, version: item.id },
      old_state: { rejected_generation_id: wasRejected ? item.id : null },
      new_state: { rejected_generation_id: wasRejected ? null : item.id },
      ai_influence: { source: 'generation' },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not record rejected variant'
  }
}

async function loadComposition(iterationId?: string) {
  const id = iterationId ?? compositionIterationId.value
  if (!id) return
  try {
    compositionSpec.value = await compositionApi.get(projectId(), id)
    compositionIterationId.value = id
  } catch {
    compositionSpec.value = null
  }
}

async function saveComposition() {
  if (!compositionIterationId.value) {
    error.value = 'Create a composition iteration first'
    return
  }
  compositionSaving.value = true
  const before = compositionSpec.value
    ? JSON.parse(JSON.stringify(compositionSpec.value))
    : null
  try {
    compositionSpec.value = await compositionApi.update(projectId(), compositionIterationId.value, {
      focal_points: compositionSpec.value?.focal_points ?? [],
      bounding_boxes: compositionSpec.value?.bounding_boxes ?? [],
      relative_positions: compositionSpec.value?.relative_positions ?? {},
      horizon: compositionSpec.value?.horizon,
      camera_elevation: compositionSpec.value?.camera_elevation,
      perspective: compositionSpec.value?.perspective,
      hierarchy: compositionSpec.value?.hierarchy ?? [],
      negative_space: compositionSpec.value?.negative_space ?? {},
      dominant_geometry: compositionSpec.value?.dominant_geometry ?? {},
      object_scale: compositionSpec.value?.object_scale ?? {},
      light_direction: compositionSpec.value?.light_direction ?? {},
    })
    await logHumanAction({
      iteration_id: compositionIterationId.value,
      action_type: 'COMPOSITION_CHANGED',
      payload: { composition_spec_id: compositionSpec.value.id, iteration_id: compositionIterationId.value },
      old_state: before ?? {},
      new_state: compositionSpec.value,
      ai_influence: { source: 'composition_editor', assisted: false },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not save composition'
  } finally {
    compositionSaving.value = false
  }
}

function setCompositionJSON(field: 'focal_points' | 'bounding_boxes', value: string) {
  if (!compositionSpec.value) return
  try {
    compositionSpec.value[field] = JSON.parse(value || '[]') as never
  } catch {
    error.value = 'Composition JSON is invalid'
  }
}

function updateCompositionText(field: 'perspective') {
  if (!compositionSpec.value) return
  compositionSpec.value[field] = compositionSpec.value[field]?.trim()
}

async function requestCompositionMutation() {
  const score = similarityResult.value?.composition_score ?? 0
  if (score < 0.8) {
    error.value = 'Composition mutation suggestions require a composition similarity score of at least 0.80'
    return
  }
  try {
    compositionMutation.value = await compositionApi.suggest(projectId(), {
      composition_spec_id: compositionSpec.value?.id,
      source_similarity_check_id: similarityResult.value?.id,
      composition_similarity: score,
    }, crypto.randomUUID())
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not suggest composition mutations'
  }
}

async function acceptCompositionMutation() {
  if (!compositionMutation.value) return
  try {
    const result = await compositionApi.accept(projectId(), compositionMutation.value.id)
    compositionMutation.value = result
    if (result.accepted_iteration_id) {
      iterations.value = (await iterationsApi.list(projectId())).iterations
      await loadComposition(result.accepted_iteration_id)
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not accept composition mutation'
  }
}

async function rejectCompositionMutation() {
  if (!compositionMutation.value) return
  try {
    compositionMutation.value = await compositionApi.reject(projectId(), compositionMutation.value.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not reject composition mutation'
  }
}

async function runSimilarityCheck() {
  const target = similarityTargetAsset.value.trim() || lastUploadedAsset.value
  if (!target) { error.value = 'Upload or select a target asset first'; return }
  try {
    similarityResult.value = await similarityApi.create(projectId(), { target_asset_id: target }, crypto.randomUUID())
    similarityTargetAsset.value = target
    const compositionIteration = iterations.value.find(item => item.type === 'composition') ?? iterations.value[0]
    if (compositionIteration) await loadComposition(compositionIteration.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not run similarity check'
  }
}

async function createCreationReport() {
  exporting.value = true
  try {
    const created = await exportsApi.create(projectId(), lastUploadedAsset.value || undefined, crypto.randomUUID())
    creationExport.value = await exportsApi.get(created.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not create Creation Report'
  } finally {
    exporting.value = false
  }
}

async function loadLibraries() {
  try {
    materials.value = (await materialsApi.list(projectId(), materialSearch.value)).items
    textures.value = (await texturesApi.list(projectId(), textureSearch.value)).items
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load libraries'
  }
}

async function chooseMaterial(item: LibraryItem) {
  const previous = selectedMaterial.value
  selectedMaterial.value = item.name
  await logHumanAction({
    action_type: 'MATERIAL_SELECTED',
    payload: { library_item_id: item.id, name: item.name, prompt_fragment: item.prompt_fragment },
    old_state: { material: previous },
    new_state: { material: item.name, prompt_fragment: item.prompt_fragment },
  })
}

async function chooseTexture(item: LibraryItem) {
  const previous = selectedTexture.value
  selectedTexture.value = item.name
  await logHumanAction({
    action_type: 'TEXTURE_SELECTED',
    payload: { library_item_id: item.id, name: item.name, prompt_fragment: item.prompt_fragment },
    old_state: { texture: previous },
    new_state: { texture: item.name, prompt_fragment: item.prompt_fragment },
  })
}

async function analyzeReference(reference: Reference) {
  const target = referenceAnalysisTarget.value.trim()
  if (!target) {
    error.value = 'Enter target asset ID before analysis'
    return
  }
  try {
    const updated = await referencesApi.analyzeInfluence(projectId(), reference.id, target)
    const index = references.value.findIndex(item => item.id === updated.id)
    if (index >= 0) references.value[index] = updated
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not analyze reference influence'
  }
}

async function selectReference(reference: Reference) {
  const previous = selectedReferenceId.value
  selectedReferenceId.value = reference.id
  try {
    await logHumanAction({
      action_type: 'REFERENCE_SELECTED',
      payload: { reference_id: reference.id, source_type: reference.source_type },
      old_state: { selected_reference_id: previous },
      new_state: { selected_reference_id: reference.id },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not record reference selection'
  }
}

async function approveProject() {
  const previous = Boolean(humanActions.value.find(action => action.action_type === 'APPROVED'))
  try {
    await logHumanAction({
      action_type: 'APPROVED',
      payload: { project_id: projectId(), version: iterations.value[iterations.value.length - 1]?.id },
      old_state: { approved: previous },
      new_state: { approved: true },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not record final approval'
  }
}

async function handleSketchImported(payload: {
  asset: Asset
  name: string
  source: 'file' | 'clipboard' | 'url'
}) {
  lastUploadedAsset.value = payload.asset.id
  referenceForm.value.asset_id = payload.asset.id
  error.value = null
  try {
    const sourceLabel = payload.source === 'clipboard'
      ? 'clipboard'
      : payload.source === 'url'
        ? 'browser URL'
        : 'file'
    const title = payload.name.trim() || 'Imported sketch'
    const description = `Imported sketch from ${sourceLabel}: asset ${payload.asset.id} (${payload.asset.mime_type}, ${payload.asset.width}×${payload.asset.height})`
    const iteration = await iterationsApi.create(projectId(), {
      type: 'sketch',
      title,
      description,
    })
    iterations.value.push(iteration)
    await logHumanAction({
      iteration_id: iteration.id,
      action_type: 'SKETCH_IMPORTED',
      payload: {
        asset_id: payload.asset.id,
        source: payload.source,
        name: payload.name,
        mime_type: payload.asset.mime_type,
        width: payload.asset.width,
        height: payload.asset.height,
        checksum: payload.asset.checksum,
      },
      old_state: { asset_id: null, iteration_id: null },
      new_state: { asset_id: payload.asset.id, iteration_id: iteration.id },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Sketch was imported but timeline update failed'
  }
}

async function addReference() {
  addingReference.value = true
  error.value = null
  try {
    const created = await referencesApi.create(projectId(), {
      asset_id: referenceForm.value.asset_id.trim() || undefined,
      source_url: referenceForm.value.source_url.trim() || undefined,
      source_type: referenceForm.value.source_type,
      license: referenceForm.value.license.trim(),
      license_verified: referenceForm.value.license_verified,
      user_owned: referenceForm.value.user_owned,
      sha256: referenceForm.value.sha256.trim() || undefined,
      notes: referenceForm.value.notes.trim() || undefined,
      influence: Object.values(referenceForm.value.influence).some(value => value > 0)
        ? { ...referenceForm.value.influence }
        : undefined,
    })
    references.value = [created, ...references.value]
    referenceForm.value = {
      asset_id: '',
      source_url: '',
      source_type: 'reference',
      license: 'unknown',
      license_verified: false,
      user_owned: false,
      sha256: '',
      notes: '',
      influence: { composition: 0, semantic: 0, color: 0, style: 0, material: 0, geometry: 0 },
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
    provenanceVerified.value = result.valid
    provenanceMessage.value = result.valid
      ? 'Hash chain is valid across ' + result.events_checked + ' events.'
      : (result.broken_links?.[0] ?? 'Hash chain verification failed.')
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
    await loadLibraries()
    const compositionIteration = iterations.value.find(item => item.type === 'composition')
    if (compositionIteration) await loadComposition(compositionIteration.id)
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
      <el-space>
        <RouterLink :to="`/projects/${projectId()}/resources`">
          <el-button plain>Creative Resources</el-button>
        </RouterLink>
        <el-tag v-if="project">{{ project.status }}</el-tag>
      </el-space>
    </el-header>

    <el-main>
      <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = null" />
      <el-skeleton v-if="loading" :rows="8" animated />

      <template v-else>
        <el-card v-if="rejectionSummary?.reasons.length" class="create-card avoid-card">
          <template #header>Avoid based on previous decisions</template>
          <el-space wrap>
            <el-tag v-for="item in rejectionSummary.reasons.slice(0, 5)" :key="item.reason">
              {{ item.reason }} · {{ item.count }}
            </el-tag>
          </el-space>
          <p class="avoid-copy">
            Эти ограничения не изменяют prompt автоматически. Нажми Apply, чтобы явно добавить их в negative prompt.
          </p>
          <el-button @click="applyAvoidConstraints">Apply to negative prompt</el-button>
        </el-card>

        <el-card class="create-card recipe-card">
  <template #header>Recipe</template>
  <label class="recipe-field">
    Recipe
    <select v-model="selectedRecipeId" @change="selectRecipe">
      <option value="">Default generation flow</option>
      <option v-for="recipe in recipes" :key="recipe.id" :value="recipe.id">
        {{ recipe.name }} · v{{ recipe.current_version }}
      </option>
    </select>
  </label>

  <template v-if="selectedRecipe?.version">
    <p class="recipe-description">
      {{ selectedRecipe.description || 'Versioned ComfyUI workflow recipe' }}
    </p>
    <div class="recipe-parameters">
      <label
        v-for="parameter in selectedRecipe.version.exposed_parameters"
        :key="parameter.name"
        class="recipe-field"
      >
        {{ parameter.name }}<span v-if="parameter.required"> *</span>
        <small v-if="parameter.description">{{ parameter.description }}</small>
        <input
          v-if="parameter.type === 'string' || parameter.type === 'image' || parameter.type === 'mask'"
          v-model="recipeParameters[parameter.name]"
          :required="parameter.required"
          :placeholder="String(parameter.default ?? '')"
        >
        <input
          v-else-if="parameter.type === 'number' || parameter.type === 'integer'"
          v-model.number="recipeParameters[parameter.name]"
          :required="parameter.required"
          type="number"
        >
        <input
          v-else
          v-model="recipeParameters[parameter.name]"
          type="checkbox"
        >
      </label>
    </div>
  </template>
</el-card>

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
              <el-alert v-if="builderText" :title="'Builder output: ' + builderText" type="info" :closable="false" />
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
                  <ul>
                    <li v-for="suggestion in prompt.ai_suggestions" :key="suggestion">
                      {{ suggestion }}
                      <el-button size="small" link type="success" @click="acceptSuggestion(prompt, suggestion)">Use</el-button>
                      <el-button size="small" link type="warning" @click="partiallyApplySuggestion(prompt, suggestion)">Insert</el-button>
                      <el-button size="small" link type="danger" @click="rejectSuggestion(prompt, suggestion)">Reject</el-button>
                    </li>
                  </ul>
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
          <template #header>Sketch import</template>
          <SketchImportZone
            :project-id="projectId()"
            @imported="handleSketchImported"
          />
          <el-tag v-if="lastUploadedAsset" type="success" class="last-asset-tag">
            Last asset: {{ lastUploadedAsset }}
          </el-tag>
        </el-card>

        <el-card class="create-card">
          <template #header>References</template>
          <el-form label-position="top" @submit.prevent="addReference">
            <el-form-item label="Asset ID (for uploaded reference)">
              <el-input v-model="referenceForm.asset_id" placeholder="UUID" />
            </el-form-item>
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
            <el-form-item label="Influence analysis target asset ID">
              <el-input v-model="referenceAnalysisTarget" placeholder="UUID of target asset" />
            </el-form-item>
            <el-form-item label="Reference influence analysis">
              <el-alert
                title="Scores are calculated by the server against a target asset."
                type="info"
                :closable="false"
              />
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
            <el-table-column label="Influence" min-width="260">
              <template #default="scope">
                <span v-if="scope.row.influence">
                  C {{ scope.row.influence.composition.toFixed(1) }},
                  S {{ scope.row.influence.semantic.toFixed(1) }},
                  Co {{ scope.row.influence.color.toFixed(1) }},
                  St {{ scope.row.influence.style.toFixed(1) }},
                  M {{ scope.row.influence.material.toFixed(1) }},
                  G {{ scope.row.influence.geometry.toFixed(1) }}
                </span>
                <span v-else>Not analysed</span>
                <el-alert v-if="scope.row.influence?.warning" :title="scope.row.influence.warning" type="warning" :closable="false" />
              </template>
            </el-table-column>
            <el-table-column label="Analyze" width="100">
              <template #default="scope"><el-button link type="primary" @click="analyzeReference(scope.row)">Analyze</el-button></template>
            </el-table-column>
            <el-table-column label="Use" width="90">
              <template #default="scope"><el-button link type="primary" @click="selectReference(scope.row)">Select</el-button></template>
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
                  <el-button v-if="item.status === 'succeeded'" size="small" type="success" @click="selectVariant(item)">Select</el-button>
                  <el-button v-if="item.status === 'succeeded'" size="small" type="danger" @click="rejectVariant(item)">Reject</el-button>
                </div>
                <small v-if="item.model_version">Model version: {{ item.model_version }}</small>
                <el-alert v-if="item.error_message" :title="item.error_message" type="error" :closable="false" />
              </div>
            </el-timeline-item>
          </el-timeline>
        </el-card>

        <el-card class="create-card">
          <template #header>Human creative actions</template>
          <el-space wrap>
            <el-button type="success" @click="approveProject">Approve final result</el-button>
            <span>Create an <strong>idea</strong>, <strong>manual_edit</strong> or <strong>composition</strong> iteration below to create a provenance-backed human action.</span>
          </el-space>
        </el-card>

        <el-card class="create-card">
          <template #header>AI Creative Assistant</template>
          <el-space wrap>
            <el-button
              v-if="assistantTools.some(tool => tool.name === 'suggest_prompt')"
              type="primary"
              :loading="assistantLoading"
              @click="runAssistantRecommendation('suggest_prompt')"
            >
              Suggest prompt
            </el-button>
            <el-button
              v-if="assistantTools.some(tool => tool.name === 'suggest_materials')"
              :loading="assistantLoading"
              @click="runAssistantRecommendation('suggest_materials')"
            >
              Suggest materials
            </el-button>
            <el-button
              v-if="assistantTools.some(tool => tool.name === 'analyze_composition')"
              :loading="assistantLoading"
              :disabled="!lastUploadedAsset"
              @click="runAssistantRecommendation('analyze_composition')"
            >
              Analyze composition
            </el-button>
          </el-space>
          <el-alert
            title="Recommendations are advisory. Nothing is applied until you choose Apply, Edit, or Ignore."
            type="info"
            :closable="false"
            style="margin-top: 12px"
          />
          <el-empty v-if="assistantActions.filter(action => action.kind === 'recommendation').length === 0" description="No AI recommendations yet." />
          <el-timeline v-else style="margin-top: 12px">
            <el-timeline-item
              v-for="action in assistantActions.filter(item => item.kind === 'recommendation')"
              :key="action.id"
              :timestamp="new Date(action.created_at).toLocaleString()"
              placement="top"
            >
              <div v-if="assistantRecommendation(action)" class="assistant-recommendation">
                <div class="iteration-head">
                  <el-tag>{{ action.tool }}</el-tag>
                  <el-tag v-if="action.decision" :type="action.decision === 'apply' ? 'success' : action.decision === 'ignore' ? 'info' : 'warning'">{{ action.decision }}</el-tag>
                  <strong>{{ assistantRecommendation(action)?.recommendation }}</strong>
                </div>
                <p><strong>Reason:</strong> {{ assistantRecommendation(action)?.reason }}</p>
                <p><strong>Affected:</strong> {{ JSON.stringify(assistantRecommendation(action)?.affected_entity) }}</p>
                <p><strong>Expected effect:</strong> {{ assistantRecommendation(action)?.expected_effect }}</p>
                <p><strong>Confidence:</strong> {{ (assistantRecommendation(action)?.confidence ?? action.confidence).toFixed(2) }}</p>
                <el-alert :title="action.uncertainty" type="warning" :closable="false" />
                <details>
                  <summary>Evidence</summary>
                  <pre class="action-payload">{{ JSON.stringify(assistantRecommendation(action)?.evidence, null, 2) }}</pre>
                </details>
                <el-space v-if="!action.decision" wrap>
                  <el-button size="small" type="success" :loading="assistantDecisionLoading === action.id" @click="decideAssistant(action, 'apply')">Apply</el-button>
                  <el-button
                    v-if="action.tool === 'suggest_prompt'"
                    size="small"
                    type="warning"
                    :loading="assistantDecisionLoading === action.id"
                    @click="decideAssistant(action, 'edit')"
                  >
                    Edit
                  </el-button>
                  <el-button size="small" :loading="assistantDecisionLoading === action.id" @click="decideAssistant(action, 'ignore')">Ignore</el-button>
                </el-space>
              </div>
            </el-timeline-item>
          </el-timeline>
        </el-card>

        <el-card class="create-card">
          <template #header>AI Fact Checker</template>
          <el-form label-position="top" @submit.prevent="runFactCheck">
            <el-form-item label="Claim">
              <el-input
                v-model="factClaim"
                maxlength="20000"
                show-word-limit
                placeholder="Enter the factual claim to assess"
              />
            </el-form-item>
            <el-form-item label="Evidence (JSON)">
              <el-input
                v-model="factEvidenceJson"
                type="textarea"
                :rows="8"
                placeholder="Evidence JSON: text, source, assessment (supports/contradicts/unknown), confidence (0..1)"
              />
            </el-form-item>
            <el-button
              type="primary"
              native-type="submit"
              :loading="factLoading"
              :disabled="!assistantTools.some(tool => tool.name === 'fact_check')"
            >
              Check claim
            </el-button>
          </el-form>
          <el-alert
            title="The checker stores claim, evidence, source, confidence and status. It does not invent evidence or independently verify source contents."
            type="info"
            :closable="false"
            style="margin-top: 12px"
          />
          <div v-if="factResult" aria-live="polite" style="margin-top: 12px">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="Status">
                {{ String(factResultValue('status') ?? 'UNKNOWN') }}
              </el-descriptions-item>
              <el-descriptions-item label="Confidence">
                {{ typeof factResultValue('confidence') === 'number' ? Number(factResultValue('confidence')).toFixed(2) : '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Supporting">
                {{ String(factResultValue('supporting_count') ?? 0) }}
              </el-descriptions-item>
              <el-descriptions-item label="Contradicting">
                {{ String(factResultValue('contradicting_count') ?? 0) }}
              </el-descriptions-item>
              <el-descriptions-item label="Unknown">
                {{ String(factResultValue('unknown_count') ?? 0) }}
              </el-descriptions-item>
              <el-descriptions-item label="Method">
                {{ String(factResultValue('method') ?? '—') }}
              </el-descriptions-item>
            </el-descriptions>
            <el-alert
              v-if="factResultValue('status') === 'UNSUPPORTED' || factResultValue('status') === 'PARTIALLY_SUPPORTED' || factResultValue('status') === 'UNKNOWN'"
              :title="String(responseUncertainty(factResultValue('status')))"
              type="warning"
              :closable="false"
              style="margin-top: 12px"
            />
            <el-table v-if="factEvidenceItems().length" :data="factEvidenceItems()" size="small" style="margin-top: 12px">
              <el-table-column prop="source" label="Source" min-width="180" />
              <el-table-column prop="assessment" label="Assessment" width="140" />
              <el-table-column prop="confidence" label="Confidence" width="110" />
              <el-table-column prop="text" label="Evidence" min-width="320" />
            </el-table>
          </div>
        </el-card>

        <el-card class="create-card">
          <template #header>AI Critic / Compare iterations</template>
          <el-space wrap>
            <el-select
              v-model="criticAIterationId"
              filterable
              clearable
              placeholder="Iteration A"
              aria-label="First iteration for critic comparison"
              style="width: 320px"
            >
              <el-option
                v-for="item in iterations"
                :key="`critic-a-${item.id}`"
                :value="item.id"
                :label="`${item.type} · ${item.title || item.id}`"
              />
            </el-select>
            <el-select
              v-model="criticBIterationId"
              filterable
              clearable
              placeholder="Iteration B"
              aria-label="Second iteration for critic comparison"
              style="width: 320px"
            >
              <el-option
                v-for="item in iterations"
                :key="`critic-b-${item.id}`"
                :value="item.id"
                :label="`${item.type} · ${item.title || item.id}`"
              />
            </el-select>
            <el-button
              type="primary"
              :loading="criticLoading"
              :disabled="!criticAIterationId || !criticBIterationId || criticAIterationId === criticBIterationId"
              @click="compareIterations"
            >
              Compare
            </el-button>
          </el-space>
          <el-alert
            title="The critic uses persisted prompts plus deterministic visual/composition descriptors; it does not claim legal similarity or uniqueness."
            type="info"
            :closable="false"
            style="margin-top: 12px"
          />
          <div v-if="criticResult" aria-live="polite">
            <el-descriptions :column="2" border style="margin-top: 12px">
              <el-descriptions-item label="Image A">
                {{ criticAssetAvailable('a') === true ? 'available' : criticAssetAvailable('a') === false ? 'unavailable' : '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Image B">
                {{ criticAssetAvailable('b') === true ? 'available' : criticAssetAvailable('b') === false ? 'unavailable' : '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Visual similarity">
                {{ criticMetric('visual_similarity')?.toFixed(2) ?? '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Composition similarity">
                {{ criticMetric('composition_similarity')?.toFixed(2) ?? '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Semantic proxy">
                {{ criticMetric('semantic_similarity_proxy')?.toFixed(2) ?? '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Style similarity">
                {{ criticMetric('style_similarity')?.toFixed(2) ?? '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Prompt similarity">
                {{ criticPromptSimilarity()?.toFixed(2) ?? '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Algorithm">
                {{ criticAlgorithm() }}
              </el-descriptions-item>
            </el-descriptions>

            <el-alert
              v-if="criticResult?.uncertainty"
              :title="String(criticResult.uncertainty)"
              type="warning"
              :closable="false"
              style="margin-top: 12px"
            />
            <el-timeline v-if="criticObservations().length" style="margin-top: 16px">
              <el-timeline-item
                v-for="(item, index) in criticObservations()"
                :key="`critic-observation-${index}`"
                placement="top"
              >
                <div class="iteration-head">
                  <el-tag size="small">{{ item.dimension }}</el-tag>
                  <strong>{{ item.observation }}</strong>
                </div>
                <p><strong>Reason:</strong> {{ item.reason }}</p>
                <p><strong>Confidence:</strong> {{ typeof item.confidence === 'number' ? item.confidence.toFixed(2) : '—' }}</p>
              </el-timeline-item>
            </el-timeline>
          </div>
        </el-card>

        <el-card class="create-card">
          <template #header>Similarity Check</template>
          <el-space wrap>
            <el-input v-model="similarityTargetAsset" placeholder="Target asset ID (defaults to latest upload)" style="width: 360px" />
            <el-button type="primary" @click="runSimilarityCheck">Run similarity</el-button>
            <el-button v-if="similarityResult && similarityResult.composition_score >= 0.8" @click="requestCompositionMutation">Suggest composition mutations</el-button>
          </el-space>
          <el-descriptions v-if="similarityResult" :column="4" border style="margin-top: 16px">
            <el-descriptions-item label="Visual">{{ similarityResult.visual_score.toFixed(3) }}</el-descriptions-item>
            <el-descriptions-item label="Composition">{{ similarityResult.composition_score.toFixed(3) }}</el-descriptions-item>
            <el-descriptions-item label="Semantic">{{ similarityResult.semantic_score.toFixed(3) }}</el-descriptions-item>
            <el-descriptions-item label="Style">{{ similarityResult.style_score.toFixed(3) }}</el-descriptions-item>
          </el-descriptions>
          <el-alert v-if="similarityResult" title="Similarity analysis is informational and does not establish that all internet sources were checked." type="warning" :closable="false" style="margin-top: 12px" />
        </el-card>

        <el-card v-if="compositionSpec" class="create-card">
          <template #header>Composition Engine</template>
          <el-form label-position="top">
            <el-row :gutter="12">
              <el-col :span="12"><el-form-item label="Focal points (JSON)"><el-input type="textarea" :model-value="JSON.stringify(compositionSpec.focal_points)" @change="setCompositionJSON('focal_points', String($event))" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="Bounding boxes (JSON)"><el-input type="textarea" :model-value="JSON.stringify(compositionSpec.bounding_boxes)" @change="setCompositionJSON('bounding_boxes', String($event))" /></el-form-item></el-col>
              <el-col :span="8"><el-form-item label="Horizon"><el-input-number v-model="compositionSpec.horizon" :min="0" :max="1" :step="0.01" /></el-form-item></el-col>
              <el-col :span="8"><el-form-item label="Camera elevation"><el-input-number v-model="compositionSpec.camera_elevation" :step="0.1" /></el-form-item></el-col>
              <el-col :span="8"><el-form-item label="Perspective"><el-input v-model="compositionSpec.perspective" @change="updateCompositionText('perspective')" /></el-form-item></el-col>
            </el-row>
            <el-space><el-button type="primary" :loading="compositionSaving" @click="saveComposition">Save composition</el-button></el-space>
          </el-form>
          <el-alert v-if="compositionMutation" :title="compositionMutation.suggestions.join(' · ')" type="info" :closable="false" style="margin-top: 12px" />
          <el-space v-if="compositionMutation" style="margin-top: 12px">
            <el-button type="success" :disabled="compositionMutation.status !== 'proposed'" @click="acceptCompositionMutation">Accept → new iteration</el-button>
            <el-button type="danger" :disabled="compositionMutation.status !== 'proposed'" @click="rejectCompositionMutation">Reject</el-button>
          </el-space>
        </el-card>

        <el-card class="create-card">
          <template #header>Material & Texture Libraries</template>
          <el-row :gutter="12">
            <el-col :span="12">
              <el-input v-model="materialSearch" placeholder="Search materials" @keyup.enter="loadLibraries" />
              <el-table :data="materials" size="small" @row-click="chooseMaterial">
                <el-table-column prop="name" label="Material" />
                <el-table-column prop="category" label="Category" />
              </el-table>
              <small v-if="selectedMaterial">Selected material: {{ selectedMaterial }}</small>
            </el-col>
            <el-col :span="12">
              <el-input v-model="textureSearch" placeholder="Search textures" @keyup.enter="loadLibraries" />
              <el-table :data="textures" size="small" @row-click="chooseTexture">
                <el-table-column prop="name" label="Texture" />
                <el-table-column prop="category" label="Category" />
              </el-table>
              <small v-if="selectedTexture">Selected texture: {{ selectedTexture }}</small>
            </el-col>
          </el-row>
        </el-card>

        <el-card class="create-card">
          <template #header>Creation Report</template>
          <el-button type="primary" :loading="exporting" :disabled="!lastUploadedAsset" @click="createCreationReport">Create Creation Report</el-button>
          <el-descriptions v-if="creationExport" :column="1" border style="margin-top: 12px">
            <el-descriptions-item label="Status">{{ creationExport.status }}</el-descriptions-item>
            <el-descriptions-item label="Export ID">{{ creationExport.id }}</el-descriptions-item>
            <el-descriptions-item label="Artifacts">
              <div v-for="(url, name) in creationExport.artifacts" :key="name"><a :href="url" target="_blank" rel="noreferrer">{{ name }}</a></div>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-dialog :model-value="assistantEditAction !== null" title="Edit AI recommendation" width="560px" @update:model-value="(value: boolean) => { if (!value) assistantEditAction = null }">
          <el-input v-model="assistantEditText" type="textarea" :rows="7" maxlength="20000" show-word-limit />
          <template #footer>
            <el-button @click="assistantEditAction = null">
              Cancel
            </el-button>
            <el-button
              type="primary"
              :loading="assistantDecisionLoading === assistantEditAction?.id"
              :disabled="!assistantEditText.trim()"
              @click="submitAssistantEdit"
            >
              Apply edit
            </el-button>
          </template>
        </el-dialog>

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
            <el-form-item :label="form.type === 'manual_edit' ? 'Description (required)' : 'Description'">
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

<style scoped>.recipe-card {
  margin-bottom: 16px;
}

.recipe-field {
  display: grid;
  gap: 6px;
  margin-bottom: 10px;
}

.recipe-field small,
.recipe-description {
  color: #667085;
}

.recipe-field select,
.recipe-field input:not([type='checkbox']) {
  width: 100%;
}

.recipe-parameters {
  display: grid;
  gap: 8px;
}

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
.assistant-recommendation {
  border-inline-start: 3px solid var(--el-color-primary);
  padding-inline-start: 12px;
}

</style>
