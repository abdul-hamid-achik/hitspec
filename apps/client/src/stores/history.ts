import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { HistoryEntry } from '@/types/api'
import { getHistory, clearHistory as apiClearHistory } from '@/api/endpoints/history'

export const useHistoryStore = defineStore('history', () => {
  const entries = ref<HistoryEntry[]>([])
  const loading = ref(false)

  async function loadHistory() {
    loading.value = true
    try {
      entries.value = await getHistory()
    } finally {
      loading.value = false
    }
  }

  function addEntry(entry: HistoryEntry) {
    entries.value.unshift(entry)
    if (entries.value.length > 100) {
      entries.value = entries.value.slice(0, 100)
    }
  }

  async function clearAll() {
    await apiClearHistory()
    entries.value = []
  }

  return { entries, loading, loadHistory, addEntry, clearAll }
})
