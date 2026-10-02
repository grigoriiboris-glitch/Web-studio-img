<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  assetsApi,
  cardBatchApi,
  generationsApi,
  recipesApi,
  variantBoardApi,
  type Generation,
  type Recipe,
} from '../../api/client'
import { parseSpreadsheet, type SpreadsheetRow } from './cardBatchSpreadsheet'

type CardStatus = 'draft' | 'pending' | 'queued' | 'running' | 'succeeded' | 'needs_revision' | 'failed' | 'finalized'
type RejectReason = 'wrong_composition' | 'wrong_style' | 'wrong_subject' | 'wrong_color' | 'wrong_detail' | 'technical' | 'other'

interface CardItem {
  id: string
  cardNumber: string
  prompt: string
  sourceRow: number
  status: CardStatus
  generationIds: string[]
  selectedGenerationId?: string
  finalizedGenerationId?: string
  promptRevision: number
  recipeId?: string
  referenceIds?: string[]
  rejectReason?: RejectReason
  rejectComment?: string
  nextIterationReason?: string
  archived?: boolean
  error?: string
  updatedAt: string
}

type BatchState = Record<string, unknown> & {
  cards: CardItem[]
  recipeId?: string
  referenceIds?: string[]
  targetWidth?: number
  targetHeight?: number
}

interface Batch {
  id: string
  name: string
  sourceFile: string
  sheet: string
  createdAt: string
  updatedAt?: string
  cards: CardItem[]
  version?: number
  recipeId?: string
  referenceIds?: string[]
  targetWidth?: number
  targetHeight?: number
}

interface ImportDiff {
  added: SpreadsheetRow[]
  changed: Array<{ card: CardItem; row: SpreadsheetRow; newPrompt: string }>
  unchanged: CardItem[]
  removed: CardItem[]
}

const routeProjectId = computed(() => String(location.pathname.match(/\/projects\/([^/]+)/)?.[1] ?? ''))
const storageKey = computed(() => 'web-studio-card-batches:' + routeProjectId.value)
const batches = ref<Batch[]>([])
const activeBatchId = ref('')
const rows = ref<SpreadsheetRow[]>([])
const sheets = ref<string[]>([])
const rowsBySheet = ref<Record<string, SpreadsheetRow[]>>({})
const sheet = ref('')
const cardColumn = ref('')
const promptColumn = ref('')
const name = ref('')
const fileName = ref('')
const importing = ref(false)
const generating = ref(false)
const stopRequested = ref(false)
const error = ref('')
const info = ref('')
const recipes = ref<Recipe[]>([])
const recipeId = ref('')
const referenceIdsText = ref('')
const targetWidth = ref<number | undefined>()
const targetHeight = ref<number | undefined>()
const generations = ref<Record<string, Generation>>({})
const downloads = ref<Record<string, string>>({})
const importMode = ref<'new' | 'diff'>('new')
const importedRows = ref<SpreadsheetRow[]>([])
const importedFileName = ref('')
const rejectedOpen = ref(true)
let saveQueue: Promise<void> = Promise.resolve()

const activeBatch = computed(() => batches.value.find(item => item.id === activeBatchId.value))
const columns = computed(() => rows.value.length ? Object.keys(rows.value[0]) : [])
const activeCards = computed(() => activeBatch.value?.cards.filter(card => !card.archived) ?? [])
const archivedCards = computed(() => activeBatch.value?.cards.filter(card => card.archived) ?? [])
const completed = computed(() => activeCards.value.filter(card => ['succeeded', 'finalized'].includes(card.status)).length)
const selectedCount = computed(() => activeCards.value.filter(card => card.selectedGenerationId).length)
const finalizedCount = computed(() => activeCards.value.filter(card => card.status === 'finalized').length)
const pendingCards = computed(() => activeCards.value.filter(card => ['pending', 'draft', 'failed', 'needs_revision'].includes(card.status)))
const rejectedCards = computed(() => activeCards.value.filter(card => card.rejectReason))
const readyToFinalize = computed(() => activeCards.value.filter(card => Boolean(card.selectedGenerationId) && card.status !== 'finalized' && checklist(card).every(x => x.status !== 'blocked')))

const importDiff = computed<ImportDiff | null>(() => {
  const batch = activeBatch.value
  if (!batch || !importedRows.value.length || !cardColumn.value || !promptColumn.value) return null
  const existing = new Map(batch.cards.filter(c => !c.archived).map(c => [c.cardNumber, c]))
  const incoming = new Set<string>()
  const added: SpreadsheetRow[] = []
  const changed: ImportDiff['changed'] = []
  const unchanged: CardItem[] = []
  for (const row of importedRows.value) {
    const number = String(row[cardColumn.value] ?? '').trim()
    const prompt = String(row[promptColumn.value] ?? '').trim()
    if (!number || !prompt) continue
    incoming.add(number)
    const card = existing.get(number)
    if (!card) added.push(row)
    else if (card.prompt !== prompt) changed.push({ card, row, newPrompt: prompt })
    else unchanged.push(card)
  }
  const removed = batch.cards.filter(c => !c.archived && !incoming.has(c.cardNumber))
  return { added, changed, unchanged, removed }
})

