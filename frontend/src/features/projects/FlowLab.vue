<!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline, vue/html-indent -->
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

const props = defineProps<{
  projectId: string
  plan: ComfyFlowPlan
  inputAssets?: Record<string, string>
}>()

const emit = defineEmits(['update:plan', 'run', 'save'])

const selectedNodeId = ref('')
const nodeInputsText = ref('')
const nodeClassType = ref('')
const disabledNodes = ref<string[]>([])
const disabledSnapshots = ref<Record<string, unknown>>({})
const validation = ref<Validation | null>(null)
const validating = ref(false)
const fixing = ref(false)
const pendingFix = ref<ComfyFlowPlan | null>(null)
const error = ref('')

const nodeEntries = computed(() => Object.entries(props.plan.workflow))
const selectedNode = computed(() => {
  if (!selectedNodeId.value) return null
  const value = props.plan.workflow[selectedNodeId.value]
  return value && typeof value === 'object' ? value as Record<string, unknown> : null
})

function selectNode(id: string) {
  selectedNodeId.value = id
  const node = props.plan.workflow[id] as Record<string, unknown> | undefined
  nodeClassType.value = typeof node?.class_type === 'string' ? node.class_type : ''
  nodeInputsText.value = JSON.stringify(node?.inputs ?? {}, null, 2)
}

watch(() => props.plan.workflow, () => {
  if (!selectedNodeId.value || !props.plan.workflow[selectedNodeId.value]) {
    const first = Object.keys(props.plan.workflow)[0]
    if (first) selectNode(first)
  }
}, { immediate: true })

function updatePlan(workflow: Record<string, unknown>) {
  emit('update:plan', { ...props.plan, workflow })
}

function applyNodeEdit() {
  if (!selectedNodeId.value) return
  let inputs: Record<string, unknown>
  try {
    const parsed = JSON.parse(nodeInputsText.value || '{}')
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('Inputs must be a JSON object')
    inputs = parsed
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Invalid node inputs JSON'
    return
  }
  const current = props.plan.workflow[selectedNodeId.value]
  const node = current && typeof current === 'object' ? { ...(current as Record<string, unknown>) } : {}
  node.class_type = nodeClassType.value.trim()
  node.inputs = inputs
  updatePlan({ ...props.plan.workflow, [selectedNodeId.value]: node })
  error.value = ''
}

function toggleNodeDisabled(id: string) {
  const workflow = { ...props.plan.workflow }
  if (disabledNodes.value.includes(id)) {
    const snapshot = disabledSnapshots.value[id]
    if (snapshot) workflow[id] = snapshot
    const next = { ...disabledSnapshots.value }
    delete next[id]
    disabledSnapshots.value = next
    disabledNodes.value = disabledNodes.value.filter(value => value !== id)
  } else {
    disabledSnapshots.value = { ...disabledSnapshots.value, [id]: workflow[id] }
    delete workflow[id]
    disabledNodes.value = [...disabledNodes.value, id]
  }
  updatePlan(workflow)
}

function activeWorkflow(): Record<string, unknown> {
  return { ...props.plan.workflow }
}

function addNode() {
  const ids = new Set(Object.keys(props.plan.workflow))
  let index = Object.keys(props.plan.workflow).length + 1
  while (ids.has(String(index))) index += 1
  const id = String(index)
  updatePlan({ ...props.plan.workflow, [id]: { class_type: 'LoadImage', inputs: {} } })
  selectNode(id)
}

function removeNode() {
  if (!selectedNodeId.value) return
  const workflow = { ...props.plan.workflow }
  delete workflow[selectedNodeId.value]
  disabledNodes.value = disabledNodes.value.filter(id => id !== selectedNodeId.value)
  selectedNodeId.value = ''
  updatePlan(workflow)
}

async function validateFlow() {
  validating.value = true
  error.value = ''
  try {
    const response = await assistantApi.execute<Validation>(
      props.projectId,
      'validate_comfy_flow',
      { workflow: activeWorkflow() },
    )
    validation.value = response.result
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Validation failed'
    validation.value = null
  } finally {
    validating.value = false
  }
}

async function fixWithAI() {
  const errors = validation.value?.errors ?? []
  if (!errors.length) {
    error.value = 'Run Validate first and provide at least one validation error'
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
          'Validation errors:',
          ...errors,
        ].join('\n'),
        current_workflow: activeWorkflow(),
        validation_errors: errors,
        inputs: props.plan.inputs,
        parameters: props.plan.parameters,
      },
    )
    pendingFix.value = response.result
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'AI fix failed'
  } finally {
    fixing.value = false
  }
}

function applyFix() {
  if (!pendingFix.value) return
  emit('update:plan', pendingFix.value)
  pendingFix.value = null
  validation.value = null
  selectedNodeId.value = ''
}

function rejectFix() {
  pendingFix.value = null
}
</script>

