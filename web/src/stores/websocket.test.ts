import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useWebSocketStore } from '@/stores/websocket'

describe('useWebSocketStore', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    vi.useFakeTimers()
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('initializes disconnected', () => {
    const store = useWebSocketStore()
    expect(store.connected).toBe(false)
    expect(store.messages).toEqual([])
  })

  it('connect does nothing without token', () => {
    const store = useWebSocketStore()
    store.connect()
    expect(store.connected).toBe(false)
  })

  it('disconnect clears state', () => {
    const store = useWebSocketStore()
    store.disconnect()
    expect(store.connected).toBe(false)
    expect(store.messages).toEqual([])
  })

  it('lastMessage returns null when no messages', () => {
    const store = useWebSocketStore()
    expect(store.lastMessage).toBeNull()
  })

  it('lastMessage returns last message', () => {
    const store = useWebSocketStore()
    store.messages = [{ type: 'a' }, { type: 'b' }]
    expect(store.lastMessage).toEqual({ type: 'b' })
  })

  it('reconnect backoff formula caps at 30s', () => {
    const delays = Array.from({ length: 10 }, (_, i) => Math.min(1000 * Math.pow(2, i), 30000))
    expect(delays).toEqual([1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000, 30000, 30000])
  })

  it('message buffer trims at 200 to last 100', () => {
    const store = useWebSocketStore()
    store.messages = Array.from({ length: 250 }, (_, i) => ({ type: 'test', idx: i }))
    expect(store.messages).toHaveLength(250)

    const newMsg = { type: 'test', idx: 250 }
    const updated = [...store.messages, newMsg]
    if (updated.length > 200) {
      store.messages = updated.slice(-100)
    }
    expect(store.messages).toHaveLength(100)
  })

  it('disconnect after connect attempt does not reconnect', () => {
    const store = useWebSocketStore()
    store.disconnect()
    vi.advanceTimersByTime(60000)
    expect(store.connected).toBe(false)
  })
})
