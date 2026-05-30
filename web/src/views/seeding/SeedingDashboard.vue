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
          {{ t('seeding.addConfig') }}
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
          <template v-if="column.key === 'title'">
            <a v-if="record.detail_url" :href="record.detail_url" target="_blank" style="color: #1890ff">{{ record.title || record.torrent_id }}</a>
            <span v-else>{{ record.title || record.torrent_id }}</span>
          </template>
          <template v-if="column.key === 'info_hash'">
            <span style="cursor:pointer;font-family:monospace;font-size:12px" @click="copyHash(record.info_hash)">{{ record.info_hash }}</span>
          </template>
          <template v-if="column.key === 'status'">
              <a-badge
                :status="record.status === 'seeding' ? 'success' : 'warning'"
                :text="translateSeedingStatus(record.status)"
              />
            </template>
          <template v-if="column.key === 'is_free'">
            <a-tag :color="record.is_free ? 'green' : 'default'">{{ record.is_free ? t('common.yes') : t('common.no') }}</a-tag>
          </template>
          <template v-if="column.key === 'has_hr'">
            <a-tag :color="record.has_hr ? 'red' : 'default'">{{ record.has_hr ? t('common.yes') : t('common.no') }}</a-tag>
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
      :confirm-loading="configSubmitting"
      width="560px"
      @ok="handleConfigSubmit"
    >
      <a-form :model="configForm" layout="vertical">
        <a-form-item :label="t('seeding.downloaderId')">
          <a-select
            v-model:value="configForm.clientId"
            :placeholder="t('seeding.pleaseInputDownloaderId')"
            :disabled="!!editingConfig"
            :options="downloaderOptions"
            show-search
            :filter-option="(input: string, option: { label: string }) => option.label.toLowerCase().includes(input.toLowerCase())"
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
        <a-form-item v-if="configForm.diskProtectEnabled" :label="t('seeding.minDiskSpaceGB')">
          <a-input-number v-model:value="configForm.minDiskSpaceGB" :min="1" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('seeding.maxActiveUploads')">
          <a-input-number v-model:value="configForm.maxActiveUploads" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('seeding.maxActiveDownloads')">
          <a-input-number v-model:value="configForm.maxActiveDownloads" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('seeding.maxActiveSeeding')">
          <a-input-number v-model:value="configForm.maxActiveSeeding" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('seeding.superSeedingDefault')">
          <a-switch v-model:checked="configForm.superSeedingDefault" />
        </a-form-item>
        <a-collapse :bordered="false" style="margin-top: 8px; background: transparent">
          <a-collapse-panel key="advanced" header="高级选项">
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="拒绝规则 ID 列表">
                  <a-input v-model:value="configForm.rejectRuleIds" placeholder="1,2,3" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="删种规则">
                  <a-select
                    v-model:value="configForm.deleteRuleIds"
                    mode="multiple"
                    :options="deleteRuleOptions"
                    placeholder="选择绑定的删种规则"
                    allow-clear
                  />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="管理范围">
                  <a-select v-model:value="configForm.scope">
                    <a-select-option value="managed">managed</a-select-option>
                    <a-select-option value="all">all</a-select-option>
                    <a-select-option value="custom">custom</a-select-option>
                  </a-select>
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="FitTime 检查间隔(ms)">
                  <a-input-number v-model:value="configForm.fitTimeCheckMs" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="紧急缓冲比例">
                  <a-input-number v-model:value="configForm.emergencyBuffer" :min="0" :max="1" :step="0.05" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="预过滤">
                  <a-switch v-model:checked="configForm.preFilterEnabled" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="最低磁盘百分比">
                  <a-input-number v-model:value="configForm.minDiskSpacePercent" :min="0" :max="100" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="空间告警">
                  <a-switch v-model:checked="configForm.spaceAlarmEnabled" />
                </a-form-item>
              </a-col>
              <a-col v-if="configForm.spaceAlarmEnabled" :span="12">
                <a-form-item label="告警阈值(GB)">
                  <a-input-number v-model:value="configForm.spaceAlarmGb" :min="1" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="增强批次大小">
                  <a-input-number v-model:value="configForm.enhancementBatchSize" :min="1" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="增强缓存 TTL(秒)">
                  <a-input-number v-model:value="configForm.enhancementCacheTtl" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="EMA 平滑系数">
                  <a-input-number v-model:value="configForm.emaAlpha" :min="0" :max="1" :step="0.05" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="活跃时间窗口">
                  <a-input v-model:value="configForm.activeTimeWindows" placeholder="08:00-22:00" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="归档粒度">
                  <a-select v-model:value="configForm.archiveGranularity">
                    <a-select-option value="daily">daily</a-select-option>
                    <a-select-option value="hourly">hourly</a-select-option>
                    <a-select-option value="weekly">weekly</a-select-option>
                  </a-select>
                </a-form-item>
              </a-col>
            </a-row>
            <a-form-item label="清理评分权重(JSON)">
              <a-textarea v-model:value="configForm.cleanupScoreWeights" :rows="2" placeholder="JSON: seed_time, ratio, ..." />
            </a-form-item>
          </a-collapse-panel>
        </a-collapse>
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
import { seedingApi, deleteRulesApi } from '@/api/seeding'
import { downloadersApi } from '@/api/downloaders'
import { formatDurationSec, formatTime, copyToClipboard } from '@/utils/format'
import { useEnumLabels } from '@/utils/enumLabels'
import type { ClientConfig, SeedingClientConfig, SeedingTorrentRecord, DeleteRule } from '@/api/types'

