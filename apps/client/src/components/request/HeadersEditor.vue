<script setup lang="ts">
import type { HeaderDTO } from '@/types/api'
import { Globe } from 'lucide-vue-next'

const { headers } = defineProps<{ headers: HeaderDTO[] }>()
</script>

<template>
  <div class="space-y-1">
    <div v-if="headers.length === 0" class="flex flex-col items-center gap-2 py-6 text-center">
      <Globe class="h-8 w-8 text-muted-foreground/30" />
      <span class="text-xs text-muted-foreground/60">No headers defined</span>
    </div>
    <div
      v-for="(header, i) in headers"
      :key="i"
      class="flex items-center gap-2 rounded-md bg-background/50 px-2.5 py-1.5 font-mono text-xs"
    >
      <span class="font-medium text-nord-8">{{ header.key }}</span>
      <span class="text-muted-foreground/40">:</span>
      <span class="flex-1 truncate text-foreground/80">{{ header.value }}</span>
    </div>
  </div>
</template>
