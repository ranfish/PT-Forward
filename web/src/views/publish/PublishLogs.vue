<template>
  <div style="padding: 24px">
    <!-- §59.507: 头部整体左对齐（原 space-between 标题左工具右——
         用户定案工具紧跟标题流式排列） -->
    <div style="margin-bottom: 16px; display: flex; align-items: center; gap: 12px; flex-wrap: wrap">
      <h3 style="margin: 0">发布日志</h3>
      <a-space>
        <!-- §59.506: 目标站模糊搜索+自动完成（用户定案：输入任意子串模糊
             命中·输入时下拉实时过滤候选站名——候选源=发布记录出现过的站
             （点选必命中）·对齐站点管理搜索的 includes 语义） -->
        <a-auto-complete
          v-model:value="searchTarget"
          :options="targetSiteOptions"
          style="width: 200px"
          placeholder="目标站搜索"
          allow-clear
          @search="onTargetSiteInput"
          @select="onFilterChange"
          @keydown.enter="onFilterChange"
        />
        <a-select
          v-model:value="statusFilter"
          style="width: 170px"
          placeholder="状态"
          allow-clear
          @change="onFilterChange"
        >
          <!-- §59.502 附:筛选项与状态列 statusLabel 单点对齐（文案零分叉——
               旧下拉只有 completed/exists 等旧值与状态列不一致） -->
          <a-select-option v-for="v in statusFilterValues" :key="v" :value="v">
            {{ statusLabel(v) }}
          </a-select-option>
        </a-select>
        <a-select
          v-model:value="triggerFilter"
          style="width: 120px"
          placeholder="触发"
          allow-clear
          @change="onFilterChange"
        >
          <a-select-option value="manual">手动</a-select-option>
          <a-select-option value="batch">批量</a-select-option>
          <a-select-option value="reseed">辅种</a-select-option>
          <a-select-option value="wizard">向导</a-select-option>
        </a-select>
        <a-range-picker v-model:value="dateRange" @change="onFilterChange" />
        <a-button @click="fetchData"><ReloadOutlined /></a-button>
        <!-- §59.374 B2: 批量补推（加种失败选中集——幂等：下载器自动校验） -->
        <a-popconfirm :title="`确定批量补推 ${selectedRowKeys.length} 条？`" @confirm="batchRepush">
          <a-badge :count="selectedRowKeys.length" :offset="[6, 0]">
            <a-button :disabled="selectedRowKeys.length === 0" :loading="batchRepushing">批量补推</a-button>
          </a-badge>
        </a-popconfirm>
        <a-button :disabled="tableData.length === 0" @click="exportCSV"><DownloadOutlined /> CSV</a-button>
        <a-switch
          v-model:checked="autoRefresh"
          checked-children="自动"
          un-checked-children="手动"
          size="small"
        />
      </a-space>
    </div>

    <a-table
      :columns="columns"
      :data-source="tableData"
      :loading="loading"
      :pagination="pagination"
      :row-selection="{ selectedRowKeys, onChange: onSelectionChange, getCheckboxProps: checkboxProps }"
      row-key="id"
      :scroll="{ x: 1170 }"
      size="small"
      @change="onTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'trigger'">
          <a-tag :color="triggerColor(record.trigger)" style="font-size: 11px">{{ triggerLabel(record.trigger) }}</a-tag>
        </template>
        <template v-if="column.key === 'title'">
          <!-- §59.502: 双行单行省略（长标题挤压时间列——width+overflow 约束） -->
          <div style="max-width: 100%">
            <div v-if="record.subtitle" :title="record.subtitle" style="color: #666; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ record.subtitle }}</div>
            <div :title="record.title" style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ record.title || record.target_site }}</div>
          </div>
        </template>
        <template v-else-if="column.key === 'source_site'">
          <a-tag>{{ record.source_site }}</a-tag>
        </template>
        <template v-else-if="column.key === 'target_site'">
          <a-tag color="blue">{{ record.target_site }}</a-tag>
        </template>
        <template v-else-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
        </template>
        <template v-else-if="column.key === 'seeded'">
          <a-tag v-if="record.seeded" color="green">已加种</a-tag>
          <a-tag v-else-if="record.seed_error" color="red">加种失败</a-tag>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'cost_ms'">
          <span v-if="record.cost_ms">{{ formatCost(record.cost_ms) }}</span>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'publish_url'">
          <a v-if="record.publish_url" :href="detailHref(record.publish_url)" target="_blank" style="font-size: 12px">查看</a>
          <span v-else style="color: #ccc">-</span>
        </template>
        <template v-else-if="column.key === 'created_at'">
          {{ formatTime(record.created_at) }}
        </template>
        <template v-else-if="column.key === 'action'">
          <a-space>
            <a-button
              v-if="record.candidate_id"
              size="small"
              type="link"
              @click="$router.push(`/publish/groups/${record.candidate_id}`)"
            >
              详情
            </a-button>
            <!-- §59.159: 补推（发布成功未推送的修复通道——push_only 直推站上种子） -->
            <a-button
              v-if="canRepush(record)"
              size="small"
              type="link"
              :loading="repushing === record.id"
              @click="repush(record)"
            >
              补推
            </a-button>
          </a-space>
          <a-popconfirm title="确定删除此记录？" @confirm="handleDelete(record)">
            <a-button size="small" type="link" danger>删除</a-button>
          </a-popconfirm>
        </template>
      </template>

      <template #expandedRowRender="{ record }">
        <div v-if="record.error_message || record.skip_reason" style="padding: 4px 0">
          <a-alert
            v-if="record.error_message"
            type="error"
            show-icon
            :message="record.error_message"
            style="margin-bottom: 4px"
          />
          <a-alert
            v-if="record.skip_reason"
            type="warning"
            show-icon
            :message="`跳过原因: ${record.skip_reason}`"
          />
        </div>
        <div v-if="record.logs" style="padding: 4px 0">
          <a-typography-text type="secondary" style="font-size: 12px; white-space: pre-wrap; font-family: monospace">{{ record.logs }}</a-typography-text>
        </div>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { executeApi } from '@/api/formConfig'
