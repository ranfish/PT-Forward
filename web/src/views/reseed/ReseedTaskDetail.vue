<template>
  <div>
    <a-page-header :title="t('reseed.taskDetail', { id: taskId })" @back="$router.push('/reseed')">
      <template v-if="task" #tags>
        <a-tag :color="statusColor(task.status)">{{ translateReseedStatus(task.status) }}</a-tag>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <template v-if="task">
        <a-descriptions bordered :column="2" style="margin-bottom: 24px">
          <a-descriptions-item :label="t('common.name')">{{ task.name }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.status')">{{ translateReseedStatus(task.status) }}</a-descriptions-item>
          <a-descriptions-item :label="t('reseed.sourceSite')">{{ resolveSiteIDs(task.source_site_ids) }}</a-descriptions-item>
          <a-descriptions-item :label="t('reseed.targetSite')">{{ resolveSiteIDs(task.target_site_ids) }}</a-descriptions-item>
          <a-descriptions-item :label="t('reseed.client')">{{ resolveDownloaderIDs(task.client_ids) }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.createdAt')">{{ formatTime(task.created_at) }}</a-descriptions-item>
        </a-descriptions>

        <a-tabs v-model:active-key="activeTab">
          <a-tab-pane key="matches" :tab="t('reseed.matchResults')">
            <div style="margin-bottom: 16px; display: flex; gap: 12px; flex-wrap: wrap; align-items: center">
              <a-input v-model:value="filter.clientId" placeholder="客户端" style="width: 150px" allow-clear @press-enter="onFilterSearch" />
              <a-input v-model:value="filter.site" placeholder="站点" style="width: 150px" allow-clear @press-enter="onFilterSearch" />
              <a-input v-model:value="filter.torrentId" placeholder="种子ID" style="width: 130px" allow-clear @press-enter="onFilterSearch" />
              <a-select v-model:value="filter.status" placeholder="状态" style="width: 120px" allow-clear>
                <a-select-option value="matched">待注入</a-select-option>
                <a-select-option value="injected">已注入</a-select-option>
                <a-select-option value="failed">失败</a-select-option>
              </a-select>
              <a-button type="primary" @click="onFilterSearch">查询</a-button>
              <a-button @click="onFilterReset">重置</a-button>
              <a-popconfirm title="确认清除所有辅种记录？此操作不可恢复" @confirm="clearAllMatches">
                <a-button danger>清除辅种记录</a-button>
              </a-popconfirm>
            </div>
            <div v-if="selectedMatchKeys.length" style="margin-bottom: 12px; display: flex; gap: 12px; align-items: center; padding: 8px 12px; background: #e6f4ff; border-radius: 4px">
              <span>已选 {{ selectedMatchKeys.length }} 项</span>
              <a-button size="small" @click="batchRetryFailed">重试失败</a-button>
              <a-popconfirm :title="`确认删除选中的 ${selectedMatchKeys.length} 条记录？`" @confirm="batchDelete">
                <a-button size="small" danger>{{ t('common.delete') }}</a-button>
              </a-popconfirm>
            </div>
            <a-table
              :columns="matchColumns"
              :data-source="matches"
              :loading="matchesLoading"
              :pagination="{
                current: matchesPage,
                pageSize: matchesPageSize,
                total: matchesTotal,
                showSizeChanger: true,
                showTotal: (t: number) => `${t} 条`,
              }"
              :row-selection="{ selectedRowKeys: selectedMatchKeys, onChange: onSelectChange }"
              row-key="id"
              size="small"
              @change="handleMatchesChange"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'source_info_hash'">
                  <span style="cursor:pointer;font-family:monospace;font-size:12px" @click="copyHash(record.source_info_hash)">{{ record.source_info_hash }}</span>
                </template>
                <template v-if="column.key === 'target_info_hash'">
                  <span style="cursor:pointer;font-family:monospace;font-size:12px" @click="copyHash(record.target_info_hash)">{{ record.target_info_hash }}</span>
                </template>
                <template v-if="column.key === 'source_torrent_id'">
                  <a v-if="record.source_detail_url" :href="record.source_detail_url" target="_blank" rel="noopener" style="font-family:monospace;font-size:12px">{{ record.source_torrent_id }}</a>
                  <span v-else style="font-family:monospace;font-size:12px">{{ record.source_torrent_id }}</span>
                </template>
                <template v-if="column.key === 'directory'">
                  <span style="font-size:12px">{{ record.directory || '-' }}</span>
                </template>
                <template v-if="column.key === 'created_at'">
                  {{ formatTime(record.created_at) }}
                </template>
                <template v-if="column.key === 'status'">
                  <a-tag :color="record.status === 'injected' ? 'green' : record.status === 'failed' ? 'red' : 'blue'">
                    {{ translateReseedStatus(record.status) }}
                  </a-tag>
                </template>
                <template v-if="column.key === 'actions'">
                  <a-button
                    type="link"
                    size="small"
                    :disabled="record.status !== 'failed'"
                    @click="retryMatch(record.id)"
                  >
                    {{ t('reseed.retry') }}
                  </a-button>
                </template>
              </template>
            </a-table>
          </a-tab-pane>
          <a-tab-pane v-if="task?.engine_mode === 'seed_feature'" key="feature" tab="特征辅种日志">
            <div v-if="featureStats" style="margin-bottom: 16px; display: flex; gap: 24px; flex-wrap: wrap; padding: 12px; background: #fafafa; border-radius: 4px">
              <span>查询次数: <strong>{{ featureStats.TotalCalls }}</strong></span>
              <span>查询种子: <strong>{{ featureStats.TotalQueried }}</strong></span>
              <span>匹配种子: <strong>{{ featureStats.TotalMatched }}</strong></span>
            </div>
            <a-table
              :columns="featureColumns"
              :data-source="featureLogs"
              :loading="featureLoading"
              :pagination="{ current: featurePage, pageSize: featurePageSize, total: featureTotal, showSizeChanger: true, showTotal: (t: number) => `${t} 条` }"
              row-key="id"
              size="small"
              @change="handleFeatureChange"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'created_at'">{{ formatTime(record.created_at) }}</template>
              </template>
            </a-table>
          </a-tab-pane>
          <a-tab-pane key="negative" :tab="t('reseed.negativeCache')">
            <div style="margin-bottom: 16px; display: flex; gap: 12px; align-items: center">
              <a-input v-model:value="negDeleteInfoHash" placeholder="InfoHash" style="width: 320px" />
              <a-input v-model:value="negDeleteSite" :placeholder="t('reseed.siteOptional')" style="width: 200px" />
              <a-popconfirm :title="t('reseed.deleteNegativeCacheConfirm')" @confirm="deleteNegativeCache">
                <a-button type="primary" danger :disabled="!negDeleteInfoHash">{{ t('common.delete') }}</a-button>
              </a-popconfirm>
            </div>
            <a-empty :description="t('reseed.deleteNegativeCacheDesc')" />
          </a-tab-pane>
          <a-tab-pane v-if="task?.engine_mode !== 'seed_feature'" key="iyuu" tab="IYUU日志">
            <div v-if="iyuuStats" style="margin-bottom: 16px; display: flex; gap: 24px; flex-wrap: wrap; padding: 12px; background: #fafafa; border-radius: 4px">
              <span>调用次数: <strong>{{ iyuuStats.TotalCalls }}</strong></span>
              <span style="color: #3f8600">成功: <strong>{{ iyuuStats.SuccessCalls }}</strong></span>
              <span style="color: #cf1322">失败: <strong>{{ iyuuStats.ErrorCalls }}</strong></span>
              <span>请求hash: <strong>{{ iyuuStats.TotalRequests }}</strong></span>
              <span>匹配hash: <strong>{{ iyuuStats.TotalMatched }}</strong></span>
              <span>返回目标: <strong>{{ iyuuStats.TotalTargets }}</strong></span>
            </div>
            <a-table
              :columns="iyuuColumns"
              :data-source="iyuuLogs"
              :loading="iyuuLoading"
              :pagination="{
                current: iyuuPage,
                pageSize: iyuuPageSize,
                total: iyuuTotal,
                showSizeChanger: true,
                showTotal: (t: number) => `${t} 条`,
              }"
              row-key="id"
              size="small"
              @change="handleIYUUChange"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'status'">
                  <a-tag :color="record.status === 'success' ? 'green' : 'red'">{{ record.status === 'success' ? '成功' : '失败' }}</a-tag>
                </template>
                <template v-if="column.key === 'created_at'">
                  {{ formatTime(record.created_at) }}
                </template>
                <template v-if="column.key === 'message'">
                  <a-tooltip :title="record.message">
                    <span style="display: inline-block; max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ record.message || '-' }}</span>
                  </a-tooltip>
                </template>
              </template>
            </a-table>
          </a-tab-pane>
        </a-tabs>
      </template>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { reseedApi, type ReseedIYUULog, type IYUULogStats, type ReseedFeatureLog, type FeatureLogStats } from '@/api/reseed'
