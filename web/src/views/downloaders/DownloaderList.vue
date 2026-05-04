<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        添加下载器
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
          <a-badge
            :status="record.status === 'online' ? 'success' : record.status === 'offline' ? 'error' : 'warning'"
            :text="record.status === 'online' ? '在线' : record.status === 'offline' ? '离线' : '未知'"
          />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/downloaders/${record.id}`)">详情</a-button>
            <a-button type="link" size="small" @click="openModal(record)">编辑</a-button>
            <a-button type="link" size="small" @click="testConnection(record.id)">测试</a-button>
            <a-popconfirm title="确定删除该下载器？" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRecord ? '编辑下载器' : '添加下载器'"
      @ok="handleSubmit"
      :confirm-loading="submitting"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item label="名称" name="name" :rules="[{ required: true, message: '请输入名称' }]">
          <a-input v-model:value="form.name" placeholder="下载器名称" />
        </a-form-item>
        <a-form-item label="类型" name="type" :rules="[{ required: true, message: '请选择类型' }]">
          <a-select v-model:value="form.type" placeholder="选择类型">
            <a-select-option value="qbittorrent">qBittorrent</a-select-option>
            <a-select-option value="transmission">Transmission</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="地址" name="url" :rules="[{ required: true, message: '请输入地址' }]">
          <a-input v-model:value="form.url" placeholder="例如: http://192.168.1.1:8080" />
        </a-form-item>
        <a-form-item label="用户名" name="username">
          <a-input v-model:value="form.username" placeholder="用户名" />
        </a-form-item>
        <a-form-item label="密码" name="password">
          <a-input-password v-model:value="form.password" placeholder="密码" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { downloadersApi } from '@/api/downloaders'
import { usePagination } from '@/composables/usePagination'

const modalVisible = ref(false)
const submitting = ref(false)
const editingRecord = ref<any>(null)

const form = reactive({
  name: '',
  type: 'qbittorrent',
  url: '',
  username: '',
  password: '',
})

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '类型', dataIndex: 'type', key: 'type' },
  { title: '地址', dataIndex: 'url', key: 'url' },
  { title: '启用', dataIndex: 'enabled', key: 'enabled', width: 80 },
  { title: '操作', key: 'actions', width: 260 },
]

const pagination = usePagination((page, size) => downloadersApi.list(page, size))

function openModal(record?: any) {
  editingRecord.value = record || null
  if (record) {
    Object.assign(form, { name: record.name, type: record.type, url: record.url, username: record.username || '', password: '' })
  } else {
    Object.assign(form, { name: '', type: 'qbittorrent', url: '', username: '', password: '' })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingRecord.value) {
      await downloadersApi.update(editingRecord.value.id, form)
    } else {
      await downloadersApi.create(form)
    }
    message.success('操作成功')
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
    await downloadersApi.delete(id)
    message.success('删除成功')
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function testConnection(id: number) {
  try {
    await downloadersApi.testConnection(id)
    message.success('连接测试成功')
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
