import client from './client'
import type { ApiResponse } from './types'

export const exclusionsApi = {
  list() {
    return client.get<ApiResponse<Array<{ target_site: string; source_site: string }>>>('/publish/exclusions')
  },
  create(data: { target_site: string; source_site: string }) {
    return client.post<ApiResponse<void>>('/publish/exclusions', data)
  },
  remove(data: { target_site: string; source_site: string }) {
    return client.delete<ApiResponse<void>>('/publish/exclusions', { data })
  },
}
