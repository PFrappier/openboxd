<script lang="ts">
// Last page shown and its URL, kept across visits so that coming back from a list renders
// the lists at once, which lets the router restore the scroll position.
const lastShown: { url?: string; result?: FilmListsPage } = {}
</script>

<script setup lang="ts">
import { computed, ref, useTemplateRef, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowUpDown, CircleAlert, Download, List } from '@lucide/vue'

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
import { posterUrl } from '@/lib/api/films'
import {
  getLists,
  LISTS_SORTS,
  type FilmList,
  type FilmListsPage,
  type ListsSort,
} from '@/lib/api/lists'
import { filmGridHeight, fitFilmGrid } from '@/lib/film-grid'
import { useFittedLayout } from '@/lib/fitted-layout'

// Splits into whole rows of 1, 2, 3, 4 or 6 lists.
const PAGE_SIZE = 12
// Number of films of a page of the watched films and of the watchlist.
const FILMS_PAGE_SIZE = 30

// Number of posters a list shows as a preview: as many slots whatever its number of films,
// so that every card looks the same.
const POSTER_SLOTS = 5

const route = useRoute()
const router = useRouter()

/** Page asked by the URL, from 1; anything else than a page number means the first one. */
const page = computed(() => {
  const asked = Number(route.query.page)
  return Number.isInteger(asked) && asked >= 1 ? asked : 1
})

const DEFAULT_SORT: ListsSort = 'created-desc'

/** Order asked by the URL; anything else than a known one means the default. */
const sort = computed(() => LISTS_SORTS.find((known) => known === route.query.sort) ?? DEFAULT_SORT)

// The first page in the default order has neither in its URL, so that the lists keep the
// URL the rest of the app links to.
const pageLocation = (number: number, inSort = sort.value) => ({
  name: 'lists',
  query: {
    ...(inSort !== DEFAULT_SORT ? { sort: inSort } : {}),
    ...(number > 1 ? { page: number } : {}),
  },
})

/** Changes the order, and goes back to the first page. */
function setSort(value: unknown) {
  const known = LISTS_SORTS.find((known) => known === value)
  if (known) return router.push(pageLocation(1, known))
}

// Sorts come in pairs, one for each direction, under the name of what they sort by.
const SORT_GROUPS: { title: string; sorts: [ListsSort, string][] }[] = [
  {
    title: 'Date de création',
    sorts: [
      ['created-desc', 'Les plus récentes'],
      ['created-asc', 'Les plus anciennes'],
    ],
  },
  {
    title: 'Nom',
    sorts: [
      ['name-asc', 'Nom de A à Z'],
      ['name-desc', 'Nom de Z à A'],
    ],
  },
  {
    title: 'Nombre de films',
    sorts: [
      ['films-desc', 'Les plus longues'],
      ['films-asc', 'Les plus courtes'],
    ],
  },
]

const lists = ref<FilmList[]>([])
const total = ref<number | null>(null)
const isLoading = ref(false)
const error = ref<string | null>(null)

if (lastShown.result && lastShown.url === route.fullPath) {
  lists.value = lastShown.result.lists
  total.value = lastShown.result.total
}

const pageCount = computed(() => Math.max(1, Math.ceil((total.value ?? 0) / PAGE_SIZE)))

// Only the answer to the latest request counts: pages can be changed faster than they load.
let latestRequest = 0

async function load() {
  // The URL also changes when leaving the page, which asks for nothing.
  if (route.name !== 'lists') return
  const request = ++latestRequest
  const asked = page.value
  const url = route.fullPath
  isLoading.value = true
  error.value = null
  try {
    const result = await getLists({ limit: PAGE_SIZE, offset: (asked - 1) * PAGE_SIZE }, sort.value)
    if (request !== latestRequest) return
    const lastPage = Math.max(1, Math.ceil(result.total / PAGE_SIZE))
    if (asked > lastPage) {
      await router.replace(pageLocation(lastPage))
      return
    }
    lists.value = result.lists
    total.value = result.total
    Object.assign(lastShown, { url, result })
  } catch (cause) {
    if (request !== latestRequest) return
    lists.value = []
    error.value =
      cause instanceof ApiError
        ? 'Les listes n’ont pas pu être chargées.'
        : 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
  } finally {
    if (request === latestRequest) isLoading.value = false
  }
}

watch([page, sort], load, { immediate: true })

const formatCount = (count: number) => count.toLocaleString('fr-FR')

// In pixels: the gap of the grid, what a card adds under its posters for the name of the
// list and its number of films, and the width under which a card gets too small to read.
const GRID_GAP = 20
const CARD_FOOTER = 56
const MIN_CARD_WIDTH = 200
// Height of the posters of a card for its width: five 2:3 posters, each 28% of it wide.
const POSTERS_RATIO = 0.42

interface GridLayout {
  columns: number
  /** Width of a card, in pixels. */
  cardWidth: number
  /** Height the grid is given, in pixels. */
  height: number
}

/**
 * Lays a full page of lists out in whole rows, with the largest cards that fit both the
 * width and the height given, as FilmGrid does for films. When the space is too small for
 * that, the height gives in: the cards keep a decent size and the page scrolls.
 *
 * The height to fit is the one a page of films takes in the same space rather than all of
 * it: the grid is then as tall as theirs, and the pagination sits where it does under them.
 */
