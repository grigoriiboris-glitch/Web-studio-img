<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  assetsApi,
  generationsApi,
  recipesApi,
  variantBoardApi,
  type Generation,
  type Recipe,
} from '../../api/client'
import { parseSpreadsheet, type SpreadsheetRow } from './cardBatchSpreadsheet'

type CardStatus = 'pending' | 'queued' | 'running' | 'succeeded' | 'needs_revision' | 'failed'
interface CardItem {
  id: string
  cardNumber: string
  prompt: string
  sourceRow: number
  status: CardStatus
  generationIds: string[]
  selectedGenerationId?: string
  promptRevision: number
  error?: string
  updatedAt: string
}
interface Batch {
  id: string
  name: string
  sourceFile: string
  sheet: string
  createdAt: string
  cards: CardItem[]
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
const editedPrompt = ref<Record<string, string>>({})
const generations = ref<Record<string, Generation>>({})
const downloads = ref<Record<string, string>>({})

const activeBatch = computed(() => batches.value.find(item => item.id === activeBatchId.value))
const columns = computed(() => rows.value.length ? Object.keys(rows.value[0]) : [])
const activeCards = computed(() => activeBatch.value?.cards ?? [])
const completed = computed(() => activeCards.value.filter(card => card.status === 'succeeded').length)
const selectedCount = computed(() => activeCards.value.filter(card => card.selectedGenerationId).length)
const pendingCards = computed(() => activeCards.value.filter(card => card.status === 'pending' || card.status === 'failed' || card.status === 'needs_revision'))

function save() {
  localStorage.setItem(storageKey.value, JSON.stringify(batches.value))
}
function uid(prefix: string) {
  return prefix + '_' + crypto.randomUUID()
}
function now() { return new Date().toISOString() }
function filenameFor(card: CardItem, generation: Generation, final = false, explicitVersion?: number) {
  const safe = card.cardNumber.replace(/[^a-zA-Z0-9._-]+/g, '_')
  const version = (explicitVersion ?? Math.max(1, card.generationIds.indexOf(generation.id) + 1)).toString().padStart(3, '0')
  return safe + '__card__v' + version + (final ? '__selected' : '') + '.png'
}
function persistCard(card: CardItem) {
  card.updatedAt = now()
  save()
}

async function loadDownload(card: CardItem, generation: Generation, final = false) {
  const source = await variantBoardApi.sources(routeProjectId.value)
  const item = source.sources.find(x => x.generation_id === generation.id)
  if (!item?.asset_id) throw new Error('Asset is not ready for generation ' + generation.id)
  const url = await assetsApi.downloadUrl(routeProjectId.value, item.asset_id)
  downloads.value[generation.id] = url.url
  const response = await fetch(url.url)
  if (!response.ok) throw new Error('Could not download generated asset')
  const blob = await response.blob()
  const href = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.download = filenameFor(card, generation, final)
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
  const generation = await generationsApi.create(
    routeProjectId.value,
    {
      prompt,
      recipe_id: recipeId.value || undefined,
      parameters: {
        card_batch_id: activeBatch.value?.id,
        card_id: card.id,
        card_number: card.cardNumber,
        source_row: card.sourceRow,
        prompt_revision: card.promptRevision,
        asset_filename: filenameFor(card, { id: 'pending' } as Generation, false, version),
        batch_name: activeBatch.value?.name,
        version,
      },
    },
    'card-batch-' + activeBatch.value?.id + '-' + card.id + '-v' + version,
  )
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
      if (card.status === 'succeeded') continue
      try {
        card.status = 'queued'
        persistCard(card)
        await generateCard(card, editedPrompt.value[card.id] || card.prompt)
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

async function regenerate(card: CardItem) {
  error.value = ''
  try {
    const prompt = editedPrompt.value[card.id] || card.prompt
    await generateCard(card, prompt)
  } catch (e) {
    card.status = 'failed'
    card.error = e instanceof Error ? e.message : String(e)
    persistCard(card)
    error.value = card.error
  }
}

function editPrompt(card: CardItem) {
  const value = window.prompt('New prompt', editedPrompt.value[card.id] || card.prompt)
  if (value == null || !value.trim()) return
  editedPrompt.value[card.id] = value.trim()
  card.prompt = value.trim()
  card.promptRevision++
  card.status = 'needs_revision'
  persistCard(card)
}

async function selectGeneration(card: CardItem, generationId: string) {
  card.selectedGenerationId = generationId
  persistCard(card)
  const generation = generations.value[generationId]
  if (generation) {
    try { await loadDownload(card, generation, true) } catch { /* download remains optional */ }
  }
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
  const cols = columns.value
  if (!cols.includes(cardColumn.value)) cardColumn.value = cols[0] || ''
  if (!cols.includes(promptColumn.value)) promptColumn.value = cols.find(c => c !== cardColumn.value) || ''
}

function createBatch() {
  if (!routeProjectId.value || !name.value.trim() || !sheet.value || !cardColumn.value || !promptColumn.value) {
    error.value = 'Project, batch name, sheet, card number and prompt columns are required.'
    return
  }
  const seen = new Set<string>()
  const cards: CardItem[] = []
  for (let i = 0; i < rows.value.length; i++) {
    const row = rows.value[i]
    const cardNumber = String(row[cardColumn.value] ?? '').trim()
    const prompt = String(row[promptColumn.value] ?? '').trim()
    if (!cardNumber && !prompt) continue
    if (!cardNumber || !prompt) throw new Error('Row ' + (i + 2) + ' must contain both card number and prompt')
    if (seen.has(cardNumber)) throw new Error('Duplicate card number: ' + cardNumber)
    seen.add(cardNumber)
    cards.push({
      id: uid('card'),
      cardNumber,
      prompt,
      sourceRow: i + 2,
      status: 'pending',
      generationIds: [],
      promptRevision: 1,
      updatedAt: now(),
    })
  }
  if (!cards.length) throw new Error('No valid card rows found')
  const batch: Batch = {
    id: uid('batch'),
    name: name.value.trim(),
    sourceFile: fileName.value,
    sheet: sheet.value,
    createdAt: now(),
    cards,
  }
  batches.value.unshift(batch)
  activeBatchId.value = batch.id
  name.value = ''
  save()
  info.value = 'Batch created: ' + cards.length + ' cards'
}

async function refreshBatch() {
  if (!activeBatch.value) return
  for (const card of activeBatch.value.cards) {
    for (const id of card.generationIds) {
      try { generations.value[id] = await generationsApi.get(routeProjectId.value, id) } catch { /* keep local state */ }
    }
  }
}

onMounted(async () => {
  try {
    batches.value = JSON.parse(localStorage.getItem(storageKey.value) || '[]') as Batch[]
    activeBatchId.value = batches.value[0]?.id || ''
    recipes.value = (await recipesApi.list(routeProjectId.value)).recipes
    await refreshBatch()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
})
</script>

<template>
  <main class="card-batch">
    <header class="header">
      <div>
        <h1>Card Batch</h1>
        <p>Пакетная генерация карточек без смешивания промтов, вариантов и ассетов.</p>
      </div>
      <button v-if="generating" type="button" @click="stopBatch">Stop queue</button>
      <button v-else type="button" class="primary" :disabled="!activeBatch || !pendingCards.length" @click="runBatch">Generate pending</button>
    </header>

    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="info" class="info">{{ info }}</p>

    <section class="panel setup">
      <div>
        <h2>1. Spreadsheet Import</h2>
        <p>Поддерживаются XLSX, CSV и TSV. Для XLSX листы выбираются отдельно.</p>
      </div>
      <input type="file" accept=".xlsx,.csv,.tsv,text/csv" :disabled="importing" @change="importFile">
      <div v-if="sheets.length" class="mapping">
        <label>Sheet<select v-model="sheet" @change="changeSheet"><option v-for="item in sheets" :key="item" :value="item">{{ item }}</option></select></label>
        <label>Card number column<select v-model="cardColumn"><option v-for="item in columns" :key="item" :value="item">{{ item }}</option></select></label>
        <label>Prompt column<select v-model="promptColumn"><option v-for="item in columns" :key="item" :value="item">{{ item }}</option></select></label>
        <label>Batch name<input v-model="name" placeholder="THE-PRICE-OF-ONE cards"></label>
        <button type="button" class="primary" @click="createBatch">Create Card Batch</button>
      </div>
      <div v-if="rows.length" class="preview">
        <strong>Preview: {{ rows.length }} rows</strong>
        <table><thead><tr><th>{{ cardColumn || 'Card' }}</th><th>{{ promptColumn || 'Prompt' }}</th></tr></thead>
          <tbody><tr v-for="(row, index) in rows.slice(0, 8)" :key="index"><td>{{ row[cardColumn] }}</td><td>{{ row[promptColumn] }}</td></tr></tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <h2>2. Batch history</h2>
      <div class="batch-list">
        <button v-for="batch in batches" :key="batch.id" type="button" :class="{ active: batch.id === activeBatchId }" @click="activeBatchId = batch.id">
          <strong>{{ batch.name }}</strong>
          <span>{{ batch.cards.length }} cards · {{ batch.cards.filter(c => c.status === 'succeeded').length }} generated</span>
        </button>
      </div>
    </section>

    <section v-if="activeBatch" class="panel">
      <div class="toolbar">
        <div><h2>3. Card Workspace — {{ activeBatch.name }}</h2><p>{{ completed }}/{{ activeCards.length }} generated · {{ selectedCount }} selected</p></div>
        <label>Recipe<select v-model="recipeId"><option value="">Provider default</option><option v-for="recipe in recipes" :key="recipe.id" :value="recipe.id">{{ recipe.name }} · v{{ recipe.current_version }}</option></select></label>
        <button type="button" @click="refreshBatch">Refresh</button>
      </div>
      <div class="cards">
        <article v-for="card in activeCards" :key="card.id" class="card" :class="card.status">
          <div class="card-head"><strong>#{{ card.cardNumber }}</strong><span>{{ card.status }}</span></div>
          <textarea v-model="card.prompt" rows="4" @change="card.promptRevision++; card.status = 'needs_revision'; persistCard(card)" />
          <small>Source row {{ card.sourceRow }} · prompt revision {{ card.promptRevision }}</small>
          <div v-if="card.error" class="card-error">{{ card.error }}</div>
          <div class="versions">
            <div v-for="generationId in card.generationIds" :key="generationId" class="version">
              <span>v{{ card.generationIds.indexOf(generationId) + 1 }}</span>
              <span>{{ generations[generationId]?.status || 'queued' }}</span>
              <button v-if="generations[generationId]?.status === 'succeeded'" type="button" @click="selectGeneration(card, generationId)">
                {{ card.selectedGenerationId === generationId ? 'Selected ✓' : 'Select' }}
              </button>
              <button v-if="generations[generationId]?.status === 'succeeded'" type="button" @click="loadDownload(card, generations[generationId], false)">Download named PNG</button>
            </div>
          </div>
          <div class="actions">
            <button type="button" @click="regenerate(card)" :disabled="generating">Regenerate</button>
            <button type="button" @click="editPrompt(card)">Edit & Generate</button>
            <button type="button" @click="openVariantBoard(card)" :disabled="card.generationIds.length < 2">Open Variant Board</button>
          </div>
        </article>
      </div>
    </section>

    <section v-if="activeBatch" class="panel">
      <h2>4. Stable identity & metadata</h2>
      <p>Каждая генерация получает <code>batch_id + card_id + card_number + source_row + prompt_revision + version</code>. Скачивание формирует человекочитаемое имя вида <code>001__card__v001.png</code>; выбранная версия получает суффикс <code>__selected</code>.</p>
      <p>Номер карточки остаётся неизменным при Regenerate и Edit & Generate. История вариантов хранится внутри конкретной карточки и может быть передана в Variant Board.</p>
    </section>
  </main>
</template>

<style scoped>
.card-batch { min-height: 100vh; padding: 24px; background: var(--el-bg-color-page, #f7f8fa); }
.header,.toolbar { display:flex; gap:16px; align-items:flex-start; justify-content:space-between; margin-bottom:18px; }
.panel { margin-bottom:18px; padding:18px; border:1px solid #ddd; border-radius:12px; background:#fff; }
.mapping { display:grid; grid-template-columns:repeat(4,minmax(160px,1fr)); gap:12px; margin-top:14px; align-items:end; }
.mapping label,.toolbar label { display:grid; gap:6px; font-weight:600; }
input,select,textarea { box-sizing:border-box; width:100%; padding:8px; border:1px solid #bbb; border-radius:7px; font:inherit; }
button { padding:8px 11px; border:1px solid #bbb; border-radius:7px; background:#fff; cursor:pointer; }
button:disabled { opacity:.5; cursor:not-allowed; }
.primary { background:#111; color:#fff; }
.error,.card-error { color:#b42318; }
.info { color:#067647; }
.preview { margin-top:16px; overflow:auto; }
table { width:100%; border-collapse:collapse; margin-top:8px; }
th,td { padding:7px; border-bottom:1px solid #eee; text-align:left; vertical-align:top; }
.batch-list { display:grid; gap:8px; grid-template-columns:repeat(auto-fill,minmax(250px,1fr)); }
.batch-list button { display:grid; gap:4px; text-align:left; }
.batch-list button.active { border-color:#409eff; }
.batch-list span,small { color:#667085; }
.cards { display:grid; gap:14px; grid-template-columns:repeat(auto-fill,minmax(330px,1fr)); }
.card { padding:12px; border:1px solid #ddd; border-radius:10px; }
.card.succeeded { border-color:#67c23a; }
.card.failed { border-color:#f56c6c; }
.card.needs_revision { border-color:#e6a23c; }
.card-head { display:flex; justify-content:space-between; margin-bottom:8px; }
.card textarea { margin-bottom:6px; }
.versions { display:grid; gap:6px; margin:12px 0; }
.version { display:grid; grid-template-columns:40px 1fr auto; gap:7px; align-items:center; padding:7px; background:#f8f9fb; border-radius:7px; }
.actions { display:flex; flex-wrap:wrap; gap:6px; }
@media (max-width:900px) { .mapping { grid-template-columns:1fr 1fr; } .header,.toolbar { flex-direction:column; } }
@media (max-width:600px) { .mapping { grid-template-columns:1fr; } }
</style>
