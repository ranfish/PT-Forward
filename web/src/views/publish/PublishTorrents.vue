<template>
  <div>
    <div class="page-toolbar">
      <a-select
        v-model:value="selectedClientId"
        style="width: 260px"
        :loading="clientsLoading"
        placeholder="选择下载器"
        @change="onClientChange"
      >
        <a-select-option v-for="c in clients" :key="c.id" :value="c.id">
          {{ c.name }} ({{ c.type }})
        </a-select-option>
      </a-select>
      <a-input-search
        v-if="torrents.length"
        v-model:value="searchText"
        placeholder="搜索种子名称..."
        style="width: 260px; margin-left: 12px"
        allow-clear
      />
      <a-select
        v-if="torrents.length"
        v-model:value="queryFilter"
        style="width: 130px; margin-left: 12px"
        placeholder="覆盖筛选"
        allow-clear
      >
        <a-select-option value="queried">已查询</a-select-option>
        <a-select-option value="unqueried">未查询</a-select-option>
      </a-select>
      <a-tag v-if="torrents.length" color="blue" style="margin-left: 8px">
        {{ filteredTorrents.length }} / {{ torrents.length }}
      </a-tag>
      <a-button size="small" style="margin-left: auto" @click="groupMappingOpen = true">
        制作组映射
      </a-button>
      <!-- 后台查询进度 -->
      <div v-if="querying" class="query-progress">
        <a-progress
          :percent="queryProgress"
          size="small"
          status="active"
          style="width: 200px"
        />
        <span class="progress-text">{{ queryDone }} / {{ queryTotal }}</span>
      </div>
    </div>

    <a-table
      :columns="columns"
      :data-source="pagedTorrents"
      :loading="loading"
      :pagination="{
        current: currentPage,
        pageSize: pageSize,
        total: filteredTorrents.length,
        showSizeChanger: true,
        pageSizeOptions: ['50', '100', '200'],
        showTotal: (total: number) => `共 ${total} 个种子`,
        size: 'small',
      }"
      row-key="info_hash"
      size="small"
      :sticky="{ offsetHeader: 48 }"
      @change="onTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'size'">
          {{ formatBytes(record.size) }}
        </template>
        <template v-if="column.key === 'coverage'">
          <a-tooltip>
            <template #title>
              <div v-if="record.coverage?.sites?.length">
                <div v-for="s in record.coverage.sites" :key="s.site_name">
                  <a-tag :color="coverageColor(s.status)" size="small" style="margin: 1px 0">
                    {{ s.site_name }}
                  </a-tag>
                  <span style="font-size: 11px; color: #999">({{ s.source }})</span>
                </div>
              </div>
              <div v-else-if="record.queried" style="color: #999">
                已查询，暂无已知覆盖
              </div>
              <div v-else style="color: #999">尚未查询</div>
            </template>
            <div class="coverage-cell">
              <span class="coverage-has">{{ record.coverage?.has_count ?? 0 }}</span>
              <span class="coverage-sep">/</span>
              <span class="coverage-total">{{ record.coverage?.total_sites ?? 0 }}</span>
              <a-tag v-if="!record.queried" color="orange" size="small" class="unqueried-tag">未查</a-tag>
            </div>
          </a-tooltip>
        </template>
        <template v-if="column.key === 'target_count'">
          <a-tag :color="(record.coverage?.target_count ?? 0) > 0 ? 'green' : 'default'">
            {{ record.coverage?.target_count ?? 0 }} 站可转
          </a-tag>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button
              type="link"
              size="small"
              :loading="queryingHash === record.info_hash"
              @click="queryCoverage(record)"
            >
              查询覆盖
            </a-button>
            <a-button
              type="primary"
              size="small"
              :disabled="(record.coverage?.target_count ?? 0) === 0"
              @click="startForward(record)"
            >
              转种
            </a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <PublishWizardModal
      v-model:open="wizardOpen"
      :preset-torrent="presetTorrent"
      :preset-client-id="selectedClientId"
      @success="onWizardSuccess"
    />

    <a-modal
      v-model:open="sourceSelectOpen"
      title="选择转种源站"
      width="520px"
      :footer="null"
    >
      <div v-if="sourceDetectRecord" style="margin-bottom: 16px">
        <span style="color: #666">种子：</span>
        <span>{{ sourceDetectRecord.name }}</span>
      </div>
      <p style="color: #999; margin-bottom: 16px">
        未自动识别制作组主站，请手动选择数据来源站点：
      </p>
      <a-radio-group v-model:value="selectedSourceSite" style="width: 100%">
        <div
          v-for="c in sourceCandidates"
          :key="c.site_name"
          style="display: flex; align-items: center; padding: 8px 0"
        >
          <a-radio :value="c.site_name" :disabled="!c.has_cookie">
            {{ c.site_name }}
          </a-radio>
          <a-tag v-if="c.torrent_id" color="blue" size="small" style="margin-left: 8px">
            ID: {{ c.torrent_id }}
          </a-tag>
          <a-tag v-if="!c.has_cookie" color="red" size="small" style="margin-left: 4px">
            缺 cookie
          </a-tag>
        </div>
      </a-radio-group>
      <div style="text-align: right; margin-top: 16px">
        <a-button style="margin-right: 8px" @click="sourceSelectOpen = false">取消</a-button>
        <a-button type="primary" :disabled="!selectedSourceSite" @click="confirmSourceSite">
          确定
        </a-button>
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
            <a-tag v-if="record.matched_site" color="green">{{ record.matched_site }}</a-tag>
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
import { ref, computed, onMounted, onUnmounted, watch, reactive } from 'vue'
import { message } from 'ant-design-vue'
import { publishTorrentsApi, type PublishTorrentItem } from '@/api/publish'
import { downloadersApi } from '@/api/downloaders'
import { formatBytes, maskDomain } from '@/utils/format'
import PublishWizardModal from './PublishWizardModal.vue'

