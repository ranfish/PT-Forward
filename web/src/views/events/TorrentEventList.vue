<template>
  <div>
    <a-page-header :title="t('event.title')">
      <template #extra>
        <a-space>
          <a-select v-model:value="siteFilter" :placeholder="t('event.filterBySite')" allow-clear style="width: 200px" @change="fetchEvents">
            <a-select-option v-for="s in siteNames" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
          <a-button @click="fetchEvents">
            <template #icon><ReloadOutlined /></template>
            {{ t('common.refresh') }}
          </a-button>
          <a-popconfirm :title="t('event.cleanupConfirm')" @confirm="handleCleanup">
            <a-button danger>{{ t('event.cleanupOldEvents') }}</a-button>
          </a-popconfirm>
        </a-space>
      </template>
    </a-page-header>

    <a-table
      :columns="columns"
      :data-source="events"
      :loading="loading"
      :pagination="{ pageSize: 20, showSizeChanger: true, showTotal: (total: number) => t('common.totalCount', { count: total }) }"
      row-key="id"
      size="small"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'site_name'">
          <a-tag>{{ record.site_name }}</a-tag>
        </template>
        <template v-if="column.key === 'size'">
          {{ formatSize(record.size) }}
        </template>
        <template v-if="column.key === 'source_id'">
          <a-typography-text :copyable="record.source_id ? { text: record.source_id } : false" style="font-size: 12px">
            {{ record.source_id ? record.source_id.substring(0, 12) + '...' : '-' }}
          </a-typography-text>
        </template>
        <template v-if="column.key === 'title'">
          <a-tooltip :title="record.title">
            {{ record.title ? (record.title.length > 60 ? record.title.substring(0, 60) + '...' : record.title) : '-' }}
          </a-tooltip>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { torrentEventsApi } from '@/api/torrent-events'
import { formatTime } from '@/utils/format'

const { t } = useI18n()

const loading = ref(false)
const events = ref<Record<string, unknown>[]>([])
const siteFilter = ref<string | undefined>(undefined)
const siteNames = ref<string[]>([])

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('common.site'), key: 'site_name', width: 120 },
  { title: t('event.torrentId'), dataIndex: 'torrent_id', key: 'torrent_id', width: 80 },
  { title: t('common.title'), key: 'title', ellipsis: true },
  { title: t('common.size'), key: 'size', width: 100 },
  { title: t('event.sourceId'), key: 'source_id', width: 140 },
  { title: t('common.time'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
]

function formatSize(bytes: number) {
  if (!bytes) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

async function fetchEvents() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {}
    if (siteFilter.value) params.site = siteFilter.value
    const resp = await torrentEventsApi.list(params)
    const body = resp.data.data
    events.value = (body?.items || []) as Record<string, unknown>[]

    if (!siteNames.value.length) {
      const names = new Set<string>()
      events.value.forEach((e: Record<string, unknown>) => { if (e.site_name) names.add(e.site_name as string) })
      siteNames.value = Array.from(names).sort()
    }
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

async function handleCleanup() {
  try {
    const resp = await torrentEventsApi.cleanup()
    message.success(t('event.cleanupCompleted', { count: resp.data.data?.deleted || 0 }))
    fetchEvents()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

onMounted(() => fetchEvents())
</script>
