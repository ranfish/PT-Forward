import { describe, it, expect } from 'vitest'
import { isTokenValid } from '@/router/index'

function makeToken(payload: object): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const body = btoa(JSON.stringify(payload))
  return `${header}.${body}.signature`
}

describe('isTokenValid', () => {
  it('returns false for null', () => {
    expect(isTokenValid(null)).toBe(false)
  })

  it('returns false for empty string', () => {
    expect(isTokenValid('')).toBe(false)
  })

  it('returns false for non-JWT string', () => {
    expect(isTokenValid('not-a-jwt')).toBe(false)
  })

  it('returns false for token with wrong number of parts', () => {
    expect(isTokenValid('part1.part2')).toBe(false)
    expect(isTokenValid('a.b.c.d')).toBe(false)
  })

  it('returns false for token with invalid base64', () => {
    expect(isTokenValid('header.!!!invalid!!!.sig')).toBe(false)
  })

  it('returns true for token with future exp', () => {
    const futureExp = Math.floor(Date.now() / 1000) + 3600
    expect(isTokenValid(makeToken({ exp: futureExp }))).toBe(true)
  })

  it('returns false for expired token', () => {
    const pastExp = Math.floor(Date.now() / 1000) - 3600
    expect(isTokenValid(makeToken({ exp: pastExp }))).toBe(false)
  })

  it('returns true for token without exp claim', () => {
    expect(isTokenValid(makeToken({ sub: 'admin' }))).toBe(true)
  })

  it('returns false for token with exp = exactly now', () => {
    const now = Math.floor(Date.now() / 1000)
    expect(isTokenValid(makeToken({ exp: now }))).toBe(false)
  })
})
