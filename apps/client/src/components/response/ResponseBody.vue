<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ body: string }>()

const formatted = computed(() => {
  try {
    return JSON.stringify(JSON.parse(props.body), null, 2)
  } catch {
    return props.body
  }
})
</script>

<template>
  <pre class="overflow-auto p-4 font-mono text-xs leading-relaxed text-foreground">{{ formatted }}</pre>
</template>
