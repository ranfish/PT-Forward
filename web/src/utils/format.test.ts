import { describe, it, expect } from 'vitest'
import { formatBytes, formatSpeed, formatTime, formatDurationNs, formatDurationSec } from '@/utils/format'

describe('formatBytes', () => {
  it('returns - for undefined', () => {
    expect(formatBytes(undefined)).toBe('-')
  })

  it('returns 0 B for 0', () => {
    expect(formatBytes(0)).toBe('0 B')
  })

  it('formats bytes', () => {
    expect(formatBytes(512)).toBe('512 B')
  })

  it('formats kilobytes', () => {
    expect(formatBytes(1024)).toBe('1.0 KB')
  })

  it('formats megabytes', () => {
    expect(formatBytes(1048576)).toBe('1.0 MB')
  })

  it('formats gigabytes', () => {
    expect(formatBytes(1073741824)).toBe('1.00 GB')
  })

  it('formats terabytes', () => {
    expect(formatBytes(1099511627776)).toBe('1.00 TB')
  })

  it('formats large terabytes', () => {
    expect(formatBytes(5 * 1099511627776)).toBe('5.00 TB')
  })
})

describe('formatSpeed', () => {
  it('returns 0 B/s for undefined', () => {
    expect(formatSpeed(undefined)).toBe('0 B/s')
  })

  it('returns 0 B/s for 0', () => {
    expect(formatSpeed(0)).toBe('0 B/s')
  })

  it('formats bytes per second', () => {
    expect(formatSpeed(512)).toBe('512.0 B/s')
  })

  it('formats kilobytes per second', () => {
    expect(formatSpeed(1024)).toBe('1.0 KB/s')
  })

  it('formats megabytes per second', () => {
    expect(formatSpeed(1048576)).toBe('1.0 MB/s')
  })

  it('formats gigabytes per second', () => {
    expect(formatSpeed(1073741824)).toBe('1.0 GB/s')
  })
})

describe('formatTime', () => {
  it('returns - for undefined', () => {
    expect(formatTime(undefined)).toBe('-')
  })

  it('returns - for empty string', () => {
    expect(formatTime('')).toBe('-')
  })

  it('formats ISO string', () => {
    const result = formatTime('2025-01-15T10:30:00Z')
    expect(result).not.toBe('-')
    expect(typeof result).toBe('string')
  })
})

describe('formatDurationNs', () => {
  it('returns - for undefined', () => {
    expect(formatDurationNs(undefined)).toBe('-')
  })

  it('returns - for 0', () => {
    expect(formatDurationNs(0)).toBe('-')
  })

  it('formats milliseconds', () => {
    expect(formatDurationNs(500_000_000)).toBe('500ms')
  })

  it('formats seconds', () => {
    expect(formatDurationNs(2_500_000_000)).toBe('2.5s')
  })
})

describe('formatDurationSec', () => {
  it('returns - for undefined', () => {
    expect(formatDurationSec(undefined)).toBe('-')
  })

  it('returns - for 0', () => {
    expect(formatDurationSec(0)).toBe('-')
  })

  it('formats minutes only', () => {
    expect(formatDurationSec(300)).toBe('5m')
  })

  it('formats hours and minutes', () => {
    expect(formatDurationSec(3661)).toBe('1h 1m')
  })

  it('formats zero minutes with hours', () => {
    expect(formatDurationSec(3600)).toBe('1h 0m')
  })
})
