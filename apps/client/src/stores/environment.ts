import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { EnvironmentDTO } from '@/types/api'
import { getEnvironments, selectEnvironment, updateEnvironment } from '@/api/endpoints/environments'

export const useEnvironmentStore = defineStore('environment', () => {
  const environments = ref<EnvironmentDTO[]>([])
  const activeEnvName = ref<string>('')
  const loading = ref(false)

  const activeEnv = computed(() => {
    return environments.value.find((e) => e.name === activeEnvName.value) ?? null
  })

  const envNames = computed(() => environments.value.map((e) => e.name))

  async function loadEnvironments() {
    loading.value = true
    try {
      const envs = await getEnvironments()
      environments.value = envs
      const active = envs.find((e) => e.isActive)
      if (active) activeEnvName.value = active.name
    } finally {
      loading.value = false
    }
  }

  async function selectEnv(name: string) {
    await selectEnvironment(name)
    activeEnvName.value = name
  }

  async function updateEnv(env: EnvironmentDTO) {
    await updateEnvironment(env)
    const idx = environments.value.findIndex((e) => e.name === env.name)
    if (idx >= 0) environments.value[idx] = env
  }

  return { environments, activeEnvName, activeEnv, envNames, loading, loadEnvironments, selectEnv, updateEnv }
})
