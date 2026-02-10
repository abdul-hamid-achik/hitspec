import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import ErrorBoundary from '@/components/common/ErrorBoundary.vue'

describe('ErrorBoundary', () => {
  const reloadMock = vi.fn()

  beforeEach(() => {
    Object.defineProperty(window, 'location', {
      value: { reload: reloadMock },
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    reloadMock.mockReset()
    vi.restoreAllMocks()
  })

  function mountBoundary() {
    return mount(ErrorBoundary, {
      slots: {
        default: '<div data-testid="slot-content">Hello World</div>',
      },
      global: {
        stubs: {
          TriangleAlert: { template: '<i data-testid="alert-icon" />' },
          RotateCcw: { template: '<i data-testid="rotate-icon" />' },
        },
      },
    })
  }

  /** Simulate an error via the exposed ref (auto-unwrapped by Vue proxy) */
  async function triggerError(wrapper: ReturnType<typeof mountBoundary>, message: string) {
    ;(wrapper.vm as Record<string, unknown>).error = new Error(message)
    await nextTick()
  }

  it('should render slot content when there is no error', () => {
    const wrapper = mountBoundary()

    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Hello World')
    expect(wrapper.find('[data-testid="alert-icon"]').exists()).toBe(false)
  })

  it('should show error UI when an error is captured', async () => {
    const wrapper = mountBoundary()

    await triggerError(wrapper, 'Boom!')

    expect(wrapper.text()).toContain('Something went wrong')
    expect(wrapper.text()).toContain('Boom!')
    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(false)
  })

  it('should display the specific error message', async () => {
    const wrapper = mountBoundary()

    await triggerError(wrapper, 'Custom error message')

    expect(wrapper.text()).toContain('Something went wrong')
    expect(wrapper.text()).toContain('Custom error message')
  })

  it('should clear error and re-show slot when dismiss (Try Again) is clicked', async () => {
    const wrapper = mountBoundary()

    await triggerError(wrapper, 'Oops')
    expect(wrapper.text()).toContain('Something went wrong')

    const dismissButton = wrapper.findAll('button').find((b) => b.text().includes('Try Again'))
    expect(dismissButton).toBeDefined()
    await dismissButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).not.toContain('Something went wrong')
    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(true)
  })

  it('should call window.location.reload when Reload button is clicked', async () => {
    const wrapper = mountBoundary()

    await triggerError(wrapper, 'Fatal')

    const reloadButton = wrapper.findAll('button').find((b) => b.text().includes('Reload'))
    expect(reloadButton).toBeDefined()
    await reloadButton!.trigger('click')

    expect(reloadMock).toHaveBeenCalledOnce()
  })

  it('should display error details in a details/summary element', async () => {
    const wrapper = mountBoundary()

    await triggerError(wrapper, 'Stack trace test')

    const details = wrapper.find('details')
    expect(details.exists()).toBe(true)
    expect(details.find('summary').text()).toContain('Error details')
  })
})
