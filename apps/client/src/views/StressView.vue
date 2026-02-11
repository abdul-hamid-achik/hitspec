<script setup lang="ts">
import StressConfig from '@/components/stress/StressConfig.vue'
import StressProfiles from '@/components/stress/StressProfiles.vue'
import StressResults from '@/components/stress/StressResults.vue'
import StressChart from '@/components/stress/StressChart.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Square, Activity, Clock, AlertTriangle, BarChart3, AlertCircle } from 'lucide-vue-next'
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getStressStatus, stopStress, getStressResult } from '@/api/endpoints/stress'
import { ws } from '@/api/websocket'
import type { StressStatus, StressStatsDTO, StressProfile, StressResultDTO } from '@/types/api'
import { toast } from 'vue-sonner'

const status = ref<StressStatus | null>(null)
const result = ref<StressResultDTO | null>(null)
const loadingStatus = ref(true)
const loadError = ref<string | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null
let unsubWs: (() => void) | null = null

async function loadStatus() {
  loadError.value = null
  loadingStatus.value = true
  try {
    status.value = await getStressStatus()
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : 'Failed to load stress test status'
    status.value = { running: false, elapsed: 0 }
  } finally {
    loadingStatus.value = false
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

async function fetchResult() {
  try {
    result.value = await getStressResult()
  } catch {
    // No result available — stay on config view
    result.value = null
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

function handleNewTest() {
  result.value = null
}

const selectedProfile = ref<StressProfile | null>(null)

function handleProfileSelected(profile: StressProfile) {
  selectedProfile.value = profile
}

onMounted(async () => {
  await loadStatus()

  // If not running, try to fetch the last result
  if (!status.value?.running) {
    await fetchResult()
  }

  unsubWs = ws.on('stress_update', (msg) => {
    const payload = msg.payload as { running: boolean; completed?: boolean; stats: StressStatsDTO; elapsed: number } | undefined
    if (payload && status.value) {
      status.value = { ...status.value, running: payload.running, stats: payload.stats, elapsed: payload.elapsed }
      syncPolling()

      // When test completes, fetch the full result
      if (!payload.running && payload.completed) {
        fetchResult()
      }
    }
  })
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
  unsubWs?.()
})
</script>

<template>
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

      <LoadingSpinner v-if="loadingStatus && !status?.running" label="Loading stress test status..." />

      <div v-if="loadError" class="mb-4 flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 p-3">
        <AlertCircle class="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
        <span class="flex-1 text-xs text-destructive">{{ loadError }}</span>
        <button
          class="shrink-0 rounded-md border border-border px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
          @click="loadStatus"
        >
          Retry
        </button>
      </div>

      <!-- Running state -->
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

        <!-- Real-time chart -->
        <StressChart v-if="status.stats" :metrics="status.stats" />
      </div>

      <!-- Results state -->
      <div v-else-if="result" class="space-y-4">
        <StressResults :result="result" @new-test="handleNewTest" />
      </div>

      <!-- Configuration when not running and no results -->
      <div v-else class="space-y-4">
        <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
          <div class="lg:col-span-2">
            <StressConfig :running="false" :profile="selectedProfile" />
          </div>
          <div>
            <StressProfiles @select="handleProfileSelected" />
          </div>
        </div>
      </div>
  </div>
</template>