import { sitesApi } from '@/api/sites'
import { downloadersApi } from '@/api/downloaders'
import { formatTime, copyToClipboard } from '@/utils/format'
import { useEnumLabels } from '@/utils/enumLabels'

const route = useRoute()
const taskId = Number(route.params.id)
const { t } = useI18n()
const { translateReseedStatus, translateMatchMethod } = useEnumLabels()

const siteMap = ref<Record<string, string>>({})
const downloaderMap = ref<Record<string, string>>({})

async function fetchSiteMap() {
  try {
    const resp = await sitesApi.list(1, 200)
    const items = resp.data?.data?.items || resp.data?.data || []
    const m: Record<string, string> = {}
    for (const s of items) {
      m[String(s.id)] = s.name
    }
    siteMap.value = m
  } catch { /* ignore */ }
}

async function fetchDownloaderMap() {
  try {
    const resp = await downloadersApi.listLight(1, 200)
    const items = resp.data?.data?.items || resp.data?.data || []
    const m: Record<string, string> = {}
    for (const d of items) {
      m[String(d.id)] = d.name
    }
    downloaderMap.value = m
  } catch { /* ignore */ }
}

function resolveSiteIDs(ids: string): string {
  if (!ids) return '-'
  return ids.split(',').map(id => siteMap.value[id.trim()] || id).join(', ')
}

