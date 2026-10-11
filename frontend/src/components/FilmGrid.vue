<script
  setup
  lang="ts"
  generic="T extends { id: number; name: string; year: number | null; posterPath: string | null }"
>
import { computed, onBeforeUnmount, onMounted, ref, useTemplateRef } from 'vue'
import { RouterLink } from 'vue-router'
import { Clapperboard } from '@lucide/vue'

import { posterUrl } from '@/lib/api/films'

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

// In pixels: the gap of the grid, what a card adds under its poster for its footer,
// and the width under which posters get too small to be told apart.
const GRID_GAP = 12
const CARD_FOOTER = 22
const MIN_CARD_WIDTH = 64

interface GridLayout {
  columns: number
  /** Width of a card, in pixels. */
  cardWidth: number
  /** Height a row is given, in pixels: the one of a card with a footer. */
  rowHeight: number
}

/**
 * Lays a full page of films out in whole rows, with the largest cards that fit both the
 * width and the height given. When the space is too small for that, the height gives in:
 * the cards keep a decent size and the page scrolls.
 *
 * Cards are fitted as if they had a footer even when they have none: every list of films
 * is then as tall, and what sits under the grid doesn't move from one to the other.
 * Posters without a footer grow into its room where the width allows it.
 */
function fitGrid(width: number, height: number): GridLayout {
  const widthFor = (columns: number) => (width - GRID_GAP * (columns - 1)) / columns
  const heightFor = (columns: number) => {
    const rows = props.pageSize / columns
    return (height - GRID_GAP * (rows - 1)) / rows
  }
  const candidates: number[] = []
  for (let columns = 1; columns <= props.pageSize; columns++) {
    if (props.pageSize % columns === 0) candidates.push(columns)
  }
  const layouts = candidates.map((columns) => ({
    columns,
    // Posters are 2:3.
    cardWidth: Math.floor(
      Math.min(widthFor(columns), ((heightFor(columns) - CARD_FOOTER) * 2) / 3),
    ),
  }))
  let fitted = layouts.reduce((a, b) => (b.cardWidth > a.cardWidth ? b : a))
  if (fitted.cardWidth < MIN_CARD_WIDTH) {
    // The most columns the width allows make for the shortest scroll.
    const columns = Math.max(1, ...candidates.filter((c) => widthFor(c) >= MIN_CARD_WIDTH))
    fitted = { columns, cardWidth: Math.floor(widthFor(columns)) }
  }

  const { columns } = fitted
  const rowHeight = fitted.cardWidth * 1.5 + CARD_FOOTER
  if (slots.footer) return { ...fitted, rowHeight }
  const cardWidth = Math.floor(Math.min(widthFor(columns), (rowHeight * 2) / 3))
  return { columns, cardWidth, rowHeight }
}

const gridArea = useTemplateRef('gridArea')
// The page the grid is part of: its <main>, which holds what sits above and under it.
let pageElement: HTMLElement | null = null
const layout = ref<GridLayout | null>(null)

function updateLayout() {
  if (!pageElement || !gridArea.value) return
  const pageBox = pageElement.getBoundingClientRect()
  const areaBox = gridArea.value.getBoundingClientRect()
  // What the screen leaves to the grid once everything above and under it is counted:
  // the top bar and the title above, the pagination under.
  const above = areaBox.top + window.scrollY
  const under = pageBox.bottom - areaBox.bottom
  const fitted = fitGrid(gridArea.value.clientWidth, window.innerHeight - above - under)
  if (
    fitted.columns !== layout.value?.columns ||
    fitted.cardWidth !== layout.value.cardWidth ||
    fitted.rowHeight !== layout.value.rowHeight
  ) {
    layout.value = fitted
  }
}

// Also covers what changes the space left without resizing the window, such as the
// pagination showing up once the number of films is known.
const pageObserver = new ResizeObserver(updateLayout)

onMounted(() => {
  pageElement = gridArea.value?.closest('main') ?? null
  updateLayout()
  if (pageElement) pageObserver.observe(pageElement)
  window.addEventListener('resize', updateLayout)
})

onBeforeUnmount(() => {
  pageObserver.disconnect()
  window.removeEventListener('resize', updateLayout)
})

const gridStyle = computed(() => {
  if (!layout.value) return undefined
  const { columns, cardWidth, rowHeight } = layout.value
  const rows = props.pageSize / columns
  return {
    gridTemplateColumns: `repeat(${columns}, ${cardWidth}px)`,
    // As tall as a full page even when the last one isn't, so that the pagination stays put.
    minHeight: `${rows * rowHeight + (rows - 1) * GRID_GAP}px`,
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
