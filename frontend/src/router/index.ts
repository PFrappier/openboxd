import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: { name: 'onboarding' } },
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
  ],
})

export default router
