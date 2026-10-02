<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { assistantApi, type ComfyFlowPlan } from '../../api/client'

type Validation = {
  compatible: boolean
  errors: string[]
  warnings: string[]
  referenced_nodes: string[]
  referenced_models: string[]
}

type ArtistStep = {
  id: string
  icon: string
  title: string
  description: string
  enabled: boolean
}

const props = defineProps<{
  projectId: string
  plan: ComfyFlowPlan
  inputAssets?: Record<string, string>
}>()

const emit = defineEmits(['update:plan', 'run', 'save'])

const validation = ref<Validation | null>(null)
const validating = ref(false)
const fixing = ref(false)
const pendingFix = ref<ComfyFlowPlan | null>(null)
const error = ref('')
const technicalOpen = ref(false)
const draggedStep = ref<string | null>(null)

const stepCatalog: ArtistStep[] = [
  { id: 'sketch', icon: '✏️', title: 'Скетч', description: 'Сохраняем композицию и важные линии.', enabled: true },
  { id: 'reference', icon: '🎨', title: 'Цветной референс', description: 'Берём цвета, материалы и настроение.', enabled: true },
  { id: 'structure', icon: '🧩', title: 'Сохраняем форму', description: 'Не даём генерации потерять основную конструкцию изображения.', enabled: true },
  { id: 'final', icon: '🖼️', title: 'Финальная иллюстрация', description: 'Получаем полноценное цветное изображение.', enabled: true },
]

const steps = ref<ArtistStep[]>(stepCatalog.map(step => ({ ...step })))

const inputLabels: Record<string, string> = {
  sketch: 'Скетч',
  color_reference: 'Цветной референс',
  color_reference_image: 'Цветной референс',
  reference: 'Референс',
  input_image: 'Исходное изображение',
  mask_image: 'Маска',
}

const visibleInputs = computed(() => props.plan.inputs.filter(input => input.required || props.inputAssets?.[input.id]))
const readiness = computed(() => {
  const missing = props.plan.inputs
    .filter(input => input.required)
    .filter(input => !props.inputAssets?.[input.id])
  if (missing.length) return { kind: 'warning' as const, title: 'Нужно добавить материалы', text: missing.map(input => input.label || inputLabels[input.id] || input.id).join(', ') }
  if (validation.value?.compatible) return { kind: 'success' as const, title: 'Всё готово', text: 'Можно создавать изображение.' }
  if (validation.value && validation.value.errors.length) return { kind: 'error' as const, title: 'Нужно исправление', text: 'Нажмите «Исправить автоматически».' }
  return { kind: 'info' as const, title: 'Проверим перед запуском', text: 'Студия сама проверит техническую часть.' }
})

const parameterDefinitions = computed(() => Object.entries(props.plan.parameters ?? {}).filter(([key]) =>
  ['reference_strength', 'color_strength', 'line_strength', 'denoise', 'strength', 'creativity'].includes(key.toLowerCase()),
))

function parameterValue(key: string): number {
  const value = props.plan.parameters?.[key]
  return typeof value === 'number' ? value : 0.7
}

function setParameter(key: string, value: number) {
  emit('update:plan', {
    ...props.plan,
    parameters: { ...(props.plan.parameters ?? {}), [key]: value },
  })
  validation.value = null
}

function inputLabel(input: { id: string; label?: string }) {
  return input.label || inputLabels[input.id] || input.id.split('_').join(' ')
}

function moveStep(from: number, to: number) {
  if (to < 0 || to >= steps.value.length || from === to) return
  const next = [...steps.value]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  steps.value = next
}

function onDrop(index: number) {
  if (!draggedStep.value) return
  const from = steps.value.findIndex(step => step.id === draggedStep.value)
  moveStep(from, index)
  draggedStep.value = null
}

function resetSteps() {
  steps.value = stepCatalog.map(step => ({ ...step }))
}

function hasRequiredAssets() {
  return props.plan.inputs.filter(input => input.required).every(input => Boolean(props.inputAssets?.[input.id]))
}

async function validateFlow() {
  validating.value = true
  error.value = ''
  try {
    const response = await assistantApi.execute<Validation>(
      props.projectId,
      'validate_comfy_flow',
      { workflow: props.plan.workflow },
    )
    validation.value = response.result
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Не удалось проверить Flow'
    validation.value = null
  } finally {
    validating.value = false
  }
}

