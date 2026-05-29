<template>
  <div>
    <a-tabs v-model:active-key="activeTab">
      <a-tab-pane key="health" :tab="t('system.serviceStatus')">
        <a-row :gutter="16" style="margin-bottom: 16px">
          <a-col :span="8">
            <a-card :title="t('system.serviceStatus')" size="small">
              <a-descriptions :column="1" size="small">
                <a-descriptions-item :label="t('common.status')">
                  <a-tag :color="health.status === 'healthy' ? 'green' : 'red'">{{ health.status || '-' }}</a-tag>
                </a-descriptions-item>
                <a-descriptions-item :label="t('system.version')">{{ health.version || '-' }}</a-descriptions-item>
                <a-descriptions-item :label="t('system.uptime')">{{ health.uptime || '-' }}</a-descriptions-item>
              </a-descriptions>
            </a-card>
          </a-col>
          <a-col :span="8">
            <a-card :title="t('system.database')" size="small">
              <a-descriptions :column="1" size="small">
                <a-descriptions-item :label="t('common.status')">
                  <a-tag :color="health.database?.ok ? 'green' : 'red'">{{ health.database?.ok ? 'ok' : 'error' }}</a-tag>
                </a-descriptions-item>
                <a-descriptions-item :label="t('system.info')">{{ health.database?.message || '-' }}</a-descriptions-item>
              </a-descriptions>
            </a-card>
          </a-col>
          <a-col :span="8">
            <a-card :title="t('nav.downloaders')" size="small">
              <a-descriptions :column="1" size="small">
                <a-descriptions-item :label="t('system.connectedClients')">{{ health.downloaders?.connected ?? '-' }}</a-descriptions-item>
              </a-descriptions>
            </a-card>
          </a-col>
        </a-row>

        <a-card :title="t('system.runtimeInfo')" size="small" style="margin-bottom: 16px">
          <a-descriptions v-if="info" :column="2" size="small">
            <a-descriptions-item :label="t('system.goVersion')">{{ info.goVersion || '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('system.os')">{{ info.os || '-' }}/{{ info.arch || '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('system.cpuCount')">{{ info.cpuCount || '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('system.goroutines')">{{ info.goroutines || '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('system.memoryUsage')">{{ formatBytes(info.memAlloc) }}</a-descriptions-item>
            <a-descriptions-item :label="t('system.heapMemory')">{{ formatBytes(info.heapAlloc) }}</a-descriptions-item>
          </a-descriptions>
        </a-card>

        <div style="text-align: right">
          <a-button @click="fetchAll">{{ t('common.refresh') }}</a-button>
        </div>
      </a-tab-pane>

      <a-tab-pane key="logs" :tab="t('nav.logs')">
        <div style="margin-bottom: 12px; display: flex; justify-content: space-between">
          <a-space>
            <a-select v-model:value="logLevel" style="width: 120px" :placeholder="t('system.logLevel')" @change="fetchLogs">
              <a-select-option value="">{{ t('system.allLevels') }}</a-select-option>
              <a-select-option value="debug">DEBUG</a-select-option>
              <a-select-option value="info">INFO</a-select-option>
              <a-select-option value="warn">WARN</a-select-option>
              <a-select-option value="error">ERROR</a-select-option>
            </a-select>
            <a-input-number v-model:value="logLimit" :min="10" :max="1000" :step="50" style="width: 120px" />
          </a-space>
          <a-space>
            <a-button @click="fetchLogs">{{ t('common.refresh') }}</a-button>
            <a-popconfirm :title="t('system.clearLogsConfirm')" @confirm="clearLogs">
              <a-button danger>{{ t('system.clearLogs') }}</a-button>
            </a-popconfirm>
          </a-space>
        </div>
        <a-table
          :columns="logColumns"
          :data-source="logEntries"
          :loading="logsLoading"
          :pagination="{ pageSize: 50, showSizeChanger: true, showTotal: (total: number) => t('common.totalCount', { count: total }) }"
          row-key="_index"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'level'">
              <a-tag :color="logLevelColor(record.level)">{{ (record.level || '-').toString().toUpperCase() }}</a-tag>
            </template>
            <template v-if="column.key === 'ts'">
              {{ formatLogTime(record.ts) }}
            </template>
            <template v-if="column.key === 'msg'">
              <span style="font-family: monospace; font-size: 12px">{{ record.msg || record.message || '-' }}</span>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { systemApi } from '@/api/system'
import { formatBytes } from '@/utils/format'

interface HealthStatus {
  status?: string
  version?: string
  uptime?: string
  database?: { ok?: boolean; message?: string }
  downloaders?: { connected?: number }
}

interface RuntimeInfo {
  goVersion?: string
  os?: string
  arch?: string
  cpuCount?: number
  goroutines?: number
  memAlloc?: number
  heapAlloc?: number
}

const { t } = useI18n()
const activeTab = ref('health')
const health = ref<HealthStatus>({})
const info = ref<RuntimeInfo | null>(null)

const logsLoading = ref(false)
const logEntries = ref<Record<string, unknown>[]>([])
const logLevel = ref('')
const logLimit = ref(200)

const logColumns = [
  { title: t('common.time'), key: 'ts', width: 180 },
  { title: t('common.status'), key: 'level', width: 80 },
  { title: t('common.title'), key: 'msg' },
]

function logLevelColor(level: unknown) {
  const l = String(level || '').toLowerCase()
  if (l === 'error') return 'red'
  if (l === 'warn') return 'orange'
  if (l === 'info') return 'blue'
  if (l === 'debug') return 'default'
  return 'default'
}

function formatLogTime(ts: unknown) {
  if (!ts) return '-'
  if (typeof ts === 'string') return ts
  if (typeof ts === 'number') {
    if (ts > 1e12) return new Date(ts).toLocaleString()
    return new Date(ts * 1000).toLocaleString()
  }
  return String(ts)
}

async function fetchHealth() {
  try {
    const resp = await systemApi.health()
    health.value = resp.data?.data || {}
  } catch {
    health.value = { status: 'unhealthy' }
  }
}

async function fetchInfo() {
  try {
    const resp = await systemApi.info()
    info.value = resp.data?.data || null
  } catch {
    info.value = null
  }
}

async function fetchAll() {
  await Promise.all([fetchHealth(), fetchInfo()])
}

async function fetchLogs() {
  logsLoading.value = true
  try {
    const resp = await systemApi.listLogs({ level: logLevel.value || undefined, limit: logLimit.value })
    const data = resp.data?.data
    const items = (data?.items || []) as unknown as Record<string, unknown>[]
    logEntries.value = items.map((item: Record<string, unknown>, idx: number) => ({ ...item, _index: idx }))
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    logsLoading.value = false
  }
}

async function clearLogs() {
  try {
    await systemApi.clearLogs()
    message.success(t('common.operationSuccess'))
    logEntries.value = []
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

onMounted(() => {
  fetchAll()
})
</script>
