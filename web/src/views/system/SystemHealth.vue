<template>
  <div>
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
        <a-descriptions-item label="Goroutines">{{ info.goroutines || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('system.memoryUsage')">{{ formatBytes(info.memAlloc) }}</a-descriptions-item>
        <a-descriptions-item :label="t('system.heapMemory')">{{ formatBytes(info.heapAlloc) }}</a-descriptions-item>
      </a-descriptions>
    </a-card>

    <div style="text-align: right; margin-bottom: 16px">
      <a-button @click="fetchAll">{{ t('common.refresh') }}</a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
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
const health = ref<HealthStatus>({})
const info = ref<RuntimeInfo | null>(null)

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

onMounted(fetchAll)
</script>
