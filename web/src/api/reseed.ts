import client from './client'
import type { ApiResponse, ApiResponsePaginated, ReseedTask, ReseedMatch, UpdatePartial } from './types'

export interface ReseedIYUULog {
  id: number
  created_at: string
  task_id: number
  request_hashes: number
  response_targets: number
  matched_hashes: number
  status: string
  message: string
  duration_ms: number
}

export interface IYUULogStats {
  TotalCalls: number
  SuccessCalls: number
  ErrorCalls: number
  TotalRequests: number
  TotalMatched: number
  TotalTargets: number
}


export interface ReseedFeatureLog {
  id: number
  created_at: string
  task_id: number
  site: string
  queried: number
  matched: number
  status: string
}

export interface FeatureLogStats {
  TotalCalls: number
  TotalQueried: number
  TotalMatched: number
}

export interface ReseedNegativeCacheItem {
  id: number
  source_site: string
  source_torrent_id: string
  source_info_hash: string
  excluded_targets: string
  last_method: string
  layer_depth: number
  expires_at: string
  created_at: string
}

export const reseedApi = {
  listTasks(page = 1, size = 20) {
    return client.get<ApiResponsePaginated<ReseedTask>>('/reseed/tasks', { params: { page, size } })
  },
  getTask(id: number) {
    return client.get<ApiResponse<ReseedTask>>(`/reseed/tasks/${id}`)
  },
  createTask(data: UpdatePartial<ReseedTask>) {
    return client.post<ApiResponse<ReseedTask>>('/reseed/tasks', data)
  },
  updateTask(id: number, data: UpdatePartial<ReseedTask>) {
    return client.put<ApiResponse<ReseedTask>>(`/reseed/tasks/${id}`, data)
  },
  deleteTask(id: number) {
    return client.delete<ApiResponse<void>>(`/reseed/tasks/${id}`)
  },
  triggerTask(id: number) {
    return client.post<ApiResponse<void>>(`/reseed/tasks/${id}/trigger`)
  },
  cancelTask(id: number) {
    return client.post<ApiResponse<void>>(`/reseed/tasks/${id}/cancel`)
  },
  getMatches(taskId: number, opts?: { page?: number; pageSize?: number; clientId?: string; site?: string; torrentId?: string; status?: string; orderField?: string; order?: string }) {
    const params: Record<string, string | number> = {}
    if (opts?.page) params.page = opts.page
    if (opts?.pageSize) params.pageSize = opts.pageSize
    if (opts?.clientId) params.clientId = opts.clientId
    if (opts?.site) params.site = opts.site
    if (opts?.torrentId) params.torrentId = opts.torrentId
    if (opts?.status) params.status = opts.status
    if (opts?.orderField) params.orderField = opts.orderField
    if (opts?.order) params.order = opts.order
    return client.get<ApiResponse<{ items: ReseedMatch[]; total: number; page: number; pageSize: number }>>(`/reseed/tasks/${taskId}/matches`, { params })
  },
  getFeatureLogs(taskId: number, page?: number, pageSize?: number) {
    const params: Record<string, number> = {}
    if (page) params.page = page
    if (pageSize) params.pageSize = pageSize
    return client.get<ApiResponse<{ items: ReseedFeatureLog[]; total: number; page: number; pageSize: number; stats: FeatureLogStats }>>(`/reseed/tasks/${taskId}/feature-logs`, { params })
  },
  clearAllMatches(taskId: number) {
    return client.post<ApiResponse<{ deleted: number }>>(`/reseed/tasks/${taskId}/matches/clear`)
  },
  batchRetryMatches(taskId: number, matchIds: number[]) {
    return client.post<ApiResponse<{ succeeded: number; failed: number; messages: string[] }>>(`/reseed/tasks/${taskId}/matches/batch-retry`, { match_ids: matchIds })
  },
  batchDeleteMatches(taskId: number, matchIds: number[]) {
    return client.post<ApiResponse<{ deleted: number }>>(`/reseed/tasks/${taskId}/matches/batch-delete`, { match_ids: matchIds })
  },
  retryMatch(taskId: number, matchId: number) {
    return client.post<ApiResponse<ReseedMatch>>(`/reseed/tasks/${taskId}/matches/${matchId}/retry`)
  },
  getNegativeCache(taskId: number, page?: number, pageSize?: number) {
    const params: Record<string, number> = {}
    if (page) params.page = page
    if (pageSize) params.pageSize = pageSize
    return client.get<ApiResponse<{ items: ReseedNegativeCacheItem[]; total: number; page: number; pageSize: number }>>(`/reseed/tasks/${taskId}/negative-cache`, { params })
  },
  deleteNegativeCache(taskId: number, infoHash: string, site?: string) {
    const params: Record<string, string> = { infoHash }
    if (site) params.site = site
    return client.delete<ApiResponse<void>>(`/reseed/tasks/${taskId}/negative-cache`, { params })
  },
  getIYUULogs(taskId: number, page?: number, pageSize?: number) {
    const params: Record<string, number> = {}
    if (page) params.page = page
    if (pageSize) params.pageSize = pageSize
    return client.get<ApiResponse<{ items: ReseedIYUULog[]; total: number; page: number; pageSize: number; stats: IYUULogStats }>>(`/reseed/tasks/${taskId}/iyuu-logs`, { params })
  },
}
