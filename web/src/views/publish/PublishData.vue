<template>
  <div style="padding: 24px">
    <div>
      <!-- 批量发布（站中心，§59.133 ③ / §59.166 一站多种接线） ═══ -->
      <div class="batch-publish-pane">
        <div class="batch-toolbar">
          <a-select
            v-model:value="selectedClient"
            style="width: 170px"
            :loading="clientsLoading"
            placeholder="下载器（全部）"
            allow-clear
            @change="onClientChange"
          >
            <a-select-option v-for="c in clients" :key="c.client_id" :value="c.client_id">
              {{ c.client_id }}（{{ c.paths.length }} 路径）
            </a-select-option>
          </a-select>
          <a-select
            v-model:value="selectedPath"
            style="width: 230px; margin-left: 10px"
            placeholder="保存路径（全部）"
            allow-clear
            :disabled="!selectedClient"
            @change="onBatchFilterChange"
          >
            <a-select-option v-for="p in pathOptions" :key="p.save_path" :value="p.save_path">
              {{ p.save_path }}（{{ p.count }}）
            </a-select-option>
          </a-select>
          <a-radio-group
            v-model:value="readyFilter"
            button-style="solid"
            size="small"
            style="margin-left: 10px"
            @change="onBatchFilterChange"
          >
            <a-radio-button value="all">全部</a-radio-button>
            <a-radio-button value="publishable" :disabled="!selectedTarget">可发布</a-radio-button>
            <a-radio-button value="published" :disabled="!selectedTarget">已发布</a-radio-button>
          </a-radio-group>
          <a-input-search
            v-model:value="injectSearch"
            placeholder="搜索簇名称..."
            style="width: 230px; margin-left: 10px"
            allow-clear
            @search="onBatchFilterChange"
          />
          <a-tag v-if="injectTotal" color="blue" style="margin-left: 8px">共 {{ injectTotal }} 簇</a-tag>
          <span v-if="selectedInjectHashes.length" :style="{ marginLeft: '8px', color: selectedInjectHashes.length > 100 ? '#cf1322' : '#1677ff' }">
            已选 {{ selectedInjectHashes.length }} 簇<span v-if="selectedInjectHashes.length > 100">（超单批上限 100——请分批）</span>
          </span>
          <a-button style="margin-left: 10px" @click="pickerOpen = true">
            {{ selectedTarget ? `目标站：${selectedTarget} ▾` : '选择发布站' }}
          </a-button>
        </div>

        <a-alert
          type="info"
          show-icon
          style="margin-bottom: 12px"
          message="一站多种：勾选多个种子簇 → 选择发布站 → 串行发布（站点配置间隔）。已覆盖目标站的簇不阻塞——发布时自动跳过并记录日志；源站禁转簇已隐藏。"
        />

        <a-table
          :columns="injectColumns"
          :data-source="filteredInjectRows"
          :loading="injectLoading"
          :pagination="{
            current: injectPage,
            pageSize: injectPageSize,
            total: injectTotal,
            showSizeChanger: true,
            pageSizeOptions: ['50', '100'],
            showTotal: (t: number) => `共 ${t} 簇`,
          }"
          row-key="hash"
          size="small"
          :scroll="{ x: 1100 }"
          :row-selection="{
            selectedRowKeys: selectedInjectHashes,
            onChange: onSelectionChange,
            getCheckboxProps: (record: SeedListItem) => ({
              disabled: !isValidHash(record.hash),
            }),
          }"
          @change="onInjectTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'">
              <div class="cluster-name">
                {{ record.name }}
                <a-tag v-if="!isValidHash(record.hash)" color="default" style="margin-left: 4px">观察期</a-tag>
              </div>
              <div v-if="record.title && record.title !== record.name" class="cluster-title">{{ record.title }}</div>
            </template>
            <template v-if="column.key === 'category'">
              <a-tag v-if="record.category" :color="categoryTagColor(record.category)" style="margin: 0">
                {{ CATEGORY_LABELS[record.category] || record.category }}
              </a-tag>
              <span v-else style="color: #999; font-size: 12px">—</span>
            </template>
            <template v-if="column.key === 'size'">
              {{ formatBytes(record.size) }}
            </template>
            <template v-if="column.key === 'sites'">
              <a-tooltip v-if="record.sites?.length" :title="record.sites.join('、')">
                <span>
                  <a-tag v-for="st in record.sites.slice(0, 4)" :key="st" size="small" style="margin: 1px">{{ st }}</a-tag>
                  <a-tag v-if="record.sites.length > 4" size="small" style="margin: 1px">+{{ record.sites.length - 4 }}</a-tag>
                </span>
              </a-tooltip>
              <span v-else style="color: #999; font-size: 12px">未知</span>
            </template>
            <template v-if="column.key === 'status'">
              <a-tag :color="statusColor(record.status)" style="margin: 0">{{ statusLabel(record.status) }}</a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-button size="small" @click="previewSeed(record)">预览种子</a-button>
            </template>
          </template>
        </a-table>

        <!-- §59.166 选站弹窗（平铺单选 + 右下发布——一种多站 Modal 同构）-->
        <a-modal
          v-model:open="pickerOpen"
          :title="batchTask ? (batchTask.finished ? '批量发布完成' : '批量发布中') : '选择发布站'"
          width="620px"
          :footer="null"
          :mask-closable="!batchTask || batchTask.finished"
        >
          <!-- 态 1：选站（batchTask 空）-->
          <template v-if="!batchTask">
            <SiteTiles v-model="pickerValue" :sites="tileSites" />
            <div style="margin-top: 20px; text-align: right">
              <a-button style="margin-right: 10px" @click="pickerOpen = false">关闭</a-button>
              <a-button
                type="primary"
                :disabled="!pickerValue || !selectedInjectHashes.length || selectedInjectHashes.length > 100 || pickerSiteBusy"
                :loading="batchSubmitting"
                @click="submitBatch"
              >
                发布到 {{ pickerValue || '目标站' }}（{{ selectedInjectHashes.length }} 种）
              </a-button>
            </div>
            <p style="margin-top: 8px; color: #999; font-size: 12px">
              点选站点即时生效；已覆盖目标站的簇不阻塞——发布时自动跳过并记录日志。
            </p>
          </template>
          <!-- 态 2：进度（公共组件——与一种多站共用）-->
          <PublishProgressPanel
            v-else-if="!batchTask.finished"
            :progress="{ done: batchTask.done, total: batchTask.total, currentTitle: batchTask.current_title }"
            :results="batchTask.results"
            row-mode="seed"
          />
          <!-- 态 3：完成汇总 -->
          <PublishProgressPanel
            v-else
            :results="batchTask.results"
            row-mode="seed"
            @done="batchTask = null"
          />
        </a-modal>

        <CrossSeedPanel
          v-model:open="previewPanelOpen"
          :preset-torrent="previewPreset"
          :initial-preview="previewDirect"
          @success="fetchInjectList"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { executeApi, type SiteBatchTask } from '@/api/formConfig'
