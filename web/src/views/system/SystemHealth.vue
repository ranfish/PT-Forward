<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="8">
        <a-card title="服务状态" size="small">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="状态">
              <a-tag :color="health.status === 'healthy' ? 'green' : 'red'">{{ health.status || '-' }}</a-tag>
            </a-descriptions-item>
            <a-descriptions-item label="版本">{{ health.version || '-' }}</a-descriptions-item>
            <a-descriptions-item label="运行时间">{{ health.uptime || '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :span="8">
        <a-card title="数据库" size="small">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="状态">
              <a-tag :color="health.db_status === 'ok' ? 'green' : 'red'">{{ health.db_status || '-' }}</a-tag>
            </a-descriptions-item>
            <a-descriptions-item label="信息">{{ health.db_message || '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :span="8">
        <a-card title="下载器" size="small">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="已连接">{{ health.connected_clients ?? '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>

    <a-card title="运行信息" size="small" style="margin-bottom: 16px">
      <a-descriptions :column="2" size="small" v-if="info">
        <a-descriptions-item label="Go 版本">{{ info.go_version || '-' }}</a-descriptions-item>
        <a-descriptions-item label="操作系统">{{ info.os || '-' }}/{{ info.arch || '-' }}</a-descriptions-item>
        <a-descriptions-item label="CPU 核数">{{ info.cpu_count || '-' }}</a-descriptions-item>
        <a-descriptions-item label="Goroutines">{{ info.goroutines || '-' }}</a-descriptions-item>
        <a-descriptions-item label="内存使用">{{ formatBytes(info.mem_alloc) }}</a-descriptions-item>
        <a-descriptions-item label="堆内存">{{ formatBytes(info.heap_alloc) }}</a-descriptions-item>
      </a-descriptions>
    </a-card>

    <div style="text-align: right; margin-bottom: 16px">
      <a-button @click="fetchAll">刷新</a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { systemApi } from '@/api/system'

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
