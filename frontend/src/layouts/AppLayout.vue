<script setup lang="ts">
import { RouterLink, RouterView, type RouteLocationRaw } from 'vue-router'
import { Download } from '@lucide/vue'

import { Button } from '@/components/ui/button'
// The app is dark-only for now, hence the dark variant.
import logo from '@/assets/logo-dark.png'

// Tabs without a `to` are placeholders for pages that don't exist yet.
const navigation: { label: string; to?: RouteLocationRaw }[] = [
  { label: 'Films', to: { name: 'films' } },
  { label: 'Journal' },
  { label: 'Watchlist' },
  { label: 'Listes' },
]

const tabClass =
  'rounded-md text-sm font-medium text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50'

const footerLinkClass =
  'rounded-sm transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50'
</script>

<template>
  <div class="flex min-h-dvh flex-col">
    <header class="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
      <!--
        Equal side columns keep the navigation centered whatever the width of the sides.
        On narrow screens the navigation drops to its own row under the logo.
      -->
      <div
        class="grid w-full grid-cols-2 items-center gap-x-4 px-4 sm:px-6 md:h-18 md:grid-cols-[1fr_auto_1fr]"
      >
        <RouterLink
          :to="{ name: 'films' }"
          class="flex h-18 items-center gap-2.5 justify-self-start rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <img :src="logo" alt="" width="40" height="40" class="size-10" />
          <span class="text-2xl font-semibold tracking-tight">Openboxd</span>
        </RouterLink>

        <nav
          aria-label="Navigation principale"
          class="order-last col-span-2 flex items-center justify-center gap-6 pb-3 md:order-none md:col-span-1 md:pb-0"
        >
          <template v-for="item in navigation" :key="item.label">
            <RouterLink
              v-if="item.to"
              :to="item.to"
              :class="tabClass"
              active-class="text-foreground!"
            >
              {{ item.label }}
            </RouterLink>
            <button v-else type="button" :class="tabClass">{{ item.label }}</button>
          </template>
        </nav>

        <Button variant="outline" size="sm" class="justify-self-end" as-child>
          <RouterLink :to="{ name: 'onboarding-import' }">
            <Download data-icon="inline-start" />
            Importer
          </RouterLink>
        </Button>
      </div>
    </header>

    <!-- Takes the height left, so that the footer sits at the bottom of short pages. -->
    <div class="flex-1">
      <RouterView />
    </div>

    <footer class="border-t">
      <div
        class="flex flex-col items-center gap-x-6 gap-y-3 px-4 py-6 text-sm text-muted-foreground sm:flex-row sm:justify-between sm:px-6"
      >
        <p>
          <span class="font-medium text-foreground">Openboxd</span>
          · bibliothèque de films gratuite et auto-hébergeable
        </p>
        <nav
          aria-label="Pied de page"
          class="flex flex-wrap items-center justify-center gap-x-6 gap-y-2"
        >
          <RouterLink :to="{ name: 'about' }" :class="footerLinkClass">À propos</RouterLink>
          <a
            href="https://codeberg.org/pfrappier/openboxd"
            target="_blank"
            rel="noopener"
            :class="footerLinkClass"
          >
            Code source
          </a>
          <a
            href="https://www.themoviedb.org"
            target="_blank"
            rel="noopener"
            :class="footerLinkClass"
          >
            Données : TMDB
          </a>
        </nav>
      </div>
    </footer>
  </div>
</template>
