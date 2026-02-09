<script setup lang="ts">
import { Play } from 'lucide-vue-next'
import { useEnvironmentStore } from '@/stores/environment'
import { useRequestStore } from '@/stores/request'
import { useCollectionStore } from '@/stores/collection'
import MethodBadge from '@/components/common/MethodBadge.vue'

const envStore = useEnvironmentStore()
const requestStore = useRequestStore()
const collection = useCollectionStore()

function handleRun() {
  if (!collection.activeFilePath || !requestStore.activeRequest) return
  requestStore.execute(collection.activeFilePath, requestStore.activeRequestIndex, envStore.activeEnvName)
}
</script>

<template>
  <header class="flex h-12 items-center justify-between border-b border-border bg-surface px-4">
    <div class="flex items-center gap-3">
      <span class="font-mono text-sm font-semibold text-accent">hitspec</span>
      <span v-if="requestStore.activeRequest" class="flex items-center gap-2 text-sm text-muted-foreground">
        <MethodBadge :method="requestStore.activeRequest.method" />
        <span class="max-w-[400px] truncate font-mono">{{ requestStore.activeRequest.url }}</span>
      </span>
    </div>
    <div class="flex items-center gap-3">
      <select
        v-model="envStore.activeEnvName"
        class="rounded border border-border bg-background px-2 py-1 text-sm text-foreground"
      >
        <option v-for="name in envStore.envNames" :key="name" :value="name">{{ name }}</option>
      </select>
      <button
        class="flex items-center gap-1.5 rounded bg-accent px-3 py-1.5 text-sm font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
        :disabled="!requestStore.activeRequest || requestStore.isExecuting"
        @click="handleRun"
      >
        <Play class="h-3.5 w-3.5" />
        {{ requestStore.isExecuting ? 'Running...' : 'Send' }}
      </button>
    </div>
  </header>
</template>
