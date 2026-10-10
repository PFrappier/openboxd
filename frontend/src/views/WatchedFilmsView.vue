<script lang="ts">
// Last page shown and its URL, kept across visits so that coming back from a film renders
// the list at once, which lets the router restore the scroll position.
const lastShown: { url?: string; result?: WatchedFilmsPage } = {}
</script>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useTemplateRef, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  Clapperboard,
  Download,
  Heart,
  SearchX,
  X,
} from '@lucide/vue'

import FilmRating from '@/components/FilmRating.vue'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Toggle } from '@/components/ui/toggle'
import { ApiError } from '@/lib/api/client'
import {
  getWatchedFilms,
  posterUrl,
  type WatchedFilm,
  type WatchedFilmsFilter,
  type WatchedFilmsPage,
} from '@/lib/api/films'

// Splits into whole rows of 3, 5, 6, 10 or 15 films, see fitGrid.
const PAGE_SIZE = 30

const route = useRoute()
const router = useRouter()

/** Page asked by the URL, from 1; anything else than a page number means the first one. */
const page = computed(() => {
  const asked = Number(route.query.page)
  return Number.isInteger(asked) && asked >= 1 ? asked : 1
})

const FILTER_NAMES = ['rating', 'liked', 'decade'] as const

/** Filters asked by the URL; those left out aren't in it. */
const filter = computed(() => {
  const asked: WatchedFilmsFilter = {}
  for (const name of FILTER_NAMES) {
    const value = route.query[name]
    if (typeof value === 'string' && value) asked[name] = value
  }
  return asked
})

const isFiltered = computed(() => Object.keys(filter.value).length > 0)

// The first page has no page number, so that the unfiltered list keeps the URL the rest
// of the app links to.
const pageLocation = (number: number, ofFilter = filter.value) => ({
  name: 'films',
  query: { ...ofFilter, ...(number > 1 ? { page: number } : {}) },
})

