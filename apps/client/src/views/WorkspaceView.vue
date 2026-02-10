<script setup lang="ts">
import RequestPanel from '@/components/request/RequestPanel.vue'
import ResponsePanel from '@/components/response/ResponsePanel.vue'
import { useCollectionStore } from '@/stores/collection'
import { useRequestStore } from '@/stores/request'
import { onMounted, watch } from 'vue'

const collection = useCollectionStore()
const requestStore = useRequestStore()

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
  <div class="flex h-full overflow-hidden">
    <div class="flex-1 overflow-hidden border-r border-border">
      <RequestPanel />
    </div>
    <div class="flex-1 overflow-hidden">
      <ResponsePanel />
    </div>
  </div>
</template>
