<template>
  <a-modal
    :open="open"
    title="批量获取数据"
    width="950px"
    :footer="null"
    destroy-on-close
    @cancel="$emit('update:open', false)"
  >
    <!-- 下载器选择 -->
    <div style="margin-bottom: 12px; display: flex; gap: 8px; align-items: center">
      <a-select
        v-model:value="clientId"
        style="width: 200px"
        placeholder="选择下载器"
        @change="fetchTorrents"
      >
        <a-select-option v-for="c in clients" :key="c.id" :value="c.id">{{ c.name }}</a-select-option>
      </a-select>
      <a-input-search
        v-if="torrents.length > 0"
        v-model:value="searchText"
        placeholder="搜索种子名"
        style="width: 250px"
        allow-clear
      />
      <a-button v-if="torrents.length > 0" @click="fetchTorrents"><ReloadOutlined /></a-button>
    </div>

    <!-- 进度条 -->
    <a-alert
      v-if="fetching"
      :message="`正在获取数据... ${doneCount}/${selectedHashes.length}`"
      type="info"
      show-icon
      style="margin-bottom: 12px"
    >
      <a-progress :percent="Math.round(doneCount / selectedHashes.length * 100)" size="small" />
      <div v-if="currentFetching" style="font-size: 12px; color: #666; margin-top: 4px">
        {{ currentFetching }}
      </div>
    </a-alert>

    <!-- 完成/失败汇总 -->
    <a-alert
      v-if="fetchDone"
      :message="`完成: ${successCount} 成功, ${failCount} 失败`"
      :type="failCount > 0 ? 'warning' : 'success'"
      show-icon
      closable
      style="margin-bottom: 12px"
      @close="fetchDone = false"
    />

    <!-- 种子列表 -->
    <a-table
      v-if="torrents.length > 0"
      :data-source="filteredTorrents"
      :columns="columns"
      :loading="loadingTorrents"
      row-key="info_hash"
      size="small"
      :scroll="{ y: 400 }"
      :row-selection="{ selectedRowKeys: selectedHashes, onChange: onSelectChange, getCheckboxProps: disableCheckbox }"
      :pagination="{ pageSize: 50, showSizeChanger: false, size: 'small' }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'">
          <span style="font-size: 12px">{{ record.name }}</span>
        </template>
        <template v-if="column.key === 'size'">
          {{ formatBytes(record.size) }}
        </template>
        <template v-if="column.key === 'cached'">
          <a-tag v-for="s in record.cachedSites" :key="s" color="green" style="margin: 1px; font-size: 11px">{{ s }}</a-tag>
          <span v-if="!record.cachedSites?.length" style="color: #ccc; font-size: 12px">无</span>
        </template>
      </template>
    </a-table>

    <a-empty v-else-if="!loadingTorrents && clientId" description="该下载器无做种" />

    <!-- Footer -->
    <div style="margin-top: 12px; display: flex; justify-content: flex-end; gap: 8px">
      <a-button @click="$emit('update:open', false)">关闭</a-button>
      <a-button
        type="primary"
        :loading="fetching"
        :disabled="selectedHashes.length === 0"
        @click="startBatchFetch"
      >
        批量获取（{{ selectedHashes.length }}）
      </a-button>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { message } from 'ant-design-vue'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { downloadersApi } from '@/api/downloaders'
import { manualForwardApi, publishDataApi } from '@/api/publish'
import { formatBytes } from '@/utils/format'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'done'): void
}>()

interface ClientItem { id: number; name: string }
interface TorrentItem {
  info_hash: string
  name: string
  size: number
  save_path: string
  state: string
  cachedSites?: string[]
}

const clients = ref<ClientItem[]>([])
const clientId = ref<number | undefined>(undefined)
const torrents = ref<TorrentItem[]>([])
const loadingTorrents = ref(false)
const searchText = ref('')
const selectedHashes = ref<string[]>([])

const fetching = ref(false)
const doneCount = ref(0)
const currentFetching = ref('')
const successCount = ref(0)
const failCount = ref(0)
const fetchDone = ref(false)

