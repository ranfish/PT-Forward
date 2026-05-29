<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end; gap: 8px">
      <a-button :disabled="selectedRowKeys.length === 0" :loading="batchTriggering" @click="batchTrigger">
        {{ t('subscription.batchTrigger', { count: selectedRowKeys.length }) }}
      </a-button>
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('subscription.addSubscription') }}
      </a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="pagination.data.value"
      :loading="pagination.loading.value"
      :row-selection="{ selectedRowKeys, onChange: onSelectionChange }"
      :pagination="{
        current: pagination.currentPage.value,
        pageSize: pagination.pageSize.value,
        total: pagination.total.value,
        showSizeChanger: true,
        showTotal: (total: number) => t('common.totalCount', { count: total }),
      }"
      row-key="id"
      @change="(pag: { current: number; pageSize: number }) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="!record.paused" @change="toggleSubscription(record)" />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/subscriptions/${record.id}`)">{{ t('common.detail') }}</a-button>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" @click="triggerFetch(record.id)">{{ t('common.trigger') }}</a-button>
            <a-popconfirm :title="t('subscription.deleteSubscriptionConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRecord ? t('subscription.editSubscription') : t('subscription.addSubscription')"
      :confirm-loading="submitting"
      width="640px"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('common.name')" name="name" :rules="[{ required: true, message: t('common.nameRequired') }]">
          <a-input v-model:value="form.name" :placeholder="t('subscription.subscriptionNamePlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('subscription.belongToSite')" name="siteName" :rules="[{ required: true, message: t('common.siteRequired') }]">
          <a-select v-model:value="form.siteName" :placeholder="t('subscription.selectSite')" show-search :filter-option="filterSite" :loading="sitesLoading">
            <a-select-option v-for="s in sites" :key="s.name" :value="s.name">{{ s.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('subscription.rssAddress')" name="urls" :rules="[{ required: true, message: t('subscription.urlRequired') }]">
          <a-textarea v-model:value="form.urls" :placeholder="t('subscription.rssUrlPerLine')" :rows="3" />
        </a-form-item>
        <a-form-item :label="t('subscription.cronExpression')" name="cron">
          <a-input v-model:value="form.cron" :placeholder="t('subscription.cronExampleLong')" />
        </a-form-item>

        <a-divider>{{ t('subscription.downloaderSettings') }}</a-divider>
        <a-form-item :label="t('downloader.title')" name="clientId">
          <a-select v-model:value="form.clientId" :placeholder="t('subscription.selectDownloader')" :loading="downloadersLoading" allow-clear>
            <a-select-option v-for="d in downloaders" :key="d.name" :value="d.name">
              {{ d.name }}（{{ d.type }}）
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('subscription.savePathHint')" name="savePath">
          <a-input v-model:value="form.savePath" placeholder="/downloads/..." />
        </a-form-item>
        <a-form-item :label="t('subscription.categoryLabel')" name="category">
          <a-input v-model:value="form.category" :placeholder="t('subscription.categoryPlaceholder')" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('subscription.addPaused')" name="addPaused">
              <a-switch v-model:checked="form.addPaused" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('subscription.autoTmm')" name="autoTmm">
              <a-switch v-model:checked="form.autoTmm" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="t('subscription.tagsLabel')" name="tags">
          <a-select v-model:value="form.tags" mode="tags" :placeholder="t('subscription.tagsPlaceholder')" style="width: 100%" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('subscription.uploadLimit')" name="uploadLimitKb">
              <a-input-number v-model:value="form.uploadLimitKb" :min="0" style="width: 100%" :placeholder="t('subscription.noLimit')" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('subscription.downloadLimit')" name="downloadLimitKb">
              <a-input-number v-model:value="form.downloadLimitKb" :min="0" style="width: 100%" :placeholder="t('subscription.noLimit')" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>{{ t('subscription.scrapeOptions') }}</a-divider>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('subscription.freeOnly')" name="scrapeFree">
              <a-switch v-model:checked="form.scrapeFree" />
              <div style="color:#999;font-size:12px;margin-top:4px">{{ t('subscription.freeOnlyHint') }}</div>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('subscription.scrapeHrLabel')" name="scrapeHr">
              <a-switch v-model:checked="form.scrapeHr" />
              <div style="color:#999;font-size:12px;margin-top:4px">{{ t('subscription.scrapeHrHint') }}</div>
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>{{ t('subscription.automation') }}</a-divider>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item :label="t('subscription.enableAutoPublish')" name="publishEnabled">
              <a-switch v-model:checked="form.publishEnabled" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('subscription.pushNotifyLabel')" name="pushNotify">
              <a-switch v-model:checked="form.pushNotify" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('subscription.autoReseedLabel')" name="autoReseed">
              <a-switch v-model:checked="form.autoReseed" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item v-if="form.publishEnabled" :label="t('subscription.publishTargets')">
          <a-select v-model:value="form.publishTargets" mode="multiple" :placeholder="t('subscription.publishTargetsPlaceholder')" :loading="sitesLoading">
            <a-select-option v-for="s in sites" :key="s.name" :value="s.name">{{ s.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="form.pushNotify" :label="t('subscription.notifyChannel')">
          <a-input v-model:value="form.notifyId" :placeholder="t('subscription.notifyChannelPlaceholder')" />
        </a-form-item>
        <a-form-item v-if="form.autoReseed" :label="t('subscription.reseedClientIds')">
          <a-select v-model:value="form.reseedClientIds" mode="multiple" :placeholder="t('subscription.reseedClientIdsPlaceholder')" :loading="downloadersLoading">
            <a-select-option v-for="d in downloaders" :key="d.name" :value="d.name">{{ d.name }}</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { useI18n } from 'vue-i18n'
import { subscriptionsApi } from '@/api/subscriptions'
import { sitesApi } from '@/api/sites'
import { downloadersApi } from '@/api/downloaders'
import { usePagination } from '@/composables/usePagination'

interface SubscriptionItem {
  [key: string]: unknown
  id: number
  name: string
  siteName: string
  urls: string[]
  cron: string
  enabled: boolean
  clientId: string
  savePath: string
  category: string
  addPaused: boolean
  autoTmm: boolean
  tags: string[]
  scrapeFree: boolean
  scrapeHr: boolean
  uploadLimitKb: number
  downloadLimitKb: number
  publishEnabled: boolean
  pushNotify: boolean
  autoReseed: boolean
}

const { t } = useI18n()

const selectedRowKeys = ref<number[]>([])
const batchTriggering = ref(false)

function onSelectionChange(keys: number[]) {
  selectedRowKeys.value = keys
}

async function batchTrigger() {
  batchTriggering.value = true
  let ok = 0
  let fail = 0
  for (const id of selectedRowKeys.value) {
    try {
      await subscriptionsApi.trigger(id)
      ok++
    } catch {
      fail++
    }
  }
  batchTriggering.value = false
  selectedRowKeys.value = []
  if (fail === 0) {
    message.success(t('subscription.batchTriggerSuccess', { count: ok }))
  } else {
    message.warning(t('subscription.batchTriggerResult', { ok, fail }))
  }
}

const modalVisible = ref(false)
const submitting = ref(false)
const editingRecord = ref<SubscriptionItem | null>(null)

const sites = ref<{ name: string }[]>([])
const sitesLoading = ref(false)
const downloaders = ref<{ name: string; type: string }[]>([])
const downloadersLoading = ref(false)

const form = reactive({
  name: '',
  siteName: '',
  urls: '',
  cron: '*/15 * * * *',
  clientId: '',
  savePath: '',
  category: '',
  addPaused: false,
  autoTmm: false,
  tags: [] as string[],
  scrapeFree: false,
  scrapeHr: false,
  uploadLimitKb: 0,
  downloadLimitKb: 0,
  publishEnabled: false,
  pushNotify: false,
  autoReseed: false,
  notifyId: '',
  publishTargets: [] as string[],
  reseedClientIds: [] as string[],
  enabled: true,
})

const columns = [
  { title: t('common.name'), dataIndex: 'name', key: 'name' },
  { title: t('common.site'), dataIndex: 'siteName', key: 'siteName' },
  { title: t('subscription.cronExpression'), dataIndex: 'cron', key: 'cron', width: 140 },
  { title: t('common.enable'), key: 'enabled', width: 80, align: 'center' as const },
  { title: t('common.actions'), key: 'actions', width: 220 },
]

const pagination = usePagination((page, size) => subscriptionsApi.list(page, size))

function filterSite(input: string, option: { key?: string }) {
  return option.key?.toLowerCase().includes(input.toLowerCase())
}

async function fetchSites() {
  sitesLoading.value = true
  try {
    const resp = await sitesApi.list(1, 200)
    sites.value = resp.data?.data?.items || resp.data?.data || []
  } catch {
    sites.value = []
  } finally {
    sitesLoading.value = false
  }
}

async function fetchDownloaders() {
  downloadersLoading.value = true
  try {
    const resp = await downloadersApi.list(1, 200)
    downloaders.value = resp.data?.data?.items || resp.data?.data || []
  } catch {
    downloaders.value = []
  } finally {
    downloadersLoading.value = false
  }
}

function openModal(record?: SubscriptionItem) {
  editingRecord.value = record || null
  if (record) {
    Object.assign(form, {
      name: record.name, siteName: record.siteName,
      urls: (record.urls || []).join('\n'), cron: record.cron || '*/15 * * * *',
      clientId: record.clientId || '',
      savePath: record.savePath || '',
      category: record.category || '',
      addPaused: record.addPaused || false,
      autoTmm: record.autoTmm || false,
      tags: record.tags || [],
      scrapeFree: record.scrapeFree || false,
      scrapeHr: record.scrapeHr || false,
      uploadLimitKb: record.uploadLimitKb || 0,
      downloadLimitKb: record.downloadLimitKb || 0,
      publishEnabled: record.publishEnabled || false,
      pushNotify: record.pushNotify || false,
      autoReseed: record.autoReseed || false,
      notifyId: record.notifyId || '',
      publishTargets: record.publishTargets || [],
      reseedClientIds: record.reseedClientIds || [],
      enabled: record.enabled ?? true,
    })
  } else {
    Object.assign(form, {
      name: '', siteName: '', urls: '', cron: '*/15 * * * *',
      clientId: '', savePath: '', category: '', addPaused: false, autoTmm: false,
      tags: [], scrapeFree: false, scrapeHr: false,
      uploadLimitKb: 0, downloadLimitKb: 0,
      publishEnabled: false, pushNotify: false, autoReseed: false,
      notifyId: '', publishTargets: [], reseedClientIds: [], enabled: true,
    })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingRecord.value) {
      await subscriptionsApi.update(editingRecord.value.id, { ...form, urls: form.urls.split('\n').filter(u => u.trim()) })
    } else {
      await subscriptionsApi.create({ ...form, urls: form.urls.split('\n').filter(u => u.trim()) })
    }
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    pagination.fetch()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await subscriptionsApi.delete(id)
    message.success(t('common.deleteSuccess'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function toggleSubscription(record: SubscriptionItem) {
  try {
    if (record.paused) {
      await subscriptionsApi.resume(record.id)
    } else {
      await subscriptionsApi.pause(record.id)
    }
    message.success(t('subscription.statusToggled'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function triggerFetch(id: number) {
  try {
    await subscriptionsApi.trigger(id)
    message.success(t('subscription.fetchTriggered'))
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

onMounted(() => {
  pagination.fetch()
  fetchSites()
  fetchDownloaders()
})
</script>
