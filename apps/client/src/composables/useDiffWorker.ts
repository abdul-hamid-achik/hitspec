import { ref, onBeforeUnmount } from 'vue'
import type { DiffLine } from '@/lib/diff'
import type { DiffWorkerRequest, DiffWorkerResponse } from '@/workers/diff.worker'

export function useDiffWorker() {
  const computing = ref(false)
  let worker: Worker | null = null
  let nextId = 0
  let pending: { resolve: (lines: DiffLine[]) => void; reject: (err: Error) => void } | null = null

  function getWorker(): Worker {
    if (!worker) {
      worker = new Worker(new URL('../workers/diff.worker.ts', import.meta.url), { type: 'module' })
      worker.onmessage = (e: MessageEvent<DiffWorkerResponse>) => {
        computing.value = false
        pending?.resolve(e.data.lines)
        pending = null
      }
      worker.onerror = (e) => {
        computing.value = false
        pending?.reject(new Error(e.message))
        pending = null
      }
    }
    return worker
  }

  function compute(oldText: string, newText: string): Promise<DiffLine[]> {
    return new Promise((resolve, reject) => {
      pending = { resolve, reject }
      computing.value = true
      const id = ++nextId
      getWorker().postMessage({ id, oldText, newText } satisfies DiffWorkerRequest)
    })
  }

  onBeforeUnmount(() => {
    worker?.terminate()
    worker = null
  })

  return { compute, computing }
}
