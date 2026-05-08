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
        <a-descriptions-item :label="t('common.createdAt')">{{ subscription.createdAt || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-tabs v-model:activeKey="activeTab">
        <a-tab-pane key="config" :tab="t('subscription.config')">
          <a-form :model="configForm" layout="vertical" style="max-width: 600px">
            <a-form-item :label="t('common.name')">
              <a-input v-model:value="configForm.name" />
            </a-form-item>
            <a-form-item :label="t('subscription.url')">
              <a-textarea v-model:value="configForm.urls" :rows="3" placeholder="每行一个 RSS URL" />
            </a-form-item>
            <a-form-item label="Cron">
              <a-input v-model:value="configForm.cron" placeholder="如 */5 * * * *" />
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
        <a-tab-pane key="rules" :tab="t('subscription.rules')">
          <a-form layout="vertical" style="max-width: 600px">
            <a-form-item :label="t('subscription.rulesJson')">
              <a-textarea v-model:value="rulesJSON" :rows="10" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" @click="saveRules" :loading="rulesSaving">{{ t('common.save') }}</a-button>
            </a-form-item>
          </a-form>
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

const configForm = reactive({ name: '', urls: '', cron: '' })
const rulesJSON = ref('{}')
const rulesSaving = ref(false)

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
    Object.assign(configForm, {
      name: subscription.value.name,
      urls: Array.isArray(subscription.value.urls) ? subscription.value.urls.join('\n') : (subscription.value.urls || ''),
      cron: subscription.value.cron || '',
    })
    if (subscription.value.rules) {
      rulesJSON.value = JSON.stringify(subscription.value.rules, null, 2)
    }
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  try {
    const payload: any = { ...configForm }
    if (typeof payload.urls === 'string') {
      payload.urls = payload.urls.split('\n').map((u: string) => u.trim()).filter(Boolean)
    }
    await subscriptionsApi.update(id, payload)
    message.success(t('common.configSaved'))
    fetchSubscription()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function runDryrun() {
  dryrunLoading.value = true
  try {
    const resp = await subscriptionsApi.dryrun(id)
    const data = resp.data.data || {}
    dryrunResults.value = (data.recentTorrents || []).map((torrent: any) => ({
      title: torrent.title || torrent.name || '-',
      size: torrent.size ? (torrent.size / 1073741824).toFixed(2) + ' GB' : '-',
      matched: torrent.matched ? t('common.yes') : t('common.no'),
      reason: torrent.reason || '-',
    }))
    message.success(t('subscription.dryrunComplete', { count: data.total || 0 }))
  } catch (e: any) {
    message.error(e.message)
  } finally {
    dryrunLoading.value = false
  }
}

async function fetchHistory() {
  historyLoading.value = true
  try {
    const resp = await subscriptionsApi.get(id)
    const sub = resp.data.data || {}
    history.value = (sub.recentFetches || []).map((f: any, idx: number) => ({
      id: idx + 1,
      fetchedAt: f.fetchedAt || f.createdAt || '-',
      newCount: f.newCount ?? 0,
      status: f.status || 'ok',
    }))
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

async function saveRules() {
  rulesSaving.value = true
  try {
    const parsed = JSON.parse(rulesJSON.value)
    await subscriptionsApi.updateRules(id, parsed)
    message.success(t('subscription.rulesSaved'))
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      message.error('JSON 格式错误')
    } else {
      message.error(e.message)
    }
  } finally {
    rulesSaving.value = false
  }
}

onMounted(() => {
  fetchSubscription()
  fetchHistory()
})
</script>
