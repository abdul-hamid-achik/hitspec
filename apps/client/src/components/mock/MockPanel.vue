<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Play, Square } from 'lucide-vue-next'
import type { MockConfig } from '@/types/api'
import { getMockConfig, startMock, stopMock } from '@/api/endpoints/mock'
import MockRouteList from './MockRouteList.vue'
import { toast } from 'vue-sonner'

const config = ref<MockConfig | null>(null)
const running = ref(false)
const port = ref(9090)

onMounted(async () => {
  try {
    config.value = await getMockConfig()
    running.value = true
  } catch {
    running.value = false
  }
})

async function handleStart() {
  try {
    await startMock({ port: port.value, routes: [] })
    running.value = true
    toast.success('Mock server started')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to start')
  }
}

async function handleStop() {
  try {
    await stopMock()
    running.value = false
    toast.success('Mock server stopped')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to stop')
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-lg border border-border bg-nord-0 p-4">
      <h3 class="mb-3 text-sm font-medium text-foreground">Mock Server</h3>
      <div class="flex items-center gap-4">
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Port</label>
          <input
            v-model.number="port"
            type="number"
            class="w-32 rounded-md border border-border bg-input px-3 py-1.5 text-sm text-foreground"
          />
        </div>
        <div class="flex items-end gap-2 self-end">
          <button
            v-if="!running"
            class="flex items-center gap-1.5 rounded-md bg-accent px-4 py-1.5 text-sm font-medium text-accent-foreground hover:bg-accent/80"
            @click="handleStart"
          >
            <Play :size="14" /> Start
          </button>
          <button
            v-else
            class="flex items-center gap-1.5 rounded-md bg-destructive px-4 py-1.5 text-sm font-medium text-foreground hover:bg-destructive/80"
            @click="handleStop"
          >
            <Square :size="14" /> Stop
          </button>
        </div>
      </div>
    </div>
    <MockRouteList v-if="config" :routes="config.routes" />
  </div>
</template>
