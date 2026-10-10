<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { CircleAlert, Clapperboard, Download, LoaderCircle } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api/client'
import { getWatchedFilms, type WatchedFilm } from '@/lib/api/films'

const PAGE_SIZE = 50

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
</script>

<template>
  <main class="mx-auto flex w-full max-w-2xl flex-col gap-8 px-4 py-10">
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
      class="divide-y rounded-xl border bg-card"
      aria-busy="true"
      aria-label="Chargement des films"
    >
      <li v-for="n in 8" :key="n" class="flex h-12 items-center justify-between gap-4 px-4">
        <span
          class="h-4 animate-pulse rounded bg-muted"
          :style="{ width: `${30 + ((n * 17) % 40)}%` }"
        />
        <span class="h-4 w-20 animate-pulse rounded bg-muted" />
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

    <ul v-else-if="films.length" class="divide-y rounded-xl border bg-card text-card-foreground">
      <li
        v-for="film in films"
        :key="film.id"
        class="flex min-h-12 items-center justify-between gap-4 px-4 py-2.5"
      >
        <p class="min-w-0 text-sm">
          <span class="font-medium">{{ film.name }}</span>
          <span v-if="film.year" class="ml-2 text-muted-foreground">{{ film.year }}</span>
        </p>
        <time
          :datetime="film.watchedOn"
          class="shrink-0 text-sm text-muted-foreground tabular-nums"
        >
          {{ formatDate(film.watchedOn) }}
        </time>
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
