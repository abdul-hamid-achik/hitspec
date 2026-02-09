import { get } from '@/api/client'
import type { WorkspaceInfo } from '@/types/api'

export function getWorkspace(): Promise<WorkspaceInfo> {
  return get<WorkspaceInfo>('/api/v1/workspace')
}