/** Changes filters, an empty value removing one, and goes back to the first page. */
function setFilter(changes: WatchedFilmsFilter) {
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

const RATINGS = [5, 4.5, 4, 3.5, 3, 2.5, 2, 1.5, 1, 0.5]

/** e.g. "★★★½". */
const ratingLabel = (stars: number) => '★'.repeat(Math.floor(stars)) + (stars % 1 ? '½' : '')

// From the current decade back to the first films.
const DECADES = Array.from(
  { length: (Math.floor(new Date().getFullYear() / 10) * 10 - 1880) / 10 + 1 },
  (_, index) => Math.floor(new Date().getFullYear() / 10) * 10 - index * 10,
)

const films = ref<WatchedFilm[]>([])
const total = ref<number | null>(null)
const isLoading = ref(false)
const error = ref<string | null>(null)

if (lastShown.result && lastShown.url === route.fullPath) {
  films.value = lastShown.result.films
  total.value = lastShown.result.total
}

const pageCount = computed(() => Math.max(1, Math.ceil((total.value ?? 0) / PAGE_SIZE)))

// Number of slots of the pagination when there are too many pages to link to them all.
const PAGE_SLOTS = 7

/**
 * Pages to link to: the first, the last and those around the current one; null is a gap.
 * There are as many slots on every page, so that the arrows stay where they are and can
 * be pressed again and again.
 */
const pageLinks = computed<(number | null)[]>(() => {
  const count = pageCount.value
  const range = (from: number, length: number) => Array.from({ length }, (_, i) => from + i)
  if (count <= PAGE_SLOTS) return range(1, count)
  // Close to an end, the pages run up to it without a gap.
  const run = PAGE_SLOTS - 2
  if (page.value < run) return [...range(1, run), null, count]
  if (page.value > count - run + 1) return [1, null, ...range(count - run + 1, run)]
  return [1, null, page.value - 1, page.value, page.value + 1, null, count]
})

// Wide enough for the longest page number, so that all the slots are the same width.
const pageSlotStyle = computed(() => ({
  width: `calc(${formatCount(pageCount.value).length}ch + 1.25rem)`,
}))

// Only the answer to the latest request counts: pages can be changed faster than they load.
let latestRequest = 0

async function load() {
  // The URL also changes when leaving the page, which asks for nothing.
  if (route.name !== 'films') return
  const request = ++latestRequest
  const asked = page.value
  const url = route.fullPath
  isLoading.value = true
  error.value = null
  try {
    const result = await getWatchedFilms(
      { limit: PAGE_SIZE, offset: (asked - 1) * PAGE_SIZE },
      filter.value,
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
        ? 'Les films n’ont pas pu être chargés.'
        : 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
  } finally {
    if (request === latestRequest) isLoading.value = false
  }
}

watch([page, filter], load, { immediate: true })

const formatCount = (count: number) => count.toLocaleString('fr-FR')

// In pixels: the gap of the grid, what a card adds under its poster for the rating,
// and the width under which posters get too small to be told apart.
const GRID_GAP = 12
const CARD_FOOTER = 22
const MIN_CARD_WIDTH = 64

interface GridLayout {
  columns: number
  /** Width of a card, in pixels. */
  cardWidth: number
}

/**
 * Lays a full page of films out in whole rows, with the largest cards that fit both the
 * width and the height given. When the space is too small for that, the height gives in:
 * the cards keep a decent size and the page scrolls.
 */
function fitGrid(width: number, height: number): GridLayout {
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
    // Posters are 2:3.
    cardWidth: Math.floor(
      Math.min(widthFor(columns), ((heightFor(columns) - CARD_FOOTER) * 2) / 3),
    ),
  }))
  const best = layouts.reduce((a, b) => (b.cardWidth > a.cardWidth ? b : a))
  if (best.cardWidth >= MIN_CARD_WIDTH) return best

  // The most columns the width allows make for the shortest scroll.
  const columns = Math.max(1, ...candidates.filter((c) => widthFor(c) >= MIN_CARD_WIDTH))
  return { columns, cardWidth: Math.floor(widthFor(columns)) }
}

const pageElement = useTemplateRef('page')
const gridArea = useTemplateRef('gridArea')
const layout = ref<GridLayout | null>(null)

function updateLayout() {
  if (!pageElement.value || !gridArea.value) return
  const pageBox = pageElement.value.getBoundingClientRect()
  const areaBox = gridArea.value.getBoundingClientRect()
  // What the screen leaves to the grid once everything above and under it is counted:
  // the top bar and the title above, the pagination under.
  const above = areaBox.top + window.scrollY
  const under = pageBox.bottom - areaBox.bottom
  const fitted = fitGrid(gridArea.value.clientWidth, window.innerHeight - above - under)
  if (fitted.columns !== layout.value?.columns || fitted.cardWidth !== layout.value.cardWidth) {
    layout.value = fitted
  }
}

// Also covers what changes the space left without resizing the window, such as the
// pagination showing up once the number of films is known.
const pageObserver = new ResizeObserver(updateLayout)

onMounted(() => {
  updateLayout()
  if (pageElement.value) pageObserver.observe(pageElement.value)
  window.addEventListener('resize', updateLayout)
})

onBeforeUnmount(() => {
  pageObserver.disconnect()
  window.removeEventListener('resize', updateLayout)
})

const gridStyle = computed(() => {
  if (!layout.value) return undefined
  const { columns, cardWidth } = layout.value
  const rows = PAGE_SIZE / columns
  return {
    gridTemplateColumns: `repeat(${columns}, ${cardWidth}px)`,
    // As tall as a full page even when the last one isn't, so that the pagination stays put.
    minHeight: `${rows * (cardWidth * 1.5 + CARD_FOOTER) + (rows - 1) * GRID_GAP}px`,
  }
})

