<template>
  <div>
    <a-tabs v-model:active-key="activeTab">
      <a-tab-pane key="candidates" :tab="t('publish.candidates')">
        <div style="margin-bottom: 16px; display: flex; justify-content: space-between">
          <a-space>
            <a-input-search
              v-model:value="candidateSearch"
              :placeholder="t('common.search')"
              style="width: 300px"
              @search="fetchCandidates"
            />
          </a-space>
        </div>

        <a-table
          :columns="candidateColumns"
          :data-source="candidates"
          :loading="candidatesLoading"
          :pagination="{ current: candidatePage, pageSize: 20, total: candidateTotal, showSizeChanger: true, showTotal: (total: number) => t('common.totalCount', { count: total }) }"
          row-key="id"
          size="small"
          @change="(pag: { current: number }) => { candidatePage = pag.current; fetchCandidates() }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'publish_status'">
              <a-tag :color="record.publish_status === 'pending' ? 'blue' : record.publish_status === 'completed' ? 'green' : record.publish_status === 'failed' ? 'red' : 'default'">
                {{ translatePublishStatus(record.publish_status) }}
              </a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-space>
                <a-button type="link" size="small" :disabled="record.publish_status === 'completed'" @click="manualPublish(record.id)">
                  {{ t('publish.publishAction') }}
                </a-button>
                <a-popconfirm :title="t('publish.deleteConfirm')" @confirm="deleteCandidate(record.id)">
                  <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="groups" :tab="t('publish.groups')">
        <a-table
          :columns="groupColumns"
          :data-source="groups"
          :loading="groupsLoading"
          :pagination="false"
          row-key="id"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="record.status === 'active' ? 'green' : record.status === 'completed' ? 'default' : 'orange'">
                {{ translatePublishStatus(record.status) }}
              </a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-space>
                <a-button type="link" size="small" @click="$router.push(`/publish/groups/${record.id}`)">{{ t('common.detail') }}</a-button>
                <a-popconfirm :title="t('publish.deleteConfirm')" @confirm="deleteGroup(record.id)">
                  <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="tasks" :tab="t('publish.tasks')">
        <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
          <a-button type="primary" @click="showCreateTaskModal = true">{{ t('common.create') }}</a-button>
        </div>
        <a-table
          :columns="taskColumns"
          :data-source="tasks"
          :loading="tasksLoading"
          :pagination="{ current: taskPage, pageSize: 20, total: taskTotal, showSizeChanger: true, showTotal: (total: number) => t('common.totalCount', { count: total }) }"
          row-key="id"
          size="small"
          @change="(pag: { current: number }) => { taskPage = pag.current; fetchTasks() }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="taskStatusColor(record.status)">{{ translatePublishStatus(record.status) }}</a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-space>
                <a-button type="link" size="small" @click="viewTaskResults()">{{ t('publish.viewResults') }}</a-button>
                <a-button v-if="record.status === 'pending' || record.status === 'running'" type="link" size="small" @click="cancelTask(record.id)">{{ t('common.cancel') }}</a-button>
                <a-popconfirm :title="t('publish.deleteConfirm')" @confirm="deleteTask(record.id)">
                  <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="results" :tab="t('publish.results')">
        <a-table
          :columns="resultColumns"
          :data-source="results"
          :loading="resultsLoading"
          :pagination="{ pageSize: 20, showSizeChanger: true, showTotal: (total: number) => t('common.totalCount', { count: total }) }"
          row-key="id"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="record.status === 'published' ? 'green' : record.status === 'skipped' ? 'orange' : record.status === 'failed' ? 'red' : 'blue'">
                {{ translatePublishStatus(record.status) }}
              </a-tag>
            </template>
            <template v-if="column.key === 'publish_url'">
              <a v-if="record.publish_url" :href="record.publish_url" target="_blank">{{ record.publish_url }}</a>
              <span v-else>-</span>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>

    <a-modal v-model:open="showCreateTaskModal" :title="t('publish.tasks')" :confirm-loading="createTaskSubmitting" width="520px" @ok="createTask">
      <a-form layout="vertical">
        <a-form-item :label="t('publish.sourceSiteId')">
          <a-select v-model:value="taskForm.sourceSiteId" show-search :placeholder="t('publish.selectSourceSite')" option-filter-prop="label" :filter-option="filterSiteOption">
            <a-select-option v-for="s in taskSites" :key="s.id" :value="s.id" :label="s.name">{{ s.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('publish.targetSites')">
          <a-select v-model:value="taskForm.targetSites" mode="multiple" show-search :placeholder="t('publish.selectTargetSites')" option-filter-prop="label" :filter-option="filterSiteOption">
            <a-select-option v-for="s in taskSites" :key="s.id" :value="s.name" :label="s.name">{{ s.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('common.type')">
          <a-select v-model:value="taskForm.type" allow-clear :placeholder="t('publish.selectType')">
            <a-select-option value="single">{{ t('publish.typeSingle') }}</a-select-option>
            <a-select-option value="batch">{{ t('publish.typeBatch') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('publish.manualCheck')">
          <a-switch v-model:checked="taskForm.manualCheck" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { publishApi } from '@/api/publish'
import { sitesApi } from '@/api/sites'
import { useEnumLabels } from '@/utils/enumLabels'
import type { PublishCandidate, PublishGroup, PublishTask, PublishResultRecord } from '@/api/types'
import { formatTime } from '@/utils/format'

const { t } = useI18n()
const { translatePublishStatus } = useEnumLabels()
const activeTab = ref('candidates')
const candidateSearch = ref('')
const candidatesLoading = ref(false)
const candidates = ref<PublishCandidate[]>([])
const candidatePage = ref(1)
const candidateTotal = ref(0)

const groupsLoading = ref(false)
const groups = ref<PublishGroup[]>([])

const tasksLoading = ref(false)
const tasks = ref<PublishTask[]>([])
const taskPage = ref(1)
const taskTotal = ref(0)

const resultsLoading = ref(false)
const results = ref<PublishResultRecord[]>([])

const candidateColumns = [
  { title: t('publish.torrentName'), dataIndex: 'torrent_name', key: 'torrent_name', ellipsis: true },
  { title: t('publish.sourceSite'), dataIndex: 'source_site', key: 'source_site', width: 120 },
  { title: t('common.size'), dataIndex: 'size', key: 'size', width: 100 },
  { title: t('publish.publishStatus'), key: 'publish_status', width: 100 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 120 },
]

const groupColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('publish.sourceSite'), dataIndex: 'source_site', key: 'source_site', width: 120 },
  { title: t('publish.sourceHash'), dataIndex: 'source_hash', key: 'source_hash', ellipsis: true },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 120 },
]

const taskColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('common.type'), dataIndex: 'type', key: 'type', width: 100 },
  { title: t('publish.sourceSiteId'), dataIndex: 'source_site_id', key: 'source_site_id', width: 100 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('publish.manualCheck'), dataIndex: 'manual_check', key: 'manual_check', width: 100, customRender: ({ text }: { text: boolean }) => text ? t('common.yes') : t('common.no') },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 150 },
]

const resultColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('publish.sourceSite'), dataIndex: 'source_site', key: 'source_site', width: 120 },
  { title: t('publish.targetSite'), dataIndex: 'target_site', key: 'target_site', width: 120 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('publish.publishUrl'), key: 'publish_url', ellipsis: true },
  { title: t('publish.errorMessage'), dataIndex: 'error_message', key: 'error_message', ellipsis: true },
  { title: t('publish.completedAt'), dataIndex: 'completed_at', key: 'completed_at', width: 180 },
]

function taskStatusColor(status: string) {
  const map: Record<string, string> = { pending: 'blue', running: 'cyan', completed: 'green', failed: 'red', cancelled: 'default' }
  return map[status] || 'default'
}

async function fetchCandidates() {
  candidatesLoading.value = true
  try {
    const resp = await publishApi.listCandidates({ page: candidatePage.value, size: 20, search: candidateSearch.value || undefined })
    const body = resp.data.data
    candidates.value = body?.items || body || []
    candidateTotal.value = body?.total || 0
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    candidatesLoading.value = false
  }
}

async function fetchGroups() {
  groupsLoading.value = true
  try {
    const resp = await publishApi.listGroups()
    groups.value = resp.data.data?.items ?? []
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    groupsLoading.value = false
  }
}

