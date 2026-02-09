import { get } from '@/api/client'
import type { SystemInfo } from '@/types/api'

export function getSystemInfo(): Promise<SystemInfo> {
  return get<SystemInfo>('/api/v1/system/info')
}
