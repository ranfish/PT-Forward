import client from './client'
import type { ApiResponse } from './types'

export const systemApi = {
  health() {
    return client.get<ApiResponse<{ status: string }>>('/system/health')
  },
  info() {
    return client.get<ApiResponse<{ version: string; buildTime: string; goVersion: string }>>('/system/info')
  },
  listLogs(params?: { level?: string; limit?: number }) {
    return client.get<ApiResponse<{ items: string[]; total: number }>>('/system/logs', { params })
  },
  clearLogs() {
    return client.delete<ApiResponse<void>>('/system/logs')
  },
}