<template>
  <div class="flow-lab">
    <div class="flow-lab-toolbar">
      <div>
        <strong>Flow Lab</strong>
        <span class="flow-lab-subtitle">Visual ComfyUI workflow editor</span>
      </div>
      <el-space wrap>
        <el-button :loading="validating" @click="validateFlow">Validate</el-button>
        <el-button :loading="fixing" :disabled="!validation?.errors.length" @click="fixWithAI">Fix with AI</el-button>
        <el-button @click="addNode">Add node</el-button>
        <el-button type="primary" :disabled="!validation?.compatible" @click="emit('run')">Run Flow</el-button>
        <el-button :disabled="!validation?.compatible" @click="emit('save')">Save as Recipe</el-button>
      </el-space>
    </div>

    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-alert
      v-if="validation"
      :title="validation.compatible ? 'Workflow is compatible' : 'Workflow has blocking errors'"
      :type="validation.compatible ? 'success' : 'error'"
      :description="[...validation.errors, ...validation.warnings].join(' • ')"
      :closable="false"
    />

    <div class="flow-lab-grid">
      <section class="flow-graph">
        <div
          v-for="([id, node], index) in nodeEntries"
          :key="id"
          class="flow-node"
          :class="{ selected: selectedNodeId === id, disabled: disabledNodes.includes(id) }"
          :style="{ '--node-index': index }"
          @click="selectNode(id)"
        >
          <div class="flow-node-header">
            <span>#{{ id }}</span>
            <el-tag size="small">{{ typeof (node as Record<string, unknown>).class_type === 'string' ? (node as Record<string, unknown>).class_type : 'Unknown' }}</el-tag>
          </div>
          <div class="flow-node-body">
            {{ Object.keys(((node as Record<string, unknown>).inputs ?? {}) as object).length }} inputs
          </div>
          <div v-if="disabledNodes.includes(id)" class="flow-node-state">disabled</div>
        </div>
      </section>

      <aside class="flow-inspector">
        <template v-if="selectedNode">
          <h4>Node #{{ selectedNodeId }}</h4>
          <el-input v-model="nodeClassType" placeholder="class_type" />
          <el-input
            v-model="nodeInputsText"
            type="textarea"
            :rows="14"
            spellcheck="false"
            class="flow-json-editor"
          />
          <el-space wrap>
            <el-button type="primary" @click="applyNodeEdit">Apply node</el-button>
            <el-button @click="toggleNodeDisabled(selectedNodeId)">
              {{ disabledNodes.includes(selectedNodeId) ? 'Enable' : 'Disable' }}
            </el-button>
            <el-button type="danger" plain @click="removeNode">Remove</el-button>
          </el-space>
        </template>
        <el-empty v-else description="Select a node" />
      </aside>
    </div>

    <div v-if="pendingFix" class="flow-fix-preview">
      <el-alert title="AI proposed a corrected workflow" type="warning" :closable="false" />
      <p>{{ pendingFix.reasoning }}</p>
      <el-space>
        <el-button type="primary" @click="applyFix">Apply correction</el-button>
        <el-button @click="rejectFix">Reject</el-button>
      </el-space>
    </div>
  </div>
</template>

<style scoped>
.flow-lab { display: grid; gap: 12px; }
.flow-lab-toolbar { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
.flow-lab-subtitle { margin-left: 8px; color: var(--el-text-color-secondary); font-size: 12px; }
.flow-lab-grid { display: grid; grid-template-columns: minmax(0, 2fr) minmax(280px, 1fr); gap: 12px; min-height: 420px; }
.flow-graph { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); align-content: start; gap: 12px; padding: 12px; border: 1px solid var(--el-border-color); border-radius: 8px; background: var(--el-fill-color-lighter); }
.flow-node { min-height: 100px; padding: 10px; border: 1px solid var(--el-border-color); border-radius: 8px; background: var(--el-bg-color); cursor: pointer; }
.flow-node.selected { border-color: var(--el-color-primary); box-shadow: 0 0 0 1px var(--el-color-primary); }
.flow-node.disabled { opacity: .5; }
.flow-node-header { display: flex; justify-content: space-between; gap: 8px; align-items: center; }
.flow-node-body { margin-top: 12px; color: var(--el-text-color-secondary); font-size: 12px; }
.flow-node-state { margin-top: 8px; color: var(--el-color-warning); font-size: 11px; }
.flow-inspector { display: grid; align-content: start; gap: 10px; padding: 12px; border: 1px solid var(--el-border-color); border-radius: 8px; }
.flow-json-editor :deep(textarea) { font-family: monospace; }
.flow-fix-preview { display: grid; gap: 8px; padding: 12px; border: 1px solid var(--el-color-warning); border-radius: 8px; }
@media (max-width: 900px) { .flow-lab-grid { grid-template-columns: 1fr; } }
</style>
