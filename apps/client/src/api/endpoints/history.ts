import { get, del } from '@/api/client'
import type { HistoryEntry } from '@/types/api'

export function getHistory(): Promise<HistoryEntry[]> {
  return get<HistoryEntry[]>('/api/v1/history')
}

export function clearHistory(): Promise<void> {
  return del('/api/v1/history')
}