function resolveDownloaderIDs(ids: string): string {
  if (!ids) return '-'
  return ids.split(',').map(id => downloaderMap.value[id.trim()] || id).join(', ')
}

function copyHash(text: string) {
  copyToClipboard(text)
  message.success(t('common.copied'))
}

interface ReseedTaskInfo {
  name: string
  status: string
  source_site_ids: string
  target_site_ids: string
  client_ids: string
  created_at: string
  engine_mode: string
}

interface ReseedMatchItem {
  id: number
  source_info_hash: string
  source_torrent_id: string
  source_detail_url?: string
  target_site: string
  target_info_hash: string
  match_method: string
  confidence: number
  status: string
  fail_reason: string
  directory?: string
  created_at: string
  client_id: string
}

const loading = ref(false)
const matchesLoading = ref(false)
const task = ref<ReseedTaskInfo | null>(null)
const matches = ref<ReseedMatchItem[]>([])
const matchesTotal = ref(0)
const matchesPage = ref(1)
const matchesPageSize = ref(20)
const activeTab = ref('matches')

const filter = reactive({ clientId: '', site: '', torrentId: '', status: '' })
const matchesOrder = reactive({ field: '', order: '' })
const selectedMatchKeys = ref<number[]>([])

const negDeleteInfoHash = ref('')
const negDeleteSite = ref('')

