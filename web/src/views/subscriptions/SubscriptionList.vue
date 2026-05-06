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
        showTotal: (total: number) => t('common.totalCount', { total }),
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
          <a-input v-model:value="form.siteName" placeholder="站点名称" />
        </a-form-item>
        <a-form-item :label="t('subscription.url')" name="urls" :rules="[{ required: true, message: t('subscription.urlRequired') }]">
          <a-textarea v-model:value="form.urls" placeholder="RSS 地址，每行一个" :rows="3" />
        </a-form-item>
        <a-form-item :label="t('subscription.cronExpression')" name="cron">
          <a-input v-model:value="form.cron" placeholder="例如: */15 * * * *" />
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
    Object.assign(form, { name: record.name, siteName: record.siteName, urls: (record.urls || []).join('\n'), cron: record.cron || '*/15 * * * *' })
  } else {
    Object.assign(form, { name: '', siteName: '', urls: '', cron: '*/15 * * * *' })
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