const clients = ref<{ id: number; name: string; type: string }[]>([])
const clientsLoading = ref(false)
const selectedClientId = ref<number | undefined>(undefined)
const torrents = ref<PublishTorrentItem[]>([])
const loading = ref(false)
const searchText = ref('')
const queryFilter = ref<string | undefined>(undefined)
const queryingHash = ref('')

// 分页
const currentPage = ref(1)
const pageSize = ref(50)

// 后台查询状态
const querying = ref(false)
const queryDone = ref(0)
const queryTotal = ref(0)

let pollTimer: ReturnType<typeof setInterval> | null = null

const wizardOpen = ref(false)
const presetTorrent = ref<{ info_hash: string; name: string; size: number; save_path: string; client_id: number; state: string; source_site?: string; source_site_id?: number; torrent_id?: string } | null>(null)

const columns = [
  { title: '种子名称', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '大小', key: 'size', width: 90 },
  { title: '覆盖', key: 'coverage', width: 120, align: 'center' as const },
  { title: '可转', key: 'target_count', width: 100 },
  { title: '操作', key: 'actions', width: 180 },
]

const filteredTorrents = computed(() => {
  let result = torrents.value
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    result = result.filter(t => t.name.toLowerCase().includes(q))
  }
  if (queryFilter.value === 'queried') {
    result = result.filter(t => t.queried)
  } else if (queryFilter.value === 'unqueried') {
    result = result.filter(t => !t.queried)
  }
  return result
})

const pagedTorrents = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredTorrents.value.slice(start, start + pageSize.value)
})

const queryProgress = computed(() => {
  if (queryTotal.value === 0) return 0
  return Math.round((queryDone.value / queryTotal.value) * 100)
})

function coverageColor(status: string): string {
  const map: Record<string, string> = {
    confirmed_has: 'green',
    probably_has: 'blue',
    confirmed_not: 'default',
    probably_not: 'default',
    unknown: 'default',
  }
  return map[status] || 'default'
}

function onTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) currentPage.value = pag.current
  if (pag.pageSize) pageSize.value = pag.pageSize
}

function onClientChange() {
  currentPage.value = 1
  fetchTorrents()
}

async function fetchClients() {
  clientsLoading.value = true
  try {
    const resp = await downloadersApi.list(1, 100)
    const data = resp.data?.data
    clients.value = (data?.items || data || []) as { id: number; name: string; type: string }[]
    if (clients.value.length > 0 && !selectedClientId.value) {
      selectedClientId.value = clients.value[0].id
      fetchTorrents()
    }
  } catch { /* ignore */ } finally {
    clientsLoading.value = false
  }
}