import SiteTiles from './SiteTiles.vue'
import PublishProgressPanel from './PublishProgressPanel.vue'
import CrossSeedPanel from './CrossSeedPanel.vue'
import { message } from 'ant-design-vue'
import { seedConfigApi, type SeedListItem } from '@/api/publish'
import { CATEGORY_LABELS } from '@/generated/dict'
import { categoryTagColor } from '@/utils/categoryDisplay'
import { formatBytes } from '@/utils/format'


// ==================== Tab 1: 批量发布（§59.133 ③ 站中心 / §59.166 一站多种） ====================

const targetSites = ref<Array<{ name: string; has_pre_audit: boolean }>>([])
const selectedTarget = ref<string | undefined>(undefined)
const clients = ref<Array<{ client_id: string; paths: Array<{ save_path: string; count: number }> }>>([])
const clientsLoading = ref(false)
const selectedClient = ref<string | undefined>(undefined)
const selectedPath = ref<string | undefined>(undefined)
const readyFilter = ref<'all' | 'publishable' | 'published'>('all') // §59.166 三态（未选站默认全部）
const injectSearch = ref('')
const injectRows = ref<SeedListItem[]>([])
const injectTotal = ref(0)
const injectLoading = ref(false)
const injectPage = ref(1)
const injectPageSize = ref(50)
const selectedInjectHashes = ref<string[]>([])

