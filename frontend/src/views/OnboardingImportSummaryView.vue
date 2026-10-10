<script setup lang="ts">
import { computed, ref, watch, type Component } from 'vue'
import { RouterLink } from 'vue-router'
import {
  ArrowRight,
  BookOpen,
  CircleAlert,
  CircleCheck,
  Eye,
  Heart,
  List,
  ListVideo,
  NotebookPen,
  Star,
} from '@lucide/vue'

import { Button } from '@/components/ui/button'
import {
  ApiError,
  getImportSummary,
  type ImportSection,
  type ImportSummary,
} from '@/lib/api/imports'

const props = defineProps<{ id: string }>()

const sections: { key: ImportSection; label: string; icon: Component }[] = [
  { key: 'watched', label: 'Films vus', icon: Eye },
  { key: 'ratings', label: 'Notes', icon: Star },
  { key: 'diary', label: 'Entrées de journal', icon: BookOpen },
  { key: 'reviews', label: 'Critiques', icon: NotebookPen },
  { key: 'watchlist', label: 'Films dans la watchlist', icon: ListVideo },
  { key: 'likes', label: 'Films aimés', icon: Heart },
  { key: 'lists', label: 'Listes', icon: List },
]

const summary = ref<ImportSummary | null>(null)
const error = ref<string | null>(null)

const rows = computed(() =>
  sections.map((section) => ({ ...section, count: summary.value?.counts[section.key] ?? 0 })),
)

async function load() {
  summary.value = null
  error.value = null
  try {
    summary.value = await getImportSummary(props.id)
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) {
      error.value = 'Cet import est introuvable.'
    } else if (cause instanceof ApiError) {
      error.value = 'Le récapitulatif n’a pas pu être chargé.'
    } else {
      error.value = 'Le serveur Openboxd est injoignable. Vérifiez qu’il est démarré.'
    }
  }
}

watch(() => props.id, load, { immediate: true })

const formatCount = (count: number) => count.toLocaleString('fr-FR')
</script>

<template>
  <main
    class="mx-auto flex min-h-svh w-full max-w-2xl flex-col justify-center gap-8 px-4 py-10 sm:pb-[12vh]"
  >
    <template v-if="error">
      <header class="space-y-2">
        <CircleAlert class="mb-4 size-10 text-destructive" />
        <h1 class="text-3xl font-semibold tracking-tight">Récapitulatif indisponible</h1>
        <p role="alert" class="text-muted-foreground">{{ error }}</p>
      </header>
      <div class="flex gap-2">
        <Button @click="load">Réessayer</Button>
        <Button variant="ghost" as-child>
          <RouterLink :to="{ name: 'onboarding-import' }">Importer un autre export</RouterLink>
        </Button>
      </div>
    </template>

    <template v-else>
      <header class="space-y-2">
        <CircleCheck class="mb-4 size-10" />
        <h1 class="text-3xl font-semibold tracking-tight">Import terminé</h1>
        <p v-if="!summary" class="text-muted-foreground">Chargement du récapitulatif…</p>
        <p v-else-if="summary.username" class="text-muted-foreground">
          Voici ce qui a été récupéré depuis le compte Letterboxd
          <span class="font-medium text-foreground">{{ summary.username }}</span
          >.
        </p>
        <p v-else class="text-muted-foreground">
          Voici ce qui a été récupéré depuis votre export Letterboxd.
        </p>
      </header>

      <dl class="divide-y rounded-xl border bg-card text-card-foreground" :aria-busy="!summary">
        <div v-for="row in rows" :key="row.key" class="flex h-12 items-center gap-3 px-4">
          <component :is="row.icon" class="size-4 shrink-0 text-muted-foreground" />
          <dt :class="['flex-1 text-sm', summary && row.count === 0 && 'text-muted-foreground']">
            {{ row.label }}
          </dt>
          <dd v-if="!summary" class="h-4 w-12 animate-pulse rounded bg-muted" />
          <dd
            v-else
            :class="[
              'text-sm font-medium tabular-nums',
              row.count === 0 && 'text-muted-foreground',
            ]"
          >
            {{ formatCount(row.count) }}
          </dd>
        </div>
      </dl>

      <footer class="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
        <Button variant="ghost" class="sm:-ml-2.5" as-child>
          <RouterLink :to="{ name: 'onboarding-import' }">Importer un autre export</RouterLink>
        </Button>
        <!-- TODO: link to the app once it exists. -->
        <Button :disabled="!summary">
          Accéder à Openboxd
          <ArrowRight data-icon="inline-end" />
        </Button>
      </footer>
    </template>
  </main>
</template>
