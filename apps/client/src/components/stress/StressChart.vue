<script setup lang="ts">
import { watch } from 'vue'
import type { StressStatsDTO } from '@/types/api'
import VChart from 'vue-echarts'
import { useStressChart } from '@/composables/useStressChart'

const props = defineProps<{ metrics: StressStatsDTO }>()

const { addMetrics, chartOption } = useStressChart()

watch(
  () => props.metrics,
  (m) => addMetrics(m),
  { immediate: true },
)
</script>

<template>
  <div class="rounded-lg border border-border bg-background p-4">
    <h3 class="mb-3 text-sm font-medium text-foreground">Real-time Metrics</h3>
    <VChart :option="chartOption" :style="{ height: '300px' }" autoresize />
  </div>
</template>
