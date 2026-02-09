import { ref, computed } from 'vue'
import type { StressMetrics } from '@/types/api'

interface DataPoint {
  time: number
  rps: number
  avgLatency: number
  p95Latency: number
  errorRate: number
}

const MAX_POINTS = 120

export function useStressChart() {
  const dataPoints = ref<DataPoint[]>([])

  function addMetrics(metrics: StressMetrics) {
    const errorRate =
      metrics.totalRequests > 0
        ? (metrics.failCount / metrics.totalRequests) * 100
        : 0

    dataPoints.value.push({
      time: metrics.elapsed,
      rps: metrics.rps,
      avgLatency: metrics.avgLatency,
      p95Latency: metrics.p95Latency,
      errorRate,
    })

    if (dataPoints.value.length > MAX_POINTS) {
      dataPoints.value = dataPoints.value.slice(-MAX_POINTS)
    }
  }

  function reset() {
    dataPoints.value = []
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
      data: ['RPS', 'Avg Latency', 'P95 Latency'],
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
        name: 'Avg Latency',
        type: 'line',
        smooth: true,
        symbol: 'none',
        yAxisIndex: 1,
        lineStyle: { color: '#88C0D0', width: 2 },
        data: dataPoints.value.map((p) => [p.time, p.avgLatency]),
      },
      {
        name: 'P95 Latency',
        type: 'line',
        smooth: true,
        symbol: 'none',
        yAxisIndex: 1,
        lineStyle: { color: '#D08770', width: 2, type: 'dashed' as const },
        data: dataPoints.value.map((p) => [p.time, p.p95Latency]),
      },
    ],
  }))

  return { dataPoints, addMetrics, reset, chartOption }
}
