<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  ArrowLeft,
  ExternalLink,
  FileArchive,
  Folder,
  FolderOpen,
  ShieldCheck,
  Upload,
  X,
} from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

// Visual only for now: export parsing and import logic come later.
// The export can come as the ZIP archive or as the folder some browsers
// (Safari by default) produce by unzipping it automatically.
type Selection = { kind: 'zip'; name: string; size: number } | { kind: 'folder'; name: string }

const selection = ref<Selection | null>(null)
const error = ref<string | null>(null)
const isDragging = ref(false)
const zipInput = ref<HTMLInputElement>()
const folderInput = ref<HTMLInputElement>()

const INVALID_FORMAT =
  'Format non reconnu. Déposez l’archive ZIP ou le dossier exporté par Letterboxd.'

function selectZip(file: File | undefined) {
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.zip')) {
    error.value = INVALID_FORMAT
    return
  }
  error.value = null
  selection.value = { kind: 'zip', name: file.name, size: file.size }
}

function selectFolder(name: string) {
  error.value = null
  selection.value = { kind: 'folder', name }
}

function onZipChange(event: Event) {
  const target = event.target as HTMLInputElement
  selectZip(target.files?.[0])
  target.value = ''
}

function onFolderChange(event: Event) {
  const target = event.target as HTMLInputElement
  const firstPath = target.files?.[0]?.webkitRelativePath
  if (firstPath) selectFolder(firstPath.split('/')[0] ?? firstPath)
  target.value = ''
}

function onDragLeave(event: DragEvent) {
  const zone = event.currentTarget as HTMLElement
  if (!zone.contains(event.relatedTarget as Node | null)) isDragging.value = false
}

function onDrop(event: DragEvent) {
  isDragging.value = false
  const entry = event.dataTransfer?.items[0]?.webkitGetAsEntry()
  if (entry?.isDirectory) {
    selectFolder(entry.name)
  } else {
    selectZip(event.dataTransfer?.files[0])
  }
}

function formatSize(bytes: number) {
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} Ko`
  return `${(bytes / 1024 / 1024).toFixed(1)} Mo`
}
</script>

<template>
  <main class="mx-auto flex min-h-svh w-full max-w-2xl flex-col justify-center gap-8 px-4 py-10 sm:pb-[12vh]">
    <div>
      <Button variant="ghost" size="sm" class="-ml-2.5" as-child>
        <RouterLink :to="{ name: 'onboarding' }">
          <ArrowLeft data-icon="inline-start" />
          Retour
        </RouterLink>
      </Button>
    </div>

    <header class="space-y-2">
      <h1 class="text-3xl font-semibold tracking-tight">Importer vos données Letterboxd</h1>
      <p class="text-muted-foreground">
        Letterboxd vous permet d’exporter toutes vos données : films vus, notes, journal, critiques,
        watchlist et listes.
      </p>
    </header>

    <ol class="space-y-3">
      <li class="flex gap-3">
        <span
          class="flex size-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-medium">1</span>
        <p class="pt-0.5 text-sm">
          Ouvrez la page
          <a href="https://letterboxd.com/settings/data/" target="_blank" rel="noopener noreferrer"
            class="inline-flex items-center gap-1 font-medium underline underline-offset-4">Settings → Data
            <ExternalLink class="size-3" />
          </a>
          de votre compte Letterboxd.
        </p>
      </li>
      <li class="flex gap-3">
        <span
          class="flex size-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-medium">2</span>
        <p class="pt-0.5 text-sm">
          Cliquez sur <span class="font-medium">Export your data</span>, le téléchargement démarre.
        </p>
      </li>
      <li class="flex gap-3">
        <span
          class="flex size-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-medium">3</span>
        <p class="pt-0.5 text-sm">
          Déposez ci-dessous l’archive ZIP téléchargée, ou le dossier obtenu en la décompressant.
          <span class="block text-muted-foreground">
            Safari décompresse l’archive automatiquement : dans ce cas, déposez le dossier.
          </span>
        </p>
      </li>
    </ol>

    <div class="space-y-2">
      <!-- Both states share the same height so the page doesn't jump when an export is picked. -->
      <div v-if="selection" class="flex h-52 items-center gap-4 rounded-xl border bg-card px-5 text-card-foreground">
        <div class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted">
          <FileArchive v-if="selection.kind === 'zip'" class="size-5" />
          <Folder v-else class="size-5" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium">{{ selection.name }}</p>
          <p class="text-sm text-muted-foreground">
            {{
              selection.kind === 'zip'
                ? `Archive ZIP · ${formatSize(selection.size)}`
                : 'Dossier décompressé'
            }}
          </p>
        </div>
        <Button variant="ghost" size="icon-sm" :aria-label="`Retirer ${selection.name}`" @click="selection = null">
          <X />
        </Button>
      </div>

      <div v-else :class="cn(
        'flex h-52 flex-col items-center justify-center gap-4 rounded-xl border border-dashed px-6 text-center transition-colors',
        isDragging && 'border-ring bg-muted/50',
        error && 'border-destructive',
      )
        " @dragover.prevent="isDragging = true" @dragleave="onDragLeave" @drop.prevent="onDrop">
        <div class="flex size-10 items-center justify-center rounded-full bg-muted">
          <Upload class="size-5 text-muted-foreground" />
        </div>
        <div class="space-y-1">
          <p class="text-sm font-medium">Glissez votre export ici</p>
          <p class="text-sm text-muted-foreground">Archive .zip ou dossier décompressé</p>
        </div>
        <div class="flex flex-wrap justify-center gap-2">
          <Button variant="outline" size="sm" @click="zipInput?.click()">
            <FileArchive data-icon="inline-start" />
            Choisir l’archive ZIP
          </Button>
          <Button variant="outline" size="sm" @click="folderInput?.click()">
            <FolderOpen data-icon="inline-start" />
            Choisir un dossier
          </Button>
        </div>
      </div>
      <input ref="zipInput" type="file" accept=".zip" class="hidden" @change="onZipChange" />
      <input ref="folderInput" type="file" webkitdirectory class="hidden" @change="onFolderChange" />

      <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
    </div>

    <footer class="flex flex-col-reverse gap-4 sm:flex-row sm:items-center sm:justify-between">
      <p class="flex items-center gap-2 text-xs text-muted-foreground">
        <ShieldCheck class="size-3.5 shrink-0" />
        Vos données restent sur votre instance, rien n’est envoyé à un service tiers.
      </p>
      <Button :disabled="!selection">Importer</Button>
    </footer>
  </main>
</template>
