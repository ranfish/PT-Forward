import client from './client'
import type { ApiResponse, ApiResponsePaginated, Site, SiteCredentials, SiteConfigOverride, UpdatePartial, SearchTorrentResult, DiscountResult } from './types'

export const sitesApi = {
  list(page = 1, size = 20, search = '', extraParams?: Record<string, string>) {
    return client.get<ApiResponsePaginated<Site>>('/sites', { params: { page, size, search, ...extraParams } })
  },
  get(id: number) {
    return client.get<ApiResponse<Site>>(`/sites/${id}`)
  },
  create(data: UpdatePartial<Site>) {
    return client.post<ApiResponse<Site>>('/sites', data)
  },
  update(id: number, data: UpdatePartial<Site>) {
    return client.put<ApiResponse<Site>>(`/sites/${id}`, data)
  },
  delete(id: number) {
    return client.delete<ApiResponse<void>>(`/sites/${id}`)
  },
  testConnection(id: number) {
    return client.post<ApiResponse<{ success: boolean; message?: string }>>(`/sites/${id}/test`)
  },
  detect(id: number) {
    return client.post<ApiResponse<{ framework: string; detail?: string }>>(`/sites/${id}/detect`)
  },
  getStats(id: number) {
    return client.get<ApiResponse<Record<string, unknown>>>(`/sites/${id}/stats`)
  },
  updateCredentials(id: number, data: SiteCredentials) {
    return client.put<ApiResponse<void>>(`/sites/${id}/credentials`, data)
  },
  getOverrides(id: number) {
    return client.get<ApiResponse<{ items: SiteConfigOverride[]; total: number }>>(`/sites/${id}/overrides`)
  },
  updateOverrides(id: number, data: UpdatePartial<SiteConfigOverride>) {
    return client.put<ApiResponse<SiteConfigOverride>>(`/sites/${id}/overrides`, data)
  },
  deleteOverrides(id: number) {
    return client.delete<ApiResponse<void>>(`/sites/${id}/overrides`)
  },
  syncAllStats() {
    return client.post<ApiResponse<{ synced: number; failed: number; failedSites: string[] }>>('/sites/stats-sync', {}, { timeout: 300000 })
  },
  getSyncAllStatus() {
    return client.get<ApiResponse<{ running: boolean; synced: number; failed: number; failedSites: string[] }>>('/sites/stats-sync', { params: { t: Date.now() } })
  },
  syncSiteStats(id: number) {
    return client.post<ApiResponse<Record<string, unknown>>>(`/sites/${id}/stats`)
  },
  batchUpdate(ids: number[], fields: UpdatePartial<Site>) {
    return client.post<ApiResponse<{ updated: number }>>('/sites/batch-update', { ids, fields })
  },
  batchSyncStats(ids: number[]) {
    return client.post<ApiResponse<{ synced: number; failed: number; failedSites: string[] }>>('/sites/batch-sync', { ids }, { timeout: 300000 })
  },
  searchTorrents(id: number, data: { query: string; category?: string; freeOnly?: boolean; sortBy?: string; maxResults?: number }) {
    return client.post<ApiResponse<SearchTorrentResult[]>>(`/sites/${id}/search`, data)
  },
  detectDiscount(id: number, data: { torrentId: string }) {
    return client.post<ApiResponse<DiscountResult>>(`/sites/${id}/discount`, data)
  },
  freezeSite(id: number, data?: { duration?: string; reason?: string }) {
    return client.post<ApiResponse<void>>(`/sites/${id}/freeze`, data || {})
  },
  unfreezeSite(id: number) {
    return client.delete<ApiResponse<void>>(`/sites/${id}/freeze`)
  },
  getFreezeStatus(id: number) {
    return client.get<ApiResponse<{ frozen: boolean; frozenAt?: string; reason?: string }>>(`/sites/${id}/freeze`)
  },
}
