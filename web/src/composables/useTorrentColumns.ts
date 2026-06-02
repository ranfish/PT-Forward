import { useI18n } from 'vue-i18n'
import { h, type VNode } from 'vue'
import { Tag } from 'ant-design-vue'
import { formatTime, copyToClipboard, formatBytes } from '@/utils/format'
import { message } from 'ant-design-vue'

export type TorrentColumnKey =
  | 'title'
  | 'site_name'
  | 'torrent_id'
  | 'discount'
  | 'is_free'
  | 'has_hr'
  | 'torrent_size'
  | 'latest_upload'
  | 'info_hash'
  | 'client_id'
  | 'source'
  | 'status'
  | 'flushed_at'
  | 'created_at'
  | 'updated_at'
  | 'actions'

export interface TorrentColumnOptions {
  show?: TorrentColumnKey[]
  hide?: TorrentColumnKey[]
  statusRender?: (record: Record<string, unknown>) => VNode | string
  actionsRender?: (record: Record<string, unknown>) => VNode
}

export function useTorrentColumns(options: TorrentColumnOptions = {}) {
  const { t } = useI18n()

  function copyHash(text: string) {
    copyToClipboard(text)
    message.success(t('common.copied'))
  }

  const allColumns: Record<TorrentColumnKey, Record<string, unknown>> = {
    title: {
      title: t('seeding.torrentName'),
      dataIndex: 'title',
      key: 'title',
      width: 260,
      ellipsis: true,
      customRender: ({ text, record }: { text: string; record: { detail_url?: string; torrent_id?: string } }) => {
        const display = text || record.torrent_id || '-'
        if (record.detail_url) {
          return h('a', { href: record.detail_url, target: '_blank', style: 'color: #1890ff' }, display)
        }
        return display
      },
    },
    site_name: {
      title: t('common.site'),
      dataIndex: 'site_name',
      key: 'site_name',
      width: 70,
    },
    torrent_id: {
      title: 'Torrent ID',
      dataIndex: 'torrent_id',
      key: 'torrent_id',
      width: 80,
      ellipsis: true,
    },
    discount: {
      title: t('seeding.discount'),
      dataIndex: 'discount',
      key: 'discount',
      width: 70,
      customRender: ({ text }: { text: string }) => {
        const map: Record<string, { label: string; color: string }> = {
          FREE: { label: 'FREE', color: 'green' },
          '2XFREE': { label: '2XFREE', color: 'gold' },
          '2XUP': { label: '2xUp', color: 'blue' },
          '2X50': { label: '2x50', color: 'cyan' },
          'PERCENT_50': { label: '50%', color: 'orange' },
          'PERCENT_25': { label: '25%', color: 'orange' },
          'PERCENT_70': { label: '70%', color: 'orange' },
          'PERCENT_75': { label: '75%', color: 'orange' },
          'PERCENT_30': { label: '30%', color: 'orange' },
        }
        const info = map[text]
        if (!info) return text || '-'
        return h(Tag, { color: info.color, size: 'small' }, () => info.label)
      },
    },
    is_free: {
      title: t('seeding.free'),
      dataIndex: 'is_free',
      key: 'is_free',
      width: 50,
      customRender: ({ text }: { text: boolean }) =>
        text ? h(Tag, { color: 'green', size: 'small' }, () => 'FREE') : '-',
    },
    has_hr: {
      title: 'HR',
      dataIndex: 'has_hr',
      key: 'has_hr',
      width: 45,
      customRender: ({ text }: { text: boolean }) =>
        text ? h(Tag, { color: 'red', size: 'small' }, () => 'HR') : '-',
    },
    torrent_size: {
      title: t('seeding.size'),
      dataIndex: 'torrent_size',
      key: 'torrent_size',
      width: 75,
      customRender: ({ text }: { text: number }) => (text ? formatBytes(text) : '-'),
    },
    latest_upload: {
      title: t('seeding.upload'),
      dataIndex: 'latest_upload',
      key: 'latest_upload',
      width: 80,
      customRender: ({ text }: { text: number }) =>
        text ? h('span', { style: 'color: #52c41a; font-weight: 500' }, formatBytes(text)) : '-',
    },
    info_hash: {
      title: 'InfoHash',
      dataIndex: 'info_hash',
      key: 'info_hash',
      width: 180,
      customRender: ({ text }: { text: string }) =>
        h('span', { style: 'cursor:pointer;font-family:monospace;font-size:12px', onClick: () => copyHash(text) }, text),
    },
    client_id: {
      title: t('seeding.client'),
      dataIndex: 'client_id',
      key: 'client_id',
      width: 60,
    },
    source: {
      title: t('seeding.source'),
      dataIndex: 'source',
      key: 'source',
      width: 60,
    },
    status: {
      title: t('common.status'),
      key: 'status',
      width: 80,
      customRender: options.statusRender
        ? ({ record }: { record: Record<string, unknown> }) => options.statusRender!(record)
        : ({ record }: { record: { status?: string } }) => record.status || '-',
    },
    flushed_at: {
      title: t('seeding.flushedAt'),
      dataIndex: 'flushed_at',
      key: 'flushed_at',
      width: 120,
      customRender: ({ text }: { text: string }) => formatTime(text),
    },
    created_at: {
      title: t('common.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 120,
      customRender: ({ text }: { text: string }) => formatTime(text),
    },
    updated_at: {
      title: t('common.updatedAt'),
      dataIndex: 'updated_at',
      key: 'updated_at',
      width: 120,
      customRender: ({ text }: { text: string }) => formatTime(text),
    },
    actions: {
      title: t('common.actions'),
      key: 'actions',
      width: 80,
      customRender: options.actionsRender
        ? ({ record }: { record: Record<string, unknown> }) => options.actionsRender!(record)
        : () => '-',
    },
  }

  const orderedKeys: TorrentColumnKey[] = [
    'title',
    'site_name',
    'torrent_id',
    'discount',
    'is_free',
    'has_hr',
    'torrent_size',
    'latest_upload',
    'info_hash',
    'client_id',
    'source',
    'status',
    'flushed_at',
    'created_at',
    'updated_at',
    'actions',
  ]

  let keys: TorrentColumnKey[]
  if (options.show) {
    keys = options.show
  } else if (options.hide) {
    keys = orderedKeys.filter(k => !options.hide!.includes(k))
  } else {
    keys = orderedKeys
  }

  const columns = keys.map(k => allColumns[k]).filter(Boolean)

  return { columns }
}
