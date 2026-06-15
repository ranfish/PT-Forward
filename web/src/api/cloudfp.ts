import client from './client'
import type { ApiResponse, CloudFPConfig } from './types'

export const cloudFpApi = {
  getConfig() {
    return client.get<ApiResponse<CloudFPConfig>>('/cloud-fp/config')
  },
  saveConfig(data: Partial<CloudFPConfig>) {
    return client.put<ApiResponse<CloudFPConfig>>('/cloud-fp/config', data)
  },
  test() {
    return client.post<ApiResponse<{ success: boolean }>>('/cloud-fp/test')
  },
  status() {
    return client.get<ApiResponse<{ enabled: boolean; breaker_open: boolean; base_url: string }>>('/cloud-fp/status')
  },
}
