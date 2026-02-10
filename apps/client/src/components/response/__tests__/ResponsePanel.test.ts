import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { Pinia } from 'pinia'
import { nextTick } from 'vue'
import ResponsePanel from '@/components/response/ResponsePanel.vue'
import { useRequestStore } from '@/stores/request'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import ResponseBody from '@/components/response/ResponseBody.vue'
import ResponseHeaders from '@/components/response/ResponseHeaders.vue'
import ResponseAssertions from '@/components/response/ResponseAssertions.vue'
import RunResultsList from '@/components/response/RunResultsList.vue'
import { CheckCircle, XCircle, MinusCircle } from 'lucide-vue-next'
import type { RunResult, ExecuteResult } from '@/types/api'

function makeResult(overrides: Partial<RunResult> = {}): RunResult {
  return {
    name: 'GET /users',
    passed: true,
    duration: 42,
    response: {
      statusCode: 200,
      status: '200 OK',
      headers: { 'content-type': 'application/json' },
      body: '{"ok":true}',
      duration: 42,
      size: 11,
    },
    assertions: [
      { subject: 'status', operator: '==', expected: 200, actual: 200, passed: true },
    ],
    ...overrides,
  }
}

function makeRunResult(overrides: Partial<ExecuteResult> = {}): ExecuteResult {
  return {
    file: 'test.http',
    duration: 100,
    passed: 2,
    failed: 0,
    skipped: 0,
    results: [makeResult(), makeResult({ name: 'POST /users' })],
    ...overrides,
  }
}

