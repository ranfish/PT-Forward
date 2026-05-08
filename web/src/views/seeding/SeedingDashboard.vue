<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('seeding.activeTorrents')" :value="status.overview?.activeTorrents || 0" :value-style="{ color: '#1890ff' }">
            <template #prefix><ThunderboltOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('seeding.pausedTorrents')" :value="status.overview?.pausedTorrents || 0" :value-style="{ color: '#faad14' }">
            <template #prefix><CloudDownloadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('seeding.totalTorrents')" :value="status.overview?.totalTorrents || 0" :value-style="{ color: '#52c41a' }">
            <template #prefix><CloudUploadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('seeding.uptime')" :value="formatDurationSec(status.uptimeSeconds || 0)" :value-style="{ color: '#722ed1' }">
            <template #prefix><TrophyOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('seeding.configs')" style="margin-bottom: 24px">
      <div style="margin-bottom: 12px; display: flex; justify-content: flex-end">
        <a-button type="primary" @click="openConfigModal()">
          <template #icon><PlusOutlined /></template>
          新建刷流配置
        </a-button>
      </div>
      <a-table
        :columns="configColumns"
        :data-source="configs"
        :loading="configsLoading"
        :pagination="false"
        row-key="id"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'enabled'">
            <a-switch :checked="record.enabled" @change="(v: boolean) => toggleConfig(record, v)" />
          </template>
          <template v-if="column.key === 'disk_protect_enabled'">
            <a-tag :color="record.disk_protect_enabled ? 'green' : 'default'">{{ record.disk_protect_enabled ? t('common.on') : t('common.off') }}</a-tag>
          </template>
          <template v-if="column.key === 'actions'">
            <a-space>
              <a-button type="link" size="small" @click="openConfigModal(record)">{{ t('common.edit') }}</a-button>
              <a-popconfirm :title="t('seeding.deleteConfigConfirm')" @confirm="handleDeleteConfig(record.id)">
                <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-card :title="t('seeding.activeTorrents')">
      <a-table
        :columns="columns"
        :data-source="torrents"
        :loading="loading"
        :pagination="{ pageSize: 20, showSizeChanger: true }"
        row-key="id"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <a-badge
              :status="record.status === 'seeding' ? 'success' : 'warning'"
              :text="record.status"
            />
          </template>
          <template v-if="column.key === 'actions'">
            <a-space>
              <a-button
                v-if="record.status !== 'seeding'"
                type="link"
                size="small"
                @click="handleResumeRecord(record.id)"
              >
                {{ t('common.resume') }}
              </a-button>
              <a-button
                v-if="record.status === 'seeding'"
                type="link"
                size="small"
                @click="handlePauseRecord(record.id)"
              >
                {{ t('common.pause') }}
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal
      v-model:open="configModalVisible"
      :title="editingConfig ? t('seeding.editConfig') : t('seeding.addConfig')"
      @ok="handleConfigSubmit"
      :confirm-loading="configSubmitting"
      width="560px"
    >
      <a-form :model="configForm" layout="vertical">
        <a-form-item :label="t('seeding.downloaderId')">
          <a-select
            v-model:value="configForm.clientId"
            :placeholder="t('seeding.pleaseInputDownloaderId')"
            :disabled="!!editingConfig"
            :options="downloaderOptions"
            show-search
            :filter-option="(input: string, option: any) => option.label.toLowerCase().includes(input.toLowerCase())"
          />
        </a-form-item>
        <a-form-item :label="t('common.enable')">
          <a-switch v-model:checked="configForm.enabled" />
        </a-form-item>
        <a-form-item :label="t('seeding.autoDeleteCron')">
          <a-input v-model:value="configForm.autoDeleteCron" placeholder="*/30 * * * *" />
        </a-form-item>
        <a-form-item :label="t('seeding.mainDataCron')">
          <a-input v-model:value="configForm.mainDataCron" placeholder="*/10 * * * *" />
        </a-form-item>
        <a-form-item :label="t('seeding.diskProtect')">
          <a-switch v-model:checked="configForm.diskProtectEnabled" />
        </a-form-item>
        <a-form-item :label="t('seeding.minDiskSpaceGB')" v-if="configForm.diskProtectEnabled">
          <a-input-number v-model:value="configForm.minDiskSpaceGB" :min="1" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('seeding.maxActiveUploads')">
          <a-input-number v-model:value="configForm.maxActiveUploads" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('seeding.maxActiveDownloads')">
          <a-input-number v-model:value="configForm.maxActiveDownloads" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('seeding.superSeedingDefault')">
          <a-switch v-model:checked="configForm.superSeedingDefault" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import {
  ThunderboltOutlined,
  CloudUploadOutlined,
  CloudDownloadOutlined,
  TrophyOutlined,
  PlusOutlined,
} from '@ant-design/icons-vue'
import { seedingApi } from '@/api/seeding'
import { downloadersApi } from '@/api/downloaders'
import { formatDurationSec } from '@/utils/format'

const { t } = useI18n()
const loading = ref(false)
const configsLoading = ref(false)
const status = ref<any>({})
const torrents = ref<any[]>([])
const configs = ref<any[]>([])
const downloaderOptions = ref<{label: string, value: string}[]>([])

