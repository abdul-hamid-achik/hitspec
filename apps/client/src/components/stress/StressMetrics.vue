<script setup lang="ts">
import type { StressStatsDTO } from '@/types/api'

const { metrics } = defineProps<{ metrics: StressStatsDTO }>()

function formatMs(ms: number): string {
  return ms < 1000 ? `${ms.toFixed(1)}ms` : `${(ms / 1000).toFixed(2)}s`
}
</script>

<template>
  <div class="rounded-lg border border-border bg-background p-4">
    <h3 class="mb-3 text-sm font-medium text-foreground">Metrics</h3>
    <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
      <div class="rounded-md bg-surface p-3">
        <div class="text-xs text-muted-foreground">Total Requests</div>
        <div class="mt-1 text-xl font-bold text-foreground">{{ metrics.total.toLocaleString() }}</div>
      </div>
      <div class="rounded-md bg-surface p-3">
        <div class="text-xs text-muted-foreground">Success Rate</div>
        <div class="mt-1 text-xl font-bold text-success">
          {{ metrics.total > 0 ? ((metrics.success / metrics.total) * 100).toFixed(1) : 0 }}%
        </div>
      </div>
      <div class="rounded-md bg-surface p-3">
        <div class="text-xs text-muted-foreground">RPS</div>
        <div class="mt-1 text-xl font-bold text-accent">{{ metrics.rps.toFixed(1) }}</div>
      </div>
      <div class="rounded-md bg-surface p-3">
        <div class="text-xs text-muted-foreground">P50</div>
        <div class="mt-1 text-lg font-semibold text-foreground">{{ formatMs(metrics.p50Ms) }}</div>
      </div>
      <div class="rounded-md bg-surface p-3">
        <div class="text-xs text-muted-foreground">P95</div>
        <div class="mt-1 text-lg font-semibold text-warning">{{ formatMs(metrics.p95Ms) }}</div>
      </div>
      <div class="rounded-md bg-surface p-3">
        <div class="text-xs text-muted-foreground">P99</div>
        <div class="mt-1 text-lg font-semibold text-destructive">{{ formatMs(metrics.p99Ms) }}</div>
      </div>
      <div class="rounded-md bg-surface p-3">
        <div class="text-xs text-muted-foreground">Errors</div>
        <div class="mt-1 text-xl font-bold" :class="metrics.errors > 0 ? 'text-destructive' : 'text-success'">
          {{ metrics.errors }}
        </div>
      </div>
    </div>
  </div>
</template>
