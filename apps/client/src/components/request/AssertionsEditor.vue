<script setup lang="ts">
import type { AssertionDTO } from '@/types/api'

defineProps<{ assertions: AssertionDTO[] }>()
</script>

<template>
  <div class="space-y-1">
    <div v-if="assertions.length === 0" class="text-sm text-muted-foreground">No assertions</div>
    <div v-for="(a, i) in assertions" :key="i" class="flex items-center gap-2 font-mono text-xs">
      <span class="text-nord-9">{{ a.subject }}</span>
      <span class="font-medium text-nord-13">{{ a.operator }}</span>
      <span class="text-foreground">{{ a.expected }}</span>
      <span class="text-muted-foreground/50">:{{ a.line }}</span>
    </div>
  </div>
</template>
