<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowLeft, CircleAlert, Clapperboard, ExternalLink } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api/client'
import { getFilm, posterUrl, type Film } from '@/lib/api/films'
import { formatDate } from '@/lib/dates'

const props = defineProps<{ id: string }>()

const film = ref<Film | null>(null)
const error = ref<string | null>(null)
const notFound = ref(false)

async function load() {
  film.value = null
  error.value = null
  notFound.value = false
  try {
    film.value = await getFilm(props.id)
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) {
      notFound.value = true
      error.value = 'Ce film est introuvable.'
    } else if (cause instanceof ApiError) {
      error.value = 'Le film n’a pas pu être chargé.'
    } else {
      error.value = 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
    }
  }
}

watch(() => props.id, load, { immediate: true })

/** e.g. "2 h 16 min", or "47 min" under an hour. */
const runtime = computed(() => {
  const minutes = film.value?.runtime
  if (!minutes) return null
  const hours = Math.floor(minutes / 60)
  return hours ? `${hours} h ${minutes % 60} min` : `${minutes} min`
})

const tmdbUrl = computed(() =>
  film.value?.tmdbId ? `https://www.themoviedb.org/movie/${film.value.tmdbId}` : null,
)
</script>

<template>
  <main class="mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-10 sm:px-6">
    <RouterLink
      :to="{ name: 'films' }"
      class="flex items-center gap-1.5 self-start rounded-md text-sm text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <ArrowLeft class="size-4" />
      Films vus
    </RouterLink>

    <div v-if="error" role="alert" class="flex flex-col items-start gap-3">
      <p class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="size-4 shrink-0" />
        {{ error }}
      </p>
      <Button v-if="!notFound" variant="outline" size="sm" @click="load">Réessayer</Button>
    </div>

    <!-- Loading -->
    <div
      v-else-if="!film"
      class="flex flex-col gap-8 sm:flex-row lg:gap-12"
      aria-busy="true"
      aria-label="Chargement du film"
    >
      <div class="aspect-2/3 w-48 shrink-0 animate-pulse rounded-xl bg-muted sm:w-64 lg:w-80" />
      <div class="flex-1 space-y-4">
        <div class="h-9 w-2/3 animate-pulse rounded bg-muted" />
        <div class="h-5 w-1/3 animate-pulse rounded bg-muted" />
        <div class="space-y-2 pt-4">
          <div v-for="n in 4" :key="n" class="h-4 animate-pulse rounded bg-muted" />
        </div>
      </div>
    </div>

    <article v-else class="flex flex-col gap-8 sm:flex-row lg:gap-12">
      <!-- The poster is decorative: the title is right next to it. -->
      <img
        v-if="film.posterPath"
        :src="posterUrl(film.posterPath, 342)"
        :srcset="`${posterUrl(film.posterPath, 342)} 1x, ${posterUrl(film.posterPath, 500)} 2x`"
        alt=""
        width="342"
        height="513"
        class="aspect-2/3 w-48 shrink-0 self-start rounded-xl border bg-muted object-cover sm:w-64 lg:w-80"
      />
      <div
        v-else
        class="flex aspect-2/3 w-48 shrink-0 items-center justify-center self-start rounded-xl border bg-muted sm:w-64 lg:w-80"
      >
        <Clapperboard class="size-10 text-muted-foreground" />
      </div>

      <div class="flex min-w-0 flex-1 flex-col gap-6">
        <header class="space-y-2">
          <h1 class="text-3xl font-semibold tracking-tight text-balance">
            {{ film.name }}
            <span v-if="film.year" class="ml-1 font-normal text-muted-foreground">
              {{ film.year }}
            </span>
          </h1>
          <p v-if="film.directors.length" class="text-muted-foreground">
            Réalisé par
            <span class="text-foreground">{{ film.directors.join(', ') }}</span>
          </p>
        </header>

        <dl
          v-if="runtime || film.watchedOn"
          class="flex flex-wrap gap-x-10 gap-y-4 border-y py-4 text-sm"
        >
          <div v-if="runtime" class="space-y-0.5">
            <dt class="text-muted-foreground">Durée</dt>
            <dd class="font-medium">{{ runtime }}</dd>
          </div>
          <div v-if="film.watchedOn" class="space-y-0.5">
            <dt class="text-muted-foreground">Vu le</dt>
            <dd class="font-medium">
              <time :datetime="film.watchedOn">{{ formatDate(film.watchedOn) }}</time>
            </dd>
          </div>
        </dl>

        <section class="space-y-2">
          <h2 class="text-sm font-medium text-muted-foreground">Synopsis</h2>
          <p v-if="film.overview" class="leading-relaxed text-pretty">{{ film.overview }}</p>
          <p v-else class="text-muted-foreground">Aucun synopsis disponible pour ce film.</p>
        </section>

        <div class="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" as-child>
            <a :href="film.letterboxdUri" target="_blank" rel="noopener">
              Letterboxd
              <ExternalLink data-icon="inline-end" />
            </a>
          </Button>
          <Button v-if="tmdbUrl" variant="outline" size="sm" as-child>
            <a :href="tmdbUrl" target="_blank" rel="noopener">
              TMDB
              <ExternalLink data-icon="inline-end" />
            </a>
          </Button>
        </div>
      </div>
    </article>
  </main>
</template>
