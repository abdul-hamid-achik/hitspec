import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { EnvironmentDTO } from '@/types/api'
import { getEnvironments, selectEnvironment } from '@/api/endpoints/environments'
import { ws } from '@/api/websocket'

export const useEnvironmentStore = defineStore('environment', () => {
  const environments = ref<EnvironmentDTO[]>([])
  const activeEnvName = ref<string>('')
  const loading = ref(false)
  const loaded = ref(false)
  const error = ref<string | null>(null)

  const activeEnv = computed(() => {
    return environments.value.find((e) => e.name === activeEnvName.value) ?? null
  })

  const envNames = computed(() => environments.value.map((e) => e.name))

  async function loadEnvironments(force = false) {
    if (loaded.value && !force) return
    loading.value = true
    error.value = null
    try {
      const envs = await getEnvironments()
      environments.value = envs
      loaded.value = true
      // The Go server does not send isActive; the workspace endpoint provides the active environment name.
      // If the store's activeEnvName was already set (from workspace), keep it.
      if (!activeEnvName.value && envs.length > 0) {
        activeEnvName.value = envs[0].name
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load environments'
    } finally {
      loading.value = false
    }
  }

  async function selectEnv(name: string) {
    const prev = activeEnvName.value
    activeEnvName.value = name
    try {
      await selectEnvironment(name)
    } catch {
      activeEnvName.value = prev
    }
  }

  let initialized = false

  function init() {
    if (initialized) return
    initialized = true
    ws.on('environment_changed', (msg) => {
      const payload = msg.payload
      if (payload && typeof payload === 'object' && 'name' in payload && typeof (payload as Record<string, unknown>).name === 'string') {
        activeEnvName.value = (payload as Record<string, unknown>).name as string
      }
    })
  }

  return { environments, activeEnvName, activeEnv, envNames, loading, error, loadEnvironments, selectEnv, init }
})
