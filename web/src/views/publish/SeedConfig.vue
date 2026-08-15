<template>
  <div style="padding: 24px">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <a-select
          v-model:value="selectedClient"
          style="width: 180px"
          placeholder="选择下载器"
          @change="onClientChange"
        >
          <a-select-option v-for="c in clientOptions" :key="c.clientId" :value="c.clientId">
            {{ c.clientName || c.clientId }}{{ c.isLocal === false ? '（远程）' : c.isLocal === true ? '（本地）' : '' }}
          </a-select-option>
        </a-select>
        <a-select
          v-if="selectedClient && pathOptions.length > 0"
          v-model:value="selectedPath"
          style="width: 280px"
          placeholder="资源路径"
          allow-clear
          @change="fetchList"
        >
          <a-select-option v-for="p in pathOptions" :key="p.path" :value="p.path">
            {{ p.path }} ({{ p.count }})
          </a-select-option>
        </a-select>
        <a-input-search
          v-if="selectedClient && selectedPath"
          v-model:value="searchText"
          placeholder="搜索标题..."
          style="width: 240px"
          allow-clear
        />
        <a-radio-group v-if="selectedClient && selectedPath" v-model:value="statusFilter" button-style="solid" size="small">
          <a-radio-button value="all">全部</a-radio-button>
          <a-radio-button value="reviewed">已审核</a-radio-button>
          <a-radio-button value="pending">待审核</a-radio-button>
          <a-radio-button value="incomplete">不完整</a-radio-button>
          <a-radio-button value="issues">禁转/无映射</a-radio-button>
        </a-radio-group>
        <a-tag v-if="total > 0" color="blue">{{ total }} 条</a-tag>
      </div>
      <div class="toolbar-right">
        <a-button
          v-if="selectedClient && selectedPath"
          :loading="loading"
          @click="fetchList"
        >
          <ReloadOutlined /> 刷新
        </a-button>
        <a-button
          v-if="selectedClient && selectedPath"
          type="primary"
          :loading="batchFetchActive"
          @click="showBatchFetch = true"
        >
          <PlusOutlined /> 获取数据
        </a-button>
      </div>
    </div>

    <!-- Empty state when no client/path selected -->
    <a-empty
      v-if="!selectedClient || !selectedPath"
      description="请选择下载器和路径查看种子列表"
      style="margin-top: 80px"
    />

    <!-- Table -->
    <a-table
      v-else
      :columns="columns"
      :data-source="filteredData"
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

    <!-- Edit Panel -->
    <CrossSeedPanel
      v-model:open="editOpen"
      :preset-torrent="editPreset"
      maintenance-only
      @success="onEditSuccess"
    />

    <!-- Batch Fetch Panel -->
    <BatchFetchPanel
      v-if="selectedClient && selectedPath"
      v-model:open="showBatchFetch"
      :client-id="selectedClient"
      :save-path="selectedPath"
      @completed="onBatchFetchCompleted"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import CrossSeedPanel from './CrossSeedPanel.vue'
import BatchFetchPanel from './BatchFetchPanel.vue'
import { seedConfigApi, type SeedListItem } from '@/api/publish'
import client from '@/api/client'
import type { ApiResponse } from '@/api/types'

// ==================== 下载器/路径选择 ====================

interface ClientPathOption {
  clientId: string
  clientName?: string
  isLocal?: boolean
}
interface PathEntry {
  path: string
  count: number
}

const clientOptions = ref<ClientPathOption[]>([])
const pathOptions = ref<PathEntry[]>([])
const selectedClient = ref<string | undefined>(undefined)
const selectedPath = ref<string | undefined>(undefined)

async function fetchSnapshotPaths() {
  try {
    const resp = await client.get<ApiResponse<Array<{ clientId: string; paths: PathEntry[] }>>>('/downloads/snapshot-paths')
    const data = resp.data?.data || []
    // 查下载器 isLocal 标志
    let localMap: Record<string, boolean> = {}
    try {
      const dlResp = await client.get<ApiResponse<{ items: Array<{ name: string; isLocal: boolean | null }> }>>('/downloaders?pageSize=200')
      for (const d of (dlResp.data?.data?.items || [])) {
        localMap[d.name] = d.isLocal ?? false
      }
    } catch { /* silent */ }
    clientOptions.value = data.map(d => ({
      clientId: d.clientId,
      isLocal: localMap[d.clientId],
    }))
    // 展开所有路径，切换 client 时过滤
    const found = data.find(d => d.clientId === selectedClient.value)
    pathOptions.value = found ? found.paths : []
  } catch {
    clientOptions.value = []
  }
}

function onClientChange() {
  selectedPath.value = undefined
  // 更新路径列表
  fetchSnapshotPaths()
  // 如果只有一个路径自动选中
  if (pathOptions.value.length === 1) {
    selectedPath.value = pathOptions.value[0].path
    fetchList()
  }
}

// ==================== 列表数据 ====================

const loading = ref(false)
const tableData = ref<SeedListItem[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(50)
const searchText = ref('')
const statusFilter = ref('all')
const batchFetchActive = ref(false)

async function fetchList() {
  if (!selectedClient.value || !selectedPath.value) return
  loading.value = true
  try {
    const resp = await seedConfigApi.listSeeds({
      client_id: selectedClient.value,
      save_path: selectedPath.value,
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

const filteredData = computed(() => {
  let result = tableData.value
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    result = result.filter(item =>
      (item.title || '').toLowerCase().includes(q) ||
      (item.name || '').toLowerCase().includes(q)
    )
  }
  if (statusFilter.value === 'reviewed') {
    result = result.filter(i => i.status === 'reviewed')
  } else if (statusFilter.value === 'pending') {
    result = result.filter(i => i.status === 'pending')
  } else if (statusFilter.value === 'incomplete') {
    result = result.filter(i => i.status === 'incomplete')
  } else if (statusFilter.value === 'issues') {
    result = result.filter(i => i.status === 'forbidden' || i.status === 'system_forbidden' || i.status === 'no_mapping')
  }
  return result
})

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
  fetchSnapshotPaths()
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