async function fetchTorrents() {
  if (!selectedClientId.value) return
  loading.value = true
  try {
    const resp = await publishTorrentsApi.list(selectedClientId.value)
    const data = resp.data?.data
    torrents.value = data?.items || []
    querying.value = data?.querying ?? false
    queryDone.value = data?.query_progress?.done ?? 0
    queryTotal.value = data?.query_progress?.total ?? 0

    if (querying.value) {
      startPolling()
    } else {
      stopPolling()
    }
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

async function pollQueryStatus() {
  if (!selectedClientId.value) return
  try {
    const resp = await publishTorrentsApi.queryStatus(selectedClientId.value)
    const data = resp.data?.data
    querying.value = data?.querying ?? false
    queryDone.value = data?.done ?? 0
    queryTotal.value = data?.total ?? 0

    if (querying.value) {
      // 查询进行中，同时刷新种子数据（覆盖数据在逐步填充）
      await refreshTorrentsSilent()
    } else {
      // 查询完成，最终刷新
      stopPolling()
      await refreshTorrentsSilent()
    }
  } catch { /* silent */ }
}

async function refreshTorrentsSilent() {
  if (!selectedClientId.value) return
  try {
    const resp = await publishTorrentsApi.list(selectedClientId.value)
    torrents.value = resp.data?.data?.items || []
  } catch { /* silent */ }
}

function startPolling() {
  if (pollTimer) return
  pollTimer = setInterval(pollQueryStatus, 5000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(queryFilter, () => { currentPage.value = 1 })

async function queryCoverage(record: PublishTorrentItem) {
  if (!selectedClientId.value) return
  queryingHash.value = record.info_hash
  try {
    const resp = await publishTorrentsApi.queryCoverage({
      client_id: selectedClientId.value,
      info_hash: record.info_hash,
      name: record.name,
      size: record.size,
    })
    const result = resp.data?.data
    if (result) {
      record.coverage = {
        has_count: result.has_count,
        total_sites: result.total_sites,
        target_count: result.target_count,
        sites: result.sites,
      }
      record.queried = true
      message.success(`覆盖查询完成：${result.has_count}/${result.total_sites}`)
    }
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    queryingHash.value = ''
  }
}

function startForward(record: PublishTorrentItem) {
  presetTorrent.value = {
    info_hash: record.info_hash,
    name: record.name,
    size: record.size,
    save_path: record.save_path,
    client_id: selectedClientId.value!,
    state: record.state,
  }
  // 检测源头站
  detectAndOpen(record)
}

async function detectAndOpen(record: PublishTorrentItem) {
  try {
    const resp = await publishTorrentsApi.detectSource({
      info_hash: record.info_hash,
      name: record.name,
    })
    const result = resp.data?.data
    if (result?.source_site) {
      // 自动检测成功（或降级选中）
      presetTorrent.value = {
        ...presetTorrent.value!,
        source_site: result.source_site,
        source_site_id: result.source_site_id,
        torrent_id: result.torrent_id,
      }
      if (!result.auto_detected && result.candidates?.length > 1) {
        // 非自动判断 + 多候选 → 弹选择弹窗
        sourceCandidates.value = result.candidates
        sourceDetectRecord.value = record
        sourceSelectOpen.value = true
        return
      }
    }
    // 直接打开向导
    wizardOpen.value = true
  } catch {
    // 检测失败 → 直接打开向导（用默认源站）
    wizardOpen.value = true
  }
}

// 源站选择弹窗
const sourceSelectOpen = ref(false)
const sourceCandidates = ref<{ site_name: string; torrent_id: string; has_cookie: boolean }[]>([])
const sourceDetectRecord = ref<PublishTorrentItem | null>(null)
const selectedSourceSite = ref<string>('')

function confirmSourceSite() {
  const cand = sourceCandidates.value.find(c => c.site_name === selectedSourceSite.value)
  if (cand && presetTorrent.value) {
    presetTorrent.value = {
      ...presetTorrent.value,
      source_site: cand.site_name,
      torrent_id: cand.torrent_id,
    }
  }
  sourceSelectOpen.value = false
  wizardOpen.value = true
}

function onWizardSuccess() {
  fetchTorrents()
}

onMounted(fetchClients)

onUnmounted(() => {
  stopPolling()
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
  showTotal: (total: number) => `共 ${total} 条`,
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
      group_name: newMapping.group_name,
      domain: newMapping.domain,
      site_name: newMapping.site_name,
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
      group_name: editingMapping.value.group_name,
      domain: editingMapping.value.domain,
      site_name: editingMapping.value.site_name,
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
.coverage-cell {
  font-size: 16px;
  font-weight: 600;
  cursor: default;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.coverage-has {
  color: #52c41a;
}
.coverage-sep {
  color: #d9d9d9;
}
.coverage-total {
  color: #999;
}
.unqueried-tag {
  margin-left: 4px;
  transform: scale(0.85);
}
.query-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}
.progress-text {
  font-size: 12px;
  color: #666;
  white-space: nowrap;
}
</style>
