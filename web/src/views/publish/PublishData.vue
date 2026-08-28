<template>
  <div style="padding: 24px">
    <a-tabs v-model:active-key="activeTab">
      <!-- ═══ Tab 1: 灌入发布（站中心，§59.133 ③） ═══ -->
      <a-tab-pane key="inject" tab="灌入发布">
        <div class="inject-toolbar">
          <a-select
            v-model:value="selectedTarget"
            style="width: 220px"
            :loading="targetSitesLoading"
            placeholder="选择目标站"
            show-search
            :filter-option="filterSiteOption"
            @change="onInjectFilterChange"
          >
            <a-select-option v-for="s in targetSites" :key="s.name" :value="s.name" :label="s.name">
              {{ s.name }}
              <a-tag v-if="!s.hasCookie" color="red" size="small" style="margin-left: 4px">缺 cookie</a-tag>
            </a-select-option>
          </a-select>
          <a-input-search
            v-model:value="injectSearch"
            placeholder="搜索簇名称..."
            style="width: 260px; margin-left: 12px"
            allow-clear
            @search="onInjectFilterChange"
          />
          <a-radio-group
            v-model:value="existFilter"
            button-style="solid"
            size="small"
            style="margin-left: 12px"
            @change="onInjectFilterChange"
          >
            <a-radio-button value="all">全部</a-radio-button>
            <a-radio-button value="new">未存在</a-radio-button>
          </a-radio-group>
          <a-tag v-if="selectedTarget" :color="publishableCount > 0 ? 'green' : 'default'" style="margin-left: 8px">
            {{ selectedTarget }}：本页可灌入 {{ publishableCount }} 簇
          </a-tag>
          <a-tooltip title="提交链路接线中（TagApplier 灰度，R3-5）">
            <a-button
              type="primary"
              style="margin-left: auto"
              :disabled="selectedInjectHashes.length === 0"
            >
              发布到 {{ selectedTarget || '目标站' }} ({{ selectedInjectHashes.length }})
            </a-button>
          </a-tooltip>
        </div>

        <a-alert
          type="info"
          show-icon
          style="margin-bottom: 12px"
          message="一站多种：以站点为中心——选定目标站，批量将 ready 簇灌入发布。已存在该站的簇默认不可选；源站禁转簇已隐藏。"
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
            size: 'small',
          }"
          row-key="hash"
          size="small"
          :scroll="{ x: 1000 }"
          :row-selection="{
            selectedRowKeys: selectedInjectHashes,
            onChange: (keys: string[]) => selectedInjectHashes = keys,
            getCheckboxProps: (record: SeedListItem) => ({ disabled: !selectedTarget || existsOnTarget(record) }),
          }"
          @change="onInjectTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'">
              <div class="cluster-name">{{ record.name }}</div>
              <div v-if="record.title && record.title !== record.name" class="cluster-title">{{ record.title }}</div>
            </template>
            <template v-if="column.key === 'category'">
              <a-tag v-if="record.category" :color="categoryTagColor(record.category)" style="margin: 0">{{ CATEGORY_LABELS[record.category] || record.category }}</a-tag>
              <span v-else style="color: #999; font-size: 12px">—</span>
            </template>
            <template v-if="column.key === 'size'">
              {{ formatBytes(record.size) }}
            </template>
            <template v-if="column.key === 'copies'">
              <a-tag :color="(record.copy_count ?? 1) > 1 ? 'blue' : 'default'">{{ record.copy_count ?? 1 }} 副本</a-tag>
            </template>
            <template v-if="column.key === 'sites'">
              <a-tooltip v-if="record.sites?.length" :title="record.sites.join('、')">
                <span>
                  <a-tag v-for="s in record.sites.slice(0, 4)" :key="s" size="small" style="margin: 1px">{{ s }}</a-tag>
                  <a-tag v-if="record.sites.length > 4" size="small" style="margin: 1px">+{{ record.sites.length - 4 }}</a-tag>
                </span>
              </a-tooltip>
              <span v-else style="color: #999; font-size: 12px">未知</span>
            </template>
            <template v-if="column.key === 'exist'">
              <template v-if="selectedTarget && existsOnTarget(record)">
                <a-tag color="warning">已存在</a-tag>
              </template>
              <a-tag v-else-if="selectedTarget" color="success">可灌入</a-tag>
              <span v-else style="color: #999; font-size: 12px">未选站</span>
            </template>
            <template v-if="column.key === 'actions'">
              <a-button size="small" @click="goRefine(record)">完善数据</a-button>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <!-- ═══ Tab 2: 数据管理（行级 metadata，原功能保留） ═══ -->
      <a-tab-pane key="manage" tab="数据管理">
        <div style="margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center">
          <a-space>
            <template v-if="selectedIds.length > 0">
              <a-button size="small" type="primary" @click="batchReview(true)">
                批量审核 ({{ selectedIds.length }})
              </a-button>
              <a-button size="small" @click="batchReview(false)">取消审核</a-button>
              <a-popconfirm :title="`确定删除选中的 ${selectedIds.length} 条记录？`" @confirm="batchDelete">
                <a-button size="small" danger>批量删除</a-button>
              </a-popconfirm>
            </template>
          </a-space>
          <a-space>
            <a-radio-group v-model:value="reviewStatus" button-style="solid" size="small" @change="onFilterChange">
              <a-radio-button value="all">全部</a-radio-button>
              <a-radio-button value="reviewed">已审核</a-radio-button>
              <a-radio-button value="unreviewed">待审核</a-radio-button>
            </a-radio-group>
            <a-button type="primary" @click="batchFetchOpen = true"><PlusOutlined /> 获取数据</a-button>
            <a-input-search
              v-model:value="searchQuery"
              placeholder="搜索标题或副标题"
              style="width: 300px"
              allow-clear
              @search="onFilterChange"
            />
            <a-select
              v-model:value="sourceSiteFilter"
              style="width: 200px"
              placeholder="源站筛选"
              allow-clear
              @change="onFilterChange"
            >
              <a-select-option v-for="s in sourceSites" :key="s" :value="s">{{ s }}</a-select-option>
            </a-select>
            <a-button @click="fetchData"><ReloadOutlined /></a-button>
          </a-space>
        </div>

        <a-table
          :columns="columns"
          :data-source="tableData"
          :loading="loading"
          :pagination="pagination"
          row-key="id"
          :scroll="{ x: 1380 }"
          size="small"
          :row-class-name="(record: SeedDataRow) => completenessPercent(record) < 50 ? 'row-incomplete' : ''"
          :row-selection="{ selectedRowKeys: selectedIds, onChange: (keys: number[]) => selectedIds = keys }"
          @change="onTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'title'">
              <div>
                <div v-if="record.subtitle" style="color: #666; font-size: 12px">{{ record.subtitle }}</div>
                <div :class="{ 'cell-missing': !record.title }">{{ record.title || '(空)' }}</div>
              </div>
            </template>
            <template v-else-if="column.key === 'site_name'">
              <a-tag color="blue">{{ record.site_name }}</a-tag>
              <CheckCircleFilled v-if="record.reviewed" style="color: #52c41a; margin-left: 4px" />
            </template>
            <template v-else-if="column.key === 'standard_type'">
              <span v-if="record.standard_type">{{ record.standard_type }}</span>
              <span v-else class="cell-missing">未设置</span>
            </template>
            <template v-else-if="column.key === 'tags'">
              <template v-if="parseTags(record.tags).length">
                <a-tag
                  v-for="t in parseTags(record.tags)"
                  :key="t"
                  :color="isForbiddenTag(t) ? 'red' : 'blue'"
                  style="margin: 1px; font-size: 11px"
                >
                  {{ t }}
                </a-tag>
              </template>
              <span v-else class="cell-missing">无</span>
            </template>
            <template v-else-if="column.key === 'completeness'">
              <a-progress
                type="circle"
                :size="40"
                :percent="completenessPercent(record)"
                :stroke-color="completenessColor(record)"
              />
            </template>
            <template v-else-if="column.key === 'flags'">
              <a-tag v-if="record.flags" color="red">{{ record.flags }}</a-tag>
            </template>
            <template v-else-if="column.key === 'updated_at'">
              {{ formatTime(record.updated_at) }}
            </template>
            <template v-else-if="column.key === 'action'">
              <a-space size="small">
                <a-button size="small" type="link" @click="openReview(record)">核对</a-button>
                <a-button size="small" type="link" @click="openMaintenance(record)">维护</a-button>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>

    <CrossSeedPanel
      v-model:open="panelOpen"
      :preset-torrent="panelPreset"
            @success="fetchData"
    />

    <BatchFetchPanel
      v-model:open="batchFetchOpen"
      @completed="fetchData"
    />

    <MetadataReviewModal
      v-model:open="reviewOpen"
      :info-hash="reviewHash"
      :torrent-name="reviewName"
      @saved="fetchData"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ReloadOutlined, CheckCircleFilled, PlusOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { publishDataApi, seedConfigApi, type SeedListItem } from '@/api/publish'
