<script setup lang="ts">
import { ref } from 'vue'
import { Play, Loader2, Share2 } from 'lucide-vue-next'
import { useRequestStore } from '@/stores/request'
import { useCollectionStore } from '@/stores/collection'
import { useEnvironmentStore } from '@/stores/environment'
import MethodBadge from '@/components/common/MethodBadge.vue'
import ExportDialog from '@/components/export/ExportDialog.vue'

const requestStore = useRequestStore()
const collection = useCollectionStore()
const envStore = useEnvironmentStore()

const showExport = ref(false)

function handleSend() {
  if (!collection.activeFilePath || !requestStore.activeRequest) return
  requestStore.execute(collection.activeFilePath, requestStore.activeRequest.name, envStore.activeEnvName)
}
</script>

<template>
  <div v-if="requestStore.activeRequest" class="flex items-center gap-2 border-b border-border p-3">
    <MethodBadge :method="requestStore.activeRequest.method" size="md" />
    <div class="flex-1 rounded border border-border bg-background px-3 py-1.5 font-mono text-sm text-foreground">
      {{ requestStore.activeRequest.url }}
    </div>
    <button
      class="rounded border border-border p-1.5 text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
      title="Export request"
      @click="showExport = true"
    >
      <Share2 class="h-4 w-4" />
    </button>
    <button
      class="flex items-center gap-1.5 rounded bg-accent px-4 py-1.5 text-sm font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
      :disabled="requestStore.isExecuting"
      @click="handleSend"
    >
      <Loader2 v-if="requestStore.isExecuting" class="h-4 w-4 animate-spin" />
      <Play v-else class="h-4 w-4" />
      Send
    </button>

    <ExportDialog
      v-model="showExport"
      :request="requestStore.activeRequest"
      :variables="envStore.activeEnv?.variables ?? {}"
    />
  </div>
</template>
