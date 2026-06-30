<template>
  <div style="padding: 24px">
    <a-page-header :title="t('downloads.title')" :subtitle="t('downloads.subtitle')">
      <template #extra>
        <a-space>
          <a-select v-model:value="filterClient" style="width: 180px" allow-clear
            :placeholder="t('downloads.filterClient')" @change="fetchData">
            <a-select-option v-for="c in clientOptions" :key="c" :value="c">{{ c }}</a-select-option>
          </a-select>
          <a-select v-model:value="filterStatus" style="width: 140px" allow-clear
            :placeholder="t('downloads.filterStatus')" @change="fetchData">
            <a-select-option value="downloading">{{ t('downloads.statusDownloading') }}</a-select-option>
            <a-select-option value="completed">{{ t('downloads.statusCompleted') }}</a-select-option>
            <a-select-option value="paused">{{ t('downloads.statusPaused') }}</a-select-option>
            <a-select-option value="error">{{ t('downloads.statusError') }}</a-select-option>
          </a-select>
          <a-button @click="fetchData" :icon="h(ReloadOutlined)">{{ t('common.refresh') }}</a-button>
        </a-space>
      </template>
    </a-page-header>

    <a-card :title="t('downloads.configTitle')" style="margin-bottom: 16px">
      <div style="margin-bottom: 12px; display: flex; justify-content: flex-end">
        <a-button type="primary" size="small" @click="openConfigModal()">{{ t('downloads.addConfig') }}</a-button>
      </div>
      <a-table :data-source="configs" :columns="configColumns" :loading="configsLoading" :pagination="false" row-key="id" size="small">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'enabled'">
            <a-switch :checked="record.enabled" @change="(v: boolean) => toggleConfig(record, v)" />
          </template>
          <template v-if="column.key === 'delete_rule_ids'">
            <template v-if="record.delete_rule_ids">
              <a-tag v-for="id in record.delete_rule_ids.split(',').filter(Boolean)" :key="id" size="small">
                {{ ruleMap.get(Number(id)) || '#' + id }}
              </a-tag>
            </template>
            <span v-else style="color: #999">-</span>
          </template>
          <template v-if="column.key === 'actions'">
            <a-space size="small">
              <a-button type="link" size="small" @click="openConfigModal(record)">{{ t('common.edit') }}</a-button>
              <a-popconfirm @confirm="handleDeleteConfig(record.id)">
                <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <div style="margin-bottom: 16px">
      <a-space>
        <a-button type="primary" @click="showAddModal = true" :icon="h(PlusOutlined)">{{ t('downloads.addDownload') }}</a-button>
      </a-space>
    </div>

    <div v-if="selectedRowKeys.length > 0" style="margin-bottom: 16px">
      <a-space>
        <span>{{ selectedRowKeys.length }} {{ t('downloads.selected') }}</span>
        <a-button size="small" @click="handleBulk('pause')" :disabled="!hasSelected">{{ t('downloads.pause') }}</a-button>
        <a-button size="small" @click="handleBulk('resume')">{{ t('downloads.resume') }}</a-button>
        <a-button size="small" @click="handleBulk('recheck')">{{ t('downloads.recheck') }}</a-button>
        <a-popconfirm :title="t('downloads.bulkDeleteConfirm')" @confirm="handleBulkDelete">
          <a-button size="small" danger>{{ t('common.delete') }}</a-button>
        </a-popconfirm>
        <a-button size="small" type="text" @click="selectedRowKeys = []">{{ t('common.cancel') }}</a-button>
      </a-space>
    </div>

    <a-table
      :data-source="tasks"
      :columns="columns"
      :loading="loading"
      row-key="id"
      :row-selection="{ selectedRowKeys, onChange: onSelectChange }"
      :pagination="{
        current: page,
        pageSize: size,
        total: total,
        showSizeChanger: true,
        showTotal: (total: number) => `${total} ${t('common.items')}`,
      }"
      @change="onPageChange"
      size="small"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'torrent_name'">
          <span :title="record.torrent_name" style="display: inline-block; max-width: 350px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; vertical-align: middle;">
            {{ record.torrent_name }}
          </span>
        </template>
        <template v-else-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
        </template>
        <template v-else-if="column.key === 'progress'">
          <div v-if="record.status === 'downloading' || record.status === 'paused'" style="min-width: 100px">
            <a-progress :percent="Math.round(record.progress)" size="small" :stroke-color="record.status === 'paused' ? '#faad14' : '#1890ff'" />
          </div>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'speed'">
          <div v-if="record.status === 'downloading'" style="font-size: 12px; line-height: 1.4">
            <div><ArrowUpOutlined /> {{ formatSpeed(record.upload_speed) }}</div>
            <div><ArrowDownOutlined /> {{ formatSpeed(record.download_speed) }}</div>
          </div>
          <span v-else-if="record.status === 'completed'" style="font-size: 12px">
            <ArrowUpOutlined /> {{ formatSpeed(record.upload_speed) }}
          </span>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'ratio'">
          <span :style="{ color: record.ratio >= 1 ? '#52c41a' : '#faad14' }">{{ record.ratio > 0 ? record.ratio.toFixed(2) : '-' }}</span>
        </template>
        <template v-else-if="column.key === 'transfer_status'">
          <a-tag v-if="record.transfer_status" :color="transferColor(record.transfer_status)">
            {{ transferLabel(record.transfer_status) }}
          </a-tag>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'total_size'">
          {{ formatBytes(record.total_size) }}
        </template>
        <template v-else-if="column.key === 'created_at'">
          {{ formatTime(record.created_at) }}
        </template>
        <template v-else-if="column.key === 'action'">
          <a-space size="small">
            <a-button v-if="record.transfer_status === 'transfer_failed' || record.transfer_status === 'transfer_partial'"
              type="link" size="small" @click="handleRetryTransfer(record)">{{ t('downloads.retryTransfer') }}</a-button>
            <a-popconfirm
              :title="t('downloads.deleteConfirm')"
              @confirm="handleDelete(record)"
              :ok-text="t('common.confirm')"
              :cancel-text="t('common.cancel')"
            >
              <template #description>
                <a-radio-group v-model:value="deleteMode" style="margin-top: 8px" v-if="record.status !== 'deleted'">
                  <a-radio :value="true">{{ t('downloads.deleteWithCompanions') }}</a-radio>
                  <a-radio :value="false">{{ t('downloads.deleteSiteOnly') }}</a-radio>
                </a-radio-group>
              </template>
              <a-button type="text" danger size="small" :disabled="record.status === 'deleted'">
                {{ t('common.delete') }}
              </a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="showAddModal" :title="t('downloads.addDownload')" :confirm-loading="adding" @ok="handleAdd" width="520px">
      <a-form layout="vertical">
        <a-form-item label="目标下载器" required>
          <a-select v-model:value="addForm.client_id" :placeholder="t('downloads.selectClient')">
            <a-select-option v-for="c in clientOptions" :key="c" :value="c">{{ c }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-tabs v-model:active-key="addMode">
          <a-tab-pane key="file" tab="上传 .torrent 文件">
            <a-upload :before-upload="handleFileSelect" :max-count="1" accept=".torrent">
              <a-button :icon="h(UploadOutlined)">{{ t('downloads.selectFile') }}</a-button>
            </a-upload>
            <span v-if="addForm.file" style="margin-left: 8px; color: #52c41a">{{ addForm.file.name }}</span>
          </a-tab-pane>
          <a-tab-pane key="url" tab="下载链接">
            <a-input v-model:value="addForm.url" placeholder="https://example.com/download.php?id=123&passkey=xxx" />
          </a-tab-pane>
        </a-tabs>
        <a-form-item label="分类（可选）" style="margin-top: 16px">
          <a-input v-model:value="addForm.category" placeholder="如 PT3" />
        </a-form-item>
        <a-form-item label="添加后暂停">
          <a-switch v-model:checked="addForm.paused" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="configModalVisible" :title="t('downloads.configTitle')" :confirm-loading="configSubmitting" @ok="handleConfigSubmit" width="600px">
      <a-form layout="vertical">
        <a-form-item label="下载器" required>
          <a-select v-model:value="configForm.client_id" :disabled="!!editingConfig" :placeholder="t('downloads.selectClient')">
            <a-select-option v-for="c in clientOptions" :key="c" :value="c">{{ c }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="删种规则">
          <a-select v-model:value="configForm.deleteRuleIds" mode="multiple" :placeholder="t('downloads.selectRules')">
            <a-select-option v-for="r in allRules" :key="r.id" :value="r.id">{{ r.alias }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="启用">
              <a-switch v-model:checked="configForm.enabled" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="管理范围">
              <a-select v-model:value="configForm.scope">
                <a-select-option value="managed">仅 PT-Forward 推送</a-select-option>
                <a-select-option value="all">全部种子</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="数据同步周期">
              <a-input v-model:value="configForm.main_data_cron" placeholder="*/20 * * * *" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="删种评估周期">
              <a-input v-model:value="configForm.auto_delete_cron" placeholder="*/30 * * * *" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="磁盘保护">
              <a-switch v-model:checked="configForm.disk_protect_enabled" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="最小剩余空间 GB">
              <a-input-number v-model:value="configForm.min_disk_space_gb" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { ReloadOutlined, ArrowUpOutlined, ArrowDownOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons-vue'
import { downloadsApi, type DownloadTask, type DownloadClientConfig } from '@/api/downloads'
import { downloadersApi } from '@/api/downloaders'
import { seedingApi, deleteRulesApi } from '@/api/seeding'
import { formatBytes, formatTime } from '@/utils/format'

const { t } = useI18n()

const tasks = ref<DownloadTask[]>([])
const loading = ref(false)
const page = ref(1)
const size = ref(20)
const total = ref(0)
const filterClient = ref<string>('')
const filterStatus = ref<string>('')
const selectedRowKeys = ref<number[]>([])
const deleteMode = ref(true)
const allClients = ref<string[]>([])
const configs = ref<DownloadClientConfig[]>([])
const configsLoading = ref(false)
const allRules = ref<{ id: number; alias: string }[]>([])
const ruleMap = computed(() => new Map(allRules.value.map(r => [r.id, r.alias])))
const configModalVisible = ref(false)
const configSubmitting = ref(false)
const editingConfig = ref<DownloadClientConfig | null>(null)
const configForm = reactive({
  client_id: '',
  enabled: true,
  deleteRuleIds: [] as number[],
  auto_delete_cron: '*/30 * * * *',
  main_data_cron: '*/20 * * * *',
  disk_protect_enabled: true,
  min_disk_space_gb: 50,
  scope: 'managed',
})

const configColumns = computed(() => [
  { title: '下载器', dataIndex: 'client_id', key: 'client_id', width: 120 },
  { title: '启用', key: 'enabled', width: 70 },
  { title: '删种规则', key: 'delete_rule_ids' },
  { title: '范围', dataIndex: 'scope', key: 'scope', width: 120 },
  { title: '操作', key: 'actions', width: 120 },
])

const showAddModal = ref(false)
const adding = ref(false)
const addMode = ref<'file' | 'url'>('file')
const addForm = reactive({
  client_id: '',
  url: '',
  category: '',
  paused: false,
  file: null as File | null,
})

const hasSelected = computed(() => selectedRowKeys.value.length > 0)

const clientOptions = computed(() => allClients.value)

const columns = computed(() => [
  { title: t('downloads.torrentName'), dataIndex: 'torrent_name', key: 'torrent_name', ellipsis: true },
  { title: t('downloads.client'), dataIndex: 'client_id', key: 'client_id', width: 100 },
  { title: t('common.status'), key: 'status', width: 90 },
  { title: t('downloads.progress'), key: 'progress', width: 120 },
  { title: t('downloads.speed'), key: 'speed', width: 100 },
  { title: t('downloads.ratio'), key: 'ratio', width: 70 },
  { title: t('downloads.transfer'), key: 'transfer_status', width: 90 },
  { title: t('downloads.size'), key: 'total_size', width: 90 },
  { title: t('downloads.created'), key: 'created_at', width: 140 },
  { title: t('common.action'), key: 'action', width: 120 },
])

function statusColor(status: string): string {
  const map: Record<string, string> = {
    downloading: 'processing',
    completed: 'success',
    paused: 'warning',
    error: 'error',
    deleted: 'default',
    pending: 'default',
  }
  return map[status] || 'default'
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    downloading: t('downloads.statusDownloading'),
    completed: t('downloads.statusCompleted'),
    paused: t('downloads.statusPaused'),
    error: t('downloads.statusError'),
    deleted: t('downloads.statusDeleted'),
    pending: t('downloads.statusPending'),
  }
  return map[status] || status
}

function transferColor(status: string): string {
  const map: Record<string, string> = {
    transfer_pending: 'warning',
    transferring: 'processing',
    transferred: 'success',
    transfer_failed: 'error',
    transfer_partial: 'error',
  }
  return map[status] || 'default'
}

function transferLabel(status: string): string {
  const map: Record<string, string> = {
    transfer_pending: t('downloads.transferPending'),
    transferring: t('downloads.transferTransferring'),
    transferred: t('downloads.transferTransferred'),
    transfer_failed: t('downloads.transferFailed'),
    transfer_partial: t('downloads.transferPartial'),
  }
  return map[status] || status
}

function formatSpeed(bytesPerSec: number): string {
  if (!bytesPerSec || bytesPerSec <= 0) return '0 B/s'
  return formatBytes(bytesPerSec) + '/s'
}

async function fetchData() {
  loading.value = true
  try {
    const resp = await downloadsApi.list({
      page: page.value,
      size: size.value,
      client_id: filterClient.value || undefined,
      status: filterStatus.value || undefined,
    })
    const data = resp.data.data
    tasks.value = data.items || []
    total.value = data.total || 0
  } catch {
    // ignore
  } finally {
    loading.value = false
  }
}

function onPageChange(pag: { current: number; pageSize: number }) {
  page.value = pag.current
  size.value = pag.pageSize
  fetchData()
}

function onSelectChange(keys: number[]) {
  selectedRowKeys.value = keys
}

async function handleBulk(action: string) {
  try {
    const resp = await downloadsApi.bulkAction(selectedRowKeys.value, action)
    const d = resp.data.data
    message.success(`${d.succeeded} ${t('common.success')}, ${d.failed} ${t('common.failed')}`)
    selectedRowKeys.value = []
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleBulkDelete() {
  try {
    const resp = await downloadsApi.bulkAction(selectedRowKeys.value, 'delete', true)
    const d = resp.data.data
    message.success(`${d.succeeded} ${t('common.success')}, ${d.failed} ${t('common.failed')}`)
    selectedRowKeys.value = []
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleDelete(record: DownloadTask) {
  try {
    await downloadsApi.delete(record.id, deleteMode.value)
    message.success(t('common.deleted'))
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleRetryTransfer(record: DownloadTask) {
  try {
    await downloadsApi.retryTransfer(record.id)
    message.success(t('downloads.transferRetrySent'))
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

const handleFileSelect = (file: File) => {
  addForm.file = file
  return false
}

async function handleAdd() {
  if (!addForm.client_id) {
    message.warning(t('downloads.selectClient'))
    return
  }
  if (addMode.value === 'file' && !addForm.file) {
    message.warning(t('downloads.selectFile'))
    return
  }
  if (addMode.value === 'url' && !addForm.url) {
    message.warning(t('downloads.inputUrl'))
    return
  }

  adding.value = true
  try {
    if (addMode.value === 'file' && addForm.file) {
      const formData = new FormData()
      formData.append('torrent', addForm.file)
      formData.append('client_id', addForm.client_id)
      if (addForm.category) formData.append('category', addForm.category)
      if (addForm.paused) formData.append('paused', 'true')
      const { default: client } = await import('@/api/client')
      await client.post('/downloads', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
    } else {
      await downloadsApi.addByUrl(addForm.client_id, addForm.url, addForm.category, addForm.paused)
    }
    message.success(t('common.operationSuccess'))
    showAddModal.value = false
    addForm.client_id = ''
    addForm.url = ''
    addForm.category = ''
    addForm.paused = false
    addForm.file = null
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    adding.value = false
  }
}

function openConfigModal(record?: DownloadClientConfig) {
  editingConfig.value = record || null
  if (record) {
    Object.assign(configForm, {
      client_id: record.client_id,
      enabled: record.enabled,
      deleteRuleIds: record.delete_rule_ids ? record.delete_rule_ids.split(',').filter(Boolean).map(Number) : [],
      auto_delete_cron: record.auto_delete_cron || '*/30 * * * *',
      main_data_cron: record.main_data_cron || '*/20 * * * *',
      disk_protect_enabled: record.disk_protect_enabled,
      min_disk_space_gb: record.min_disk_space_gb || 50,
      scope: record.scope || 'managed',
    })
  } else {
    Object.assign(configForm, {
      client_id: '', enabled: true, deleteRuleIds: [],
      auto_delete_cron: '*/30 * * * *', main_data_cron: '*/20 * * * *',
      disk_protect_enabled: true, min_disk_space_gb: 50, scope: 'managed',
    })
  }
  configModalVisible.value = true
}

async function handleConfigSubmit() {
  if (!configForm.client_id) {
    message.warning(t('downloads.selectClient'))
    return
  }
  configSubmitting.value = true
  try {
    const payload = {
      client_id: configForm.client_id,
      enabled: configForm.enabled,
      delete_rule_ids: configForm.deleteRuleIds.join(','),
      auto_delete_cron: configForm.auto_delete_cron,
      main_data_cron: configForm.main_data_cron,
      disk_protect_enabled: configForm.disk_protect_enabled,
      min_disk_space_gb: configForm.min_disk_space_gb,
      scope: configForm.scope,
    }
    if (editingConfig.value) {
      await downloadsApi.updateConfig(editingConfig.value.id, payload)
    } else {
      await downloadsApi.createConfig(payload)
    }
    message.success(t('common.operationSuccess'))
    configModalVisible.value = false
    fetchConfigs()
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    configSubmitting.value = false
  }
}

async function toggleConfig(record: DownloadClientConfig, checked: boolean) {
  try {
    await downloadsApi.updateConfig(record.id, { enabled: checked })
    record.enabled = checked
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleDeleteConfig(id: number) {
  try {
    await downloadsApi.deleteConfig(id)
    message.success(t('common.deleted'))
    fetchConfigs()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function fetchConfigs() {
  configsLoading.value = true
  try {
    const resp = await downloadsApi.listConfigs()
    configs.value = resp.data.data || []
  } catch {
    // ignore
  } finally {
    configsLoading.value = false
  }
}

onMounted(async () => {
  try {
    const resp = await downloadersApi.list(1, 200)
    allClients.value = (resp.data.data?.items || []).map((c: { name: string }) => c.name).sort()
  } catch {
    // ignore
  }
  try {
    const resp = await deleteRulesApi.list()
    allRules.value = (resp.data.data?.items || []).map((r: { id: number; alias: string }) => ({ id: r.id, alias: r.alias }))
  } catch {
    // ignore
  }
  fetchConfigs()
  fetchData()
  setInterval(fetchData, 30000)
})
</script>
