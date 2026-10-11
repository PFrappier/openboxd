<script lang="ts">
// Last page shown and its URL, kept across visits so that coming back from a film renders
// the reviews at once, which lets the router restore the scroll position.
const lastShown: { url?: string; result?: ReviewsPage } = {}
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  ArrowUpDown,
  CircleAlert,
  Clapperboard,
  Download,
  Heart,
  NotebookPen,
  Repeat2,
} from '@lucide/vue'

import FilmRating from '@/components/FilmRating.vue'
import ListPagination from '@/components/ListPagination.vue'
import ReviewText from '@/components/ReviewText.vue'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ApiError } from '@/lib/api/client'
import { posterUrl } from '@/lib/api/films'
import {
  getReviews,
  REVIEWS_SORTS,
  type ReviewEntry,
  type ReviewsPage,
  type ReviewsSort,
} from '@/lib/api/reviews'
import { formatDate } from '@/lib/dates'

// Few of them: reviews are read through, and can be long.
const PAGE_SIZE = 12

const route = useRoute()
const router = useRouter()

/** Page asked by the URL, from 1; anything else than a page number means the first one. */
const page = computed(() => {
  const asked = Number(route.query.page)
  return Number.isInteger(asked) && asked >= 1 ? asked : 1
})

const DEFAULT_SORT: ReviewsSort = 'reviewed-desc'

/** Order asked by the URL; anything else than a known one means the default. */
const sort = computed(
  () => REVIEWS_SORTS.find((known) => known === route.query.sort) ?? DEFAULT_SORT,
)

// The first page in the default order has neither in its URL, so that the reviews keep the
// URL the rest of the app links to.
const pageLocation = (number: number, inSort = sort.value) => ({
  name: 'reviews',
  query: {
    ...(inSort !== DEFAULT_SORT ? { sort: inSort } : {}),
    ...(number > 1 ? { page: number } : {}),
  },
})

/** Changes the order, and goes back to the first page. */
function setSort(value: unknown) {
  const known = REVIEWS_SORTS.find((known) => known === value)
  if (known) return router.push(pageLocation(1, known))
}

// Sorts come in pairs, one for each direction, under the name of what they sort by.
const SORT_GROUPS: { title: string; sorts: [ReviewsSort, string][] }[] = [
  {
    title: 'Date de la critique',
    sorts: [
      ['reviewed-desc', 'Les plus récentes'],
      ['reviewed-asc', 'Les plus anciennes'],
    ],
  },
  {
    title: 'Note',
    sorts: [
      ['rating-desc', 'Mieux notés'],
      ['rating-asc', 'Moins bien notés'],
    ],
  },
  {
    title: 'Titre',
    sorts: [
      ['name-asc', 'Titre de A à Z'],
      ['name-desc', 'Titre de Z à A'],
    ],
  },
]

const reviews = ref<ReviewEntry[]>([])
const total = ref<number | null>(null)
const isLoading = ref(false)
const error = ref<string | null>(null)

if (lastShown.result && lastShown.url === route.fullPath) {
  reviews.value = lastShown.result.reviews
  total.value = lastShown.result.total
}

const pageCount = computed(() => Math.max(1, Math.ceil((total.value ?? 0) / PAGE_SIZE)))

// Only the answer to the latest request counts: pages can be changed faster than they load.
let latestRequest = 0

async function load() {
  // The URL also changes when leaving the page, which asks for nothing.
  if (route.name !== 'reviews') return
  const request = ++latestRequest
  const asked = page.value
  const url = route.fullPath
  isLoading.value = true
  error.value = null
  try {
    const result = await getReviews(
      { limit: PAGE_SIZE, offset: (asked - 1) * PAGE_SIZE },
      sort.value,
    )
    if (request !== latestRequest) return
    const lastPage = Math.max(1, Math.ceil(result.total / PAGE_SIZE))
    if (asked > lastPage) {
      await router.replace(pageLocation(lastPage))
      return
    }
    reviews.value = result.reviews
    total.value = result.total
    Object.assign(lastShown, { url, result })
  } catch (cause) {
    if (request !== latestRequest) return
    reviews.value = []
    error.value =
      cause instanceof ApiError
        ? 'Les critiques n’ont pas pu être chargées.'
        : 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
  } finally {
    if (request === latestRequest) isLoading.value = false
  }
}

watch([page, sort], load, { immediate: true })

const formatCount = (count: number) => count.toLocaleString('fr-FR')

const posterClass = 'aspect-2/3 w-16 rounded-md border bg-muted object-cover sm:w-20'

const filmLinkClass =
  'rounded-md outline-none transition-opacity hover:opacity-80 focus-visible:ring-3 focus-visible:ring-ring/50'
</script>

