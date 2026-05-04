<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        创建任务
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
            <a-button type="link" size="small" @click="$router.push(`/reseed/tasks/${record.id}`)">详情</a-button>
            <a-button type="link" size="small" @click="triggerTask(record.id)" :disabled="record.status === 'running'">触发</a-button>
            <a-button type="link" size="small" @click="cancelTask(record.id)" :disabled="record.status !== 'running'">取消</a-button>
            <a-popconfirm title="确定删除该任务？" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      title="创建辅种任务"
      @ok="handleSubmit"
      :confirm-loading="submitting"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item label="任务名称" name="name" :rules="[{ required: true, message: '请输入任务名称' }]">
          <a-input v-model:value="form.name" placeholder="辅种任务名称" />
        </a-form-item>
        <a-form-item label="下载器 ID" name="clientIds" :rules="[{ required: true, message: '请输入下载器 ID' }]">
          <a-input v-model:value="form.clientIds" placeholder="逗号分隔的下载器 ID" />
        </a-form-item>
        <a-form-item label="源站点 ID" name="sourceSiteIds" :rules="[{ required: true, message: '请输入源站点' }]">
          <a-input v-model:value="form.sourceSiteIds" placeholder="逗号分隔的源站点域名" />
        </a-form-item>
        <a-form-item label="目标站点 ID" name="targetSiteIds" :rules="[{ required: true, message: '请输入目标站点' }]">
          <a-input v-model:value="form.targetSiteIds" placeholder="逗号分隔的目标站点域名" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { reseedApi } from '@/api/reseed'
import { usePagination } from '@/composables/usePagination'

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
    message.success('任务已创建')
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
    message.success('任务已触发')
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function cancelTask(id: number) {
  try {
    await reseedApi.cancelTask(id)
    message.success('任务已取消')
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function handleDelete(id: number) {
  try {
    await reseedApi.deleteTask(id)
    message.success('删除成功')
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
