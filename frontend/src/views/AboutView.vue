<script setup lang="ts">
import { ExternalLink } from '@lucide/vue'

import { Button } from '@/components/ui/button'
// TMDB's official logo: its terms forbid altering it.
import tmdbLogo from '@/assets/tmdb-logo.svg'
</script>

<template>
  <main class="mx-auto flex w-full max-w-2xl flex-col gap-10 px-4 py-10 sm:px-6">
    <header class="space-y-3">
      <h1 class="text-3xl font-semibold tracking-tight">À propos</h1>
      <p class="leading-relaxed text-muted-foreground">
        Openboxd est une bibliothèque de films gratuite, open source et auto-hébergeable.
      </p>
      <Button variant="outline" size="sm" as-child>
        <a href="https://codeberg.org/pfrappier/openboxd" target="_blank" rel="noopener">
          Code source
          <ExternalLink data-icon="inline-end" />
        </a>
      </Button>
    </header>

    <section class="space-y-4">
      <h2 class="text-lg font-semibold tracking-tight">Crédits</h2>
      <div class="space-y-4 rounded-xl border bg-card p-5 text-card-foreground">
        <a
          href="https://www.themoviedb.org"
          target="_blank"
          rel="noopener"
          class="inline-block rounded-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <img :src="tmdbLogo" alt="TMDB" width="123" height="16" class="h-4 w-auto" />
        </a>
        <p class="text-sm leading-relaxed">
          Les affiches et les informations sur les films (synopsis, durée, réalisation) proviennent
          de TMDB. Ce produit utilise TMDB et les API TMDB, mais n’est ni approuvé, ni certifié, ni
          autrement validé par TMDB.
        </p>
        <!-- The notice TMDB's API terms of use require, word for word. -->
        <p lang="en" class="text-sm leading-relaxed text-muted-foreground">
          This product uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise
          approved by TMDB.
        </p>
      </div>
    </section>
  </main>
</template>
