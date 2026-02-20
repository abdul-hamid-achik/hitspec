import { computeDiff, type DiffLine } from '@/lib/diff'

export interface DiffWorkerRequest {
  id: number
  oldText: string
  newText: string
}

export interface DiffWorkerResponse {
  id: number
  lines: DiffLine[]
}

self.onmessage = (e: MessageEvent<DiffWorkerRequest>) => {
  const { id, oldText, newText } = e.data
  const lines = computeDiff(oldText, newText)
  self.postMessage({ id, lines } satisfies DiffWorkerResponse)
}
