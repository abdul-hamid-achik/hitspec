<script setup lang="ts">
import { BarChart3, Activity, Clock, AlertTriangle, Timer, CheckCircle, XCircle, RotateCcw } from 'lucide-vue-next'
import type { StressResultDTO } from '@/types/api'

const { result } = defineProps<{ result: StressResultDTO }>()
const emit = defineEmits<{ (e: 'newTest'): void }>()

function formatMs(ms: number): string {
  return ms < 1000 ? `${ms.toFixed(1)}ms` : `${(ms / 1000).toFixed(2)}s`
}

function formatDuration(ms: number): string {
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)}s`
  const m = Math.floor(s / 60)
  const rem = s % 60
  return `${m}m ${rem.toFixed(0)}s`
}
</script>

<template>
  <div class="space-y-4">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <CheckCircle class="h-4 w-4 text-success" />
        <span class="text-sm font-medium text-foreground">Test Complete</span>
        <span class="text-xs text-muted-foreground">{{ formatDuration(result.durationMs) }}</span>
      </div>
      <button
        class="flex items-center gap-1.5 rounded-md border border-border px-3 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        @click="emit('newTest')"
      >
        <RotateCcw class="h-3 w-3" />
        Run Again
      </button>
    </div>

    <!-- Summary cards -->
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
      <div class="rounded-lg border border-border bg-surface p-3">
        <div class="flex items-center gap-1.5 text-[10px] uppercase text-muted-foreground/50">
          <BarChart3 class="h-3 w-3" />
          Total Requests
        </div>
        <div class="mt-1.5 font-mono text-xl font-bold tabular-nums text-foreground">{{ result.total.toLocaleString() }}</div>
      </div>
      <div class="rounded-lg border border-border bg-surface p-3">
        <div class="flex items-center gap-1.5 text-[10px] uppercase text-muted-foreground/50">
          <Activity class="h-3 w-3" />
          Avg RPS
        </div>
        <div class="mt-1.5 font-mono text-xl font-bold tabular-nums text-accent">{{ result.rps.toFixed(1) }}</div>
      </div>
      <div class="rounded-lg border border-border bg-surface p-3">
        <div class="flex items-center gap-1.5 text-[10px] uppercase text-muted-foreground/50">
          <CheckCircle class="h-3 w-3" />
          Success Rate
        </div>
        <div class="mt-1.5 font-mono text-xl font-bold tabular-nums text-success">{{ (result.successRate * 100).toFixed(1) }}%</div>
      </div>
      <div class="rounded-lg border border-border bg-surface p-3">
        <div class="flex items-center gap-1.5 text-[10px] uppercase text-muted-foreground/50">
          <AlertTriangle class="h-3 w-3" />
          Errors
        </div>
        <div class="mt-1.5 font-mono text-xl font-bold tabular-nums" :class="result.errors > 0 ? 'text-destructive' : 'text-success'">
          {{ result.errors.toLocaleString() }}
        </div>
      </div>
    </div>

    <!-- Latency summary -->
    <div class="rounded-lg border border-border bg-surface p-4">
      <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/50">
        <Clock class="mr-1.5 inline h-3 w-3" />
        Latency Percentiles
      </h3>
      <div class="grid grid-cols-4 gap-4 sm:grid-cols-7">
        <div>
          <div class="text-[10px] text-muted-foreground/50">Min</div>
          <div class="font-mono text-sm tabular-nums text-foreground">{{ formatMs(result.minMs) }}</div>
        </div>
        <div>
          <div class="text-[10px] text-muted-foreground/50">P50</div>
          <div class="font-mono text-sm tabular-nums text-foreground">{{ formatMs(result.p50Ms) }}</div>
        </div>
        <div>
          <div class="text-[10px] text-muted-foreground/50">P95</div>
          <div class="font-mono text-sm tabular-nums text-warning">{{ formatMs(result.p95Ms) }}</div>
        </div>
        <div>
          <div class="text-[10px] text-muted-foreground/50">P99</div>
          <div class="font-mono text-sm tabular-nums text-destructive">{{ formatMs(result.p99Ms) }}</div>
        </div>
        <div>
          <div class="text-[10px] text-muted-foreground/50">Max</div>
          <div class="font-mono text-sm tabular-nums text-foreground">{{ formatMs(result.maxMs) }}</div>
        </div>
        <div>
          <div class="text-[10px] text-muted-foreground/50">Mean</div>
          <div class="font-mono text-sm tabular-nums text-foreground">{{ formatMs(result.meanMs) }}</div>
        </div>
        <div>
          <div class="text-[10px] text-muted-foreground/50">StdDev</div>
          <div class="font-mono text-sm tabular-nums text-foreground">{{ formatMs(result.stdDevMs) }}</div>
        </div>
      </div>
    </div>

    <!-- Per-request breakdown -->
    <div v-if="result.breakdown.length > 0" class="rounded-lg border border-border bg-surface p-4">
      <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/50">
        <Timer class="mr-1.5 inline h-3 w-3" />
        Per-Request Breakdown
      </h3>
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="border-b border-border text-[10px] uppercase text-muted-foreground/50">
              <th class="pb-2 pr-4 font-medium">Request</th>
              <th class="pb-2 pr-4 text-right font-medium">Total</th>
              <th class="pb-2 pr-4 text-right font-medium">Success</th>
              <th class="pb-2 pr-4 text-right font-medium">Errors</th>
              <th class="pb-2 pr-4 text-right font-medium">P50</th>
              <th class="pb-2 pr-4 text-right font-medium">P95</th>
              <th class="pb-2 pr-4 text-right font-medium">P99</th>
              <th class="pb-2 text-right font-medium">Mean</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="req in result.breakdown"
              :key="req.name"
              class="border-b border-border/50 last:border-0"
            >
              <td class="py-2 pr-4 font-mono text-foreground">{{ req.name }}</td>
              <td class="py-2 pr-4 text-right font-mono tabular-nums text-foreground">{{ req.total.toLocaleString() }}</td>
              <td class="py-2 pr-4 text-right font-mono tabular-nums text-success">{{ req.success.toLocaleString() }}</td>
              <td class="py-2 pr-4 text-right font-mono tabular-nums" :class="req.errors > 0 ? 'text-destructive' : 'text-muted-foreground'">
                {{ req.errors.toLocaleString() }}
              </td>
              <td class="py-2 pr-4 text-right font-mono tabular-nums text-foreground">{{ formatMs(req.p50Ms) }}</td>
              <td class="py-2 pr-4 text-right font-mono tabular-nums text-foreground">{{ formatMs(req.p95Ms) }}</td>
              <td class="py-2 pr-4 text-right font-mono tabular-nums text-foreground">{{ formatMs(req.p99Ms) }}</td>
              <td class="py-2 text-right font-mono tabular-nums text-foreground">{{ formatMs(req.meanMs) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Threshold results -->
    <div v-if="result.thresholds && result.thresholds.length > 0" class="rounded-lg border border-border bg-surface p-4">
      <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/50">Thresholds</h3>
      <div class="space-y-2">
        <div
          v-for="threshold in result.thresholds"
          :key="threshold.name"
          class="flex items-center gap-2 rounded-md px-3 py-2"
          :class="threshold.passed ? 'bg-success/5' : 'bg-destructive/5'"
        >
          <CheckCircle v-if="threshold.passed" class="h-3.5 w-3.5 shrink-0 text-success" />
          <XCircle v-else class="h-3.5 w-3.5 shrink-0 text-destructive" />
          <span class="text-xs font-medium text-foreground">{{ threshold.name }}</span>
          <span class="ml-auto font-mono text-xs tabular-nums text-muted-foreground">
            {{ threshold.actual }} <span class="text-muted-foreground/50">(expected: {{ threshold.expected }})</span>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