function now() { return new Date().toISOString() }
function uid(prefix: string) { return prefix + '_' + crypto.randomUUID() }
function parsedReferenceIds(text: string) { return text.split(/[\s,]+/).map(x => x.trim()).filter(Boolean) }

function batchState(batch: Batch): BatchState {
  return {
    cards: batch.cards,
    recipeId: batch.recipeId,
    referenceIds: batch.referenceIds,
    targetWidth: batch.targetWidth,
    targetHeight: batch.targetHeight,
  }
}

function save() {
  localStorage.setItem(storageKey.value, JSON.stringify(batches.value))
  const batch = activeBatch.value
  if (!batch?.version) return
  saveQueue = saveQueue.then(async () => {
    const remote = await cardBatchApi.update(routeProjectId.value, batch.id, {
      version: batch.version!,
      source_file: batch.sourceFile,
      sheet: batch.sheet,
      mapping: { card_number_column: cardColumn.value, prompt_column: promptColumn.value },
      state: batchState(batch),
    })
    batch.version = remote.version
    batch.updatedAt = remote.updated_at
  }).catch(e => {
    error.value = e instanceof Error ? e.message : String(e)
  })
}

function persistCard(card: CardItem) {
  card.updatedAt = now()
  save()
}

function filenameFor(card: CardItem, generationId: string, final = false, explicitVersion?: number) {
  const safe = card.cardNumber.replace(/[^a-zA-Z0-9._-]+/g, '_')
  const version = Math.max(1, explicitVersion ?? card.generationIds.indexOf(generationId) + 1).toString().padStart(3, '0')
  return safe + '__card__v' + version + (final ? '__selected' : '') + '.png'
}

function checklist(card: CardItem) {
  const generation = card.selectedGenerationId ? generations.value[card.selectedGenerationId] : undefined
  const params = generation?.final_parameters || generation?.parameters || {}
  const width = Number(params.width ?? params.target_width ?? params['output_width'] ?? 0)
  const height = Number(params.height ?? params.target_height ?? params['output_height'] ?? 0)
  return [
    { key: 'selected', label: 'Selected generation', status: card.selectedGenerationId ? 'passed' : 'blocked' },
    { key: 'success', label: 'Generation succeeded', status: generation?.status === 'succeeded' ? 'passed' : 'blocked' },
    { key: 'prompt', label: 'Prompt revision recorded', status: card.promptRevision > 0 ? 'passed' : 'blocked' },
    { key: 'provenance', label: 'Generation provenance', status: generation?.parameters?.card_batch_id === activeBatch.value?.id ? 'passed' : 'warning' },
    { key: 'dimensions', label: 'Print dimensions', status: (activeBatch.value?.targetWidth && activeBatch.value?.targetHeight) ? (width >= activeBatch.value.targetWidth && height >= activeBatch.value.targetHeight ? 'passed' : 'blocked') : 'warning' },
    { key: 'identity', label: 'Stable card number', status: card.cardNumber ? 'passed' : 'blocked' },
  ] as Array<{ key: string; label: string; status: 'passed' | 'warning' | 'blocked' }>
}

async function loadDownload(card: CardItem, generationId: string, final = false) {
  const source = await variantBoardApi.sources(routeProjectId.value)
  const item = source.sources.find(x => x.generation_id === generationId)
  if (!item?.asset_id) throw new Error('Asset is not ready for generation ' + generationId)
  const url = await assetsApi.downloadUrl(routeProjectId.value, item.asset_id)
  downloads.value[generationId] = url.url
  const response = await fetch(url.url)
  if (!response.ok) throw new Error('Could not download generated asset')
  const blob = await response.blob()
  const href = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.download = filenameFor(card, generationId, final)
  anchor.click()
  setTimeout(() => URL.revokeObjectURL(href), 1000)
}

async function waitForGeneration(card: CardItem, generationId: string) {
  for (let attempt = 0; attempt < 120; attempt++) {
    const generation = await generationsApi.get(routeProjectId.value, generationId)
    generations.value[generationId] = generation
    if (generation.status === 'succeeded') {
      card.status = 'succeeded'
      persistCard(card)
      return generation
    }
    if (generation.status === 'failed' || generation.status === 'cancelled') {
      card.status = 'failed'
      card.error = generation.error_message || generation.status
      persistCard(card)
      return generation
    }
    await new Promise(resolve => setTimeout(resolve, 1500))
  }
  card.status = 'failed'
  card.error = 'Generation timeout'
  persistCard(card)
  return generations.value[generationId]
}

