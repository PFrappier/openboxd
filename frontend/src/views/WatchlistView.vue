<script lang="ts">
// Last page shown and its URL, kept across visits so that coming back from a film renders
// the watchlist at once, which lets the router restore the scroll position.
const lastShown: { url?: string; result?: WatchlistPage } = {}
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowUpDown, CircleAlert, Download, ListVideo, SearchX, X } from '@lucide/vue'

import FilmGrid from '@/components/FilmGrid.vue'
import ListPagination from '@/components/ListPagination.vue'
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
import {
  getWatchlist,
  WATCHLIST_SORTS,
  type WatchlistFilm,
  type WatchlistFilter,
  type WatchlistPage,
  type WatchlistSort,
} from '@/lib/api/watchlist'
import { DECADES } from '@/lib/decades'

// Splits into whole rows of 3, 5, 6, 10 or 15 films, see FilmGrid.
const PAGE_SIZE = 30

const route = useRoute()
const router = useRouter()

/** Page asked by the URL, from 1; anything else than a page number means the first one. */
const page = computed(() => {
  const asked = Number(route.query.page)
  return Number.isInteger(asked) && asked >= 1 ? asked : 1
})

const FILTER_NAMES = ['decade'] as const

/** Filters asked by the URL; those left out aren't in it. */
const filter = computed(() => {
  const asked: WatchlistFilter = {}
  for (const name of FILTER_NAMES) {
    const value = route.query[name]
    if (typeof value === 'string' && value) asked[name] = value
  }
  return asked
})

const isFiltered = computed(() => Object.keys(filter.value).length > 0)

const DEFAULT_SORT: WatchlistSort = 'added-desc'

/** Order asked by the URL; anything else than a known one means the default. */
const sort = computed(
  () => WATCHLIST_SORTS.find((known) => known === route.query.sort) ?? DEFAULT_SORT,
)

// The first page in the default order has neither in its URL, so that the unfiltered
// watchlist keeps the URL the rest of the app links to.
const pageLocation = (number: number, ofFilter = filter.value, inSort = sort.value) => ({
  name: 'watchlist',
  query: {
    ...ofFilter,
    ...(inSort !== DEFAULT_SORT ? { sort: inSort } : {}),
    ...(number > 1 ? { page: number } : {}),
  },
})

/** Changes the order, and goes back to the first page. */
function setSort(value: unknown) {
  const known = WATCHLIST_SORTS.find((known) => known === value)
  if (known) return router.push(pageLocation(1, filter.value, known))
}

// Sorts come in pairs, one for each direction, under the name of what they sort by.
const SORT_GROUPS: { title: string; sorts: [WatchlistSort, string][] }[] = [
  {
    title: 'Date d’ajout',
    sorts: [
      ['added-desc', 'Ajout le plus récent'],
      ['added-asc', 'Ajout le plus ancien'],
    ],
  },
  {
    title: 'Titre',
    sorts: [
      ['name-asc', 'Titre de A à Z'],
      ['name-desc', 'Titre de Z à A'],
    ],
  },
  {
    title: 'Année de sortie',
    sorts: [
      ['release-desc', 'Sortie la plus récente'],
      ['release-asc', 'Sortie la plus ancienne'],
    ],
  },
  {
    title: 'Durée',
    sorts: [
      ['runtime-desc', 'Les plus longs'],
      ['runtime-asc', 'Les plus courts'],
    ],
  },
]

/** Changes filters, an empty value removing one, and goes back to the first page. */
function setFilter(changes: WatchlistFilter) {
  const changed = { ...filter.value, ...changes }
  for (const name of FILTER_NAMES) {
    if (!changed[name]) delete changed[name]
  }
  return router.push(pageLocation(1, changed))
}

const resetFilters = () => router.push(pageLocation(1, {}))

// Value of the choice of a select that filters nothing: an item can't have an empty one.
const NO_FILTER = 'all'

/** The filter a select was set to, empty for none. */
const selected = (value: unknown) => (typeof value === 'string' && value !== NO_FILTER ? value : '')

const films = ref<WatchlistFilm[]>([])
const total = ref<number | null>(null)
const isLoading = ref(false)
const error = ref<string | null>(null)

if (lastShown.result && lastShown.url === route.fullPath) {
  films.value = lastShown.result.films
  total.value = lastShown.result.total
}

const pageCount = computed(() => Math.max(1, Math.ceil((total.value ?? 0) / PAGE_SIZE)))

