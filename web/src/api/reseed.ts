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
  getMatches(taskId: number, page?: number, pageSize?: number) {
    const params: Record<string, number> = {}
    if (page) params.page = page
    if (pageSize) params.pageSize = pageSize
    return client.get<ApiResponse<{ items: ReseedMatch[]; total: number; page: number; pageSize: number }>>(`/reseed/tasks/${taskId}/matches`, { params })
  },
  retryMatch(taskId: number, matchId: number) {
    return client.post<ApiResponse<ReseedMatch>>(`/reseed/tasks/${taskId}/matches/${matchId}/retry`)
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