const pathOptions = computed(() => {
  const c = clients.value.find(x => x.client_id === selectedClient.value)
  return c ? c.paths : []
})

// 簇 hash 有效性（观察期造键 `${client}|${name}` 非真 infohash——不可执行发布）
function isValidHash(h: string | undefined): boolean {
  return !!h && h.length === 40 && !h.includes('|')
}

async function fetchClients() {
  clientsLoading.value = true
  try {
    const resp = await seedConfigApi.uniquePaths()
    clients.value = resp.data?.data?.clients || []
  } catch { /* ignore */ } finally {
    clientsLoading.value = false
  }
}

function onClientChange() {
  selectedPath.value = undefined
  injectPage.value = 1
  fetchInjectList()
}

function onBatchFilterChange() {
  injectPage.value = 1
  fetchInjectList()
}

// §59.166 预览种子（CrossSeedPanel——一种多站同款；pending 簇经此返回编辑）
const previewPanelOpen = ref(false)
const previewPreset = ref<{ info_hash: string; name: string; size: number; save_path: string; client_id: string; source_site?: string } | null>(null)
const previewDirect = ref(false)

function previewSeed(record: SeedListItem) {
  previewPreset.value = {
    info_hash: record.hash,
    name: record.name,
    size: record.size,
    save_path: record.save_path,
    // §59.170 BUG-1: client_id 原样传字符串（"PT0"）——原 Number() NaN→0→'' 致截图 400
    client_id: record.client_id,
    source_site: record.site_name,
  }
  // §59.170 F2: 直开预览（含自动保存）仅 reviewed 行——非 reviewed 开编辑模式
  // （§59.166 自述"pending 簇经此返回编辑"本意；无门禁时空表单自动 PUT 会
  // 连坐簇 reviewed——46 行实锤）
  previewDirect.value = !!record.reviewed
  previewPanelOpen.value = true
}

// §59.166 选站弹窗状态（pickerValue 即时同步 selectedTarget——工具栏按钮显示）
const pickerOpen = ref(false)
const pickerValue = ref('')
watch(pickerValue, (v) => { selectedTarget.value = v || undefined })

const pickerSiteBusy = computed(() => {
  const t = batchTask.value
  return t !== null && !t.finished && t.target_site === pickerValue.value
})

const tileSites = computed(() => targetSites.value.map(t => ({ name: t.name, hasPreAudit: t.has_pre_audit })))

function onSelectionChange(keys: (string | number)[]) {
  selectedInjectHashes.value = keys.map(String)
}

// ═══ §59.166 一站多种：批量任务（提交+轮询+断线恢复）═══
const batchTask = ref<SiteBatchTask | null>(null)
const batchSubmitting = ref(false)
let pollTimer: ReturnType<typeof setTimeout> | undefined

// §59.166 数据状态列（一种多站同款）
const STATUS_META: Record<string, { label: string; color: string }> = {
  forbidden: { label: '禁转', color: 'red' },
  system_forbidden: { label: '系统禁转', color: 'red' },
  no_mapping: { label: '无源站映射', color: 'volcano' },
  reviewed: { label: '已审核', color: 'green' },
  pending: { label: '待审核', color: 'blue' },
  incomplete: { label: '配置不完整', color: 'orange' },
  unfetched: { label: '未获取', color: 'default' },
}
function statusLabel(st: string): string {
  return STATUS_META[st]?.label || st
}

function statusColor(st: string): string {
  return STATUS_META[st]?.color || 'default'
}

