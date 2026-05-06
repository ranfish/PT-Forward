<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button @click="fetchTasks">刷新</a-button>
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
          <a-tag :color="record.paused ? 'orange' : 'green'">{{ record.paused ? '已暂停' : '运行中' }}</a-tag>
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
            <a-button size="small" @click="triggerTask(record)" :disabled="record.paused">触发</a-button>
            <a-button v-if="record.paused" size="small" type="primary" @click="resumeTask(record)">恢复</a-button>
            <a-button v-else size="small" @click="pauseTask(record)">暂停</a-button>
            <a-button size="small" @click="openReschedule(record)">调期</a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="rescheduleVisible"
      title="修改调度周期"
      @ok="doReschedule"
      :confirm-loading="rescheduleLoading"
    >
      <a-form :label-col="{ span: 6 }">
        <a-form-item label="任务名称">
          <a-input :value="rescheduleTarget?.name" disabled />
        </a-form-item>
        <a-form-item label="当前周期">
          <a-input :value="rescheduleTarget?.schedule" disabled />
        </a-form-item>
        <a-form-item label="新周期">
          <a-input v-model:value="newSchedule" placeholder="例: */5 * * * *（支持 5/6 位 cron）" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { schedulerApi, type SchedulerTask } from '@/api/scheduler'

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
    message.error('获取任务列表失败')
  } finally {
    loading.value = false
  }
}

async function triggerTask(task: SchedulerTask) {
  try {
    await schedulerApi.trigger(task.name)
    message.success(`已触发 ${task.name}`)
    await fetchTasks()
  } catch {
    message.error('触发失败')
  }
}

async function pauseTask(task: SchedulerTask) {
  try {
    await schedulerApi.pause(task.name)
    message.success(`已暂停 ${task.name}`)
    await fetchTasks()
  } catch {
    message.error('暂停失败')
  }
}

async function resumeTask(task: SchedulerTask) {
  try {
    await schedulerApi.resume(task.name)
    message.success(`已恢复 ${task.name}`)
    await fetchTasks()
  } catch {
    message.error('恢复失败')
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
    message.success('调度周期已更新')
    rescheduleVisible.value = false
    await fetchTasks()
  } catch {
    message.error('修改失败，请检查 cron 表达式')
  } finally {
    rescheduleLoading.value = false
  }
}

onMounted(fetchTasks)
</script>
