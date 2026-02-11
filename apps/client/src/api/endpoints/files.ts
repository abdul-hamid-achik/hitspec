import { get, post, del } from '@/api/client'
import { ApiError } from '@/api/client'
import type { WorkspaceInfo, ParsedFile } from '@/types/api'

export function getWorkspace(): Promise<WorkspaceInfo> {
  return get<WorkspaceInfo>('/api/v1/workspace')
}

export function getFile(path: string): Promise<ParsedFile> {
  return get<ParsedFile>(`/api/v1/files/${encodeURIComponent(path)}`)
}

export async function getFileRaw(path: string): Promise<string> {
  const url = `/api/v1/files/raw/${encodeURIComponent(path)}`
  const baseUrl = import.meta.env.DEV ? '' : window.location.origin
  const res = await fetch(`${baseUrl}${url}`)
  if (!res.ok) {
    throw new ApiError(res.status, res.statusText, await res.text())
  }
  return res.text()
}

export async function saveFile(path: string, content: string): Promise<ParsedFile> {
  const url = `/api/v1/files/${encodeURIComponent(path)}`
  const baseUrl = import.meta.env.DEV ? '' : window.location.origin
  const res = await fetch(`${baseUrl}${url}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body: content,
  })
  if (!res.ok) {
    let errorBody: unknown
    try { errorBody = await res.json() } catch { errorBody = await res.text() }
    throw new ApiError(res.status, res.statusText, errorBody)
  }
  return res.json() as Promise<ParsedFile>
}

export function createFile(path: string, content?: string): Promise<ParsedFile> {
  return post<ParsedFile>('/api/v1/files', { path, content })
}

export function deleteFile(path: string): Promise<void> {
  return del(`/api/v1/files/${encodeURIComponent(path)}`)
}
