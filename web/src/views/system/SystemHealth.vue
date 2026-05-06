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
              <a-tag :color="health.db_status === 'ok' ? 'green' : 'red'">{{ health.db_status || '-' }}</a-tag>
            </a-descriptions-item>
            <a-descriptions-item :label="t('system.info')">{{ health.db_message || '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :span="8">
        <a-card :title="t('nav.downloaders')" size="small">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item :label="t('system.connectedClients')">{{ health.connected_clients ?? '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('system.runtimeInfo')" size="small" style="margin-bottom: 16px">
      <a-descriptions :column="2" size="small" v-if="info">
        <a-descriptions-item :label="t('system.goVersion')">{{ info.go_version || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('system.os')">{{ info.os || '-' }}/{{ info.arch || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('system.cpuCount')">{{ info.cpu_count || '-' }}</a-descriptions-item>
        <a-descriptions-item label="Goroutines">{{ info.goroutines || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('system.memoryUsage')">{{ formatBytes(info.mem_alloc) }}</a-descriptions-item>
        <a-descriptions-item :label="t('system.heapMemory')">{{ formatBytes(info.heap_alloc) }}</a-descriptions-item>
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

const { t } = useI18n()

const health = ref<any>({})
const info = ref<any>(null)

function formatBytes(bytes?: number) {
  if (!bytes) return '-'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
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

onMounted(fetchAll)
</script>