async function manualPublish(id: number) {
  try {
    await publishApi.manualPublish(id)
    message.success(t('publish.publishTriggered'))
    fetchCandidates()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function deleteCandidate(id: number) {
  try {
    await publishApi.deleteCandidate(id)
    message.success(t('common.deleted'))
    fetchCandidates()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function deleteGroup(id: number) {
  try {
    await publishApi.deleteGroup(id)
    message.success(t('common.deleted'))
    fetchGroups()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function fetchTasks() {
  tasksLoading.value = true
  try {
    const resp = await publishApi.listTasks({ page: taskPage.value, size: 20 })
    const body = resp.data.data
    tasks.value = body?.items || []
    taskTotal.value = body?.total || 0
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    tasksLoading.value = false
  }
}

async function deleteTask(id: number) {
  try {
    await publishApi.deleteTask(id)
    message.success(t('common.deleted'))
    fetchTasks()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function cancelTask(id: number) {
  try {
    await publishApi.cancelTask(id)
    message.success(t('common.operationSuccess'))
    fetchTasks()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function viewTaskResults() {
  resultsLoading.value = true
  activeTab.value = 'results'
  try {
    const resp = await publishApi.listResults({ limit: 100 })
    const body = resp.data.data
    const allResults = body?.items || []
    results.value = allResults
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    resultsLoading.value = false
  }
}

async function fetchResults() {
  resultsLoading.value = true
  try {
    const resp = await publishApi.listResults({ limit: 100 })
    const body = resp.data.data
    results.value = body?.items || []
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    resultsLoading.value = false
  }
}

const showCreateTaskModal = ref(false)
const createTaskSubmitting = ref(false)
const taskSites = ref<{ id: number; name: string }[]>([])
const taskForm = reactive({
  sourceSiteId: undefined as number | undefined,
  targetSites: [] as string[],
  type: undefined as string | undefined,
  manualCheck: false,
})

function filterSiteOption(input: string, option: { label?: string }) {
  return (option.label || '').toLowerCase().includes(input.toLowerCase())
}

async function fetchTaskSites() {
  try {
    const resp = await sitesApi.list(1, 200)
    const body = resp.data.data
    taskSites.value = (body?.items || body || []) as { id: number; name: string }[]
  } catch { /* ignore */ }
}

async function createTask() {
  if (!taskForm.sourceSiteId || taskForm.targetSites.length === 0) {
    message.warning(t('publish.sourceAndTargetRequired'))
    return
  }
  createTaskSubmitting.value = true
  try {
    await publishApi.createTask({
      source_site_id: taskForm.sourceSiteId,
      target_sites: taskForm.targetSites,
      type: taskForm.type || undefined,
      manual_check: taskForm.manualCheck,
    })
    message.success(t('common.success'))
    showCreateTaskModal.value = false
    taskForm.sourceSiteId = undefined
    taskForm.targetSites = []
    taskForm.type = undefined
    taskForm.manualCheck = false
    fetchTasks()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    createTaskSubmitting.value = false
  }
}

onMounted(() => {
  fetchCandidates()
  fetchGroups()
  fetchTasks()
  fetchResults()
  fetchTaskSites()
})
</script>
