<template>
  <div>
    <a-page-header :title="t('audit.title')">
      <template #extra>
        <a-space>
          <a-select v-model:value="moduleFilter" :placeholder="t('audit.filterModule')" allow-clear style="width: 140px" @change="fetchLogs">
            <a-select-option v-for="m in modules" :key="m" :value="m">{{ m }}</a-select-option>
          </a-select>
          <a-select v-model:value="actionFilter" :placeholder="t('audit.filterAction')" allow-clear style="width: 120px" @change="fetchLogs">
            <a-select-option v-for="a in actions" :key="a" :value="a">{{ a }}</a-select-option>
          </a-select>
          <a-date-picker v-model:value="startDate" :placeholder="t('audit.startDate')" style="width: 150px" value-format="YYYY-MM-DD" @change="fetchLogs" />
          <a-date-picker v-model:value="endDate" :placeholder="t('audit.endDate')" style="width: 150px" value-format="YYYY-MM-DD" @change="fetchLogs" />
          <a-button @click="fetchLogs">
            <template #icon><ReloadOutlined /></template>
            {{ t('common.refresh') }}
          </a-button>
        </a-space>
      </template>
    </a-page-header>

    <a-table
      :columns="columns"
      :data-source="logs"
      :loading="loading"
      :pagination="pagination"
      row-key="id"
      size="small"
      @change="handleTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'module'">
          <a-tag color="blue">{{ record.module }}</a-tag>
        </template>
        <template v-if="column.key === 'action'">
          <a-tag :color="actionColor(record.action)">{{ record.action }}</a-tag>
        </template>
        <template v-if="column.key === 'detail'">
          <a-tooltip :title="record.detail">
            {{ record.detail ? (record.detail.length > 60 ? record.detail.substring(0, 60) + '...' : record.detail) : '-' }}
          </a-tooltip>
        </template>
        <template v-if="column.key === 'created_at'">
          {{ formatTime(record.created_at) }}
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { systemApi } from '@/api/system'
import { formatTime } from '@/utils/format'

const { t } = useI18n()

const loading = ref(false)
const logs = ref<Record<string, unknown>[]>([])
const moduleFilter = ref<string | undefined>(undefined)
const actionFilter = ref<string | undefined>(undefined)
const startDate = ref<string | undefined>(undefined)
const endDate = ref<string | undefined>(undefined)

const modules = ['auth', 'rss', 'site', 'seeding', 'delete_rule', 'client', 'system', 'settings', 'cookiecloud']
const actions = ['create', 'update', 'delete', 'trigger', 'sync', 'login', 'clear', 'batch_update', 'batch_sync', 'update_credentials']

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total: number) => t('common.totalCount', { count: total }),
})

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('common.time'), key: 'created_at', width: 170 },
  { title: t('audit.actor'), dataIndex: 'actor', key: 'actor', width: 80 },
  { title: t('audit.module'), key: 'module', width: 100 },
  { title: t('audit.action'), key: 'action', width: 100 },
  { title: t('audit.targetType'), dataIndex: 'target_type', key: 'target_type', width: 90 },
  { title: t('audit.targetId'), dataIndex: 'target_id', key: 'target_id', width: 90 },
  { title: t('audit.detail'), key: 'detail', ellipsis: true },
]

function actionColor(action: string) {
  if (action === 'delete' || action === 'clear') return 'red'
  if (action === 'create') return 'green'
  if (action === 'update' || action === 'batch_update' || action === 'update_credentials') return 'orange'
  return 'blue'
}

function handleTableChange(pag: { current: number; pageSize: number }) {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchLogs()
}

async function fetchLogs() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: pagination.current,
      size: pagination.pageSize,
    }
    if (moduleFilter.value) params.module = moduleFilter.value
    if (actionFilter.value) params.action = actionFilter.value
    if (startDate.value) params.start_date = startDate.value
    if (endDate.value) params.end_date = endDate.value
    const resp = await systemApi.listAuditLogs(params)
    const body = resp.data.data
    logs.value = (body?.items || []) as unknown as Record<string, unknown>[]
    pagination.total = body?.total || 0
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

onMounted(() => fetchLogs())
</script>
