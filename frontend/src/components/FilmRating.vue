<script setup lang="ts">
import { computed } from 'vue'
import { Star, StarHalf } from '@lucide/vue'

const props = defineProps<{
  /** In stars, from 0.5 to 5 by steps of 0.5. */
  rating: number
}>()

const fullStars = computed(() => Math.floor(props.rating))
const hasHalfStar = computed(() => props.rating % 1 !== 0)

/** e.g. "Note : 4,5 sur 5". */
const label = computed(() => `Note : ${props.rating.toLocaleString('fr-FR')} sur 5`)
</script>

<template>
  <!-- As on Letterboxd, only the stars given are drawn: no empty ones. -->
  <span class="inline-flex items-center gap-0.5">
    <Star v-for="n in fullStars" :key="n" class="size-[1em] fill-current" />
    <StarHalf v-if="hasHalfStar" class="size-[1em] fill-current" />
    <span class="sr-only">{{ label }}</span>
  </span>
</template>
