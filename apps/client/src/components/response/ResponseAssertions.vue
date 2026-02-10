<script setup lang="ts">
import { CheckCircle, XCircle } from 'lucide-vue-next'
import type { AssertionResult } from '@/types/api'

defineProps<{ assertions: AssertionResult[] }>()
</script>

<template>
  <div class="space-y-2 p-4">
    <div
      v-for="(a, i) in assertions"
      :key="i"
      class="flex items-start gap-2 rounded border border-border bg-background p-2"
    >
      <CheckCircle v-if="a.passed" class="mt-0.5 h-4 w-4 shrink-0 text-success" />
      <XCircle v-else class="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
      <div class="flex-1 font-mono text-xs">
        <div class="flex items-center gap-2">
          <span class="text-nord-9">{{ a.subject }}</span>
          <span class="font-medium text-nord-13">{{ a.operator }}</span>
          <span class="text-foreground">{{ a.expected }}</span>
        </div>
        <div v-if="!a.passed && a.message" class="mt-1 text-destructive">{{ a.message }}</div>
        <div v-if="!a.passed" class="mt-0.5 text-muted-foreground">
          got: <span class="text-foreground">{{ a.actual }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
