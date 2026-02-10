import { ref, computed } from 'vue'
import type { StressStatsDTO } from '@/types/api'

interface DataPoint {
  time: number
  rps: number
  p50Ms: number
  p95Ms: number
  errorRate: number
}

const MAX_POINTS = 120

export function useStressChart() {
  const dataPoints = ref<DataPoint[]>([])
  let elapsedCounter = 0

  function addMetrics(metrics: StressStatsDTO) {
    elapsedCounter += 0.5

    dataPoints.value.push({
      time: elapsedCounter,
      rps: metrics.rps,
      p50Ms: metrics.p50Ms,
      p95Ms: metrics.p95Ms,
      errorRate: metrics.errorRate * 100,
    })

    if (dataPoints.value.length > MAX_POINTS) {
      dataPoints.value = dataPoints.value.slice(-MAX_POINTS)
    }
  }

  function reset() {
    dataPoints.value = []
    elapsedCounter = 0
  }

  const chartOption = computed(() => ({
    animation: false,
    backgroundColor: 'transparent',
    grid: {
      top: 40,
      right: 60,
      bottom: 30,
      left: 60,
    },
    legend: {
      data: ['RPS', 'P50 Latency', 'P95 Latency'],
      textStyle: { color: '#D8DEE9' },
      top: 5,
    },
    xAxis: {
      type: 'value' as const,
      name: 'Time (s)',
      nameTextStyle: { color: '#4C566A' },
      axisLine: { lineStyle: { color: '#4C566A' } },
      axisLabel: { color: '#D8DEE9' },
      data: dataPoints.value.map((p) => p.time),
    },
    yAxis: [
      {
        type: 'value' as const,
        name: 'RPS',
        nameTextStyle: { color: '#A3BE8C' },
        axisLine: { lineStyle: { color: '#A3BE8C' } },
        axisLabel: { color: '#D8DEE9' },
        splitLine: { lineStyle: { color: '#3B4252' } },
      },
      {
        type: 'value' as const,
        name: 'Latency (ms)',
        nameTextStyle: { color: '#88C0D0' },
        axisLine: { lineStyle: { color: '#88C0D0' } },
        axisLabel: { color: '#D8DEE9' },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: 'RPS',
        type: 'line',
        smooth: true,
        symbol: 'none',
        lineStyle: { color: '#A3BE8C', width: 2 },
        data: dataPoints.value.map((p) => [p.time, p.rps]),
      },
      {
        name: 'P50 Latency',
        type: 'line',
        smooth: true,
        symbol: 'none',
        yAxisIndex: 1,
        lineStyle: { color: '#88C0D0', width: 2 },
        data: dataPoints.value.map((p) => [p.time, p.p50Ms]),
      },
      {
        name: 'P95 Latency',
        type: 'line',
        smooth: true,
        symbol: 'none',
        yAxisIndex: 1,
        lineStyle: { color: '#D08770', width: 2, type: 'dashed' as const },
        data: dataPoints.value.map((p) => [p.time, p.p95Ms]),
      },
    ],
  }))

  return { dataPoints, addMetrics, reset, chartOption }
}
