<template>
  <div>
    <a-page-header :title="subscription.name || t('subscription.subscriptionDetail')" @back="$router.push('/subscriptions')" />

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('common.name')">{{ subscription.name }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.site')">{{ subscription.siteName }}</a-descriptions-item>
        <a-descriptions-item :label="t('subscription.url')" :span="2">{{ (subscription.urls || []).join(', ') }}</a-descriptions-item>
        <a-descriptions-item label="Cron">{{ subscription.cron }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.status')">
          <a-badge :status="subscription.enabled ? 'success' : 'default'" :text="subscription.enabled ? t('common.enabled') : t('common.disabled')" />
        </a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ subscription.created_at || subscription.createdAt || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-tabs v-model:activeKey="activeTab">
        <a-tab-pane key="config" :tab="t('subscription.config')">
          <a-form :model="configForm" layout="vertical" style="max-width: 600px">
            <a-form-item :label="t('common.name')">
              <a-input v-model:value="configForm.name" />
            </a-form-item>
            <a-form-item :label="t('subscription.url')">
              <a-input v-model:value="configForm.url" />
            </a-form-item>
            <a-form-item :label="t('subscription.fetchInterval')">
              <a-input-number v-model:value="configForm.interval" :min="1" style="width: 100%" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" @click="saveConfig">{{ t('common.saveConfig') }}</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>
        <a-tab-pane key="dryrun" :tab="t('subscription.dryrun')">
          <a-button type="primary" @click="runDryrun" :loading="dryrunLoading" style="margin-bottom: 16px">
            {{ t('subscription.runDryrun') }}
          </a-button>
          <a-table
            :columns="dryrunColumns"
            :data-source="dryrunResults"
            :pagination="false"
            row-key="title"
            size="small"
          />
        </a-tab-pane>
        <a-tab-pane key="history" :tab="t('subscription.fetchHistory')">
          <a-table
            :columns="historyColumns"
            :data-source="history"
            :loading="historyLoading"
            :pagination="{ pageSize: 20 }"
            row-key="id"
            size="small"
          />
        </a-tab-pane>
      </a-tabs>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { subscriptionsApi } from '@/api/subscriptions'

const { t } = useI18n()

const route = useRoute()
const id = Number(route.params.id)

const loading = ref(false)
const dryrunLoading = ref(false)
const historyLoading = ref(false)
const subscription = ref<any>({})
const dryrunResults = ref<any[]>([])
const history = ref<any[]>([])
const activeTab = ref('config')

const configForm = reactive({ name: '', url: '', interval: 15 })

const dryrunColumns = [
  { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '大小', dataIndex: 'size', key: 'size', width: 100 },
  { title: '匹配', dataIndex: 'matched', key: 'matched', width: 80 },
  { title: '原因', dataIndex: 'reason', key: 'reason' },
]

const historyColumns = [
  { title: '时间', dataIndex: 'fetchedAt', key: 'fetchedAt', width: 180 },
  { title: '新种子数', dataIndex: 'newCount', key: 'newCount', width: 100 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
]

async function fetchSubscription() {
  loading.value = true
  try {
    const resp = await subscriptionsApi.get(id)
    subscription.value = resp.data.data || {}
    Object.assign(configForm, { name: subscription.value.name, url: subscription.value.url, interval: subscription.value.interval || 15 })
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  try {
    await subscriptionsApi.update(id, configForm)
    message.success(t('common.configSaved'))
    fetchSubscription()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function runDryrun() {
  dryrunLoading.value = true
  try {
    message.info(t('subscription.dryrunInDevelopment'))
  } finally {
    dryrunLoading.value = false
  }
}

async function fetchHistory() {
  historyLoading.value = true
  try {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

onMounted(() => {
  fetchSubscription()
  fetchHistory()
})
</script>