describe('ResponsePanel', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function mountPanel() {
    return shallowMount(ResponsePanel, {
      global: { plugins: [pinia] },
    })
  }

  it('should show empty state when no result and not executing', () => {
    const store = useRequestStore()
    store.lastResult = null
    store.lastRunResult = null
    store.isExecuting = false
    store.error = null

    const wrapper = mountPanel()

    expect(wrapper.findComponent(EmptyState).exists()).toBe(true)
    expect(wrapper.findComponent(EmptyState).props('title')).toBe('No response yet')
  })

  it('should show loading spinner when executing without progress', () => {
    const store = useRequestStore()
    store.isExecuting = true
    store.executionProgress = null

    const wrapper = mountPanel()

    expect(wrapper.findComponent(LoadingSpinner).exists()).toBe(true)
    expect(wrapper.findComponent(EmptyState).exists()).toBe(false)
  })

  it('should show progress bar when executing with progress', () => {
    const store = useRequestStore()
    store.isExecuting = true
    store.executionProgress = {
      currentRequest: 'GET /users',
      index: 1,
      total: 3,
      completed: 1,
      results: [{ name: 'GET /health', passed: true, duration: 10 }],
    }

    const wrapper = mountPanel()

    expect(wrapper.findComponent(LoadingSpinner).exists()).toBe(false)
    expect(wrapper.text()).toContain('1/3 requests')
    expect(wrapper.text()).toContain('GET /users')
    // Completed result row
    expect(wrapper.text()).toContain('GET /health')
    expect(wrapper.findComponent(CheckCircle).exists()).toBe(true)
  })

  it('should show status badge and tabs when a result exists', () => {
    const store = useRequestStore()
    store.lastResult = makeResult()

    const wrapper = mountPanel()

    expect(wrapper.findComponent(StatusBadge).exists()).toBe(true)
    expect(wrapper.findComponent(StatusBadge).props('code')).toBe(200)
    // Default tab is body
    expect(wrapper.findComponent(ResponseBody).exists()).toBe(true)
    // Check passed icon
    expect(wrapper.findComponent(CheckCircle).exists()).toBe(true)
    expect(wrapper.text()).toContain('Passed')
  })

  it('should show failed status for a failed result', () => {
    const store = useRequestStore()
    store.lastResult = makeResult({ passed: false })

    const wrapper = mountPanel()

    expect(wrapper.findComponent(XCircle).exists()).toBe(true)
    expect(wrapper.text()).toContain('Failed')
  })

  it('should show skipped status for a skipped result', () => {
    const store = useRequestStore()
    store.lastResult = makeResult({ passed: false, skipped: true, skipReason: 'condition not met' })

    const wrapper = mountPanel()

    expect(wrapper.findComponent(MinusCircle).exists()).toBe(true)
    expect(wrapper.text()).toContain('Skipped')
    expect(wrapper.text()).toContain('condition not met')
  })

  it('should show ERROR badge when result has error but no response', () => {
    const store = useRequestStore()
    store.lastResult = makeResult({
      passed: false,
      error: 'connection refused',
      response: undefined,
    })

    const wrapper = mountPanel()

    expect(wrapper.findComponent(StatusBadge).exists()).toBe(false)
    expect(wrapper.text()).toContain('ERROR')
  })

  it('should switch tabs when tab buttons are clicked', async () => {
    const store = useRequestStore()
    store.lastResult = makeResult()

    const wrapper = mountPanel()

    // Initially shows body tab
    expect(wrapper.findComponent(ResponseBody).exists()).toBe(true)

    // Click headers tab
    const tabButtons = wrapper.findAll('button')
    const headersTab = tabButtons.find((b) => b.text().includes('Headers'))
    expect(headersTab).toBeDefined()
    await headersTab!.trigger('click')

    expect(wrapper.findComponent(ResponseHeaders).exists()).toBe(true)
    expect(wrapper.findComponent(ResponseBody).exists()).toBe(false)

    // Click assertions tab
    const assertionsTab = tabButtons.find((b) => b.text().includes('Assertions'))
    await assertionsTab!.trigger('click')

    expect(wrapper.findComponent(ResponseAssertions).exists()).toBe(true)
    expect(wrapper.findComponent(ResponseHeaders).exists()).toBe(false)
  })

  it('should show assertion count badge', () => {
    const store = useRequestStore()
    store.lastResult = makeResult()

    const wrapper = mountPanel()

    // The assertion count badge shows "1" (one assertion)
    const tabButtons = wrapper.findAll('button')
    const assertionsTab = tabButtons.find((b) => b.text().includes('Assertions'))
    expect(assertionsTab!.text()).toContain('1')
  })

  it('should display duration and size in status bar', () => {
    const store = useRequestStore()
    store.lastResult = makeResult()

    const wrapper = mountPanel()

    expect(wrapper.text()).toContain('42ms')
    // Size: 11 bytes = 0.0KB
    expect(wrapper.text()).toContain('KB')
  })

  it('should show Results tab when lastRunResult has results', () => {
    const store = useRequestStore()
    store.lastRunResult = makeRunResult()
    store.lastResult = makeResult()

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const resultsTab = tabButtons.find((b) => b.text().includes('Results'))
    expect(resultsTab).toBeDefined()
    // Results count badge
    expect(resultsTab!.text()).toContain('2')
  })

  it('should not show Results tab when no run results', () => {
    const store = useRequestStore()
    store.lastRunResult = null
    store.lastResult = makeResult()

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const resultsTab = tabButtons.find((b) => b.text().includes('Results'))
    expect(resultsTab).toBeUndefined()
  })

  it('should auto-switch to Results tab when lastRunResult is set', async () => {
    const store = useRequestStore()
    store.lastResult = makeResult()

    const wrapper = mountPanel()

    // Initially on body tab
    expect(wrapper.findComponent(ResponseBody).exists()).toBe(true)

    // Simulate run completing
    store.lastRunResult = makeRunResult()
    await nextTick()

    expect(wrapper.findComponent(RunResultsList).exists()).toBe(true)
  })

  it('should show error state with dismiss button', async () => {
    const store = useRequestStore()
    store.error = 'Network timeout'

    const wrapper = mountPanel()

    expect(wrapper.text()).toContain('Request Failed')
    expect(wrapper.text()).toContain('Network timeout')

    // Click dismiss
    const dismissBtn = wrapper.findAll('button').find((b) => b.text().includes('Dismiss'))
    expect(dismissBtn).toBeDefined()
    await dismissBtn!.trigger('click')

    expect(store.error).toBeNull()
  })
})