async function generateCard(card: CardItem, prompt = card.prompt) {
  card.status = 'running'
  card.error = undefined
  persistCard(card)
  const version = card.generationIds.length + 1
  const selectedRecipe = card.recipeId || activeBatch.value?.recipeId || recipeId.value || undefined
  const refs = card.referenceIds?.length ? card.referenceIds : activeBatch.value?.referenceIds
  const generation = await generationsApi.create(routeProjectId.value, {
    prompt,
    recipe_id: selectedRecipe,
    reference_ids: refs,
    parameters: {
      card_batch_id: activeBatch.value?.id,
      card_id: card.id,
      card_number: card.cardNumber,
      source_row: card.sourceRow,
      prompt_revision: card.promptRevision,
      asset_filename: filenameFor(card, 'pending', false, version),
      batch_name: activeBatch.value?.name,
      version,
      target_width: activeBatch.value?.targetWidth,
      target_height: activeBatch.value?.targetHeight,
      reference_ids: refs,
      rejection_reason: card.rejectReason,
      rejection_comment: card.rejectComment,
    },
  }, 'card-batch-' + activeBatch.value?.id + '-' + card.id + '-v' + version)
  card.generationIds.push(generation.id)
  generations.value[generation.id] = generation
  persistCard(card)
  await waitForGeneration(card, generation.id)
}

async function runBatch() {
  if (!activeBatch.value || generating.value) return
  generating.value = true
  stopRequested.value = false
  error.value = ''
  try {
    for (const card of activeBatch.value.cards) {
      if (stopRequested.value) break
      if (card.archived || card.status === 'finalized') continue
      if (card.status === 'succeeded' && card.selectedGenerationId) continue
      try {
        card.status = 'queued'
        persistCard(card)
        await generateCard(card, card.prompt)
      } catch (e) {
        card.status = 'failed'
        card.error = e instanceof Error ? e.message : String(e)
        persistCard(card)
      }
    }
  } finally {
    generating.value = false
  }
}

function stopBatch() { stopRequested.value = true }

async function regenerate(card: CardItem, fromReject = false) {
  error.value = ''
  try {
    const prompt = fromReject && card.nextIterationReason
      ? card.prompt + '\n\nRevision requirement: ' + card.nextIterationReason
      : card.prompt
    await generateCard(card, prompt)
  } catch (e) {
    card.status = 'failed'
    card.error = e instanceof Error ? e.message : String(e)
    persistCard(card)
    error.value = card.error
  }
}

function editPrompt(card: CardItem) {
  const value = window.prompt('New prompt', card.prompt)
  if (value == null || !value.trim()) return
  card.prompt = value.trim()
  card.promptRevision++
  card.status = 'needs_revision'
  persistCard(card)
}

function rejectCard(card: CardItem) {
  const reason = window.prompt('Reject reason: wrong_composition | wrong_style | wrong_subject | wrong_color | wrong_detail | technical | other', card.rejectReason || '')
  if (!reason) return
  const allowed: RejectReason[] = ['wrong_composition','wrong_style','wrong_subject','wrong_color','wrong_detail','technical','other']
  if (!allowed.includes(reason as RejectReason)) {
    error.value = 'Unknown reject reason'
    return
  }
  card.rejectReason = reason as RejectReason
  card.rejectComment = window.prompt('Reject comment (optional)', card.rejectComment || '') || undefined
  card.nextIterationReason = card.rejectComment || reason.replaceAll('_', ' ')
  card.status = 'needs_revision'
  card.finalizedGenerationId = undefined
  persistCard(card)
}

function clearReject(card: CardItem) {
  card.rejectReason = undefined
  card.rejectComment = undefined
  card.nextIterationReason = undefined
  persistCard(card)
}

async function selectGeneration(card: CardItem, generationId: string) {
  card.selectedGenerationId = generationId
  card.status = 'succeeded'
  card.finalizedGenerationId = undefined
  persistCard(card)
}

function finalizeCard(card: CardItem) {
  const checks = checklist(card)
  if (checks.some(x => x.status === 'blocked')) {
    error.value = 'Card #' + card.cardNumber + ' is blocked by the production checklist.'
    return
  }
  card.finalizedGenerationId = card.selectedGenerationId
  card.status = 'finalized'
  persistCard(card)
}

function unfinalizeCard(card: CardItem) {
  card.finalizedGenerationId = undefined
  card.status = card.selectedGenerationId ? 'succeeded' : 'needs_revision'
  persistCard(card)
}

