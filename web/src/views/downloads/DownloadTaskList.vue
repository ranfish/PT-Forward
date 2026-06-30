<template>
  <div style="padding: 24px">
    <a-page-header :title="t('downloads.title')" :subtitle="t('downloads.subtitle')">
      <template #extra>
        <a-space>
          <a-select v-model:value="filterClient" style="width: 180px" allow-clear
            :placeholder="t('downloads.filterClient')" @change="fetchData">
            <a-select-option v-for="c in clientOptions" :key="c" :value="c">{{ c }}</a-select-option>
          </a-select>
          <a-select v-model:value="filterStatus" style="width: 140px" allow-clear
            :placeholder="t('downloads.filterStatus')" @change="fetchData">
            <a-select-option value="downloading">{{ t('downloads.statusDownloading') }}</a-select-option>
            <a-select-option value="completed">{{ t('downloads.statusCompleted') }}</a-select-option>
            <a-select-option value="paused">{{ t('downloads.statusPaused') }}</a-select-option>
            <a-select-option value="error">{{ t('downloads.statusError') }}</a-select-option>
          </a-select>
          <a-button @click="fetchData" :icon="h(ReloadOutlined)">{{ t('common.refresh') }}</a-button>
        </a-space>
      </template>
    </a-page-header>

    <div v-if="selectedRowKeys.length > 0" style="margin-bottom: 16px">
      <a-space>
        <span>{{ selectedRowKeys.length }} {{ t('downloads.selected') }}</span>
        <a-button size="small" @click="handleBulk('pause')" :disabled="!hasSelected">{{ t('downloads.pause') }}</a-button>
        <a-button size="small" @click="handleBulk('resume')">{{ t('downloads.resume') }}</a-button>
        <a-button size="small" @click="handleBulk('recheck')">{{ t('downloads.recheck') }}</a-button>
        <a-popconfirm :title="t('downloads.bulkDeleteConfirm')" @confirm="handleBulkDelete">
          <a-button size="small" danger>{{ t('common.delete') }}</a-button>
        </a-popconfirm>
        <a-button size="small" type="text" @click="selectedRowKeys = []">{{ t('common.cancel') }}</a-button>
      </a-space>
    </div>

    <a-table
      :data-source="tasks"
      :columns="columns"
      :loading="loading"
      row-key="id"
      :row-selection="{ selectedRowKeys, onChange: onSelectChange }"
      :pagination="{
        current: page,
        pageSize: size,
        total: total,
        showSizeChanger: true,
        showTotal: (total: number) => `${total} ${t('common.items')}`,
      }"
      @change="onPageChange"
      size="small"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'torrent_name'">
          <span :title="record.torrent_name" style="display: inline-block; max-width: 350px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; vertical-align: middle;">
            {{ record.torrent_name }}
          </span>
        </template>
        <template v-else-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
        </template>
        <template v-else-if="column.key === 'progress'">
          <div v-if="record.status === 'downloading' || record.status === 'paused'" style="min-width: 100px">
            <a-progress :percent="Math.round(record.progress)" size="small" :stroke-color="record.status === 'paused' ? '#faad14' : '#1890ff'" />
          </div>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'speed'">
          <div v-if="record.status === 'downloading'" style="font-size: 12px; line-height: 1.4">
            <div><ArrowUpOutlined /> {{ formatSpeed(record.upload_speed) }}</div>
            <div><ArrowDownOutlined /> {{ formatSpeed(record.download_speed) }}</div>
          </div>
          <span v-else-if="record.status === 'completed'" style="font-size: 12px">
            <ArrowUpOutlined /> {{ formatSpeed(record.upload_speed) }}
          </span>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'ratio'">
          <span :style="{ color: record.ratio >= 1 ? '#52c41a' : '#faad14' }">{{ record.ratio > 0 ? record.ratio.toFixed(2) : '-' }}</span>
        </template>
        <template v-else-if="column.key === 'transfer_status'">
          <a-tag v-if="record.transfer_status" :color="transferColor(record.transfer_status)">
            {{ transferLabel(record.transfer_status) }}
          </a-tag>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'total_size'">
          {{ formatBytes(record.total_size) }}
        </template>
        <template v-else-if="column.key === 'created_at'">
          {{ formatTime(record.created_at) }}
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { ReloadOutlined, ArrowUpOutlined, ArrowDownOutlined } from '@ant-design/icons-vue'
import { downloadsApi, type DownloadTask } from '@/api/downloads'
import { formatBytes, formatTime } from '@/utils/format'

