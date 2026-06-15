<template>
  <div>
    <a-page-header :title="t('cloudFp.title')" :sub-title="t('cloudFp.subtitle')" />

    <a-spin :spinning="loading">
      <a-form :model="form" layout="vertical" style="max-width: 600px">
        <a-form-item :label="t('cloudFp.baseUrl')">
          <a-input v-model:value="form.base_url" placeholder="http://127.0.0.1:8766" />
        </a-form-item>
        <a-form-item :label="t('cloudFp.apiToken')">
          <a-input-password v-model:value="form.api_token" placeholder="***" />
        </a-form-item>
        <a-form-item :label="t('common.enable')">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
        <a-form-item :label="t('cloudFp.requestTimeout')">
          <a-input-number v-model:value="form.request_timeout_sec" :min="5" :max="60" style="width: 100%" />
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

    <a-card :title="t('cloudFp.statusTitle')">
      <a-descriptions :column="1" size="small">
        <a-descriptions-item :label="t('common.status')">
          <a-badge v-if="status?.enabled" status="success" :text="t('common.enabled')" />
          <a-badge v-else status="default" :text="t('common.disabled')" />
        </a-descriptions-item>
        <a-descriptions-item :label="t('cloudFp.breaker')">
          <a-badge v-if="status?.breaker_open" status="error" :text="t('cloudFp.breakerOpen')" />
          <a-badge v-else-if="status?.enabled" status="success" :text="t('cloudFp.breakerClosed')" />
          <a-badge v-else status="default" :text="t('common.disabled')" />
        </a-descriptions-item>
      </a-descriptions>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { cloudFpApi } from '@/api/cloudfp'
import type { CloudFPConfig } from '@/api/types'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const status = ref<{ enabled: boolean; breaker_open: boolean; base_url: string } | null>(null)

const form = reactive<CloudFPConfig>({
  id: 0,
  base_url: 'http://127.0.0.1:8766',
  api_token: '',
  enabled: false,
  request_timeout_sec: 10,
})

async function fetchConfig() {
  loading.value = true
  try {
    const resp = await cloudFpApi.getConfig()
    const data = resp.data?.data
    if (data) {
      form.id = data.id || 0
      form.base_url = data.base_url || 'http://127.0.0.1:8766'
      form.api_token = data.api_token || ''
      form.enabled = data.enabled || false
      form.request_timeout_sec = data.request_timeout_sec || 10
    }
  } catch {
  } finally {
    loading.value = false
  }
}

async function fetchStatus() {
  try {
    const resp = await cloudFpApi.status()
    status.value = resp.data?.data || null
  } catch {
  }
}

async function handleSave() {
  saving.value = true
  try {
    await cloudFpApi.saveConfig({
      base_url: form.base_url,
      api_token: form.api_token,
      enabled: form.enabled,
      request_timeout_sec: form.request_timeout_sec,
    })
    message.success(t('common.saveSuccess'))
    fetchStatus()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    await cloudFpApi.test()
    message.success(t('cookiecloud.connectionTestSuccess'))
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  fetchConfig()
  fetchStatus()
})
</script>
