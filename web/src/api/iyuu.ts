import client from './client'
import type { ApiResponse, IYUUConfig, IYUUCoverageStatus } from './types'

export const iyuuApi = {
  getConfig() {
    return client.get<ApiResponse<IYUUConfig>>('/iyuu/config')
  },
  saveConfig(data: Partial<Omit<IYUUConfig, 'id'>>) {
    return client.put<ApiResponse<IYUUConfig>>('/iyuu/config', data)
  },
  listSites() {
    return client.get<ApiResponse<{ items: Array<{ IYUUSid: number; SiteName: string; SiteDomain: string; Enabled: boolean }>; total: number }>>('/iyuu/sites')
  },
  syncSites() {
    return client.post<ApiResponse<{ synced: number }>>('/iyuu/sites')
  },
  query(data: { infoHashes: string[] }) {
    return client.post<ApiResponse<{ results: Array<{ source_info_hash: string; targets: Array<{ sid: number; torrent_id: number; info_hash: string; group: number }> }>; total: number }>>('/iyuu/query', data)
  },
  test() {
    return client.post<ApiResponse<{ success: boolean }>>('/iyuu/test')
  },
  status() {
    return client.get<ApiResponse<{ available: boolean; domains: string[] }>>('/iyuu/status')
  },
  // §59.535 刀3: 覆盖定时任务——手动触发+状态
  coverageRun() {
    return client.post<ApiResponse<{ message: string }>>('/iyuu/coverage/run')
  },
  coverageStatus() {
    return client.get<ApiResponse<IYUUCoverageStatus>>('/iyuu/coverage/status')
  },
  supportedTargets() {
    return client.get<ApiResponse<{ sites: Array<{ site_id: number; name: string; domain: string; iyuu_sid: number }>; total: number }>>('/iyuu/supported-targets')
  },
}
