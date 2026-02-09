import { get } from '@/api/client'
import type { WorkspaceInfo, ParsedFile } from '@/types/api'

export function getWorkspace(): Promise<WorkspaceInfo> {
  return get<WorkspaceInfo>('/api/v1/workspace')
}

export function getFile(path: string): Promise<ParsedFile> {
  return get<ParsedFile>(`/api/v1/files/${encodeURIComponent(path)}`)
}
