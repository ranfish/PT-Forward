<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        添加通知渠道
      </a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="channels"
      :loading="loading"
      :pagination="false"
      row-key="id"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'type'">
          <a-tag>{{ record.type }}</a-tag>
        </template>
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" @change="toggleChannel(record)" />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="openModal(record)">编辑</a-button>
            <a-button type="link" size="small" @click="testChannel(record.id)">测试</a-button>
            <a-popconfirm title="确定删除该渠道？" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingChannel ? '编辑通知渠道' : '添加通知渠道'"
      @ok="handleSubmit"
      :confirm-loading="submitting"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item label="名称" name="name" :rules="[{ required: true, message: '请输入名称' }]">
          <a-input v-model:value="form.name" placeholder="渠道名称" />
        </a-form-item>
        <a-form-item label="类型" name="type" :rules="[{ required: true, message: '请选择类型' }]">
          <a-select v-model:value="form.type" placeholder="选择通知类型">
            <a-select-option value="telegram">Telegram</a-select-option>
            <a-select-option value="bark">Bark</a-select-option>
            <a-select-option value="webhook">Webhook</a-select-option>
            <a-select-option value="serverchan">Server酱</a-select-option>
            <a-select-option value="dingtalk">钉钉</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="配置" name="config">
          <a-textarea v-model:value="form.config" :rows="4" placeholder="JSON 格式的渠道配置" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { notificationsApi } from '@/api/notifications'

const loading = ref(false)
const modalVisible = ref(false)
const submitting = ref(false)
const editingChannel = ref<any>(null)
const channels = ref<any[]>([])

const form = reactive({
  name: '',
  type: 'telegram',
  config: '',
})

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '类型', key: 'type', width: 120 },
  { title: '启用', key: 'enabled', width: 80 },
  { title: '创建时间', dataIndex: 'createdAt', key: 'createdAt', width: 180 },
  { title: '操作', key: 'actions', width: 200 },
]

async function fetchChannels() {
  loading.value = true
  try {
    const resp = await notificationsApi.list()
    channels.value = resp.data.data?.items || resp.data.data || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

function openModal(record?: any) {
  editingChannel.value = record || null
  if (record) {
    Object.assign(form, { name: record.name, type: record.type, config: typeof record.config === 'string' ? record.config : JSON.stringify(record.config, null, 2) })
  } else {
    Object.assign(form, { name: '', type: 'telegram', config: '' })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    const payload = { ...form, config: form.config ? JSON.parse(form.config) : {} }
    if (editingChannel.value) {
      await notificationsApi.update(editingChannel.value.id, payload)
    } else {
      await notificationsApi.create(payload)
    }
    message.success('操作成功')
    modalVisible.value = false
    fetchChannels()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await notificationsApi.delete(id)
    message.success('删除成功')
    fetchChannels()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function testChannel(id: number) {
  try {
    await notificationsApi.test(id)
    message.success('测试通知已发送')
  } catch (e: any) {
    message.error(e.message)
  }
}

async function toggleChannel(record: any) {
  try {
    await notificationsApi.update(record.id, { ...record, enabled: !record.enabled })
    message.success('状态已切换')
    fetchChannels()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(fetchChannels)
</script>
