import { post } from '@/api/client'
import type { ExportResultDTO } from '@/types/api'

export function exportToCurl(file: string, requestName?: string): Promise<ExportResultDTO> {
  return post<ExportResultDTO>('/api/v1/export/curl', { file, requestName })
}
