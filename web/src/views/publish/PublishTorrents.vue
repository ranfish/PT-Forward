<template>
  <div>
    <div class="page-toolbar">
      <a-select
        v-model:value="selectedClient"
        style="width: 180px"
        :loading="clientsLoading"
        placeholder="下载器（全部）"
        allow-clear
        @change="onFilterChange"
      >
        <a-select-option v-for="c in clients" :key="c.client_id" :value="c.client_id">
          {{ c.client_id }} ({{ totalPathsOf(c) }})
        </a-select-option>
      </a-select>
      <a-select
        v-model:value="selectedPath"
        style="width: 220px; margin-left: 12px"
        placeholder="保存路径（全部）"
        allow-clear
        :disabled="!selectedClient"
        @change="onFilterChange"
      >
        <a-select-option v-for="p in pathOptions" :key="p.save_path" :value="p.save_path">
          {{ p.save_path }} ({{ p.count }})
        </a-select-option>
      </a-select>
      <a-radio-group
        v-model:value="readyFilter"
        button-style="solid"
        size="small"
        style="margin-left: 12px"
        @change="onFilterChange"
      >
        <a-radio-button value="all">全部</a-radio-button>
        <a-radio-button value="ready">可发布</a-radio-button>
        <a-radio-button value="pending">待完善</a-radio-button>
      </a-radio-group>
      <a-input-search
        v-model:value="searchText"
        placeholder="搜索簇名称..."
        style="width: 260px; margin-left: 12px"
        allow-clear
        @search="onFilterChange"
      />
      <a-tag v-if="tableData.length" color="blue" style="margin-left: 8px">
        共 {{ total }} 簇
      </a-tag>
      <a-button size="small" style="margin-left: auto" @click="groupMappingOpen = true">
        制作组映射
      </a-button>
      <a-button size="small" @click="openDeclFilters">
        过滤规则
      </a-button>
    </div>

    <a-alert
      type="info"
      show-icon
      style="margin-bottom: 12px"
      message="一种多站：以簇为中心——选择一个种子簇，向多个目标站发布。数据不完整的簇请先到种子配置页完善（发布页零拉取）。"
    />

    <a-table
      :columns="columns"
      :data-source="tableData"
      :loading="loading"
      :pagination="{
        current: currentPage,
        pageSize: pageSize,
        total: total,
        showSizeChanger: true,
        pageSizeOptions: ['50', '100', '200'],
        showTotal: (t: number) => `共 ${t} 簇`,
        size: 'small',
      }"
      row-key="hash"
      size="small"
      :scroll="{ x: 1100 }"
      :sticky="{ offsetHeader: 48 }"
      :row-class-name="(record: SeedListItem) => record.reviewed ? 'cluster-row-ready' : 'cluster-row-pending'"
      @change="onTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'">
          <div class="cluster-name">{{ record.name }}</div>
          <div v-if="record.title && record.title !== record.name" class="cluster-title">{{ record.title }}</div>
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
        <template v-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)" style="margin: 0">{{ statusLabel(record.status) }}</a-tag>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-tooltip title="提交链路接线中（TagApplier 灰度，R3-5）">
              <a-button type="primary" size="small" disabled>选站发布</a-button>
            </a-tooltip>
            <a-button size="small" @click="goRefine(record)">完善数据</a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="declFilterOpen"
      title="声明过滤规则"
      width="600px"
      :footer="null"
      destroy-on-close
    >
      <a-alert
        type="info"
        show-icon
        message="过滤规则用于自动从简介中移除 ARDTU/CSAUTO 等工具声明"
        style="margin-bottom: 16px"
      />
      <div v-if="declFilterIsDefault" style="margin-bottom: 12px">
        <a-tag color="orange">当前使用默认规则</a-tag>
      </div>
      <a-list
        :data-source="declFilterPatterns"
        size="small"
        bordered
      >
        <template #renderItem="{ item, index }">
          <a-list-item>
            <div style="display: flex; justify-content: space-between; align-items: center; width: 100%">
              <span style="font-family: monospace; font-size: 13px">{{ item }}</span>
              <a-button type="link" danger size="small" @click="declFilterPatterns.splice(index, 1)">
                删除
              </a-button>
            </div>
          </a-list-item>
        </template>
      </a-list>
      <div style="margin-top: 12px; display: flex; gap: 8px">
        <a-input
          v-model:value="newDeclPattern"
          placeholder="输入过滤关键词（如 ARDTU工具自动发布）"
          @press-enter="addDeclPattern"
        />
        <a-button type="primary" size="small" @click="addDeclPattern">添加</a-button>
      </div>
      <div style="margin-top: 16px; text-align: right">
        <a-button style="margin-right: 8px" @click="declFilterOpen = false">取消</a-button>
        <a-button type="primary" :loading="declFilterSaving" @click="saveDeclFilters">保存</a-button>
      </div>
    </a-modal>

    <a-modal
      v-model:open="groupMappingOpen"
      title="制作组 → 源站映射"
      width="800px"
      :footer="null"
      destroy-on-close
    >
      <div style="margin-bottom: 12px; display: flex; gap: 8px">
        <a-input v-model:value="newMapping.group_name" placeholder="制作组名（如 CMCTV）" style="width: 160px" />
        <a-input v-model:value="newMapping.domain" placeholder="域名（如 springsunday.net）" style="width: 220px" />
        <a-input v-model:value="newMapping.site_name" placeholder="站点名（留空自动匹配）" style="width: 160px" />
        <a-button type="primary" size="small" @click="addMapping">添加</a-button>
      </div>
      <a-table
        :columns="mappingColumns"
        :data-source="mappings"
        :pagination="mappingPagination"
        row-key="id"
        size="small"
        @change="(pag: { current?: number; pageSize?: number }) => { if (pag.current) mappingPagination.current = pag.current; if (pag.pageSize) mappingPagination.pageSize = pag.pageSize }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'domain'">
            {{ maskDomain(record.domain) }}
          </template>
          <template v-if="column.key === 'matched_site'">
            <a-tag v-if="record.matchedSite" color="green">{{ record.matchedSite }}</a-tag>
            <a-tag v-else color="red">未匹配</a-tag>
          </template>
          <template v-if="column.key === 'actions'">
            <a-button type="link" size="small" @click="editMapping(record)">编辑</a-button>
            <a-popconfirm v-if="!record.is_builtin" title="确定删除？" @confirm="deleteMapping(record.id)">
              <a-button type="link" danger size="small">删除</a-button>
            </a-popconfirm>
            <a-tooltip v-else title="内置官组不可删除">
              <a-button type="link" danger size="small" disabled>删除</a-button>
            </a-tooltip>
          </template>
        </template>
      </a-table>
    </a-modal>

    <a-modal
      v-model:open="editMappingOpen"
      title="编辑映射"
      width="420px"
      @ok="saveMapping"
    >
      <a-form layout="vertical">
        <a-form-item label="制作组名">
          <a-input v-model:value="editingMapping.group_name" />
        </a-form-item>
        <a-form-item label="域名">
          <a-input v-model:value="editingMapping.domain" />
        </a-form-item>
        <a-form-item label="站点名（手动指定，留空用域名自动匹配）">
          <a-input v-model:value="editingMapping.site_name" placeholder="留空自动匹配" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { seedConfigApi, publishTorrentsApi, type SeedListItem } from '@/api/publish'
