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
    // §59.498 附：返回 model.CookieCloudSyncHistory 形态（snake_case JSON tag）
    return client.post<ApiResponse<{ synced_sites: number; skipped_sites: number }>>('/cookiecloud/sync')
  },
  listHistory(params?: { page?: number; size?: number }) {
    return client.get<ApiResponsePaginated<unknown>>('/cookiecloud/history', { params })
  },
  test() {
    return client.post<ApiResponse<{ success: boolean }>>('/cookiecloud/test')
  },
}
