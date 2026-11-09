<template>
  <div>
    <a-form layout="vertical" style="max-width: 600px">
      <a-form-item label="CookieCloud 模式">
        <a-radio-group v-model:value="form.mode" @change="updateBuiltinUrl">
          <a-radio-button value="remote">远程服务器</a-radio-button>
          <a-radio-button value="builtin">本地内置</a-radio-button>
        </a-radio-group>
      </a-form-item>

      <template v-if="form.mode === 'builtin'">
        <a-alert type="info" style="margin-bottom: 16px" show-icon>
          <template #message>
            浏览器插件配置地址：<a-typography-text copyable>{{ builtinUrl }}</a-typography-text>
          </template>
        </a-alert>
      </template>

      <a-form-item v-if="form.mode === 'remote'" :label="t('cookiecloud.serverUrl')">
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
        <a-switch v-model:checked="form.syncEnabled" />
      </a-form-item>
      <a-form-item v-if="form.syncEnabled" :label="t('cookiecloud.syncInterval')">
        <a-input-number v-model:value="form.syncInterval" :min="10" :max="1440" style="width: 200px" />
      </a-form-item>
      <a-form-item>
        <a-space>
          <a-button :loading="testing" @click="handleTest">{{ t('site.testConnection') }}</a-button>
          <a-button :loading="syncing" @click="handleSync">{{ t('cookiecloud.syncNow') }}</a-button>
        </a-space>
      </a-form-item>
    </a-form>

    <a-divider style="max-width: 600px" />

    <a-card :title="t('cookiecloud.syncHistory')" style="max-width: 900px" size="small">
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
// §59.497: CookieCloud 模块子组件（自 /cookiecloud 独立页迁入 /settings——
// 侧边栏省空间+settings 实功能化）。保存并入 settings 整页保存
// （defineExpose save），测试/立即同步为模块内独立操作。
// API 层字段 camelCase（cookiecloud_handler.go struct JSON tag）。
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { cookiecloudApi } from '@/api/cookiecloud'
import { formatTime, formatDurationNs } from '@/utils/format'

const { t } = useI18n()

const testing = ref(false)
const syncing = ref(false)
const historyLoading = ref(false)
const histories = ref<Record<string, unknown>[]>([])

const form = reactive({
  mode: 'remote' as 'remote' | 'builtin',
  serverUrl: '',
  uuid: '',
  password: '',
  cryptoType: 'legacy',
  syncEnabled: false,
  syncInterval: 60,
})

const builtinUrl = ref('')

function updateBuiltinUrl() {
  const proto = window.location.protocol
  const host = window.location.host
  builtinUrl.value = `${proto}//${host}/cookiecloud`
}

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

async function load() {
  try {
    const resp = await cookiecloudApi.getConfig()
    const d = resp.data?.data
    if (d) {
      form.mode = (d.mode as 'remote' | 'builtin') || 'remote'
      form.serverUrl = d.serverUrl || ''
      form.uuid = d.uuid || ''
      form.password = d.password || ''
      form.cryptoType = d.cryptoType || 'legacy'
      form.syncEnabled = Boolean(d.syncEnabled)
      form.syncInterval = d.syncInterval || 60
    }
  } catch {
    // 空配置形态——保持默认
  }
  updateBuiltinUrl()
}

// 整页保存时由父组件调用（§59.497：并入 settings 整页保存——失败
// 由父组件 catch 提示区分·不阻断其它设置）
async function save() {
  await cookiecloudApi.saveConfig({
    mode: form.mode,
    serverUrl: form.serverUrl,
    uuid: form.uuid,
    password: form.password,
    cryptoType: form.cryptoType,
    syncEnabled: form.syncEnabled,
    syncInterval: form.syncInterval,
  })
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
    message.success(t('cookiecloud.syncCompleted', { n: resp.data?.data?.synced_sites || 0 }))
    loadHistory()
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    message.error(err?.response?.data?.message || t('cookiecloud.syncFailed'))
  } finally {
    syncing.value = false
  }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    const resp = await cookiecloudApi.listHistory({
      page: historyPagination.current,
      size: historyPagination.pageSize,
    })
    const d = resp.data?.data
    histories.value = (d?.items || []) as Record<string, unknown>[]
    historyPagination.total = d?.total || 0
  } catch {
    histories.value = []
  } finally {
    historyLoading.value = false
  }
}

function handleHistoryTableChange(pag: { current: number; pageSize: number }) {
  historyPagination.current = pag.current
  historyPagination.pageSize = pag.pageSize
  loadHistory()
}

defineExpose({ load, save })

onMounted(() => {
  load()
  loadHistory()
})
</script>
