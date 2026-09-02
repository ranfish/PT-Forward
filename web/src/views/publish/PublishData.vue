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
            <a-radio-button value="ready">可发布</a-radio-button>
            <a-radio-button value="all">全部</a-radio-button>
            <a-radio-button value="pending">待完善</a-radio-button>
          </a-radio-group>
          <a-input-search
            v-model:value="injectSearch"
            placeholder="搜索簇名称..."
            style="width: 230px; margin-left: 10px"
            allow-clear
            @search="onBatchFilterChange"
          />
          <a-tag v-if="injectTotal" color="blue" style="margin-left: 8px">共 {{ injectTotal }} 簇</a-tag>
          <span v-if="selectedInjectHashes.length" style="margin-left: 8px; color: #1677ff">
            已选 {{ selectedInjectHashes.length }} 簇
          </span>
        </div>

        <!-- 目标站平铺单选 + 一键发布（§59.166 镜像一种多站点选交互）-->
        <div class="target-row">
          <span class="target-label">目标站</span>
          <div class="site-tiles">
            <div
              v-for="t in targetSites"
              :key="t.name"
              class="site-tile"
              :class="{ active: selectedTarget === t.name }"
              @click="onTargetPick(t.name)"
            >
              <span class="tile-name">{{ t.name }}</span>
              <a-tag v-if="t.has_pre_audit" color="blue" class="tile-tag">官方预检</a-tag>
            </div>
          </div>
          <a-button
            type="primary"
            style="margin-left: auto"
            :disabled="!selectedInjectHashes.length || !selectedTarget || (batchTask !== null && !batchTask.finished)"
            :loading="batchSubmitting"
            @click="submitBatch"
          >
            {{ batchTask && !batchTask.finished ? '发布中…' : `发布到 ${selectedTarget || '目标站'}（${selectedInjectHashes.length}）` }}
          </a-button>
        </div>

        <a-alert
          type="info"
          show-icon
          style="margin-bottom: 12px"
          message="一站多种：勾选多个种子簇 → 点亮一个目标站 → 一键串行发布（站点配置间隔）。已存在目标站的簇默认不可选；源站禁转簇已隐藏。"
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
            pageSizeOptions: ['50', '100', '200'],
            showTotal: (t: number) => `共 ${t} 簇`,
          }"
          row-key="hash"
          size="small"
          :scroll="{ x: 1100 }"
          :row-selection="{
            selectedRowKeys: selectedInjectHashes,
            onChange: onSelectionChange,
            getCheckboxProps: (record: SeedListItem) => ({
              disabled: !selectedTarget || existsOnTarget(record) || !isValidHash(record.hash),
            }),
          }"
          @change="onInjectTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'">
              <div class="cluster-name">{{ record.name }}</div>
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
            <template v-if="column.key === 'exist'">
              <template v-if="selectedTarget && existsOnTarget(record)">
                <a-tag color="warning">已存在</a-tag>
              </template>
              <a-tag v-else-if="selectedTarget && !isValidHash(record.hash)" color="default">无有效 hash</a-tag>
              <a-tag v-else-if="selectedTarget" color="success">可发布</a-tag>
              <span v-else style="color: #999; font-size: 12px">未选站</span>
            </template>
            <template v-if="column.key === 'actions'">
              <a-button size="small" @click="goRefine(record)">完善数据</a-button>
            </template>
          </template>
        </a-table>

        <!-- §59.166 一站多种：批量任务进度/结果 -->
        <div v-if="batchTask" class="batch-panel">
          <template v-if="batchTask && !batchTask.finished">
            <a-progress
              :percent="batchPercent"
              :format="() => `${batchTask?.done ?? 0}/${batchTask?.total ?? 0}`"
              status="active"
            />
            <p class="batch-current">
              正在发布 {{ batchTask.done + 1 }}/{{ batchTask.total }}
              <span v-if="batchTask.current_title">：{{ batchTask.current_title }}</span>
              <a-spin size="small" style="margin-left: 8px" />
            </p>
          </template>
          <template v-else>
            <a-alert :type="batchAlertType" show-icon style="margin-bottom: 12px">
              <template #message>
                批量发布完成：发布成功 {{ successCount }} 站次 · 已存在 {{ dupCount }} · 失败 {{ failCount }}
                <span v-if="batchTask.error" style="color: #cf1322">（{{ batchTask.error }}）</span>
              </template>
            </a-alert>
            <a-table
              v-if="batchTask.results.length"
              :columns="batchResultColumns"
              :data-source="batchTask.results"
              :pagination="false"
              size="small"
              row-key="info_hash"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'status'">
                  <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
                </template>
                <template v-else-if="column.key === 'url'">
                  <a v-if="record.url" :href="record.url" target="_blank">查看种子</a>
                  <span v-else style="color: #999">—</span>
                </template>
                <template v-else-if="column.key === 'message'">
                  <span :style="record.status === 'failed' ? 'color: #cf1322' : ''">{{ record.message || '—' }}</span>
                </template>
              </template>
            </a-table>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { executeApi, type SiteBatchTask } from '@/api/formConfig'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { seedConfigApi, type SeedListItem } from '@/api/publish'
