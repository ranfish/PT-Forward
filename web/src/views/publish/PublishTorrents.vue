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
      :scroll="{ y: 520 }"
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { message } from 'ant-design-vue'
import { publishTorrentsApi, type PublishTorrentItem } from '@/api/publish'
import { downloadersApi } from '@/api/downloaders'
import { formatBytes } from '@/utils/format'
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
const presetTorrent = ref<{ info_hash: string; name: string; size: number; save_path: string; client_id: number; state: string } | null>(null)

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
  wizardOpen.value = true
}

function onWizardSuccess() {
  fetchTorrents()
}

onMounted(fetchClients)

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.page-toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 4px;
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
