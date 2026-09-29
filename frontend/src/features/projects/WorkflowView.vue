<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
const route=useRoute()
const projectId=String(route.params.projectId)
const mode=ref('develop')
const policy=ref<any>(null)
const lifecycle=ref<any>(null)
const usage=ref<any>(null)
const error=ref('')
const q=ref('')
const results=ref<any[]>([])
async function api(path:string,init?:RequestInit){const r=await fetch('/api/v1'+path,{credentials:'include',headers:{'Content-Type':'application/json'},...init});const d=await r.json();if(!r.ok)throw new Error(d?.error?.message||'Request failed');return d}
async function load(){try{policy.value=await api('/projects/'+projectId+'/mode');mode.value=policy.value.mode;lifecycle.value=await api('/projects/'+projectId+'/lifecycle');usage.value=await api('/projects/'+projectId+'/usage')}catch(e:any){error.value=e.message}}
async function setMode(m:string){try{policy.value=await api('/projects/'+projectId+'/mode',{method:'PUT',body:JSON.stringify({mode:m})});mode.value=m}catch(e:any){error.value=e.message}}
async function search(){try{const d=await api('/search?q='+encodeURIComponent(q.value));results.value=d.results}catch(e:any){error.value=e.message}}
async function lifecycleAction(a:string){try{lifecycle.value=await api('/projects/'+projectId+'/lifecycle',{method:'POST',body:JSON.stringify({action:a})})}catch(e:any){error.value=e.message}}
onMounted(load)
</script>
<template><main><h1>Project workflow</h1><p v-if="error">{{error}}</p><div><button v-for="m in ['explore','develop','finalize']" :key="m" @click="setMode(m)">{{m}}</button></div><p v-if="policy">{{policy.mode}} · max variants: {{policy.max_variants}} · {{policy.generation_priority}}</p><section><h2>Lifecycle</h2><p>{{lifecycle?.status}}</p><button @click="lifecycleAction('archive')">Archive</button><button @click="lifecycleAction('delete')">Delete</button><button v-if="lifecycle?.status==='deleted'" @click="lifecycleAction('restore')">Restore</button></section><section><h2>Usage</h2><p>{{usage?.daily_units}} / {{usage?.daily_limit}} today</p><p>{{usage?.monthly_units}} / {{usage?.monthly_limit}} month</p><p>{{usage?.project_units}} / {{usage?.project_limit}} project</p></section><section><h2>Search</h2><form @submit.prevent="search"><input v-model="q" minlength="2" placeholder="Search creative content"><button>Search</button></form><ul><li v-for="x in results" :key="x.type+x.id">{{x.type}} — {{x.title}}</li></ul></section></main></template>