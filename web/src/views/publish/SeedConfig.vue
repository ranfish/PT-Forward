<template>
  <div style="padding: 24px">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <a-input-search
          v-model:value="searchText"
          placeholder="搜索标题/种子名..."
          style="width: 260px"
          allow-clear
          @search="onFilterChange"
        />
        <a-radio-group v-model:value="statusFilter" button-style="solid" size="small" @change="onFilterChange">
          <a-radio-button value="all">全部</a-radio-button>
          <a-radio-button value="reviewed">已审核</a-radio-button>
          <a-radio-button value="pending">待审核</a-radio-button>
          <a-radio-button value="incomplete">不完整</a-radio-button>
          <a-radio-button value="unfetched">未获取</a-radio-button>
          <a-radio-button value="issues">禁转/无映射</a-radio-button>
        </a-radio-group>

        <!-- 已激活的筛选 tag -->
        <a-tag v-if="hasActiveFilters" closable color="blue" @close.prevent="clearFilters">
          {{ activeFilterText }}
        </a-tag>

        <a-tag v-if="total > 0">{{ total }} 条</a-tag>
      </div>
      <div class="toolbar-right">
        <a-button :loading="loading" @click="fetchList">
          <ReloadOutlined /> 刷新
        </a-button>
        <a-popconfirm
          :title="`确定清除 ${selectedHashes.length} 个种子的已获取数据？`"
          ok-text="清除"
          cancel-text="取消"
          @confirm="batchClear"
        >
          <a-button danger :loading="batchClearing" :disabled="selectedHashes.length === 0">
            <ClearOutlined /> 批量清除{{ selectedHashes.length > 0 ? `（${selectedHashes.length}）` : '' }}
          </a-button>
        </a-popconfirm>
        <a-button @click="filterVisible = true">
          <FilterOutlined /> 筛选
        </a-button>
        <a-button
          v-if="filterClient && filterPath"
          type="primary"
          :loading="batchFetchActive"
          @click="showBatchFetch = true"
        >
          <PlusOutlined /> 获取数据
        </a-button>
      </div>
    </div>

    <!-- Table -->
    <a-table
      :columns="columns"
      :data-source="tableData"
      :loading="loading"
      :pagination="{
        current: currentPage,
        pageSize: pageSize,
        total: total,
        showSizeChanger: true,
        pageSizeOptions: ['20', '50', '100'],
        showTotal: (total: number) => `共 ${total} 条`,
        size: 'small',
      }"
      row-key="hash"
      size="small"
      :scroll="{ x: 1400 }"
      :sticky="{ offsetHeader: 48 }"
      :row-class-name="(record: SeedListItem) => statusRowClass(record.status)"
      :row-selection="{
        selectedRowKeys: selectedHashes,
        onChange: onSelectChange,
        getCheckboxProps: (record: SeedListItem) => ({ disabled: record.status === 'unfetched' }),
      }"
      @change="onTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'title'">
          <div class="title-cell">
            <div class="title-main" :class="{ 'text-missing': !record.title }">
              {{ record.title || record.name || '(未获取)' }}
            </div>
            <div v-if="record.subtitle" class="title-sub">{{ record.subtitle }}</div>
          </div>
        </template>

        <template v-else-if="column.key === 'client'">
          <div class="client-cell">
            <div>{{ record.client_id }}</div>
            <div class="client-path" :title="record.save_path">{{ shortPath(record.save_path) }}</div>
          </div>
        </template>

        <template v-else-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)" size="small">
            {{ statusLabel(record.status) }}
          </a-tag>
        </template>

        <template v-else-if="column.key === 'size'">
          {{ formatSize(record.size) }}
        </template>

        <template v-else-if="column.key === 'tech'">
          <div class="tech-cell">
            <span v-if="record.resolution" class="tech-badge tech-res">{{ record.resolution }}</span>
            <span v-if="record.video_codec" class="tech-badge tech-codec">{{ record.video_codec }}</span>
            <span v-if="record.audio_codec" class="tech-badge tech-audio">{{ record.audio_codec }}</span>
            <span v-if="record.hdr" class="tech-badge tech-hdr">{{ record.hdr }}</span>
            <span v-if="!record.resolution && !record.video_codec && !record.audio_codec" class="text-missing">—</span>
          </div>
        </template>

        <template v-else-if="column.key === 'fields'">
          <div class="fields-cell">
            <span :class="record.title ? 'ck' : 'ck-miss'">标题</span>
            <span :class="record.poster ? 'ck' : 'ck-miss'">海报</span>
            <span :class="record.has_screenshots ? 'ck' : 'ck-miss'">截图</span>
            <span :class="record.has_description ? 'ck' : 'ck-miss'">简介</span>
            <span :class="record.has_mediainfo ? 'ck' : 'ck-miss'">MI</span>
          </div>
        </template>

        <template v-else-if="column.key === 'action'">
          <a-space :size="0">
            <a-button
              size="small"
              type="link"
              :loading="fetchingSet.has(record.hash)"
              @click="fetchSingle(record)"
            >
              {{ record.status === 'unfetched' ? '获取' : '重获' }}
            </a-button>
            <a-button
              v-if="record.status !== 'unfetched'"
              size="small"
              type="link"
              danger
              :loading="clearingSet.has(record.hash)"
              @click="clearSingle(record)"
            >
              清除
            </a-button>
            <a-button
              v-if="canEdit(record.status)"
              size="small"
              type="link"
              @click="openEdit(record)"
            >
              编辑
            </a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <!-- Filter Modal（二级筛选页） -->
    <a-modal
      v-model:open="filterVisible"
      title="筛选选项"
      ok-text="确认"
      cancel-text="取消"
      :width="520"
      @ok="applyFilters"
    >
      <a-form layout="vertical">
        <a-form-item label="下载器">
          <a-select
            v-model:value="tempClient"
            placeholder="全部下载器"
            allow-clear
            style="width: 100%"
            :options="clientSelectOptions"
            @change="tempPath = undefined"
          />
        </a-form-item>
        <a-form-item label="保存路径">
          <a-select
            v-model:value="tempPath"
            placeholder="全部路径"
            allow-clear
            style="width: 100%"
            :disabled="!tempClient"
            :options="pathSelectOptions"
          />
          <div v-if="!tempClient" style="color: #999; font-size: 12px">选择下载器后可筛选具体路径</div>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- Edit Panel -->
    <CrossSeedPanel
      v-model:open="editOpen"
      :preset-torrent="editPreset"
      maintenance-only
      @success="onEditSuccess"
    />

    <!-- Batch Fetch Panel -->
    <BatchFetchPanel
      v-if="filterClient && filterPath"
      v-model:open="showBatchFetch"
      :client-id="filterClient"
      :save-path="filterPath"
      @completed="onBatchFetchCompleted"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { PlusOutlined, ReloadOutlined, FilterOutlined, ClearOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import CrossSeedPanel from './CrossSeedPanel.vue'
import BatchFetchPanel from './BatchFetchPanel.vue'
import { seedConfigApi, type SeedListItem } from '@/api/publish'

// ==================== 筛选状态（§59.29：挂载即查，筛选弹层选路径） ====================

interface ClientPathEntry {
  client_id: string
  paths: Array<{ save_path: string; count: number }>
}

const clientPathTree = ref<ClientPathEntry[]>([])

// 生效的筛选（传给 API）
const filterClient = ref<string | undefined>(undefined)
const filterPath = ref<string | undefined>(undefined)
const statusFilter = ref('all')
const searchText = ref('')

// 弹层临时状态
const filterVisible = ref(false)
const tempClient = ref<string | undefined>(undefined)
const tempPath = ref<string | undefined>(undefined)

// 持久化 key
const LS_KEY = 'seed-config-filters'

function loadPersistedFilters() {
  try {
    const raw = localStorage.getItem(LS_KEY)
    if (!raw) return
    const saved = JSON.parse(raw)
    filterClient.value = saved.client || undefined
    filterPath.value = saved.path || undefined
    statusFilter.value = saved.status || 'all'
    tempClient.value = filterClient.value
    tempPath.value = filterPath.value
  } catch { /* silent */ }
}

function persistFilters() {
  try {
    localStorage.setItem(LS_KEY, JSON.stringify({
      client: filterClient.value || '',
      path: filterPath.value || '',
      status: statusFilter.value,
    }))
  } catch { /* silent */ }
}

const clientSelectOptions = computed(() =>
  clientPathTree.value.map(c => ({ value: c.client_id, label: c.client_id }))
)

const pathSelectOptions = computed(() => {
  const entry = clientPathTree.value.find(c => c.client_id === tempClient.value)
  return (entry?.paths || []).map(p => ({ value: p.save_path, label: `${p.save_path} (${p.count})` }))
})

const hasActiveFilters = computed(() => !!filterClient.value || !!filterPath.value)

const activeFilterText = computed(() => {
  if (!filterClient.value) return ''
  if (!filterPath.value) return filterClient.value
  return `${filterClient.value}:${shortPath(filterPath.value)}`
})

function shortPath(p: string): string {
  const parts = p.split('/').filter(Boolean)
  return parts.length > 2 ? '.../' + parts.slice(-2).join('/') : p
}

async function fetchClientPaths() {
  try {
    const resp = await seedConfigApi.uniquePaths()
    clientPathTree.value = resp.data?.data?.clients || []
  } catch {
    clientPathTree.value = []
  }
}

function applyFilters() {
  filterClient.value = tempClient.value || undefined
  filterPath.value = tempPath.value || undefined
  // 切筛选重置到第一页
  currentPage.value = 1
  persistFilters()
  filterVisible.value = false
  fetchList()
}

function clearFilters() {
  filterClient.value = undefined
  filterPath.value = undefined
  tempClient.value = undefined
  tempPath.value = undefined
  currentPage.value = 1
  persistFilters()
  fetchList()
}

function onFilterChange() {
  currentPage.value = 1
  persistFilters()
  fetchList()
}

// ==================== 列表数据（§59.29：状态/搜索后端过滤） ====================

const loading = ref(false)
const tableData = ref<SeedListItem[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(50)
const batchFetchActive = ref(false)

async function fetchList() {
  loading.value = true
  try {
    const resp = await seedConfigApi.listSeeds({
      client_id: filterClient.value || '',
      save_path: filterPath.value || '',
      status: statusFilter.value === 'all' ? '' : (statusFilter.value === 'issues' ? 'issues' : statusFilter.value),
      search: searchText.value,
      page: currentPage.value,
      page_size: pageSize.value,
    })
    tableData.value = resp.data?.data?.items || []
    total.value = resp.data?.data?.total || 0
  } catch {
    message.error('加载列表失败')
    tableData.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) currentPage.value = pag.current
  if (pag.pageSize) pageSize.value = pag.pageSize
  fetchList()
}

// ==================== 5 态标签 ====================

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    forbidden: '禁转',
    system_forbidden: '系统禁转',
    no_mapping: '无源站映射',
    reviewed: '已审核',
    pending: '待审核',
    incomplete: '不完整',
    unfetched: '未获取',
  }
  return map[status] || status
}

