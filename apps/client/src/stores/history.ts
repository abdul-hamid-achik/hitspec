import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { HistoryEntry } from '@/types/api'
import { getHistory, clearHistory as apiClearHistory } from '@/api/endpoints/history'
import { toast } from 'vue-sonner'

export const useHistoryStore = defineStore('history', () => {
  const entries = ref<HistoryEntry[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function loadHistory() {
    loading.value = true
    error.value = null
    try {
      entries.value = await getHistory()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load history'
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
    try {
      await apiClearHistory()
      entries.value = []
      toast.success('History cleared')
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to clear history'
      error.value = msg
      toast.error(msg)
    }
  }

  return { entries, loading, error, loadHistory, addEntry, clearAll }
})
