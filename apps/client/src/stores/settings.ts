import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { ConfigDTO, SystemInfo } from '@/types/api'
import { getConfig, updateConfig } from '@/api/endpoints/config'
import { getSystemInfo } from '@/api/endpoints/system'

export const useSettingsStore = defineStore('settings', () => {
  const config = ref<ConfigDTO | null>(null)
  const systemInfo = ref<SystemInfo | null>(null)
  const sidebarWidth = ref(260)
  const loading = ref(false)

  async function loadConfig() {
    loading.value = true
    try {
      config.value = await getConfig()
    } finally {
      loading.value = false
    }
  }

  async function loadSystemInfo() {
    systemInfo.value = await getSystemInfo()
  }

  async function saveConfig(updates: Partial<ConfigDTO>) {
    await updateConfig(updates)
    if (config.value) {
      config.value = { ...config.value, ...updates }
    }
  }

  return { config, systemInfo, sidebarWidth, loading, loadConfig, loadSystemInfo, saveConfig }
})
