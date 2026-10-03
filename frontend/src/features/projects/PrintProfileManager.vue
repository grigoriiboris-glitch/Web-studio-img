<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { printProfilesApi, type PrintProfile, type PrintProfileVersion } from '../../api/client'

const props = defineProps<{ projectId: string }>()
type Orientation = 'portrait' | 'landscape'
type ColorSpace = 'RGB' | 'CMYK'
type OutputFormat = 'png' | 'tiff' | 'pdf'
interface Spec {
  width_mm: number
  height_mm: number
  bleed_mm: number
  safe_zone_mm: number
  dpi: number
  orientation: Orientation
  color_space: ColorSpace
  format: OutputFormat
  crop_marks: boolean
  double_sided: boolean
}
const defaults = (): Spec => ({ width_mm: 63, height_mm: 88, bleed_mm: 3, safe_zone_mm: 3, dpi: 300, orientation: 'portrait', color_space: 'CMYK', format: 'tiff', crop_marks: true, double_sided: false })
const profiles = ref<PrintProfile[]>([])
const versions = ref<PrintProfileVersion[]>([])
const selected = ref<PrintProfile | null>(null)
const name = ref('')
const key = ref('')
const spec = ref<Spec>(defaults())
const editing = ref(false)
const error = ref('')
const info = ref('')

const pixelWidth = computed(() => Math.round(spec.value.width_mm / 25.4 * spec.value.dpi))
const pixelHeight = computed(() => Math.round(spec.value.height_mm / 25.4 * spec.value.dpi))
const totalWidth = computed(() => spec.value.width_mm + spec.value.bleed_mm * 2)
const totalHeight = computed(() => spec.value.height_mm + spec.value.bleed_mm * 2)

function reset() {
  selected.value = null
  versions.value = []
  name.value = ''
  key.value = ''
  spec.value = defaults()
  editing.value = false
  error.value = ''
}
async function load() {
  try { profiles.value = (await printProfilesApi.list(props.projectId)).print_profiles } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
}
async function selectProfile(profile: PrintProfile) {
  try {
    const details = await printProfilesApi.get(props.projectId, profile.id)
    selected.value = profile
    name.value = profile.name
    key.value = profile.key
    spec.value = { ...defaults(), ...(details.version.spec as Partial<Spec>) }
    versions.value = (await printProfilesApi.versions(props.projectId, profile.id)).versions
    editing.value = true
    error.value = ''
  } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
}
function validate() {
  if (!name.value.trim()) return 'Profile name is required.'
  if (!key.value.trim()) return 'Profile key is required.'
  if (spec.value.width_mm <= 0 || spec.value.height_mm <= 0) return 'Physical dimensions must be positive.'
  if (spec.value.bleed_mm < 0 || spec.value.safe_zone_mm < 0) return 'Bleed and safe zone cannot be negative.'
  if (spec.value.dpi < 72 || spec.value.dpi > 1200) return 'DPI must be between 72 and 1200.'
  if (spec.value.safe_zone_mm * 2 >= Math.min(spec.value.width_mm, spec.value.height_mm)) return 'Safe zone is too large for the card.'
  return ''
}
async function saveProfile() {
  const message = validate()
  if (message) { error.value = message; return }
  error.value = ''
  try {
    if (selected.value) {
      const result = await printProfilesApi.update(props.projectId, selected.value.id, { name: name.value.trim(), spec: { ...spec.value } })
      info.value = 'Published print profile v' + result.version.version
    } else {
      const result = await printProfilesApi.create(props.projectId, { key: key.value.trim().toLowerCase(), name: name.value.trim(), spec: { ...spec.value } })
      selected.value = result.print_profile
      info.value = 'Created print profile v1'
    }
    await load()
    if (selected.value) await selectProfile(selected.value)
  } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
}
async function archive() {
  if (!selected.value || !confirm('Archive this print profile?')) return
  try { await printProfilesApi.archive(props.projectId, selected.value.id); reset(); await load() } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
}
function selectVersion(version: PrintProfileVersion) {
  spec.value = { ...defaults(), ...(version.spec as Partial<Spec>) }
}
onMounted(load)
</script>

