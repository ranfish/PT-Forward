import client from './client'
import type { ApiResponse, ApiResponsePaginated, ListParams, ManualForwardSubmitRequest, PublishCandidate, PublishGroup, PublishTask, PublishResultRecord } from './types'

export const publishApi = {
  listCandidates(params?: ListParams & { status?: string }) {
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
  listResults(params?: ListParams & { status?: string; target_site?: string }) {
    return client.get<ApiResponsePaginated<PublishResultRecord>>('/publish/results', { params })
  },
}

export const publishTorrentsApi = {
  list(clientId?: number) {
    return client.get<ApiResponse<{ items: PublishTorrentItem[]; total: number; total_sites: number; querying: boolean; query_progress: { done: number; total: number } }>>('/publish/torrents', { params: clientId ? { client_id: clientId } : {} })
  },
  queryCoverage(data: { client_id: number; info_hash: string; name?: string; size?: number }) {
    return client.post<ApiResponse<CoverageResult>>('/publish/torrents/coverage', data)
  },
  queryStatus(clientId: number) {
    return client.get<ApiResponse<{ querying: boolean; done: number; total: number }>>('/publish/torrents/query-status', { params: { client_id: clientId } })
  },
  detectSource(data: { info_hash: string; name: string }) {
    return client.post<ApiResponse<SourceDetectResult>>('/publish/torrents/detect-source', data)
  },
  listGroupMappings() {
    return client.get<ApiResponse<{ items: Array<Record<string, unknown> & { id: number }>; total: number }>>('/publish/torrents/group-mappings')
  },
  createGroupMapping(data: { group_name: string; domain: string; site_name: string }) {
    return client.post<ApiResponse<unknown>>('/publish/torrents/group-mappings', data)
  },
  updateGroupMapping(id: number, data: { group_name: string; domain: string; site_name: string }) {
    return client.put<ApiResponse<unknown>>(`/publish/torrents/group-mappings/${id}`, data)
  },
  deleteGroupMapping(id: number) {
    return client.delete<ApiResponse<unknown>>(`/publish/torrents/group-mappings/${id}`)
  },
  listGroupedSiteNames() {
    return client.get<ApiResponse<{ sites: string[] }>>('/publish/torrents/group-mappings/sites')
  },
  batchPublish(data: { client_id: number; source_site: string; target_site: string; items: { info_hash: string; name: string; size: number; save_path: string }[] }) {
    return client.post<ApiResponse<{ created: number; failed: number; candidate_ids: number[]; target_site: string }>>('/publish/torrents/batch-publish', data)
  },
  getDeclarationFilters() {
    return client.get<ApiResponse<{ patterns: string[]; is_default: boolean }>>('/publish/torrents/declaration-filters')
  },
  setDeclarationFilters(patterns: string[]) {
    return client.put<ApiResponse<{ patterns: string[]; message: string }>>('/publish/torrents/declaration-filters', { patterns })
  },
  previewTitle(data: { target_site: string; title_components: Record<string, string> }) {
    return client.post<ApiResponse<{ title: string; target_site: string }>>('/publish/torrents/preview-title', data)
  },
  previewTitleBatch(data: { target_sites: string[]; title_components: Record<string, string> }) {
    return client.post<ApiResponse<{ results: Record<string, string> }>>('/publish/torrents/preview-title-batch', data)
  },
}

export interface SourceDetectResult {
  source_site: string
  source_site_id: number
  group_name: string
  torrent_id: string
  auto_detected: boolean
  candidates: { site_name: string; torrent_id: string; has_cookie: boolean }[]
}

export interface PublishTorrentItem {
  info_hash: string
  name: string
  size: number
  save_path: string
  state: string
  uploaded: number
  queried: boolean
  coverage: {
    has_count: number
    total_sites: number
    target_count: number
    sites: CoverageSite[]
  }
}

export interface CoverageSite {
  site_name: string
  status: string
  source: string
  confidence: number
  torrent_id: string
  detail_url: string
  queried_at: string
}

export interface CoverageResult {
  info_hash: string
  sites: CoverageSite[]
  has_count: number
  total_sites: number
  target_count: number
}

export const manualForwardApi = {
  seededTorrents(clientId?: number) {
    return client.get<ApiResponse<unknown[]>>('/manual-forward/seeded-torrents', { params: clientId ? { client_id: clientId } : {} })
  },
  startAnalyze(data: { client_id: number; info_hash: string; name: string; save_path: string; source_site?: string; source_torrent_id?: string; metadata_priority?: string }) {
    return client.post<ApiResponse<{ taskId: number }>>('/manual-forward/analyze', data)
  },
  pollAnalyze(taskId: number) {
    return client.get<ApiResponse<{ status: string; result?: unknown }>>(`/manual-forward/analyze/${taskId}`)
  },
  eligibleTargets(data: { source_site: string; blocked_targets?: string[] }) {
    return client.post<ApiResponse<string[]>>('/manual-forward/eligible-targets', data)
  },
  mergeFields(data: { info_hash: string; mode: string }) {
    return client.post<ApiResponse<unknown>>('/manual-forward/merge', data)
  },
  previewFields(data: { info_hash: string; target_site: string; mode?: string }) {
    return client.post<ApiResponse<unknown>>('/manual-forward/preview', data)
  },
  submit(data: ManualForwardSubmitRequest) {
    return client.post<ApiResponse<void>>('/manual-forward/submit', data)
  },
  batchSubmit(items: ManualForwardSubmitRequest[]) {
    return client.post<ApiResponse<{ succeeded: number; failed: number }>>('/manual-forward/batch-submit', { items })
  },
}
