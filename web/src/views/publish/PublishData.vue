<template>
  <div style="padding: 24px">
    <div style="margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center">
      <h3 style="margin: 0">一站多种</h3>
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
            <a-button size="small" type="link" @click="openMaintenance(record)">维护</a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <CrossSeedPanel
      v-model:open="panelOpen"
      :preset-torrent="panelPreset"
      @success="fetchData"
    />

    <BatchFetchPanel
      v-model:open="batchFetchOpen"
      @done="fetchData"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ReloadOutlined, CheckCircleFilled, PlusOutlined } from '@ant-design/icons-vue'
import { publishDataApi } from '@/api/publish'
import CrossSeedPanel from './CrossSeedPanel.vue'
import BatchFetchPanel from './BatchFetchPanel.vue'
import { formatTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()

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

async function tryOpenFromDeepLink() {
  const infoHash = route.query.info_hash as string
  const siteName = route.query.site_name as string
  if (!infoHash) return
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

onMounted(async () => {
  await fetchData()
  tryOpenFromDeepLink()
})
</script>

<style scoped>
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
</style>
