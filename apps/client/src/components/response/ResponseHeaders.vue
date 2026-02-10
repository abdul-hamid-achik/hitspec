<script setup lang="ts">
defineProps<{ headers: Record<string, string | string[]> }>()
</script>

<template>
  <div class="space-y-1 p-4">
    <template v-if="Object.keys(headers).length > 0">
      <div v-for="(values, key) in headers" :key="key" class="flex gap-2 font-mono text-xs">
        <span class="font-medium text-nord-8">{{ key }}:</span>
        <span class="text-foreground">{{ Array.isArray(values) ? values.join(', ') : values }}</span>
      </div>
    </template>
    <p v-else class="text-xs text-muted-foreground">No headers</p>
  </div>
</template>
