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

  return {
    translateRole,
    translateDownloaderType,
    translateSeedingStatus,
    translatePublishStatus,
    translatePublishRole,
    translateReseedStatus,
  }
}
