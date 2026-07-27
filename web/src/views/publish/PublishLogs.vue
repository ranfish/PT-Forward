<template>
  <div style="padding: 24px">
    <div style="margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center">
      <h3 style="margin: 0">发布日志</h3>
      <a-space>
        <a-input-search
          v-model:value="searchTarget"
          placeholder="目标站搜索"
          style="width: 200px"
          allow-clear
          @search="onFilterChange"
        />
        <a-select
          v-model:value="statusFilter"
          style="width: 130px"
          placeholder="状态"
          allow-clear
          @change="onFilterChange"
        >
          <a-select-option value="completed">成功</a-select-option>
          <a-select-option value="failed">失败</a-select-option>
          <a-select-option value="skipped">跳过</a-select-option>
          <a-select-option value="publishing">发布中</a-select-option>
          <a-select-option value="exists">已存在</a-select-option>
          <a-select-option value="edited">已编辑</a-select-option>
        </a-select>
        <a-select
          v-model:value="triggerFilter"
          style="width: 120px"
          placeholder="触发"
          allow-clear
          @change="onFilterChange"
        >
          <a-select-option value="manual">手动</a-select-option>
          <a-select-option value="batch">批量</a-select-option>
          <a-select-option value="reseed">辅种</a-select-option>
          <a-select-option value="wizard">向导</a-select-option>
        </a-select>
        <a-range-picker v-model:value="dateRange" @change="onFilterChange" />
        <a-button @click="fetchData"><ReloadOutlined /></a-button>
        <a-button @click="exportCSV" :disabled="tableData.length === 0"><DownloadOutlined /> CSV</a-button>
        <a-switch
          v-model:checked="autoRefresh"
          checked-children="自动"
          un-checked-children="手动"
          size="small"
        />
      </a-space>
    </div>

    <a-table
      :columns="columns"
      :data-source="tableData"
      :loading="loading"
      :pagination="pagination"
      row-key="id"
      :scroll="{ x: 1770 }"
      size="small"
      @change="onTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'trigger'">
          <a-tag :color="triggerColor(record.trigger)" style="font-size: 11px">{{ triggerLabel(record.trigger) }}</a-tag>
        </template>
        <template v-if="column.key === 'title'">
          <div>
            <div v-if="record.subtitle" style="color: #666; font-size: 12px">{{ record.subtitle }}</div>
            <div>{{ record.title || record.target_site }}</div>
          </div>
        </template>
        <template v-else-if="column.key === 'source_site'">
          <a-tag>{{ record.source_site }}</a-tag>
        </template>
        <template v-else-if="column.key === 'target_site'">
          <a-tag color="blue">{{ record.target_site }}</a-tag>
        </template>
        <template v-else-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
        </template>
        <template v-else-if="column.key === 'seeded'">
          <a-tag v-if="record.seeded" color="green">已加种</a-tag>
          <a-tag v-else-if="record.seed_error" color="red">加种失败</a-tag>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'cost_ms'">
          <span v-if="record.cost_ms">{{ formatCost(record.cost_ms) }}</span>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'publish_url'">
          <a v-if="record.publish_url" :href="record.publish_url" target="_blank" style="font-size: 12px">查看</a>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'created_at'">
          {{ formatTime(record.created_at) }}
        </template>
        <template v-else-if="column.key === 'action'">
          <a-button
            v-if="record.candidate_id"
            size="small"
            type="link"
            @click="$router.push(`/publish/groups/${record.candidate_id}`)"
          >
            详情
          </a-button>
        </template>
      </template>

      <template #expandedRowRender="{ record }">
        <div v-if="record.error_message || record.skip_reason" style="padding: 4px 0">
          <a-alert
            v-if="record.error_message"
            type="error"
            show-icon
            :message="record.error_message"
            style="margin-bottom: 4px"
          />
          <a-alert
            v-if="record.skip_reason"
            type="warning"
            show-icon
            :message="`跳过原因: ${record.skip_reason}`"
          />
        </div>
        <div v-if="record.logs" style="padding: 4px 0">
          <a-typography-text type="secondary" style="font-size: 12px; white-space: pre-wrap; font-family: monospace">{{ record.logs }}</a-typography-text>
        </div>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import { ReloadOutlined, DownloadOutlined } from '@ant-design/icons-vue'
import { publishApi } from '@/api/publish'
import type { PublishResultRecord } from '@/api/types'
import { formatTime } from '@/utils/format'

const STORAGE_KEY = 'publish_logs_filters'

interface PersistedFilters {
  search?: string
  status?: string
  trigger?: string
  page_size?: number
  date_start?: string
  date_end?: string
}

function loadPersistedFilters(): PersistedFilters {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw) as PersistedFilters
  } catch { /* silent */ }
  return {}
}

function persistFilters() {
  const data: PersistedFilters = {
    search: searchTarget.value || undefined,
    status: statusFilter.value,
    trigger: triggerFilter.value,
    page_size: pagination.value.pageSize,
    date_start: dateRange.value?.[0]?.format('YYYY-MM-DD'),
    date_end: dateRange.value?.[1]?.format('YYYY-MM-DD'),
  }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
  } catch { /* silent */ }
}

