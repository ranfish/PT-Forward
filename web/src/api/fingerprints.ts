import client from './client'
import type { ApiResponse, ApiResponsePaginated } from './types'

export const fingerprintsApi = {
  list(page = 1, size = 20) {
    return client.get<ApiResponsePaginated<unknown>>('/fingerprints', { params: { page, size } })
  },
  get(id: number) {
    return client.get<ApiResponse<unknown>>(`/fingerprints/${id}`)
  },
  search(params: { infoHash?: string; piecesHash?: string }) {
    return client.get<ApiResponse<{ items: unknown[]; total: number }>>('/fingerprints/search', { params })
  },
  delete(id: number) {
    return client.delete<ApiResponse<void>>(`/fingerprints/${id}`)
  },
  deleteCache() {
    return client.delete<ApiResponse<{ deleted: number }>>('/fingerprints/cache')
  },
}