// Five stars and a heart are 6em and 14px of gaps wide: shrunk to fit the narrowest cards.
const ratingStyle = computed(() => ({
  fontSize: `${Math.min(14, ((layout.value?.cardWidth ?? 100) - 14) / 6)}px`,
}))

const gridClass = 'grid content-start justify-center gap-3'

/** Lets the browser pick the poster matching the card's width and the screen's density. */
function posterSrcset(posterPath: string) {
  return ([185, 342, 500] as const)
    .map((width) => `${posterUrl(posterPath, width)} ${width}w`)
    .join(', ')
}

const posterClass =
  'aspect-2/3 w-full rounded-lg border bg-muted object-cover transition-colors group-hover:border-foreground/40 group-focus-visible:border-ring group-focus-visible:ring-3 group-focus-visible:ring-ring/50'
</script>

<template>
  <!-- Kept compact: a whole page of films and its pagination are meant to fit the screen. -->
  <main ref="page" class="mx-auto flex w-full max-w-7xl flex-col gap-5 px-4 py-6 sm:px-6">
    <header class="flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
      <div class="flex items-baseline gap-x-3">
        <h1 class="text-2xl font-semibold tracking-tight">Films vus</h1>
        <p v-if="total !== null" class="text-muted-foreground">
          {{ formatCount(total) }} {{ total > 1 ? 'films' : 'film' }}
        </p>
      </div>

      <div role="group" aria-label="Filtrer les films" class="flex flex-wrap items-center gap-2">
        <Button v-if="isFiltered" type="button" variant="ghost" size="sm" @click="resetFilters">
          <X data-icon="inline-start" />
          Réinitialiser
        </Button>
        <Select
          :model-value="filter.rating ?? NO_FILTER"
          @update:model-value="setFilter({ rating: selected($event) })"
        >
          <SelectTrigger size="sm" aria-label="Filtrer par note">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="NO_FILTER">Toutes les notes</SelectItem>
            <SelectItem v-for="stars in RATINGS" :key="stars" :value="String(stars)">
              {{ ratingLabel(stars) }}
            </SelectItem>
            <SelectItem value="none">Non notés</SelectItem>
          </SelectContent>
        </Select>
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
        <Toggle
          variant="outline"
          size="sm"
          :model-value="filter.liked === 'true'"
          @update:model-value="setFilter({ liked: $event ? 'true' : '' })"
        >
          <Heart
            data-icon="inline-start"
            :class="filter.liked ? 'fill-current text-brand' : 'text-muted-foreground'"
          />
          Aimés
        </Toggle>
      </div>
    </header>

    <div ref="gridArea">
      <!-- First load -->
      <ul
        v-if="total === null && !error"
        :class="gridClass"
        :style="gridStyle"
        aria-busy="true"
        aria-label="Chargement des films"
      >
        <li v-for="n in PAGE_SIZE" :key="n">
          <div class="aspect-2/3 animate-pulse rounded-lg bg-muted" />
        </li>
      </ul>

      <!-- Nothing left by the filters -->
      <div
        v-else-if="total === 0 && isFiltered"
        class="flex flex-col items-center gap-4 rounded-xl border border-dashed px-6 py-14 text-center"
      >
        <div class="flex size-10 items-center justify-center rounded-full bg-muted">
          <SearchX class="size-5 text-muted-foreground" />
        </div>
        <div class="space-y-1">
          <p class="font-medium">Aucun film ne correspond</p>
          <p class="text-sm text-muted-foreground">Essayez de retirer un filtre.</p>
        </div>
        <Button variant="outline" @click="resetFilters">Réinitialiser les filtres</Button>
      </div>

      <!-- Empty library -->
      <div
        v-else-if="total === 0"
        class="flex flex-col items-center gap-4 rounded-xl border border-dashed px-6 py-14 text-center"
      >
        <div class="flex size-10 items-center justify-center rounded-full bg-muted">
          <Clapperboard class="size-5 text-muted-foreground" />
        </div>
        <div class="space-y-1">
          <p class="font-medium">Aucun film pour l’instant</p>
          <p class="text-sm text-muted-foreground">
            Importez votre export Letterboxd pour retrouver ici vos films vus.
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
        v-else-if="films.length"
        :class="[gridClass, { 'opacity-50': isLoading }]"
        class="transition-opacity"
        :style="gridStyle"
        :aria-busy="isLoading"
      >
        <li v-for="film in films" :key="film.id">
          <RouterLink
            :to="{ name: 'film', params: { id: film.id } }"
            class="group relative flex flex-col gap-1.5 outline-none"
          >
            <!-- The poster is decorative: the tooltip names the film. -->
            <img
              v-if="film.posterPath"
              :src="posterUrl(film.posterPath, 185)"
              :srcset="posterSrcset(film.posterPath)"
              :sizes="`${layout?.cardWidth ?? 185}px`"
              alt=""
              width="342"
              height="513"
              loading="lazy"
              :class="posterClass"
            />
            <!-- Without a poster, the title is the only way to tell the film apart. -->
            <div
              v-else
              :class="posterClass"
              class="flex flex-col items-center justify-center gap-1 p-2 text-center text-xs"
              aria-hidden="true"
            >
              <Clapperboard class="size-6 text-muted-foreground" />
              <p class="line-clamp-3 font-medium">{{ film.name }}</p>
              <p v-if="film.year" class="text-muted-foreground">{{ film.year }}</p>
            </div>
            <!-- Stays within the page's side padding, even on the outer columns. -->
            <span
              class="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1.5 w-max max-w-[calc(100%+2rem)] -translate-x-1/2 rounded-md bg-foreground px-2 py-1 text-center text-xs font-medium text-background opacity-0 shadow-md transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
            >
              {{ film.name }}
              <span v-if="film.year" class="font-normal opacity-70">{{ film.year }}</span>
            </span>
            <!-- Always there, so that every card is as tall as fitGrid expects. -->
            <div class="flex h-4 items-center gap-1.5" :style="ratingStyle">
              <FilmRating v-if="film.rating !== null" :rating="film.rating" />
              <template v-if="film.liked">
                <Heart class="size-[1em] fill-current text-brand" />
                <span class="sr-only">Aimé</span>
              </template>
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

    <nav
      v-if="pageCount > 1"
      aria-label="Pagination"
      class="flex flex-wrap items-center justify-center gap-1"
    >
      <Button v-if="page > 1" variant="ghost" size="icon-sm" as-child>
        <RouterLink :to="pageLocation(page - 1)" aria-label="Page précédente">
          <ChevronLeft />
        </RouterLink>
      </Button>
      <Button v-else variant="ghost" size="icon-sm" disabled aria-label="Page précédente">
        <ChevronLeft />
      </Button>

      <template v-for="(number, index) in pageLinks" :key="index">
        <span
          v-if="number === null"
          class="text-center text-sm text-muted-foreground"
          :style="pageSlotStyle"
        >
          …
        </span>
        <Button
          v-else
          :variant="number === page ? 'secondary' : 'ghost'"
          size="sm"
          class="px-0 tabular-nums"
          :style="pageSlotStyle"
          as-child
        >
          <RouterLink
            :to="pageLocation(number)"
            :aria-label="`Page ${number}`"
            :aria-current="number === page ? 'page' : undefined"
          >
            {{ formatCount(number) }}
          </RouterLink>
        </Button>
      </template>

      <Button v-if="page < pageCount" variant="ghost" size="icon-sm" as-child>
        <RouterLink :to="pageLocation(page + 1)" aria-label="Page suivante">
          <ChevronRight />
        </RouterLink>
      </Button>
      <Button v-else variant="ghost" size="icon-sm" disabled aria-label="Page suivante">
        <ChevronRight />
      </Button>
    </nav>
  </main>
</template>
