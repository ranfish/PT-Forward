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
        <template v-if="column.key === 'status'">
          <a-tag :color="record.paused ? 'orange' : 'green'">{{ record.paused ? t('common.paused') : t('common.running') }}</a-tag>
        </template>
        <template v-if="column.key === 'last_run_at'">
          {{ record.last_run_at ? new Date(record.last_run_at).toLocaleString() : '-' }}
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
            <a-button size="small" @click="triggerTask(record)" :disabled="record.paused">{{ t('common.trigger') }}</a-button>
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
      @ok="doReschedule"
      :confirm-loading="rescheduleLoading"
    >
      <a-form :label-col="{ span: 6 }">
        <a-form-item :label="t('scheduler.taskName')">
          <a-input :value="rescheduleTarget?.name" disabled />
        </a-form-item>
        <a-form-item :label="t('scheduler.currentPeriod')">
          <a-input :value="rescheduleTarget?.schedule" disabled />
        </a-form-item>
        <a-form-item :label="t('scheduler.newPeriod')">
          <a-input v-model:value="newSchedule" placeholder="例: */5 * * * *（支持 5/6 位 cron）" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { schedulerApi, type SchedulerTask } from '@/api/scheduler'

const { t } = useI18n()

const loading = ref(false)
const tasks = ref<SchedulerTask[]>([])

const rescheduleVisible = ref(false)
const rescheduleLoading = ref(false)
const rescheduleTarget = ref<SchedulerTask | null>(null)
const newSchedule = ref('')

const columns = [
  { title: '任务名称', dataIndex: 'name', key: 'name', width: 220 },
  { title: '类型', dataIndex: 'type', key: 'type', width: 100 },
  { title: '调度周期', key: 'schedule', width: 160 },
  { title: '状态', key: 'status', width: 90 },
  { title: '上次运行', key: 'last_run_at', width: 180 },
  { title: '成功/失败', key: 'stats', width: 100 },
  { title: '最近错误', dataIndex: 'last_error', key: 'last_error', ellipsis: true },
  { title: '操作', key: 'actions', width: 240, fixed: 'right' },
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