function statusColor(status: string): string {
  const map: Record<string, string> = {
    forbidden: 'red',
    system_forbidden: 'orange',
    no_mapping: 'default',
    reviewed: 'success',
    pending: 'warning',
    incomplete: 'default',
    unfetched: 'default',
  }
  return map[status] || 'default'
}

function statusRowClass(status: string): string {
  if (status === 'forbidden' || status === 'system_forbidden') return 'row-forbidden'
  if (status === 'no_mapping') return 'row-disabled'
  return ''
}

function canEdit(status: string): boolean {
  // §59.28: unfetched 无 metadata，编辑必 404，不提供编辑入口
  return status === 'reviewed' || status === 'pending' || status === 'incomplete'
}

// ==================== 工具函数 ====================

function formatSize(bytes: number): string {
  if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + ' GiB'
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(0) + ' MiB'
  return bytes + ' B'
}

const columns = [
  { title: '标题', key: 'title', ellipsis: true, width: 280 },
  { title: '下载器/路径', key: 'client', width: 140 },
  { title: '状态', key: 'status', width: 90 },
  { title: '大小', key: 'size', width: 80 },
  { title: '技术参数', key: 'tech', width: 200 },
  { title: '字段', key: 'fields', width: 150 },
  { title: '操作', key: 'action', width: 200, fixed: 'right' as const },
]

