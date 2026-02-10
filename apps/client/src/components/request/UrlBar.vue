<script setup lang="ts">
import { ref, computed } from 'vue'
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

// Merge file-level variables (@baseUrl = ...) with environment variables
const exportVariables = computed(() => {
  const vars: Record<string, unknown> = {}
  // File-level variables first (lower priority)
  const fileVars = collection.activeFile?.variables ?? []
  for (const v of fileVars) {
    vars[v.name] = v.value
  }
  // Environment variables override file-level ones
  const envVars = envStore.activeEnv?.variables ?? {}
  for (const [k, v] of Object.entries(envVars)) {
    vars[k] = v
  }
  return vars
})

function handleSend() {
  if (!collection.activeFilePath || !requestStore.activeRequest) return
  requestStore.execute(collection.activeFilePath, requestStore.activeRequest.name, envStore.activeEnvName)
}
</script>

<template>
  <div v-if="requestStore.activeRequest" class="flex items-center gap-2 border-b border-border px-3 py-2.5">
    <MethodBadge :method="requestStore.activeRequest.method" size="md" />
    <div class="flex-1 overflow-hidden rounded-md border border-border bg-background px-3 py-1.5 font-mono text-xs text-foreground/80">
      <span class="block truncate">{{ requestStore.activeRequest.url }}</span>
    </div>
    <button
      aria-label="Export request"
      class="rounded-md border border-border p-1.5 text-muted-foreground/60 transition-colors hover:bg-surface-hover hover:text-foreground"
      title="Export request"
      @click="showExport = true"
    >
      <Share2 class="h-3.5 w-3.5" />
    </button>
    <button
      aria-label="Send request"
      class="flex items-center gap-1.5 rounded-md bg-accent px-3 py-1.5 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
      :disabled="requestStore.isExecuting"
      @click="handleSend"
    >
      <Loader2 v-if="requestStore.isExecuting" class="h-3.5 w-3.5 animate-spin" />
      <Play v-else class="h-3.5 w-3.5" />
      Send
    </button>

    <ExportDialog
      v-model="showExport"
      :request="requestStore.activeRequest"
      :variables="exportVariables"
    />
  </div>
</template>
