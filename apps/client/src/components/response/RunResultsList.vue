<script setup lang="ts">
import type { ExecuteResult, RunResult } from '@/types/api'
import StatusBadge from '@/components/common/StatusBadge.vue'
import MethodBadge from '@/components/common/MethodBadge.vue'
import { CheckCircle, XCircle, MinusCircle, Clock, Zap } from 'lucide-vue-next'

const props = defineProps<{ results: ExecuteResult }>()
const emit = defineEmits<{ select: [result: RunResult] }>()

function resultIcon(result: RunResult) {
  if (result.skipped) return MinusCircle
  return result.passed ? CheckCircle : XCircle
}

function resultIconClass(result: RunResult) {
  if (result.skipped) return 'text-muted-foreground'
  return result.passed ? 'text-success' : 'text-destructive'
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Summary bar -->
    <div class="flex items-center gap-3 border-b border-border px-4 py-2.5">
      <div class="flex items-center gap-2 text-xs font-medium">
        <span class="text-success">{{ results.passed }} passed</span>
        <span v-if="results.failed > 0" class="text-destructive">{{ results.failed }} failed</span>
        <span v-if="results.skipped > 0" class="text-muted-foreground">{{ results.skipped }} skipped</span>
      </div>
      <div class="flex-1" />
      <div class="flex items-center gap-1 text-xs text-muted-foreground">
        <Clock class="h-3.5 w-3.5" />
        <span>{{ results.duration }}ms</span>
      </div>
      <div class="flex items-center gap-1 text-xs text-muted-foreground">
        <Zap class="h-3.5 w-3.5" />
        <span>{{ results.results.length }} requests</span>
      </div>
    </div>

    <!-- Results list -->
    <div class="flex-1 overflow-auto">
      <button
        v-for="(result, i) in results.results"
        :key="i"
        class="flex w-full items-center gap-2.5 border-b border-border px-4 py-2 text-left transition-colors hover:bg-muted/50"
        @click="emit('select', result)"
      >
        <component
          :is="resultIcon(result)"
          class="h-4 w-4 shrink-0"
          :class="resultIconClass(result)"
        />
        <MethodBadge v-if="result.request?.method" :method="result.request.method" size="sm" />
        <span class="flex-1 truncate text-xs text-foreground">{{ result.name }}</span>
        <StatusBadge v-if="result.response?.statusCode" :code="result.response.statusCode" />
        <span class="shrink-0 text-xs tabular-nums text-muted-foreground">{{ result.duration }}ms</span>
      </button>
    </div>
  </div>
</template>
