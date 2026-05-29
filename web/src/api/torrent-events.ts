import client from './client'
import type { ApiResponse, ApiResponsePaginated } from './types'

export const torrentEventsApi = {
  list(params?: { site?: string }) {
    return client.get<ApiResponsePaginated<unknown>>('/torrent-events', { params })
  },
  get(id: number) {
    return client.get<ApiResponse<unknown>>(`/torrent-events/${id}`)
  },
  cleanup() {
    return client.post<ApiResponse<{ deleted: number }>>('/torrent-events/cleanup')
  },
}
