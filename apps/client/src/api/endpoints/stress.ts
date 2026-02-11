import { get, post } from '@/api/client'
import type { StressStartRequest, StressStatus, StressResultDTO } from '@/types/api'

export function getStressStatus(): Promise<StressStatus> {
  return get<StressStatus>('/api/v1/stress/status')
}

export function startStress(config: StressStartRequest): Promise<void> {
  return post('/api/v1/stress/start', config)
}

export function stopStress(): Promise<void> {
  return post('/api/v1/stress/stop')
}

export function getStressResult(): Promise<StressResultDTO> {
  return get<StressResultDTO>('/api/v1/stress/result')
}
