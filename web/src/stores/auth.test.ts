import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/auth', () => ({
  authApi: {
    login: vi.fn().mockResolvedValue({
      data: { data: { accessToken: 'test-access', refreshToken: 'test-refresh' } },
    }),
  },
}))

vi.mock('@/router', () => ({
  default: { push: vi.fn() },
}))

vi.mock('@/stores/websocket', () => ({
  useWebSocketStore: vi.fn(() => ({
    disconnect: vi.fn(),
  })),
}))

describe('useAuthStore', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    setActivePinia(createPinia())
  })

  it('initializes with no token', () => {
    const store = useAuthStore()
    expect(store.accessToken).toBe('')
    expect(store.isLoggedIn).toBe(false)
  })

  it('initializes with existing token from localStorage', () => {
    localStorage.setItem('pt-forward-access-token', 'saved-token')
    setActivePinia(createPinia())
    const store = useAuthStore()
    expect(store.accessToken).toBe('saved-token')
    expect(store.isLoggedIn).toBe(true)
  })

  it('login stores tokens and sets state', async () => {
    const store = useAuthStore()
    await store.login('admin', 'password')
    expect(store.accessToken).toBe('test-access')
    expect(store.isLoggedIn).toBe(true)
    expect(localStorage.getItem('pt-forward-access-token')).toBe('test-access')
    expect(localStorage.getItem('pt-forward-refresh-token')).toBe('test-refresh')
  })

  it('logout clears state and tokens', async () => {
    localStorage.setItem('pt-forward-access-token', 'token')
    setActivePinia(createPinia())
    const store = useAuthStore()
    await store.login('admin', 'password')
    expect(store.isLoggedIn).toBe(true)

    store.logout()
    expect(store.accessToken).toBe('')
    expect(store.isLoggedIn).toBe(false)
    expect(localStorage.getItem('pt-forward-access-token')).toBeNull()
    expect(localStorage.getItem('pt-forward-refresh-token')).toBeNull()
  })

  it('logout navigates to Login', async () => {
    const { default: router } = await import('@/router')
    const store = useAuthStore()
    store.logout()
    expect(router.push).toHaveBeenCalledWith({ name: 'Login' })
  })

  it('logout disconnects websocket', async () => {
    const { useWebSocketStore } = await import('@/stores/websocket')
    const mockDisconnect = vi.fn()
    vi.mocked(useWebSocketStore).mockReturnValue({ disconnect: mockDisconnect } as unknown as ReturnType<typeof useWebSocketStore>)
    const store = useAuthStore()
    store.logout()
    expect(mockDisconnect).toHaveBeenCalled()
  })
})
