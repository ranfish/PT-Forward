<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end; gap: 8px">
      <a-button type="primary" @click="wizardOpen = true">
        <template #icon><PlusOutlined /></template>
        {{ t('publish.manualForward') }}
      </a-button>
    </div>
    <a-tabs v-model:active-key="activeTab">
      <a-tab-pane key="candidates" :tab="t('publish.candidates')">
        <div class="tab-toolbar">
          <a-input-search
            v-model:value="candidateSearch"
            :placeholder="t('common.search')"
            style="width: 260px"
            allow-clear
            @search="onCandidateFilterChange"
          />
          <a-select
            v-model:value="candidateStatus"
            style="width: 130px"
            placeholder="状态筛选"
            allow-clear
            @change="onCandidateFilterChange"
          >
            <a-select-option value="pending">待处理</a-select-option>
            <a-select-option value="downloading">下载中</a-select-option>
            <a-select-option value="publishing">发布中</a-select-option>
            <a-select-option value="done">已完成</a-select-option>
            <a-select-option value="failed">失败</a-select-option>
            <a-select-option value="skipped">已跳过</a-select-option>
          </a-select>
          <a-badge :count="candidateTotal" :number-style="{ backgroundColor: '#1890ff' }" :overflow-count="9999" />
          <div class="tab-toolbar-right">
            <a-button
              v-if="selectedCandidateIds.length > 0"
              type="primary"
              size="small"
              :loading="batchRetrying"
              @click="batchRetry"
            >
              <template #icon><ReloadOutlined /></template>
              批量重试 ({{ selectedCandidateIds.length }})
            </a-button>
          </div>
        </div>

        <a-table
          :columns="candidateColumns"
          :data-source="candidates"
          :loading="candidatesLoading"
          :pagination="{ current: candidatePage, pageSize: candidatePageSize, total: candidateTotal, showSizeChanger: true, showTotal: (total: number) => t('common.totalCount', { count: total }) }"
          row-key="id"
          size="small"
          :row-selection="{ selectedRowKeys: selectedCandidateIds, onChange: (keys: number[]) => selectedCandidateIds = keys }"
          :expand="{ expandedRowRender: undefined }"
          @change="(pag: { current?: number; pageSize?: number }) => { if (pag.current) candidatePage = pag.current; if (pag.pageSize) candidatePageSize = pag.pageSize; fetchCandidates() }"
        >
          <template #expandedRowRender="{ record }">
            <div class="candidate-detail">
              <a-descriptions size="small" :column="3" bordered>
                <a-descriptions-item label="InfoHash">
                  <span style="font-family: monospace; font-size: 12px">{{ record.info_hash || '-' }}</span>
                </a-descriptions-item>
                <a-descriptions-item label="源种子ID">{{ record.source_torrent_id || '-' }}</a-descriptions-item>
                <a-descriptions-item label="下载器">{{ record.client_id || '-' }}</a-descriptions-item>
                <a-descriptions-item label="保存路径" :span="3">
                  <span style="font-family: monospace; font-size: 12px">{{ record.local_save_path || '-' }}</span>
                </a-descriptions-item>
                <a-descriptions-item v-if="record.skip_reason" label="跳过原因" :span="3">
                  <span style="color: #faad14">{{ record.skip_reason }}</span>
                </a-descriptions-item>
                <a-descriptions-item v-if="record.publish_result" label="发布结果" :span="3">
                  <span style="color: #ff4d4f">{{ record.publish_result }}</span>
                </a-descriptions-item>
              </a-descriptions>
            </div>
          </template>
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'publish_status'">
              <a-tag :color="publishStatusColor(record.publish_status)" style="margin: 0">
                {{ translatePublishStatus(record.publish_status) }}
              </a-tag>
            </template>
            <template v-if="column.key === 'target_sites'">
              <div class="target-site-tags">
                <a-tag
                  v-for="site in parseTargetSites(record.target_sites)"
                  :key="site"
                  size="small"
                  :color="sitePublishColor(record.publish_status)"
                >
                  {{ site }}
                </a-tag>
              </div>
            </template>
            <template v-if="column.key === 'actions'">
              <a-space>
                <a-button
                  type="link"
                  size="small"
                  :disabled="record.publish_status === 'done' || record.publish_status === 'publishing'"
                  @click="manualPublish(record.id)"
                >
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
        <div class="tab-toolbar">
          <a-select v-model:value="resultStatusFilter" style="width: 120px" placeholder="状态" allow-clear @change="onResultFilterChange">
            <a-select-option value="published">已发布</a-select-option>
            <a-select-option value="publishing">发布中</a-select-option>
            <a-select-option value="failed">失败</a-select-option>
            <a-select-option value="skipped">已跳过</a-select-option>
            <a-select-option value="pending">待发布</a-select-option>
          </a-select>
          <a-input-search
            v-model:value="resultTargetFilter"
            placeholder="目标站名称"
            style="width: 160px; margin-left: 8px"
            allow-clear
            @search="onResultFilterChange"
          />
          <a-tag color="blue">{{ resultTotal }}</a-tag>
        </div>
        <a-table
          :columns="resultColumns"
          :data-source="results"
          :loading="resultsLoading"
          :pagination="{
            current: resultPage,
            pageSize: resultPageSize,
            total: resultTotal,
            showSizeChanger: true,
            pageSizeOptions: ['20', '50', '100'],
            showTotal: (total: number) => t('common.totalCount', { count: total }),
            size: 'small',
          }"
          row-key="id"
          size="small"
          :sticky="{ offsetHeader: 48 }"
          @change="(pag: { current?: number; pageSize?: number }) => { if (pag.current) resultPage = pag.current; if (pag.pageSize) resultPageSize = pag.pageSize; fetchResultsFiltered() }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="resultStatusColor(record.status)">
                {{ translatePublishStatus(record.status) }}
              </a-tag>
            </template>
            <template v-if="column.key === 'publish_url'">
              <a v-if="record.publish_url" :href="record.publish_url" target="_blank" style="font-size: 12px">查看种子</a>
              <span v-else>-</span>
            </template>
            <template v-if="column.key === 'created_at'">
              {{ formatTime(record.created_at) }}
            </template>
            <template v-if="column.key === 'candidate_id'">
              <a-button type="link" size="small" @click="jumpToCandidate(record.candidate_id)">{{ record.candidate_id }}</a-button>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>

    <PublishWizardModal v-model:open="wizardOpen" @success="onWizardSuccess" />

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
import { ref, reactive, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { publishApi } from '@/api/publish'
import { sitesApi } from '@/api/sites'
import { useEnumLabels } from '@/utils/enumLabels'
import PublishWizardModal from './PublishWizardModal.vue'
import type { PublishCandidate, PublishGroup, PublishTask, PublishResultRecord } from '@/api/types'
import { formatTime, formatBytes } from '@/utils/format'

const { t } = useI18n()
const { translatePublishStatus, translatePublishType } = useEnumLabels()
const activeTab = ref('candidates')
const wizardOpen = ref(false)
const candidateSearch = ref('')
const candidateStatus = ref<string | undefined>(undefined)
const candidatesLoading = ref(false)
const candidates = ref<PublishCandidate[]>([])
const candidatePage = ref(1)
const candidateTotal = ref(0)
const candidatePageSize = ref(20)
const selectedCandidateIds = ref<number[]>([])
const batchRetrying = ref(false)

const groupsLoading = ref(false)
const groups = ref<PublishGroup[]>([])

const tasksLoading = ref(false)
const tasks = ref<PublishTask[]>([])
const taskPage = ref(1)
const taskTotal = ref(0)

const resultsLoading = ref(false)
const results = ref<PublishResultRecord[]>([])
const resultStatusFilter = ref<string | undefined>(undefined)
const resultTargetFilter = ref('')
const resultPage = ref(1)
const resultPageSize = ref(20)
const resultTotal = ref(0)

const candidateColumns = [
  { title: t('publish.torrentName'), dataIndex: 'torrent_name', key: 'torrent_name', ellipsis: true },
  { title: t('publish.sourceSite'), dataIndex: 'source_site', key: 'source_site', width: 90 },
  { title: '目标站', key: 'target_sites', width: 200 },
  { title: t('common.size'), dataIndex: 'size', key: 'size', width: 80, customRender: ({ text }: { text: number }) => formatBytes(text) },
  { title: t('publish.publishStatus'), key: 'publish_status', width: 80 },
  { title: '重试', dataIndex: 'retry_count', key: 'retry_count', width: 50 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 150, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 110 },
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
  { title: t('common.type'), dataIndex: 'type', key: 'type', width: 100, customRender: ({ text }: { text: string }) => translatePublishType(text) },
  { title: t('publish.sourceSiteId'), dataIndex: 'source_site_id', key: 'source_site_id', width: 100 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('publish.manualCheck'), dataIndex: 'manual_check', key: 'manual_check', width: 100, customRender: ({ text }: { text: boolean }) => text ? t('common.yes') : t('common.no') },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 150 },
]

const resultColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: '候选ID', key: 'candidate_id', width: 80 },
  { title: t('publish.sourceSite'), dataIndex: 'source_site', key: 'source_site', width: 90 },
  { title: t('publish.targetSite'), dataIndex: 'target_site', key: 'target_site', width: 90 },
  { title: t('common.status'), key: 'status', width: 80 },
  { title: t('publish.publishUrl'), key: 'publish_url', width: 80 },
  { title: t('publish.errorMessage'), dataIndex: 'error_message', key: 'error_message', ellipsis: true },
  { title: t('common.createdAt'), key: 'created_at', width: 140 },
]

function publishStatusColor(status: string) {
  const map: Record<string, string> = { pending: 'blue', downloading: 'cyan', completed: 'geekblue', publishing: 'orange', done: 'green', failed: 'red', skipped: 'default', orphan: 'volcano' }
  return map[status] || 'default'
}

function taskStatusColor(status: string) {
  const map: Record<string, string> = { pending: 'blue', running: 'cyan', completed: 'green', failed: 'red', cancelled: 'default' }
  return map[status] || 'default'
}

function resultStatusColor(status: string) {
  const map: Record<string, string> = { published: 'green', publishing: 'processing', failed: 'red', skipped: 'orange', pending: 'blue' }
  return map[status] || 'default'
}

