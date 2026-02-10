import { ref, watch } from 'vue'
import { defineStore } from 'pinia'
import type { HistoryRun, HistoryResult } from '@/types/api'
import { fetchRuns, fetchRunDetails, clearAllHistory, deleteRun as apiDeleteRun } from '@/api/endpoints/history'
import { useRequestStore } from '@/stores/request'
import { toast } from 'vue-sonner'

const PAGE_SIZE = 20

export const useHistoryStore = defineStore('history', () => {
  const runs = ref<HistoryRun[]>([])
  const totalRuns = ref(0)
  const currentPage = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Expanded run details: map of runId -> results
  const expandedRunId = ref<number | null>(null)
  const expandedResults = ref<HistoryResult[]>([])
  const loadingDetails = ref(false)

  async function loadRuns(page = 0) {
    loading.value = true
    error.value = null
    try {
      const data = await fetchRuns(PAGE_SIZE, page * PAGE_SIZE)
      runs.value = data.runs
      totalRuns.value = data.total
      currentPage.value = page
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load history'
    } finally {
      loading.value = false
    }
  }

  async function loadRunDetails(id: number) {
    if (expandedRunId.value === id) {
      // Collapse if already expanded
      expandedRunId.value = null
      expandedResults.value = []
      return
    }
    loadingDetails.value = true
    try {
      const data = await fetchRunDetails(id)
      expandedRunId.value = id
      expandedResults.value = data.results
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to load run details')
    } finally {
      loadingDetails.value = false
    }
  }

  async function clearAll() {
    try {
      await clearAllHistory()
      runs.value = []
      totalRuns.value = 0
      currentPage.value = 0
      expandedRunId.value = null
      expandedResults.value = []
      toast.success('History cleared')
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to clear history'
      error.value = msg
      toast.error(msg)
    }
  }

  async function removeRun(id: number) {
    try {
      await apiDeleteRun(id)
      if (expandedRunId.value === id) {
        expandedRunId.value = null
        expandedResults.value = []
      }
      await loadRuns(currentPage.value)
      toast.success('Run deleted')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete run')
    }
  }

  const hasNextPage = () => (currentPage.value + 1) * PAGE_SIZE < totalRuns.value
  const hasPrevPage = () => currentPage.value > 0

  function nextPage() {
    if (hasNextPage()) loadRuns(currentPage.value + 1)
  }

  function prevPage() {
    if (hasPrevPage()) loadRuns(currentPage.value - 1)
  }

  // Auto-reload after execution completes
  const requestStore = useRequestStore()
  watch(
    () => requestStore.isExecuting,
    (newVal, oldVal) => {
      if (oldVal === true && newVal === false) {
        loadRuns(0)
      }
    },
  )

  return {
    runs,
    totalRuns,
    currentPage,
    loading,
    error,
    expandedRunId,
    expandedResults,
    loadingDetails,
    loadRuns,
    loadRunDetails,
    clearAll,
    removeRun,
    hasNextPage,
    hasPrevPage,
    nextPage,
    prevPage,
  }
})