async function fixWithAI() {
  const errors = validation.value?.errors ?? []
  if (!errors.length) {
    error.value = 'Сначала нажмите «Проверить готовность».'
    return
  }
  fixing.value = true
  error.value = ''
  try {
    const response = await assistantApi.execute<ComfyFlowPlan>(
      props.projectId,
      'create_comfy_flow',
      {
        task: [
          props.plan.prompt || props.plan.description || 'Repair this ComfyUI workflow',
          'Repair the supplied workflow instead of redesigning it.',
          'Keep the artist intent and current flow. Only fix what is necessary.',
          'Validation errors:',
          ...errors,
        ].join('\n'),
        current_workflow: props.plan.workflow,
        validation_errors: errors,
        inputs: props.plan.inputs,
        parameters: props.plan.parameters,
      },
    )
    pendingFix.value = response.result
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Не удалось исправить Flow'
  } finally {
    fixing.value = false
  }
}

function applyFix() {
  if (!pendingFix.value) return
  emit('update:plan', pendingFix.value)
  pendingFix.value = null
  validation.value = null
}

function rejectFix() {
  pendingFix.value = null
}

watch(() => props.plan.workflow, () => {
  validation.value = null
}, { deep: true })
</script>

<template>
  <div class="artist-flow">
    <div class="artist-intro">
      <div>
        <div class="artist-kicker">Творческий процесс</div>
        <h3>{{ plan.name || 'Создание иллюстрации' }}</h3>
        <p>{{ plan.description || 'Соберите процесс из понятных творческих шагов. Техническая часть настраивается автоматически.' }}</p>
      </div>
      <el-tag :type="readiness.kind">{{ readiness.title }}</el-tag>
    </div>

    <el-alert
      :title="readiness.title"
      :description="readiness.text"
      :type="readiness.kind"
      :closable="false"
      class="readiness"
    />

    <section class="steps">
      <div class="section-title">
        <div>
          <strong>Ваш процесс</strong>
          <span>Перетащите шаги, если хотите изменить порядок.</span>
        </div>
        <el-button text @click="resetSteps">Сбросить</el-button>
      </div>

      <div class="step-list">
        <div
          v-for="(step, index) in steps"
          :key="step.id"
          class="artist-step"
          :class="{ muted: !step.enabled }"
          draggable="true"
          @dragstart="draggedStep = step.id"
          @dragover.prevent
          @drop.prevent="onDrop(index)"
        >
          <div class="step-number">{{ index + 1 }}</div>
          <div class="step-icon">{{ step.icon }}</div>
          <div class="step-copy">
            <strong>{{ step.title }}</strong>
            <span>{{ step.description }}</span>
          </div>
          <el-switch v-model="step.enabled" />
        </div>
      </div>
    </section>

    <section v-if="visibleInputs.length" class="materials">
      <div class="section-title">
        <div>
          <strong>Материалы</strong>
          <span>Студия использует их внутри Flow автоматически.</span>
        </div>
      </div>
      <div class="material-list">
        <div v-for="input in visibleInputs" :key="input.id" class="material-card">
          <div class="material-icon">{{ input.id.includes('reference') ? '🎨' : input.id.includes('mask') ? '◼️' : '🖼️' }}</div>
          <div>
            <strong>{{ inputLabel(input) }}</strong>
            <span v-if="inputAssets?.[input.id]">Добавлено</span>
            <span v-else>Нужно добавить</span>
          </div>
          <el-tag v-if="inputAssets?.[input.id]" type="success" size="small">Готово</el-tag>
          <el-tag v-else type="warning" size="small">Нужно</el-tag>
        </div>
      </div>
    </section>

    <section v-if="parameterDefinitions.length" class="controls">
      <div class="section-title">
        <div>
          <strong>Настройки</strong>
          <span>Только параметры, которые понятны художнику.</span>
        </div>
      </div>
      <div class="control-list">
        <div v-for="[key] in parameterDefinitions" :key="key" class="control">
          <div class="control-head">
            <span>{{ key === 'reference_strength' ? 'Влияние референса' : key === 'color_strength' ? 'Влияние цвета' : key === 'line_strength' ? 'Сохранение линий' : key === 'creativity' ? 'Степень изменений' : 'Сила изменения' }}</span>
            <strong>{{ Math.round(parameterValue(key) * 100) }}%</strong>
          </div>
          <el-slider :model-value="parameterValue(key)" :min="0" :max="1" :step="0.01" @update:model-value="(value: string | number) => setParameter(key, Number(value))" />
        </div>
      </div>
    </section>

    <div class="actions">
      <el-button :loading="validating" @click="validateFlow">Проверить готовность</el-button>
      <el-button
        v-if="validation?.errors.length"
        :loading="fixing"
        type="warning"
        @click="fixWithAI"
      >
        Исправить автоматически
      </el-button>
      <el-button
        type="primary"
        size="large"
        :disabled="!hasRequiredAssets() || !validation?.compatible"
        @click="emit('run')"
      >
        ✨ Создать иллюстрацию
      </el-button>
      <el-button :disabled="!validation?.compatible" @click="emit('save')">Сохранить процесс</el-button>
    </div>

    <el-alert
      v-if="pendingFix"
      title="Студия подготовила исправление"
      :description="pendingFix.reasoning || 'Техническая проблема Flow была исправлена без изменения творческой задачи.'"
      type="info"
      :closable="false"
    >
      <template #default>
        <el-space>
          <el-button type="primary" @click="applyFix">Применить</el-button>
          <el-button @click="rejectFix">Отменить</el-button>
        </el-space>
      </template>
    </el-alert>

    <details class="technical">
      <summary @click="technicalOpen = !technicalOpen">Для разработчика</summary>
      <div v-if="technicalOpen" class="technical-content">
        <p>ComfyUI используется только как внутренний исполнитель. Этот раздел не нужен художнику.</p>
        <pre>{{ JSON.stringify(plan.workflow, null, 2) }}</pre>
      </div>
    </details>
  </div>