interface SeedingStatusData {
  overview?: {
    activeTorrents?: number
    pausedTorrents?: number
    totalTorrents?: number
  }
  uptimeSeconds?: number
  running?: boolean
  lastRunAt?: string | null
  [key: string]: unknown
}

const { t } = useI18n()
const { translateSeedingStatus } = useEnumLabels()
const loading = ref(false)
const configsLoading = ref(false)
const status = ref<SeedingStatusData>({})
const torrents = ref<SeedingTorrentRecord[]>([])
const configs = ref<SeedingClientConfig[]>([])
const downloaderOptions = ref<{label: string, value: string}[]>([])
const deleteRuleOptions = ref<{label: string, value: number}[]>([])

const configModalVisible = ref(false)
const configSubmitting = ref(false)
const editingConfig = ref<SeedingClientConfig | null>(null)
const configForm = reactive({
  clientId: '',
  enabled: true,
  autoDeleteCron: '*/30 * * * *',
  mainDataCron: '*/10 * * * *',
  diskProtectEnabled: false,
  minDiskSpaceGB: 50,
  maxActiveUploads: 0,
  maxActiveDownloads: 0,
  maxActiveSeeding: 100,
  superSeedingDefault: false,
  rejectRuleIds: '',
  deleteRuleIds: [] as number[],
  fitTimeCheckMs: 2000,
  emergencyBuffer: 0.2,
  spaceAlarmEnabled: false,
  spaceAlarmGb: 10,
  minDiskSpacePercent: 0,
  scope: 'managed',
  preFilterEnabled: true,
  enhancementBatchSize: 20,
  enhancementCacheTtl: 600,
  activeTimeWindows: '',
  emaAlpha: 0.1,
  cleanupScoreWeights: '',
  archiveGranularity: 'daily',
})

const columns = [
  { title: t('seeding.fieldTorrentName'), dataIndex: 'title', key: 'title', ellipsis: true, width: 260 },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 60 },
  { title: t('seeding.fieldTorrentID'), dataIndex: 'torrent_id', key: 'torrent_id', ellipsis: true, width: 60 },
  { title: t('seeding.client'), dataIndex: 'client_id', key: 'client_id', width: 50 },
  { title: 'InfoHash', dataIndex: 'info_hash', key: 'info_hash', ellipsis: true, width: 200 },
  { title: t('common.status'), key: 'status', width: 80 },
  { title: t('seeding.isFree'), key: 'is_free', width: 60 },
  { title: 'HR', key: 'has_hr', width: 50 },
  { title: t('seeding.source'), dataIndex: 'source', key: 'source', width: 70 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 150, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 80 },
]

const configColumns = [
  { title: t('seeding.downloaderId'), dataIndex: 'client_id', key: 'client_id', width: 120 },
  { title: t('seeding.autoDeleteCron'), dataIndex: 'auto_delete_cron', key: 'auto_delete_cron', width: 140 },
  { title: t('seeding.mainDataCron'), dataIndex: 'maindata_cron', key: 'maindata_cron', width: 140 },
  { title: t('seeding.diskProtect'), key: 'disk_protect_enabled', width: 90 },
  { title: t('seeding.minDiskSpaceGB'), dataIndex: 'min_disk_space_gb', key: 'min_disk_space_gb', width: 100 },
  { title: '删种规则', dataIndex: 'delete_rule_ids', key: 'delete_rule_ids', width: 120 },
  { title: t('common.enable'), key: 'enabled', width: 80 },
  { title: t('common.actions'), key: 'actions', width: 140 },
]

