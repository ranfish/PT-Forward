<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('reseed.addTask') }}
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
        showTotal: (total: number) => `共 ${total} 条`,
      }"
      row-key="id"
      @change="(pag: any) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/reseed/tasks/${record.id}`)">{{ t('common.detail') }}</a-button>
            <a-button type="link" size="small" @click="triggerTask(record.id)" :disabled="record.status === 'running'">{{ t('common.trigger') }}</a-button>
            <a-button type="link" size="small" @click="cancelTask(record.id)" :disabled="record.status !== 'running'">{{ t('common.cancel') }}</a-button>
            <a-popconfirm :title="t('reseed.deleteTaskConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="t('reseed.createReseedTask')"
      @ok="handleSubmit"
      :confirm-loading="submitting"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('reseed.taskName')" name="name" :rules="[{ required: true, message: t('reseed.pleaseEnterTaskName') }]">
          <a-input v-model:value="form.name" :placeholder="t('reseed.taskNamePlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('reseed.clientId')" name="clientIds" :rules="[{ required: true, message: t('reseed.pleaseEnterClientId') }]">
          <a-input v-model:value="form.clientIds" :placeholder="t('reseed.clientIdPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('reseed.sourceSiteIds')" name="sourceSiteIds" :rules="[{ required: true, message: t('reseed.pleaseEnterSourceSite') }]">
          <a-input v-model:value="form.sourceSiteIds" :placeholder="t('reseed.sourceSiteIdsPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('reseed.targetSiteIds')" name="targetSiteIds" :rules="[{ required: true, message: t('reseed.pleaseEnterTargetSite') }]">
          <a-input v-model:value="form.targetSiteIds" :placeholder="t('reseed.targetSiteIdsPlaceholder')" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { reseedApi } from '@/api/reseed'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()
const modalVisible = ref(false)
const submitting = ref(false)

const form = reactive({
  name: '',
  clientIds: '',
  sourceSiteIds: '',
  targetSiteIds: '',
})

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '客户端', dataIndex: 'client_ids', key: 'client_ids', ellipsis: true },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: '操作', key: 'actions', width: 240 },
]

const pagination = usePagination((page, size) => reseedApi.listTasks(page, size))

function statusColor(status: string) {
  const map: Record<string, string> = { pending: 'blue', running: 'green', completed: 'default', failed: 'red' }
  return map[status] || 'default'
}

function openModal() {
  Object.assign(form, { name: '', clientIds: '', sourceSiteIds: '', targetSiteIds: '' })
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    await reseedApi.createTask(form)
    message.success(t('reseed.taskCreated'))
    modalVisible.value = false
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function triggerTask(id: number) {
  try {
    await reseedApi.triggerTask(id)
    message.success(t('reseed.taskTriggered'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function cancelTask(id: number) {
  try {
    await reseedApi.cancelTask(id)
    message.success(t('reseed.taskCancelled'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function handleDelete(id: number) {
  try {
    await reseedApi.deleteTask(id)
    message.success(t('common.deleted'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