const columns = [
  { title: '种子名', key: 'name', ellipsis: true },
  { title: '大小', key: 'size', width: 80 },
  { title: '已有数据', key: 'cached', width: 200 },
]

const filteredTorrents = computed(() => {
  if (!searchText.value) return torrents.value
  return torrents.value.filter(t => t.name.toLowerCase().includes(searchText.value.toLowerCase()))
})

watch(() => props.open, async (val) => {
  if (val) {
    reset()
    await fetchClients()
  }
})

function reset() {
  torrents.value = []
  selectedHashes.value = []
  searchText.value = ''
  fetching.value = false
  doneCount.value = 0
  successCount.value = 0
  failCount.value = 0
  fetchDone.value = false
  currentFetching.value = ''
}

async function fetchClients() {
  try {
    const resp = await downloadersApi.listLight(1, 100)
    const items = resp.data?.data?.items || []
    clients.value = items.map((c) => ({
      id: c.id,
      name: c.name,
    }))
    if (clients.value.length > 0 && !clientId.value) {
      clientId.value = clients.value[0].id
      await fetchTorrents()
    }
  } catch { /* silent */ }
}

async function fetchTorrents() {
  if (!clientId.value) return
  loadingTorrents.value = true
  torrents.value = []
  selectedHashes.value = []
  try {
    const resp = await manualForwardApi.seededTorrents(clientId.value)
    const items = (resp.data?.data || []) as unknown[]
    torrents.value = items.map((item) => {
      const obj = item as Record<string, unknown>
      return {
        info_hash: obj.info_hash as string,
        name: obj.name as string,
        size: obj.size as number,
        save_path: obj.save_path as string,
        state: obj.state as string,
        cachedSites: [],
      }
    })
    // 异步加载每个种子的已缓存站点
    for (const t of torrents.value) {
      publishDataApi.cachedSites(t.info_hash).then(resp => {
        const sites = resp.data?.data?.sites || []
        t.cachedSites = sites.map(s => s.site_name)
      }).catch(() => {})
    }
  } catch { /* silent */ } finally {
    loadingTorrents.value = false
  }
}

function onSelectChange(keys: string[]) {
  selectedHashes.value = keys
}

function disableCheckbox(record: TorrentItem) {
  return { disabled: record.cachedSites?.length ? false : false }
}

async function startBatchFetch() {
  if (selectedHashes.value.length === 0) return
  fetching.value = true
  fetchDone.value = false
  doneCount.value = 0
  successCount.value = 0
  failCount.value = 0

  for (const hash of selectedHashes.value) {
    const torrent = torrents.value.find(t => t.info_hash === hash)
    if (!torrent) continue
    currentFetching.value = torrent.name
    try {
      const resp = await manualForwardApi.startAnalyze({
        client_id: clientId.value!,
        info_hash: torrent.info_hash,
        name: torrent.name,
        save_path: torrent.save_path,
        size: torrent.size,
        fetch_source: 'batch_fetch',
      })
      const taskId = resp.data?.data?.task_id
      if (!taskId) throw new Error('任务创建失败')
      // Poll until done
      await pollTask(taskId)
      successCount.value++
    } catch {
      failCount.value++
    }
    doneCount.value++
  }

  fetching.value = false
  currentFetching.value = ''
  fetchDone.value = true
  message.success(`批量获取完成: ${successCount.value} 成功, ${failCount.value} 失败`)
  emit('done')
}

function pollTask(taskId: number): Promise<void> {
  return new Promise((resolve, reject) => {
    async function poll() {
      try {
        const resp = await manualForwardApi.pollAnalyze(taskId)
        const task = resp.data?.data as Record<string, unknown> | undefined
        if (!task) { resolve(); return }
        const status = task.status as string
        if (status === 'done') { resolve(); return }
        if (status === 'failed') { reject(new Error(task.error as string || '分析失败')); return }
        setTimeout(poll, 2000)
      } catch (e) {
        reject(e)
      }
    }
    setTimeout(poll, 1500)
  })
}
</script>
