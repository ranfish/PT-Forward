import client from './client'
import type { ApiResponse, ApiResponsePaginated, ListParams, PublishCandidate, PublishTask, PublishResultRecord } from './types'

export const publishApi = {
  listCandidates(params?: ListParams & { status?: string }) {
    return client.get<ApiResponsePaginated<PublishCandidate>>('/publish/candidates', { params })
  },
  deleteCandidate(id: number) {
    return client.delete<ApiResponse<void>>(`/publish/candidates/${id}`)
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

  // §59.374 B2: 批量补推（加种失败集——幂等：下载器自动校验）
  batchRepush(ids: number[]) {
    // per-request 放大：批量长任务（并发 2×N 条×重试链）超全局 30s——
    // 后端 Background 预算 30min，此处对齐拿汇总响应
    return client.post<ApiResponse<{ requested: number; matched: number; pushed: number; failed: number }>>('/publish/seeds/batch-repush', { ids }, { timeout: 600000 })
  },
}

export const publishTorrentsApi = {
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
  getDeclarationFilters() {
    return client.get<ApiResponse<{ patterns: string[]; is_default: boolean }>>('/publish/torrents/declaration-filters')
  },
  setDeclarationFilters(patterns: string[]) {
    return client.put<ApiResponse<{ patterns: string[]; message: string }>>('/publish/torrents/declaration-filters', { patterns })
  },
}

export const manualForwardApi = {
  refresh(data: { type: string; name: string; savePath?: string; infoHash?: string; siteName?: string; screenshots?: string[] }) {
    return client.post<ApiResponse<Record<string, unknown>>>('/manual-forward/refresh', data)
  },
  // §59.51: 后台截图任务（is_local=true 专用，长任务轮询）
  startScreenshotCapture(data: { name: string; savePath: string; clientId: number; infoHash?: string; siteName?: string; subtitleSid?: number }) {
    return client.post<ApiResponse<{ started: boolean }>>('/manual-forward/screenshot-capture', data)
  },
  screenshotCaptureProgress() {
    return client.get<ApiResponse<{ active: boolean; status: string; name: string; screenshots: string[] | null; error: string }>>('/manual-forward/screenshot-capture-progress')
  },
}

export const publishDataApi = {
  listSeedData(params?: { page?: number; page_size?: number; search?: string; source_site?: string; review_status?: 'all' | 'reviewed' | 'unreviewed' }) {
    return client.get<ApiResponse<{ items: unknown[]; total: number; page: number; page_size: number }>>('/publish/seed-data', { params })
  },
  stats(days?: number) {
    return client.get<ApiResponse<{ stats: { todayPublish: number; todaySuccess: number; todayFailed: number; pendingCount: number; reviewedCount: number; totalMetadata: number; yesterdayPublish: number; yesterdaySuccess: number; unreviewedCount: number }; recent: unknown[]; trend: Array<{ day: string; success: number; failed: number }>; target_site_top: Array<{ site: string; count: number }>; status_distribution: Array<{ status: string; count: number }> }>>('/publish/stats', { params: days ? { days } : undefined })
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
// §59.469 附六: 音乐结构化元数据（契约 22 字段——17 现役+B1-B3 预留 5；
// tag 对齐 model.MusicMeta·后端扩展后前端槽位自动点亮）
export interface MusicMeta {
  artist?: string
  album?: string
  genre?: string
  tags?: string[]
  format?: string
  audio_mode?: string
  media?: string
  year?: string
  share_rule?: string
  origin?: string
  other_versions?: string
  track_list?: string
  log_text?: string
  log_score?: number
  has_log?: boolean
  has_cue?: boolean
  release_type?: string
  // B1-B3 预留（MusicMeta 17→22——bit_depth/sample_rate 海豚 MI 标题解析·
  // label/catalogue_no/edition 海豚 remaster 现成；皇后/乌托邦按需）
  bit_depth?: string
  sample_rate?: string
  label?: string
  catalogue_no?: string
  edition?: string
}

export interface SeedListItem {
  hash: string
  name: string
  size: number
  client_id: number
  save_path: string
  // §59.38 观察期行（组聚合，hash 为造键 client|name）
  variants?: number
  last_seen?: string
  cleanup_in_days?: number
  site_name?: string
  title?: string
  subtitle?: string
  poster?: string
  reviewed?: boolean
  flags?: string
  source_category?: string
  fetch_source?: string
  has_mediainfo?: boolean
  is_disc?: boolean
  disc_scan_pending?: boolean
  disc_scan_failed?: boolean // §59.361 A: 扫描失败（带原因时间——非"扫描中"）
  disc_scan_error?: string
  disc_scan_failed_at?: string
  has_description?: boolean
  has_screenshots?: boolean
  fetched_at?: string
  fetched?: boolean
  status: string // forbidden / system_forbidden / no_mapping / reviewed / pending / incomplete / unfetched
  // §59.469 附八: 音乐种行标记（SeedConfig 主入口"详情"按钮/MDP 分流）
  is_music?: boolean
  // §59.469 附九: 字段列音乐行红绿标（方案 1a·与审核门同构）
  music_artist?: string
  music_album?: string
  // §59.131 ②: 簇口径增量字段（发布页一种多站）
  copy_count?: number
  sites?: string[]
  category?: string
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
  tags?: string[]
  tag_labels?: string[] // §59.106: 后端权威显示名（与 tags 索引对齐）
  medium?: string // §59.108: 编辑表单媒介输入框数据源
  audio_tracks?: number
  // §59.469 附六: MDP 消费——音乐结构化元数据（音乐种非 undefined；
  // 入口分流判据+MDP 22 字段渲染）
  music?: MusicMeta
  // §59.228: Tab1 三分区新键（PTGen 资产/帧率/媒介 canonical 单点）
  chinese_title?: string
  english_title?: string
  frame_rate?: string
  medium_canonical?: string
  // §59.75: 产地/类型（PTGen 源归一——keys=canonical 供发布映射, labels=展示）
  region?: { keys: string[]; labels: string[] }
  genre?: { keys: string[]; labels: string[] }
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
  encode?: boolean
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
  // §59.38: 观察期列表（返回结构同 listSeeds：items/total；行字段 client_id/name/variants/last_seen/save_path/size/cleanup_in_days）
  listObserving(params: { client_id?: number; save_path?: string; search?: string; page?: number; page_size?: number }) {
    return client.get<ApiResponse<{ items: SeedListItem[]; total: number }>>('/publish/seeds', { params: { ...params, status: 'observing' } })
  },
  // §59.38: 观察期立即清理
  purgeObserving(clientId: number, name: string) {
    return client.post<ApiResponse<{ message: string; deleted_snaps: number; deleted_metas: number }>>('/publish/seeds/observing/purge', { clientId, name })
  },
  listSeeds(params: { client_id?: number; save_path?: string; status?: string; search?: string; ready?: string; publish_state?: string; target_site?: string; exclude_forbidden?: string; page?: number; page_size?: number }) {
    return client.get<ApiResponse<{ items: SeedListItem[]; total: number }>>('/publish/seeds', { params })
  },
  // §59.361 A: 单颗重扫（失败态行入口——清失败记录+重新入队）
  rescanBDInfo(clientId: number, savePath: string, name: string) {
    return client.post<ApiResponse<{ enqueued: boolean; reason?: string }>>('/publish/bdinfo/rescan', { clientId, savePath, name })
  },
  uniquePaths() {
    return client.get<ApiResponse<{ clients: Array<{ client_id: number; name: string; paths: Array<{ save_path: string; count: number }> }> }>>('/publish/seeds/unique-paths')
  },
  getSeed(infoHash: string, clientId?: number | string) {
    return client.get<ApiResponse<SeedDetail>>(`/publish/seeds/${infoHash}`, { params: clientId !== undefined && clientId !== '' ? { client_id: clientId } : undefined })
  },
  // §59.469 附六: MDP 入口分流探测——getSeed 成功且 music 在场→返回完整
  // detail（MDP 纯渲染消费·音乐场景总请求恒 1 次）；任何失败/不在场→
  // null（调用方 fallback 转种抽屉现状链·零破坏——宁漏勿误同构）
  async probeMusicSeed(infoHash: string, clientId?: number | string): Promise<SeedDetail | null> {
    try {
      const resp = await seedConfigApi.getSeed(infoHash, clientId)
      const d = resp.data?.data
      return d?.music ? d : null
    } catch {
      return null
    }
  },
  putSeed(infoHash: string, data: { poster?: string; screenshots?: string[]; description?: string; tags?: string[]; siteName?: string; reviewed?: boolean; force?: boolean }) {
    return client.put<ApiResponse<{ reviewed: boolean; missing_fields: string[]; reassembled_title?: string; rendered_description?: string; render_error?: string; title_incomplete?: boolean }>>(`/publish/seeds/${infoHash}`, data)
  },
  // §59.479: force=true 批量重获（禁转/无映射视图——绕过簇跳过+强刷 PTGen）
  batchFetch(items: Array<{ hash: string; name: string; size: number; savePath: string }>, clientId: number, force = false) {
    return client.post<ApiResponse<{ message: string; total: number }>>('/publish/seeds/batch-fetch', { items, clientId, force })
  },
  batchFetchProgress() {
    return client.get<ApiResponse<{ active: boolean; stopRequested?: boolean; total: number; done: number; failed: number; items: Array<{ hash: string; name: string; status: string; error?: string }> }>>('/publish/seeds/batch-fetch-progress')
  },
  // §59.495: 批量获取 graceful 停止——当前颗完成后续止·剩余保持未获取
  batchFetchCancel() {
    return client.post<ApiResponse<{ message: string; stopped: boolean }>>('/publish/seeds/batch-fetch-cancel')
  },

  // §59.514 附八 P2: 重建重组标题（任务化+进度+graceful 停止）
  recomputeStart() {
    return client.post<ApiResponse<{ message: string; total: number }>>('/publish/seeds/recompute-profiles-task')
  },
  recomputeProgress() {
    return client.get<ApiResponse<{ active: boolean; total: number; done: number; failed: number; finished: boolean; error?: string }>>('/publish/seeds/recompute-progress')
  },
  recomputeCancel() {
    return client.post<ApiResponse<{ message: string }>>('/publish/seeds/recompute-cancel')
  },
  snapshotUnconfigured(clientId: number, savePath: string) {
    return client.get<ApiResponse<{ items: Array<{ hash: string; name: string; size: number; clientId: number; savePath: string }>; total: number }>>('/downloads/snapshot-unconfigured', { params: { client_id: clientId, save_path: savePath } })
  },
  getFetchPriority() {
    return client.get<ApiResponse<{ priority: string[] }>>('/publish/fetch-priority')
  },
  setFetchPriority(priority: string[]) {
    return client.put<ApiResponse<{ priority: string[] }>>('/publish/fetch-priority', { priority })
  },
  fetchSingleSeed(infoHash: string, clientId: number) {
    return client.post<ApiResponse<{ message: string }>>(`/publish/seeds/${infoHash}/fetch`, {}, { params: { client_id: clientId } })
  },
  // §59.288: UI 批量审核（hash 形态——列表行天然主键；后端解析+簇同步）
  batchReviewByHashes(infoHashes: string[], reviewed = true) {
    return client.post<ApiResponse<{ updated: number }>>('/publish/seed-data/batch-review', { info_hashes: infoHashes, reviewed })
  },
  deleteSeed(infoHash: string) {
    return client.delete<ApiResponse<{ message: string }>>(`/publish/seeds/${infoHash}`)
  },
}
