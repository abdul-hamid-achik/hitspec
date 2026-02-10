<script setup lang="ts">
import { CheckCircle, XCircle, ShieldCheck } from 'lucide-vue-next'
import type { AssertionResult } from '@/types/api'

const { assertions } = defineProps<{ assertions: AssertionResult[] }>()
</script>

<template>
  <div class="space-y-2 p-4">
    <div v-if="assertions.length === 0" class="flex flex-col items-center gap-2 py-6 text-center">
      <ShieldCheck class="h-8 w-8 text-muted-foreground/30" />
      <span class="text-xs text-muted-foreground/60">No assertions to display</span>
    </div>
    <div
      v-for="(a, i) in assertions"
      :key="i"
      class="flex items-start gap-2 rounded-lg border p-2.5 transition-colors"
      :class="a.passed ? 'border-success/20 bg-success/5' : 'border-destructive/20 bg-destructive/5'"
    >
      <CheckCircle v-if="a.passed" class="mt-0.5 h-4 w-4 shrink-0 text-success" />
      <XCircle v-else class="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
      <div class="flex-1 font-mono text-xs">
        <div class="flex items-center gap-2">
          <span class="text-nord-9">{{ a.subject }}</span>
          <span class="font-semibold text-nord-13">{{ a.operator }}</span>
          <span class="text-foreground/80">{{ a.expected }}</span>
        </div>
        <div v-if="!a.passed && a.message" class="mt-1 text-destructive/80">{{ a.message }}</div>
        <div v-if="!a.passed" class="mt-0.5 text-muted-foreground/60">
          got: <span class="text-foreground/80">{{ a.actual }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