const iyuuLoading = ref(false)
const iyuuLogs = ref<ReseedIYUULog[]>([])
const iyuuTotal = ref(0)
const iyuuPage = ref(1)
const iyuuPageSize = ref(20)
const iyuuStats = ref<IYUULogStats | null>(null)
const featureLoading = ref(false)
const featureLogs = ref<ReseedFeatureLog[]>([])
const featureTotal = ref(0)
const featurePage = ref(1)
const featurePageSize = ref(20)
const featureStats = ref<FeatureLogStats | null>(null)
const featureColumns = [
  { title: '时间', key: 'created_at', width: 180 },
  { title: '站点', dataIndex: 'site', key: 'site', width: 120 },
  { title: '查询数', dataIndex: 'queried', key: 'queried', width: 90 },
  { title: '匹配数', dataIndex: 'matched', key: 'matched', width: 90 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 80 },
]

const iyuuColumns = [
  { title: '时间', key: 'created_at', width: 180 },
  { title: '状态', key: 'status', width: 80 },
  { title: '请求hash', dataIndex: 'request_hashes', key: 'request_hashes', width: 90 },
  { title: '匹配hash', dataIndex: 'matched_hashes', key: 'matched_hashes', width: 90 },
  { title: '返回目标', dataIndex: 'response_targets', key: 'response_targets', width: 90 },
  { title: '耗时(ms)', dataIndex: 'duration_ms', key: 'duration_ms', width: 90 },
  { title: '消息', key: 'message', ellipsis: true },
]

const matchColumns = [
  { title: t('reseed.sourceInfoHash'), dataIndex: 'source_info_hash', key: 'source_info_hash', ellipsis: true, sorter: true },
  { title: '客户端', dataIndex: 'client_id', key: 'client_id', width: 100, sorter: true },
  { title: t('reseed.targetSite'), dataIndex: 'target_site', key: 'target_site', width: 120, sorter: true },
  { title: '种子ID', dataIndex: 'source_torrent_id', key: 'source_torrent_id', width: 110, sorter: true },
  { title: '资源文件夹', dataIndex: 'directory', key: 'directory', ellipsis: true, sorter: true },
  { title: t('reseed.targetInfoHash'), dataIndex: 'target_info_hash', key: 'target_info_hash', ellipsis: true },
  { title: t('reseed.matchMethod'), dataIndex: 'match_method', key: 'match_method', width: 100, customRender: ({ text }: { text: string }) => translateMatchMethod(text) },
  { title: t('reseed.confidence'), dataIndex: 'confidence', key: 'confidence', width: 80, sorter: true },
  { title: t('common.status'), key: 'status', width: 100, sorter: true },
  { title: '匹配时间', dataIndex: 'created_at', key: 'created_at', width: 160, sorter: true },
  { title: t('reseed.failReason'), dataIndex: 'fail_reason', key: 'fail_reason', ellipsis: true },
  { title: t('common.actions'), key: 'actions', width: 80 },
]

function statusColor(status: string) {
  const map: Record<string, string> = { idle: 'blue', running: 'green', completed: 'default', failed: 'red' }
  return map[status] || 'default'
}

