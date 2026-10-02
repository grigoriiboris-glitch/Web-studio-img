<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { assistantApi, type ComfyFlowPlan, type ComfyFlowStep } from '../../api/client'

type TestError = {
  code: string
  message: string
  node?: string
  input?: string
  recoverable: boolean
}

type TestResult = {
  ok: boolean
  category: 'workflow' | 'runtime' | 'connection' | 'container' | 'model' | 'unknown'
  errors: TestError[]
  warnings: string[]
  repair: { available: boolean; strategy: 'ai_flow' | 'pi_agent' | 'manual' }
  test_id?: string
  duration_ms?: number
}

type RepairHistoryItem = {
  attempt: number
  status: 'testing' | 'repairing' | 'passed' | 'failed' | 'stopped'
  category?: TestResult['category']
  action?: string
  errors: string[]
}

type Validation = {
  compatible: boolean
  errors: TestError[]
  warnings: string[]
  referenced_nodes: string[]
  referenced_models: string[]
  category?: TestResult['category']
  repair?: TestResult['repair']
  test_id?: string
  duration_ms?: number
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
const testRunAt = ref<string | null>(null)
const repairHistory = ref<RepairHistoryItem[]>([])
const repairStopped = ref(false)
const repairing = ref(false)
const formattedTestRunAt = computed(() => testRunAt.value ? new Date(testRunAt.value).toLocaleTimeString() : '')

const stepCatalog: ArtistStep[] = [
  { id: 'sketch', icon: '✏️', title: 'Скетч', description: 'Сохраняем композицию и важные линии.', enabled: true },
  { id: 'reference', icon: '🎨', title: 'Цветной референс', description: 'Берём цвета, материалы и настроение.', enabled: true },
  { id: 'structure', icon: '🧩', title: 'Сохраняем форму', description: 'Не даём генерации потерять основную конструкцию изображения.', enabled: true },
  { id: 'final', icon: '🖼️', title: 'Финальная иллюстрация', description: 'Получаем полноценное цветное изображение.', enabled: true },
]

const steps = ref<ArtistStep[]>(stepCatalog.map(step => ({ ...step })))
watch(() => props.plan.flow_steps, (flowSteps) => {
  if (!flowSteps?.length) return
  const enabled = new Map(flowSteps.map(step => [step.id, step.enabled]))
  const ordered = [...flowSteps].sort((a, b) => a.order - b.order).map(step => step.id)
  const catalog = new Map(stepCatalog.map(step => [step.id, step]))
  steps.value = ordered.map(id => ({ ...(catalog.get(id) ?? stepCatalog[0]), enabled: enabled.get(id) ?? true }))
}, { deep: true })
const emitFlowSteps = () => {
  emit('update:plan', {
    ...props.plan,
    flow_steps: steps.value.map((step, order): ComfyFlowStep => ({ id: step.id as ComfyFlowStep['id'], enabled: step.enabled, order })),
  })
}

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
  emitFlowSteps()
}

function onDrop(index: number) {
  if (!draggedStep.value) return
  const from = steps.value.findIndex(step => step.id === draggedStep.value)
  moveStep(from, index)
  draggedStep.value = null
}

function resetSteps() {
  steps.value = stepCatalog.map(step => ({ ...step }))
  emitFlowSteps()
}

function hasRequiredAssets() {
  return props.plan.inputs.filter(input => input.required).every(input => Boolean(props.inputAssets?.[input.id]))
}

