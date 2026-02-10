import { ref, computed, watchEffect } from 'vue'
import { defineStore } from 'pinia'

export type ThemeMode = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'hitspec-theme'

function getSystemTheme(): 'light' | 'dark' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>((localStorage.getItem(STORAGE_KEY) as ThemeMode) || 'dark')
  const resolved = computed<'light' | 'dark'>(() =>
    mode.value === 'system' ? getSystemTheme() : mode.value,
  )

  function setMode(newMode: ThemeMode) {
    mode.value = newMode
    localStorage.setItem(STORAGE_KEY, newMode)
  }

  function toggle() {
    const next: ThemeMode = mode.value === 'dark' ? 'light' : mode.value === 'light' ? 'system' : 'dark'
    setMode(next)
  }

  // Sync resolved theme to <html> class and data attribute
  watchEffect(() => {
    const html = document.documentElement
    html.classList.toggle('dark', resolved.value === 'dark')
    html.classList.toggle('light', resolved.value === 'light')
    html.setAttribute('data-theme', resolved.value)
  })

  // Listen for system theme changes when in system mode
  const mql = window.matchMedia('(prefers-color-scheme: dark)')
  mql.addEventListener('change', () => {
    if (mode.value === 'system') {
      // Force reactivity update by re-reading mode
      mode.value = 'system'
    }
  })

  return { mode, resolved, setMode, toggle }
})
