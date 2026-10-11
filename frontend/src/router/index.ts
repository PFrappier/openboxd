import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  // Going back or forward returns to where the page was left; anything else starts at the top.
  scrollBehavior: (_to, _from, savedPosition) => savedPosition ?? { top: 0 },
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
        {
          path: 'films/:id',
          name: 'film',
          component: () => import('@/views/FilmView.vue'),
          props: true,
        },
        {
          path: 'journal',
          name: 'diary',
          component: () => import('@/views/DiaryView.vue'),
        },
        {
          path: 'critiques',
          name: 'reviews',
          component: () => import('@/views/ReviewsView.vue'),
        },
        {
          path: 'watchlist',
          name: 'watchlist',
          component: () => import('@/views/WatchlistView.vue'),
        },
        {
          path: 'listes',
          name: 'lists',
          component: () => import('@/views/ListsView.vue'),
        },
        {
          path: 'listes/:id',
          name: 'list',
          component: () => import('@/views/ListView.vue'),
          props: true,
        },
        {
          path: 'a-propos',
          name: 'about',
          component: () => import('@/views/AboutView.vue'),
        },
      ],
    },
  ],
})

export default router
