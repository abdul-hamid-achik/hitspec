<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ body?: string }>()

const formatted = computed(() => {
  if (!props.body) return ''
  try {
    return JSON.stringify(JSON.parse(props.body), null, 2)
  } catch {
    return props.body
  }
})
</script>

<template>
  <pre v-if="formatted" class="overflow-auto p-4 font-mono text-xs leading-relaxed text-foreground">{{ formatted }}</pre>
  <p v-else class="p-4 text-xs text-muted-foreground">No response body</p>
</template>
