import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../features/projects/DashboardView.vue'
import StudioView from '../features/projects/StudioView.vue'
import CreativeResourcesView from '../features/projects/CreativeResourcesView.vue'
import WorkflowView from '../features/projects/WorkflowView.vue'
import VariantBoardView from '../features/projects/VariantBoardView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/projects' },
    { path: '/projects', name: 'projects', component: DashboardView },
    { path: '/projects/:projectId/studio', name: 'project-studio', component: StudioView },
    { path: '/projects/:projectId/resources', name: 'creative-resources', component: CreativeResourcesView },
    { path: '/projects/:projectId/workflow', name: 'project-workflow', component: WorkflowView },
    { path: '/projects/:projectId/variants', name: 'variant-board', component: VariantBoardView },
    { path: '/:pathMatch(.*)*', redirect: '/projects' },
  ],
})

export default router
