import { ref } from 'vue'
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

  // Monotonic counter to detect stale responses from concurrent executions
  let executionId = 0

  async function execute(filePath: string, requestName?: string, environment?: string) {
    if (isExecuting.value) return
    const thisId = ++executionId
    isExecuting.value = true
    error.value = null
    lastResult.value = null
    lastRunResult.value = null
    try {
      const result = await apiExecute({ file: filePath, requestName, environment })
      if (thisId !== executionId) return // superseded by a newer execution
      // Find the matching result for the specific request
      if (requestName) {
        const match = result.results.find(r => r.name === requestName)
        if (match) {
          lastResult.value = match
        } else if (result.results.length > 0) {
          lastResult.value = result.results[0]
        }
      } else if (result.results.length > 0) {
        lastResult.value = result.results[0]
      }
    } catch (e) {
      if (thisId !== executionId) return
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      if (thisId === executionId) {
        isExecuting.value = false
      }
    }
  }

  async function runFile(filePath: string, environment?: string) {
    if (isExecuting.value) return
    const thisId = ++executionId
    isExecuting.value = true
    error.value = null
    lastResult.value = null
    try {
      const result = await executeFileApi(filePath, environment)
      if (thisId !== executionId) return
      // Store the full run result for the response panel
      lastRunResult.value = result
      if (result.results.length > 0) {
        lastResult.value = result.results[0]
      }
    } catch (e) {
      if (thisId !== executionId) return
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      if (thisId === executionId) {
        isExecuting.value = false
      }
    }
  }

  function setActiveRequest(request: RequestDTO | null, index: number = 0) {
    activeRequest.value = request
    activeRequestIndex.value = index
    // If we have run results, try to find the matching result for this request
    if (lastRunResult.value && request) {
      const match = lastRunResult.value.results.find(r => r.name === request.name)
      if (match) {
        lastResult.value = match
        return
      }
    }
    // Only clear lastResult if switching to a different file context
    // Keep lastRunResult intact so the Results tab persists
    lastResult.value = null
    error.value = null
  }

  return { activeRequest, activeRequestIndex, lastResult, lastRunResult, isExecuting, error, execute, runFile, setActiveRequest }
})
