import client from './client'
import type { ApiResponse, IYUUConfig } from './types'

export const iyuuApi = {
  getConfig() {
    return client.get<ApiResponse<IYUUConfig>>('/iyuu/config')
  },
  saveConfig(data: Partial<Omit<IYUUConfig, 'id'>>) {
    return client.put<ApiResponse<IYUUConfig>>('/iyuu/config', data)
  },
  listSites() {
    return client.get<ApiResponse<Array<{ name: string; url: string }>>>('/iyuu/sites')
  },
  syncSites() {
    return client.post<ApiResponse<{ synced: number }>>('/iyuu/sites')
  },
  query(data: { infoHashes: string[] }) {
    return client.post<ApiResponse<Array<{ infoHash: string; sites: string[] }>>>('/iyuu/query', data)
  },
  test() {
    return client.post<ApiResponse<{ success: boolean }>>('/iyuu/test')
  },
  status() {
    return client.get<ApiResponse<{ available: boolean; domains: string[] }>>('/iyuu/status')
  },
  supportedTargets() {
    return client.get<ApiResponse<{ sites: Array<{ site_id: number; name: string; domain: string; iyuu_sid: number }>; total: number }>>('/iyuu/supported-targets')
  },
}
