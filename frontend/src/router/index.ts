import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../features/projects/DashboardView.vue'
import StudioView from '../features/projects/StudioView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/projects' },
    { path: '/projects', name: 'projects', component: DashboardView },
    { path: '/projects/:projectId/studio', name: 'project-studio', component: StudioView },
    { path: '/:pathMatch(.*)*', redirect: '/projects' },
  ],
})

export default router