const configModalVisible = ref(false)
const configSubmitting = ref(false)
const editingConfig = ref<any>(null)
const configForm = reactive({
  clientId: '',
  enabled: true,
  autoDeleteCron: '*/30 * * * *',
  mainDataCron: '*/10 * * * *',
  diskProtectEnabled: false,
  minDiskSpaceGB: 50,
  maxActiveUploads: 0,
  maxActiveDownloads: 0,
  superSeedingDefault: false,
})

const columns = [
  { title: 'Torrent ID', dataIndex: 'torrent_id', key: 'torrent_id', ellipsis: true },
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: '客户端', dataIndex: 'client_id', key: 'client_id', width: 100 },
  { title: 'InfoHash', dataIndex: 'info_hash', key: 'info_hash', ellipsis: true },
  { title: '状态', key: 'status', width: 100 },
  { title: '免费', dataIndex: 'is_free', key: 'is_free', width: 60 },
  { title: 'HR', dataIndex: 'has_hr', key: 'has_hr', width: 60 },
  { title: '来源', dataIndex: 'source', key: 'source', width: 80 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: '操作', key: 'actions', width: 100 },
]

const configColumns = [
  { title: '下载器 ID', dataIndex: 'client_id', key: 'client_id', width: 120 },
  { title: '删种 Cron', dataIndex: 'auto_delete_cron', key: 'auto_delete_cron', width: 140 },
  { title: '主数据 Cron', dataIndex: 'maindata_cron', key: 'maindata_cron', width: 140 },
  { title: '磁盘保护', key: 'disk_protect_enabled', width: 90 },
  { title: '最小空间', dataIndex: 'min_disk_space_gb', key: 'min_disk_space_gb', width: 100 },
  { title: '启用', key: 'enabled', width: 80 },
  { title: '操作', key: 'actions', width: 140 },
]

async function fetchData() {
  loading.value = true
  try {
    const [statusResp, torrentsResp] = await Promise.all([
      seedingApi.getStatus(),
      seedingApi.getTorrents(1, 20),
    ])
    status.value = statusResp.data.data || {}
    torrents.value = torrentsResp.data.data?.items || torrentsResp.data.data || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function fetchConfigs() {
  configsLoading.value = true
  try {
    const resp = await seedingApi.getConfig()
    const data = resp.data.data
    configs.value = Array.isArray(data) ? data : data?.items || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    configsLoading.value = false
  }
}

function openConfigModal(record?: any) {
  editingConfig.value = record || null
  if (record) {
    Object.assign(configForm, {
      clientId: record.client_id || '',
      enabled: record.enabled || false,
      autoDeleteCron: record.auto_delete_cron || '*/30 * * * *',
      mainDataCron: record.maindata_cron || '*/10 * * * *',
      diskProtectEnabled: record.disk_protect_enabled || false,
      minDiskSpaceGB: record.min_disk_space_gb || 50,
      maxActiveUploads: record.max_active_uploads || 0,
      maxActiveDownloads: record.max_active_downloads || 0,
      superSeedingDefault: record.super_seeding_default || false,
    })
  } else {
    Object.assign(configForm, {
      clientId: '',
      enabled: true,
      autoDeleteCron: '*/30 * * * *',
      mainDataCron: '*/10 * * * *',
      diskProtectEnabled: false,
      minDiskSpaceGB: 50,
      maxActiveUploads: 0,
      maxActiveDownloads: 0,
      superSeedingDefault: false,
    })
  }
  configModalVisible.value = true
}

async function handleConfigSubmit() {
  if (!configForm.clientId) {
    message.error(t('seeding.pleaseInputDownloaderId'))
    return
  }
  configSubmitting.value = true
  try {
    if (editingConfig.value) {
      await seedingApi.updateConfig(editingConfig.value.id, configForm)
    } else {
      await seedingApi.createConfig(configForm)
    }
    message.success(t('common.operationSuccess'))
    configModalVisible.value = false
    fetchConfigs()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    configSubmitting.value = false
  }
}

async function handleDeleteConfig(configId: number) {
  try {
    await seedingApi.deleteConfig(configId)
    message.success(t('common.deleted'))
    fetchConfigs()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function toggleConfig(record: any, checked: boolean) {
  try {
    await seedingApi.updateConfig(record.id, { enabled: checked })
    message.success(t('common.statusUpdated'))
    fetchConfigs()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function handleResumeRecord(recordId: number) {
  try {
    await seedingApi.resumeRecord(recordId)
    message.success(t('common.resumed'))
    fetchData()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function handlePauseRecord(recordId: number) {
  try {
    await seedingApi.pauseRecord(recordId)
    message.success(t('common.paused'))
    fetchData()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function fetchDownloaders() {
  try {
    const resp = await downloadersApi.list(1, 100)
    const items = resp.data.data?.items || resp.data.data || []
    downloaderOptions.value = items.map((d: any) => ({
      label: d.name || d.id,
      value: d.name || d.id,
    }))
  } catch (_e: any) {}
}

onMounted(() => {
  fetchData()
  fetchConfigs()
  fetchDownloaders()
})
</script>
