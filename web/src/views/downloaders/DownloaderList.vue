<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('downloader.addDownloader') }}
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
      @change="(pag: { current: number; pageSize: number }) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-badge
            :status="record.enabled ? 'success' : 'default'"
            :text="record.enabled ? t('common.online') : t('common.offline')"
          />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/downloaders/${record.id}`)">{{ t('common.detail') }}</a-button>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" @click="testConnection(record.id)">{{ t('common.test') }}</a-button>
            <a-popconfirm :title="t('downloader.deleteDownloaderConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRecord ? t('downloader.editDownloader') : t('downloader.addDownloader')"
      :confirm-loading="submitting"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('common.name')" name="name" :rules="[{ required: true, message: t('common.name') }]">
          <a-input v-model:value="form.name" :placeholder="t('common.name')" />
        </a-form-item>
        <a-form-item :label="t('common.type')" name="type" :rules="[{ required: true, message: t('common.type') }]">
          <a-select v-model:value="form.type" :placeholder="t('common.type')">
            <a-select-option value="qbittorrent">qBittorrent</a-select-option>
            <a-select-option value="transmission">Transmission</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('common.address')" name="url" :rules="[{ required: true, message: t('common.address') }]">
          <a-input v-model:value="form.url" :placeholder="t('downloader.addressPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('common.username')" name="username">
          <a-input v-model:value="form.username" :placeholder="t('common.username')" />
        </a-form-item>
        <a-form-item :label="t('common.password')" name="password">
          <a-input-password v-model:value="form.password" :placeholder="t('common.password')" />
        </a-form-item>
        <a-form-item :label="t('downloader.role')" name="role" :rules="[{ required: true, message: t('downloader.selectRole') }]">
          <a-select v-model:value="form.role" :placeholder="t('downloader.selectRole')">
            <a-select-option value="download">{{ t('downloader.roleDownload') }}</a-select-option>
            <a-select-option value="seeding">{{ t('downloader.roleSeeding') }}</a-select-option>
            <a-select-option value="source">{{ t('downloader.roleSource') }}</a-select-option>
            <a-select-option value="reseed">{{ t('downloader.roleReseed') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('common.enable')" name="enabled">
          <a-switch v-model:checked="form.enabled" />
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
import { downloadersApi } from '@/api/downloaders'
import { usePagination } from '@/composables/usePagination'
import type { ClientConfig } from '@/api/types'

const { t } = useI18n()
const modalVisible = ref(false)
const submitting = ref(false)
const editingRecord = ref<ClientConfig | null>(null)

const form = reactive({
  name: '',
  type: 'qbittorrent',
  url: '',
  username: '',
  password: '',
  role: 'download',
  enabled: true,
})

const columns = [
  { title: t('common.name'), dataIndex: 'name', key: 'name' },
  { title: t('common.type'), dataIndex: 'type', key: 'type' },
  { title: t('common.address'), dataIndex: 'url', key: 'url' },
  { title: t('downloader.role'), dataIndex: 'role', key: 'role', width: 80 },
  { title: t('common.enable'), dataIndex: 'enabled', key: 'enabled', width: 80 },
  { title: t('common.actions'), key: 'actions', width: 260 },
]

const pagination = usePagination((page, size) => downloadersApi.list(page, size))

function openModal(record?: ClientConfig) {
  editingRecord.value = record || null
  if (record) {
    Object.assign(form, { name: record.name, type: record.type, url: record.url, username: record.username || '', password: '', role: record.role || 'download', enabled: record.enabled ?? true })
  } else {
    Object.assign(form, { name: '', type: 'qbittorrent', url: '', username: '', password: '', role: 'download', enabled: true })
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
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    pagination.fetch()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await downloadersApi.delete(id)
    message.success(t('common.deleteSuccess'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function testConnection(id: number) {
  try {
    await downloadersApi.testConnection(id)
    message.success(t('common.testSuccess'))
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

onMounted(() => pagination.fetch())
</script>