async function fetchTask() {
  loading.value = true
  try {
    const resp = await reseedApi.getTask(taskId)
    task.value = resp.data.data || null
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function fetchMatches() {
  matchesLoading.value = true
  try {
    const resp = await reseedApi.getMatches(taskId, {
      page: matchesPage.value,
      pageSize: matchesPageSize.value,
      clientId: filter.clientId || undefined,
      site: filter.site || undefined,
      torrentId: filter.torrentId || undefined,
      status: filter.status || undefined,
      orderField: matchesOrder.field || undefined,
      order: matchesOrder.order || undefined,
    })
    matches.value = resp.data.data?.items ?? []
    matchesTotal.value = resp.data.data?.total ?? 0
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    matchesLoading.value = false
  }
}

function handleMatchesChange(pag: { current?: number; pageSize?: number }, _filters: unknown, sorter: { field?: string; order?: 'ascend' | 'descend' }) {
  if (pag.current) matchesPage.value = pag.current
  if (pag.pageSize) matchesPageSize.value = pag.pageSize
  if (sorter?.field && sorter.order) {
    matchesOrder.field = sorter.field
    matchesOrder.order = sorter.order === 'ascend' ? 'asc' : 'desc'
  } else {
    matchesOrder.field = ''
    matchesOrder.order = ''
  }
  fetchMatches()
}

async function clearAllMatches() {
  try {
    const resp = await reseedApi.clearAllMatches(taskId)
    const deleted = resp.data.data?.deleted ?? 0
    message.success(`已清除 ${deleted} 条记录`)
    selectedMatchKeys.value = []
    matchesPage.value = 1
    fetchMatches()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

function onFilterSearch() {
  matchesPage.value = 1
  selectedMatchKeys.value = []
  fetchMatches()
}

function onFilterReset() {
  filter.clientId = ''
  filter.site = ''
  filter.torrentId = ''
  filter.status = ''
  matchesOrder.field = ''
  matchesOrder.order = ''
  matchesPage.value = 1
  selectedMatchKeys.value = []
  fetchMatches()
}

function onSelectChange(keys: number[]) {
  selectedMatchKeys.value = keys
}

async function batchRetryFailed() {
  const failedIds = matches.value
    .filter(m => selectedMatchKeys.value.includes(m.id) && m.status === 'failed')
    .map(m => m.id)
  if (!failedIds.length) {
    message.warning('选中的记录中没有失败项')
    return
  }
  try {
    const resp = await reseedApi.batchRetryMatches(taskId, failedIds)
    const d = resp.data.data
    message.success(`重试完成：成功 ${d?.succeeded ?? 0}，失败 ${d?.failed ?? 0}`)
    selectedMatchKeys.value = []
    fetchMatches()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function batchDelete() {
  if (!selectedMatchKeys.value.length) return
  try {
    const resp = await reseedApi.batchDeleteMatches(taskId, selectedMatchKeys.value)
    const deleted = resp.data.data?.deleted ?? 0
    message.success(`已删除 ${deleted} 条`)
    selectedMatchKeys.value = []
    fetchMatches()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function retryMatch(matchId: number) {
  try {
    await reseedApi.retryMatch(taskId, matchId)
    message.success(t('reseed.retryTriggered'))
    fetchMatches()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function deleteNegativeCache() {
  if (!negDeleteInfoHash.value) return
  try {
    await reseedApi.deleteNegativeCache(taskId, negDeleteInfoHash.value, negDeleteSite.value || undefined)
    message.success(t('common.deleted'))
    negDeleteInfoHash.value = ''
    negDeleteSite.value = ''
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function fetchIYUULogs() {
  iyuuLoading.value = true
  try {
    const resp = await reseedApi.getIYUULogs(taskId, iyuuPage.value, iyuuPageSize.value)
    iyuuLogs.value = resp.data.data?.items ?? []
    iyuuTotal.value = resp.data.data?.total ?? 0
    iyuuStats.value = resp.data.data?.stats ?? null
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    iyuuLoading.value = false
  }
}

function handleIYUUChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) iyuuPage.value = pag.current
  if (pag.pageSize) iyuuPageSize.value = pag.pageSize
  fetchIYUULogs()
}

async function fetchFeatureLogs() {
  featureLoading.value = true
  try {
    const resp = await reseedApi.getFeatureLogs(taskId, featurePage.value, featurePageSize.value)
    featureLogs.value = resp.data.data?.items ?? []
    featureTotal.value = resp.data.data?.total ?? 0
    featureStats.value = resp.data.data?.stats ?? null
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    featureLoading.value = false
  }
}

function handleFeatureChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) featurePage.value = pag.current
  if (pag.pageSize) featurePageSize.value = pag.pageSize
  fetchFeatureLogs()
}

onMounted(() => {
  fetchSiteMap()
  fetchDownloaderMap()
  fetchTask()
  fetchMatches()
  fetchIYUULogs()
  fetchFeatureLogs()
})
</script>
