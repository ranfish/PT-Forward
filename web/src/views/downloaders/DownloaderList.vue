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
        <template v-if="column.key === 'type'">
          {{ translateDownloaderType(record.type) }}
        </template>
        <template v-if="column.key === 'role'">
          {{ translateRole(record.role) }}
        </template>
        <template v-if="column.key === 'enabled'">
          <a-space :size="8" align="center">
            <a-switch :checked="record.enabled" size="small" @change="(val: boolean) => toggleEnabled(record, val)" />
            <a-badge
              v-if="record.enabled"
              :status="record.connected ? 'success' : 'error'"
              :text="record.connected ? t('common.online') : t('common.disconnected')"
            />
            <span v-else style="color: #999">{{ t('common.disabled') }}</span>
          </a-space>
        </template>
        <template v-if="column.key === 'downloadSpeed'">
          <span><ArrowDownOutlined /> {{ formatSpeed(record.downloadSpeed) }}</span>
        </template>
        <template v-if="column.key === 'uploadSpeed'">
          <span><ArrowUpOutlined /> {{ formatSpeed(record.uploadSpeed) }}</span>
        </template>
        <template v-if="column.key === 'freeSpace'">
          <span><DatabaseOutlined /> {{ formatBytes(record.freeSpace) }}</span>
        </template>
        <template v-if="column.key === 'totalDiskSpace'">
          <span>{{ formatBytes(record.totalDiskSpace) }}</span>
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
        <a-form-item label="媒体文件位置" name="isLocal" :rules="[{ required: true, message: '请选择媒体文件位置' }]">
          <a-radio-group v-model:value="form.isLocal">
            <a-radio :value="true">本地发布（媒体文件在本机可访问）</a-radio>
            <a-radio :value="false">转种上盒（媒体文件不在本机，使用源站数据）</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item :label="t('common.enable')" name="enabled">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('downloader.isDefault')">
              <a-switch v-model:checked="form.isDefault" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('downloader.transferTargetId')">
              <a-select
                v-model:value="form.transferTargetId"
                :placeholder="form.type !== 'qbittorrent' ? '仅 qBittorrent 支持主辅分离转移' : t('downloader.transferTargetIdPlaceholder')"
                :disabled="form.type !== 'qbittorrent'"
                allow-clear
              >
                <a-select-option
                  v-for="c in (pagination.data.value as ClientConfig[]).filter(d => d.role === 'reseed' && d.name !== form.name)"
                  :key="c.id"
                  :value="c.name"
                >
                  {{ c.name }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>

        <a-form-item label="种子文件路径" name="torrentDir">
          <a-input v-model:value="form.torrentDir" placeholder="选填。仅当下载器是 transmission 且与 PT-Forward 同机时填写（导出种子做主辅分离转移用）；qbittorrent 不需要" />
        </a-form-item>

        <a-divider>{{ t('downloader.pathMappings') }}</a-divider>
        <div v-for="(pm, idx) in form.pathMappings" :key="idx" style="margin-bottom: 8px">
          <a-row :gutter="8" align="middle">
            <a-col :span="10">
              <a-input v-model:value="pm.sourcePath" :placeholder="t('downloader.sourcePathPlaceholder')" />
            </a-col>
            <a-col :span="10">
              <a-input v-model:value="pm.reseedPath" :placeholder="t('downloader.reseedPathPlaceholder')" />
            </a-col>
            <a-col :span="4">
              <a-button type="text" danger @click="form.pathMappings.splice(idx, 1)">-</a-button>
            </a-col>
          </a-row>
        </div>
        <a-button type="dashed" block @click="form.pathMappings.push({ sourcePath: '', reseedPath: '' })">+ {{ t('downloader.addPathMapping') }}</a-button>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { PlusOutlined, ArrowDownOutlined, ArrowUpOutlined, DatabaseOutlined } from '@ant-design/icons-vue'
import { downloadersApi } from '@/api/downloaders'
import { usePagination } from '@/composables/usePagination'
import { formatBytes, formatSpeed } from '@/utils/format'
import { useEnumLabels } from '@/utils/enumLabels'
import type { ClientConfig } from '@/api/types'

const { t } = useI18n()
const { translateRole, translateDownloaderType } = useEnumLabels()
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
  isDefault: false,
  transferTargetId: '',
  torrentDir: '',
  isLocal: null as boolean | null,
  pathMappings: [] as { sourcePath: string; reseedPath: string }[],
})

watch(() => form.type, (val) => {
  if (val !== 'qbittorrent' && form.transferTargetId) {
    form.transferTargetId = ''
  }
})

const columns = [
  { title: t('common.name'), dataIndex: 'name', key: 'name' },
  { title: t('common.type'), dataIndex: 'type', key: 'type' },
  { title: t('common.address'), dataIndex: 'url', key: 'url' },
  { title: t('downloader.downloadSpeed'), key: 'downloadSpeed', width: 120 },
  { title: t('downloader.uploadSpeed'), key: 'uploadSpeed', width: 120 },
  { title: t('downloader.freeSpace'), key: 'freeSpace', width: 120 },
  { title: t('downloader.totalDiskSpace'), key: 'totalDiskSpace', width: 120 },
  { title: t('downloader.role'), dataIndex: 'role', key: 'role', width: 110 },
  { title: t('common.enable'), dataIndex: 'enabled', key: 'enabled', width: 160 },
  { title: t('common.actions'), key: 'actions', width: 260 },
]

const pagination = usePagination((page, size) => downloadersApi.list(page, size))

function openModal(record?: ClientConfig) {
  editingRecord.value = record || null
  if (record) {
    Object.assign(form, { name: record.name, type: record.type, url: record.url, username: record.username || '', password: '', role: record.role || 'download', enabled: record.enabled ?? true, isDefault: record.isDefault || false, transferTargetId: record.transferTargetId || '', torrentDir: record.torrentDir || '', isLocal: record.isLocal ?? null, pathMappings: (record.pathMappings || []).map((p: { sourcePath: string; reseedPath: string }) => ({ sourcePath: p.sourcePath || '', reseedPath: p.reseedPath || '' })) })
  } else {
    Object.assign(form, { name: '', type: 'qbittorrent', url: '', username: '', password: '', role: 'download', enabled: true, isDefault: false, transferTargetId: '', torrentDir: '', isLocal: null, pathMappings: [] })
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

async function toggleEnabled(record: ClientConfig, val: boolean) {
  try {
    await downloadersApi.update(record.id, { ...record, enabled: val })
    record.enabled = val
    message.success(t('common.operationSuccess'))
  } catch (e: unknown) {
    message.error((e as Error).message)
    pagination.fetch()
  }
}

onMounted(() => pagination.fetch())
</script>
