import client from './client'
import type { ApiResponse, ApiResponsePaginated, CookieCloudConfig } from './types'

export const cookiecloudApi = {
  getConfig() {
    return client.get<ApiResponse<CookieCloudConfig>>('/cookiecloud/config')
  },
  saveConfig(data: Partial<Omit<CookieCloudConfig, 'id'>>) {
    return client.put<ApiResponse<CookieCloudConfig>>('/cookiecloud/config', data)
  },
  sync() {
    return client.post<ApiResponse<{ synced: number }>>('/cookiecloud/sync')
  },
  listHistory(params?: { page?: number; size?: number }) {
    return client.get<ApiResponsePaginated<unknown>>('/cookiecloud/history', { params })
  },
  test() {
    return client.post<ApiResponse<{ success: boolean }>>('/cookiecloud/test')
  },
}
