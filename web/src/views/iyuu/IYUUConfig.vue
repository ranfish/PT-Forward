<template>
  <div>
    <a-page-header title="IYUU 辅种" sub-title="全局辅种服务配置" />

    <a-spin :spinning="loading">
      <a-form :model="form" layout="vertical" style="max-width: 600px">
        <a-form-item label="API Token" :rules="[{ required: true, message: '请输入 IYUU Token' }]">
          <a-input v-model:value="form.token" placeholder="IYUU API Token" />
        </a-form-item>
        <a-form-item label="启用">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" :loading="saving" @click="handleSave">保存</a-button>
            <a-button :loading="testing" @click="handleTest">测试连接</a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </a-spin>

    <a-divider />

    <a-card title="站点映射" :loading="sitesLoading">
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
            <a-badge :status="record.Enabled ? 'success' : 'default'" :text="record.Enabled ? '启用' : '禁用'" />
          </template>
        </template>
      </a-table>
      <a-empty v-else description="保存配置后查看站点列表" />
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { iyuuApi } from '@/api/iyuu'

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
    message.success('保存成功')
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
    message.success('连接测试成功')
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