async function fetchData() {
  loading.value = true
  try {
    const [statusResp, torrentsResp] = await Promise.all([
      seedingApi.getStatus(),
      seedingApi.getTorrents(1, 20),
    ])
    status.value = statusResp.data.data as SeedingStatusData || {}
    torrents.value = (torrentsResp.data.data?.items || torrentsResp.data.data || []) as SeedingTorrentRecord[]
  } catch (e: unknown) {
    message.error((e as Error).message)
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
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    configsLoading.value = false
  }
}

function openConfigModal(record?: SeedingClientConfig) {
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
      maxActiveSeeding: record.max_active_seeding || 100,
      superSeedingDefault: record.super_seeding_default || false,
      rejectRuleIds: record.reject_rule_ids || '',
      deleteRuleIds: record.delete_rule_ids ? record.delete_rule_ids.split(',').map(Number).filter(n => !isNaN(n)) : [],
      fitTimeCheckMs: record.fit_time_check_ms ?? 2000,
      emergencyBuffer: record.emergency_buffer ?? 0.2,
      spaceAlarmEnabled: record.space_alarm_enabled || false,
      spaceAlarmGb: record.space_alarm_gb ?? 10,
      minDiskSpacePercent: record.min_disk_space_percent ?? 0,
      scope: record.scope || 'managed',
      preFilterEnabled: record.pre_filter_enabled ?? true,
      enhancementBatchSize: record.enhancement_batch_size ?? 20,
      enhancementCacheTtl: record.enhancement_cache_ttl ?? 600,
      activeTimeWindows: record.active_time_windows || '',
      emaAlpha: record.ema_alpha ?? 0.1,
      cleanupScoreWeights: record.cleanup_score_weights || '',
      archiveGranularity: record.archive_granularity || 'daily',
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
      maxActiveSeeding: 100,
      superSeedingDefault: false,
      rejectRuleIds: '',
      deleteRuleIds: [] as number[],
      fitTimeCheckMs: 2000,
      emergencyBuffer: 0.2,
      spaceAlarmEnabled: false,
      spaceAlarmGb: 10,
      minDiskSpacePercent: 0,
      scope: 'managed',
      preFilterEnabled: true,
      enhancementBatchSize: 20,
      enhancementCacheTtl: 600,
      activeTimeWindows: '',
      emaAlpha: 0.1,
      cleanupScoreWeights: '',
      archiveGranularity: 'daily',
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
    const payload = {
      ...configForm,
      deleteRuleIds: configForm.deleteRuleIds.length > 0 ? configForm.deleteRuleIds.join(',') : '',
    }
    if (editingConfig.value) {
      await seedingApi.updateConfig(editingConfig.value.id, payload)
    } else {
      await seedingApi.createConfig(payload)
    }
    message.success(t('common.operationSuccess'))
    configModalVisible.value = false
    fetchConfigs()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    configSubmitting.value = false
  }
}

async function handleDeleteConfig(configId: number) {
  try {
    await seedingApi.deleteConfig(configId)
    message.success(t('common.deleted'))
    fetchConfigs()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function toggleConfig(record: SeedingClientConfig, checked: boolean) {
  try {
    await seedingApi.updateConfig(record.id, { enabled: checked })
    message.success(t('common.statusUpdated'))
    fetchConfigs()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function handleResumeRecord(recordId: number) {
  try {
    await seedingApi.resumeRecord(recordId)
    message.success(t('common.resumed'))
    fetchData()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function handlePauseRecord(recordId: number) {
  try {
    await seedingApi.pauseRecord(recordId)
    message.success(t('common.paused'))
    fetchData()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function fetchDownloaders() {
  try {
    const resp = await downloadersApi.list(1, 100)
    const items = resp.data.data?.items || resp.data.data || []
    downloaderOptions.value = items.map((d: ClientConfig) => ({
      label: d.name || String(d.id),
      value: d.name || String(d.id),
    }))
  } catch {
  }
}

async function fetchDeleteRules() {
  try {
    const resp = await deleteRulesApi.list()
    const items = resp.data.data?.items || []
    deleteRuleOptions.value = items.map((r: DeleteRule) => ({
      label: `#${r.id} ${r.alias || r.type} (优先级:${r.priority})`,
      value: r.id,
    }))
  } catch {
  }
}

function copyHash(text: string) {
  copyToClipboard(text)
  message.success(t('common.copied'))
}

onMounted(() => {
  fetchData()
  fetchConfigs()
  fetchDownloaders()
  fetchDeleteRules()
})
</script>