import client from '@/api/client'
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import { ReloadOutlined, DownloadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { publishApi } from '@/api/publish'
import type { PublishResultRecord } from '@/api/types'
import { formatTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()

const STORAGE_KEY = 'publish_logs_filters'

interface PersistedFilters {
  search?: string
  status?: string
  trigger?: string
  page_size?: number
  date_start?: string
  date_end?: string
}

function loadPersistedFilters(): PersistedFilters {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw) as PersistedFilters
  } catch { /* silent */ }
  return {}
}

function persistFilters() {
  const data: PersistedFilters = {
    search: searchTarget.value || undefined,
    status: statusFilter.value,
    trigger: triggerFilter.value,
    page_size: pagination.value.pageSize,
    date_start: dateRange.value?.[0]?.format('YYYY-MM-DD'),
    date_end: dateRange.value?.[1]?.format('YYYY-MM-DD'),
  }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
  } catch { /* silent */ }
}

const persisted = loadPersistedFilters()

const loading = ref(false)
const tableData = ref<PublishResultRecord[]>([])
const searchTarget = ref(persisted.search || '')

// §59.506: 目标站自动完成——候选源=发布记录出现过的站（DistinctTargetSites
// 端点·按量降序·点选必命中）；输入实时 includes 过滤（对齐站点管理搜索语义）
const allTargetSites = ref<string[]>([])
const targetSiteOptions = computed(() => {
  const q = searchTarget.value.trim().toLowerCase()
  if (!q) return allTargetSites.value.map(s => ({ value: s }))
  return allTargetSites.value
    .filter(s => s.toLowerCase().includes(q))
    .map(s => ({ value: s }))
})
function onTargetSiteInput() {
  if (allTargetSites.value.length === 0) {
    client.get('/publish/results/target-sites')
      .then((resp) => {
        allTargetSites.value = (resp.data?.data?.sites as string[]) || []
      })
      .catch(() => { /* silent——候选源缺省不影响搜索本身 */ })
  }
}
const statusFilter = ref<string | undefined>(persisted.status)
const triggerFilter = ref<string | undefined>(persisted.trigger)
const dateRange = ref<[Dayjs, Dayjs] | undefined>(
  persisted.date_start && persisted.date_end
    ? [dayjs(persisted.date_start), dayjs(persisted.date_end)]
    : undefined
)
const autoRefresh = ref(true)

