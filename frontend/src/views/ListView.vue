<script lang="ts">
// Last page shown and its URL, kept across visits so that coming back from a film renders
// the list at once, which lets the router restore the scroll position.
const lastShown: { url?: string; result?: FilmListPage } = {}
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowLeft, CircleAlert, ExternalLink, List } from '@lucide/vue'

import FilmGrid from '@/components/FilmGrid.vue'
import FilmRating from '@/components/FilmRating.vue'
import ListPagination from '@/components/ListPagination.vue'
import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api/client'
import { getList, type FilmList, type FilmListPage, type ListFilm } from '@/lib/api/lists'
import { formatDate } from '@/lib/dates'

const props = defineProps<{ id: string }>()

// Splits into whole rows of 3, 5, 6, 10 or 15 films, see FilmGrid.
const PAGE_SIZE = 30

const route = useRoute()
const router = useRouter()

/** The page of the lists this one was opened from, or else their first page. */
const lists = computed(() => {
  const previous = router.options.history.state.back
  return typeof previous === 'string' && router.resolve(previous).name === 'lists'
    ? previous
    : { name: 'lists' }
})

/** Page asked by the URL, from 1; anything else than a page number means the first one. */
const page = computed(() => {
  const asked = Number(route.query.page)
  return Number.isInteger(asked) && asked >= 1 ? asked : 1
})

const pageLocation = (number: number) => ({
  name: 'list',
  params: { id: props.id },
  query: number > 1 ? { page: number } : {},
})

const list = ref<FilmList | null>(null)
const films = ref<ListFilm[]>([])
const isLoading = ref(false)
const error = ref<string | null>(null)
const notFound = ref(false)

if (lastShown.result && lastShown.url === route.fullPath) {
  list.value = lastShown.result.list
  films.value = lastShown.result.films
}

const pageCount = computed(() => Math.max(1, Math.ceil((list.value?.filmCount ?? 0) / PAGE_SIZE)))

// Only the answer to the latest request counts: pages can be changed faster than they load.
let latestRequest = 0

async function load() {
  // The URL also changes when leaving the page, which asks for nothing.
  if (route.name !== 'list') return
  const request = ++latestRequest
  const asked = page.value
  const url = route.fullPath
  // Another list doesn't keep showing the one it replaces.
  if (list.value && String(list.value.id) !== props.id) {
    list.value = null
    films.value = []
  }
  isLoading.value = true
  error.value = null
  notFound.value = false
  try {
    const result = await getList(props.id, { limit: PAGE_SIZE, offset: (asked - 1) * PAGE_SIZE })
    if (request !== latestRequest) return
    const lastPage = Math.max(1, Math.ceil(result.list.filmCount / PAGE_SIZE))
    if (asked > lastPage) {
      await router.replace(pageLocation(lastPage))
      return
    }
    list.value = result.list
    films.value = result.films
    Object.assign(lastShown, { url, result })
  } catch (cause) {
    if (request !== latestRequest) return
    films.value = []
    if (cause instanceof ApiError && cause.status === 404) {
      notFound.value = true
      error.value = 'Cette liste est introuvable.'
    } else if (cause instanceof ApiError) {
      error.value = 'La liste n’a pas pu être chargée.'
    } else {
      error.value = 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
    }
  } finally {
    if (request === latestRequest) isLoading.value = false
  }
}

watch([() => props.id, page], load, { immediate: true })

const formatCount = (count: number) => count.toLocaleString('fr-FR')

// A three-digit position and five stars are 7em and 14px of gaps wide: shrunk to fit the
// narrowest cards.
const footerStyle = (cardWidth = 100) => ({
  fontSize: `${Math.min(14, (cardWidth - 14) / 7)}px`,
})
</script>

<template>
  <!-- Kept compact: a whole page of films and its pagination are meant to fit the screen. -->
  <main class="mx-auto flex w-full max-w-7xl flex-col gap-5 px-4 py-6 sm:px-6">
    <RouterLink
      :to="lists"
      class="flex items-center gap-1.5 self-start rounded-md text-sm text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <ArrowLeft class="size-4" />
      Listes
    </RouterLink>

    <header v-if="list" class="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
      <div class="min-w-0 space-y-1">
        <div class="flex flex-wrap items-baseline gap-x-3">
          <h1 class="text-2xl font-semibold tracking-tight text-balance">{{ list.name }}</h1>
          <p class="text-muted-foreground">
            {{ formatCount(list.filmCount) }} {{ list.filmCount > 1 ? 'films' : 'film' }}
          </p>
        </div>
        <p class="text-sm text-muted-foreground">
          Créée le
          <time :datetime="list.createdOn">{{ formatDate(list.createdOn) }}</time>
        </p>
        <p v-if="list.description" class="max-w-prose pt-2 text-sm whitespace-pre-line">
          {{ list.description }}
        </p>
      </div>
      <Button variant="outline" size="sm" as-child>
        <a :href="list.letterboxdUri" target="_blank" rel="noopener">
          Letterboxd
          <ExternalLink data-icon="inline-end" />
        </a>
      </Button>
    </header>
    <div v-else-if="!error" class="h-8 w-64 animate-pulse rounded bg-muted" />

    <!-- Empty list -->
    <div
      v-if="list?.filmCount === 0"
      class="flex flex-col items-center gap-4 rounded-xl border border-dashed px-6 py-14 text-center"
    >
      <div class="flex size-10 items-center justify-center rounded-full bg-muted">
        <List class="size-5 text-muted-foreground" />
      </div>
      <p class="font-medium">Cette liste ne contient aucun film</p>
    </div>

    <!-- Placeholders during the first load, then the films. -->
    <FilmGrid
      v-else-if="films.length || !error"
      :films="films"
      :page-size="PAGE_SIZE"
      :pending="!list"
      :busy="isLoading"
    >
      <template #footer="{ film, cardWidth }">
        <div class="flex items-center gap-1.5" :style="footerStyle(cardWidth)">
          <span class="text-muted-foreground tabular-nums">
            <span class="sr-only">Position</span>
            {{ film.position }}
          </span>
          <FilmRating v-if="film.rating !== null" :rating="film.rating" />
        </div>
      </template>
    </FilmGrid>

    <div v-if="error" role="alert" class="flex flex-col items-start gap-3">
      <p class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="size-4 shrink-0" />
        {{ error }}
      </p>
      <Button v-if="!notFound" variant="outline" size="sm" @click="load">Réessayer</Button>
    </div>

    <ListPagination :page="page" :page-count="pageCount" :to="pageLocation" />
  </main>
</template>
