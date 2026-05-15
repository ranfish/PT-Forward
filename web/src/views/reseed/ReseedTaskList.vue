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
          <a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag>
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
      @ok="handleSubmit"
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
        <a-form-item label="Enabled" name="enabled">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
        <a-form-item label="Size Tolerance %" name="sizeTolerancePercent">
          <a-input-number v-model:value="form.sizeTolerancePercent" :min="0" :max="100" :step="0.1" style="width: 100%" placeholder="1.0" />
        </a-form-item>
        <a-form-item label="Confidence Threshold" name="confidenceThreshold">
          <a-input-number v-model:value="form.confidenceThreshold" :min="0" :max="1" :step="0.05" style="width: 100%" placeholder="0.7" />
        </a-form-item>
        <a-form-item label="Schedule" name="schedule">
          <a-input v-model:value="form.schedule" placeholder="0 */6 * * *" />
        </a-form-item>
        <a-form-item label="Max Injections Per Run" name="maxInjectionsPerRun">
          <a-input-number v-model:value="form.maxInjectionsPerRun" :min="1" :max="1000" style="width: 100%" placeholder="100" />
        </a-form-item>
        <a-form-item label="Reseed Category" name="reseedCategory">
          <a-input v-model:value="form.reseedCategory" placeholder="cross-seed" />
        </a-form-item>
        <a-form-item label="排除目标站" name="targetSiteExcludes">
          <a-input v-model:value="form.targetSiteExcludes" placeholder="site1,site2" />
        </a-form-item>
        <a-form-item label="排除发布组" name="releaseGroupExcludes">
          <a-input v-model:value="form.releaseGroupExcludes" placeholder="group1,group2" />
        </a-form-item>
        <a-form-item label="排除分类" name="categoryExcludes">
          <a-input v-model:value="form.categoryExcludes" placeholder="cat1,cat2" />
        </a-form-item>
        <a-form-item label="排除标题关键词" name="titleKeywordExcludes">
          <a-input v-model:value="form.titleKeywordExcludes" placeholder="keyword1,keyword2" />
        </a-form-item>
        <a-form-item label="匹配方法" name="matchMethods">
          <a-input v-model:value="form.matchMethods" placeholder="hash,title,size" />
        </a-form-item>
        <a-collapse :bordered="false" style="margin-top: 8px; background: transparent">
          <a-collapse-panel key="advanced" header="高级选项">
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="回退匹配" name="fallbackEnabled">
                  <a-switch v-model:checked="form.fallbackEnabled" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="最大回退数" name="maxFallbacks">
                  <a-input-number v-model:value="form.maxFallbacks" :min="0" :max="20" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-form-item label="引擎模式" name="engineMode">
              <a-select v-model:value="form.engineMode">
                <a-select-option value="e1_manual">e1_manual</a-select-option>
                <a-select-option value="e2_auto">e2_auto</a-select-option>
                <a-select-option value="e3_hybrid">e3_hybrid</a-select-option>
              </a-select>
            </a-form-item>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="注入间隔(秒)" name="injectionIntervalS">
                  <a-input-number v-model:value="form.injectionIntervalS" :min="0" :max="300" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="注入抖动(秒)" name="injectionJitterS">
                  <a-input-number v-model:value="form.injectionJitterS" :min="0" :max="60" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="注入并发" name="injectionConcurrency">
                  <a-input-number v-model:value="form.injectionConcurrency" :min="1" :max="20" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="扫描并发" name="scanConcurrency">
                  <a-input-number v-model:value="form.scanConcurrency" :min="1" :max="20" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="最大重试" name="maxRetries">
                  <a-input-number v-model:value="form.maxRetries" :min="0" :max="100" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="重试间隔(小时)" name="retryIntervalH">
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
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { PlusOutlined, EditOutlined } from '@ant-design/icons-vue'
import { reseedApi } from '@/api/reseed'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()
const modalVisible = ref(false)
const submitting = ref(false)
const editingTask = ref<Record<string, unknown> | null>(null)

const defaultForm = {
  name: '',
  enabled: false,
  clientIds: '',
  sourceSiteIds: '',
  targetSiteIds: '',
  sizeTolerancePercent: 1.0,
  confidenceThreshold: 0.7,
  schedule: '0 */6 * * *',
  maxInjectionsPerRun: 100,
  reseedCategory: 'cross-seed',
  targetSiteExcludes: '',
  releaseGroupExcludes: '',
  categoryExcludes: '',
  titleKeywordExcludes: '',
  matchMethods: '',
  fallbackEnabled: true,
  maxFallbacks: 3,
  engineMode: 'e1_manual',
  injectionIntervalS: 15,
  injectionJitterS: 5,
  injectionConcurrency: 3,
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
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: t('common.actions'), key: 'actions', width: 240 },
]

const pagination = usePagination((page, size) => reseedApi.listTasks(page, size))

function statusColor(status: string) {
  const map: Record<string, string> = { pending: 'blue', running: 'green', completed: 'default', failed: 'red' }
  return map[status] || 'default'
}

function openModal(record?: Record<string, unknown>) {
  editingTask.value = record || null
  if (record) {
    Object.assign(form, {
      name: record.name || '',
      enabled: record.enabled || false,
      clientIds: record.clientIds || record.client_ids || '',
      sourceSiteIds: record.sourceSiteIds || record.source_site_ids || '',
      targetSiteIds: record.targetSiteIds || record.target_site_ids || '',
      sizeTolerancePercent: record.sizeTolerancePercent ?? 1.0,
      confidenceThreshold: record.confidenceThreshold ?? 0.7,
      schedule: record.schedule || '0 */6 * * *',
      maxInjectionsPerRun: record.maxInjectionsPerRun ?? 100,
      reseedCategory: record.reseedCategory || 'cross-seed',
      targetSiteExcludes: record.targetSiteExcludes || record.target_site_excludes || '',
      releaseGroupExcludes: record.releaseGroupExcludes || record.release_group_excludes || '',
      categoryExcludes: record.categoryExcludes || record.category_excludes || '',
      titleKeywordExcludes: record.titleKeywordExcludes || record.title_keyword_excludes || '',
      matchMethods: record.matchMethods || record.match_methods || '',
      fallbackEnabled: record.fallbackEnabled ?? record.fallback_enabled ?? true,
      maxFallbacks: record.maxFallbacks ?? record.max_fallbacks ?? 3,
      engineMode: record.engineMode || record.engine_mode || 'e1_manual',
      injectionIntervalS: record.injectionIntervalS ?? record.injection_interval_s ?? 15,
      injectionJitterS: record.injectionJitterS ?? record.injection_jitter_s ?? 5,
      injectionConcurrency: record.injectionConcurrency ?? record.injection_concurrency ?? 3,
      scanConcurrency: record.scanConcurrency ?? record.scan_concurrency ?? 5,
      maxRetries: record.maxRetries ?? record.max_retries ?? 3,
      retryIntervalH: record.retryIntervalH ?? record.retry_interval_h ?? 24,
    })
  } else {
    Object.assign(form, { ...defaultForm })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingTask.value) {
      await reseedApi.updateTask(editingTask.value.id as number, form)
      message.success(t('common.operationSuccess'))
    } else {
      await reseedApi.createTask(form)
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

onMounted(() => pagination.fetch())
</script>