import { sitesApi } from '@/api/sites'
import type { Site } from '@/api/types'
import CrossSeedPanel from './CrossSeedPanel.vue'
import BatchFetchPanel from './BatchFetchPanel.vue'
import MetadataReviewModal from './MetadataReviewModal.vue'
import { formatBytes, formatTime } from '@/utils/format'
import { CATEGORY_LABELS } from '@/generated/dict'
import { categoryTagColor } from '@/utils/categoryDisplay'

const route = useRoute()
const router = useRouter()

const activeTab = ref('inject')

// ==================== Tab 1: 灌入发布（§59.133 ③ 站中心灌入） ====================

const targetSites = ref<Site[]>([])
const targetSitesLoading = ref(false)
const selectedTarget = ref<string | undefined>(undefined)
const injectSearch = ref('')
const existFilter = ref<'all' | 'new'>('all')
const injectRows = ref<SeedListItem[]>([])
const injectTotal = ref(0)
const injectLoading = ref(false)
const injectPage = ref(1)
const injectPageSize = ref(50)
const selectedInjectHashes = ref<string[]>([])

const injectColumns = [
  { title: '簇名称', dataIndex: 'name', key: 'name', ellipsis: true, sorter: (a: SeedListItem, b: SeedListItem) => a.name.localeCompare(b.name) },
  { title: '类型', key: 'category', width: 80, sorter: (a: SeedListItem, b: SeedListItem) => (a.category || '').localeCompare(b.category || '') },
  { title: '大小', key: 'size', width: 90, sorter: (a: SeedListItem, b: SeedListItem) => a.size - b.size },
  { title: '副本', key: 'copies', width: 90, align: 'center' as const, sorter: (a: SeedListItem, b: SeedListItem) => (a.copy_count ?? 1) - (b.copy_count ?? 1) },
  { title: '已有站点', key: 'sites', width: 220 },
  { title: '目标站状态', key: 'exist', width: 110, align: 'center' as const },
  { title: '操作', key: 'actions', width: 100 },
]

