import { get, put } from '@/api/client'
import type { ConfigDTO } from '@/types/api'

export function getConfig(): Promise<ConfigDTO> {
  return get<ConfigDTO>('/api/v1/config')
}

export function updateConfig(config: Partial<ConfigDTO>): Promise<void> {
  return put('/api/v1/config', config)
}
