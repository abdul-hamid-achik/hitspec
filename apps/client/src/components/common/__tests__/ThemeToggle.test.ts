import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { Pinia } from 'pinia'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { useThemeStore } from '@/stores/theme'
import { Sun, Moon, Monitor } from 'lucide-vue-next'

function createMockMatchMedia(prefersDark = false) {
  return vi.fn().mockImplementation((query: string) => ({
    matches: query === '(prefers-color-scheme: dark)' ? prefersDark : false,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

describe('ThemeToggle', () => {
  let pinia: Pinia

  beforeEach(() => {
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
      length: 0,
      key: vi.fn(),
    })
    vi.stubGlobal('matchMedia', createMockMatchMedia(false))

    pinia = createPinia()
    setActivePinia(pinia)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function mountToggle() {
    return shallowMount(ThemeToggle, {
      global: { plugins: [pinia] },
    })
  }

  it('should render Moon icon for dark theme (default)', () => {
    const store = useThemeStore()
    expect(store.mode).toBe('dark')

    const wrapper = mountToggle()

    expect(wrapper.findComponent(Moon).exists()).toBe(true)
    expect(wrapper.findComponent(Sun).exists()).toBe(false)
    expect(wrapper.findComponent(Monitor).exists()).toBe(false)
  })

  it('should render Sun icon for light theme', () => {
    const store = useThemeStore()
    store.setMode('light')

    const wrapper = mountToggle()

    expect(wrapper.findComponent(Sun).exists()).toBe(true)
    expect(wrapper.findComponent(Moon).exists()).toBe(false)
    expect(wrapper.findComponent(Monitor).exists()).toBe(false)
  })

  it('should render Monitor icon for system theme', () => {
    const store = useThemeStore()
    store.setMode('system')

    const wrapper = mountToggle()

    expect(wrapper.findComponent(Monitor).exists()).toBe(true)
    expect(wrapper.findComponent(Sun).exists()).toBe(false)
    expect(wrapper.findComponent(Moon).exists()).toBe(false)
  })

  it('should call toggle when clicked', async () => {
    const store = useThemeStore()
    const toggleSpy = vi.spyOn(store, 'toggle')

    const wrapper = mountToggle()
    await wrapper.find('button').trigger('click')

    expect(toggleSpy).toHaveBeenCalledOnce()
  })

  it('should have correct aria-label reflecting current mode', () => {
    useThemeStore()

    const wrapper = mountToggle()

    expect(wrapper.find('button').attributes('aria-label')).toBe(
      'Theme: dark. Click to switch.',
    )
  })
})
