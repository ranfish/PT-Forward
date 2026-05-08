<template>
  <div>
    <a-page-header title="IYUU 辅种" :sub-title="t('iyuu.subtitle')" />

    <a-spin :spinning="loading">
      <a-form :model="form" layout="vertical" style="max-width: 600px">
        <a-form-item :label="t('iyuu.apiToken')" :rules="[{ required: true, message: t('iyuu.tokenRequired') }]">
          <a-input v-model:value="form.token" placeholder="IYUU API Token" />
        </a-form-item>
        <a-form-item :label="t('common.enable')">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
        <a-form-item label="Base URL">
          <a-input v-model:value="form.baseURL" placeholder="https://2025.iyuu.cn" />
        </a-form-item>
        <a-form-item label="VIP">
          <a-switch v-model:checked="form.isVIP" />
        </a-form-item>
        <a-form-item label="Version">
          <a-input v-model:value="form.version" placeholder="1.0.0" />
        </a-form-item>
        <a-form-item label="Request Timeout (s)">
          <a-input-number v-model:value="form.requestTimeoutSec" :min="5" :max="300" style="width: 100%" />
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
        <a-button size="small" :loading="syncing" @click="handleSyncSites">{{ t('cookiecloud.syncNow') }}</a-button>
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
const sites = ref<any[]>([])

const form = reactive({
  token: '',
  enabled: false,
  baseURL: 'https://2025.iyuu.cn',
  isVIP: false,
  version: '1.0.0',
  requestTimeoutSec: 60,
})

const siteColumns = [
  { title: 'SID', dataIndex: 'IYUUSid', key: 'IYUUSid', width: 80 },
  { title: '站点名', dataIndex: 'SiteName', key: 'SiteName' },
  { title: '域名', dataIndex: 'SiteDomain', key: 'SiteDomain' },
  { title: '状态', key: 'Enabled', width: 80 },
]

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
      requestTimeoutSec: form.requestTimeoutSec,
    })
    message.success(t('common.saveSuccess'))
    fetchSites()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    await iyuuApi.test()
    message.success(t('cookiecloud.connectionTestSuccess'))
  } catch (e: any) {
    message.error(e.message)
  } finally {
    testing.value = false
  }
}

async function fetchSites() {
  sitesLoading.value = true
  try {
    const resp = await iyuuApi.listSites()
    sites.value = resp.data?.data?.items || resp.data?.data || []
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
  } catch (e: any) {
    message.error(e.message)
  } finally {
    syncing.value = false
  }
}

onMounted(() => {
  fetchConfig()
  fetchSites()
})
</script>
