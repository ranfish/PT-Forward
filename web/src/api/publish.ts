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
  listResults(params?: ListParams & { status?: string; target_site?: string; trigger?: string; start_date?: string; end_date?: string }) {
    return client.get<ApiResponsePaginated<PublishResultRecord>>('/publish/results', { params })
  },
  deleteResult(id: number) {
    return client.delete<ApiResponse<{ deleted: number }>>(`/publish/results/${id}`)
  },
}

export const publishTorrentsApi = {
  list(clientId?: number) {
    return client.get<ApiResponse<{ items: PublishTorrentItem[]; total: number; total_sites: number; querying: boolean; query_progress: { done: number; total: number } }>>('/publish/torrents', { params: clientId ? { client_id: clientId } : {} })
  },
  queryCoverage(data: { clientId: number; infoHash: string; name?: string; size?: number }) {
    return client.post<ApiResponse<CoverageResult>>('/publish/torrents/coverage', data)
  },
  batchQueryCoverage(data: { clientId: number; infoHashes: string[] }) {
    return client.post<ApiResponse<{ queried: number }>>('/publish/torrents/batch-coverage', data)
  },
  queryStatus(clientId: number) {
    return client.get<ApiResponse<{ querying: boolean; done: number; total: number }>>('/publish/torrents/query-status', { params: { client_id: clientId } })
  },
  detectSource(data: { infoHash: string; name: string }) {
    return client.post<ApiResponse<SourceDetectResult>>('/publish/torrents/detect-source', data)
  },
  listGroupMappings() {
    return client.get<ApiResponse<{ items: Array<Record<string, unknown> & { id: number }>; total: number }>>('/publish/torrents/group-mappings')
  },
  createGroupMapping(data: { groupName: string; domain: string; siteName: string }) {
    return client.post<ApiResponse<unknown>>('/publish/torrents/group-mappings', data)
  },
  updateGroupMapping(id: number, data: { groupName: string; domain: string; siteName: string }) {
    return client.put<ApiResponse<unknown>>(`/publish/torrents/group-mappings/${id}`, data)
  },
  deleteGroupMapping(id: number) {
    return client.delete<ApiResponse<unknown>>(`/publish/torrents/group-mappings/${id}`)
  },
  listGroupedSiteNames() {
    return client.get<ApiResponse<{ sites: string[] }>>('/publish/torrents/group-mappings/sites')
  },
  batchPublish(data: { clientId: number; sourceSite: string; targetSite: string; items: { infoHash: string; name: string; size: number; savePath: string }[] }) {
    return client.post<ApiResponse<{ created: number; failed: number; candidate_ids: number[]; target_site: string }>>('/publish/torrents/batch-publish', data)
  },
  getDeclarationFilters() {
    return client.get<ApiResponse<{ patterns: string[]; is_default: boolean }>>('/publish/torrents/declaration-filters')
  },
  setDeclarationFilters(patterns: string[]) {
    return client.put<ApiResponse<{ patterns: string[]; message: string }>>('/publish/torrents/declaration-filters', { patterns })
  },
  previewTitle(data: { targetSite: string; titleComponents: Record<string, string> }) {
    return client.post<ApiResponse<{ title: string; target_site: string }>>('/publish/torrents/preview-title', data)
  },
  previewTitleBatch(data: { targetSites: string[]; titleComponents: Record<string, string> }) {
    return client.post<ApiResponse<{ results: Record<string, string> }>>('/publish/torrents/preview-title-batch', data)
  },
}

export interface SourceDetectResult {
  source_site: string
  source_site_id: number
  group_name: string
  torrent_id: string
  auto_detected: boolean
  candidates: { siteName: string; torrentId: string; hasCookie: boolean }[]
}

export interface PublishTorrentItem {
  info_hash: string
  name: string
  size: number
  save_path: string
  state: string
  uploaded: number
  progress: number
  ratio: number
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
  startAnalyze(data: { clientId: number; infoHash: string; name: string; savePath: string; size?: number; sourceSite?: string; sourceTorrentId?: string; metadataPriority?: string; fetchSource?: string }) {
    return client.post<ApiResponse<{ task_id: number }>>('/manual-forward/start-analyze', data)
  },
  parseTitle(title: string) {
    return client.post<ApiResponse<{ components: unknown; title_components: Record<string, string>; standardized: unknown; category: string }>>('/manual-forward/parse-title', { title })
  },
  pollAnalyze(taskId: number) {
    return client.get<ApiResponse<{ status: string; result?: unknown }>>(`/manual-forward/analyze/${taskId}`)
  },
  eligibleTargets(data: { sourceSite: string; blockedTargets?: string[] }) {
    return client.post<ApiResponse<string[]>>('/manual-forward/eligible-targets', data)
  },
  mergeFields(data: { infoHash: string; mode: string }) {
    return client.post<ApiResponse<unknown>>('/manual-forward/merge', data)
  },
  previewFields(data: { infoHash: string; targetSite: string; mode?: string; userOverrides?: Record<string, string> }) {
    return client.post<ApiResponse<unknown>>('/manual-forward/preview', data)
  },
  submit(data: ManualForwardSubmitRequest) {
    return client.post<ApiResponse<void>>('/manual-forward/submit', data)
  },
  batchSubmit(items: ManualForwardSubmitRequest[]) {
    return client.post<ApiResponse<{ succeeded: number; failed: number }>>('/manual-forward/batch-submit', { items })
  },
  refresh(data: { type: string; name: string; savePath?: string; infoHash?: string; siteName?: string; screenshots?: string[] }) {
    return client.post<ApiResponse<Record<string, unknown>>>('/manual-forward/refresh', data)
  },
}

