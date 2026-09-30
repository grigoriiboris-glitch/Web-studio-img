<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { branchesApi, iterationsApi, type Branch, type BranchDifference } from '../../api/client'

const props = defineProps<{ projectId: string }>()
const emit = defineEmits<{ merged: []; switched: [branchId: string] }>()
const branches = ref<Branch[]>([])
const iterations = ref<{ id: string; branch_id: string; created_at: string }[]>([])
const activeId = ref('')
const sourceId = ref('')
const targetId = ref('')
const differences = ref<BranchDifference[]>([])
const selections = ref<Record<string, string>>({})
const newName = ref('')
const creating = ref(false)
const merging = ref(false)
const message = ref('')

const activeBranches = computed(() => branches.value.filter(b => b.status === 'active'))
const sourceBranches = computed(() => activeBranches.value.filter(b => b.id !== targetId.value))
const conflictDimensions = computed(() => differences.value.filter(d => d.different).map(d => d.dimension))

async function load() {
  const [b, i] = await Promise.all([branchesApi.list(props.projectId), iterationsApi.list(props.projectId)])
  branches.value = b.branches
  iterations.value = i.iterations
  if (!activeId.value || !branches.value.some(b => b.id === activeId.value && b.status === 'active')) {
    activeId.value = branches.value.find(b => b.name === 'main')?.id ?? branches.value[0]?.id ?? ''
  }
  if (!targetId.value) targetId.value = activeId.value
  if (!sourceId.value) sourceId.value = activeBranches.value.find(b => b.id !== targetId.value)?.id ?? ''
}
watch(activeId, value => { if (value) { targetId.value = value; emit('switched', value) } })
watch([sourceId, targetId], async () => {
  if (!sourceId.value || !targetId.value || sourceId.value === targetId.value) { differences.value = []; selections.value = {}; return }
  try { differences.value = (await branchesApi.compare(props.projectId, sourceId.value, targetId.value)).differences; selections.value = {} } catch { differences.value = [] }
})

function latestIteration(branchId: string) {
  return [...iterations.value].filter(i => i.branch_id === branchId).sort((a,b) => a.created_at.localeCompare(b.created_at)).at(-1)?.id
}
async function createBranch() {
  if (!newName.value.trim()) return
  creating.value = true; message.value = ''
  try {
    const created = await branchesApi.create(props.projectId, { name: newName.value.trim(), parent_iteration_id: latestIteration(activeId.value) })
    branches.value.push(created); newName.value = ''; activeId.value = created.id; message.value = 'Branch created.'
  } catch (e) { message.value = e instanceof Error ? e.message : 'Could not create branch' } finally { creating.value = false }
}
async function rename(branch: Branch) {
  const name = window.prompt('Branch name', branch.name)?.trim()
  if (!name || name === branch.name) return
  try { Object.assign(branch, await branchesApi.update(props.projectId, branch.id, { name, status: branch.status })) } catch (e) { message.value = e instanceof Error ? e.message : 'Could not rename branch' }
}
async function archive(branch: Branch) {
  if (branch.name === 'main') return
  try { Object.assign(branch, await branchesApi.update(props.projectId, branch.id, { name: branch.name, status: 'archived' })); if (activeId.value === branch.id) activeId.value = branches.value.find(b => b.name === 'main' && b.status === 'active')?.id ?? '' } catch (e) { message.value = e instanceof Error ? e.message : 'Could not archive branch' }
}
async function merge() {
  if (!targetId.value || !sourceId.value || conflictDimensions.value.some(d => !selections.value[d])) { message.value = 'Resolve every conflicting decision before merge.'; return }
  merging.value = true; message.value = ''
  try {
    const decisions = Object.fromEntries(conflictDimensions.value.map(d => [d, { source_branch_id: selections.value[d] }]))
    await branchesApi.merge(props.projectId, targetId.value, { source_branch_ids: [sourceId.value], decisions })
    message.value = 'Merge created a new immutable iteration.'
    await load(); emit('merged')
  } catch (e) { message.value = e instanceof Error ? e.message : 'Could not merge branches' } finally { merging.value = false }
}
onMounted(load)
</script>

<template>
  <el-card class="branch-card">
    <template #header>Creative Branches</template>
    <el-space wrap>
      <el-select v-model="activeId" filterable placeholder="Switch branch" style="width: 260px">
        <el-option v-for="b in activeBranches" :key="b.id" :value="b.id" :label="b.name" />
      </el-select>
      <el-input v-model="newName" placeholder="New branch name" style="width: 220px" @keyup.enter="createBranch" />
      <el-button type="primary" :loading="creating" :disabled="!newName.trim()" @click="createBranch">Create branch</el-button>
    </el-space>
    <el-table :data="branches" size="small" style="margin-top: 12px">
      <el-table-column prop="name" label="Branch" />
      <el-table-column prop="status" label="Status" width="100" />
      <el-table-column label="Actions" width="190">
        <template #default="{ row }">
          <el-button size="small" @click="rename(row)" :disabled="row.status === 'archived'">Rename</el-button>
          <el-button size="small" type="danger" @click="archive(row)" :disabled="row.status === 'archived' || row.name === 'main'">Archive</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-divider />
    <el-space wrap>
      <el-select v-model="sourceId" filterable placeholder="Compare source" style="width: 240px">
        <el-option v-for="b in sourceBranches" :key="b.id" :value="b.id" :label="b.name" />
      </el-select>
      <el-select v-model="targetId" filterable placeholder="Target branch" style="width: 240px">
        <el-option v-for="b in activeBranches" :key="b.id" :value="b.id" :label="b.name" />
      </el-select>
    </el-space>
    <el-table v-if="differences.length" :data="differences" size="small" style="margin-top: 12px">
      <el-table-column prop="dimension" label="Decision" width="150" />
      <el-table-column label="Source">
        <template #default="{ row }"><pre>{{ JSON.stringify(row.source) }}</pre></template>
      </el-table-column>
      <el-table-column label="Target">
        <template #default="{ row }"><pre>{{ JSON.stringify(row.target) }}</pre></template>
      </el-table-column>
      <el-table-column label="Merge choice" width="230">
        <template #default="{ row }">
          <el-select v-if="row.different" v-model="selections[row.dimension]" placeholder="Choose source">
            <el-option :value="sourceId" :label="branches.find(b => b.id === sourceId)?.name ?? 'Source'" />
          </el-select>
          <el-tag v-else type="success">Same</el-tag>
        </template>
      </el-table-column>
    </el-table>
    <el-alert v-if="sourceId && !differences.length" title="No persisted decision differences for these branches." type="info" :closable="false" style="margin-top: 12px" />
    <el-button v-if="sourceId && targetId" type="success" :loading="merging" :disabled="!differences.length || conflictDimensions.some(d => !selections[d])" style="margin-top: 12px" @click="merge">Merge decisions → new iteration</el-button>
    <el-alert v-if="message" :title="message" type="info" :closable="false" style="margin-top: 12px" />
  </el-card>
</template>

<style scoped>
.branch-card { margin-bottom: 16px; }
pre { white-space: pre-wrap; word-break: break-word; margin: 0; }
</style>
