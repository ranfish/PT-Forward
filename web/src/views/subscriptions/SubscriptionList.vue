<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('subscription.addSubscription') }}
      </a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="pagination.data.value"
      :loading="pagination.loading.value"
      :pagination="{
        current: pagination.currentPage.value,
        pageSize: pagination.pageSize.value,
        total: pagination.total.value,
        showSizeChanger: true,
        showTotal: (total: number) => t('common.totalCount', { count: total }),
      }"
      row-key="id"
      @change="(pag: any) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" @change="toggleSubscription(record)" />
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
      @ok="handleSubmit"
      :confirm-loading="submitting"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('common.name')" name="name" :rules="[{ required: true, message: t('common.nameRequired') }]">
          <a-input v-model:value="form.name" placeholder="订阅名称" />
        </a-form-item>
        <a-form-item :label="t('common.site')" name="siteName" :rules="[{ required: true, message: t('common.siteRequired') }]">
          <a-input v-model:value="form.siteName" :placeholder="t('common.site')" />
        </a-form-item>
        <a-form-item :label="t('subscription.url')" name="urls" :rules="[{ required: true, message: t('subscription.urlRequired') }]">
          <a-textarea v-model:value="form.urls" placeholder="RSS 地址，每行一个" :rows="3" />
        </a-form-item>
        <a-form-item :label="t('subscription.cronExpression')" name="cron">
          <a-input v-model:value="form.cron" placeholder="例如: */15 * * * *" />
        </a-form-item>
        <a-form-item label="Client ID" name="clientId">
          <a-input v-model:value="form.clientId" placeholder="qBittorrent" />
        </a-form-item>
        <a-form-item label="Save Path" name="savePath">
          <a-input v-model:value="form.savePath" placeholder="/downloads/..." />
        </a-form-item>
        <a-form-item label="Category" name="category">
          <a-input v-model:value="form.category" />
        </a-form-item>
        <a-form-item label="Add Paused" name="addPaused">
          <a-switch v-model:checked="form.addPaused" />
        </a-form-item>
        <a-form-item label="Auto TMM" name="autoTmm">
          <a-switch v-model:checked="form.autoTmm" />
        </a-form-item>
        <a-form-item label="Tags" name="tags">
          <a-select v-model:value="form.tags" mode="tags" placeholder="标签" style="width: 100%" />
        </a-form-item>
        <a-form-item label="Scrape Free" name="scrapeFree">
          <a-switch v-model:checked="form.scrapeFree" />
        </a-form-item>
        <a-form-item label="Scrape HR" name="scrapeHr">
          <a-switch v-model:checked="form.scrapeHr" />
        </a-form-item>
        <a-form-item label="Upload Limit (KB/s)" name="uploadLimitKb">
          <a-input-number v-model:value="form.uploadLimitKb" :min="0" style="width: 100%" placeholder="0 = 不限" />
        </a-form-item>
        <a-form-item label="Download Limit (KB/s)" name="downloadLimitKb">
          <a-input-number v-model:value="form.downloadLimitKb" :min="0" style="width: 100%" placeholder="0 = 不限" />
        </a-form-item>
        <a-form-item label="Publish Enabled" name="publishEnabled">
          <a-switch v-model:checked="form.publishEnabled" />
        </a-form-item>
        <a-form-item label="Push Notify" name="pushNotify">
          <a-switch v-model:checked="form.pushNotify" />
        </a-form-item>
        <a-form-item label="Auto Reseed" name="autoReseed">
          <a-switch v-model:checked="form.autoReseed" />
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
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()

const modalVisible = ref(false)
const submitting = ref(false)
const editingRecord = ref<any>(null)

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
  enabled: true,
})

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '站点', dataIndex: 'siteName', key: 'siteName' },
  { title: 'Cron', dataIndex: 'cron', key: 'cron', width: 140 },
  { title: '启用', key: 'enabled', width: 80 },
  { title: '操作', key: 'actions', width: 220 },
]

const pagination = usePagination((page, size) => subscriptionsApi.list(page, size))

function openModal(record?: any) {
  editingRecord.value = record || null
  if (record) {
    Object.assign(form, {
      name: record.name, siteName: record.siteName,
      urls: (record.urls || []).join('\n'), cron: record.cron || '*/15 * * * *',
      clientId: record.client_id || record.clientId || '',
      savePath: record.save_path || record.savePath || '',
      category: record.category || '',
      addPaused: record.add_paused || record.addPaused || false,
      autoTmm: record.auto_tmm || record.autoTmm || false,
      tags: record.tags || [],
      scrapeFree: record.scrape_free || record.scrapeFree || false,
      scrapeHr: record.scrape_hr || record.scrapeHr || false,
      uploadLimitKb: record.upload_limit_kb || record.uploadLimitKb || 0,
      downloadLimitKb: record.download_limit_kb || record.downloadLimitKb || 0,
      publishEnabled: record.publish_enabled || record.publishEnabled || false,
      pushNotify: record.push_notify || record.pushNotify || false,
      autoReseed: record.auto_reseed || record.autoReseed || false,
      enabled: record.enabled ?? true,
    })
  } else {
    Object.assign(form, {
      name: '', siteName: '', urls: '', cron: '*/15 * * * *',
      clientId: '', savePath: '', category: '', addPaused: false, autoTmm: false,
      tags: [], scrapeFree: false, scrapeHr: false,
      uploadLimitKb: 0, downloadLimitKb: 0,
      publishEnabled: false, pushNotify: false, autoReseed: false, enabled: true,
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
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await subscriptionsApi.delete(id)
    message.success(t('common.deleteSuccess'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function toggleSubscription(record: any) {
  try {
    if (record.enabled) {
      await subscriptionsApi.pause(record.id)
    } else {
      await subscriptionsApi.resume(record.id)
    }
    message.success(t('subscription.statusToggled'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function triggerFetch(id: number) {
  try {
    await subscriptionsApi.trigger(id)
    message.success(t('subscription.fetchTriggered'))
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
