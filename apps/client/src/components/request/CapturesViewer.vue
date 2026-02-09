<script setup lang="ts">
import { computed } from 'vue'
import { useRequestStore } from '@/stores/request'

const requestStore = useRequestStore()

const captures = computed(() => requestStore.activeRequest?.captures ?? [])
</script>

<template>
  <div>
    <h3 class="mb-3 text-sm font-medium text-foreground">Captures</h3>
    <div v-if="captures.length > 0" class="space-y-1">
      <div
        v-for="(capture, i) in captures"
        :key="i"
        class="flex items-center gap-2 rounded-md border border-border bg-nord-0 px-3 py-2 font-mono text-sm"
      >
        <span class="text-warning">{{ capture.name }}</span>
        <span class="text-nord-3">=</span>
        <span class="text-muted-foreground">{{ capture.source }}</span>
        <span class="text-foreground">{{ capture.expression }}</span>
        <span class="ml-auto text-xs text-nord-3">L{{ capture.line }}</span>
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground">
      No captures defined
    </p>
  </div>
</template>
