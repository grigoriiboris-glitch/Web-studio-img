<!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline -->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RouterLink } from 'vue-router'

import { projectsApi, type Project } from '../../api/client'

const projects = ref<Project[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const dialogVisible = ref(false)
const editingId = ref<string | null>(null)
const form = reactive({ name: '', description: '' })

async function loadProjects() {
  loading.value = true
  error.value = null
  try {
    projects.value = (await projectsApi.list()).projects
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load projects'
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.name = ''
  form.description = ''
  dialogVisible.value = true
}

function openEdit(project: Project) {
  editingId.value = project.id
  form.name = project.name
  form.description = project.description ?? ''
  dialogVisible.value = true
}

async function saveProject() {
  if (!form.name.trim()) {
    ElMessage.warning('Project name is required')
    return
  }
  try {
    if (editingId.value) {
      const updated = await projectsApi.update(editingId.value, {
        name: form.name,
        description: form.description || undefined,
        status: projects.value.find((item) => item.id === editingId.value)?.status ?? 'active',
      })
      projects.value = projects.value.map((item) => item.id === updated.id ? updated : item)
    } else {
      const created = await projectsApi.create({
        name: form.name,
        description: form.description || undefined,
      })
      projects.value.unshift(created)
    }
    dialogVisible.value = false
    ElMessage.success('Project saved')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not save project')
  }
}

async function archiveProject(project: Project) {
  try {
    await ElMessageBox.confirm(
      `Archive “${project.name}”?`,
      'Archive project',
      { type: 'warning' },
    )
    const archived = await projectsApi.archive(project.id)
    projects.value = projects.value.map((item) => item.id === archived.id ? archived : item)
    ElMessage.success('Project archived')
  } catch {
    // cancelled
  }
}

onMounted(loadProjects)
</script>

<template>
  <el-container class="dashboard">
    <el-header>
      <div class="header-content">
        <div>
          <strong>Web Studio</strong>
          <span>Projects</span>
        </div>
        <div class="header-actions"><RouterLink to="/library"><el-button>Asset Library</el-button></RouterLink><el-button type="primary" @click="openCreate">New project</el-button></div>
      </div>
    </el-header>

    <el-main>
      <el-alert
        v-if="error"
        :title="error"
        type="error"
        show-icon
        class="error"
      />

      <div v-loading="loading" class="project-grid">
        <el-card v-for="project in projects" :key="project.id" class="project-card">
          <template #header>
            <div class="card-header">
              <strong>{{ project.name }}</strong>
              <el-tag :type="project.status === 'active' ? 'success' : 'info'">
                {{ project.status }}
              </el-tag>
            </div>
          </template>
          <p>{{ project.description || 'No description yet.' }}</p>
          <div class="actions">
            <el-button link @click="openEdit(project)">Edit</el-button>
            <el-button link type="danger" @click="archiveProject(project)">Archive</el-button>
            <RouterLink :to="`/projects/${project.id}/studio`">
              <el-button type="primary" plain>Open Studio</el-button>
            </RouterLink>
          </div>
        </el-card>
      </div>

      <el-empty v-if="!loading && !error && projects.length === 0" description="No projects yet">
        <el-button type="primary" @click="openCreate">Create your first project</el-button>
      </el-empty>
    </el-main>

    <el-dialog v-model="dialogVisible" :title="editingId ? 'Edit project' : 'New project'" width="480px">
      <el-form label-position="top" @submit.prevent="saveProject">
        <el-form-item label="Name">
          <el-input v-model="form.name" maxlength="200" show-word-limit @keyup.enter="saveProject" />
        </el-form-item>
        <el-form-item label="Description">
          <el-input v-model="form.description" type="textarea" :rows="4" maxlength="2000" show-word-limit />
        </el-form-item>
        <div class="dialog-actions">
          <el-button @click="dialogVisible = false">Cancel</el-button>
          <el-button type="primary" @click="saveProject">Save</el-button>
        </div>
      </el-form>
    </el-dialog>
  </el-container>
</template>

<style scoped>
.dashboard {
  min-height: 100vh;
}

.header-content,
.card-header,
.actions,
.dialog-actions,
.header-actions {
  display: flex;
  align-items: center;
}

.header-content,
.card-header,
.dialog-actions {
  justify-content: space-between;
}

.project-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.project-card p {
  min-height: 48px;
  color: var(--el-text-color-secondary);
}

.header-actions { gap: 8px; }

.actions {
  gap: 8px;
  flex-wrap: wrap;
}

.error {
  margin-bottom: 16px;
}
</style>
