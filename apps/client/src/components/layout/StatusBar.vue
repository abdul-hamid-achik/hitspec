<script setup lang="ts">
import { Circle } from 'lucide-vue-next'
import { ws } from '@/api/websocket'
import { ref, onMounted, onUnmounted } from 'vue'
import { useEnvironmentStore } from '@/stores/environment'
import { useRequestStore } from '@/stores/request'

const envStore = useEnvironmentStore()
const requestStore = useRequestStore()
const connected = ref(false)
let interval: ReturnType<typeof setInterval>

onMounted(() => {
  interval = setInterval(() => {
    connected.value = ws.connected
  }, 1000)
})

onUnmounted(() => clearInterval(interval))
</script>

<template>
  <footer class="flex h-6 items-center justify-between border-t border-border bg-surface px-3 text-xs text-muted-foreground">
    <div class="flex items-center gap-3">
      <span class="flex items-center gap-1">
        <Circle class="h-2 w-2" :class="connected ? 'fill-success text-success' : 'fill-destructive text-destructive'" />
        {{ connected ? 'Connected' : 'Disconnected' }}
      </span>
      <span v-if="envStore.activeEnvName">env: {{ envStore.activeEnvName }}</span>
    </div>
    <div class="flex items-center gap-3">
      <span v-if="requestStore.lastResult">
        {{ requestStore.lastResult.duration }}ms
      </span>
    </div>
  </footer>
</template>
