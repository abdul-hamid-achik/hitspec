<script setup lang="ts">
import { Circle, Timer, Wifi, WifiOff, Eye } from 'lucide-vue-next'
import { ws } from '@/api/websocket'
import { ref, onMounted, onUnmounted } from 'vue'
import { useEnvironmentStore } from '@/stores/environment'
import { useRequestStore } from '@/stores/request'
import { useCollectionStore } from '@/stores/collection'

const envStore = useEnvironmentStore()
const requestStore = useRequestStore()
const collection = useCollectionStore()
const connected = ref(false)
const watchActive = ref(false)
const lastFileChange = ref<string | null>(null)
let interval: ReturnType<typeof setInterval>
let fileChangeTimeout: ReturnType<typeof setTimeout> | null = null
let unsubFileChanged: (() => void) | null = null

onMounted(() => {
  interval = setInterval(() => {
    connected.value = ws.connected
  }, 1000)

  // Listen for file change events to show watch mode indicator
  unsubFileChanged = ws.on('file_changed', (msg) => {
    watchActive.value = true
    const payload = msg.payload as { path?: string } | undefined
    lastFileChange.value = payload?.path ?? 'file changed'
    if (fileChangeTimeout) clearTimeout(fileChangeTimeout)
    fileChangeTimeout = setTimeout(() => {
      lastFileChange.value = null
    }, 3000)
  })
})

onUnmounted(() => {
  clearInterval(interval)
  unsubFileChanged?.()
  if (fileChangeTimeout) {
    clearTimeout(fileChangeTimeout)
    fileChangeTimeout = null
  }
})
</script>

<template>
  <footer class="flex h-6 items-center justify-between border-t border-border bg-surface px-3 text-[11px] text-muted-foreground/70" aria-live="polite">
    <div class="flex items-center gap-3">
      <span class="flex items-center gap-1.5">
        <span
          class="inline-block h-1.5 w-1.5 rounded-full"
          :class="connected ? 'bg-success' : 'bg-destructive animate-pulse'"
        />
        {{ connected ? 'Connected' : 'Disconnected' }}
      </span>
      <span v-if="envStore.activeEnvName" class="border-l border-border/50 pl-3">
        env: <span class="text-foreground/70">{{ envStore.activeEnvName }}</span>
      </span>
      <span v-if="collection.activeFilePath" class="border-l border-border/50 pl-3 max-w-[200px] truncate">
        {{ collection.activeFilePath?.split('/').pop() }}
      </span>
      <span v-if="watchActive" class="flex items-center gap-1 border-l border-border/50 pl-3 text-accent">
        <Eye class="h-3 w-3" />
        Watch
        <span v-if="lastFileChange" class="ml-1 max-w-[120px] truncate text-muted-foreground">
          {{ lastFileChange.split('/').pop() }}
        </span>
      </span>
    </div>
    <div class="flex items-center gap-3">
      <span v-if="requestStore.isExecuting" class="flex items-center gap-1 text-accent">
        <div class="h-3 w-3 animate-spin rounded-full border border-accent/30 border-t-accent" />
        <template v-if="requestStore.executionProgress">
          {{ requestStore.executionProgress.completed }}/{{ requestStore.executionProgress.total }}
          <span class="max-w-[140px] truncate text-muted-foreground">{{ requestStore.executionProgress.currentRequest }}</span>
        </template>
        <template v-else>Executing...</template>
      </span>
      <span v-else-if="requestStore.lastResult" class="flex items-center gap-1">
        <Timer class="h-3 w-3" />
        {{ requestStore.lastResult.response?.duration ?? requestStore.lastResult.duration }}ms
      </span>
    </div>
  </footer>
</template>
