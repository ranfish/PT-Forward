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

export interface MetadataDetail {
  info_hash: string
  site_name: string
  torrent_id: string
  title: string
  subtitle: string
  source_category: string
  standard_type: string
  tags: string
  flags: string
  source_description: string
  description: string
  screenshots: string
  poster: string
  mediainfo: string
  source_mediainfo: string
  imdb_url: string
  douban_url: string
  reviewed: boolean
  fetch_source: string
}

export const metadataApi = {
  get(infoHash: string, siteName?: string) {
    const params: Record<string, string> = { info_hash: infoHash }
    if (siteName) params.site_name = siteName
    return client.get<ApiResponse<{ items: MetadataDetail[]; total: number }>>('/metadata', { params })
  },
  update(data: {
    infoHash: string
    siteName?: string
    title?: string
    subtitle?: string
    standardType?: string
    tags?: string
    description?: string
    screenshots?: string
  }) {
    return client.put<ApiResponse<{ success: boolean }>>('/metadata', data)
  },
  updateType(infoHash: string, standardType: string, siteName?: string) {
    return client.put<ApiResponse<{ success: boolean }>>('/metadata/type', {
      infoHash: infoHash,
      standardType: standardType,
      siteName: siteName,
    })
  },
  setReviewed(infoHash: string, reviewed: boolean, siteName?: string) {
    return client.put<ApiResponse<{ success: boolean }>>('/metadata/review', {
      infoHash: infoHash,
      reviewed,
      siteName: siteName,
    })
  },
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
