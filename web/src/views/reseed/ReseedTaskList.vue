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
        showTotal: (total: number) => t('common.totalCount', { count: total }),
      }"
      row-key="id"
      @change="(pag: { current: number; pageSize: number }) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ translateReseedStatus(record.status) }}</a-tag>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/reseed/tasks/${record.id}`)">{{ t('common.detail') }}</a-button>
            <a-button type="link" size="small" @click="openModal(record)"><EditOutlined /> {{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" :disabled="record.status === 'running'" @click="triggerTask(record.id)">{{ t('common.trigger') }}</a-button>
            <a-button type="link" size="small" :disabled="record.status !== 'running'" @click="cancelTask(record.id)">{{ t('common.cancel') }}</a-button>
            <a-popconfirm :title="t('reseed.deleteTaskConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingTask ? t('common.edit') : t('reseed.createReseedTask')"
      :confirm-loading="submitting"
      width="800px"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('reseed.taskName')" name="name" :rules="[{ required: true, message: t('reseed.pleaseEnterTaskName') }]">
              <a-input v-model:value="form.name" :placeholder="t('reseed.taskNamePlaceholder')" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('reseed.enabled')" name="enabled">
              <a-switch v-model:checked="form.enabled" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('reseed.clientId')" name="clientIds" :rules="[{ required: true, message: t('reseed.pleaseEnterClientId') }]">
              <a-select v-model:value="form.clientIds" :placeholder="t('reseed.clientIdPlaceholder')" :loading="downloadersLoading" show-search :filter-option="filterOption" allow-clear>
                <a-select-option v-for="d in downloaders" :key="d.id" :value="String(d.id)" :label="`${d.name}（${d.type}）`">
                  {{ d.name }}（{{ d.type }}）
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('reseed.reseedMode')" name="engineMode">
              <a-select v-model:value="form.engineMode" @change="onEngineModeChange">
                <a-select-option value="seed_feature">{{ t('reseed.seedFeature') }}</a-select-option>
                <a-select-option value="iyuu_cloud">{{ t('reseed.iyuuCloud') }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item name="sourceSiteIds" :rules="[{ required: true, message: t('reseed.pleaseEnterSourceSite') }]">
              <template #label>
                {{ t('reseed.sourceSiteIds') }}
                <a-button type="link" size="small" @click="selectAllSource">{{ t('common.selectAll') }}</a-button>
                <a-button type="link" size="small" @click="form.sourceSiteIds = []">{{ t('common.deselectAll') }}</a-button>
              </template>
              <a-select v-model:value="form.sourceSiteIds" mode="multiple" :placeholder="t('reseed.sourceSiteIdsPlaceholder')" :loading="sitesLoading" show-search :filter-option="filterOption" allow-clear>
                <a-select-option v-for="s in sourceSites" :key="s.id" :value="String(s.id)" :label="`${s.name}（${s.domain}）`">
                  {{ s.name }}（{{ s.domain }}）
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item name="targetSiteIds" :rules="[{ required: true, message: t('reseed.pleaseEnterTargetSite') }]">
              <template #label>
                {{ t('reseed.targetSiteIds') }}
                <a-button type="link" size="small" @click="selectAllTarget">{{ t('common.selectAll') }}</a-button>
                <a-button type="link" size="small" @click="form.targetSiteIds = []">{{ t('common.deselectAll') }}</a-button>
              </template>
              <a-select v-model:value="form.targetSiteIds" mode="multiple" :placeholder="t('reseed.targetSiteIdsPlaceholder')" :loading="sitesLoading" show-search :filter-option="filterOption" allow-clear>
                <a-select-option v-for="s in targetSites" :key="s.id" :value="String(s.id)" :label="`${s.name}（${s.domain}）`">
                  {{ s.name }}（{{ s.domain }}）
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-divider>{{ t('reseed.scheduleAndLimits') }}</a-divider>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('reseed.schedule')" name="schedule">
              <a-input v-model:value="form.schedule" placeholder="0 */6 * * *" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('reseed.maxInjectionsPerRun')" name="maxInjectionsPerRun">
              <a-input-number v-model:value="form.maxInjectionsPerRun" :min="0" :max="10000" style="width: 100%" placeholder="0" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('reseed.sizeTolerancePercent')" name="sizeTolerancePercent">
              <a-input-number v-model:value="form.sizeTolerancePercent" :min="0" :max="100" :step="0.1" style="width: 100%" placeholder="1.0" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('reseed.confidenceThreshold')" name="confidenceThreshold">
              <a-input-number v-model:value="form.confidenceThreshold" :min="0" :max="1" :step="0.05" style="width: 100%" placeholder="0.7" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('reseed.reseedCategory')" name="reseedCategory">
              <a-input v-model:value="form.reseedCategory" placeholder="cross-seed" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="辅种标签" name="reseedTags">
              <a-input v-model:value="form.reseedTags" placeholder="reseed,pt-forward" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-divider>{{ t('reseed.excludeRules') }}</a-divider>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('reseed.excludeTargetSite')" name="targetSiteExcludes">
              <a-input v-model:value="form.targetSiteExcludes" placeholder="site1,site2" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('reseed.excludeReleaseGroup')" name="releaseGroupExcludes">
              <a-input v-model:value="form.releaseGroupExcludes" placeholder="group1,group2" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('reseed.excludeCategory')" name="categoryExcludes">
              <a-input v-model:value="form.categoryExcludes" placeholder="cat1,cat2" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('reseed.excludeTitleKeyword')" name="titleKeywordExcludes">
              <a-input v-model:value="form.titleKeywordExcludes" placeholder="keyword1,keyword2" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-collapse :bordered="false" style="margin-top: 8px; background: transparent">
          <a-collapse-panel key="advanced" :header="t('common.advancedOptions')">
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('reseed.fallbackMatch')" name="fallbackEnabled">
                  <a-switch v-model:checked="form.fallbackEnabled" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('reseed.maxFallbacks')" name="maxFallbacks">
                  <a-input-number v-model:value="form.maxFallbacks" :min="0" :max="20" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('reseed.injectionIntervalS')" name="injectionIntervalS">
                  <a-input-number v-model:value="form.injectionIntervalS" :min="0" :max="300" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('reseed.injectionJitterS')" name="injectionJitterS">
                  <a-input-number v-model:value="form.injectionJitterS" :min="0" :max="60" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('reseed.injectionConcurrency')" name="injectionConcurrency">
                  <a-input-number v-model:value="form.injectionConcurrency" :min="1" :max="20" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('reseed.scanConcurrency')" name="scanConcurrency">
                  <a-input-number v-model:value="form.scanConcurrency" :min="1" :max="20" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('reseed.maxRetries')" name="maxRetries">
                  <a-input-number v-model:value="form.maxRetries" :min="0" :max="100" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('reseed.retryIntervalH')" name="retryIntervalH">
                  <a-input-number v-model:value="form.retryIntervalH" :min="0" :max="720" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
          </a-collapse-panel>
        </a-collapse>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { PlusOutlined, EditOutlined } from '@ant-design/icons-vue'
import { reseedApi } from '@/api/reseed'
import { downloadersApi } from '@/api/downloaders'
import { sitesApi } from '@/api/sites'
import { usePagination } from '@/composables/usePagination'
import { useEnumLabels } from '@/utils/enumLabels'

import type { ReseedTask, ClientConfig, Site } from '@/api/types'
import { formatTime } from '@/utils/format'

const { t } = useI18n()
const { translateReseedStatus } = useEnumLabels()
const modalVisible = ref(false)
const submitting = ref(false)
const editingTask = ref<ReseedTask | null>(null)

const reseedModeMatchMethods: Record<string, string> = {
  seed_feature: 'pieces_hash,file_tree,size_title,fingerprint',
  iyuu_cloud: 'iyuu',
}

function onEngineModeChange(mode: string) {
  form.matchMethods = reseedModeMatchMethods[mode] || ''
}

const downloaders = ref<ClientConfig[]>([])
const downloadersLoading = ref(false)
const sites = ref<Site[]>([])
const sitesLoading = ref(false)

const sourceSites = computed(() => sites.value.filter(s => s.isSource))
const targetSites = computed(() => sites.value.filter(s => s.isTarget))

async function fetchDownloaders() {
  downloadersLoading.value = true
  try {
    const resp = await downloadersApi.list(1, 200)
    const items: ClientConfig[] = resp.data?.data?.items || resp.data?.data || []
    downloaders.value = items.filter(d => d.role === 'download' || d.role === 'master_reseed' || d.role === 'reseed')
  } catch {
    downloaders.value = []
  } finally {
    downloadersLoading.value = false
  }
}

async function fetchSites() {
  sitesLoading.value = true
  try {
    const resp = await sitesApi.list(1, 200)
    sites.value = resp.data?.data?.items || resp.data?.data || []
  } catch {
    sites.value = []
  } finally {
    sitesLoading.value = false
  }
}

function filterOption(input: string, option: any) {
  const label = option.label || option.value || ''
  return String(label).toLowerCase().includes(input.toLowerCase())
}

function selectAllSource() {
  form.sourceSiteIds = sourceSites.value.map(s => String(s.id))
}

function selectAllTarget() {
  form.targetSiteIds = targetSites.value.map(s => String(s.id))
}

const defaultForm = {
  name: '',
  enabled: false,
  clientIds: '',
  sourceSiteIds: [] as string[],
  targetSiteIds: [] as string[],
  sizeTolerancePercent: 1.0,
  confidenceThreshold: 0.7,
  schedule: '0 */6 * * *',
  maxInjectionsPerRun: 0,
  reseedCategory: 'cross-seed',
  reseedTags: 'reseed,pt-forward',
  targetSiteExcludes: '',
  releaseGroupExcludes: '',
  categoryExcludes: '',
  titleKeywordExcludes: '',
  matchMethods: '',
  fallbackEnabled: true,
  maxFallbacks: 3,
  engineMode: 'seed_feature',
  injectionIntervalS: 1,
  injectionJitterS: 5,
  injectionConcurrency: 10,
  scanConcurrency: 5,
  maxRetries: 3,
  retryIntervalH: 24,
}

const form = reactive({ ...defaultForm })

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('common.name'), dataIndex: 'name', key: 'name' },
  { title: t('reseed.client'), dataIndex: 'client_ids', key: 'client_ids', ellipsis: true },
  { title: t('common.status'), dataIndex: 'status', key: 'status', width: 100 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 240 },
]

const pagination = usePagination((page, size) => reseedApi.listTasks(page, size))

function statusColor(status: string) {
  const map: Record<string, string> = { pending: 'blue', running: 'green', completed: 'default', failed: 'red' }
  return map[status] || 'default'
}

function openModal(record?: ReseedTask) {
  editingTask.value = record || null
  if (record) {
    Object.assign(form, {
      name: record.name || '',
      enabled: record.enabled || false,
      clientIds: record.client_ids || '',
      sourceSiteIds: record.source_site_ids ? record.source_site_ids.split(',').map((s: string) => s.trim()).filter(Boolean) : [],
      targetSiteIds: record.target_site_ids ? record.target_site_ids.split(',').map((s: string) => s.trim()).filter(Boolean) : [],
      sizeTolerancePercent: record.size_tolerance_percent ?? 1.0,
      confidenceThreshold: record.confidence_threshold ?? 0.7,
      schedule: record.schedule || '0 */6 * * *',
      maxInjectionsPerRun: record.max_injections_per_run ?? 0,
      reseedCategory: record.reseed_category || 'cross-seed',
      reseedTags: record.reseed_tags || 'reseed,pt-forward',
      targetSiteExcludes: record.target_site_excludes || '',
      releaseGroupExcludes: record.release_group_excludes || '',
      categoryExcludes: record.category_excludes || '',
      titleKeywordExcludes: record.title_keyword_excludes || '',
      matchMethods: record.match_methods || '',
      fallbackEnabled: record.fallback_enabled ?? true,
      maxFallbacks: record.max_fallbacks ?? 3,
      engineMode: record.engine_mode || 'seed_feature',
      injectionIntervalS: record.injection_interval_s ?? 1,
      injectionJitterS: record.injection_jitter_s ?? 5,
      injectionConcurrency: record.injection_concurrency ?? 10,
      scanConcurrency: record.scan_concurrency ?? 5,
      maxRetries: record.max_retries ?? 3,
      retryIntervalH: record.retry_interval_h ?? 24,
    })
  } else {
    Object.assign(form, { ...defaultForm, sourceSiteIds: [], targetSiteIds: [] })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    const payload = {
      ...form,
      sourceSiteIds: (form.sourceSiteIds as string[]).join(','),
      targetSiteIds: (form.targetSiteIds as string[]).join(','),
    }
    if (editingTask.value) {
      await reseedApi.updateTask(editingTask.value.id as number, payload)
      message.success(t('common.operationSuccess'))
    } else {
      await reseedApi.createTask(payload)
      message.success(t('reseed.taskCreated'))
    }
    modalVisible.value = false
    pagination.fetch()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    submitting.value = false
  }
}

async function triggerTask(id: number) {
  try {
    await reseedApi.triggerTask(id)
    message.success(t('reseed.taskTriggered'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function cancelTask(id: number) {
  try {
    await reseedApi.cancelTask(id)
    message.success(t('reseed.taskCancelled'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function handleDelete(id: number) {
  try {
    await reseedApi.deleteTask(id)
    message.success(t('common.deleted'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

onMounted(() => {
  pagination.fetch()
  fetchDownloaders()
  fetchSites()
})
</script>
