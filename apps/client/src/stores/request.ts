import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { RequestDTO, RunResult, ExecuteResult } from '@/types/api'
import { executeRequest as apiExecute, executeFile as executeFileApi } from '@/api/endpoints/execute'

export const useRequestStore = defineStore('request', () => {
  const activeRequest = ref<RequestDTO | null>(null)
  const activeRequestIndex = ref(0)
  const lastResult = ref<RunResult | null>(null)
  const lastRunResult = ref<ExecuteResult | null>(null)
  const isExecuting = ref(false)
  const error = ref<string | null>(null)

  const hasPassed = computed(() => lastResult.value?.passed ?? null)

  async function execute(filePath: string, requestName?: string, environment?: string) {
    isExecuting.value = true
    error.value = null
    lastResult.value = null
    try {
      const result = await apiExecute({ file: filePath, requestName, environment })
      if (result.results.length > 0) {
        lastResult.value = result.results[0]
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      isExecuting.value = false
    }
  }

  async function runFile(filePath: string, environment?: string) {
    isExecuting.value = true
    error.value = null
    lastResult.value = null
    try {
      const result = await executeFileApi(filePath, environment)
      // Store the full run result for the response panel
      lastRunResult.value = result
      if (result.results.length > 0) {
        lastResult.value = result.results[0]
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      isExecuting.value = false
    }
  }

  function setActiveRequest(request: RequestDTO | null, index: number = 0) {
    activeRequest.value = request
    activeRequestIndex.value = index
    lastResult.value = null
    error.value = null
  }

  function clear() {
    activeRequest.value = null
    lastResult.value = null
    lastRunResult.value = null
    error.value = null
  }

  return { activeRequest, activeRequestIndex, lastResult, lastRunResult, isExecuting, error, hasPassed, execute, runFile, setActiveRequest, clear }
})