</template>

<style scoped>
.artist-flow { display: grid; gap: 18px; }
.artist-intro, .section-title, .control-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.artist-intro { padding: 18px; border-radius: 14px; background: var(--el-fill-color-lighter); }
.artist-intro h3 { margin: 4px 0; font-size: 20px; }
.artist-intro p, .section-title span, .step-copy span, .material-card span { color: var(--el-text-color-secondary); }
.artist-kicker { color: var(--el-color-primary); font-size: 12px; text-transform: uppercase; letter-spacing: .08em; }
.readiness { margin: 0; }
.section-title { margin-bottom: 8px; }
.section-title div { display: grid; gap: 3px; }
.step-list { display: grid; gap: 8px; }
.artist-step { display: grid; grid-template-columns: 32px 44px minmax(0,1fr) auto; gap: 12px; align-items: center; padding: 14px; border: 1px solid var(--el-border-color); border-radius: 12px; background: var(--el-bg-color); cursor: grab; }
.artist-step:active { cursor: grabbing; }
.artist-step.muted { opacity: .55; }
.step-number { font-weight: 700; color: var(--el-text-color-secondary); }
.step-icon, .material-icon { font-size: 26px; text-align: center; }
.step-copy { display: grid; gap: 3px; }
.material-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; }
.material-card { display: grid; grid-template-columns: 36px minmax(0,1fr) auto; gap: 10px; align-items: center; padding: 12px; border: 1px solid var(--el-border-color); border-radius: 12px; }
.material-card div:nth-child(2) { display: grid; gap: 3px; }
.control-list { display: grid; gap: 14px; }
.control { padding: 12px 14px; border: 1px solid var(--el-border-color); border-radius: 12px; }
.control-head strong { color: var(--el-color-primary); }
.actions { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
.technical { border-top: 1px solid var(--el-border-color); padding-top: 10px; color: var(--el-text-color-secondary); }
.technical summary { cursor: pointer; }
.technical-content pre { max-height: 360px; overflow: auto; padding: 12px; border-radius: 8px; background: var(--el-fill-color-darker); color: var(--el-color-white); }
@media (max-width: 700px) { .artist-intro, .section-title { align-items: flex-start; flex-direction: column; } .artist-step { grid-template-columns: 28px 38px minmax(0,1fr); } .artist-step :deep(.el-switch) { grid-column: 3; justify-self: start; } }
</style>