import { formatBytes, maskDomain } from '@/utils/format'

// §59.131 ②: 一种多站页——簇口径改造（R3-1 定案）。
// 数据源 = /publish/seeds 簇端点（零拉取）；行 = (client,path,name) 簇。
// 覆盖查询/转种向导/detect-source 现场调用全部移除（发布页纯消费层）。

const router = useRouter()

const clients = ref<Array<{ client_id: string; paths: Array<{ save_path: string; count: number }> }>>([])
const clientsLoading = ref(false)

const STORAGE_KEY = 'publish_clusters_filters'

interface PersistedFilters {
  client?: string
  path?: string
  ready?: string
  search?: string
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
    client: selectedClient.value || undefined,
    path: selectedPath.value || undefined,
    ready: readyFilter.value,
    search: searchText.value || undefined,
    page_size: pageSize.value,
  }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
  } catch { /* silent */ }
}

const persisted = loadPersistedFilters()

const selectedClient = ref<string | undefined>(persisted.client)
const selectedPath = ref<string | undefined>(persisted.path)
const readyFilter = ref(persisted.ready || 'all')
const searchText = ref(persisted.search || '')

const tableData = ref<SeedListItem[]>([])
const total = ref(0)
const loading = ref(false)
const currentPage = ref(1)
const pageSize = ref(persisted.page_size || 50)

watch([selectedClient, selectedPath, readyFilter, pageSize], persistFilters)

const pathOptions = computed(() => {
  const c = clients.value.find(c => c.client_id === selectedClient.value)
  return c?.paths || []
})

function totalPathsOf(c: { paths: Array<{ count: number }> }): number {
  return c.paths.reduce((sum, p) => sum + p.count, 0)
}

const columns = [
  { title: '簇名称', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '大小', key: 'size', width: 90 },
  { title: '副本', key: 'copies', width: 90, align: 'center' as const },
  { title: '已有站点', key: 'sites', width: 220 },
  { title: '数据状态', key: 'status', width: 110 },
  { title: '操作', key: 'actions', width: 200 },
]

const STATUS_META: Record<string, { label: string; color: string }> = {
  forbidden: { label: '禁转', color: 'red' },
  system_forbidden: { label: '系统禁转', color: 'red' },
  no_mapping: { label: '无源站映射', color: 'volcano' },
  reviewed: { label: '已审核', color: 'green' },
  pending: { label: '待审核', color: 'blue' },
  incomplete: { label: '配置不完整', color: 'orange' },
  unfetched: { label: '未获取', color: 'default' },
}

function statusLabel(s: string): string {
  return STATUS_META[s]?.label || s
}

function statusColor(s: string): string {
  return STATUS_META[s]?.color || 'default'
}

function onTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) currentPage.value = pag.current
  if (pag.pageSize) pageSize.value = pag.pageSize
  fetchList()
}

function onFilterChange() {
  currentPage.value = 1
  if (selectedClient.value === undefined) selectedPath.value = undefined
  persistFilters()
  fetchList()
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

async function fetchList() {
  loading.value = true
  try {
    const resp = await seedConfigApi.listSeeds({
      client_id: selectedClient.value || '',
      save_path: selectedPath.value || '',
      ready: readyFilter.value === 'all' ? '' : readyFilter.value,
      search: searchText.value,
      page: currentPage.value,
      page_size: pageSize.value,
    })
    tableData.value = ((resp.data?.data?.items || []) as SeedListItem[]).map((it) => ({
      ...it,
      hash: it.hash || `${it.client_id}|${it.name}`,
    }))
    total.value = resp.data?.data?.total || 0
  } catch {
    message.error('加载簇列表失败')
    tableData.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 引导跳回种子配置页（R3-1 deep-link：自动应用筛选+定位+打开编辑）
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

// 搜索防抖（输入停顿后自动查）
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(searchText, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => onFilterChange(), 400)
})

onMounted(() => {
  fetchClients()
  fetchList()
})

// --- 映射管理 ---
const groupMappingOpen = ref(false)
const mappings = ref<Array<Record<string, unknown> & { id: number }>>([])
const editMappingOpen = ref(false)
const editingMapping = ref({ id: 0, group_name: '', domain: '', site_name: '' })
const newMapping = reactive({ group_name: '', domain: '', site_name: '' })

const mappingColumns = [
  { title: '制作组', dataIndex: 'group_name', key: 'group_name', width: 120 },
  { title: '域名', dataIndex: 'domain', key: 'domain', ellipsis: true },
  { title: '匹配站点', key: 'matched_site', width: 120 },
  { title: '操作', key: 'actions', width: 120 },
]
const mappingPagination = reactive({
  current: 1,
  pageSize: 20,
  showSizeChanger: true,
  pageSizeOptions: ['20', '50', '100'],
  showTotal: (t: number) => `共 ${t} 条`,
  size: 'small' as const,
})

async function fetchMappings() {
  try {
    const resp = await publishTorrentsApi.listGroupMappings()
    mappings.value = (resp.data?.data?.items || []) as Array<Record<string, unknown> & { id: number }>
  } catch { /* ignore */ }
}

watch(groupMappingOpen, (val) => {
  if (val) fetchMappings()
})

async function addMapping() {
  if (!newMapping.group_name) return
  try {
    await publishTorrentsApi.createGroupMapping({
      groupName: newMapping.group_name,
      domain: newMapping.domain,
      siteName: newMapping.site_name,
    })
    newMapping.group_name = ''
    newMapping.domain = ''
    newMapping.site_name = ''
    fetchMappings()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

function editMapping(record: Record<string, unknown> & { id: number }) {
  editingMapping.value = {
    id: record.id,
    group_name: record.group_name as string,
    domain: record.domain as string,
    site_name: record.site_name as string || '',
  }
  editMappingOpen.value = true
}

async function saveMapping() {
  try {
    await publishTorrentsApi.updateGroupMapping(editingMapping.value.id, {
      groupName: editingMapping.value.group_name,
      domain: editingMapping.value.domain,
      siteName: editingMapping.value.site_name,
    })
    editMappingOpen.value = false
    fetchMappings()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function deleteMapping(id: number) {
  try {
    await publishTorrentsApi.deleteGroupMapping(id)
    fetchMappings()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

// --- 声明过滤规则管理 ---
const declFilterOpen = ref(false)
const declFilterPatterns = ref<string[]>([])
const declFilterIsDefault = ref(false)
const declFilterSaving = ref(false)
const newDeclPattern = ref('')

async function openDeclFilters() {
  try {
    const resp = await publishTorrentsApi.getDeclarationFilters()
    const data = resp.data?.data
    declFilterPatterns.value = data?.patterns || []
    declFilterIsDefault.value = data?.is_default ?? true
  } catch { /* ignore */ }
  newDeclPattern.value = ''
  declFilterOpen.value = true
}

function addDeclPattern() {
  const pattern = newDeclPattern.value.trim()
  if (pattern && !declFilterPatterns.value.includes(pattern)) {
    declFilterPatterns.value.push(pattern)
    newDeclPattern.value = ''
  }
}

async function saveDeclFilters() {
  declFilterSaving.value = true
  try {
    await publishTorrentsApi.setDeclarationFilters(declFilterPatterns.value)
    message.success('过滤规则已保存')
    declFilterOpen.value = false
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    declFilterSaving.value = false
  }
}
</script>

<style scoped>
.page-toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 4px;
  position: sticky;
  top: 0;
  z-index: 10;
  background: #fff;
  padding: 8px 0;
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

<style>
.cluster-row-ready {
  background-color: #f6ffed !important;
}
.cluster-row-pending {
  background-color: #fffbe6 !important;
}
</style>