<template>
  <main class="mx-auto flex w-full max-w-4xl flex-col gap-5 px-4 py-6 sm:px-6">
    <header class="flex flex-wrap items-center gap-x-3 gap-y-3">
      <div class="flex items-baseline gap-x-3">
        <h1 class="text-2xl font-semibold tracking-tight">Critiques</h1>
        <p v-if="total !== null" class="text-muted-foreground">
          {{ formatCount(total) }} {{ total > 1 ? 'critiques' : 'critique' }}
        </p>
      </div>
      <!-- Pushed to the end of the row; as wide whatever the order, so that it stays put. -->
      <Select :model-value="sort" @update:model-value="setSort">
        <SelectTrigger size="sm" class="w-60 sm:ml-auto" aria-label="Trier les critiques">
          <span class="flex min-w-0 items-center gap-1.5">
            <ArrowUpDown class="text-muted-foreground" />
            <SelectValue />
          </span>
        </SelectTrigger>
        <SelectContent>
          <SelectGroup v-for="group in SORT_GROUPS" :key="group.title">
            <SelectLabel>{{ group.title }}</SelectLabel>
            <SelectItem v-for="[value, label] in group.sorts" :key="value" :value="value">
              {{ label }}
            </SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
    </header>

    <!-- First load -->
    <ul
      v-if="total === null && !error"
      class="divide-y overflow-hidden rounded-xl border bg-card"
      aria-busy="true"
      aria-label="Chargement des critiques"
    >
      <li v-for="n in 5" :key="n" class="flex gap-4 px-4 py-4">
        <div class="aspect-2/3 w-16 shrink-0 animate-pulse rounded-md bg-muted sm:w-20" />
        <div class="flex-1 space-y-3">
          <div
            class="h-5 animate-pulse rounded bg-muted"
            :style="{ width: `${30 + ((n * 17) % 40)}%` }"
          />
          <div class="h-4 w-1/3 animate-pulse rounded bg-muted" />
          <div class="h-4 animate-pulse rounded bg-muted" />
        </div>
      </li>
    </ul>

    <!-- No review -->
    <div
      v-else-if="total === 0"
      class="flex flex-col items-center gap-4 rounded-xl border border-dashed px-6 py-14 text-center"
    >
      <div class="flex size-10 items-center justify-center rounded-full bg-muted">
        <NotebookPen class="size-5 text-muted-foreground" />
      </div>
      <div class="space-y-1">
        <p class="font-medium">Aucune critique pour l’instant</p>
        <p class="text-sm text-muted-foreground">
          Importez votre export Letterboxd pour retrouver ici vos critiques.
        </p>
      </div>
      <Button as-child>
        <RouterLink :to="{ name: 'onboarding-import' }">
          <Download data-icon="inline-start" />
          Importer depuis Letterboxd
        </RouterLink>
      </Button>
    </div>

    <ul
      v-else-if="reviews.length"
      class="divide-y overflow-hidden rounded-xl border bg-card text-card-foreground transition-opacity"
      :class="{ 'opacity-50': isLoading }"
      :aria-busy="isLoading"
    >
      <!-- Not a link as a whole, unlike an entry of the diary: the text can hold links. -->
      <li v-for="review in reviews" :key="review.id">
        <article class="flex gap-4 px-4 py-4">
          <!-- The poster is decorative, and its link the same as the one of the title. -->
          <RouterLink
            :to="{ name: 'film', params: { id: review.film.id } }"
            :class="filmLinkClass"
            class="shrink-0 self-start"
            tabindex="-1"
            aria-hidden="true"
          >
            <img
              v-if="review.film.posterPath"
              :src="posterUrl(review.film.posterPath, 92)"
              :srcset="`${posterUrl(review.film.posterPath, 92)} 1x, ${posterUrl(review.film.posterPath, 185)} 2x`"
              alt=""
              width="92"
              height="138"
              loading="lazy"
              :class="posterClass"
            />
            <div v-else :class="posterClass" class="flex items-center justify-center">
              <Clapperboard class="size-5 text-muted-foreground" />
            </div>
          </RouterLink>

          <div class="flex min-w-0 flex-1 flex-col gap-2">
            <header class="space-y-1">
              <h2 class="text-lg leading-snug">
                <RouterLink
                  :to="{ name: 'film', params: { id: review.film.id } }"
                  :class="filmLinkClass"
                  class="font-medium"
                >
                  {{ review.film.name }}
                </RouterLink>
                <span v-if="review.film.year" class="ml-2 text-base text-muted-foreground">
                  {{ review.film.year }}
                </span>
              </h2>
              <p
                class="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-sm text-muted-foreground"
              >
                <span v-if="review.rating !== null" class="flex text-base text-foreground">
                  <FilmRating :rating="review.rating" />
                </span>
                <template v-if="review.film.liked">
                  <Heart class="size-4 fill-current text-brand" />
                  <span class="sr-only">Aimé</span>
                </template>
                <template v-if="review.rewatch">
                  <Repeat2 class="size-4" />
                  <span class="sr-only">Revu</span>
                </template>
                <!-- The day of the viewing it is about, or else the one it was written. -->
                <time :datetime="review.watchedOn ?? review.reviewedOn">
                  {{ review.watchedOn ? 'Vu le' : 'Écrite le' }}
                  {{ formatDate(review.watchedOn ?? review.reviewedOn) }}
                </time>
              </p>
            </header>
            <ReviewText :text="review.text" />
          </div>
        </article>
      </li>
    </ul>

    <div v-if="error" role="alert" class="flex flex-col items-start gap-3">
      <p class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="size-4 shrink-0" />
        {{ error }}
      </p>
      <Button variant="outline" size="sm" @click="load">Réessayer</Button>
    </div>

    <ListPagination :page="page" :page-count="pageCount" :to="pageLocation" />
  </main>
</template>
