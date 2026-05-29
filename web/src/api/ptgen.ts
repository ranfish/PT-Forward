import client from './client'
import type { ApiResponse } from './types'

export const ptgenApi = {
  query(data: { query: string }) {
    return client.post<ApiResponse<Record<string, unknown>>>('/ptgen/query', data)
  },
  listCache(params?: { page?: number; size?: number; keyword?: string }) {
    return client.get<ApiResponse<Record<string, unknown>[]>>('/ptgen/cache', { params })
  },
  cleanCache(retainDays?: number) {
    return client.delete<ApiResponse<{ deleted: number }>>('/ptgen/cache', { params: { retainDays } })
  },
}