// ==================== 编辑 ====================

const editOpen = ref(false)
const editPreset = ref<any>(null)

function openEdit(item: SeedListItem) {
  if (!canEdit(item.status)) {
    message.warning('该种子不可编辑：' + statusLabel(item.status))
    return
  }
  editPreset.value = {
    info_hash: item.hash,
    hash: item.hash,
    name: item.name,
    size: item.size,
    save_path: item.save_path,
    client_id: item.client_id,
    source_site: item.site_name,
  }
  editOpen.value = true
}

function onEditSuccess() {
  fetchList()
}

// ==================== 单种子获取/清除 ====================

const fetchingSet = ref<Set<string>>(new Set())
const clearingSet = ref<Set<string>>(new Set())

async function fetchSingle(item: SeedListItem) {
  fetchingSet.value.add(item.hash)
  try {
    await seedConfigApi.fetchSingleSeed(item.hash, item.client_id)
    message.success(item.status === 'unfetched' ? '获取成功' : '已重新获取，请重新审核')
    await fetchList()
  } catch (e: any) {
    message.error('获取失败：' + (e?.response?.data?.message || e?.message || '未知错误'))
  } finally {
    fetchingSet.value.delete(item.hash)
  }
}

const selectedHashes = ref<string[]>([])
const batchClearing = ref(false)

