<script setup lang="ts">
import type { BodyDTO } from '@/types/api'

defineProps<{ body?: BodyDTO | null }>()
</script>

<template>
  <div>
    <div v-if="!body" class="text-sm text-muted-foreground">No body</div>
    <div v-else>
      <div class="mb-2 text-xs text-muted-foreground">Type: {{ body.type }}</div>
      <pre class="overflow-auto rounded border border-border bg-background p-3 font-mono text-xs text-foreground">{{ body.content }}</pre>
    </div>
  </div>
</template>
