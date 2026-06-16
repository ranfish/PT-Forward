<template>
  <div>
    <a-page-header :title="t('iyuu.title')" :sub-title="t('iyuu.subtitle')" />

    <a-spin :spinning="loading">
      <a-form :model="form" layout="vertical" style="max-width: 600px">
        <a-form-item :label="t('iyuu.apiToken')" :rules="[{ required: true, message: t('iyuu.tokenRequired') }]">
          <a-input v-model:value="form.token" :placeholder="t('iyuu.tokenPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('common.enable')">
          <a-switch v-model:checked="form.enabled" @change="handleSave" />
        </a-form-item>
        <a-form-item :label="t('iyuu.baseUrl')">
          <a-input v-model:value="form.baseURL" placeholder="https://2025.iyuu.cn" />
        </a-form-item>
        <a-form-item :label="t('iyuu.vip')">
          <a-switch v-model:checked="form.isVIP" @change="handleSave" />
        </a-form-item>
        <a-form-item :label="t('iyuu.version')">
          <a-input v-model:value="form.version" placeholder="1.0.0" />
        </a-form-item>
        <a-form-item :label="t('iyuu.requestTimeout')">
          <a-input-number v-model:value="form.requestTimeoutSec" :min="5" :max="300" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('iyuu.syncIntervalHours')">
          <a-input-number v-model:value="form.syncIntervalHours" :min="1" :max="168" style="width: 100%" />
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" :loading="saving" @click="handleSave">{{ t('common.save') }}</a-button>
            <a-button :loading="testing" @click="handleTest">{{ t('site.testConnection') }}</a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </a-spin>

    <a-divider />

    <a-card :title="t('iyuu.siteMapping')" :loading="sitesLoading">
      <template #extra>
        <a-button size="small" :loading="syncing" @click="handleSyncSites">{{ t('iyuu.syncSites') }}</a-button>
      </template>
      <a-table
        v-if="sites.length > 0"
        :columns="siteColumns"
        :data-source="sites"
        :pagination="false"
        row-key="IYUUSid"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'Enabled'">
            <a-badge :status="record.Enabled ? 'success' : 'default'" :text="record.Enabled ? t('common.enabled') : t('common.disabled')" />
          </template>
        </template>
      </a-table>
      <a-empty v-else :description="t('iyuu.viewAfterSave')" />
    </a-card>

    <a-divider />

    <a-card :title="t('iyuu.manualQuery')" style="margin-top: 16px">
      <a-form layout="vertical">
        <a-form-item :label="t('iyuu.infoHashes')">
          <a-textarea v-model:value="queryHashes" :rows="3" :placeholder="t('iyuu.infoHashesPlaceholder')" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" :loading="querying" @click="handleQuery">{{ t('iyuu.query') }}</a-button>
        </a-form-item>
      </a-form>
      <a-table
        v-if="queryResults.length > 0"
        :columns="queryColumns"
        :data-source="queryResults"
        :pagination="false"
        row-key="sid"
        size="small"
        style="margin-top: 12px"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'torrents'">
            <span>{{ record.torrents?.length || 0 }}</span>
          </template>
        </template>
      </a-table>
      <a-empty v-else-if="queryExecuted" :description="t('common.noData')" style="margin-top: 12px" />
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { iyuuApi } from '@/api/iyuu'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const syncing = ref(false)
const sitesLoading = ref(false)
const sites = ref<Record<string, unknown>[]>([])

const form = reactive({
  token: '',
  enabled: false,
  baseURL: 'https://2025.iyuu.cn',
  isVIP: false,
  version: '1.0.0',
  requestTimeoutSec: 60,
  syncIntervalHours: 24,
})

const siteColumns = [
  { title: t('iyuu.sid'), dataIndex: 'IYUUSid', key: 'IYUUSid', width: 80 },
  { title: t('iyuu.siteName'), dataIndex: 'SiteName', key: 'SiteName' },
  { title: t('iyuu.siteDomain'), dataIndex: 'SiteDomain', key: 'SiteDomain' },
  { title: t('common.status'), key: 'Enabled', width: 80 },
]

const queryHashes = ref('')
const querying = ref(false)
const queryResults = ref<Record<string, unknown>[]>([])
const queryExecuted = ref(false)

const queryColumns = [
  { title: t('iyuu.sid'), dataIndex: 'sid', key: 'sid', width: 80 },
  { title: t('iyuu.siteName'), dataIndex: 'site_name', key: 'site_name' },
  { title: t('common.status'), dataIndex: 'status', key: 'status', width: 100 },
  { title: t('common.title'), key: 'torrents', width: 100 },
]

async function handleQuery() {
  const hashes = queryHashes.value.split(/[\n,;\s]+/).map(h => h.trim()).filter(Boolean)
  if (!hashes.length) {
    message.warning(t('iyuu.infoHashesRequired'))
    return
  }
  querying.value = true
  queryExecuted.value = false
  try {
    const resp = await iyuuApi.query({ infoHashes: hashes })
    queryResults.value = resp.data?.data || []
    queryExecuted.value = true
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    querying.value = false
  }
}

async function fetchConfig() {
  loading.value = true
  try {
    const resp = await iyuuApi.getConfig()
    const data = resp.data?.data || {}
    form.token = data.token || ''
    form.enabled = data.enabled || false
    form.baseURL = data.baseUrl ?? 'https://2025.iyuu.cn'
    form.isVIP = data.isVip ?? false
    form.version = data.version ?? '1.0.0'
    form.requestTimeoutSec = data.requestTimeoutMs ? Math.round(data.requestTimeoutMs / 1000) : 60
    form.syncIntervalHours = data.syncIntervalHours ?? 24
  } catch {
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await iyuuApi.saveConfig({
      token: form.token,
      baseUrl: form.baseURL,
      enabled: form.enabled,
      isVip: form.isVIP,
      version: form.version,
      requestTimeoutMs: form.requestTimeoutSec * 1000,
      syncIntervalHours: form.syncIntervalHours,
    })
    message.success(t('common.saveSuccess'))
    fetchSites()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    await iyuuApi.test()
    message.success(t('cookiecloud.connectionTestSuccess'))
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    testing.value = false
  }
}

async function fetchSites() {
  sitesLoading.value = true
  try {
    const resp = await iyuuApi.listSites()
    const data = resp.data?.data
    sites.value = data?.items || data || []
  } catch {
  } finally {
    sitesLoading.value = false
  }
}

async function handleSyncSites() {
  syncing.value = true
  try {
    await iyuuApi.syncSites()
    message.success(t('common.operationSuccess'))
    fetchSites()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    syncing.value = false
  }
}

onMounted(() => {
  fetchConfig()
  fetchSites()
})
</script>
