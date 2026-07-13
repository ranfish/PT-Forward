import { useI18n } from 'vue-i18n'

const roleMap: Record<string, string> = {
  download: 'downloader.roleDownload',
  seeding: 'downloader.roleSeeding',
  source: 'downloader.roleSource',
  reseed: 'downloader.roleReseed',
}

const downloaderTypeMap: Record<string, string> = {
  qbittorrent: 'downloader.typeQbittorrent',
  transmission: 'downloader.typeTransmission',
}

const seedingStatusMap: Record<string, string> = {
  pending: 'seeding.pendingStatus',
  seeding: 'seeding.seedingStatus',
  paused_free_end: 'seeding.pausedFreeEnd',
  paused_rule: 'seeding.pausedRule',
  downloading: 'seeding.downloadingStatus',
  deleted: 'seeding.deletedStatus',
  paused: 'seeding.pausedStatus',
  stopped: 'seeding.stoppedStatus',
  unregistered: 'seeding.unregisteredStatus',
}

const publishStatusMap: Record<string, string> = {
  pending: 'publish.statusPending',
  completed: 'publish.statusCompleted',
  failed: 'publish.statusFailed',
  active: 'publish.statusActive',
  publishing: 'publish.statusPublishing',
  publish_failed: 'publish.statusPublishFailed',
  partially_paused: 'publish.statusPartiallyPaused',
  all_paused: 'publish.statusAllPaused',
  monitoring: 'publish.statusMonitoring',
  deleting: 'publish.statusDeleting',
  deleted: 'publish.statusDeleted',
  published: 'publish.statusPublished',
  skipped: 'publish.statusSkipped',
  running: 'publish.statusRunning',
  cancelled: 'publish.statusCancelled',
  done: 'publish.statusDone',
  exists: 'publish.statusExists',
  edited: 'publish.statusEdited',
}

const publishRoleMap: Record<string, string> = {
  source: 'publish.roleSource',
  target: 'publish.roleTarget',
}

const reseedStatusMap: Record<string, string> = {
  idle: 'reseed.statusIdle',
  running: 'reseed.statusRunning',
  completed: 'reseed.statusCompleted',
  failed: 'reseed.statusFailed',
  pending: 'reseed.statusPending',
  injected: 'reseed.statusInjected',
  new: 'reseed.statusNew',
  active: 'reseed.statusActive',
  matched: 'reseed.statusMatched',
  skipped: 'reseed.statusSkipped',
  cancelled: 'reseed.statusCancelled',
}

const matchMethodMap: Record<string, string> = {
  pieces_hash: 'Pieces Hash',
  iyuu: 'IYUU',
  fingerprint: '本地指纹',
  cloud_fingerprint: '云指纹',
  size_title: '大小+标题',
  search_verify: '搜索验证',
}

const memberStatusMap: Record<string, string> = {
  new: '新建',
  uploaded: '已上传',
  uploading: '上传中',
  injected: '已注入',
  seeding_confirmed: '做种确认',
  downloading: '下载中',
  paused: '已暂停',
  error: '错误',
  banned: '已封禁',
  deleted: '已删除',
}

const seedingSourceMap: Record<string, string> = {
  rss: 'RSS',
  free_wait: '免费等待',
  imported: '导入',
  manual: '手动',
}

const publishTypeMap: Record<string, string> = {
  single: '单站',
  batch: '批量',
}

const auditModuleMap: Record<string, string> = {
  auth: '认证', rss: 'RSS', site: '站点', seeding: '刷流',
  delete_rule: '删种规则', client: '下载器', system: '系统',
  settings: '设置', cookiecloud: 'CookieCloud', notification: '通知',
}

const auditActionMap: Record<string, string> = {
  create: '创建', update: '更新', delete: '删除', trigger: '触发',
  sync: '同步', login: '登录', clear: '清理', batch_update: '批量更新',
  batch_sync: '批量同步', update_credentials: '更新凭证', import: '导入',
}

const qbStateMap: Record<string, string> = {
  stalledDL: '停滞下载', uploading: '上传中', pausedUP: '暂停做种',
  pausedTL: '暂停', downloading: '下载中', queuedUP: '排队做种',
  allocating: '分配空间', checkingResumeData: '检查恢复数据',
  metaDL: '获取元数据', forcedDL: '强制下载', forcedUP: '强制上传',
  checkingUP: '检查中', stalledUP: '停滞做种', error: '错误',
  missingFiles: '文件缺失',
}

const fetchStatusMap: Record<string, string> = {
  ok: '成功', error: '失败',
}

export function useEnumLabels() {
  const { t } = useI18n()

  function translateRole(role: string): string {
    const key = roleMap[role]
    return key ? t(key) : role
  }

  function translateDownloaderType(tp: string): string {
    const key = downloaderTypeMap[tp]
    return key ? t(key) : tp
  }

  function translateSeedingStatus(status: string): string {
    const key = seedingStatusMap[status]
    return key ? t(key) : status
  }

  function translatePublishStatus(status: string): string {
    const key = publishStatusMap[status]
    return key ? t(key) : status
  }

  function translatePublishRole(role: string): string {
    const key = publishRoleMap[role]
    return key ? t(key) : role
  }

  function translateReseedStatus(status: string): string {
    const key = reseedStatusMap[status]
    return key ? t(key) : status
  }

  function translateMatchMethod(m: string): string {
    return matchMethodMap[m] || m
  }

  function translateMemberStatus(s: string): string {
    return memberStatusMap[s] || s
  }

  function translateSeedingSource(s: string): string {
    return seedingSourceMap[s] || s
  }

  function translatePublishType(tp: string): string {
    return publishTypeMap[tp] || tp
  }

  function translateAuditModule(m: string): string {
    return auditModuleMap[m] || m
  }

  function translateAuditAction(a: string): string {
    return auditActionMap[a] || a
  }

  function translateQbState(s: string): string {
    return qbStateMap[s] || s
  }

  function translateFetchStatus(s: string): string {
    return fetchStatusMap[s] || s
  }

  return {
    translateRole,
    translateDownloaderType,
    translateSeedingStatus,
    translatePublishStatus,
    translatePublishRole,
    translateReseedStatus,
    translateMatchMethod,
    translateMemberStatus,
    translateSeedingSource,
    translatePublishType,
    translateAuditModule,
    translateAuditAction,
    translateQbState,
    translateFetchStatus,
  }
}
