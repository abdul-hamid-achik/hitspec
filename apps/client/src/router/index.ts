import { createRouter, createWebHistory } from 'vue-router'
import { useCollectionStore } from '@/stores/collection'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: () => import('@/components/layout/AppShell.vue'),
      children: [
        {
          path: '',
          component: () => import('@/views/WorkspaceView.vue'),
        },
        {
          path: 'stress',
          component: () => import('@/views/StressView.vue'),
        },
        {
          path: 'mock',
          component: () => import('@/views/MockView.vue'),
        },
        {
          path: 'contract',
          component: () => import('@/views/ContractView.vue'),
        },
        {
          path: 'record',
          component: () => import('@/views/RecordView.vue'),
        },
        {
          path: 'history',
          component: () => import('@/views/HistoryView.vue'),
        },
        {
          path: 'settings',
          component: () => import('@/views/SettingsView.vue'),
        },
        {
          path: 'import',
          component: () => import('@/views/ImportView.vue'),
        },
        {
          path: 'cookies',
          component: () => import('@/views/CookiesView.vue'),
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

router.beforeEach(() => {
  const collection = useCollectionStore()
  if (collection.dirtyFiles.size > 0) {
    return window.confirm('You have unsaved changes. Leave this page?')
  }
})

export { router }
