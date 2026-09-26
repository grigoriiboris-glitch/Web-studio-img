<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { projectsApi, type Project } from '../../api/client'

const route = useRoute()
const project = ref<Project | null>(null)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    project.value = await projectsApi.get(String(route.params.projectId))
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load project'
  }
})
</script>

<template>
  <el-container class="studio">
    <el-header class="header">
      <RouterLink to="/projects">
        <el-button link>← Projects</el-button>
      </RouterLink>
      <strong>{{ project?.name ?? 'Studio' }}</strong>
      <el-tag v-if="project">{{ project.status }}</el-tag>
    </el-header>

    <el-main>
      <el-alert v-if="error" :title="error" type="error" show-icon />
      <el-card v-else-if="project">
        <template #header>Studio entry point</template>
        <el-empty description="Generation, iterations and references will appear here." />
      </el-card>
      <el-skeleton v-else :rows="5" animated />
    </el-main>
  </el-container>
</template>

<style scoped>
.studio {
  min-height: 100vh;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
