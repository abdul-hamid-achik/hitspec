<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { Zap } from 'lucide-vue-next'
import { ref } from 'vue'
import { startStress, stopStress, getStressStatus } from '@/api/endpoints/stress'
import type { StressStatus } from '@/types/api'

const status = ref<StressStatus | null>(null)
const loading = ref(false)

async function loadStatus() {
  status.value = await getStressStatus()
}

async function handleStart() {
  loading.value = true
  try {
    await startStress({
      filePath: '',
      requestIndex: 0,
      concurrency: 10,
      duration: '30s',
      rps: 100,
    })
    await loadStatus()
  } finally {
    loading.value = false
  }
}

async function handleStop() {
  await stopStress()
  await loadStatus()
}

loadStatus()
</script>

<template>
  <AppShell>
    <div class="p-6">
      <h1 class="mb-4 text-lg font-semibold text-foreground">Stress Testing</h1>
      <div v-if="status?.running" class="space-y-4">
        <div class="flex items-center gap-2">
          <div class="h-2 w-2 animate-pulse rounded-full bg-success" />
          <span class="text-sm text-foreground">Running</span>
          <button
            class="ml-4 rounded bg-destructive px-3 py-1 text-sm text-foreground hover:bg-destructive/80"
            @click="handleStop"
          >
            Stop
          </button>
        </div>
        <div v-if="status.metrics" class="grid grid-cols-4 gap-4">
          <div class="rounded border border-border bg-surface p-3">
            <div class="text-xs text-muted-foreground">Total Requests</div>
            <div class="mt-1 text-lg font-semibold text-foreground">{{ status.metrics.totalRequests }}</div>
          </div>
          <div class="rounded border border-border bg-surface p-3">
            <div class="text-xs text-muted-foreground">RPS</div>
            <div class="mt-1 text-lg font-semibold text-foreground">{{ status.metrics.rps.toFixed(1) }}</div>
          </div>
          <div class="rounded border border-border bg-surface p-3">
            <div class="text-xs text-muted-foreground">P95 Latency</div>
            <div class="mt-1 text-lg font-semibold text-foreground">{{ status.metrics.p95Latency.toFixed(0) }}ms</div>
          </div>
          <div class="rounded border border-border bg-surface p-3">
            <div class="text-xs text-muted-foreground">Errors</div>
            <div class="mt-1 text-lg font-semibold text-destructive">{{ status.metrics.failCount }}</div>
          </div>
        </div>
      </div>
      <EmptyState v-else :icon="Zap" title="No stress test running" description="Configure and start a stress test">
        <button
          class="mt-2 rounded bg-accent px-4 py-2 text-sm font-medium text-accent-foreground hover:bg-accent/80"
          :disabled="loading"
          @click="handleStart"
        >
          Start Stress Test
        </button>
      </EmptyState>
    </div>
  </AppShell>
</template>
