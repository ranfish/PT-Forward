export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

export interface PaginatedData<T = unknown> {
  items: T[]
  total: number
  page: number
  size: number
}

export type ApiResponsePaginated<T = unknown> = ApiResponse<PaginatedData<T>>

export interface ListParams {
  page?: number
  size?: number
  search?: string
  sort_by?: string
  sort_dir?: string
  limit?: number
  [key: string]: string | number | undefined
}

export interface SeedingConfigRequest {
  clientId?: string
  enabled?: boolean
  deleteRuleIds?: string
  rejectRuleIds?: string
  autoDeleteCron?: string
  mainDataCron?: string
  fitTimeCheckMs?: number
  diskProtectEnabled?: boolean
  minDiskSpaceGB?: number
  emergencyBuffer?: number
  spaceAlarmEnabled?: boolean
  spaceAlarmGb?: number
  minDiskSpacePercent?: number
  maxActiveUploads?: number
  maxActiveDownloads?: number
  superSeedingDefault?: boolean
  scope?: string
  preFilterEnabled?: boolean
  enhancementBatchSize?: number
  enhancementCacheTtl?: number
  activeTimeWindows?: string
  emaAlpha?: number
  cleanupScoreWeights?: string
  archiveGranularity?: string
  reannounceBefore?: boolean
  reannounceRetries?: number
  reannounceIntervalMs?: number
  reannounceWaitMs?: number
  minSeedHoursBeforeDelete?: number
}

export interface SeedingClientConfig {
  id: number
  client_id: string
  created_at: string
  updated_at: string
  enabled: boolean
  delete_rule_ids: string
  reject_rule_ids: string
  auto_delete_cron: string
  maindata_cron: string
  fit_time_check_ms: number
  disk_protect_enabled: boolean
  min_disk_space_gb: number
  emergency_buffer: number
  space_alarm_enabled: boolean
  space_alarm_gb: number
  min_disk_space_percent: number
  max_active_uploads: number
  max_active_downloads: number
  max_active_seeding: number
  super_seeding_default: boolean
  scope: string
  pre_filter_enabled: boolean
  enhancement_batch_size: number
  enhancement_cache_ttl: number
  active_time_windows: string
  ema_alpha: number
  cleanup_score_weights: string
  archive_granularity: string
  reannounce_before: boolean
  reannounce_retries: number
  reannounce_interval_ms: number
  reannounce_wait_ms: number
  min_seed_hours_before_delete: number
}

export interface SeedingTorrentRecord {
  id: number
  created_at: string
  updated_at: string
  client_id: string
  info_hash: string
  site_name: string
  torrent_id: string
  has_hr: boolean
  hr_seed_time_h: number
  is_free: boolean
  free_end_at: string | null
  free_level: string
  discount: string
  status: string
  last_action_by: string
  source: string
  first_matched_at: string | null
  title: string
  detail_url: string
}

export interface DeleteRule {
  id: number
  created_at: string
  updated_at: string
  alias: string
  priority: number
  enabled: boolean
  type: string
  logic: string
  conditions: string
  expr: string
  fit_time: number
  action: string
  delete_num: number
  limit_speed_bytes: number
  reannounce_before: boolean
  reannounce_wait_ms: number
  reannounce_retries: number
  reannounce_interval_ms: number
  delete_companions: boolean
}

export interface SeedingScoringConfig {
  enabled: boolean
  half_life_hours: number
  site_weights_json: string
  include_2xup: boolean
  max_candidates: number
  max_active_seeding: number
  top_n_confirm: number
  batch_limit: number
  min_score: number
  push_interval_ms: number
}

export interface ScoringLog {
  id: number
  cycle_id: string
  client_id: string
  info_hash: string
  site_name: string
  torrent_id: string
  title: string
  detail_url: string
  score: number
  demand: number
  upload_val: number
  recency: number
  created_at: string
}

export interface PathMapping {
  sourcePath: string
  reseedPath: string
}

