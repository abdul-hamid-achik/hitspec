import { get } from '@/api/client'
import type { StressProfile } from '@/types/api'

export function getStressProfiles(): Promise<StressProfile[]> {
  return get<StressProfile[]>('/api/v1/stress/profiles')
}
