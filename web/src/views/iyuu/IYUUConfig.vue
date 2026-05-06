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
      <a-empty :description="t('iyuu.viewAfterSave')" />
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
const sitesLoading = ref(false)
const sites = ref<any[]>([])

const form = reactive({
  token: '',
  enabled: false,
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
  } catch {
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await iyuuApi.saveConfig(form)
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

onMounted(() => {
  fetchConfig()
})
</script>
