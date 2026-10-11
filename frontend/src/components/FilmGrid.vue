<script
  setup
  lang="ts"
  generic="T extends { id: number; name: string; year: number | null; posterPath: string | null }"
>
import { computed, useTemplateRef } from 'vue'
import { RouterLink } from 'vue-router'
import { Clapperboard } from '@lucide/vue'

import { posterUrl } from '@/lib/api/films'
import { filmGridHeight, fitFilmGrid } from '@/lib/film-grid'
import { useFittedLayout } from '@/lib/fitted-layout'

const props = defineProps<{
  films: T[]
  /** Number of films of a full page: the grid is laid out in whole rows of it. */
  pageSize: number
  /** Shows placeholders instead of films, while the first page loads. */
  pending?: boolean
  /** Dims the films, while another page loads. */
  busy?: boolean
}>()

const slots = defineSlots<{
  /** What a card shows under its poster, one line high; `cardWidth` is in pixels. */
  footer?(props: { film: T; cardWidth: number | undefined }): unknown
}>()

const layout = useFittedLayout(useTemplateRef('gridArea'), (width, height) =>
  fitFilmGrid(width, height, props.pageSize, Boolean(slots.footer)),
)

const gridStyle = computed(() => {
  if (!layout.value) return undefined
  const { columns, cardWidth } = layout.value
  return {
    gridTemplateColumns: `repeat(${columns}, ${cardWidth}px)`,
    // As tall as a full page even when the last one isn't, so that the pagination stays put.
    minHeight: `${filmGridHeight(layout.value, props.pageSize)}px`,
  }
})

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
  <div ref="gridArea">
    <ul
      v-if="pending"
      :class="gridClass"
      :style="gridStyle"
      aria-busy="true"
      aria-label="Chargement des films"
    >
      <li v-for="n in pageSize" :key="n">
        <div class="aspect-2/3 animate-pulse rounded-lg bg-muted" />
      </li>
    </ul>

    <ul
      v-else
      :class="[gridClass, { 'opacity-50': busy }]"
      class="transition-opacity"
      :style="gridStyle"
      :aria-busy="busy"
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
          <!-- Always there when cards have one, so that each is as tall as fitGrid expects. -->
          <div v-if="$slots.footer" class="flex h-4 items-center">
            <slot name="footer" :film="film" :card-width="layout?.cardWidth" />
          </div>
        </RouterLink>
      </li>
    </ul>
  </div>
</template>
