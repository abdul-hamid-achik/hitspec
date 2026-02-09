import { get, post } from '@/api/client'
import type { StressConfig, StressStatus } from '@/types/api'

export function getStressStatus(): Promise<StressStatus> {
  return get<StressStatus>('/api/v1/stress/status')
}

export function startStress(config: StressConfig): Promise<void> {
  return post('/api/v1/stress/start', config)
}

export function stopStress(): Promise<void> {
  return post('/api/v1/stress/stop')
}
