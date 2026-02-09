<script setup lang="ts">
import type { HeaderDTO } from '@/types/api'

defineProps<{ headers: HeaderDTO[] }>()
</script>

<template>
  <div class="space-y-1">
    <div v-if="headers.length === 0" class="text-sm text-muted-foreground">No headers</div>
    <div v-for="(header, i) in headers" :key="i" class="flex items-center gap-2 font-mono text-xs">
      <span class="font-medium text-nord-8">{{ header.key }}</span>
      <span class="text-muted-foreground">:</span>
      <span class="text-foreground">{{ header.value }}</span>
    </div>
  </div>
</template>
