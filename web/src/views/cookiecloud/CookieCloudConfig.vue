<template>
  <div>
    <a-page-header title="CookieCloud" sub-title="浏览器 Cookie 自动同步" />

    <a-spin :spinning="loading">
      <a-form :model="form" layout="vertical" style="max-width: 600px">
        <a-form-item label="服务器地址">
          <a-input v-model:value="form.serverUrl" placeholder="https://cookiecloud.example.com" />
        </a-form-item>
        <a-form-item label="UUID">
          <a-input v-model:value="form.uuid" placeholder="CookieCloud UUID" />
        </a-form-item>
        <a-form-item label="密码">
          <a-input-password v-model:value="form.password" placeholder="CookieCloud 密码" />
        </a-form-item>
        <a-form-item label="加密方式">
          <a-select v-model:value="form.cryptoType" style="width: 200px">
            <a-select-option value="legacy">Legacy</a-select-option>
            <a-select-option value="aes-256-gcm">AES-256-GCM</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="自动同步">
          <a-switch v-model:checked="form.syncEnabled" />
        </a-form-item>
        <a-form-item label="同步间隔（分钟）">
          <a-input-number v-model:value="form.syncInterval" :min="10" :max="1440" style="width: 200px" />
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" :loading="saving" @click="handleSave">保存</a-button>
            <a-button :loading="testing" @click="handleTest">测试连接</a-button>
            <a-button :loading="syncing" @click="handleSync">立即同步</a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </a-spin>

    <a-divider />

    <a-card title="同步历史">
      <a-table
        :columns="historyColumns"
        :data-source="histories"
        :loading="historyLoading"
        :pagination="historyPagination"
        row-key="id"
        size="small"
        @change="handleHistoryTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <a-badge
              :status="record.status === 'completed' ? 'success' : record.status === 'failed' ? 'error' : 'processing'"
              :text="record.status === 'completed' ? '成功' : record.status === 'failed' ? '失败' : '运行中'"
            />
          </template>
          <template v-if="column.key === 'sync_duration'">
            {{ formatDuration(record.sync_duration) }}
          </template>
          <template v-if="column.key === 'created_at'">
            {{ formatTime(record.created_at) }}
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { cookiecloudApi } from '@/api/cookiecloud'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const syncing = ref(false)
const historyLoading = ref(false)
const histories = ref<any[]>([])

const form = reactive({
  serverUrl: '',
  uuid: '',
  password: '',
  cryptoType: 'legacy',
  syncEnabled: false,
  syncInterval: 60,
})

const historyPagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showSizeChanger: true,
})

const historyColumns = [
  { title: '时间', key: 'created_at', width: 180 },
  { title: '状态', key: 'status', width: 100 },
  { title: '同步站点', dataIndex: 'synced_sites', width: 100 },
  { title: '跳过站点', dataIndex: 'skipped_sites', width: 100 },
  { title: '耗时', key: 'sync_duration', width: 120 },
  { title: '错误信息', dataIndex: 'error_message', ellipsis: true },
]

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

function formatDuration(ns: number) {
  if (!ns) return '-'
  const ms = ns / 1000000
  if (ms < 1000) return `${ms.toFixed(0)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

async function fetchConfig() {
  loading.value = true
  try {
    const resp = await cookiecloudApi.getConfig()
    const data = resp.data?.data || {}
    form.serverUrl = data.serverUrl || ''
    form.uuid = data.uuid || ''
    form.password = ''
    form.cryptoType = data.cryptoType || 'legacy'
    form.syncEnabled = data.syncEnabled || false
    form.syncInterval = data.syncInterval || 60
  } catch {
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await cookiecloudApi.saveConfig(form)
    message.success('保存成功')
  } catch (e: any) {
    message.error(e?.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    await cookiecloudApi.test()
    message.success('连接测试成功')
  } catch (e: any) {
    message.error(e?.response?.data?.message || '连接测试失败')
  } finally {
    testing.value = false
  }
}

async function handleSync() {
  syncing.value = true
  try {
    const resp = await cookiecloudApi.sync()
    const data = resp.data?.data || {}
    message.success(`同步完成：${data.synced_sites || 0} 个站点已同步`)
    fetchHistory()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '同步失败')
  } finally {
    syncing.value = false
  }
}

async function fetchHistory() {
  historyLoading.value = true
  try {
    const resp = await cookiecloudApi.listHistory({
      page: historyPagination.current,
      size: historyPagination.pageSize,
    })
    const data = resp.data?.data || {}
    histories.value = data.items || []
    historyPagination.total = data.total || 0
  } catch {
  } finally {
    historyLoading.value = false
  }
}

function handleHistoryTableChange(pagination: any) {
  historyPagination.current = pagination.current
  historyPagination.pageSize = pagination.pageSize
  fetchHistory()
}

onMounted(() => {
  fetchConfig()
  fetchHistory()
})
</script>
