import client from './client'
import type {
  ApiResponse,
  ApiResponsePaginated,
  SeedingClientConfig,
  SeedingTorrentRecord,
  SeedingScoringConfig,
  SeedingStatsOverview,
  SeedingSiteStat,
  SeedingSiteTrendPoint,
  SeedingSpeedTrendPoint,
  ScoringLog,
  DeleteRule,
  ScoringDryrunRequest,
  ListParams,
  SeedingConfigRequest,
  CreateWithoutId,
  UpdatePartial,
} from './types'

export const seedingApi = {
  getStatus() {
    return client.get<ApiResponse<{ running: boolean; lastRunAt: string | null }>>('/seeding/status')
  },
  getConfig() {
    return client.get<ApiResponse<{ items: SeedingClientConfig[]; total: number }>>('/seeding/configs')
  },
  updateConfig(id: number, data: SeedingConfigRequest) {
    return client.put<ApiResponse<SeedingClientConfig>>(`/seeding/configs/${id}`, data)
  },
  createConfig(data: SeedingConfigRequest) {
    return client.post<ApiResponse<SeedingClientConfig>>('/seeding/configs', data)
  },
  deleteConfig(id: number) {
    return client.delete<ApiResponse<void>>(`/seeding/configs/${id}`)
  },
  listRecords(page = 1, size = 20, filters?: { search?: string; clientId?: string; site?: string; status?: string }) {
    const params: Record<string, string | number> = { page, size }
    if (filters) {
      if (filters.search) params.search = filters.search
      if (filters.clientId) params.client_id = filters.clientId
      if (filters.site) params.site = filters.site
      if (filters.status) params.status = filters.status
    }
    return client.get<ApiResponsePaginated<SeedingTorrentRecord>>('/seeding/records', { params })
  },
  getStatsOverview() {
    return client.get<ApiResponse<SeedingStatsOverview>>('/seeding/stats/overview')
  },
  getStatsBySite() {
    return client.get<ApiResponse<{ items: SeedingSiteStat[]; total: number }>>('/seeding/stats/by-site')
  },
  getStatsTorrents(page = 1, size = 20) {
    return client.get<ApiResponsePaginated<unknown>>('/seeding/stats/torrents', { params: { page, size } })
  },
  getTorrents(page = 1, size = 20) {
    return client.get<ApiResponsePaginated<unknown>>('/seeding/torrents', { params: { page, size } })
  },
  resumeRecord(id: number) {
    return client.post<ApiResponse<void>>(`/seeding/records/${id}/resume`)
  },
  pauseRecord(id: number) {
    return client.post<ApiResponse<void>>(`/seeding/records/${id}/pause`)
  },
  scoringDryrun(data: ScoringDryrunRequest) {
    return client.post<ApiResponse<{ score: number; demand: number }>>('/seeding/scoring-dryrun', data)
  },
  getScoringConfig(subscriptionId?: number) {
    const params: Record<string, number> = {}
    if (subscriptionId) params.subscriptionId = subscriptionId
    return client.get<ApiResponse<SeedingScoringConfig>>('/seeding/scoring-config', { params })
  },
  updateScoringConfig(data: UpdatePartial<SeedingScoringConfig>, subscriptionId?: number) {
    const params: Record<string, number> = {}
    if (subscriptionId) params.subscriptionId = subscriptionId
    return client.put<ApiResponse<SeedingScoringConfig>>('/seeding/scoring-config', data, { params })
  },
  listScoringLogs(params?: ListParams) {
    return client.get<ApiResponsePaginated<ScoringLog>>('/seeding/scoring-logs', { params })
  },
}

export const seedingClientsApi = {
  trigger(clientId: string) {
    return client.post<ApiResponse<void>>(`/seeding/clients/${clientId}/trigger`)
  },
}

export const seedingDryrunApi = {
  runAll() {
    return client.post<ApiResponse<void>>('/seeding/dryrun')
  },
  runBySubscription(subId: string) {
    return client.post<ApiResponse<void>>(`/seeding/dryrun/${subId}`)
  },
}

export const seedingStatsApi = {
  siteTrend(site: string, range = '7d') {
    return client.get<ApiResponse<{ site: string; trends: SeedingSiteTrendPoint[] }>>(`/seeding/stats/by-site/${encodeURIComponent(site)}/trend`, { params: { range } })
  },
  downloaderSpeedTrend(id: string, range = '24h') {
    return client.get<ApiResponse<{ clientId: string; points: SeedingSpeedTrendPoint[] }>>(`/seeding/stats/downloader/${id}/speed-trend`, { params: { range } })
  },
}

export const deleteRulesApi = {
  list() {
    return client.get<ApiResponse<{ items: DeleteRule[]; total: number }>>('/seeding/delete-rules')
  },
  create(data: CreateWithoutId<DeleteRule>) {
    return client.post<ApiResponse<DeleteRule>>('/seeding/delete-rules', data)
  },
  update(id: number, data: UpdatePartial<DeleteRule>) {
    return client.put<ApiResponse<DeleteRule>>(`/seeding/delete-rules/${id}`, data)
  },
  delete(id: number) {
    return client.delete<ApiResponse<void>>(`/seeding/delete-rules/${id}`)
  },
  test(id: number) {
    return client.post<ApiResponse<{ matched: boolean; torrentsAffected: number }>>(`/seeding/delete-rules/${id}/test`)
  },
}
