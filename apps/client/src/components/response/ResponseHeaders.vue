<script setup lang="ts">
defineProps<{ headers: Record<string, string[]> }>()
</script>

<template>
  <div class="space-y-1 p-4">
    <div v-for="(values, key) in headers" :key="key" class="flex gap-2 font-mono text-xs">
      <span class="font-medium text-nord-8">{{ key }}:</span>
      <span class="text-foreground">{{ values.join(', ') }}</span>
    </div>
  </div>
</template>
