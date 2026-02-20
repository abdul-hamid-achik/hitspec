import { ref, watch } from 'vue'
import { defineStore } from 'pinia'
import type { ConfigDTO, SystemInfo } from '@/types/api'
import { getConfig, updateConfig } from '@/api/endpoints/config'
import { getSystemInfo } from '@/api/endpoints/system'
import { toast } from 'vue-sonner'

export const useSettingsStore = defineStore('settings', () => {
  const config = ref<ConfigDTO | null>(null)
  const systemInfo = ref<SystemInfo | null>(null)
  const sidebarWidth = ref(260)
  const workspaceSplitRatio = ref(
    parseFloat(localStorage.getItem('hitspec:workspaceSplitRatio') ?? '0.5'),
  )
  watch(workspaceSplitRatio, (v) => localStorage.setItem('hitspec:workspaceSplitRatio', String(v)))
  const loading = ref(false)
  const loaded = ref(false)
  const error = ref<string | null>(null)

  async function loadConfig(force = false) {
    if (loaded.value && !force) return
    loading.value = true
    error.value = null
    try {
      config.value = await getConfig()
      loaded.value = true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load config'
    } finally {
      loading.value = false
    }
  }

  async function loadSystemInfo() {
    try {
      systemInfo.value = await getSystemInfo()
    } catch {
      // System info is non-critical; silently ignore
    }
  }

  async function saveConfig(updates: Partial<ConfigDTO>) {
    const prev = config.value ? JSON.parse(JSON.stringify(config.value)) as ConfigDTO : null
    if (config.value) {
      config.value = { ...config.value, ...updates }
    }
    try {
      await updateConfig(updates)
      toast.success('Settings saved')
    } catch (e) {
      config.value = prev
      toast.error(e instanceof Error ? e.message : 'Failed to save settings')
    }
  }

  return { config, systemInfo, sidebarWidth, workspaceSplitRatio, loading, error, loadConfig, loadSystemInfo, saveConfig }
})
