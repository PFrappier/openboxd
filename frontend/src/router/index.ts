import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/onboarding',
      name: 'onboarding',
      component: () => import('@/views/OnboardingView.vue'),
    },
    {
      path: '/onboarding/import',
      name: 'onboarding-import',
      component: () => import('@/views/OnboardingImportView.vue'),
    },
    {
      path: '/onboarding/import/:id',
      name: 'onboarding-import-summary',
      component: () => import('@/views/OnboardingImportSummaryView.vue'),
      props: true,
    },
    {
      // Pages of the app itself, under the shared top bar.
      path: '/',
      component: () => import('@/layouts/AppLayout.vue'),
      children: [
        { path: '', redirect: { name: 'onboarding' } },
        {
          path: 'films',
          name: 'films',
          component: () => import('@/views/WatchedFilmsView.vue'),
        },
      ],
    },
  ],
})

export default router
