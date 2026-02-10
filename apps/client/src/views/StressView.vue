<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import StressConfig from '@/components/stress/StressConfig.vue'
import StressProfiles from '@/components/stress/StressProfiles.vue'
import { Square, Activity, Clock, AlertTriangle, BarChart3 } from 'lucide-vue-next'
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getStressStatus, stopStress } from '@/api/endpoints/stress'
import { ws } from '@/api/websocket'
import type { StressStatus, StressStatsDTO, StressProfile } from '@/types/api'
import { toast } from 'vue-sonner'

const status = ref<StressStatus | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null
let unsubWs: (() => void) | null = null

async function loadStatus() {
  try {
    status.value = await getStressStatus()
  } catch {
    status.value = { running: false, elapsed: 0 }
  }
  syncPolling()
}

function syncPolling() {
  if (status.value?.running && !pollTimer) {
    pollTimer = setInterval(loadStatus, 3000)
  } else if (!status.value?.running && pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function handleStop() {
  if (!window.confirm('Stop the running stress test?')) return
  try {
    await stopStress()
    toast.success('Stress test stopped')
    await loadStatus()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to stop stress test')
  }
}

function handleProfileSelected(_profile: StressProfile) {
  // TODO: populate StressConfig fields from the selected profile
}

onMounted(() => {
  loadStatus()
  unsubWs = ws.on('stress_update', (msg) => {
    const payload = msg.payload as { stats: StressStatsDTO; elapsed: number } | undefined
    if (payload && status.value) {
      status.value = { ...status.value, running: true, stats: payload.stats, elapsed: payload.elapsed }
    }
  })
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
  unsubWs?.()
})
</script>

<template>
  <AppShell>
    <div class="h-full overflow-auto p-6">
      <div class="mb-4 flex items-center justify-between">
        <h1 class="text-lg font-semibold text-foreground">Stress Testing</h1>
        <button
          v-if="status?.running"
          class="flex items-center gap-1.5 rounded-md border border-destructive/50 px-3 py-1 text-xs font-medium text-destructive transition-colors hover:bg-destructive/10"
          @click="handleStop"
        >
          <Square class="h-3 w-3" />
          Stop Test
        </button>
      </div>

      <div v-if="status?.running" class="space-y-4">
        <!-- Running indicator -->
        <div class="flex items-center gap-2">
          <div class="h-2 w-2 animate-pulse rounded-full bg-success" />
          <span class="text-sm font-medium text-foreground">Running</span>
          <span v-if="status.elapsed" class="text-xs text-muted-foreground/50">{{ status.elapsed.toFixed(0) }}s elapsed</span>
        </div>

        <!-- Metrics grid -->
        <div v-if="status.stats" class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div class="rounded-lg border border-border bg-surface p-3">
            <div class="flex items-center gap-1.5 text-[10px] uppercase text-muted-foreground/50">
              <BarChart3 class="h-3 w-3" />
              Total Requests
            </div>
            <div class="mt-1.5 text-xl font-bold tabular-nums text-foreground">{{ status.stats.total.toLocaleString() }}</div>
          </div>
          <div class="rounded-lg border border-border bg-surface p-3">
            <div class="flex items-center gap-1.5 text-[10px] uppercase text-muted-foreground/50">
              <Activity class="h-3 w-3" />
              RPS
            </div>
            <div class="mt-1.5 text-xl font-bold tabular-nums text-accent">{{ status.stats.rps.toFixed(1) }}</div>
          </div>
          <div class="rounded-lg border border-border bg-surface p-3">
            <div class="flex items-center gap-1.5 text-[10px] uppercase text-muted-foreground/50">
              <Clock class="h-3 w-3" />
              P95 Latency
            </div>
            <div class="mt-1.5 text-xl font-bold tabular-nums text-foreground">{{ status.stats.p95Ms.toFixed(0) }}<span class="text-sm font-normal text-muted-foreground">ms</span></div>
          </div>
          <div class="rounded-lg border border-border bg-surface p-3">
            <div class="flex items-center gap-1.5 text-[10px] uppercase text-muted-foreground/50">
              <AlertTriangle class="h-3 w-3" />
              Errors
            </div>
            <div class="mt-1.5 text-xl font-bold tabular-nums" :class="status.stats.errors > 0 ? 'text-destructive' : 'text-success'">{{ status.stats.errors }}</div>
          </div>
        </div>

        <!-- Latency percentiles -->
        <div v-if="status.stats" class="rounded-lg border border-border bg-surface p-4">
          <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/50">Latency Percentiles</h3>
          <div class="grid grid-cols-3 gap-4">
            <div>
              <div class="text-[10px] text-muted-foreground/50">P50</div>
              <div class="font-mono text-sm tabular-nums text-foreground">{{ status.stats.p50Ms.toFixed(1) }}ms</div>
            </div>
            <div>
              <div class="text-[10px] text-muted-foreground/50">P95</div>
              <div class="font-mono text-sm tabular-nums text-foreground">{{ status.stats.p95Ms.toFixed(1) }}ms</div>
            </div>
            <div>
              <div class="text-[10px] text-muted-foreground/50">P99</div>
              <div class="font-mono text-sm tabular-nums text-foreground">{{ status.stats.p99Ms.toFixed(1) }}ms</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Configuration when not running -->
      <div v-else class="space-y-4">
        <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
          <div class="lg:col-span-2">
            <StressConfig :running="false" />
          </div>
          <div>
            <StressProfiles @select="handleProfileSelected" />
          </div>
        </div>
      </div>
    </div>
  </AppShell>
</template>
