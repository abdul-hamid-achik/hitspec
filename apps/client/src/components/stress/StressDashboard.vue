<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import type { StressStatus, StressStatsDTO } from '@/types/api'
import { getStressStatus } from '@/api/endpoints/stress'
import { ws } from '@/api/websocket'
import StressChart from './StressChart.vue'
import StressConfig from './StressConfig.vue'
import StressMetricsDisplay from './StressMetrics.vue'

const status = ref<StressStatus>({ running: false, elapsed: 0 })
const latestMetrics = ref<StressStatsDTO | null>(null)

let unsub: (() => void) | null = null

onMounted(async () => {
  try {
    status.value = await getStressStatus()
    if (status.value.stats) {
      latestMetrics.value = status.value.stats
    }
  } catch {
    // Server may be unreachable; keep default status
  }
  unsub = ws.on('stress_update', (msg) => {
    const payload = msg.payload as { stats: StressStatsDTO; elapsed: number }
    latestMetrics.value = payload.stats
    status.value.running = true
  })
})

onBeforeUnmount(() => {
  unsub?.()
})
</script>

<template>
  <div class="space-y-6">
    <StressConfig :running="status.running" />
    <StressChart v-if="latestMetrics" :metrics="latestMetrics" />
    <StressMetricsDisplay v-if="latestMetrics" :metrics="latestMetrics" />
  </div>
</template>
