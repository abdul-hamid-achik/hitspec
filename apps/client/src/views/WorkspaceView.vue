<script setup lang="ts">
import RequestPanel from '@/components/request/RequestPanel.vue'
import ResponsePanel from '@/components/response/ResponsePanel.vue'
import SplitPane from '@/components/common/SplitPane.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { useCollectionStore } from '@/stores/collection'
import { useRequestStore } from '@/stores/request'
import { useSettingsStore } from '@/stores/settings'
import { onMounted, watch } from 'vue'
import { FolderTree, AlertCircle } from 'lucide-vue-next'

const collection = useCollectionStore()
const requestStore = useRequestStore()
const settings = useSettingsStore()

onMounted(() => {
  // File loading and env loading is handled by AppShell.
  // Init sets up the WebSocket file-change listener (idempotent).
  collection.init()
})

watch(() => collection.activeFile, (file) => {
  if (file && file.requests.length > 0) {
    requestStore.setActiveRequest(file.requests[0], 0)
  } else {
    requestStore.setActiveRequest(null)
  }
})
</script>

<template>
  <!-- Loading state: workspace files are still being fetched -->
  <div v-if="collection.loading && collection.files.length === 0" class="flex h-full items-center justify-center">
    <LoadingSpinner size="lg" label="Loading workspace..." />
  </div>

  <!-- Error state: workspace failed to load and no files cached -->
  <div v-else-if="collection.error && collection.files.length === 0" class="flex h-full flex-col items-center justify-center gap-3 p-8">
    <div class="rounded-xl bg-destructive/10 p-3">
      <AlertCircle class="h-10 w-10 text-destructive/60" />
    </div>
    <div class="space-y-1 text-center">
      <h3 class="text-sm font-medium text-foreground">Failed to load workspace</h3>
      <p class="max-w-sm text-xs text-muted-foreground">{{ collection.error }}</p>
    </div>
    <button
      class="rounded-md border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
      @click="collection.loadFiles()"
    >
      Retry
    </button>
  </div>

  <!-- Empty state: no files in workspace -->
  <div v-else-if="!collection.loading && collection.files.length === 0 && !collection.activeFilePath" class="flex h-full items-center justify-center">
    <EmptyState
      :icon="FolderTree"
      title="No files in workspace"
      description="Add .http or .hitspec files to your project directory to get started"
    />
  </div>

  <!-- Normal workspace: request editor + response panel -->
  <SplitPane v-else v-model:ratio="settings.workspaceSplitRatio" :min-left="300" :min-right="300">
    <template #left>
      <RequestPanel />
    </template>
    <template #right>
      <ResponsePanel />
    </template>
  </SplitPane>
</template>
