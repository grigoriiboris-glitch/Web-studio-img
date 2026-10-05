import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../features/projects/DashboardView.vue'
import StudioView from '../features/projects/StudioView.vue'
import CreativeResourcesView from '../features/projects/CreativeResourcesView.vue'
import VisualDNAV2View from '../features/projects/VisualDNAV2View.vue'
import AssetLibraryView from '../features/library/AssetLibraryView.vue'
import WorkflowView from '../features/projects/WorkflowView.vue'
import VariantBoardView from '../features/projects/VariantBoardView.vue'
import CardBatchView from '../features/projects/CardBatchView.vue'
import AuthView from '../features/auth/AuthView.vue'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: AuthView },
    { path: '/register', name: 'register', component: AuthView },
    { path: '/', redirect: '/projects' },
    { path: '/projects', name: 'projects', component: DashboardView, meta: { requiresAuth: true } },
    { path: '/projects/:projectId/studio', name: 'project-studio', component: StudioView, meta: { requiresAuth: true } },
    { path: '/projects/:projectId/resources', name: 'creative-resources', component: CreativeResourcesView, meta: { requiresAuth: true } },
    { path: '/projects/:projectId/visual-dna-v2', name: 'visual-dna-v2', component: VisualDNAV2View, meta: { requiresAuth: true } },
    { path: '/projects/:projectId/workflow', name: 'project-workflow', component: WorkflowView, meta: { requiresAuth: true } },
    { path: '/projects/:projectId/variants', name: 'variant-board', component: VariantBoardView, meta: { requiresAuth: true } },
    { path: '/projects/:projectId/card-batch', name: 'card-batch', component: CardBatchView, meta: { requiresAuth: true } },
    { path: '/library', name: 'asset-library', component: AssetLibraryView, meta: { requiresAuth: true } },
    { path: '/:pathMatch(.*)*', redirect: '/projects' },
  ],
})

export default router


router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.initialize()
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if ((to.name === 'login' || to.name === 'register') && auth.isAuthenticated) {
    return { name: 'projects' }
  }
})