// Only the answer to the latest request counts: pages can be changed faster than they load.
let latestRequest = 0

async function load() {
  // The URL also changes when leaving the page, which asks for nothing.
  if (route.name !== 'watchlist') return
  const request = ++latestRequest
  const asked = page.value
  const url = route.fullPath
  isLoading.value = true
  error.value = null
  try {
    const result = await getWatchlist(
      { limit: PAGE_SIZE, offset: (asked - 1) * PAGE_SIZE },
      filter.value,
      sort.value,
    )
    if (request !== latestRequest) return
    const lastPage = Math.max(1, Math.ceil(result.total / PAGE_SIZE))
    if (asked > lastPage) {
      await router.replace(pageLocation(lastPage))
      return
    }
    films.value = result.films
    total.value = result.total
    Object.assign(lastShown, { url, result })
  } catch (cause) {
    if (request !== latestRequest) return
    films.value = []
    error.value =
      cause instanceof ApiError
        ? 'La watchlist n’a pas pu être chargée.'
        : 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
  } finally {
    if (request === latestRequest) isLoading.value = false
  }
}

watch([page, filter, sort], load, { immediate: true })

const formatCount = (count: number) => count.toLocaleString('fr-FR')

const emptyStateClass =
  'flex flex-col items-center gap-4 rounded-xl border border-dashed px-6 py-14 text-center'
</script>

<template>
  <!-- Kept compact: a whole page of films and its pagination are meant to fit the screen. -->
  <main class="mx-auto flex w-full max-w-7xl flex-col gap-5 px-4 py-6 sm:px-6">
    <header class="flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
      <div class="flex items-baseline gap-x-3">
        <h1 class="text-2xl font-semibold tracking-tight">Watchlist</h1>
        <p v-if="total !== null" class="text-muted-foreground">
          {{ formatCount(total) }} {{ total > 1 ? 'films' : 'film' }}
        </p>
      </div>

      <div role="group" aria-label="Filtrer la watchlist" class="flex flex-wrap items-center gap-2">
        <Button v-if="isFiltered" type="button" variant="ghost" size="sm" @click="resetFilters">
          <X data-icon="inline-start" />
          Réinitialiser
        </Button>
        <Select
          :model-value="filter.decade ?? NO_FILTER"
          @update:model-value="setFilter({ decade: selected($event) })"
        >
          <SelectTrigger size="sm" aria-label="Filtrer par décennie">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="NO_FILTER">Toutes les décennies</SelectItem>
            <SelectItem v-for="decade in DECADES" :key="decade" :value="String(decade)">
              Années {{ decade }}
            </SelectItem>
          </SelectContent>
        </Select>
        <!-- As wide whatever the order, so that the filters don't move when it changes. -->
        <Select :model-value="sort" @update:model-value="setSort">
          <SelectTrigger size="sm" class="w-64" aria-label="Trier la watchlist">
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
      </div>
    </header>

    <!-- Nothing left by the filters -->
    <div v-if="total === 0 && isFiltered" :class="emptyStateClass">
      <div class="flex size-10 items-center justify-center rounded-full bg-muted">
        <SearchX class="size-5 text-muted-foreground" />
      </div>
      <div class="space-y-1">
        <p class="font-medium">Aucun film ne correspond</p>
        <p class="text-sm text-muted-foreground">Essayez de retirer un filtre.</p>
      </div>
      <Button variant="outline" @click="resetFilters">Réinitialiser les filtres</Button>
    </div>

    <!-- Empty watchlist -->
    <div v-else-if="total === 0" :class="emptyStateClass">
      <div class="flex size-10 items-center justify-center rounded-full bg-muted">
        <ListVideo class="size-5 text-muted-foreground" />
      </div>
      <div class="space-y-1">
        <p class="font-medium">Aucun film à voir pour l’instant</p>
        <p class="text-sm text-muted-foreground">
          Importez votre export Letterboxd pour retrouver ici votre watchlist.
        </p>
      </div>
      <Button as-child>
        <RouterLink :to="{ name: 'onboarding-import' }">
          <Download data-icon="inline-start" />
          Importer depuis Letterboxd
        </RouterLink>
      </Button>
    </div>

    <!-- Placeholders during the first load, then the films. -->
    <FilmGrid
      v-else-if="films.length || !error"
      :films="films"
      :page-size="PAGE_SIZE"
      :pending="total === null"
      :busy="isLoading"
    />

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
