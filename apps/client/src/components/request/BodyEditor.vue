<script setup lang="ts">
import type { BodyDTO } from '@/types/api'
import { AlignLeft } from 'lucide-vue-next'

defineProps<{ body?: BodyDTO | null }>()
</script>

<template>
  <div>
    <div v-if="!body" class="flex flex-col items-center gap-2 py-6 text-center">
      <AlignLeft class="h-8 w-8 text-muted-foreground/30" />
      <span class="text-xs text-muted-foreground/60">No request body</span>
    </div>
    <div v-else>
      <div class="mb-2 flex items-center gap-2">
        <span class="rounded bg-accent/10 px-1.5 py-0.5 text-[10px] font-medium uppercase text-accent">{{ body.contentType }}</span>
      </div>
      <pre class="overflow-auto rounded border border-border bg-background p-3 font-mono text-xs text-foreground">{{ body.raw }}</pre>
    </div>
  </div>
</template>
