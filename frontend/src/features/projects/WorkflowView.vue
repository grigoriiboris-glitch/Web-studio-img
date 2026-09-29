<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const projectId = String(route.params.projectId)
const mode = ref('develop')
const policy = ref<any>(null)
const lifecycle = ref<any>(null)
const usage = ref<any>(null)
const query = ref('')
const results = ref<any[]>([])
const error = ref('')

async function api(path: string, init?: RequestInit) {
  const response = await fetch('/api/v1' + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  const data = await response.json()
  if (!response.ok) throw new Error(data?.error?.message || 'Request failed')
  return data
}

async function load() {
  try {
    policy.value = await api('/projects/' + projectId + '/mode')
    mode.value = policy.value.mode
    lifecycle.value = await api('/projects/' + projectId + '/lifecycle')
    usage.value = await api('/projects/' + projectId + '/usage')
  } catch (e: any) {
    error.value = e.message
  }
}

async function setMode(next: string) {
  try {
    policy.value = await api('/projects/' + projectId + '/mode', {
      method: 'PUT',
      body: JSON.stringify({ mode: next }),
    })
    mode.value = next
  } catch (e: any) {
    error.value = e.message
  }
}

async function lifecycleAction(action: string) {
  try {
    lifecycle.value = await api('/projects/' + projectId + '/lifecycle', {
      method: 'POST',
      body: JSON.stringify({ action }),
    })
  } catch (e: any) {
    error.value = e.message
  }
}

async function search() {
  try {
    const data = await api('/search?q=' + encodeURIComponent(query.value))
    results.value = data.results
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline -->
  <main>
    <div class="workflow-header">
      <h1>Project workflow</h1>
      <a class="variant-link" :href="'/projects/' + projectId + '/variants'">Variant Board</a>
    </div>
    <p v-if="error">
      {{ error }}
    </p>

    <section>
      <h2>Mode</h2>
      <button
        v-for="item in ['explore', 'develop', 'finalize']"
        :key="item"
        type="button"
        @click="setMode(item)"
      >
        {{ item }}
      </button>
      <p v-if="policy">
        {{ mode }} · max variants: {{ policy.max_variants }} · {{ policy.generation_priority }}
      </p>
    </section>

    <section>
      <h2>Lifecycle</h2>
      <p>
        Status: {{ lifecycle?.status }}
      </p>
      <button
        type="button"
        @click="lifecycleAction('archive')"
      >
        Archive
      </button>
      <button
        type="button"
        @click="lifecycleAction('delete')"
      >
        Delete
      </button>
      <button
        v-if="lifecycle?.status === 'deleted'"
        type="button"
        @click="lifecycleAction('restore')"
      >
        Restore
      </button>
    </section>

    <section>
      <h2>Usage</h2>
      <p>Today: {{ usage?.daily_units }} / {{ usage?.daily_limit }}</p>
      <p>Month: {{ usage?.monthly_units }} / {{ usage?.monthly_limit }}</p>
      <p>Project: {{ usage?.project_units }} / {{ usage?.project_limit }}</p>
    </section>

    <section>
      <h2>Search</h2>
      <form @submit.prevent="search">
        <input
          v-model="query"
          minlength="2"
          placeholder="Search creative content"
        >
        <button type="submit">
          Search
        </button>
      </form>
      <ul>
        <li
          v-for="item in results"
          :key="item.type + item.id"
        >
          {{ item.type }} — {{ item.title }}
        </li>
      </ul>
    </section>
  </main>
</template>
