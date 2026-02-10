import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { Pinia } from 'pinia'
import RequestPanel from '@/components/request/RequestPanel.vue'
import { useRequestStore } from '@/stores/request'
import EmptyState from '@/components/common/EmptyState.vue'
import UrlBar from '@/components/request/UrlBar.vue'
import HeadersEditor from '@/components/request/HeadersEditor.vue'
import BodyEditor from '@/components/request/BodyEditor.vue'
import AssertionBuilder from '@/components/request/AssertionBuilder.vue'
import type { RequestDTO } from '@/types/api'

function makeRequest(overrides: Partial<RequestDTO> = {}): RequestDTO {
  return {
    name: 'Get Users',
    method: 'GET',
    url: 'https://api.example.com/users',
    line: 1,
    headers: [
      { key: 'Content-Type', value: 'application/json', line: 2 },
      { key: 'Authorization', value: 'Bearer token', line: 3 },
    ],
    assertions: [
      { subject: 'status', operator: '==', expected: 200, line: 5 },
    ],
    captures: [
      { name: 'userId', source: 'jsonpath', path: '$.id', line: 6 },
    ],
    ...overrides,
  }
}

describe('RequestPanel', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function mountPanel() {
    return shallowMount(RequestPanel, {
      global: { plugins: [pinia] },
    })
  }

  it('should show empty state when no active request', () => {
    const store = useRequestStore()
    store.activeRequest = null

    const wrapper = mountPanel()

    expect(wrapper.findComponent(EmptyState).exists()).toBe(true)
    expect(wrapper.findComponent(EmptyState).props('title')).toBe('No request selected')
    expect(wrapper.findComponent(UrlBar).exists()).toBe(false)
  })

  it('should render UrlBar when an active request exists', () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest()

    const wrapper = mountPanel()

    expect(wrapper.findComponent(UrlBar).exists()).toBe(true)
    expect(wrapper.findComponent(EmptyState).exists()).toBe(false)
  })

  it('should show description when request has one', () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest({ description: 'Fetches all users' })

    const wrapper = mountPanel()

    expect(wrapper.text()).toContain('Fetches all users')
  })

  it('should not show description paragraph when request has none', () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest({ description: undefined })

    const wrapper = mountPanel()

    // The description p tag should not exist
    const paragraphs = wrapper.findAll('p')
    const descP = paragraphs.find((p) => p.classes().some((c) => c.includes('border-b')))
    expect(descP).toBeUndefined()
  })

  it('should display four tabs: Headers, Body, Assertions, Captures', () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest()

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const tabLabels = tabButtons.map((b) => b.text())

    expect(tabLabels.some((t) => t.includes('Headers'))).toBe(true)
    expect(tabLabels.some((t) => t.includes('Body'))).toBe(true)
    expect(tabLabels.some((t) => t.includes('Assertions'))).toBe(true)
    expect(tabLabels.some((t) => t.includes('Captures'))).toBe(true)
  })

  it('should show header count badge', () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest()

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const headersTab = tabButtons.find((b) => b.text().includes('Headers'))
    expect(headersTab!.text()).toContain('2')
  })

  it('should show assertion count badge', () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest()

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const assertionsTab = tabButtons.find((b) => b.text().includes('Assertions'))
    expect(assertionsTab!.text()).toContain('1')
  })

  it('should show HeadersEditor by default (first tab)', () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest()

    const wrapper = mountPanel()

    expect(wrapper.findComponent(HeadersEditor).exists()).toBe(true)
  })

  it('should switch to Body tab when clicked', async () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest()

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const bodyTab = tabButtons.find((b) => b.text().includes('Body'))
    await bodyTab!.trigger('click')

    expect(wrapper.findComponent(BodyEditor).exists()).toBe(true)
    expect(wrapper.findComponent(HeadersEditor).exists()).toBe(false)
  })

  it('should switch to Assertions tab when clicked', async () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest()

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const assertionsTab = tabButtons.find((b) => b.text().includes('Assertions'))
    await assertionsTab!.trigger('click')

    expect(wrapper.findComponent(AssertionBuilder).exists()).toBe(true)
    expect(wrapper.findComponent(HeadersEditor).exists()).toBe(false)
  })

  it('should show captures content when Captures tab is clicked', async () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest()

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const capturesTab = tabButtons.find((b) => b.text().includes('Captures'))
    await capturesTab!.trigger('click')

    expect(wrapper.text()).toContain('userId')
    expect(wrapper.text()).toContain('jsonpath')
    expect(wrapper.text()).toContain('$.id')
  })

  it('should show "No captures defined" when no captures exist', async () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest({ captures: [] })

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const capturesTab = tabButtons.find((b) => b.text().includes('Captures'))
    await capturesTab!.trigger('click')

    expect(wrapper.text()).toContain('No captures defined')
  })

  it('should show 0 for header count when no headers', () => {
    const store = useRequestStore()
    store.activeRequest = makeRequest({ headers: undefined })

    const wrapper = mountPanel()

    const tabButtons = wrapper.findAll('button')
    const headersTab = tabButtons.find((b) => b.text().includes('Headers'))
    expect(headersTab!.text()).toContain('0')
  })
})
