<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import type { StressStatus, StressMetrics } from '@/types/api'
import { getStressStatus } from '@/api/endpoints/stress'
import { ws } from '@/api/websocket'
import StressChart from './StressChart.vue'
import StressConfig from './StressConfig.vue'
import StressMetricsDisplay from './StressMetrics.vue'

const status = ref<StressStatus>({ running: false })
const latestMetrics = ref<StressMetrics | null>(null)

let unsub: (() => void) | null = null

onMounted(async () => {
  status.value = await getStressStatus()
  unsub = ws.on('stress_update', (msg) => {
    latestMetrics.value = msg.payload as StressMetrics
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
