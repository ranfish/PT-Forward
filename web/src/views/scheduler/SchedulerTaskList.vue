<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button @click="fetchTasks">{{ t('common.refresh') }}</a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="tasks"
      :loading="loading"
      :pagination="false"
      row-key="name"
      size="small"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'">
          <a-button type="link" size="small" @click="viewDetail(record)">{{ taskLabel(record.name) }}</a-button>
        </template>
        <template v-if="column.key === 'status'">
          <a-tag :color="record.paused ? 'orange' : 'green'">{{ record.paused ? t('common.paused') : t('common.running') }}</a-tag>
        </template>
        <template v-if="column.key === 'last_run_at'">
          {{ formatTime(record.last_run_at) }}
        </template>
        <template v-if="column.key === 'stats'">
          <span style="color: #52c41a">{{ record.success_count }}</span> / <span style="color: #ff4d4f">{{ record.error_count }}</span>
        </template>
        <template v-if="column.key === 'schedule'">
          <a-typography-text
            copyable
            :content="record.schedule"
            style="cursor: pointer"
            @click="openReschedule(record)"
          >
            {{ record.schedule }}
          </a-typography-text>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button size="small" :disabled="record.paused" @click="triggerTask(record)">{{ t('common.trigger') }}</a-button>
            <a-button v-if="record.paused" size="small" type="primary" @click="resumeTask(record)">{{ t('common.resume') }}</a-button>
            <a-button v-else size="small" @click="pauseTask(record)">{{ t('common.pause') }}</a-button>
            <a-button size="small" @click="openReschedule(record)">{{ t('scheduler.reschedule') }}</a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="rescheduleVisible"
      :title="t('scheduler.editSchedule')"
      :confirm-loading="rescheduleLoading"
      @ok="doReschedule"
    >
      <a-form :label-col="{ span: 6 }">
        <a-form-item :label="t('scheduler.taskName')">
          <a-input :value="rescheduleTarget ? taskLabel(rescheduleTarget.name) : ''" disabled />
        </a-form-item>
        <a-form-item :label="t('scheduler.currentPeriod')">
          <a-input :value="rescheduleTarget?.schedule" disabled />
        </a-form-item>
        <a-form-item :label="t('scheduler.newPeriod')">
          <a-input v-model:value="newSchedule" :placeholder="t('scheduler.cronPlaceholder')" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="detailVisible" :title="t('scheduler.taskDetail')" :footer="null" width="500px">
      <a-spin :spinning="detailLoading">
        <a-descriptions v-if="detailData" bordered :column="1" size="small">
          <a-descriptions-item :label="t('scheduler.taskName')">{{ taskLabel(detailData.name) }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.type')">{{ typeLabel(detailData.type) }}</a-descriptions-item>
          <a-descriptions-item :label="t('scheduler.schedulePeriod')">{{ detailData.schedule }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.status')">
            <a-tag :color="detailData.paused ? 'orange' : 'green'">{{ detailData.paused ? t('common.paused') : t('common.running') }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item :label="t('scheduler.lastRun')">{{ formatTime(detailData.last_run_at) }}</a-descriptions-item>
          <a-descriptions-item :label="t('scheduler.successFail')">
            <span style="color: #52c41a">{{ detailData.success_count }}</span> / <span style="color: #ff4d4f">{{ detailData.error_count }}</span>
          </a-descriptions-item>
          <a-descriptions-item v-if="detailData.last_error" :label="t('scheduler.lastError')">{{ detailData.last_error }}</a-descriptions-item>
        </a-descriptions>
      </a-spin>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { schedulerApi } from '@/api/scheduler'
import { formatTime } from '@/utils/format'
import type { SchedulerTask } from '@/api/types'

const taskNameMap: Record<string, string> = {
  client_ping: '客户端心跳检测',
  publish_pending: '待发布处理',
  publish_groups: '发布组处理',
  publish_lifecycle: '发布生命周期管理',
  publish_seeding_confirm: '发布做种确认',
  reseed_tasks: '辅种任务处理',
  seeding_cleanup: '刷流清理',
  seeding_auto_delete: '刷流自动删除',
  seeding_traffic_stats: '刷流流量统计',
  rss_disk_budget_expire: 'RSS 磁盘预算过期',
  rss_recheck_waiting: 'RSS 重新检查等待',
  rss_cleanup_old_seen: 'RSS 清理旧记录',
  notification_cleanup_history: '通知历史清理',
  site_stats_sync: '站点统计同步',
  seeding_free_wait_check: '刷流免费等待检查',
}

function taskLabel(name: string): string {
  return taskNameMap[name] || name
}

const taskTypeMap: Record<string, string> = {
  maintenance: '维护',
  publish: '发布',
  reseed: '辅种',
  seeding: '刷流',
  rss: 'RSS',
}

function typeLabel(tp: string): string {
  return taskTypeMap[tp] || tp
}

const { t } = useI18n()

const loading = ref(false)
const tasks = ref<SchedulerTask[]>([])

const rescheduleVisible = ref(false)
const rescheduleLoading = ref(false)
const rescheduleTarget = ref<SchedulerTask | null>(null)
const newSchedule = ref('')
const detailVisible = ref(false)
const detailLoading = ref(false)
const detailData = ref<SchedulerTask | null>(null)

const columns = [
  { title: t('scheduler.taskName'), dataIndex: 'name', key: 'name', width: 220 },
  { title: t('common.type'), dataIndex: 'type', key: 'type', width: 100, customRender: ({ text }: { text: string }) => typeLabel(text) },
  { title: t('scheduler.schedulePeriod'), key: 'schedule', width: 160 },
  { title: t('common.status'), key: 'status', width: 90 },
  { title: t('scheduler.lastRun'), key: 'last_run_at', width: 180 },
  { title: t('scheduler.successFail'), key: 'stats', width: 100 },
  { title: t('scheduler.lastError'), dataIndex: 'last_error', key: 'last_error', ellipsis: true },
  { title: t('common.actions'), key: 'actions', width: 240, fixed: 'right' },
]

async function fetchTasks() {
  loading.value = true
  try {
    const resp = await schedulerApi.list()
    tasks.value = resp.data?.data?.items || []
  } catch {
    message.error(t('scheduler.fetchTaskListFailed'))
  } finally {
    loading.value = false
  }
}

async function triggerTask(task: SchedulerTask) {
  try {
    await schedulerApi.trigger(task.name)
    message.success(t('scheduler.taskTriggered', { name: task.name }))
    await fetchTasks()
  } catch {
    message.error(t('scheduler.triggerFailed'))
  }
}

async function pauseTask(task: SchedulerTask) {
  try {
    await schedulerApi.pause(task.name)
    message.success(t('scheduler.taskPaused', { name: task.name }))
    await fetchTasks()
  } catch {
    message.error(t('scheduler.pauseFailed'))
  }
}

async function resumeTask(task: SchedulerTask) {
  try {
    await schedulerApi.resume(task.name)
    message.success(t('scheduler.taskResumed', { name: task.name }))
    await fetchTasks()
  } catch {
    message.error(t('scheduler.resumeFailed'))
  }
}

function openReschedule(task: SchedulerTask) {
  rescheduleTarget.value = task
  newSchedule.value = task.schedule
  rescheduleVisible.value = true
}

async function viewDetail(task: SchedulerTask) {
  detailVisible.value = true
  detailLoading.value = true
  detailData.value = null
  try {
    const resp = await schedulerApi.get(task.name)
    detailData.value = resp.data?.data || task
  } catch {
    detailData.value = task
  } finally {
    detailLoading.value = false
  }
}

async function doReschedule() {
  if (!rescheduleTarget.value || !newSchedule.value) return
  rescheduleLoading.value = true
  try {
    await schedulerApi.reschedule(rescheduleTarget.value.name, newSchedule.value)
    message.success(t('scheduler.scheduleUpdated'))
    rescheduleVisible.value = false
    await fetchTasks()
  } catch {
    message.error(t('scheduler.rescheduleFailed'))
  } finally {
    rescheduleLoading.value = false
  }
}

onMounted(fetchTasks)
</script>
