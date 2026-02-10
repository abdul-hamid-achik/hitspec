<script setup lang="ts">
import { Play, Search, PanelLeft, Loader2 } from 'lucide-vue-next'
import { useEnvironmentStore } from '@/stores/environment'
import { useRequestStore } from '@/stores/request'
import { useCollectionStore } from '@/stores/collection'
import MethodBadge from '@/components/common/MethodBadge.vue'

const envStore = useEnvironmentStore()
const requestStore = useRequestStore()
const collection = useCollectionStore()

const emit = defineEmits<{
  'open-command-palette': []
  'toggle-sidebar': []
}>()

const isMac = navigator.platform.toLowerCase().includes('mac')
const modKey = isMac ? '\u2318' : 'Ctrl'

function handleRun() {
  if (!collection.activeFilePath) return
  requestStore.runFile(collection.activeFilePath, envStore.activeEnvName)
}
</script>

<template>
  <header class="flex h-11 items-center justify-between border-b border-border bg-surface px-3">
    <div class="flex items-center gap-2">
      <button
        aria-label="Toggle sidebar"
        class="rounded p-1 text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        title="Toggle sidebar"
        @click="emit('toggle-sidebar')"
      >
        <PanelLeft class="h-4 w-4" />
      </button>
      <span class="font-mono text-sm font-bold text-accent">hitspec</span>

      <!-- Active request breadcrumb -->
      <template v-if="requestStore.activeRequest">
        <span class="text-muted-foreground/30">/</span>
        <span class="flex items-center gap-1.5 text-sm text-muted-foreground">
          <MethodBadge :method="requestStore.activeRequest.method" size="sm" />
          <span class="max-w-[300px] truncate font-mono text-xs">{{ requestStore.activeRequest.url }}</span>
        </span>
      </template>
    </div>

    <div class="flex items-center gap-2">
      <!-- Command palette trigger -->
      <button
        aria-label="Open command palette"
        class="flex items-center gap-2 rounded-md border border-border bg-background/50 px-2.5 py-1 text-xs text-muted-foreground/60 transition-colors hover:border-border hover:bg-background hover:text-muted-foreground"
        @click="emit('open-command-palette')"
      >
        <Search class="h-3.5 w-3.5" />
        <span class="hidden sm:inline">Search...</span>
        <kbd class="rounded border border-border/50 bg-surface px-1 py-px text-[10px]">{{ modKey }}K</kbd>
      </button>

      <!-- Environment selector -->
      <select
        aria-label="Select environment"
        :value="envStore.activeEnvName"
        class="rounded-md border border-border bg-background px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        @change="envStore.selectEnv(($event.target as HTMLSelectElement).value)"
      >
        <option v-for="name in envStore.envNames" :key="name" :value="name">{{ name }}</option>
      </select>

      <!-- Run all button -->
      <button
        aria-label="Run all requests in file"
        class="flex items-center gap-1.5 rounded-md bg-accent px-3 py-1 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
        :disabled="!collection.activeFilePath || requestStore.isExecuting"
        @click="handleRun"
      >
        <Loader2 v-if="requestStore.isExecuting" class="h-3.5 w-3.5 animate-spin" />
        <Play v-else class="h-3.5 w-3.5" />
        {{ requestStore.isExecuting ? 'Running...' : 'Run All' }}
      </button>
    </div>
  </header>
</template>
