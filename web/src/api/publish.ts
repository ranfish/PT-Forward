import client from './client'
import type { ApiResponse, ApiResponsePaginated, ListParams, ManualForwardSubmitRequest, PublishCandidate, PublishGroup, PublishTask, PublishResultRecord } from './types'

export const publishApi = {
  listCandidates(params?: ListParams) {
    return client.get<ApiResponsePaginated<PublishCandidate>>('/publish/candidates', { params })
  },
  getCandidate(id: number) {
    return client.get<ApiResponse<PublishCandidate>>(`/publish/candidates/${id}`)
  },
  deleteCandidate(id: number) {
    return client.delete<ApiResponse<void>>(`/publish/candidates/${id}`)
  },
  manualPublish(id: number) {
    return client.post<ApiResponse<void>>(`/publish/candidates/${id}/publish`)
  },
  listGroups() {
    return client.get<ApiResponse<{ items: PublishGroup[]; total: number }>>('/publish/groups')
  },
  getGroup(id: number) {
    return client.get<ApiResponse<PublishGroup>>(`/publish/groups/${id}`)
  },
  deleteGroup(id: number) {
    return client.delete<ApiResponse<void>>(`/publish/groups/${id}`)
  },
  pauseGroup(id: number) {
    return client.post<ApiResponse<void>>(`/publish/groups/${id}/lifecycle/pause`)
  },
  resumeGroup(id: number) {
    return client.post<ApiResponse<void>>(`/publish/groups/${id}/lifecycle/resume`)
  },
  lifecycleDeleteGroup(id: number) {
    return client.post<ApiResponse<void>>(`/publish/groups/${id}/lifecycle/delete`)
  },
  createTask(data: { source_site_id?: number; sourceSiteId?: number; target_sites?: string[]; manual_check?: boolean; [key: string]: unknown }) {
    return client.post<ApiResponse<PublishTask>>('/publish/tasks', data)
  },
  listTasks(params?: ListParams) {
    return client.get<ApiResponsePaginated<PublishTask>>('/publish/tasks', { params })
  },
  getTask(id: number) {
    return client.get<ApiResponse<PublishTask>>(`/publish/tasks/${id}`)
  },
  deleteTask(id: number) {
    return client.delete<ApiResponse<void>>(`/publish/tasks/${id}`)
  },
  cancelTask(id: number) {
    return client.post<ApiResponse<void>>(`/publish/tasks/${id}/cancel`)
  },
  listResults(params?: ListParams) {
    return client.get<ApiResponsePaginated<PublishResultRecord>>('/publish/results', { params })
  },
}

export const manualForwardApi = {
  seededTorrents(clientId?: number) {
    return client.get<ApiResponse<unknown[]>>('/manual-forward/seeded-torrents', { params: clientId ? { client_id: clientId } : {} })
  },
  startAnalyze(data: { client_id: number; info_hash: string; name: string; save_path: string }) {
    return client.post<ApiResponse<{ taskId: number }>>('/manual-forward/analyze', data)
  },
  pollAnalyze(taskId: number) {
    return client.get<ApiResponse<{ status: string; result?: unknown }>>(`/manual-forward/analyze/${taskId}`)
  },
  eligibleTargets(data: { source_site: string; blocked_targets?: string[] }) {
    return client.post<ApiResponse<string[]>>('/manual-forward/eligible-targets', data)
  },
  submit(data: ManualForwardSubmitRequest) {
    return client.post<ApiResponse<void>>('/manual-forward/submit', data)
  },
  batchSubmit(items: ManualForwardSubmitRequest[]) {
    return client.post<ApiResponse<{ succeeded: number; failed: number }>>('/manual-forward/batch-submit', { items })
  },
}