const pagination = ref({
  current: 1,
  pageSize: persisted.page_size || 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`,
})

// §59.502 附:状态筛选值集——与状态列 statusLabel 单点对齐（文案调
// statusLabel(v) 零分叉）。顺序=现役主流（§59.159 四终态+dedup）在前、
// 存量兼容值（completed/exists/edited——旧记录）在后。seed_failed 为
// 跨状态子集筛选（seed_error 非空未加种·§59.374 B2 后端分支）非真实
// status 值。
// §59.507: 筛选项终集 13→8——243 生产 3288 行回归（五值+uploaded 可达）：
// 删 publishing（同步落终态·DB 恒无死值）+旧值四项（completed/exists/
// edited/skipped·243 零行）；statusLabel/statusColor 保留旧值翻译（历史行
// 显示兼容·仅筛选项删）；unreviewed/ineligible 零行按需生长不预加。
const statusFilterValues = [
  'pushed', 'pushed_existing', 'uploaded', 'uploaded_existing',
  'existing', 'duplicate', 'failed', 'seed_failed',
]

// §59.502 附:窄数据列收紧（数据不宽表头宽——旧 x:1770 超列宽总和 530px
// 把时间列推出视口要拉滚动条）。列宽总和 1134+勾选列 32≈1170。
const columns = [
  { title: '触发', key: 'trigger', width: 60 },
  { title: '源站', key: 'source_site', width: 80 },
  { title: '目标站', key: 'target_site', width: 80 },
  { title: '种子ID', dataIndex: 'torrent_id', key: 'torrent_id', width: 70 },
  { title: '标题', key: 'title', ellipsis: true, width: 300 },
  { title: '状态', key: 'status', width: 84 },
  { title: '加种', key: 'seeded', width: 60 },
  { title: '耗时', key: 'cost_ms', width: 70 },
  { title: '链接', key: 'publish_url', width: 60 },
  { title: '时间', key: 'created_at', width: 150 },
  { title: '操作', key: 'action', width: 120, fixed: 'right' as const },
]

let pollTimer: ReturnType<typeof setInterval> | null = null
let pollRefreshing = false

const repushing = ref<number | null>(null)  // 单颗补推 loading 键
// §59.374 B2: 多选批量补推——仅加种失败行可选（seed_error 非空未 seeded）
const selectedRowKeys = ref<number[]>([])
const batchRepushing = ref<boolean | undefined>(undefined)
function checkboxProps(record: Record<string, unknown>): { disabled: boolean } {
  return { disabled: !(record.seed_error && !record.seeded) }
}
function onSelectionChange(keys: (number | string)[]) {
  selectedRowKeys.value = keys.map(Number)
}
async function batchRepush() {
  if (selectedRowKeys.value.length === 0) return
  batchRepushing.value = true
  try {
    const res = await publishApi.batchRepush(selectedRowKeys.value)
    const d = res.data?.data
    if (d) {
      if (d.pushed > 0) {
        message.success(`批量补推：${d.pushed} 成功 / ${d.failed} 失败`)
      } else {
        message.warning(`批量补推全部失败（${d.failed} 条）`)
      }
      selectedRowKeys.value = []
      await fetchData(true)
    }
  } catch (e) {
    message.error(String(e))
  } finally {
    batchRepushing.value = undefined
  }
}

// §59.502: 补推判据纯粹化（用户定案两轮收敛）——核心场景=发布链
// 欠的收尾：上传成功→站方返回 tid→下载 .torrent/推下载器失败（站点或
// 网络抖动）→ uploaded（一公里断裂·补推=完成自己的收尾·幂等轻量）。
// 附带=duplicate（辅种增益·tid 在手做种别人老种）。其余排除：
// pushed/pushed_existing 已推种（seeded 是 status 投影恒 1——"推过
// 但校验不过"数据形态不存在·用户论证：不是同一种子补推也无用）；
// failed/existing 无 tid（existing=辅种业务范畴·无权威 ID）。
function canRepush(r: { torrent_id?: string; status?: string; downloader_id?: string; save_path?: string }): boolean {
  if (!r.torrent_id || !r.downloader_id || !r.save_path) return false
  return ['uploaded', 'duplicate'].includes(r.status ?? '')
}

async function repush(r: { id: number; torrent_id?: string; target_site?: string; source_info_hash?: string; downloader_id?: string; save_path?: string }) {
  repushing.value = r.id
  try {
    const res = await executeApi.execute(r.source_info_hash ?? '', r.target_site ?? '', {
      pushOnly: true,
      torrentId: r.torrent_id ?? '',
      dryRun: false,
      pushClientId: r.downloader_id ?? '',
      pushSavePath: r.save_path ?? '',
    })
    const result = res.data?.data?.result
    if (result && ['pushed', 'pushed_existing'].includes(result.status)) {
      message.success(`补推成功：${result.status}`)
      await fetchData(true)
    } else {
      message.warning(`补推未成功：${result?.message || result?.status || '未知'}`)
    }
  } catch (e) {
    message.error(String(e))
  } finally {
    repushing.value = null
  }
}

async function fetchData(silent = false) {
  if (!silent) loading.value = true
  if (pollRefreshing) return
  pollRefreshing = true
  try {
    const params: Record<string, unknown> = {
      page: pagination.value.current,
      pageSize: pagination.value.pageSize,
    }
    if (statusFilter.value) params.status = statusFilter.value
    if (triggerFilter.value) params.trigger = triggerFilter.value
    if (searchTarget.value) params.target_site = searchTarget.value
    if (dateRange.value && dateRange.value[0] && dateRange.value[1]) {
      params.start_date = dateRange.value[0].format('YYYY-MM-DD')
      params.end_date = dateRange.value[1].format('YYYY-MM-DD')
    }
    const resp = await publishApi.listResults(params as Parameters<typeof publishApi.listResults>[0])
    const data = resp.data?.data
    if (data) {
      tableData.value = data.items as PublishResultRecord[]
      pagination.value.total = data.total
    }
  } catch {
    // silent
  } finally {
    loading.value = false
    pollRefreshing = false
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    if (autoRefresh.value && !pollRefreshing) {
      fetchData(true)
    }
  }, 3000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(autoRefresh, (on) => {
  if (on) startPolling()
})

watch([searchTarget, statusFilter, triggerFilter, dateRange], persistFilters)

function onFilterChange() {
  pagination.value.current = 1
  fetchData()
}

function onTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) pagination.value.current = pag.current
  if (pag.pageSize) pagination.value.pageSize = pag.pageSize
  persistFilters()
  fetchData()
}

async function handleDelete(record: PublishResultRecord) {
  try {
    await publishApi.deleteResult(record.id)
    message.success('已删除')
    fetchData()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

function statusColor(status: string): string {
  const map: Record<string, string> = {
    completed: 'green',
    edited: 'green',
    failed: 'red',
    skipped: 'orange',
    exists: 'gold',
    duplicate: 'gold', // §59.491: 站上已有（在线/本地 dedup 命中——防重正确终态）
    publishing: 'blue',
    // §59.502: §59.159 四终态映射补全（此前缺——中英混杂根源）
    pushed: 'green',
    pushed_existing: 'green', // 已推种（302 权威 ID·含下载器已有形态）
    uploaded: 'gold', // 上传成功未做种（一公里断裂——补推核心场景）
    uploaded_existing: 'gold', // 上传成功（站上已有形态）未做种
    existing: 'gold', // 站上已有（文案判定·无权威 ID——辅种业务范畴）
    seed_failed: 'orange', // §59.374 B2 子集筛选用（非真实 status·状态列不显示）
  }
  return map[status] || 'default'
}

// §59.491: 链接协议防御——存量记录 publish_url 无协议头（detailURLOf 旧版
// 裸 Domain 拼接）·裸 href 会被浏览器当相对路径拼系统地址。根修在后端·
// 此处兜底存量行。
function detailHref(url: string): string {
  if (/^https?:\/\//.test(url)) return url
  return 'https://' + url
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    completed: '成功',
    edited: '已编辑',
    failed: '发布失败', // §59.491: 与"加种失败"区分（加种=推下载器·发布=上传站方）
    skipped: '跳过',
    exists: '已存在',
    duplicate: '站上已有', // §59.491: 在线/本地 dedup 命中（tid 在记录·可补推做种）
    publishing: '发布中',
    // §59.502: §59.159 四终态映射补全（此前缺——显示英文原值）
    pushed: '发布成功',
    pushed_existing: '已发布·站上已有', // 302 权威 ID 已推种（含下载器已有）
    uploaded: '已上传未做种', // 一公里断裂——补推核心场景
    uploaded_existing: '已上传·站上已有', // 上传成功（已有形态）未做种
    existing: '站上已有', // 文案判定·无权威 ID·辅种业务范畴（同 duplicate 文案·按钮区分）
    seed_failed: '加种失败', // §59.374 B2 子集筛选用（非真实 status·状态列不显示）
  }
  return map[status] || status
}

function formatCost(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}min`
}

function triggerColor(trigger: string): string {
  const map: Record<string, string> = {
    manual: 'blue',
    batch: 'purple',
    reseed: 'cyan',
    wizard: 'geekblue',
  }
  return map[trigger] || 'default'
}

function triggerLabel(trigger: string): string {
  const map: Record<string, string> = {
    manual: '手动',
    batch: '批量',
    reseed: '辅种',
    wizard: '向导',
  }
  return map[trigger] || trigger || '-'
}

onMounted(() => {
  applyRouteQuery()
  fetchData()
  startPolling()
})

function applyRouteQuery() {
  const q = route.query
  let hasQuery = false
  if (q.status) {
    statusFilter.value = q.status as string
    hasQuery = true
  }
  if (q.trigger) {
    triggerFilter.value = q.trigger as string
    hasQuery = true
  }
  if (q.target_site) {
    searchTarget.value = q.target_site as string
    hasQuery = true
  }
  if (hasQuery) {
    router.replace({ query: {} })
  }
}

onUnmounted(() => {
  stopPolling()
})

function exportCSV() {
  const headers = ['时间', '源站', '目标站', '标题', '状态', '耗时', '链接', '错误']
  const rows = tableData.value.map((r: PublishResultRecord) => [
    formatTime(r.created_at),
    r.source_site || '',
    r.target_site || '',
    (r.title || r.subtitle || '').replace(/"/g, '""'),
    r.status || '',
    r.cost_ms ? formatCost(r.cost_ms) : '',
    r.publish_url || '',
    (r.error_message || '').replace(/"/g, '""').replace(/\n/g, ' '),
  ])
  const csv = [headers, ...rows].map(row => row.map(cell => `"${cell}"`).join(',')).join('\n')
  const blob = new Blob(['\ufeff' + csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `publish-logs-${new Date().toISOString().slice(0, 10)}.csv`
  link.click()
  URL.revokeObjectURL(url)
}
</script>
