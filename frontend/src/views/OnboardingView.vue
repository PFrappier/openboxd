<script setup lang="ts">
import type { Component } from 'vue'
import { ArrowRight, Download, Sparkles } from '@lucide/vue'

// The app is dark-only for now, hence the dark variant.
import logo from '@/assets/logo-dark.png'

type StartMode = 'import' | 'scratch'

const options: {
  value: StartMode
  title: string
  description: string
  icon: Component
}[] = [
  {
    value: 'import',
    title: 'Importer depuis Letterboxd',
    description: 'Récupérez votre historique, vos notes, critiques et listes.',
    icon: Download,
  },
  {
    value: 'scratch',
    title: 'Commencer de zéro',
    description: 'Démarrez avec un journal vide et ajoutez vos films au fil de l’eau.',
    icon: Sparkles,
  },
]

// eslint-disable-next-line @typescript-eslint/no-unused-vars -- used once navigation is wired
function start(mode: StartMode) {
  // TODO: go to the Letterboxd import step, or straight to the app.
}
</script>

<template>
  <main
    class="mx-auto flex min-h-svh w-full max-w-2xl flex-col justify-center gap-6 px-4 py-10 sm:pb-[12vh]"
  >
    <header class="space-y-2">
      <img :src="logo" alt="" class="mb-4 size-14" />
      <h1 class="text-3xl font-semibold tracking-tight">Bienvenue sur Openboxd</h1>
      <p class="text-muted-foreground">
        Votre journal de films, hébergé chez vous. Comment souhaitez-vous commencer ?
      </p>
    </header>

    <div class="grid gap-3 sm:grid-cols-2">
      <button
        v-for="option in options"
        :key="option.value"
        type="button"
        class="group flex flex-col items-start gap-3 rounded-xl border bg-card p-4 text-left text-card-foreground transition-colors outline-none hover:border-ring hover:bg-muted/50 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        @click="start(option.value)"
      >
        <div class="flex w-full items-start justify-between">
          <div class="flex size-9 items-center justify-center rounded-lg bg-muted">
            <component :is="option.icon" class="size-4.5" />
          </div>
          <ArrowRight
            class="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground"
          />
        </div>
        <div class="space-y-1">
          <p class="font-medium">{{ option.title }}</p>
          <p class="text-sm text-muted-foreground">{{ option.description }}</p>
        </div>
      </button>
    </div>
  </main>
</template>
