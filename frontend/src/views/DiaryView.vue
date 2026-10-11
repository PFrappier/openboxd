<script lang="ts">
// Last page shown and its URL, kept across visits so that coming back from a film renders
// the diary at once, which lets the router restore the scroll position.
const lastShown: { url?: string; result?: DiaryPage } = {}
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  ArrowUpDown,
  BookOpen,
  CircleAlert,
  Clapperboard,
  Download,
  Heart,
  Repeat2,
  SearchX,
  X,
} from '@lucide/vue'

import FilmRating from '@/components/FilmRating.vue'
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
import { Toggle } from '@/components/ui/toggle'
import { ApiError } from '@/lib/api/client'
import {
  DIARY_SORTS,
  getDiary,
  type DiaryEntry,
  type DiaryFilter,
  type DiaryPage,
  type DiarySort,
} from '@/lib/api/diary'
import { posterUrl } from '@/lib/api/films'
import { formatMonth, formatShortMonth, formatWeekday } from '@/lib/dates'
import { RATINGS, ratingLabel } from '@/lib/ratings'

const PAGE_SIZE = 50

const route = useRoute()
const router = useRouter()

/** Page asked by the URL, from 1; anything else than a page number means the first one. */
const page = computed(() => {
  const asked = Number(route.query.page)
  return Number.isInteger(asked) && asked >= 1 ? asked : 1
})

const FILTER_NAMES = ['year', 'rating', 'liked', 'rewatch'] as const

/** Filters asked by the URL; those left out aren't in it. */
const filter = computed(() => {
  const asked: DiaryFilter = {}
  for (const name of FILTER_NAMES) {
    const value = route.query[name]
    if (typeof value === 'string' && value) asked[name] = value
  }
  return asked
})

const isFiltered = computed(() => Object.keys(filter.value).length > 0)

const DEFAULT_SORT: DiarySort = 'watched-desc'

/** Order asked by the URL; anything else than a known one means the default. */
const sort = computed(() => DIARY_SORTS.find((known) => known === route.query.sort) ?? DEFAULT_SORT)

/**
 * Whether the diary reads as one: by day of viewing, under the months. In any other order
 * it is a plain list, where each entry tells its whole date.
 */
const isChronological = computed(() => sort.value.startsWith('watched-'))

// The first page in the default order has neither in its URL, so that the unfiltered
// diary keeps the URL the rest of the app links to.
const pageLocation = (number: number, ofFilter = filter.value, inSort = sort.value) => ({
  name: 'diary',
  query: {
    ...ofFilter,
    ...(inSort !== DEFAULT_SORT ? { sort: inSort } : {}),
    ...(number > 1 ? { page: number } : {}),
  },
})

/** Changes the order, and goes back to the first page. */
function setSort(value: unknown) {
  const known = DIARY_SORTS.find((known) => known === value)
  if (known) return router.push(pageLocation(1, filter.value, known))
}