import { CATEGORY_LABELS } from '@/generated/dict'
import { categoryTagColor } from '@/utils/categoryDisplay'
import { formatBytes } from '@/utils/format'

const router = useRouter()

// ==================== Tab 1: 批量发布（§59.133 ③ 站中心 / §59.166 一站多种） ====================

const targetSites = ref<Array<{ name: string; has_pre_audit: boolean }>>([])
const selectedTarget = ref<string | undefined>(undefined)
const clients = ref<Array<{ client_id: string; paths: Array<{ save_path: string; count: number }> }>>([])
const clientsLoading = ref(false)
const selectedClient = ref<string | undefined>(undefined)
const selectedPath = ref<string | undefined>(undefined)
const readyFilter = ref<'ready' | 'all' | 'pending'>('ready') // §59.166 用户定案：默认只显示配置好的种子
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

function onTargetPick(name: string) {
  selectedTarget.value = selectedTarget.value === name ? undefined : name
  // 回归审核补：换站清勾选——已存在语义随目标站变化（旧站中心逻辑，重构时丢失）
  selectedInjectHashes.value = []
}

function onSelectionChange(keys: (string | number)[]) {
  selectedInjectHashes.value = keys.map(String)
}

// ═══ §59.166 一站多种：批量任务（提交+轮询+断线恢复）═══
const batchTask = ref<SiteBatchTask | null>(null)
const batchSubmitting = ref(false)
let pollTimer: ReturnType<typeof setTimeout> | undefined

const batchPercent = computed(() => {
  const t = batchTask.value
  if (!t) return 0
  return Math.round((t.done / Math.max(t.total, 1)) * 100)
})
const successCount = computed(() => (batchTask.value?.results || []).filter(r => ['pushed', 'pushed_existing'].includes(r.status)).length)
const dupCount = computed(() => (batchTask.value?.results || []).filter(r => ['duplicate', 'existing'].includes(r.status)).length)
const failCount = computed(() => (batchTask.value?.results || []).filter(r => r.status === 'failed').length)
const batchAlertType = computed(() => (failCount.value > 0 ? 'warning' : 'success'))
const batchResultColumns = [
  { title: '种子', key: 'title', ellipsis: true },
  { title: '状态', key: 'status', width: 100 },
  { title: '说明', key: 'message', ellipsis: true },
  { title: '链接', key: 'url', width: 100 },
]

function statusText(st: string): string {
  const m: Record<string, string> = {
    pushed: '发布成功', pushed_existing: '已推种', duplicate: '站上已有', existing: '站上已有', failed: '失败',
  }
  return m[st] || st
}
function statusColor(st: string): string {
  if (['pushed', 'pushed_existing'].includes(st)) return 'success'
  if (['duplicate', 'existing'].includes(st)) return 'warning'
  return 'error'
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
      schedulePoll(tid)
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

// 挂载恢复：目标站有运行中任务则接回进度（断线无伤）
async function resumeActiveBatch() {
  if (!selectedTarget.value) return
  try {
    const res = await executeApi.siteBatchActive(selectedTarget.value)
    const t = res.data?.data
    if (t && !t.finished) {
      batchTask.value = t
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
  { title: '目标站状态', key: 'exist', width: 110, align: 'center' as const },
  { title: '操作', key: 'actions', width: 100 },
]

function existsOnTarget(record: SeedListItem): boolean {
  if (!selectedTarget.value) return false
  return (record.sites || []).includes(selectedTarget.value)
}

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
      ready: readyFilter.value === 'ready' ? 'true' : (readyFilter.value === 'pending' ? 'false' : ''),
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

// 引导跳回种子配置页（同 ② deep-link）
function goRefine(record: SeedListItem) {
  router.push({
    path: '/publish/seeds',
    query: {
      client_id: record.client_id,
      save_path: record.save_path,
      name: record.name,
      focus: '1',
    },
  })
}

let injectSearchTimer: ReturnType<typeof setTimeout> | undefined
watch(injectSearch, () => {
  if (injectSearchTimer) clearTimeout(injectSearchTimer)
  injectSearchTimer = setTimeout(() => onInjectFilterChange(), 400)
})

// 换目标站：恢复该站运行中任务（§59.166 断线无伤）
watch(selectedTarget, () => {
  if (!batchTask.value || batchTask.value.finished) resumeActiveBatch()
})

onMounted(() => {
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