const { t } = useI18n()

const tasks = ref<DownloadTask[]>([])
const loading = ref(false)
const page = ref(1)
const size = ref(20)
const total = ref(0)
const filterClient = ref<string>('')
const filterStatus = ref<string>('')
const selectedRowKeys = ref<number[]>([])

const hasSelected = computed(() => selectedRowKeys.value.length > 0)

const clientOptions = computed(() => {
  const set = new Set<string>()
  tasks.value.forEach(t => { if (t.client_id) set.add(t.client_id) })
  return Array.from(set).sort()
})

const columns = computed(() => [
  { title: t('downloads.torrentName'), dataIndex: 'torrent_name', key: 'torrent_name', ellipsis: true },
  { title: t('downloads.client'), dataIndex: 'client_id', key: 'client_id', width: 100 },
  { title: t('common.status'), key: 'status', width: 90 },
  { title: t('downloads.progress'), key: 'progress', width: 120 },
  { title: t('downloads.speed'), key: 'speed', width: 100 },
  { title: t('downloads.ratio'), key: 'ratio', width: 70 },
  { title: t('downloads.transfer'), key: 'transfer_status', width: 90 },
  { title: t('downloads.size'), key: 'total_size', width: 90 },
  { title: t('downloads.created'), key: 'created_at', width: 140 },
])

function statusColor(status: string): string {
  const map: Record<string, string> = {
    downloading: 'processing',
    completed: 'success',
    paused: 'warning',
    error: 'error',
    deleted: 'default',
    pending: 'default',
  }
  return map[status] || 'default'
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    downloading: t('downloads.statusDownloading'),
    completed: t('downloads.statusCompleted'),
    paused: t('downloads.statusPaused'),
    error: t('downloads.statusError'),
    deleted: t('downloads.statusDeleted'),
    pending: t('downloads.statusPending'),
  }
  return map[status] || status
}

function transferColor(status: string): string {
  const map: Record<string, string> = {
    transfer_pending: 'warning',
    transferring: 'processing',
    transferred: 'success',
    transfer_failed: 'error',
    transfer_partial: 'error',
  }
  return map[status] || 'default'
}

function transferLabel(status: string): string {
  const map: Record<string, string> = {
    transfer_pending: t('downloads.transferPending'),
    transferring: t('downloads.transferTransferring'),
    transferred: t('downloads.transferTransferred'),
    transfer_failed: t('downloads.transferFailed'),
    transfer_partial: t('downloads.transferPartial'),
  }
  return map[status] || status
}

function formatSpeed(bytesPerSec: number): string {
  if (!bytesPerSec || bytesPerSec <= 0) return '0 B/s'
  return formatBytes(bytesPerSec) + '/s'
}

async function fetchData() {
  loading.value = true
  try {
    const resp = await downloadsApi.list({
      page: page.value,
      size: size.value,
      client_id: filterClient.value || undefined,
      status: filterStatus.value || undefined,
    })
    const data = resp.data.data
    tasks.value = data.items || []
    total.value = data.total || 0
  } catch {
    // ignore
  } finally {
    loading.value = false
  }
}

function onPageChange(pag: { current: number; pageSize: number }) {
  page.value = pag.current
  size.value = pag.pageSize
  fetchData()
}

function onSelectChange(keys: number[]) {
  selectedRowKeys.value = keys
}

async function handleBulk(action: string) {
  try {
    const resp = await downloadsApi.bulkAction(selectedRowKeys.value, action)
    const d = resp.data.data
    message.success(`${d.succeeded} ${t('common.success')}, ${d.failed} ${t('common.failed')}`)
    selectedRowKeys.value = []
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleBulkDelete() {
  try {
    const resp = await downloadsApi.bulkAction(selectedRowKeys.value, 'delete', true)
    const d = resp.data.data
    message.success(`${d.succeeded} ${t('common.success')}, ${d.failed} ${t('common.failed')}`)
    selectedRowKeys.value = []
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

onMounted(() => {
  fetchData()
  setInterval(fetchData, 30000)
})
</script>
