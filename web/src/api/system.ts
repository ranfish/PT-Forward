import client from './client'
import type { ApiResponse, PaginatedData } from './types'

export interface AuditLog {
  id: number
  created_at: string
  actor: string
  module: string
  action: string
  target_type: string
  target_id: string
  detail: string
  result: string
}

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
  listAuditLogs(params?: Record<string, unknown>) {
    return client.get<ApiResponse<PaginatedData<AuditLog>>>('/system/audit-logs', { params })
  },
}
