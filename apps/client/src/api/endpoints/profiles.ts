import { get, post, put, del } from '@/api/client'
import type { StressProfile } from '@/types/api'

export function getStressProfiles(): Promise<StressProfile[]> {
  return get<StressProfile[]>('/api/v1/stress/profiles')
}

export function createStressProfile(profile: StressProfile): Promise<StressProfile> {
  return post<StressProfile>('/api/v1/stress/profiles', profile)
}

export function updateStressProfile(name: string, profile: Omit<StressProfile, 'name'>): Promise<StressProfile> {
  return put<StressProfile>(`/api/v1/stress/profiles/${encodeURIComponent(name)}`, profile)
}

export function deleteStressProfile(name: string): Promise<void> {
  return del(`/api/v1/stress/profiles/${encodeURIComponent(name)}`)
}