function onResultFilterChange() {
  resultPage.value = 1
  fetchResultsFiltered()
}

async function fetchResultsFiltered() {
  resultsLoading.value = true
  try {
    const resp = await publishApi.listResults({
      page: resultPage.value,
      size: resultPageSize.value,
      status: resultStatusFilter.value,
      target_site: resultTargetFilter.value || undefined,
    })
    const body = resp.data?.data
    if (body?.items) {
      results.value = body.items
      resultTotal.value = body.total || 0
    } else {
      results.value = (body as unknown as PublishResultRecord[]) || []
      resultTotal.value = results.value.length
    }
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    resultsLoading.value = false
  }
}

function jumpToCandidate(id: number) {
  activeTab.value = 'candidates'
  candidateSearch.value = String(id)
  fetchCandidates()
}

function parseTargetSites(raw: string): string[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return raw.split(',').map(s => s.trim()).filter(Boolean)
  }
}

function sitePublishColor(status: string): string {
  const map: Record<string, string> = {
    done: 'green', completed: 'green', published: 'green',
    failed: 'red',
    skipped: 'orange',
    publishing: 'blue',
    pending: 'default',
    downloading: 'cyan',
  }
  return map[status] || 'default'
}

function onCandidateFilterChange() {
  candidatePage.value = 1
  fetchCandidates()
}

const hasActiveCandidates = computed(() =>
  candidates.value.some(c =>
    c.publish_status === 'pending' ||
    c.publish_status === 'downloading' ||
    c.publish_status === 'publishing'
  )
)

let candidatePollTimer: ReturnType<typeof setInterval> | null = null

function startCandidatePoll() {
  if (candidatePollTimer) return
  candidatePollTimer = setInterval(async () => {
    if (activeTab.value !== 'candidates') return
    if (!hasActiveCandidates.value) return
    await fetchCandidatesSilent()
  }, 5000)
}

function stopCandidatePoll() {
  if (candidatePollTimer) {
    clearInterval(candidatePollTimer)
    candidatePollTimer = null
  }
}

async function fetchCandidatesSilent() {
  try {
    const resp = await publishApi.listCandidates({
      page: candidatePage.value,
      size: candidatePageSize.value,
      search: candidateSearch.value || undefined,
      status: candidateStatus.value,
    })
    const body = resp.data?.data
    candidates.value = body?.items || []
    candidateTotal.value = body?.total || 0
  } catch { /* silent */ }
}

async function fetchCandidates() {
  candidatesLoading.value = true
  try {
    const resp = await publishApi.listCandidates({
      page: candidatePage.value,
      size: candidatePageSize.value,
      search: candidateSearch.value || undefined,
      status: candidateStatus.value,
    })
    const body = resp.data?.data
    candidates.value = body?.items || []
    candidateTotal.value = body?.total || 0
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    candidatesLoading.value = false
  }
}

async function batchRetry() {
  if (selectedCandidateIds.value.length === 0) return
  batchRetrying.value = true
  let ok = 0
  let fail = 0
  for (const id of selectedCandidateIds.value) {
    try {
      await publishApi.manualPublish(id)
      ok++
    } catch {
      fail++
    }
  }
  batchRetrying.value = false
  selectedCandidateIds.value = []
  if (fail === 0) {
    message.success(`已重试 ${ok} 个候选`)
  } else {
    message.warning(`成功 ${ok}，失败 ${fail}`)
  }
  fetchCandidates()
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
      sourceSiteId: taskForm.sourceSiteId,
      targetSites: taskForm.targetSites,
      type: taskForm.type || undefined,
      manualCheck: taskForm.manualCheck,
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

function onWizardSuccess() {
  activeTab.value = 'candidates'
  fetchCandidates()
}

onMounted(() => {
  fetchCandidates()
  fetchGroups()
  fetchTasks()
  fetchResultsFiltered()
  fetchTaskSites()
})

onUnmounted(() => {
  stopCandidatePoll()
})
</script>

<style scoped>
.tab-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.tab-toolbar-right {
  margin-left: auto;
}
.target-site-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}
.target-site-tags :deep(.ant-tag) {
  margin: 0;
  font-size: 12px;
}
.candidate-detail {
  padding: 8px 0;
}
</style>
