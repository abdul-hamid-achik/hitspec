import { get, put } from '@/api/client'
import type { EnvironmentDTO } from '@/types/api'

export function getEnvironments(): Promise<EnvironmentDTO[]> {
  return get<EnvironmentDTO[]>('/api/v1/environments')
}

export function selectEnvironment(name: string): Promise<void> {
  // Set the active environment via the config endpoint's defaultEnvironment field
  return put('/api/v1/config', { defaultEnvironment: name })
}

export function updateEnvironment(env: EnvironmentDTO): Promise<void> {
  return put(`/api/v1/environments/${encodeURIComponent(env.name)}`, env)
}
