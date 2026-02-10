import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { HistoryList, HistoryRun, HistoryResult } from '@/types/api'

vi.mock('@/api/endpoints/history', () => ({
  fetchRuns: vi.fn(),
  fetchRunDetails: vi.fn(),
  clearAllHistory: vi.fn(),
  deleteRun: vi.fn(),
}))

vi.mock('vue-sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

vi.mock('@/api/endpoints/execute', () => ({
  executeRequest: vi.fn(),
  executeFile: vi.fn(),
}))

import { fetchRuns, fetchRunDetails, clearAllHistory, deleteRun } from '@/api/endpoints/history'
import { toast } from 'vue-sonner'
import { useHistoryStore } from '@/stores/history'

const makeRun = (id: number, overrides: Partial<HistoryRun> = {}): HistoryRun => ({
  id,
  filePath: 'api.http',
  startedAt: '2026-01-01T00:00:00Z',
  durationMs: 500,
  passed: 1,
  failed: 0,
  skipped: 0,
  total: 1,
  ...overrides,
})

const makeResult = (id: number): HistoryResult => ({
  id,
  requestName: `Request ${id}`,
  method: 'GET',
  url: '/test',
  statusCode: 200,
  durationMs: 100,
  passed: true,
})

const makeHistoryList = (runs: HistoryRun[], total: number): HistoryList => ({
  runs,
  total,
  limit: 20,
  offset: 0,
})

describe('History Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(fetchRuns).mockReset()
    vi.mocked(fetchRunDetails).mockReset()
    vi.mocked(clearAllHistory).mockReset()
    vi.mocked(deleteRun).mockReset()
    vi.mocked(toast.success).mockReset()
    vi.mocked(toast.error).mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('initial state', () => {
    it('should have correct defaults', () => {
      const store = useHistoryStore()

      expect(store.runs).toEqual([])
      expect(store.totalRuns).toBe(0)
      expect(store.currentPage).toBe(0)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.expandedRunId).toBeNull()
      expect(store.expandedResults).toEqual([])
      expect(store.loadingDetails).toBe(false)
    })
  })

  describe('loadRuns', () => {
    it('should fetch and store runs', async () => {
      const runs = [makeRun(1), makeRun(2)]
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList(runs, 2))
      const store = useHistoryStore()

      await store.loadRuns()

      expect(fetchRuns).toHaveBeenCalledWith(20, 0)
      expect(store.runs).toEqual(runs)
      expect(store.totalRuns).toBe(2)
      expect(store.currentPage).toBe(0)
      expect(store.loading).toBe(false)
    })

    it('should fetch specific page', async () => {
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList([], 50))
      const store = useHistoryStore()

      await store.loadRuns(2)

      expect(fetchRuns).toHaveBeenCalledWith(20, 40)
      expect(store.currentPage).toBe(2)
    })

    it('should set loading during fetch', async () => {
      let resolvePromise!: (value: HistoryList) => void
      vi.mocked(fetchRuns).mockReturnValueOnce(
        new Promise((resolve) => { resolvePromise = resolve }),
      )
      const store = useHistoryStore()

      const promise = store.loadRuns()
      expect(store.loading).toBe(true)

      resolvePromise(makeHistoryList([], 0))
      await promise
      expect(store.loading).toBe(false)
    })

    it('should set error on failure', async () => {
      vi.mocked(fetchRuns).mockRejectedValueOnce(new Error('Server error'))
      const store = useHistoryStore()

      await store.loadRuns()

      expect(store.error).toBe('Server error')
      expect(store.loading).toBe(false)
    })

    it('should set generic error for non-Error throws', async () => {
      vi.mocked(fetchRuns).mockRejectedValueOnce('unknown')
      const store = useHistoryStore()

      await store.loadRuns()

      expect(store.error).toBe('Failed to load history')
    })
  })

  describe('loadRunDetails', () => {
    it('should fetch and expand run details', async () => {
      const results = [makeResult(1), makeResult(2)]
      vi.mocked(fetchRunDetails).mockResolvedValueOnce({
        run: makeRun(5),
        results,
      })
      const store = useHistoryStore()

      await store.loadRunDetails(5)

      expect(fetchRunDetails).toHaveBeenCalledWith(5)
      expect(store.expandedRunId).toBe(5)
      expect(store.expandedResults).toEqual(results)
      expect(store.loadingDetails).toBe(false)
    })

    it('should collapse when clicking already-expanded run', async () => {
      const results = [makeResult(1)]
      vi.mocked(fetchRunDetails).mockResolvedValueOnce({
        run: makeRun(3),
        results,
      })
      const store = useHistoryStore()

      await store.loadRunDetails(3)
      expect(store.expandedRunId).toBe(3)

      // Click same run again -> collapse
      await store.loadRunDetails(3)
      expect(store.expandedRunId).toBeNull()
      expect(store.expandedResults).toEqual([])
    })

    it('should show toast error on failure', async () => {
      vi.mocked(fetchRunDetails).mockRejectedValueOnce(new Error('Not found'))
      const store = useHistoryStore()

      await store.loadRunDetails(99)

      expect(toast.error).toHaveBeenCalledWith('Not found')
      expect(store.loadingDetails).toBe(false)
    })
  })

  describe('clearAll', () => {
    it('should clear all history and reset state', async () => {
      vi.mocked(clearAllHistory).mockResolvedValueOnce(undefined)
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList([makeRun(1)], 1))
      const store = useHistoryStore()

      await store.loadRuns()
      expect(store.runs).toHaveLength(1)

      await store.clearAll()

      expect(clearAllHistory).toHaveBeenCalled()
      expect(store.runs).toEqual([])
      expect(store.totalRuns).toBe(0)
      expect(store.currentPage).toBe(0)
      expect(store.expandedRunId).toBeNull()
      expect(store.expandedResults).toEqual([])
      expect(toast.success).toHaveBeenCalledWith('History cleared')
    })

    it('should show error toast on failure', async () => {
      vi.mocked(clearAllHistory).mockRejectedValueOnce(new Error('Permission denied'))
      const store = useHistoryStore()

      await store.clearAll()

      expect(store.error).toBe('Permission denied')
      expect(toast.error).toHaveBeenCalledWith('Permission denied')
    })
  })

  describe('removeRun', () => {
    it('should delete a run and reload current page', async () => {
      vi.mocked(deleteRun).mockResolvedValueOnce(undefined)
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList([makeRun(2)], 1))
      const store = useHistoryStore()

      // Set current page
      store.currentPage = 1

      await store.removeRun(1)

      expect(deleteRun).toHaveBeenCalledWith(1)
      expect(fetchRuns).toHaveBeenCalledWith(20, 20) // page 1 * 20
      expect(toast.success).toHaveBeenCalledWith('Run deleted')
    })

    it('should collapse expanded details if deleting the expanded run', async () => {
      vi.mocked(deleteRun).mockResolvedValueOnce(undefined)
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList([], 0))
      vi.mocked(fetchRunDetails).mockResolvedValueOnce({
        run: makeRun(5),
        results: [makeResult(1)],
      })
      const store = useHistoryStore()

      await store.loadRunDetails(5)
      expect(store.expandedRunId).toBe(5)

      await store.removeRun(5)

      expect(store.expandedRunId).toBeNull()
      expect(store.expandedResults).toEqual([])
    })

    it('should show toast error on failure', async () => {
      vi.mocked(deleteRun).mockRejectedValueOnce(new Error('DB error'))
      const store = useHistoryStore()

      await store.removeRun(1)

      expect(toast.error).toHaveBeenCalledWith('DB error')
    })
  })

  describe('pagination', () => {
    it('hasNextPage should return true when more pages exist', async () => {
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList([], 50))
      const store = useHistoryStore()

      await store.loadRuns(0)

      expect(store.hasNextPage()).toBe(true)
    })

    it('hasNextPage should return false on last page', async () => {
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList([], 20))
      const store = useHistoryStore()

      await store.loadRuns(0)

      expect(store.hasNextPage()).toBe(false)
    })

    it('hasPrevPage should return false on first page', () => {
      const store = useHistoryStore()
      expect(store.hasPrevPage()).toBe(false)
    })

    it('hasPrevPage should return true on page > 0', async () => {
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList([], 50))
      const store = useHistoryStore()

      await store.loadRuns(1)

      expect(store.hasPrevPage()).toBe(true)
    })

    it('nextPage should advance to the next page', async () => {
      vi.mocked(fetchRuns)
        .mockResolvedValueOnce(makeHistoryList([], 50))
        .mockResolvedValueOnce(makeHistoryList([], 50))
      const store = useHistoryStore()

      await store.loadRuns(0)
      await store.nextPage()

      expect(store.currentPage).toBe(1)
    })

    it('nextPage should not go past last page', async () => {
      vi.mocked(fetchRuns).mockResolvedValueOnce(makeHistoryList([], 10))
      const store = useHistoryStore()

      await store.loadRuns(0)
      await store.nextPage()

      // should not have made a second fetch
      expect(fetchRuns).toHaveBeenCalledTimes(1)
      expect(store.currentPage).toBe(0)
    })

    it('prevPage should go back a page', async () => {
      vi.mocked(fetchRuns)
        .mockResolvedValueOnce(makeHistoryList([], 50))
        .mockResolvedValueOnce(makeHistoryList([], 50))
      const store = useHistoryStore()

      await store.loadRuns(2)
      await store.prevPage()

      expect(store.currentPage).toBe(1)
    })

    it('prevPage should not go below 0', () => {
      const store = useHistoryStore()
      store.prevPage()

      expect(fetchRuns).not.toHaveBeenCalled()
      expect(store.currentPage).toBe(0)
    })
  })
})
