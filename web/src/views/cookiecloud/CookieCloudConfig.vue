<template>
  <div>
    <a-page-header :title="t('cookiecloud.title')" :sub-title="t('cookiecloud.subtitle')" />

    <a-spin :spinning="loading">
      <a-form :model="form" layout="vertical" style="max-width: 600px">
        <a-form-item :label="t('cookiecloud.serverUrl')">
          <a-input v-model:value="form.serverUrl" placeholder="https://cookiecloud.example.com" />
        </a-form-item>
        <a-form-item :label="t('cookiecloud.uuid')">
          <a-input v-model:value="form.uuid" :placeholder="t('cookiecloud.uuidPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('common.password')">
          <a-input-password v-model:value="form.password" :placeholder="t('cookiecloud.passwordPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('cookiecloud.cryptoType')">
          <a-select v-model:value="form.cryptoType" style="width: 200px">
            <a-select-option value="legacy">{{ t('cookiecloud.cryptoLegacy') }}</a-select-option>
            <a-select-option value="aes-256-gcm">AES-256-GCM</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('cookiecloud.autoSync')">
          <a-switch v-model:checked="form.syncEnabled" @change="handleSave" />
        </a-form-item>
        <a-form-item :label="t('cookiecloud.syncInterval')">
          <a-input-number v-model:value="form.syncInterval" :min="10" :max="1440" style="width: 200px" />
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" :loading="saving" @click="handleSave">{{ t('common.save') }}</a-button>
            <a-button :loading="testing" @click="handleTest">{{ t('site.testConnection') }}</a-button>
            <a-button :loading="syncing" @click="handleSync">{{ t('cookiecloud.syncNow') }}</a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </a-spin>

    <a-divider />

    <a-card :title="t('cookiecloud.syncHistory')">
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
              :text="record.status === 'completed' ? t('cookiecloud.completed') : record.status === 'failed' ? t('cookiecloud.failed') : t('cookiecloud.running')"
            />
          </template>
          <template v-if="column.key === 'sync_duration'">
            {{ formatDurationNs(record.sync_duration) }}
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
import { useI18n } from 'vue-i18n'
import { cookiecloudApi } from '@/api/cookiecloud'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const syncing = ref(false)
const historyLoading = ref(false)
const histories = ref<Record<string, unknown>[]>([])

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
  { title: t('common.time'), key: 'created_at', width: 180 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('cookiecloud.syncedSites'), dataIndex: 'synced_sites', width: 100 },
  { title: t('cookiecloud.skippedSites'), dataIndex: 'skipped_sites', width: 100 },
  { title: t('cookiecloud.duration'), key: 'sync_duration', width: 120 },
  { title: t('cookiecloud.errorMessage'), dataIndex: 'error_message', ellipsis: true },
]

import { formatTime, formatDurationNs } from '@/utils/format'

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
    message.success(t('common.saveSuccess'))
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    message.error(err?.response?.data?.message || t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    await cookiecloudApi.test()
    message.success(t('cookiecloud.connectionTestSuccess'))
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    message.error(err?.response?.data?.message || t('cookiecloud.connectionTestFailed'))
  } finally {
    testing.value = false
  }
}

async function handleSync() {
  syncing.value = true
  try {
    const resp = await cookiecloudApi.sync()
    const data = resp.data?.data || { synced: 0 }
    message.success(t('cookiecloud.syncCompleted', { count: data.synced || 0 }))
    fetchHistory()
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    message.error(err?.response?.data?.message || t('cookiecloud.syncFailed'))
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
    const data = resp.data?.data || { items: [], total: 0 }
    histories.value = (data.items || []) as Record<string, unknown>[]
    historyPagination.total = data.total || 0
  } catch {
  } finally {
    historyLoading.value = false
  }
}

function handleHistoryTableChange(pagination: { current: number; pageSize: number }) {
  historyPagination.current = pagination.current
  historyPagination.pageSize = pagination.pageSize
  fetchHistory()
}

onMounted(() => {
  fetchConfig()
  fetchHistory()
})
</script>