// Sorts come in pairs, one for each direction, under the name of what they sort by.
const SORT_GROUPS: { title: string; sorts: [DiarySort, string][] }[] = [
  {
    title: 'Date de visionnage',
    sorts: [
      ['watched-desc', 'Vus le plus récemment'],
      ['watched-asc', 'Vus le plus anciennement'],
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
  {
    title: 'Année de sortie',
    sorts: [
      ['release-desc', 'Sortie la plus récente'],
      ['release-asc', 'Sortie la plus ancienne'],
    ],
  },
]

/** Changes filters, an empty value removing one, and goes back to the first page. */
function setFilter(changes: DiaryFilter) {
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

const entries = ref<DiaryEntry[]>([])
const total = ref<number | null>(null)
/** The years the diary spans, latest first. */
const years = ref<number[]>([])
const isLoading = ref(false)
const error = ref<string | null>(null)

if (lastShown.result && lastShown.url === route.fullPath) {
  entries.value = lastShown.result.entries
  total.value = lastShown.result.total
  years.value = lastShown.result.years
}

/** The years to choose from: the one asked by the URL is there even without a viewing. */
const yearChoices = computed(() => {
  const asked = Number(filter.value.year)
  if (!filter.value.year || years.value.includes(asked)) return years.value.map(String)
  return [filter.value.year, ...years.value.map(String)]
})

const pageCount = computed(() => Math.max(1, Math.ceil((total.value ?? 0) / PAGE_SIZE)))

// Only the answer to the latest request counts: pages can be changed faster than they load.
let latestRequest = 0

async function load() {
  // The URL also changes when leaving the page, which asks for nothing.
  if (route.name !== 'diary') return
  const request = ++latestRequest
  const asked = page.value
  const url = route.fullPath
  isLoading.value = true
  error.value = null
  try {
    const result = await getDiary(
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
    entries.value = result.entries
    total.value = result.total
    years.value = result.years
    Object.assign(lastShown, { url, result })
  } catch (cause) {
    if (request !== latestRequest) return
    entries.value = []
    error.value =
      cause instanceof ApiError
        ? 'Le journal n’a pas pu être chargé.'
        : 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
  } finally {
    if (request === latestRequest) isLoading.value = false
  }
}

watch([page, filter, sort], load, { immediate: true })

/**
 * The entries of the page, under the month they were watched in when the diary is
 * chronological, and else in a single group without a title.
 */
const groups = computed(() => {
  const groups: { key: string; title: string | null; entries: DiaryEntry[] }[] = []
  if (!isChronological.value) {
    return entries.value.length ? [{ key: 'all', title: null, entries: entries.value }] : []
  }
  for (const entry of entries.value) {
    // YYYY-MM
    const key = entry.watchedOn.slice(0, 7)
    const last = groups.at(-1)
    if (last?.key === key) last.entries.push(entry)
    else groups.push({ key, title: formatMonth(entry.watchedOn), entries: [entry] })
  }
  return groups
})

/** Day of the month of a YYYY-MM-DD date, e.g. "5". */
const dayOfMonth = (isoDate: string) => String(Number(isoDate.slice(8, 10)) || '')

const formatCount = (count: number) => count.toLocaleString('fr-FR')

const posterClass = 'aspect-2/3 w-10 shrink-0 rounded-md border bg-muted object-cover sm:w-12'

const emptyStateClass =
  'flex flex-col items-center gap-4 rounded-xl border border-dashed px-6 py-14 text-center'
</script>

<template>
  <main class="mx-auto flex w-full max-w-4xl flex-col gap-5 px-4 py-6 sm:px-6">
    <header class="flex flex-col gap-3">
      <div class="flex items-baseline gap-x-3">
        <h1 class="text-2xl font-semibold tracking-tight">Journal</h1>
        <p v-if="total !== null" class="text-muted-foreground">
          {{ formatCount(total) }} {{ total > 1 ? 'entrées' : 'entrée' }}
        </p>
      </div>

      <div role="group" aria-label="Filtrer le journal" class="flex flex-wrap items-center gap-2">
        <Select
          :model-value="filter.year ?? NO_FILTER"
          @update:model-value="setFilter({ year: selected($event) })"
        >
          <SelectTrigger size="sm" aria-label="Filtrer par année de visionnage">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="NO_FILTER">Toutes les années</SelectItem>
            <SelectItem v-for="year in yearChoices" :key="year" :value="year">
              {{ year }}
            </SelectItem>
          </SelectContent>
        </Select>
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
            <SelectItem value="none">Sans note</SelectItem>
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
        <Toggle
          variant="outline"
          size="sm"
          :model-value="filter.rewatch === 'true'"
          @update:model-value="setFilter({ rewatch: $event ? 'true' : '' })"
        >
          <Repeat2 data-icon="inline-start" :class="{ 'text-muted-foreground': !filter.rewatch }" />
          Revus
        </Toggle>
        <Button v-if="isFiltered" type="button" variant="ghost" size="sm" @click="resetFilters">
          <X data-icon="inline-start" />
          Réinitialiser
        </Button>
        <!-- Pushed to the end of the row; as wide whatever the order, so that it stays put. -->
        <Select :model-value="sort" @update:model-value="setSort">
          <SelectTrigger size="sm" class="w-60 sm:ml-auto" aria-label="Trier le journal">
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

    <!-- First load -->
    <ul
      v-if="total === null && !error"
      class="divide-y overflow-hidden rounded-xl border bg-card"
      aria-busy="true"
      aria-label="Chargement du journal"
    >
      <li v-for="n in 8" :key="n" class="flex items-center gap-4 px-4 py-2.5">
        <div class="h-9 w-9 animate-pulse rounded bg-muted" />
        <div class="aspect-2/3 w-10 animate-pulse rounded-md bg-muted" />
        <div
          class="h-4 animate-pulse rounded bg-muted"
          :style="{ width: `${30 + ((n * 17) % 40)}%` }"
        />
      </li>
    </ul>

    <!-- Nothing left by the filters -->
    <div v-else-if="total === 0 && isFiltered" :class="emptyStateClass">
      <div class="flex size-10 items-center justify-center rounded-full bg-muted">
        <SearchX class="size-5 text-muted-foreground" />
      </div>
      <div class="space-y-1">
        <p class="font-medium">Aucune entrée ne correspond</p>
        <p class="text-sm text-muted-foreground">Essayez de retirer un filtre.</p>
      </div>
      <Button variant="outline" @click="resetFilters">Réinitialiser les filtres</Button>
    </div>

    <!-- Empty diary -->
    <div v-else-if="total === 0" :class="emptyStateClass">
      <div class="flex size-10 items-center justify-center rounded-full bg-muted">
        <BookOpen class="size-5 text-muted-foreground" />
      </div>
      <div class="space-y-1">
        <p class="font-medium">Aucune entrée pour l’instant</p>
        <p class="text-sm text-muted-foreground">
          Importez votre export Letterboxd pour retrouver ici votre journal.
        </p>
      </div>
      <Button as-child>
        <RouterLink :to="{ name: 'onboarding-import' }">
          <Download data-icon="inline-start" />
          Importer depuis Letterboxd
        </RouterLink>
      </Button>
    </div>

    <div
      v-else-if="entries.length"
      class="flex flex-col gap-4 transition-opacity"
      :class="{ 'opacity-50': isLoading }"
      :aria-busy="isLoading"
    >
      <section v-for="group in groups" :key="group.key">
        <!--
          Stays under the top bar while its month scrolls by. Only where the bar has a
          known height: on narrow screens its navigation wraps onto a second row.
        -->
        <h2
          v-if="group.title"
          class="z-[5] bg-background py-2 text-sm font-medium text-muted-foreground first-letter:uppercase md:sticky md:top-[calc(4.5rem+1px)]"
        >
          {{ group.title }}
        </h2>
        <ul class="divide-y overflow-hidden rounded-xl border bg-card text-card-foreground">
          <li v-for="entry in group.entries" :key="entry.id">
            <RouterLink
              :to="{ name: 'film', params: { id: entry.film.id } }"
              class="flex items-center gap-4 px-4 py-2.5 transition-colors outline-none hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:ring-inset sm:py-3"
            >
              <!--
                Under a month the day is enough; elsewhere the month and year come along.
                Its two parts sit side by side where the width allows it.
              -->
              <time
                :datetime="entry.watchedOn"
                class="flex shrink-0 flex-col items-center sm:flex-row sm:items-baseline sm:gap-2"
                :class="isChronological ? 'w-9 sm:w-20' : 'w-18 sm:w-32'"
              >
                <span class="text-xl leading-6 font-semibold tabular-nums sm:w-7 sm:text-right">
                  {{ dayOfMonth(entry.watchedOn) }}
                </span>
                <span class="text-xs whitespace-nowrap text-muted-foreground sm:text-sm">
                  {{
                    isChronological
                      ? formatWeekday(entry.watchedOn)
                      : formatShortMonth(entry.watchedOn)
                  }}
                </span>
              </time>

              <!-- The poster is decorative: the title is right next to it. -->
              <img
                v-if="entry.film.posterPath"
                :src="posterUrl(entry.film.posterPath, 92)"
                :srcset="`${posterUrl(entry.film.posterPath, 92)} 1x, ${posterUrl(entry.film.posterPath, 185)} 2x`"
                alt=""
                width="92"
                height="138"
                loading="lazy"
                :class="posterClass"
              />
              <div v-else :class="posterClass" class="flex items-center justify-center">
                <Clapperboard class="size-4 text-muted-foreground" />
              </div>

              <!-- In a row where the width allows it, and else the signs under the title. -->
              <div class="flex min-w-0 flex-1 flex-col gap-1 sm:flex-row sm:items-center sm:gap-5">
                <p class="min-w-0 flex-1 sm:text-lg">
                  <span class="line-clamp-2 font-medium">{{ entry.film.name }}</span>
                  <span v-if="entry.film.year" class="text-sm text-muted-foreground sm:hidden">
                    {{ entry.film.year }}
                  </span>
                </p>
                <!-- The release year has its own column, kept even without one. -->
                <span class="hidden w-10 text-right text-muted-foreground tabular-nums sm:block">
                  {{ entry.film.year }}
                </span>

                <!-- Each sign has its own slot, so that they line up from an entry to the next. -->
                <div class="flex shrink-0 items-center gap-2 text-sm sm:gap-2.5 sm:text-lg">
                  <!-- As wide as five stars. -->
                  <span class="flex sm:w-[calc(5em+0.5rem)] sm:justify-end">
                    <FilmRating v-if="entry.rating !== null" :rating="entry.rating" />
                  </span>
                  <span class="flex justify-center sm:w-[1em]">
                    <template v-if="entry.rewatch">
                      <Repeat2 class="size-[1em] text-muted-foreground" />
                      <span class="sr-only">Revu</span>
                    </template>
                  </span>
                  <span class="flex justify-center sm:w-[1em]">
                    <template v-if="entry.film.liked">
                      <Heart class="size-[1em] fill-current text-brand" />
                      <span class="sr-only">Aimé</span>
                    </template>
                  </span>
                </div>
              </div>
            </RouterLink>
          </li>
        </ul>
      </section>
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
