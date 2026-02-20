import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { HistoryResultWithRun } from '@/types/api'
import { fetchResultsByRequest } from '@/api/endpoints/history'

export const useComparisonStore = defineStore('comparison', () => {
  const requestHistory = ref<HistoryResultWithRun[]>([])
  const selectedA = ref<HistoryResultWithRun | null>(null)
  const selectedB = ref<HistoryResultWithRun | null>(null)
  const loading = ref(false)
  const currentRequestName = ref('')
  const currentFilePath = ref('')

  const hasComparison = computed(() => selectedA.value !== null && selectedB.value !== null)

  async function loadHistory(name: string, file: string) {
    if (name === currentRequestName.value && file === currentFilePath.value && requestHistory.value.length > 0) {
      return
    }
    currentRequestName.value = name
    currentFilePath.value = file
    loading.value = true
    try {
      const data = await fetchResultsByRequest(name, file, 50)
      requestHistory.value = data.results
    } catch {
      requestHistory.value = []
    } finally {
      loading.value = false
    }
  }

  function selectForCompare(result: HistoryResultWithRun) {
    if (selectedA.value?.id === result.id) {
      selectedA.value = null
      return
    }
    if (selectedB.value?.id === result.id) {
      selectedB.value = null
      return
    }
    if (!selectedA.value) {
      selectedA.value = result
    } else if (!selectedB.value) {
      selectedB.value = result
    } else {
      // Replace B
      selectedB.value = result
    }
  }

  function clearSelection() {
    selectedA.value = null
    selectedB.value = null
  }

  function reset() {
    requestHistory.value = []
    selectedA.value = null
    selectedB.value = null
    currentRequestName.value = ''
    currentFilePath.value = ''
    loading.value = false
  }

  return {
    requestHistory,
    selectedA,
    selectedB,
    loading,
    currentRequestName,
    currentFilePath,
    hasComparison,
    loadHistory,
    selectForCompare,
    clearSelection,
    reset,
  }
})
