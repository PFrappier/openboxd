<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { CircleAlert, Clapperboard, Download, LoaderCircle } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api/client'
import { getWatchedFilms, posterUrl, type WatchedFilm } from '@/lib/api/films'

// Fills whole rows whatever the number of columns, from 2 to 6.
const PAGE_SIZE = 60

const films = ref<WatchedFilm[]>([])
const total = ref<number | null>(null)
const isLoading = ref(false)
const error = ref<string | null>(null)

const hasMore = computed(() => total.value !== null && films.value.length < total.value)

async function loadMore() {
  if (isLoading.value) return
  isLoading.value = true
  error.value = null
  try {
    const page = await getWatchedFilms({ limit: PAGE_SIZE, offset: films.value.length })
    films.value.push(...page.films)
    total.value = page.total
  } catch (cause) {
    error.value =
      cause instanceof ApiError
        ? 'Les films n’ont pas pu être chargés.'
        : 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
  } finally {
    isLoading.value = false
  }
}

onMounted(loadMore)

const dateFormat = new Intl.DateTimeFormat('fr-FR', { dateStyle: 'medium', timeZone: 'UTC' })

/** Formats a YYYY-MM-DD date, read as UTC so the day never shifts with the time zone. */
function formatDate(isoDate: string) {
  const date = new Date(`${isoDate}T00:00:00Z`)
  return Number.isNaN(date.getTime()) ? isoDate : dateFormat.format(date)
}

const formatCount = (count: number) => count.toLocaleString('fr-FR')

const gridClass =
  'grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6'

/** Lets the browser pick the poster matching the card's width and the screen's density. */
function posterSrcset(posterPath: string) {
  return ([185, 342, 500] as const)
    .map((width) => `${posterUrl(posterPath, width)} ${width}w`)
    .join(', ')
}

// Width of a card for each number of columns of gridClass.
const posterSizes =
  '(min-width: 80rem) 200px, (min-width: 64rem) 20vw, (min-width: 48rem) 25vw, (min-width: 40rem) 33vw, 50vw'
</script>

<template>
  <main class="mx-auto flex w-full max-w-7xl flex-col gap-8 px-4 py-10 sm:px-6">
    <header class="space-y-1">
      <h1 class="text-3xl font-semibold tracking-tight">Films vus</h1>
      <p v-if="total !== null" class="text-muted-foreground">
        {{ formatCount(total) }} {{ total > 1 ? 'films' : 'film' }}
      </p>
      <p v-else class="text-muted-foreground">&nbsp;</p>
    </header>

    <!-- First load -->
    <ul
      v-if="total === null && !error"
      :class="gridClass"
      aria-busy="true"
      aria-label="Chargement des films"
    >
      <li v-for="n in 12" :key="n" class="overflow-hidden rounded-xl border bg-card">
        <div class="aspect-2/3 animate-pulse bg-muted" />
        <div class="space-y-2 p-3">
          <div
            class="h-4 animate-pulse rounded bg-muted"
            :style="{ width: `${50 + ((n * 17) % 40)}%` }"
          />
          <div class="h-3 w-1/2 animate-pulse rounded bg-muted" />
        </div>
      </li>
    </ul>

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

    <ul v-else-if="films.length" :class="gridClass">
      <li
        v-for="film in films"
        :key="film.id"
        class="flex flex-col overflow-hidden rounded-xl border bg-card text-card-foreground"
      >
        <!-- The poster is decorative: the title is right under it. -->
        <img
          v-if="film.posterPath"
          :src="posterUrl(film.posterPath, 342)"
          :srcset="posterSrcset(film.posterPath)"
          :sizes="posterSizes"
          alt=""
          width="342"
          height="513"
          loading="lazy"
          class="aspect-2/3 w-full bg-muted object-cover"
        />
        <div v-else class="flex aspect-2/3 w-full items-center justify-center bg-muted">
          <Clapperboard class="size-8 text-muted-foreground" />
        </div>
        <div class="flex flex-1 flex-col gap-0.5 p-3 text-sm">
          <p class="line-clamp-2 font-medium">{{ film.name }}</p>
          <p class="truncate text-muted-foreground">
            <span v-if="film.year">{{ film.year }}</span>
            <span v-if="film.year && film.directors.length"> · </span>
            <span v-if="film.directors.length">{{ film.directors.join(', ') }}</span>
          </p>
          <p class="mt-auto pt-2 text-xs text-muted-foreground">
            Vu le
            <time :datetime="film.watchedOn">{{ formatDate(film.watchedOn) }}</time>
          </p>
        </div>
      </li>
    </ul>

    <div v-if="error" role="alert" class="flex flex-col items-start gap-3">
      <p class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="size-4 shrink-0" />
        {{ error }}
      </p>
      <Button variant="outline" size="sm" @click="loadMore">Réessayer</Button>
    </div>

    <footer v-else-if="films.length && total !== null" class="flex flex-col items-center gap-3">
      <p class="text-sm text-muted-foreground">
        {{ formatCount(films.length) }} sur {{ formatCount(total) }}
      </p>
      <Button v-if="hasMore" variant="outline" :disabled="isLoading" @click="loadMore">
        <LoaderCircle v-if="isLoading" data-icon="inline-start" class="animate-spin" />
        Afficher plus
      </Button>
    </footer>
  </main>
</template>
