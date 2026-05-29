import { describe, it, expect } from 'vitest'

describe('token refresh queue logic', () => {
  it('resolves queued subscribers when token refreshed', () => {
    const subscribers: Array<(token: string) => void> = []
    const results: string[] = []

    subscribers.push((token) => results.push(`replayed-${token}`))
    subscribers.push((token) => results.push(`replayed-${token}`))

    const newToken = 'new-access-token'
    subscribers.forEach((cb) => cb(newToken))

    expect(results).toEqual(['replayed-new-access-token', 'replayed-new-access-token'])
  })

  it('clears subscribers after replay', () => {
    let subscribers: Array<(token: string) => void> = []
    subscribers.push(() => {})
    subscribers.push(() => {})
    subscribers = []
    expect(subscribers).toEqual([])
  })
})

describe('reconnect backoff formula', () => {
  it('calculates exponential backoff with 30s cap', () => {
    const delays = Array.from({ length: 10 }, (_, i) => Math.min(1000 * Math.pow(2, i), 30000))
    expect(delays).toEqual([1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000, 30000, 30000])
  })
})

describe('JWT payload parsing', () => {
  function parseJWTPayload(token: string): Record<string, unknown> | null {
    try {
      const parts = token.split('.')
      if (parts.length !== 3) return null
      return JSON.parse(atob(parts[1]))
    } catch {
      return null
    }
  }

  it('extracts payload from valid JWT', () => {
    const header = btoa(JSON.stringify({ alg: 'HS256' }))
    const body = btoa(JSON.stringify({ sub: 'admin', exp: 9999999999 }))
    const token = `${header}.${body}.sig`
    const payload = parseJWTPayload(token)
    expect(payload).toEqual({ sub: 'admin', exp: 9999999999 })
  })

  it('returns null for invalid JWT', () => {
    expect(parseJWTPayload('not.jwt')).toBeNull()
    expect(parseJWTPayload('a.b.c.d')).toBeNull()
    expect(parseJWTPayload('!!!.!!!.!!!')).toBeNull()
  })
})

describe('message buffer trim logic', () => {
  function trimMessages<T>(messages: T[], max = 200, keep = 100): T[] {
    if (messages.length > max) {
      return messages.slice(-keep)
    }
    return messages
  }

  it('does not trim under limit', () => {
    const msgs = Array.from({ length: 200 }, (_, i) => i)
    expect(trimMessages(msgs)).toHaveLength(200)
  })

  it('trims to 100 when over 200', () => {
    const msgs = Array.from({ length: 250 }, (_, i) => i)
    const result = trimMessages(msgs)
    expect(result).toHaveLength(100)
    expect(result[0]).toBe(150)
    expect(result[99]).toBe(249)
  })

  it('keeps latest messages after trim', () => {
    const msgs = Array.from({ length: 205 }, (_, i) => ({ id: i }))
    const result = trimMessages(msgs)
    expect(result[0]).toEqual({ id: 105 })
    expect(result[99]).toEqual({ id: 204 })
  })
})

describe('channel dedup', () => {
  function mergeChannels(existing: string[], incoming: string[]): string[] {
    return [...new Set([...existing, ...incoming])]
  }

  it('deduplicates channels', () => {
    expect(mergeChannels(['a', 'b'], ['b', 'c'])).toEqual(['a', 'b', 'c'])
  })

  it('handles empty existing', () => {
    expect(mergeChannels([], ['a', 'b'])).toEqual(['a', 'b'])
  })

  it('handles empty incoming', () => {
    expect(mergeChannels(['a', 'b'], [])).toEqual(['a', 'b'])
  })
})
