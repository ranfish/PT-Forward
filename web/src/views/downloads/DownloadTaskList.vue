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

    <a-table
      :data-source="tasks"
      :columns="columns"
      :loading="loading"
      row-key="id"
      :pagination="{
        current: page,
        pageSize: size,
        total: total,
        showSizeChanger: true,
        showTotal: (total: number) => `${total} ${t('common.items')}`,
      }"
      @change="onPageChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'torrent_name'">
          <span :title="record.torrent_name" style="display: inline-block; max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
            {{ record.torrent_name }}
          </span>
        </template>
        <template v-else-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
          <span v-if="record.progress > 0 && record.progress < 100 && record.status === 'downloading'" style="margin-left: 4px; font-size: 12px; color: #999">
            {{ record.progress.toFixed(1) }}%
          </span>
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
        <template v-else-if="column.key === 'action'">
          <a-popconfirm
            :title="t('downloads.deleteConfirm')"
            @confirm="handleDelete(record)"
            :ok-text="t('common.confirm')"
            :cancel-text="t('common.cancel')"
          >
            <template #icon></template>
            <template #description>
              <a-radio-group v-model:value="deleteMode" style="margin-top: 8px" v-if="record.status !== 'deleted'">
                <a-radio :value="true">{{ t('downloads.deleteWithCompanions') }}</a-radio>
                <a-radio :value="false">{{ t('downloads.deleteSiteOnly') }}</a-radio>
              </a-radio-group>
            </template>
            <a-button type="text" danger size="small" :disabled="record.status === 'deleted'">
              {{ t('common.delete') }}
            </a-button>
          </a-popconfirm>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { ReloadOutlined } from '@ant-design/icons-vue'
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
const deleteMode = ref(true)

const clientOptions = computed(() => {
  const set = new Set<string>()
  tasks.value.forEach(t => { if (t.client_id) set.add(t.client_id) })
  return Array.from(set).sort()
})

const columns = computed(() => [
  { title: t('downloads.torrentName'), dataIndex: 'torrent_name', key: 'torrent_name', ellipsis: true },
  { title: t('downloads.client'), dataIndex: 'client_id', key: 'client_id', width: 120 },
  { title: t('downloads.site'), dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: t('common.status'), key: 'status', width: 140 },
  { title: t('downloads.transfer'), key: 'transfer_status', width: 120 },
  { title: t('downloads.size'), key: 'total_size', width: 100 },
  { title: t('downloads.created'), key: 'created_at', width: 150 },
  { title: t('common.action'), key: 'action', width: 80 },
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

async function handleDelete(record: DownloadTask) {
  try {
    await downloadsApi.delete(record.id, deleteMode.value)
    message.success(t('common.deleted'))
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

onMounted(() => {
  fetchData()
  // Auto refresh every 30s
  setInterval(fetchData, 30000)
})
</script>
