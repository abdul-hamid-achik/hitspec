<script setup lang="ts">
import { ref } from 'vue'
import { Play, Square } from 'lucide-vue-next'
import { startStress, stopStress } from '@/api/endpoints/stress'
import { useCollectionStore } from '@/stores/collection'
import { toast } from 'vue-sonner'

const { running } = defineProps<{ running: boolean }>()

const collection = useCollectionStore()
const concurrency = ref(10)
const duration = ref('30s')
const rps = ref(100)

async function handleStart() {
  if (!collection.activeFilePath) {
    toast.error('No file selected')
    return
  }
  try {
    await startStress({
      files: [collection.activeFilePath],
      duration: duration.value,
      rate: rps.value,
      vus: concurrency.value,
    })
    toast.success('Stress test started')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to start')
  }
}

async function handleStop() {
  try {
    await stopStress()
    toast.success('Stress test stopped')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to stop')
  }
}
</script>

<template>
  <div class="rounded-lg border border-border bg-background p-4">
    <h3 class="mb-3 text-sm font-medium text-foreground">Configuration</h3>
    <div v-if="!collection.activeFilePath" class="mb-3 rounded-md border border-warning/30 bg-warning/5 px-3 py-2 text-xs text-warning">
      No file selected. Open a .http file in the Workspace first.
    </div>
    <div v-else class="mb-3 flex items-center gap-2 text-xs text-muted-foreground">
      <span>Target file:</span>
      <span class="rounded bg-surface px-2 py-0.5 font-mono text-foreground">{{ collection.activeFilePath.split('/').pop() }}</span>
    </div>
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <div>
        <label class="mb-1 block text-xs text-muted-foreground">Concurrency</label>
        <input
          v-model.number="concurrency"
          type="number"
          min="1"
          class="w-full rounded-md border border-border bg-input px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
      </div>
      <div>
        <label class="mb-1 block text-xs text-muted-foreground">Duration</label>
        <input
          v-model="duration"
          type="text"
          placeholder="30s"
          class="w-full rounded-md border border-border bg-input px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
      </div>
      <div>
        <label class="mb-1 block text-xs text-muted-foreground">Target RPS</label>
        <input
          v-model.number="rps"
          type="number"
          min="1"
          class="w-full rounded-md border border-border bg-input px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
      </div>
    </div>
    <div class="mt-4 flex gap-2">
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
</template>