<template>
  <section class="print-profiles">
    <div class="head">
      <div><h2>Print Profiles</h2><p>Physical production constraints are versioned separately from generation defaults.</p></div>
      <button type="button" @click="reset">New profile</button>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="info" class="info">{{ info }}</p>
    <div class="layout">
      <aside class="list">
        <button v-for="profile in profiles" :key="profile.id" type="button" :class="{ active: selected?.id === profile.id }" @click="selectProfile(profile)">
          <strong>{{ profile.name }}</strong><span>{{ profile.key }} · v{{ profile.current_version }}</span>
        </button>
      </aside>
      <div class="editor">
        <div class="grid">
          <label>Key<input v-model="key" :disabled="!!selected" placeholder="standard-card"></label>
          <label>Name<input v-model="name" placeholder="Standard Card 63×88 mm"></label>
          <label>Width, mm<input v-model.number="spec.width_mm" type="number" min="1" step="0.1"></label>
          <label>Height, mm<input v-model.number="spec.height_mm" type="number" min="1" step="0.1"></label>
          <label>Bleed, mm<input v-model.number="spec.bleed_mm" type="number" min="0" step="0.1"></label>
          <label>Safe zone, mm<input v-model.number="spec.safe_zone_mm" type="number" min="0" step="0.1"></label>
          <label>DPI<input v-model.number="spec.dpi" type="number" min="72" max="1200" step="1"></label>
          <label>Orientation<select v-model="spec.orientation"><option value="portrait">Portrait</option><option value="landscape">Landscape</option></select></label>
          <label>Color space<select v-model="spec.color_space"><option value="CMYK">CMYK</option><option value="RGB">RGB</option></select></label>
          <label>Format<select v-model="spec.format"><option value="tiff">TIFF</option><option value="png">PNG</option><option value="pdf">PDF</option></select></label>
          <label class="check"><input v-model="spec.crop_marks" type="checkbox"> Crop marks</label>
          <label class="check"><input v-model="spec.double_sided" type="checkbox"> Double-sided</label>
        </div>
        <div class="derived">
          <strong>Print output</strong>
          <span>Trim: {{ spec.width_mm }}×{{ spec.height_mm }} mm</span>
          <span>With bleed: {{ totalWidth.toFixed(1) }}×{{ totalHeight.toFixed(1) }} mm</span>
          <span>Raster target: {{ pixelWidth }}×{{ pixelHeight }} px @ {{ spec.dpi }} DPI</span>
        </div>
        <div class="actions">
          <button type="button" class="primary" @click="saveProfile">{{ selected ? 'Publish new version' : 'Create profile' }}</button>
          <button v-if="selected" type="button" @click="archive">Archive</button>
        </div>
        <div v-if="versions.length" class="history">
          <strong>Version history</strong>
          <button v-for="version in versions" :key="version.id" type="button" @click="selectVersion(version)">v{{ version.version }} · {{ new Date(version.created_at).toLocaleString() }}</button>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.print-profiles { margin:18px 0; padding:18px; border:1px solid #ddd; border-radius:12px; background:#fff; }
.head,.actions { display:flex; justify-content:space-between; gap:10px; align-items:center; }
.layout { display:grid; grid-template-columns:220px 1fr; gap:18px; margin-top:12px; }
.list { display:grid; gap:7px; align-content:start; }
.list button { text-align:left; padding:9px; border:1px solid #ddd; border-radius:7px; background:#fff; }
.list button.active { outline:2px solid #222; }
.list span,.list strong { display:block; }
.list span { color:#666; font-size:12px; }
.grid { display:grid; grid-template-columns:repeat(4,minmax(130px,1fr)); gap:10px; }
.grid label { display:flex; flex-direction:column; gap:5px; font-size:12px; }
input,select,button { font:inherit; }
input,select { padding:7px; border:1px solid #ccc; border-radius:6px; }
.check { flex-direction:row !important; align-items:center; padding-top:22px; }
.check input { width:auto; }
.derived { display:flex; gap:14px; flex-wrap:wrap; margin:14px 0; padding:10px; background:#f6f6f6; border-radius:8px; }
.error { color:#900; padding:8px; background:#fee; border-radius:7px; }
.info { color:#174d17; padding:8px; background:#eef8ee; border-radius:7px; }
button { padding:7px 10px; border:1px solid #bbb; border-radius:6px; background:#fff; cursor:pointer; }
button.primary { background:#222; color:#fff; border-color:#222; }
.history { display:grid; gap:6px; margin-top:14px; }
.history button { text-align:left; }
@media (max-width:800px) { .layout,.grid { grid-template-columns:1fr; } }
</style>