async function validateFlow() {
  validating.value = true
  repairing.value = false
  repairStopped.value = false
  testRunAt.value = null
  error.value = ''
  repairHistory.value = []

  let currentPlan = props.plan
  try {
    for (let attempt = 1; attempt <= 3; attempt += 1) {
      if (repairStopped.value) {
        repairHistory.value.push({ attempt, status: 'stopped', errors: [] })
        break
      }

      repairHistory.value.push({ attempt, status: 'testing', action: 'Реальный execution test', errors: [] })
      const response = await assistantApi.execute<TestResult>(
        props.projectId,
        'test_comfy_flow',
        {
          workflow: currentPlan.workflow,
          flow_steps: currentPlan.flow_steps,
          artist_steps: currentPlan.artist_steps,
          input_assets: props.inputAssets,
          prompt: currentPlan.prompt,
          negative_prompt: currentPlan.negative_prompt,
          aspect_ratio: currentPlan.parameters?.aspect_ratio,
          seed: currentPlan.parameters?.seed,
          parameters: currentPlan.parameters,
        },
      )
      const result = response.result
      const messages = result.errors
      repairHistory.value[repairHistory.value.length - 1] = {
        attempt,
        status: result.ok ? 'passed' : 'failed',
        category: result.category,
        errors: messages.map(item => item.message),
      }
      validation.value = {
        compatible: result.ok,
        errors: messages,
        warnings: result.warnings,
        referenced_nodes: [],
        referenced_models: [],
        category: result.category,
        repair: result.repair,
        test_id: result.test_id,
        duration_ms: result.duration_ms,
      }
      testRunAt.value = new Date().toISOString()

      if (result.ok || result.category !== 'workflow' || attempt >= 3) break

      repairing.value = true
      repairHistory.value.push({
        attempt,
        status: 'repairing',
        category: result.category,
        action: 'AI исправляет только техническую часть workflow',
        errors: messages.map(item => item.message),
      })
      const repair = await assistantApi.execute<ComfyFlowPlan>(
        props.projectId,
        'create_comfy_flow',
        {
          task: [
            currentPlan.prompt || currentPlan.description || 'Repair this ComfyUI workflow',
            'Repair only the technical workflow error. Preserve artist intent, Artist Flow order/enabled state, inputs, Creative Brief and Color Reference contract.',
            'Validation/execution errors:',
            ...messages.map(item => item.message),
          ].join('\n'),
          current_workflow: currentPlan.workflow,
          validation_errors: messages.map(item => item.message),
          inputs: currentPlan.inputs,
          parameters: currentPlan.parameters,
          flow_steps: currentPlan.flow_steps,
          max_attempts: 1,
        },
      )
      currentPlan = repair.result
      emit('update:plan', currentPlan)
      repairing.value = false
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Не удалось выполнить тест ComfyUI'
    validation.value = null
    testRunAt.value = new Date().toISOString()
  } finally {
    repairing.value = false
    validating.value = false
  }
}

function stopRepairLoop() {
  repairStopped.value = true
}

async function sendToPiAgent() {
  const last = validation.value
  if (!last || last.category === 'workflow' || !last.errors.length) return
  const bridgeUrl = String(import.meta.env.VITE_PI_AGENT_BRIDGE_URL ?? '').trim()
  if (!bridgeUrl) {
    error.value = 'Локальный pi.dev bridge не настроен: задайте VITE_PI_AGENT_BRIDGE_URL.'
    return
  }
  try {
    const parsed = new URL(bridgeUrl)
    if (!['localhost', '127.0.0.1', '[::1]'].includes(parsed.hostname)) {
      throw new Error('pi.dev bridge должен быть доступен только через localhost')
    }
    const response = await fetch(parsed.toString(), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        protocol: 'comfyui-self-healing/v1',
        projectId: props.projectId,
        container: 'comfyui',
        endpoint: 'local-comfyui',
        testId: last.test_id ?? '',
        category: last.category,
        errors: last.errors.map(item => ({ code: item.code, message: item.message, node: item.node, input: item.input, recoverable: item.recoverable })),
        workflowFingerprint: props.plan.flow_fingerprint,
        allowedActions: {
          safe: ['inspect_runtime', 'read_logs', 'read_status'],
          repair: ['edit_allowlisted_workflow', 'edit_allowlisted_config', 'restart_comfyui', 'retest'],
          dangerous: [],
        },
        policy: {
          arbitraryShell: false,
          hostFilesystemAccess: false,
          volumeDeletion: false,
          imageReplacement: false,
          networkMutation: false,
        },
      }),
    })
    if (!response.ok) throw new Error('pi.dev bridge вернул HTTP ' + response.status)
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Не удалось передать диагностику pi.dev агенту'
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
          ...errors.map(item => item.message),
        ].join('\n'),
        current_workflow: props.plan.workflow,
        validation_errors: errors.map(item => item.message),
        inputs: props.plan.inputs,
        parameters: props.plan.parameters,
        flow_steps: props.plan.flow_steps,
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
      v-if="error"
      title="Тест не выполнен"
      :description="error"
      type="error"
      :closable="false"
      show-icon
      class="test-error"
    />

    <el-alert
      :title="readiness.title"
      :description="readiness.text"
      :type="readiness.kind"
      :closable="false"
      class="readiness"
    />

    <el-alert
      v-if="validation && validation.errors.length"
      title="Ошибки теста"
      type="error"
      :closable="false"
      show-icon
      class="test-errors"
    >
      <ul>
        <li v-for="(item, index) in validation.errors" :key="`error-${index}`">{{ item.message }}</li>
      </ul>
    </el-alert>

    <el-alert
      v-if="validation?.category && validation.category !== 'unknown'"
      :title="`Категория: ${validation.category}`"
      :description="validation.repair ? `Восстановление: ${validation.repair.strategy}` : ''"
      type="info"
      :closable="false"
      class="test-errors"
    />

    <el-alert
      v-if="validation && validation.warnings.length"
      title="Предупреждения теста"
      type="warning"
      :closable="false"
      show-icon
      class="test-errors"
    >
      <ul>
        <li v-for="(item, index) in validation.warnings" :key="`warning-${index}`">{{ item }}</li>
      </ul>
    </el-alert>

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
          <el-switch v-model="step.enabled" @change="emitFlowSteps" />
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

    <div v-if="repairHistory.length" class="repair-history">
      <strong>История теста</strong>
      <div v-for="item in repairHistory" :key="`${item.attempt}-${item.status}-${item.action ?? ''}`" class="repair-item">
        <span>Попытка {{ item.attempt }}</span>
        <span>{{ item.status === 'testing' ? 'Тест' : item.status === 'repairing' ? 'AI исправляет' : item.status === 'passed' ? 'Готово' : item.status === 'stopped' ? 'Остановлено' : 'Ошибка' }}</span>
        <span v-if="item.category">{{ item.category }}</span>
      </div>
    </div>

    <div class="actions">
      <el-button :loading="validating" type="info" @click="validateFlow">
        🧪 {{ validating ? 'Тест выполняется…' : 'Запустить тест' }}
      </el-button>
      <span v-if="testRunAt && !validating" class="test-time">Последний тест: {{ formattedTestRunAt }}</span>
      <el-button v-if="validating && !repairStopped" @click="stopRepairLoop">Остановить восстановление</el-button>
      <el-button
        v-if="validation && validation.category && validation.category !== 'workflow' && validation.repair?.strategy === 'pi_agent'"
        type="warning"
        @click="sendToPiAgent"
      >
        🛠 Передать локальному pi.dev
      </el-button>
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
.test-error, .test-errors { margin: 0; }
.test-errors ul { margin: 6px 0 0; padding-left: 20px; }
.test-errors li { margin: 4px 0; }
.test-time { color: var(--el-text-color-secondary); font-size: 12px; }
.repair-history { display: grid; gap: 8px; padding: 12px 14px; border: 1px solid var(--el-border-color); border-radius: 12px; }
.repair-item { display: flex; flex-wrap: wrap; gap: 10px; color: var(--el-text-color-secondary); font-size: 13px; }
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