async function submitBatch() {
  if (!selectedTarget.value || !selectedInjectHashes.value.length) return
  const valid = selectedInjectHashes.value.filter(isValidHash)
  if (!valid.length) {
    message.warning('所选簇均无有效 infoHash（观察期簇需先完善数据）')
    return
  }
  if (valid.length < selectedInjectHashes.value.length) {
    message.info(`已剔除 ${selectedInjectHashes.value.length - valid.length} 个无有效 hash 的簇`)
  }
  batchSubmitting.value = true
  try {
    const res = await executeApi.executeSiteBatch(valid, selectedTarget.value)
    const tid = res.data?.data?.task_id
    if (tid) {
      batchTask.value = { task_id: tid, target_site: selectedTarget.value, total: res.data?.data?.total || selectedInjectHashes.value.length, done: 0, results: [], finished: false, started_at: new Date().toISOString() }
      selectedInjectHashes.value = []
      // §59.166 弹窗进度态：发布后弹窗不关（进度→汇总→[完成]手动关闭）
      selectedTarget.value = undefined
      pickerValue.value = ''
      schedulePoll(tid)
    }
  } catch (err) {
    // §59.166 回归审核补：40901 人话提示（同站互斥——可能来自其它标签页提交）
    const e2 = err as { response?: { status?: number; data?: { message?: string } }; message?: string }
    const status = e2?.response?.status
    if (status === 409) {
      message.warning('该站已有批量任务运行中，请等待完成')
    } else {
      // §59.166 实战回归：非 409 静默吞错=「点击发布没反应」——必须人话报错
      const detail = e2?.response?.data?.message || e2?.message || '请求失败'
      message.error('发布失败: ' + detail)
    }
  } finally {
    batchSubmitting.value = false
  }
}

function schedulePoll(taskId: string) {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = setTimeout(async () => {
    try {
      const res = await executeApi.siteBatchProgress(taskId)
      const t = res.data?.data
      if (t) batchTask.value = t
      if (t && !t.finished) schedulePoll(taskId)
    } catch (err) {
      // 404=任务过期（30 分钟 TTL）→ 终止轮询并标记完成；其他错误（网络抖动）继续
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 404) {
        if (batchTask.value) batchTask.value = { ...batchTask.value, finished: true, error: '任务进度已过期（结果见发布日志）' }
      } else {
        schedulePoll(taskId)
      }
    }
  }, 2000)
}

// 挂载恢复：全部活跃任务无锚查询（选站不持久化——刷新后站名已丢，§59.166 回归审核修正）
async function resumeActiveBatch() {
  try {
    const res = await executeApi.siteBatchActiveAll()
    const list = res.data?.data as SiteBatchTask[] | SiteBatchTask | null | undefined
    const t = Array.isArray(list) ? list[0] : list
    if (t && !t.finished) {
      batchTask.value = t
      pickerOpen.value = true // 断线恢复：自动开弹窗（进度态在眼前）
      schedulePoll(t.task_id)
    }
  } catch { /* 无任务 */ }
}

onUnmounted(() => {
  if (pollTimer) clearTimeout(pollTimer)
})

const injectColumns = [
  { title: '簇名称', dataIndex: 'name', key: 'name', ellipsis: true, sorter: (a: SeedListItem, b: SeedListItem) => a.name.localeCompare(b.name) },
  { title: '类型', key: 'category', width: 80, sorter: (a: SeedListItem, b: SeedListItem) => (a.category || '').localeCompare(b.category || '') },
  { title: '大小', key: 'size', width: 90, sorter: (a: SeedListItem, b: SeedListItem) => a.size - b.size },
  { title: '副本', key: 'copies', width: 90, align: 'center' as const, sorter: (a: SeedListItem, b: SeedListItem) => (a.copy_count ?? 1) - (b.copy_count ?? 1) },
  { title: '已有站点', key: 'sites', width: 220 },
  { title: '数据状态', key: 'status', width: 90, align: 'center' as const },
  { title: '操作', key: 'actions', width: 100 },
]


// 行集直通（已存在预判在行级 exist 列与勾选禁用表达——§59.166 重构）
const filteredInjectRows = computed(() => injectRows.value)

async function fetchTargetSites() {
  // §59.166: form-config targets（已启用发布配置——与 execute-site-batch 后端校验同口径）
  try {
    const resp = await executeApi.targets()
    targetSites.value = (resp.data?.data || []) as Array<{ name: string; has_pre_audit: boolean }>
  } catch { /* ignore */ }
}

