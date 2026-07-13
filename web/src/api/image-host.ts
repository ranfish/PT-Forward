import client from './client'
import type { ApiResponse } from './types'

export interface ImageHostConfig {
  hosts: string[]
  default: string
  strategy: string
  agsvpt_email: string
  agsvpt_configured: boolean
  health: Record<string, string>
}

export interface ImageHostTestResult {
  success: boolean
  url: string
  host: string
}

export const imageHostApi = {
  get() {
    return client.get<ApiResponse<ImageHostConfig>>('/settings/image-host')
  },
  update(data: {
    default?: string
    strategy?: string
    agsvpt_email?: string
    agsvpt_password?: string
  }) {
    return client.put<ApiResponse<void>>('/settings/image-host', data)
  },
  test(host?: string) {
    return client.post<ApiResponse<ImageHostTestResult>>('/settings/image-host/test', { host })
  },
}

export interface PublishLimit {
  id?: number
  site_name: string
  enabled: boolean
  max_count: number
  window_hours: number
  current_count?: number
}

export const publishLimitApi = {
  list() {
    return client.get<ApiResponse<{ items: PublishLimit[]; total: number }>>('/publish/limits')
  },
  get(siteName: string) {
    return client.get<ApiResponse<{ exists: boolean; limit?: PublishLimit }>>(`/publish/limits/${siteName}`)
  },
  upsert(data: PublishLimit) {
    return client.put<ApiResponse<{ success: boolean; limit: PublishLimit }>>('/publish/limits', data)
  },
  delete(siteName: string) {
    return client.delete<ApiResponse<{ success: boolean }>>(`/publish/limits/${siteName}`)
  },
}