function fitGrid(width: number, available: number): GridLayout {
  const height = filmGridHeight(fitFilmGrid(width, available, FILMS_PAGE_SIZE), FILMS_PAGE_SIZE)
  const widthFor = (columns: number) => (width - GRID_GAP * (columns - 1)) / columns
  const heightFor = (columns: number) => {
    const rows = PAGE_SIZE / columns
    return (height - GRID_GAP * (rows - 1)) / rows
  }
  const candidates: number[] = []
  for (let columns = 1; columns <= PAGE_SIZE; columns++) {
    if (PAGE_SIZE % columns === 0) candidates.push(columns)
  }
  const layouts = candidates.map((columns) => ({
    columns,
    cardWidth: Math.floor(
      Math.min(widthFor(columns), (heightFor(columns) - CARD_FOOTER) / POSTERS_RATIO),
    ),
  }))
  const fitted = layouts.reduce((a, b) => (b.cardWidth > a.cardWidth ? b : a))
  if (fitted.cardWidth >= MIN_CARD_WIDTH) return { ...fitted, height }
  // The most columns the width allows make for the shortest scroll.
  const columns = Math.max(1, ...candidates.filter((c) => widthFor(c) >= MIN_CARD_WIDTH))
  const cardWidth = Math.floor(widthFor(columns))
  const rows = PAGE_SIZE / columns
  const rowHeight = cardWidth * POSTERS_RATIO + CARD_FOOTER
  return { columns, cardWidth, height: Math.max(height, rows * rowHeight + (rows - 1) * GRID_GAP) }
}

const layout = useFittedLayout(useTemplateRef('gridArea'), fitGrid)

const gridStyle = computed(() => {
  if (!layout.value) return undefined
  const { columns, cardWidth, height } = layout.value
  return {
    gridTemplateColumns: `repeat(${columns}, ${cardWidth}px)`,
    // As tall as a full page even when the last one isn't, so that the pagination stays put.
    minHeight: `${height}px`,
  }
})

const gridClass = 'grid content-start justify-center gap-5'

// Five posters, each over the next one by 10% of the card: 5 × 28% − 4 × 10% fills it.
const posterClass = 'aspect-2/3 w-[28%] shrink-0 rounded-md border bg-muted object-cover shadow-md'
</script>

<template>
  <!-- Kept compact: a whole page of lists and its pagination are meant to fit the screen. -->
  <main class="mx-auto flex w-full max-w-7xl flex-col gap-5 px-4 py-6 sm:px-6">
    <header class="flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
      <div class="flex items-baseline gap-x-3">
        <h1 class="text-2xl font-semibold tracking-tight">Listes</h1>
        <p v-if="total !== null" class="text-muted-foreground">
          {{ formatCount(total) }} {{ total > 1 ? 'listes' : 'liste' }}
        </p>
      </div>

      <Select :model-value="sort" @update:model-value="setSort">
        <SelectTrigger size="sm" class="w-64" aria-label="Trier les listes">
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

    <div ref="gridArea">
      <!-- No list -->
      <div
        v-if="total === 0"
        class="flex flex-col items-center gap-4 rounded-xl border border-dashed px-6 py-14 text-center"
      >
        <div class="flex size-10 items-center justify-center rounded-full bg-muted">
          <List class="size-5 text-muted-foreground" />
        </div>
        <div class="space-y-1">
          <p class="font-medium">Aucune liste pour l’instant</p>
          <p class="text-sm text-muted-foreground">
            Importez votre export Letterboxd pour retrouver ici vos listes.
          </p>
        </div>
        <Button as-child>
          <RouterLink :to="{ name: 'onboarding-import' }">
            <Download data-icon="inline-start" />
            Importer depuis Letterboxd
          </RouterLink>
        </Button>
      </div>

      <!-- Placeholders during the first load -->
      <ul
        v-else-if="total === null && !error"
        :class="gridClass"
        :style="gridStyle"
        aria-busy="true"
        aria-label="Chargement des listes"
      >
        <li v-for="n in PAGE_SIZE" :key="n" class="space-y-3">
          <div class="aspect-100/42 animate-pulse rounded-md bg-muted" />
          <div class="h-5 w-2/3 animate-pulse rounded bg-muted" />
        </li>
      </ul>

      <ul
        v-else-if="lists.length"
        :class="[gridClass, { 'opacity-50': isLoading }]"
        class="transition-opacity"
        :style="gridStyle"
        :aria-busy="isLoading"
      >
        <li v-for="list in lists" :key="list.id">
          <RouterLink
            :to="{ name: 'list', params: { id: list.id } }"
            class="group flex flex-col gap-3 rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <!-- The posters are decorative: the name of the list is right under them. -->
            <div class="flex" aria-hidden="true">
              <template v-for="slot in POSTER_SLOTS" :key="slot">
                <img
                  v-if="list.posters[slot - 1]"
                  :src="posterUrl(list.posters[slot - 1]!, 154)"
                  :srcset="`${posterUrl(list.posters[slot - 1]!, 154)} 1x, ${posterUrl(list.posters[slot - 1]!, 342)} 2x`"
                  alt=""
                  width="154"
                  height="231"
                  loading="lazy"
                  :class="[posterClass, { '-ml-[10%]': slot > 1 }]"
                  :style="{ zIndex: POSTER_SLOTS - slot }"
                />
                <div
                  v-else
                  :class="[posterClass, { '-ml-[10%]': slot > 1 }]"
                  :style="{ zIndex: POSTER_SLOTS - slot }"
                />
              </template>
            </div>
            <div class="min-w-0">
              <h2 class="truncate font-medium underline-offset-4 group-hover:underline">
                {{ list.name }}
              </h2>
              <p class="text-sm text-muted-foreground">
                {{ formatCount(list.filmCount) }} {{ list.filmCount > 1 ? 'films' : 'film' }}
              </p>
            </div>
          </RouterLink>
        </li>
      </ul>
    </div>

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