export interface ClientConfig {
  id: number
  name: string
  type: string
  url: string
  username: string
  role: string
  transferTargetId?: string
  enabled: boolean
  isDefault: boolean
  torrentDir?: string
  pathMappings: PathMapping[]
  downloadSpeed: number
  uploadSpeed: number
  freeSpace: number
  totalDiskSpace: number
  connected: boolean
  createdAt: string
  updatedAt: string
}

export interface ClientPublishTarget {
  id: number
  client_id: number
  site_name: string
  category_mapping: string
  source_mapping: string
  codec_mapping: string
  auto_publish: boolean
  notify_on_publish: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface TorrentInfo {
  hash: string
  name: string
  is_finished: boolean
  is_paused: boolean
  removed: boolean
  state: string
  error: string
  num_complete: number
  num_incomplete: number
  ratio: number
  save_path: string
  tags: string[]
  total_size: number
  category: string
  progress: number
  uploaded: number
  upload_speed: number
  download_speed: number
  seed_time: number
  added_at: string
}

export interface Site {
  id: number
  domain: string
  name: string
  createdAt: string
  updatedAt: string
  baseUrl: string
  framework: string
  authType: string
  enabled: boolean
  hasPasskey: boolean
  passkeyMasked?: string
  passkeyAlias?: string
  passkeyHint?: string
  hasCookie: boolean
  cookieMasked?: string
  hasApiKey: boolean
  apiKeyMasked?: string
  hasBearerToken: boolean
  hasAuthKey: boolean
  authKeyMasked?: string
  hasAuthHash: boolean
  hasRssKey: boolean
  rssKeyMasked?: string
  userId?: number
  hashStrategy: string
  sizeStrategy: string
  idStrategy: string
  idPattern: string
  hashXmlTagName?: string
  sizeXmlTagName?: string
  hashUrlParamName?: string
  sizeDescRegex?: string
  sizeTitleRegex?: string
  sizeBaseUnit: number
  requiresSideLoading: boolean
  downloadMode: string
  downloadUrlTemplate?: string
  downloadPagePattern?: string
  frameworkDetected: boolean
  frameworkVerified: boolean
  detectionDetail?: string
  cookieCloudSync: boolean
  cookieCloudDomain?: string
  alternativeDomains?: string
  lastSyncAt?: string | null
  isSource: boolean
  isTarget: boolean
  participateAutoPublish: boolean
  assumeFree: boolean
  overrideRssUrl?: string
  overrideSavePath?: string
  proxyUrl?: string
  skipSslVerify: boolean
  maxConcurrent: number
  uploadBytes: string
  downloadBytes: string
  seedingPoints: number
  seedingSize: string
  seedingCount: number
  userClass: string
  ratio: number
  bonusPoints: number
  statsSyncedAt?: string | null
  hrStrategy: string
  targetTypes?: string
  reseedLimitCount: number
  reseedLimitInterval: number
  iyuuLimitCount: number
  iyuuLimitInterval: number
  supportsPiecesHashApi: boolean
}

export interface SiteConfigOverride {
  id: number
  created_at: string
  updated_at: string
  site_name: string
  field_path: string
  field_value: string
  source: string
}

export interface SiteCredentials {
  passkey?: string
  cookie?: string
  apiKey?: string
  bearerToken?: string
  authKey?: string
  authHash?: string
  userId?: number
  rssKey?: string
}

export interface FilterRule {
  id: number
  name: string
  ruleType: string
  priority: number
  conditions: RuleCondition[]
  savePath?: string
  category?: string
  tags?: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface RuleCondition {
  key: string
  compare_type: string
  value: string
}

export interface RSSSubscription {
  id: number
  createdAt: string
  updatedAt: string
  name: string
  enabled: boolean
  urls: string[]
  siteName: string
  cron: string
  clientId?: string
  savePath?: string
  category?: string
  addPaused: boolean
  autoTmm: boolean
  uploadLimitKb?: number
  downloadLimitKb?: number
  tags?: string[]
  scrapeFree: boolean
  scrapeHr: boolean
  pushNotify: boolean
  notifyId?: string
  publishEnabled: boolean
  publishTargets?: string[]
  autoReseed: boolean
  reseedClientIds?: string[]
  skipSameSize: boolean
  skipSameSizeWindowMin: number
  skipSameSizeStrict: boolean
  addCountPerHour: number
  conditions?: RuleCondition[]
  useCustomRegex: boolean
  regexStr: string
  replaceStr: string
  freeWaitEnabled: boolean
  freeWaitMaxWaitSec: number
  freeWaitRecheckSec: number
  freeWaitMinRemain: number
  recheckEnabled: boolean
  recheckIntervalH: number
  recheckMaxCount: number
  recheckMaxAgeH: number
  feasibilityEnabled: boolean
  feasibilitySpeedLimit: number
  feasibilitySizeLimit: number
  feasibilitySafety: number
  diskBudgetEnabled: boolean
  diskBudgetMinGB: number
  candidateClients?: string[]
  clientSelection: string
  diskGuardEnabled: boolean
  diskGuardThreshold: number
  minScore: number
  lifecyclePauseSeeders: number
  lifecycleDeleteSeeders: number
  lifecycleDeleteSeedHours: number
  paused: boolean
  pauseReason: string
  pausedAt?: string | null
}

export interface NotificationChannel {
  id: number
  type: string
  name: string
  enabled: boolean
  healthy: boolean
  createdAt: string
  updatedAt: string
  config?: string
  events?: string
  maxErrorsPerHour: number
  timeoutMs: number
  quietHoursStart?: string
  quietHoursEnd?: string
  messageTemplate?: string
  hasConfig: boolean
  overrides?: string
  failoverGroupId?: string
  minPriority: number
  digestTemplate?: string
  digestIntervalMin: number
  consecutiveFailures?: number
}

export interface NotificationHistory {
  id: number
  channelId: number
  event: string
  level: string
  title: string
  body: string
  success: boolean
  errorMsg: string
  createdAt: string
}

export interface PublishCandidate {
  id: number
  created_at: string
  updated_at: string
  subscription_id: string
  source_site: string
  source_torrent_id: string
  info_hash: string
  torrent_name: string
  size: number
  client_id: string
  source_client_id: string
  local_save_path: string
  local_file_path: string
  target_sites: string
  discount: string
  free_end_at: string | null
  has_hr: boolean
  download_completed: boolean
  completed_at: string | null
  publish_status: string
  publish_result: string
  skip_reason: string
  user_overrides: string
  role: string
}

export interface PublishGroup {
  id: number
  created_at: string
  updated_at: string
  candidate_id: number
  source_hash: string
  source_site: string
  source_torrent_id: string
  subscription_id: string
  status: string
  last_error: string
  seed_start_time: string | null
}

export interface PublishTask {
  id: number
  type: string
  source_site_id: number
  target_sites: string[]
  manual_check: boolean
  checked_at: string | null
  status: string
  created_at: string
  updated_at: string
}

export interface PublishResultRecord {
  id: number
  created_at: string
  updated_at: string
  candidate_id: number
  source_site: string
  target_site: string
  torrent_id: string
  is_official: boolean
  has_forbid: boolean
  has_exclusive: boolean
  hr_detected: boolean
  size_out_of_range: boolean
  cross_site_excluded: boolean
  team_detected: string
  status: string
  skip_reason: string
  publish_url: string
  error_message: string
  completed_at: string | null
}

export interface ReseedTask {
  id: number
  created_at: string
  updated_at: string
  name: string
  enabled: boolean
  client_ids: string
  source_site_ids: string
  target_site_ids: string
  target_site_excludes: string
  release_group_excludes: string
  category_excludes: string
  title_keyword_excludes: string
  match_methods: string
  confidence_threshold: number
  fallback_enabled: boolean
  max_fallbacks: number
  engine_mode: string
  size_tolerance_percent: number
  max_injections_per_run: number
  injection_interval_s: number
  injection_jitter_s: number
  injection_concurrency: number
  scan_concurrency: number
  reseed_category: string
  reseed_tags: string
  schedule: string
  status: string
  version: number
  max_retries: number
  retry_interval_h: number
}

export interface ReseedMatch {
  id: number
  created_at: string
  updated_at: string
  client_id: string
  source_site: string
  source_torrent_id: string
  source_info_hash: string
  target_site: string
  target_torrent_id: string
  target_info_hash: string
  match_method: string
  confidence: number
  decision_type: string
  decision_detail: string
  status: string
  injected_at: string | null
  fail_reason: string
  retry_count: number
  next_retry_at: string | null
}

export interface CookieCloudConfig {
  id: number
  serverUrl: string
  uuid: string
  hasPassword: boolean
  cryptoType: string
  syncEnabled: boolean
  syncInterval: number
  lastSyncAt?: string | null
}

export interface IYUUConfig {
  id: number
  token: string
  baseUrl: string
  isVip: boolean
  enabled: boolean
  version: string
  requestTimeoutMs: number
  syncIntervalHours: number
}

export interface CloudFPConfig {
  id: number
  base_url: string
  api_token: string
  enabled: boolean
  request_timeout_sec: number
}

export interface LifecycleConfig {
  pauseSeeders: boolean
  deleteSeeders: boolean
  deleteSeedHours: number
  checkInterval: string
  maxConcurrentChecks: number
}

export interface BackpressureResponse {
  queueDepth: number
  maxQueueDepth: number
  activePublishes: number
  maxConcurrentPublishes: number
  bandwidthLimitKB: number
  isThrottled: boolean
  pauseOnPressure: boolean
}

export interface BackpressureUpdateRequest {
  max_concurrent?: number
  pause_on_pressure?: boolean
}

export interface SeedingStatsOverview {
  totalTorrents: number
  activeTorrents: number
  pausedTorrents: number
  realSeeding: number
  realDownloading: number
  totalUploadBytes: number
  totalDownloadBytes: number
  todayUploadBytes: number
  todayDownloadBytes: number
  globalRatio: number
  todayDeleted: number
  todayAdded: number
}

export interface SeedingSiteStat {
  siteName: string
  seedingCount: number
  totalCount: number
  todayAdded: number
  todayDeleted: number
  activeFree: number
  activeNonFree: number
  todayUploadBytes: number
  historyUploadBytes: number
}

export interface SeedingSiteTrendPoint {
  date: string
  count: number
}

export interface SeedingSpeedTrendPoint {
  timestamp: string
  uploadSpeed: number
  downloadSpeed: number
}

export interface ScoringDryrunRequest {
  seeders: number
  leechers: number
  ageHours: number
  size: number
  discount: string
  halfLifeHours: number
  siteWeight: number
}

export interface SettingsRestoreRequest {
  settings: Record<string, string>
}

export interface ManualForwardSubmitRequest {
  client_id?: number
  info_hash?: string
  name?: string
  save_path?: string
  source_site?: string
  source_site_id?: number
  source_torrent_id?: string
  title?: string
  description?: string
  media_info?: string
  screenshots?: string | string[]
  target_sites?: string[]
}

export type CreateWithoutId<T> = Omit<T, 'id' | 'createdAt' | 'updatedAt' | 'created_at' | 'updated_at'>
export type UpdatePartial<T> = Partial<CreateWithoutId<T>>

export interface SearchTorrentResult {
  torrent_id: string
  title: string
  size: number
  seeders: number
  leechers: number
  discount: string
  publish_at: string
  detail_url: string
  download_url: string
}

export interface DiscountResult {
  level: string
  free_end_at: string | null
  multiplier: number
}

export interface SchedulerTask {
  name: string
  type: string
  schedule: string
  last_run_at: string | null
  last_error: string
  success_count: number
  error_count: number
  paused: boolean
}
