<script setup lang="ts">
import { Globe } from 'lucide-vue-next'

defineProps<{ headers: Record<string, string | string[]> }>()
</script>

<template>
  <div class="space-y-1 p-4">
    <template v-if="Object.keys(headers).length > 0">
      <div
        v-for="(values, key) in headers"
        :key="key"
        class="flex gap-2 rounded-md bg-background/50 px-2.5 py-1.5 font-mono text-xs"
      >
        <span class="shrink-0 font-medium text-nord-8">{{ key }}:</span>
        <span class="min-w-0 break-all text-foreground/80">{{ Array.isArray(values) ? values.join(', ') : values }}</span>
      </div>
    </template>
    <div v-else class="flex flex-col items-center gap-2 py-6 text-center">
      <Globe class="h-8 w-8 text-muted-foreground/30" />
      <span class="text-xs text-muted-foreground/60">No response headers</span>
    </div>
  </div>
</template>
