import client from './client'
import type { ApiResponse, SettingsRestoreRequest } from './types'

export const settingsApi = {
  get(prefix?: string) {
    const params: Record<string, string> = {}
    if (prefix) params.prefix = prefix
    return client.get<ApiResponse<Record<string, string>>>('/settings', { params })
  },
  update(key: string, data: { value: string }) {
    return client.put<ApiResponse<void>>(`/settings/${key}`, data)
  },
  remove(key: string) {
    return client.delete<ApiResponse<void>>(`/settings/${key}`)
  },
  backup() {
    return client.get<ApiResponse<Record<string, string>>>('/settings/backup')
  },
  restore(data: SettingsRestoreRequest) {
    return client.post<ApiResponse<void>>('/settings/restore', data)
  },
}