function onSelectChange(keys: string[]) {
  selectedHashes.value = keys
}

async function batchClear() {
  if (selectedHashes.value.length === 0) return
  batchClearing.value = true
  let ok = 0
  let fail = 0
  try {
    for (const h of selectedHashes.value) {
      try {
        await seedConfigApi.deleteSeed(h)
        ok++
      } catch {
        fail++
      }
    }
    if (fail === 0) message.success(`已清除 ${ok} 个种子`)
    else message.warning(`清除完成：成功 ${ok}，失败 ${fail}`)
    selectedHashes.value = []
    await fetchList()
  } finally {
    batchClearing.value = false
  }
}

async function clearSingle(item: SeedListItem) {
  clearingSet.value.add(item.hash)
  try {
    await seedConfigApi.deleteSeed(item.hash)
    message.success('已清除')
    await fetchList()
  } catch (e: any) {
    message.error('清除失败：' + (e?.response?.data?.message || e?.message || '未知错误'))
  } finally {
    clearingSet.value.delete(item.hash)
  }
}

// ==================== 批量获取 ====================

const showBatchFetch = ref(false)

function onBatchFetchCompleted() {
  showBatchFetch.value = false
  fetchList()
}

// ==================== 初始化 ====================

onMounted(() => {
  loadPersistedFilters()
  fetchClientPaths()
  fetchList()
})

// 搜索防抖（输入停顿后自动查）
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(searchText, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => onFilterChange(), 400)
})
</script>

<style scoped>
.toolbar {
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.title-cell .title-main {
  font-weight: 500;
}
.title-cell .title-sub {
  font-size: 12px;
  color: #999;
}
.client-cell .client-path {
  font-size: 11px;
  color: #999;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 130px;
}
.tech-cell {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.tech-badge {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  white-space: nowrap;
}
.tech-res { background: #e6f7ff; color: #1890ff; }
.tech-codec { background: #f6ffed; color: #52c41a; }
.tech-audio { background: #fff7e6; color: #faad14; }
.tech-hdr { background: #fff0f6; color: #eb2f96; }
.text-missing { color: #ccc; font-style: italic; }
.fields-cell {
  display: flex;
  gap: 3px;
  font-size: 10px;
}
.fields-cell .ck { color: #52c41a; }
.fields-cell .ck::before { content: '✓ '; }
.fields-cell .ck-miss { color: #ff4d4f; }
.fields-cell .ck-miss::before { content: '✗ '; }
:deep(.row-forbidden) {
  background: #fff2f0;
}
:deep(.row-disabled) {
  opacity: 0.5;
}
</style>
