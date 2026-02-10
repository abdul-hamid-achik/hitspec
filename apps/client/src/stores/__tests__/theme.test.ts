import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick } from 'vue'
import { useThemeStore } from '@/stores/theme'

// Stable mock for matchMedia — listeners are tracked so we can fire change events
let mediaMatches = false
const mediaListeners: Array<(e: { matches: boolean }) => void> = []

function createMockMatchMedia() {
  mediaListeners.length = 0
  return vi.fn().mockImplementation((query: string) => ({
    matches: query === '(prefers-color-scheme: dark)' ? mediaMatches : false,
    media: query,
    addEventListener: (_event: string, cb: (e: { matches: boolean }) => void) => {
      mediaListeners.push(cb)
    },
    removeEventListener: vi.fn(),
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

describe('Theme Store', () => {
  let storageMap: Record<string, string>

  beforeEach(() => {
    // Reset DOM state
    document.documentElement.classList.remove('dark', 'light')
    document.documentElement.removeAttribute('data-theme')

    // Mock localStorage
    storageMap = {}
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => storageMap[key] ?? null),
      setItem: vi.fn((key: string, value: string) => {
        storageMap[key] = value
      }),
      removeItem: vi.fn((key: string) => {
        delete storageMap[key]
      }),
      clear: vi.fn(),
      length: 0,
      key: vi.fn(),
    })

    // Mock matchMedia
    mediaMatches = false
    vi.stubGlobal('matchMedia', createMockMatchMedia())

    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('default mode', () => {
    it('should default to dark when localStorage has no value', () => {
      const store = useThemeStore()
      expect(store.mode).toBe('dark')
    })

    it('should read persisted mode from localStorage', () => {
      storageMap['hitspec-theme'] = 'light'
      // Need a fresh pinia so the store re-initialises
      setActivePinia(createPinia())
      const store = useThemeStore()
      expect(store.mode).toBe('light')
    })
  })

  describe('setMode', () => {
    it('should update mode and persist to localStorage', () => {
      const store = useThemeStore()

      store.setMode('light')

      expect(store.mode).toBe('light')
      expect(localStorage.setItem).toHaveBeenCalledWith('hitspec-theme', 'light')
    })

    it('should persist system mode to localStorage', () => {
      const store = useThemeStore()

      store.setMode('system')

      expect(store.mode).toBe('system')
      expect(localStorage.setItem).toHaveBeenCalledWith('hitspec-theme', 'system')
    })
  })

  describe('toggle', () => {
    it('should cycle dark -> light -> system -> dark', () => {
      const store = useThemeStore()

      // starts at dark
      expect(store.mode).toBe('dark')

      store.toggle()
      expect(store.mode).toBe('light')

      store.toggle()
      expect(store.mode).toBe('system')

      store.toggle()
      expect(store.mode).toBe('dark')
    })

    it('should persist each toggle step to localStorage', () => {
      const store = useThemeStore()

      store.toggle() // dark -> light
      expect(localStorage.setItem).toHaveBeenCalledWith('hitspec-theme', 'light')

      store.toggle() // light -> system
      expect(localStorage.setItem).toHaveBeenCalledWith('hitspec-theme', 'system')

      store.toggle() // system -> dark
      expect(localStorage.setItem).toHaveBeenCalledWith('hitspec-theme', 'dark')
    })
  })

  describe('resolved', () => {
    it('should return dark when mode is dark', () => {
      const store = useThemeStore()
      expect(store.resolved).toBe('dark')
    })

    it('should return light when mode is light', () => {
      const store = useThemeStore()
      store.setMode('light')
      expect(store.resolved).toBe('light')
    })

    it('should return system preference when mode is system (prefers dark)', () => {
      mediaMatches = true
      vi.stubGlobal('matchMedia', createMockMatchMedia())
      setActivePinia(createPinia())

      const store = useThemeStore()
      store.setMode('system')

      expect(store.resolved).toBe('dark')
    })

    it('should return system preference when mode is system (prefers light)', () => {
      mediaMatches = false
      vi.stubGlobal('matchMedia', createMockMatchMedia())
      setActivePinia(createPinia())

      const store = useThemeStore()
      store.setMode('system')

      expect(store.resolved).toBe('light')
    })
  })

  describe('watchEffect — DOM sync', () => {
    it('should add dark class to documentElement when resolved is dark', async () => {
      useThemeStore()
      await nextTick()
      expect(document.documentElement.classList.contains('dark')).toBe(true)
      expect(document.documentElement.classList.contains('light')).toBe(false)
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    })

    it('should add light class to documentElement when resolved is light', async () => {
      const store = useThemeStore()
      store.setMode('light')
      await nextTick()

      expect(document.documentElement.classList.contains('light')).toBe(true)
      expect(document.documentElement.classList.contains('dark')).toBe(false)
      expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    })

    it('should switch classes when toggling', async () => {
      const store = useThemeStore()
      await nextTick()
      expect(document.documentElement.classList.contains('dark')).toBe(true)

      store.setMode('light')
      await nextTick()
      expect(document.documentElement.classList.contains('light')).toBe(true)
      expect(document.documentElement.classList.contains('dark')).toBe(false)
    })
  })
})
