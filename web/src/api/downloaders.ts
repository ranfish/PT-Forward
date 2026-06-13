import client from './client'
import type { ApiResponse, ApiResponsePaginated, ClientConfig, ClientPublishTarget, TorrentInfo, UpdatePartial, CreateWithoutId } from './types'

export const downloadersApi = {
  list(page = 1, size = 20) {
    return client.get<ApiResponsePaginated<ClientConfig>>('/downloaders', { params: { page, size } })
  },
  listLight(page = 1, size = 200) {
    return client.get<ApiResponsePaginated<ClientConfig>>('/downloaders', { params: { page, size, light: 'true' } })
  },
  get(id: number) {
    return client.get<ApiResponse<ClientConfig>>(`/downloaders/${id}`)
  },
  create(data: UpdatePartial<ClientConfig>) {
    return client.post<ApiResponse<ClientConfig>>('/downloaders', data)
  },
  update(id: number, data: UpdatePartial<ClientConfig>) {
    return client.put<ApiResponse<ClientConfig>>(`/downloaders/${id}`, data)
  },
  delete(id: number) {
    return client.delete<ApiResponse<void>>(`/downloaders/${id}`)
  },
  testConnection(id: number) {
    return client.post<ApiResponse<{ success: boolean; message?: string }>>(`/downloaders/${id}/test`)
  },
  getTorrents(id: number, params?: { filter?: string; category?: string; tag?: string }) {
    return client.get<ApiResponse<{ items: TorrentInfo[]; total: number }>>(`/downloaders/${id}/torrents`, { params })
  },
  getTorrent(id: number, infoHash: string) {
    return client.get<ApiResponse<TorrentInfo>>(`/downloaders/${id}/torrents/${infoHash}`)
  },
  getFreeSpace(id: number) {
    return client.get<ApiResponse<{ freeSpace: number }>>(`/downloaders/${id}/free-space`)
  },
  getMaindata(id: number) {
    return client.get<ApiResponse<Record<string, unknown>>>(`/downloaders/${id}/maindata`)
  },
  pauseTorrent(id: number, infoHash: string) {
    return client.post<ApiResponse<void>>(`/downloaders/${id}/torrents/${infoHash}/pause`)
  },
  resumeTorrent(id: number, infoHash: string) {
    return client.post<ApiResponse<void>>(`/downloaders/${id}/torrents/${infoHash}/resume`)
  },
  deleteTorrent(id: number, infoHash: string) {
    return client.delete<ApiResponse<void>>(`/downloaders/${id}/torrents/${infoHash}`)
  },
  listPublishTargets() {
    return client.get<ApiResponse<ClientPublishTarget[]>>('/downloaders/publish-targets')
  },
  createPublishTarget(data: CreateWithoutId<ClientPublishTarget>) {
    return client.post<ApiResponse<ClientPublishTarget>>('/downloaders/publish-targets', data)
  },
  updatePublishTarget(data: Partial<ClientPublishTarget> & { id: number }) {
    return client.put<ApiResponse<ClientPublishTarget>>('/downloaders/publish-targets', data)
  },
  deletePublishTarget(id: number) {
    return client.delete<ApiResponse<void>>('/downloaders/publish-targets', { data: { id } })
  },
}