async function fetchInjectList() {
  injectLoading.value = true
  try {
    const resp = await seedConfigApi.listSeeds({
      client_id: selectedClient.value || '',
      save_path: selectedPath.value || '',
      // §59.166 三态：publishable/published 走 target_site+publish_state（后端
      // publishable 自含 reviewed）；all 不带
      publish_state: (readyFilter.value === 'publishable' || readyFilter.value === 'published') ? readyFilter.value : '',
      target_site: readyFilter.value === 'all' ? '' : (selectedTarget.value || ''),
      // §59.143: 发布页隐藏源站禁转簇
      exclude_forbidden: 'true',
      search: injectSearch.value,
      page: injectPage.value,
      page_size: injectPageSize.value,
    })
    injectRows.value = ((resp.data?.data?.items || []) as SeedListItem[]).map((it) => ({
      ...it,
      hash: it.hash || `${it.client_id}|${it.name}`,
    }))
    injectTotal.value = resp.data?.data?.total || 0
    // 跨页保留勾选（批量发布跨页累积）；目标站变化时已存在语义变，onInjectFilterChange 显式清
  } catch {
    message.error('加载 ready 簇失败')
    injectRows.value = []
    injectTotal.value = 0
  } finally {
    injectLoading.value = false
    persistFilters()
  }
}

function onInjectFilterChange() {
  injectPage.value = 1
  // 筛选变化清勾选（目标站变化→已存在语义变；搜索变化→行集变）
  selectedInjectHashes.value = []
  fetchInjectList()
}

function onInjectTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) injectPage.value = pag.current
  if (pag.pageSize) injectPageSize.value = pag.pageSize
  fetchInjectList()
}


let injectSearchTimer: ReturnType<typeof setTimeout> | undefined
watch(injectSearch, () => {
  if (injectSearchTimer) clearTimeout(injectSearchTimer)
  injectSearchTimer = setTimeout(() => onInjectFilterChange(), 400)
})

// 换目标站联动（§59.166 用户定案）：选站→切"可发布"；清站→回"全部"
// （发布消费清站后筛选自动复位——可发布/已发布在未选站时无意义）
watch(selectedTarget, (v) => {
  if (v) readyFilter.value = 'publishable'
  else if (readyFilter.value !== 'all') readyFilter.value = 'all'
  if (!batchTask.value || batchTask.value.finished) resumeActiveBatch()
})

// §59.166 记忆：下载器/路径/ready/搜索/页大小（一种多站同款机制，独立 key）
const FILTERS_KEY = 'publish_data_filters'
function persistFilters() {
  try {
    localStorage.setItem(FILTERS_KEY, JSON.stringify({
      client: selectedClient.value || undefined,
      path: selectedPath.value || undefined,
      // ready 不持久化（§59.166：站视角三态离站无意义——恢复恒 all）
      search: injectSearch.value || undefined,
      page_size: injectPageSize.value,
    }))
  } catch { /* silent */ }
}
function restoreFilters() {
  try {
    const raw = localStorage.getItem(FILTERS_KEY)
    if (!raw) return
    const f = JSON.parse(raw) as { client?: string; path?: string; ready?: 'all' | 'publishable' | 'published'; search?: string; page_size?: number }
    if (f.client) selectedClient.value = f.client
    if (f.path) selectedPath.value = f.path
    // §59.166 回归审核：站不持久化——publishable/published 恢复后无站=禁用 radio
    // 矛盾选中态（定案④从恢复路径复现）——非 all 一律回退
    if (f.ready && f.ready === 'all') readyFilter.value = 'all'
    if (f.search) injectSearch.value = f.search
    if (f.page_size) injectPageSize.value = f.page_size
  } catch { /* silent */ }
}

onMounted(() => {
  restoreFilters()
  fetchClients()
  fetchTargetSites()
  fetchInjectList()
  resumeActiveBatch()
})
</script>

<style scoped>
.batch-panel {
  margin-top: 16px;
  padding: 12px;
  border: 1px solid #e8e8e8;
  border-radius: 8px;
  background: #fafafa;
}
.batch-current {
  margin: 8px 0 0;
  color: #555;
}
.inject-toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
  flex-wrap: wrap;
  gap: 4px;
}
:deep(.row-incomplete) {
  background-color: #fff2f0;
}
:deep(.row-incomplete:hover) {
  background-color: #ffe7e4;
}
.cell-missing {
  color: #cf1322;
  font-weight: 500;
}
.cluster-name {
  font-size: 13px;
  word-break: break-all;
}
.cluster-title {
  font-size: 12px;
  color: #999;
  margin-top: 2px;
}
</style>