function filterSiteOption(input: string, option: { label?: string; value?: string }): boolean {
  const label = option?.label || option?.value || ''
  return String(label).toLowerCase().includes(input.toLowerCase())
}

function existsOnTarget(record: SeedListItem): boolean {
  if (!selectedTarget.value) return false
  return (record.sites || []).includes(selectedTarget.value)
}

const publishableCount = computed(() => {
  if (!selectedTarget.value) return 0
  return filteredInjectRows.value.filter(r => !existsOnTarget(r)).length
})

// "未存在"视图：本地过滤当前页（服务端无 exists 口径，total 保持服务端值）
const filteredInjectRows = computed(() => {
  if (existFilter.value !== 'new' || !selectedTarget.value) return injectRows.value
  return injectRows.value.filter(r => !existsOnTarget(r))
})

async function fetchTargetSites() {
  targetSitesLoading.value = true
  try {
    const resp = await sitesApi.list(1, 300, '', { is_target: 'true' })
    const data = resp.data?.data
    targetSites.value = ((data?.items || data || []) as Site[]).filter(s => s.enabled)
  } catch { /* ignore */ } finally {
    targetSitesLoading.value = false
  }
}

async function fetchInjectList() {
  injectLoading.value = true
  try {
    const resp = await seedConfigApi.listSeeds({
      ready: 'true',
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
    // 跨页保留勾选（批量灌入跨页累积）；目标站变化时已存在语义变，onInjectFilterChange 显式清
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

// 切回灌入 Tab 时刷新 ready 簇（维护保存后 reviewed 状态可能变化）
watch(activeTab, (tab) => {
  if (tab === 'inject') fetchInjectList()
})

// ==================== Tab 2: 数据管理（原功能保留） ====================

interface SeedDataRow {
  id: number
  info_hash: string
  site_name: string
  torrent_id: string
  title: string
  subtitle: string
  standard_type: string
  tags: string
  flags: string
  reviewed: boolean
  poster: string
  mediainfo: string
  description: string
  screenshots: string
  updated_at: string
}

const STORAGE_KEY = 'publish_data_filters'

interface PersistedFilters {
  search?: string
  source_site?: string
  review_status?: 'all' | 'reviewed' | 'unreviewed'
  page_size?: number
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
    search: searchQuery.value || undefined,
    source_site: sourceSiteFilter.value,
    review_status: reviewStatus.value,
    page_size: pagination.value.pageSize,
  }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
  } catch { /* silent */ }
}

const persisted = loadPersistedFilters()

const loading = ref(false)
const tableData = ref<SeedDataRow[]>([])
const searchQuery = ref(persisted.search || '')
const sourceSiteFilter = ref<string | undefined>(persisted.source_site)
const sourceSites = ref<string[]>([])
const reviewStatus = ref<'all' | 'reviewed' | 'unreviewed'>(persisted.review_status || 'all')

watch([searchQuery, sourceSiteFilter, reviewStatus], persistFilters)

const forbiddenTagKeywords = ['禁转', '独占', '谢绝转载', '限时禁转', '严禁转载', '禁止转载', '谢绝搬运']

function isForbiddenTag(tag: string): boolean {
  return forbiddenTagKeywords.some(kw => tag.includes(kw))
}

function onFilterChange() {
  pagination.value.current = 1
  fetchData()
}

const pagination = ref({
  current: 1,
  pageSize: persisted.page_size || 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`,
})

const columns = [
  { title: '站点', key: 'site_name', width: 120 },
  { title: '种子ID', dataIndex: 'torrent_id', key: 'torrent_id', width: 80 },
  { title: '标题', key: 'title', ellipsis: true },
  { title: '类型', key: 'standard_type', width: 100 },
  { title: '标签', key: 'tags', width: 180 },
  { title: '完整度', key: 'completeness', width: 100, align: 'center' as const },
  { title: '标记', key: 'flags', width: 80 },
  { title: '更新时间', key: 'updated_at', width: 150 },
  { title: '操作', key: 'action', width: 80, fixed: 'right' as const },
]

const panelOpen = ref(false)
const batchFetchOpen = ref(false)
const reviewOpen = ref(false)
const reviewHash = ref('')
const reviewName = ref('')
const selectedIds = ref<number[]>([])
const panelPreset = ref<{ info_hash: string; name: string; size: number; save_path: string; client_id: number; source_site?: string; source_site_id?: number } | null>(null)

let fetchSeq = 0

async function fetchData() {
  const seq = ++fetchSeq
  loading.value = true
  try {
    const resp = await publishDataApi.listSeedData({
      page: pagination.value.current,
      page_size: pagination.value.pageSize,
      search: searchQuery.value || undefined,
      source_site: sourceSiteFilter.value,
      review_status: reviewStatus.value === 'all' ? undefined : reviewStatus.value,
    })
    if (seq !== fetchSeq) return
    const data = resp.data?.data
    if (data) {
      tableData.value = data.items as SeedDataRow[]
      pagination.value.total = data.total
      const sites = new Set<string>()
      for (const item of tableData.value) {
        if (item.site_name) sites.add(item.site_name)
      }
      sourceSites.value = [...sites].sort()
    }
  } catch {
    // silent
  } finally {
    loading.value = false
  }
}

function onTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) pagination.value.current = pag.current
  if (pag.pageSize) pagination.value.pageSize = pag.pageSize
  persistFilters()
  fetchData()
}

async function batchReview(reviewed: boolean) {
  try {
    const resp = await publishDataApi.batchReview([...selectedIds.value], reviewed)
    message.success(`已${reviewed ? '审核' : '取消审核'} ${resp.data?.data?.updated || 0} 条`)
    selectedIds.value = []
    fetchData()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function batchDelete() {
  try {
    const resp = await publishDataApi.batchDelete([...selectedIds.value])
    message.success(`已删除 ${resp.data?.data?.deleted || 0} 条`)
    selectedIds.value = []
    fetchData()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

function parseTags(tagsStr: string): string[] {
  if (!tagsStr) return []
  try {
    const parsed = JSON.parse(tagsStr)
    if (Array.isArray(parsed)) return parsed
  } catch { /* not json */ }
  return tagsStr.split(',').map(t => t.trim()).filter(Boolean)
}

function completenessPercent(record: SeedDataRow): number {
  const fields = ['title', 'subtitle', 'description', 'mediainfo', 'poster', 'screenshots']
  const filled = fields.filter(f => {
    const val = (record as unknown as Record<string, unknown>)[f]
    return val && String(val).trim() && String(val).trim() !== '[]'
  }).length
  return Math.round((filled / fields.length) * 100)
}

function completenessColor(record: SeedDataRow): string {
  const pct = completenessPercent(record)
  if (pct >= 83) return '#52c41a'
  if (pct >= 50) return '#faad14'
  return '#cf1322'
}

function openMaintenance(record: SeedDataRow) {
  panelPreset.value = {
    info_hash: record.info_hash,
    name: record.title || record.subtitle || '',
    size: 0,
    save_path: '',
    client_id: 0,
    source_site: record.site_name,
    source_site_id: parseInt(record.torrent_id) || 0,
  }
  panelOpen.value = true
}

function openReview(record: SeedDataRow) {
  reviewHash.value = record.info_hash
  reviewName.value = record.title || record.subtitle || ''
  reviewOpen.value = true
}

async function tryOpenFromDeepLink() {
  const infoHash = route.query.info_hash as string
  const siteName = route.query.site_name as string
  if (!infoHash) return
  activeTab.value = 'manage'
  try {
    const resp = await publishDataApi.listSeedData({ search: infoHash, page_size: 100 })
    const items = resp.data?.data?.items as SeedDataRow[] | undefined
    const match = items?.find(r => r.info_hash === infoHash && (!siteName || r.site_name === siteName))
    if (match) {
      openMaintenance(match)
    }
  } catch { /* silent */ }
}

function onPanelClose() {
  if (route.query.info_hash || route.query.site_name) {
    router.replace({ query: {} })
  }
}

watch(panelOpen, (now, prev) => {
  if (prev && !now) onPanelClose()
})

onMounted(() => {
  fetchTargetSites()
  fetchInjectList()
  fetchData()
  tryOpenFromDeepLink()
})
</script>

<style scoped>
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