async function openVariantBoard(card: CardItem) {
  const ids = card.generationIds.filter(id => generations.value[id]?.status === 'succeeded')
  if (ids.length < 2) {
    error.value = 'Generate at least two versions of this card before opening Variant Board.'
    return
  }
  try {
    const set = await variantBoardApi.createSet(routeProjectId.value, {
      name: 'Card ' + card.cardNumber + ' — ' + (activeBatch.value?.name || 'Batch'),
      generation_ids: ids,
    })
    window.location.href = '/projects/' + routeProjectId.value + '/variants?set=' + encodeURIComponent(set.variant_set.id)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

async function importFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  importing.value = true
  error.value = ''
  try {
    const parsed = await parseSpreadsheet(file)
    sheets.value = parsed.sheets
    rowsBySheet.value = parsed.rowsBySheet
    sheet.value = parsed.sheets[0] || ''
    rows.value = parsed.rowsBySheet[sheet.value] || []
    fileName.value = file.name
    cardColumn.value = columns.value.find(c => /card|number|номер/i.test(c)) || columns.value[0] || ''
    promptColumn.value = columns.value.find(c => /prompt|промт/i.test(c)) || columns.value.find(c => c !== cardColumn.value) || ''
    importedRows.value = rows.value
    importedFileName.value = file.name
    importMode.value = activeBatch.value ? 'diff' : 'new'
    info.value = 'Loaded ' + rows.value.length + ' rows'
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    importing.value = false
    input.value = ''
  }
}

function changeSheet() {
  rows.value = rowsBySheet.value[sheet.value] || []
  importedRows.value = rows.value
  const cols = columns.value
  if (!cols.includes(cardColumn.value)) cardColumn.value = cols[0] || ''
  if (!cols.includes(promptColumn.value)) promptColumn.value = cols.find(c => c !== cardColumn.value) || ''
}

function cardFromRow(row: SpreadsheetRow, sourceRow: number): CardItem {
  return {
    id: uid('card'),
    cardNumber: String(row[cardColumn.value] ?? '').trim(),
    prompt: String(row[promptColumn.value] ?? '').trim(),
    sourceRow,
    status: 'pending',
    generationIds: [],
    promptRevision: 1,
    updatedAt: now(),
  }
}

async function createBatch() {
  if (!routeProjectId.value || !name.value.trim() || !sheet.value || !cardColumn.value || !promptColumn.value) {
    error.value = 'Project, batch name, sheet, card number and prompt columns are required.'
    return
  }
  const seen = new Set<string>()
  const cards: CardItem[] = []
  for (let i = 0; i < rows.value.length; i++) {
    const card = cardFromRow(rows.value[i], i + 2)
    if (!card.cardNumber && !card.prompt) continue
    if (!card.cardNumber || !card.prompt) throw new Error('Row ' + (i + 2) + ' must contain both card number and prompt')
    if (seen.has(card.cardNumber)) throw new Error('Duplicate card number: ' + card.cardNumber)
    seen.add(card.cardNumber)
    cards.push(card)
  }
  if (!cards.length) throw new Error('No valid card rows found')
  const batch: Batch = {
    id: uid('batch'),
    name: name.value.trim(),
    sourceFile: fileName.value,
    sheet: sheet.value,
    createdAt: now(),
    cards,
    recipeId: recipeId.value || undefined,
    referenceIds: parsedReferenceIds(referenceIdsText.value),
    targetWidth: targetWidth.value,
    targetHeight: targetHeight.value,
  }
  try {
    const remote = await cardBatchApi.create(routeProjectId.value, {
      name: batch.name,
      source_file: batch.sourceFile,
      sheet: batch.sheet,
      mapping: { card_number_column: cardColumn.value, prompt_column: promptColumn.value },
      state: batchState(batch),
    })
    batch.id = remote.id
    batch.version = remote.version
    batch.createdAt = remote.created_at
    batch.updatedAt = remote.updated_at
    batches.value.unshift(batch)
    activeBatchId.value = batch.id
    name.value = ''
    save()
    info.value = 'Batch created: ' + cards.length + ' cards'
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

function applyImportDiff() {
  const batch = activeBatch.value
  const diff = importDiff.value
  if (!batch || !diff) return
  for (const item of diff.changed) {
    item.card.prompt = item.newPrompt
    item.card.promptRevision++
    item.card.status = 'needs_revision'
    item.card.error = undefined
    item.card.updatedAt = now()
  }
  for (const row of diff.added) {
    const number = String(row[cardColumn.value] ?? '').trim()
    const prompt = String(row[promptColumn.value] ?? '').trim()
    if (!number || !prompt) continue
    batch.cards.push(cardFromRow(row, rows.value.indexOf(row) + 2))
  }
  for (const card of diff.removed) card.archived = true
  batch.sourceFile = importedFileName.value
  batch.sheet = sheet.value
  save()
  info.value = 'Import reconciled: ' + diff.added.length + ' added, ' + diff.changed.length + ' changed, ' + diff.removed.length + ' archived.'
  importedRows.value = []
}

function selectBatch(id: string) {
  activeBatchId.value = id
  const batch = batches.value.find(item => item.id === id)
  if (!batch) return
  recipeId.value = batch.recipeId || ''
  referenceIdsText.value = (batch.referenceIds || []).join(', ')
  targetWidth.value = batch.targetWidth
  targetHeight.value = batch.targetHeight
}

function restoreArchived(card: CardItem) {
  card.archived = false
  card.status = card.selectedGenerationId ? 'succeeded' : 'draft'
  persistCard(card)
}

function updateBatchSettings() {
  if (!activeBatch.value) return
  activeBatch.value.recipeId = recipeId.value || undefined
  activeBatch.value.referenceIds = parsedReferenceIds(referenceIdsText.value)
  activeBatch.value.targetWidth = targetWidth.value
  activeBatch.value.targetHeight = targetHeight.value
  save()
}

function setCardRecipe(card: CardItem, value: string) {
  card.recipeId = value || undefined
  persistCard(card)
}

function setCardReferences(card: CardItem, value: string) {
  card.referenceIds = parsedReferenceIds(value)
  persistCard(card)
}

async function refreshBatch() {
  if (!activeBatch.value) return
  for (const card of activeBatch.value.cards) {
    for (const id of card.generationIds) {
      try { generations.value[id] = await generationsApi.get(routeProjectId.value, id) } catch { /* keep local state */ }
    }
  }
}

function manifest() {
  const batch = activeBatch.value
  if (!batch) return null
  return {
    schema: 'web-studio/card-batch-manifest/v1',
    batchId: batch.id,
    projectId: routeProjectId.value,
    name: batch.name,
    sourceFile: batch.sourceFile,
    exportedAt: now(),
    cards: batch.cards.filter(c => c.status === 'finalized' && c.finalizedGenerationId).map(c => ({
      cardNumber: c.cardNumber,
      cardId: c.id,
      generationId: c.finalizedGenerationId,
      assetFilename: filenameFor(c, c.finalizedGenerationId!, true),
      promptRevision: c.promptRevision,
      prompt: c.prompt,
      recipeId: c.recipeId || batch.recipeId,
      referenceIds: c.referenceIds?.length ? c.referenceIds : batch.referenceIds || [],
      sourceRow: c.sourceRow,
      rejectReason: c.rejectReason,
    })),
  }
}

function downloadManifest() {
  const data = manifest()
  if (!data) return
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const href = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.download = activeBatch.value!.name.replace(/[^a-zA-Z0-9._-]+/g, '_') + '__manifest.json'
  anchor.click()
  setTimeout(() => URL.revokeObjectURL(href), 1000)
}

async function exportBatch() {
  const batch = activeBatch.value
  if (!batch) return
  const cards = batch.cards.filter(c => c.status === 'finalized' && c.finalizedGenerationId)
  if (!cards.length) {
    error.value = 'Finalize at least one card before export.'
    return
  }
  downloadManifest()
  for (const card of cards) {
    try { await loadDownload(card, card.finalizedGenerationId!, true) } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    }
    await new Promise(resolve => setTimeout(resolve, 250))
  }
}

onMounted(async () => {
  try {
    const remote = await cardBatchApi.list(routeProjectId.value)
    batches.value = remote.batches.map(item => {
      const state = item.state as unknown as BatchState
      return {
        id: item.id,
        name: item.name,
        sourceFile: item.source_file,
        sheet: item.sheet,
        createdAt: item.created_at,
        updatedAt: item.updated_at,
        version: item.version,
        cards: Array.isArray(state.cards) ? state.cards : [],
        recipeId: state.recipeId,
        referenceIds: state.referenceIds,
        targetWidth: state.targetWidth,
        targetHeight: state.targetHeight,
      }
    })
    if (!batches.value.length) {
      const legacy = JSON.parse(localStorage.getItem(storageKey.value) || '[]') as Batch[]
      for (const item of legacy) {
        const created = await cardBatchApi.create(routeProjectId.value, {
          name: item.name,
          source_file: item.sourceFile,
          sheet: item.sheet,
          mapping: {},
          state: batchState(item),
        })
        item.id = created.id
        item.version = created.version
        item.createdAt = created.created_at
        item.updatedAt = created.updated_at
        batches.value.push(item)
      }
    }
    activeBatchId.value = batches.value[0]?.id || ''
    const first = batches.value[0]
    if (first) {
      recipeId.value = first.recipeId || ''
      referenceIdsText.value = (first.referenceIds || []).join(', ')
      targetWidth.value = first.targetWidth
      targetHeight.value = first.targetHeight
    }
    const remoteFirst = remote.batches[0]
    cardColumn.value = String(remoteFirst?.mapping.card_number_column ?? '')
    promptColumn.value = String(remoteFirst?.mapping.prompt_column ?? '')
    localStorage.setItem(storageKey.value, JSON.stringify(batches.value))
    recipes.value = (await recipesApi.list(routeProjectId.value)).recipes
    await refreshBatch()
  } catch (e) {
    batches.value = JSON.parse(localStorage.getItem(storageKey.value) || '[]') as Batch[]
    activeBatchId.value = batches.value[0]?.id || ''
    try { recipes.value = (await recipesApi.list(routeProjectId.value)).recipes } catch { /* keep empty */ }
    await refreshBatch()
    error.value = e instanceof Error ? e.message : String(e)
  }
})
</script>

<template>
  <main class="card-batch">
    <header class="header">
      <div>
        <h1>Card Batch</h1>
        <p>Production pipeline: spreadsheet → card → generation → review → finalize → export.</p>
      </div>
      <div class="header-actions">
        <button v-if="generating" type="button" @click="stopBatch">Stop queue</button>
        <button v-else type="button" class="primary" :disabled="!activeBatch || !pendingCards.length" @click="runBatch">Generate pending</button>
        <button type="button" :disabled="!activeBatch" @click="exportBatch">Export finalized</button>
      </div>
    </header>

    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="info" class="info">{{ info }}</p>

    <section class="panel dashboard" v-if="activeBatch">
      <div><strong>{{ activeCards.length }}</strong><span>cards</span></div>
      <div><strong>{{ completed }}</strong><span>generated</span></div>
      <div><strong>{{ selectedCount }}</strong><span>selected</span></div>
      <div><strong>{{ finalizedCount }}</strong><span>finalized</span></div>
      <div><strong>{{ rejectedCards.length }}</strong><span>rejected</span></div>
      <div><strong>{{ pendingCards.length }}</strong><span>needs work</span></div>
      <div><strong>{{ archivedCards.length }}</strong><span>archived</span></div>
    </section>

    <section class="panel setup">
      <div>
        <h2>1. Spreadsheet Import / Re-import</h2>
        <p>Новый файл создаёт Batch, повторный импорт сравнивается по стабильному <code>cardNumber</code>.</p>
      </div>
      <input type="file" accept=".xlsx,.csv,.tsv,text/csv" :disabled="importing" @change="importFile">
      <div v-if="sheets.length" class="mapping">
        <label>Sheet<select v-model="sheet" @change="changeSheet"><option v-for="item in sheets" :key="item" :value="item">{{ item }}</option></select></label>
        <label>Card number<select v-model="cardColumn"><option v-for="item in columns" :key="item" :value="item">{{ item }}</option></select></label>
        <label>Prompt<select v-model="promptColumn"><option v-for="item in columns" :key="item" :value="item">{{ item }}</option></select></label>
        <label v-if="!activeBatch || importMode === 'new'">Batch name<input v-model="name" placeholder="THE-PRICE-OF-ONE cards"></label>
        <button v-if="!activeBatch || importMode === 'new'" type="button" class="primary" @click="createBatch">Create Card Batch</button>
        <button v-else type="button" class="primary" @click="applyImportDiff">Apply diff</button>
      </div>
      <div v-if="importDiff" class="diff">
        <strong>Re-import diff</strong>
        <span>{{ importDiff.added.length }} added · {{ importDiff.changed.length }} changed · {{ importDiff.unchanged.length }} unchanged · {{ importDiff.removed.length }} archived</span>
        <ul><li v-for="item in importDiff.changed" :key="item.card.id">#{{ item.card.cardNumber }} prompt changed → revision {{ item.card.promptRevision + 1 }}</li></ul>
      </div>
      <div v-if="rows.length" class="preview">
        <strong>Preview: {{ rows.length }} rows</strong>
        <table><thead><tr><th>{{ cardColumn || 'Card' }}</th><th>{{ promptColumn || 'Prompt' }}</th></tr></thead>
          <tbody><tr v-for="(row, index) in rows.slice(0, 8)" :key="index"><td>{{ row[cardColumn] }}</td><td>{{ row[promptColumn] }}</td></tr></tbody>
        </table>
      </div>
    </section>

    <section class="panel" v-if="activeBatch">
      <h2>2. Production settings</h2>
      <div class="settings">
        <label>Batch Recipe<select v-model="recipeId" @change="updateBatchSettings"><option value="">Provider default</option><option v-for="recipe in recipes" :key="recipe.id" :value="recipe.id">{{ recipe.name }} · v{{ recipe.current_version }}</option></select></label>
        <label>Reference asset IDs<input :value="referenceIdsText" placeholder="asset/reference IDs, comma separated" @change="referenceIdsText = ($event.target as HTMLInputElement).value; updateBatchSettings"></label>
        <label>Target width<input v-model.number="targetWidth" type="number" min="1" @change="updateBatchSettings"></label>
        <label>Target height<input v-model.number="targetHeight" type="number" min="1" @change="updateBatchSettings"></label>
      </div>
      <p class="muted">Card-level Recipe and Reference override batch defaults. Generation provenance stores the resolved values.</p>
    </section>

    <section class="panel">
      <h2>3. Batch history</h2>
      <div class="batch-list">
        <button v-for="batch in batches" :key="batch.id" type="button" :class="{ active: batch.id === activeBatchId }" @click="selectBatch(batch.id)">
          <strong>{{ batch.name }}</strong>
          <span>{{ batch.cards.filter(c => !c.archived).length }} cards · {{ batch.cards.filter(c => c.status === 'finalized').length }} finalized</span>
        </button>
      </div>
    </section>

    <section v-if="activeBatch" class="panel">
      <div class="toolbar">
        <div><h2>4. Card Workspace</h2><p>{{ completed }}/{{ activeCards.length }} generated · {{ finalizedCount }} finalized</p></div>
        <button type="button" @click="refreshBatch">Refresh</button>
      </div>

      <div class="cards">
        <article v-for="card in activeCards" :key="card.id" class="card" :class="card.status">
          <div class="card-head"><strong>#{{ card.cardNumber }}</strong><span>{{ card.status }}</span></div>
          <textarea v-model="card.prompt" rows="4" @change="card.promptRevision++; card.status = 'needs_revision'; persistCard(card)" />
          <small>Source row {{ card.sourceRow }} · prompt revision {{ card.promptRevision }}</small>

          <div class="card-settings">
            <label>Recipe<select :value="card.recipeId || ''" @change="setCardRecipe(card, ($event.target as HTMLSelectElement).value)"><option value="">Batch default</option><option v-for="recipe in recipes" :key="recipe.id" :value="recipe.id">{{ recipe.name }} · v{{ recipe.current_version }}</option></select></label>
            <label>References<input :value="(card.referenceIds || []).join(', ')" placeholder="override reference IDs" @change="setCardReferences(card, ($event.target as HTMLInputElement).value)"></label>
          </div>

          <div v-if="card.rejectReason" class="reject-box">
            <strong>Rejected: {{ card.rejectReason }}</strong>
            <span>{{ card.rejectComment || card.nextIterationReason }}</span>
            <button type="button" @click="regenerate(card, true)">Regenerate with correction</button>
            <button type="button" @click="clearReject(card)">Clear rejection</button>
          </div>
          <div v-if="card.error" class="card-error">{{ card.error }}</div>

          <div class="versions">
            <div v-for="generationId in card.generationIds" :key="generationId" class="version">
              <span>v{{ card.generationIds.indexOf(generationId) + 1 }}</span>
              <span>{{ generations[generationId]?.status || 'queued' }}</span>
              <span v-if="card.selectedGenerationId === generationId">selected</span>
              <span v-if="card.finalizedGenerationId === generationId">finalized</span>
              <button v-if="generations[generationId]?.status === 'succeeded'" type="button" @click="selectGeneration(card, generationId)">Select</button>
              <button v-if="generations[generationId]?.status === 'succeeded'" type="button" @click="loadDownload(card, generationId, false)">Download</button>
            </div>
          </div>

          <div class="checklist">
            <strong>Production checklist</strong>
            <span v-for="check in checklist(card)" :key="check.key" :class="check.status">● {{ check.label }}</span>
          </div>

          <div class="actions">
            <button type="button" @click="regenerate(card)" :disabled="generating">Regenerate</button>
            <button type="button" @click="editPrompt(card)">Edit prompt</button>
            <button type="button" @click="rejectCard(card)">Reject</button>
            <button type="button" @click="openVariantBoard(card)" :disabled="card.generationIds.length < 2">Variant Board</button>
            <button v-if="card.status !== 'finalized'" type="button" class="primary" @click="finalizeCard(card)" :disabled="!card.selectedGenerationId">Finalize</button>
            <button v-else type="button" @click="unfinalizeCard(card)">Reopen</button>
          </div>
        </article>
      </div>
    </section>

    <section v-if="activeBatch && archivedCards.length" class="panel">
      <h2>Archived by re-import</h2>
      <div class="archived"><span v-for="card in archivedCards" :key="card.id">#{{ card.cardNumber }} <button type="button" @click="restoreArchived(card)">Restore</button></span></div>
    </section>

    <section v-if="activeBatch && rejectedCards.length" class="panel">
      <h2 @click="rejectedOpen = !rejectedOpen">Reject → next iteration <small>{{ rejectedOpen ? 'hide' : 'show' }}</small></h2>
      <div v-if="rejectedOpen" class="reject-summary">
        <div v-for="reason in ['wrong_composition','wrong_style','wrong_subject','wrong_color','wrong_detail','technical','other']" :key="reason">
          <strong>{{ reason }}</strong><span>{{ rejectedCards.filter(c => c.rejectReason === reason).length }}</span>
        </div>
      </div>
    </section>

    <section v-if="activeBatch" class="panel export">
      <h2>6. Final export</h2>
      <p>Экспортирует только <code>finalized</code> карточки, сохраняет стабильные имена и manifest с provenance. Selected и finalized — разные состояния.</p>
      <button type="button" class="primary" @click="exportBatch">Export finalized cards + manifest</button>
      <button type="button" @click="downloadManifest">Manifest only</button>
    </section>
  </main>
</template>

<style scoped>
.card-batch { min-height:100vh; padding:24px; background:var(--el-bg-color-page,#f7f8fa); }
.header,.toolbar { display:flex; gap:16px; align-items:flex-start; justify-content:space-between; margin-bottom:18px; }
.header-actions { display:flex; gap:8px; flex-wrap:wrap; }
.panel { margin-bottom:18px; padding:18px; border:1px solid #ddd; border-radius:12px; background:#fff; }
.mapping,.settings { display:grid; grid-template-columns:repeat(4,minmax(160px,1fr)); gap:12px; margin-top:14px; align-items:end; }
.mapping label,.settings label,.card-settings label { display:flex; flex-direction:column; gap:6px; font-size:13px; }
input,select,textarea { width:100%; box-sizing:border-box; padding:8px; border:1px solid #ccc; border-radius:7px; font:inherit; }
button { padding:8px 12px; border:1px solid #bbb; border-radius:7px; background:#fff; cursor:pointer; }
button.primary { background:#222; color:#fff; border-color:#222; }
button:disabled { opacity:.5; cursor:not-allowed; }
.error { padding:10px; background:#fee; color:#900; border-radius:8px; }
.info { padding:10px; background:#eef8ee; color:#174d17; border-radius:8px; }
.muted,small { color:#666; }
.preview { margin-top:14px; overflow:auto; }
table { width:100%; border-collapse:collapse; margin-top:8px; }
th,td { border-bottom:1px solid #eee; text-align:left; padding:7px; }
.dashboard { display:grid; grid-template-columns:repeat(7,1fr); gap:10px; }
.dashboard div { padding:12px; background:#f5f5f5; border-radius:8px; text-align:center; }
.dashboard strong,.dashboard span { display:block; }
.dashboard strong { font-size:22px; }
.batch-list { display:flex; gap:8px; flex-wrap:wrap; }
.batch-list button { display:flex; flex-direction:column; align-items:flex-start; gap:3px; }
.batch-list button.active { outline:2px solid #222; }
.cards { display:grid; grid-template-columns:repeat(auto-fill,minmax(330px,1fr)); gap:14px; }
.card { border:1px solid #ddd; border-radius:10px; padding:12px; }
.card-head,.version,.actions,.reject-box { display:flex; gap:8px; align-items:center; flex-wrap:wrap; }
.card-head { justify-content:space-between; margin-bottom:8px; }
.card.running { border-color:#999; }
.card.failed { border-color:#c66; }
.card.finalized { border-color:#494; }
.card-settings { display:grid; gap:8px; margin:10px 0; }
.versions { display:grid; gap:6px; margin-top:10px; }
.version { justify-content:space-between; padding:6px; background:#f7f7f7; border-radius:6px; }
.checklist { display:grid; gap:3px; margin:10px 0; font-size:12px; }
.checklist .passed { color:#176b25; }
.checklist .warning { color:#8a6500; }
.checklist .blocked { color:#a00; }
.reject-box { padding:9px; margin:8px 0; background:#fff5df; border-radius:7px; }
.reject-box span { flex:1; }
.card-error { color:#a00; margin:8px 0; }
.actions { margin-top:10px; }
.diff { margin-top:12px; padding:10px; background:#f5f7ff; border-radius:8px; display:grid; gap:5px; }
.archived { display:flex; gap:8px; flex-wrap:wrap; }
.archived span { padding:6px; background:#f3f3f3; border-radius:6px; }
.reject-summary { display:grid; grid-template-columns:repeat(4,1fr); gap:8px; }
.reject-summary div { display:flex; justify-content:space-between; padding:8px; background:#f5f5f5; border-radius:6px; }
.export { display:flex; gap:10px; align-items:center; flex-wrap:wrap; }
code { background:#f1f1f1; padding:1px 4px; border-radius:4px; }
@media (max-width:900px) { .mapping,.settings,.dashboard { grid-template-columns:1fr 1fr; } }
@media (max-width:600px) { .mapping,.settings,.dashboard { grid-template-columns:1fr; } }
</style>