export const publishDataApi = {
  cachedSites(infoHash: string) {
    return client.get<ApiResponse<{ info_hash: string; sites: Array<{ id: number; siteName: string; torrentId: string; reviewed: boolean; fetchedAt: string; title: string; subtitle: string }> }>>('/publish/cached-sites', { params: { info_hash: infoHash } })
  },
  listSeedData(params?: { page?: number; page_size?: number; search?: string; source_site?: string; review_status?: 'all' | 'reviewed' | 'unreviewed' }) {
    return client.get<ApiResponse<{ items: unknown[]; total: number; page: number; page_size: number }>>('/publish/seed-data', { params })
  },
  saveSeedData(id: number, data: { title?: string; subtitle?: string; description?: string; screenshots?: string; poster?: string; mediainfo?: string; tags?: string }) {
    return client.put<ApiResponse<{ success: boolean; id: number }>>(`/publish/seed-data/${id}`, data)
  },
  stats(days?: number) {
    return client.get<ApiResponse<{ stats: { todayPublish: number; todaySuccess: number; todayFailed: number; pendingCount: number; reviewedCount: number; totalMetadata: number; yesterdayPublish: number; yesterdaySuccess: number; unreviewedCount: number }; recent: unknown[]; trend: Array<{ day: string; success: number; failed: number }>; target_site_top: Array<{ site: string; count: number }>; status_distribution: Array<{ status: string; count: number }> }>>('/publish/stats', { params: days ? { days } : undefined })
  },
  coverageCache(infoHash: string) {
    return client.get<ApiResponse<{ info_hash: string; sites: Array<{ site_name: string; status: string; source: string }> }>>('/publish/coverage-cache', { params: { info_hash: infoHash } })
  },
  batchReview(ids: number[], reviewed: boolean) {
    return client.post<ApiResponse<{ updated: number }>>('/publish/seed-data/batch-review', { ids, reviewed })
  },
  batchDelete(ids: number[]) {
    return client.post<ApiResponse<{ deleted: number }>>('/publish/seed-data/batch-delete', { ids })
  },
  getSourcePriority() {
    return client.get<ApiResponse<{ priority: string[] }>>('/publish/source-priority')
  },
  setSourcePriority(priority: string[]) {
    return client.put<ApiResponse<{ priority: string[] }>>('/publish/source-priority', { priority })
  },
}

// §59.20 种子配置页 API
export interface SeedListItem {
  hash: string
  name: string
  size: number
  client_id: string
  save_path: string
  site_name?: string
  title?: string
  subtitle?: string
  poster?: string
  reviewed?: boolean
  flags?: string
  source_category?: string
  fetch_source?: string
  has_mediainfo?: boolean
  has_description?: boolean
  has_screenshots?: boolean
  fetched_at?: string
  fetched?: boolean
  status: string // forbidden / system_forbidden / no_mapping / reviewed / pending / incomplete / unfetched
}

export interface SeedDetail {
  info_hash: string
  site_name: string
  title: string
  subtitle: string
  poster: string
  description: string
  screenshots: string[]
  mediainfo: string
  bdinfo: string
  statement: string
  imdb_url: string
  douban_url: string
  tmdb_url: string
  flags: string
  source_category: string
  reviewed: boolean
  fetched_at: string
  fetch_source: string
  // 14 DB 平铺字段
  category: string
  form: string
  resolution: string
  video_codec: string
  audio_codec: string
  audio_channels: string
  audio_tech: string
  hdr: string
  bit_depth: string
  source_type: string
  specification: string
  source_platform: string
  edition_info: string
  region_code: string
  // 5 ParseTitleTech 字段
  main_title: string
  season_episode: string
  year: string
  release_group: string
  chinese_prefix: string
  // 校验
  missing_fields: string[]
}

export const seedConfigApi = {
  listSeeds(params: { client_id: string; save_path: string; page?: number; page_size?: number }) {
    return client.get<ApiResponse<{ items: SeedListItem[]; total: number }>>('/publish/seeds', { params })
  },
  getSeed(infoHash: string, clientId?: string) {
    return client.get<ApiResponse<SeedDetail>>(`/publish/seeds/${infoHash}`, { params: clientId ? { client_id: clientId } : undefined })
  },
  putSeed(infoHash: string, data: { poster?: string; screenshots?: string[]; description?: string; siteName?: string }) {
    return client.put<ApiResponse<{ reviewed: boolean; missing_fields: string[] }>>(`/publish/seeds/${infoHash}`, data)
  },
  batchFetch(items: Array<{ hash: string; name: string; size: number; savePath: string }>, clientId: string) {
    return client.post<ApiResponse<{ message: string; total: number }>>('/publish/seeds/batch-fetch', { items, clientId })
  },
  batchFetchProgress() {
    return client.get<ApiResponse<{ active: boolean; total: number; done: number; failed: number; items: Array<{ hash: string; name: string; status: string; error?: string }> }>>('/publish/seeds/batch-fetch-progress')
  },
  snapshotUnconfigured(clientId: string, savePath: string) {
    return client.get<ApiResponse<{ items: Array<{ hash: string; name: string; size: number; clientId: string; savePath: string }>; total: number }>>('/downloads/snapshot-unconfigured', { params: { client_id: clientId, save_path: savePath } })
  },
  getFetchPriority() {
    return client.get<ApiResponse<{ priority: string[] }>>('/publish/fetch-priority')
  },
  setFetchPriority(priority: string[]) {
    return client.put<ApiResponse<{ priority: string[] }>>('/publish/fetch-priority', { priority })
  },
}
