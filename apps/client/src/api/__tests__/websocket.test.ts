import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { WebSocketClient } from '@/api/websocket'
import type { WSMessage } from '@/types/api'

class MockWebSocket {
  static OPEN = 1
  static CLOSED = 3

  readyState = MockWebSocket.OPEN
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  sent: string[] = []

  send(data: string) {
    this.sent.push(data)
  }

  close(_code?: number, _reason?: string) {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.()
  }

  simulateOpen() {
    this.onopen?.()
  }

  simulateMessage(data: WSMessage) {
    this.onmessage?.({ data: JSON.stringify(data) })
  }

  simulateError() {
    this.onerror?.()
  }
}

describe('WebSocketClient', () => {
  let client: WebSocketClient
  let mockWs: MockWebSocket

  beforeEach(() => {
    vi.useFakeTimers()
    mockWs = new MockWebSocket()
    const MockCtor = vi.fn(() => mockWs) as unknown as typeof WebSocket
    MockCtor.OPEN = 1
    MockCtor.CLOSED = 3
    MockCtor.CLOSING = 2
    MockCtor.CONNECTING = 0
    vi.stubGlobal('WebSocket', MockCtor)
    client = new WebSocketClient()
  })

  afterEach(() => {
    client.disconnect()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('should start disconnected', () => {
    expect(client.connected).toBe(false)
  })

  it('should connect and set connected state', () => {
    client.connect()
    mockWs.simulateOpen()
    expect(client.connected).toBe(true)
  })

  it('should dispatch messages to type-specific listeners', () => {
    const handler = vi.fn()
    client.on('file_changed', handler)
    client.connect()
    mockWs.simulateOpen()

    const msg: WSMessage = { type: 'file_changed', payload: { path: 'test.http' }, timestamp: new Date().toISOString() }
    mockWs.simulateMessage(msg)

    expect(handler).toHaveBeenCalledWith(msg)
  })

  it('should dispatch to wildcard listeners', () => {
    const handler = vi.fn()
    client.on('*', handler)
    client.connect()
    mockWs.simulateOpen()

    const msg: WSMessage = { type: 'execution_complete', payload: {}, timestamp: new Date().toISOString() }
    mockWs.simulateMessage(msg)

    expect(handler).toHaveBeenCalledWith(msg)
  })

  it('should return unsubscribe function', () => {
    const handler = vi.fn()
    const unsub = client.on('file_changed', handler)
    client.connect()
    mockWs.simulateOpen()

    unsub()

    const msg: WSMessage = { type: 'file_changed', payload: {}, timestamp: new Date().toISOString() }
    mockWs.simulateMessage(msg)

    expect(handler).not.toHaveBeenCalled()
  })

  it('should send messages when connected', () => {
    client.connect()
    mockWs.simulateOpen()

    client.send('ping')

    expect(mockWs.sent).toHaveLength(1)
    expect(JSON.parse(mockWs.sent[0])).toEqual({ type: 'ping', payload: undefined })
  })

  it('should set connected to false on disconnect', () => {
    client.connect()
    mockWs.simulateOpen()
    expect(client.connected).toBe(true)

    client.disconnect()
    expect(client.connected).toBe(false)
  })
})
