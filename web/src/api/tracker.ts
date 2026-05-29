import client from './client'
import type { ApiResponse } from './types'

export const trackerApi = {
  listMembers() {
    return client.get<ApiResponse<{ items: unknown[]; total: number }>>('/tracker/members')
  },
  getMember(hash: string) {
    return client.get<ApiResponse<unknown>>(`/tracker/members/${encodeURIComponent(hash)}`)
  },
  getHistory(params?: { site?: string; limit?: number }) {
    return client.get<ApiResponse<{ items: unknown[]; total: number }>>('/tracker/history', { params })
  },
}