const persisted = loadPersistedFilters()

const loading = ref(false)
const tableData = ref<PublishResultRecord[]>([])
const searchTarget = ref(persisted.search || '')
const statusFilter = ref<string | undefined>(persisted.status)
const triggerFilter = ref<string | undefined>(persisted.trigger)
const dateRange = ref<[Dayjs, Dayjs] | undefined>(
  persisted.date_start && persisted.date_end
    ? [dayjs(persisted.date_start), dayjs(persisted.date_end)]
    : undefined
)
const autoRefresh = ref(true)

const pagination = ref({
  current: 1,
  pageSize: persisted.page_size || 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`,
})

const columns = [
  { title: '触发', key: 'trigger', width: 80 },
  { title: '源站', key: 'source_site', width: 100 },
  { title: '目标站', key: 'target_site', width: 100 },
  { title: '种子ID', dataIndex: 'torrent_id', key: 'torrent_id', width: 90 },
  { title: '标题', key: 'title', ellipsis: true },
  { title: '状态', key: 'status', width: 80 },
  { title: '加种', key: 'seeded', width: 80 },
  { title: '耗时', key: 'cost_ms', width: 80 },
  { title: '链接', key: 'publish_url', width: 60 },
  { title: '时间', key: 'created_at', width: 150 },
  { title: '操作', key: 'action', width: 70, fixed: 'right' as const },
]

let pollTimer: ReturnType<typeof setInterval> | null = null
let pollRefreshing = false

async function fetchData(silent = false) {
  if (!silent) loading.value = true
  if (pollRefreshing) return
  pollRefreshing = true
  try {
    const params: Record<string, unknown> = {
      page: pagination.value.current,
      pageSize: pagination.value.pageSize,
    }
    if (statusFilter.value) params.status = statusFilter.value
    if (triggerFilter.value) params.trigger = triggerFilter.value
    if (searchTarget.value) params.target_site = searchTarget.value
    if (dateRange.value && dateRange.value[0] && dateRange.value[1]) {
      params.start_date = dateRange.value[0].format('YYYY-MM-DD')
      params.end_date = dateRange.value[1].format('YYYY-MM-DD')
    }
    const resp = await publishApi.listResults(params as Parameters<typeof publishApi.listResults>[0])
    const data = resp.data?.data
    if (data) {
      tableData.value = data.items as PublishResultRecord[]
      pagination.value.total = data.total
    }
  } catch {
    // silent
  } finally {
    loading.value = false
    pollRefreshing = false
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    if (autoRefresh.value && !pollRefreshing) {
      fetchData(true)
    }
  }, 3000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(autoRefresh, (on) => {
  if (on) startPolling()
})

watch([searchTarget, statusFilter, triggerFilter, dateRange], persistFilters)

function onFilterChange() {
  pagination.value.current = 1
  fetchData()
}

function onTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) pagination.value.current = pag.current
  if (pag.pageSize) pagination.value.pageSize = pag.pageSize
  persistFilters()
  fetchData()
}

function statusColor(status: string): string {
  const map: Record<string, string> = {
    completed: 'green',
    edited: 'green',
    failed: 'red',
    skipped: 'orange',
    exists: 'gold',
    publishing: 'blue',
  }
  return map[status] || 'default'
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    completed: '成功',
    edited: '已编辑',
    failed: '失败',
    skipped: '跳过',
    exists: '已存在',
    publishing: '发布中',
  }
  return map[status] || status
}

function formatCost(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}min`
}

function triggerColor(trigger: string): string {
  const map: Record<string, string> = {
    manual: 'blue',
    batch: 'purple',
    reseed: 'cyan',
    wizard: 'geekblue',
  }
  return map[trigger] || 'default'
}

function triggerLabel(trigger: string): string {
  const map: Record<string, string> = {
    manual: '手动',
    batch: '批量',
    reseed: '辅种',
    wizard: '向导',
  }
  return map[trigger] || trigger || '-'
}

onMounted(() => {
  fetchData()
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})

function exportCSV() {
  const headers = ['时间', '源站', '目标站', '标题', '状态', '耗时', '链接', '错误']
  const rows = tableData.value.map((r: PublishResultRecord) => [
    formatTime(r.created_at),
    r.source_site || '',
    r.target_site || '',
    (r.title || r.subtitle || '').replace(/"/g, '""'),
    r.status || '',
    r.cost_ms ? formatCost(r.cost_ms) : '',
    r.publish_url || '',
    (r.error_message || '').replace(/"/g, '""').replace(/\n/g, ' '),
  ])
  const csv = [headers, ...rows].map(row => row.map(cell => `"${cell}"`).join(',')).join('\n')
  const blob = new Blob(['\ufeff' + csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `publish-logs-${new Date().toISOString().slice(0, 10)}.csv`
  link.click()
  URL.revokeObjectURL(url)
}
</script>
