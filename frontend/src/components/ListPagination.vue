<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import { ChevronLeft, ChevronRight } from '@lucide/vue'

import { Button } from '@/components/ui/button'

const props = defineProps<{
  /** The current page, from 1. */
  page: number
  pageCount: number
  /** Where the link to a page leads. */
  to: (page: number) => RouteLocationRaw
}>()

// Number of slots of the pagination when there are too many pages to link to them all.
const PAGE_SLOTS = 7

/**
 * Pages to link to: the first, the last and those around the current one; null is a gap.
 * There are as many slots on every page, so that the arrows stay where they are and can
 * be pressed again and again.
 */
const pageLinks = computed<(number | null)[]>(() => {
  const count = props.pageCount
  const range = (from: number, length: number) => Array.from({ length }, (_, i) => from + i)
  if (count <= PAGE_SLOTS) return range(1, count)
  // Close to an end, the pages run up to it without a gap.
  const run = PAGE_SLOTS - 2
  if (props.page < run) return [...range(1, run), null, count]
  if (props.page > count - run + 1) return [1, null, ...range(count - run + 1, run)]
  return [1, null, props.page - 1, props.page, props.page + 1, null, count]
})

const formatCount = (count: number) => count.toLocaleString('fr-FR')

// Wide enough for the longest page number, so that all the slots are the same width.
const pageSlotStyle = computed(() => ({
  width: `calc(${formatCount(props.pageCount).length}ch + 1.25rem)`,
}))
</script>

<template>
  <nav
    v-if="pageCount > 1"
    aria-label="Pagination"
    class="flex flex-wrap items-center justify-center gap-1"
  >
    <Button v-if="page > 1" variant="ghost" size="icon-sm" as-child>
      <RouterLink :to="to(page - 1)" aria-label="Page précédente">
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
          :to="to(number)"
          :aria-label="`Page ${number}`"
          :aria-current="number === page ? 'page' : undefined"
        >
          {{ formatCount(number) }}
        </RouterLink>
      </Button>
    </template>

    <Button v-if="page < pageCount" variant="ghost" size="icon-sm" as-child>
      <RouterLink :to="to(page + 1)" aria-label="Page suivante">
        <ChevronRight />
      </RouterLink>
    </Button>
    <Button v-else variant="ghost" size="icon-sm" disabled aria-label="Page suivante">
      <ChevronRight />
    </Button>
  </nav>
</template>
