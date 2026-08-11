<template>
  <a-modal
    :open="open"
    title="获取种子数据"
    width="900px"
    :footer="null"
    destroy-on-close
    @cancel="handleClose"
  >
    <!-- 工具栏 -->
    <div style="margin-bottom: 12px; display: flex; gap: 8px; align-items: center; flex-wrap: wrap">
      <a-input-search
        v-if="torrents.length > 0"
        v-model:value="searchText"
        placeholder="搜索种子名"
        style="width: 250px"
        allow-clear
      />
      <a-button size="small" @click="openPriorityDialog">获取优先级</a-button>
      <a-button v-if="torrents.length > 0" size="small" @click="loadUnconfigured">刷新</a-button>
      <a-tag v-if="torrents.length > 0" color="blue">{{ torrents.length }} 个未配置</a-tag>
    </div>

    <!-- 进度条 -->
    <a-alert
      v-if="fetching"
      :message="`正在获取数据... ${progress.done}/${progress.total}`"
      type="info"
      show-icon
      style="margin-bottom: 12px"
    >
      <a-progress :percent="progress.total > 0 ? Math.round(progress.done / progress.total * 100) : 0" size="small" />
    </a-alert>

    <!-- 完成汇总 -->
    <a-alert
      v-if="fetchDone"
      :message="`完成: ${progress.done - progress.failed} 成功${progress.failed > 0 ? ', ' + progress.failed + ' 失败' : ''}`"
      :type="progress.failed > 0 ? 'warning' : 'success'"
      show-icon
      closable
      style="margin-bottom: 12px"
      @close="fetchDone = false"
    >
      <div v-if="progress.items.length > 0" style="max-height: 200px; overflow-y: auto; margin-top: 8px">
        <div v-for="item in progress.items" :key="item.hash" style="display: flex; align-items: center; gap: 6px; padding: 2px 0; font-size: 12px">
          <span v-if="item.status === 'done'" style="color: #52c41a">✓</span>
          <span v-else-if="item.status === 'failed'" style="color: #ff4d4f">✗</span>
          <span v-else style="color: #999">…</span>
          <span style="flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ item.name }}</span>
          <span v-if="item.error" style="color: #999; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ item.error }}</span>
        </div>
      </div>
    </a-alert>

    <!-- 种子列表 -->
    <a-table
      v-if="torrents.length > 0"
      :data-source="filteredTorrents"
      :columns="columns"
      :loading="loading"
      row-key="hash"
      size="small"
      :scroll="{ y: 400 }"
      :row-selection="{ selectedRowKeys: selectedHashes, onChange: onSelectChange }"
      :pagination="{ pageSize: 50, showSizeChanger: false, size: 'small' }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'">
          <span style="font-size: 12px">{{ record.name }}</span>
        </template>
        <template v-if="column.key === 'size'">
          {{ formatBytes(record.size) }}
        </template>
      </template>
    </a-table>

    <a-empty v-else-if="!loading" description="无未配置种子（全部已获取或无快照）" />

    <!-- 优先级设置弹窗 -->
    <a-modal
      v-model:open="priorityDialogVisible"
      title="获取站点优先级"
      width="520px"
      :confirm-loading="prioritySaving"
      @ok="savePriority"
    >
      <a-alert
        type="info"
        show-icon
        style="margin-bottom: 12px"
        message="设置「获取数据」时的站点优先级（制作组映射命中优先于本列表）"
      />
      <div style="min-height: 40px">
        <a-tag
          v-for="(site, index) in priorityList"
          :key="site"
          :color="index < 3 ? ['success', 'processing', 'warning'][index] : 'default'"
          style="margin: 4px; cursor: move; user-select: none; font-size: 13px; padding: 4px 12px"
          draggable="true"
          @dragstart="draggedIndex = index"
          @dragover.prevent
          @drop="onDrop(index)"
        >
          {{ index + 1 }}. {{ site }}
        </a-tag>
        <span v-if="priorityList.length === 0" style="color: #999; font-size: 12px">未配置（使用默认优先级）</span>
      </div>
      <div style="margin-top: 8px; font-size: 12px; color: #999">拖拽调整顺序</div>
    </a-modal>

    <!-- Footer -->
    <div style="margin-top: 12px; display: flex; justify-content: flex-end; gap: 8px">
      <a-button @click="handleClose">关闭</a-button>
      <a-button
        type="primary"
        :loading="fetching"
        :disabled="selectedHashes.length === 0"
        @click="startBatchFetch"
      >
        获取数据（{{ selectedHashes.length }}）
      </a-button>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { message } from 'ant-design-vue'
import { seedConfigApi } from '@/api/publish'
import { formatBytes } from '@/utils/format'

const props = defineProps<{
  open: boolean
  clientId?: string
  savePath?: string
}>()
const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'completed'): void
}>()

interface UnconfiguredTorrent {
  hash: string
  name: string
  size: number
  client_id: string
  save_path: string
}

const torrents = ref<UnconfiguredTorrent[]>([])
const loading = ref(false)
const searchText = ref('')
const selectedHashes = ref<string[]>([])

const fetching = ref(false)
const fetchDone = ref(false)
const progress = ref({
  active: false,
  total: 0,
  done: 0,
  failed: 0,
  items: [] as Array<{ hash: string; name: string; status: string; error?: string }>,
})

const priorityDialogVisible = ref(false)
const prioritySaving = ref(false)
const priorityList = ref<string[]>([])
const draggedIndex = ref<number | null>(null)

const columns = [
  { title: '种子名', key: 'name', ellipsis: true },
  { title: '大小', key: 'size', width: 80 },
]

const filteredTorrents = computed(() => {
  if (!searchText.value) return torrents.value
  const q = searchText.value.toLowerCase()
  return torrents.value.filter(t => t.name.toLowerCase().includes(q))
})

watch(() => props.open, async (val) => {
  if (val) {
    reset()
    await loadUnconfigured()
  }
})

function reset() {
  torrents.value = []
  selectedHashes.value = []
  searchText.value = ''
  fetching.value = false
  fetchDone.value = false
  progress.value = { active: false, total: 0, done: 0, failed: 0, items: [] }
}

async function loadUnconfigured() {
  if (!props.clientId || !props.savePath) return
  loading.value = true
  try {
    const resp = await seedConfigApi.snapshotUnconfigured(props.clientId, props.savePath)
    torrents.value = resp.data?.data?.items || []
  } catch {
    message.error('加载未配置种子失败')
    torrents.value = []
  } finally {
    loading.value = false
  }
}

function onSelectChange(keys: string[]) {
  selectedHashes.value = keys
}

async function startBatchFetch() {
  if (selectedHashes.value.length === 0) return
  const items = torrents.value
    .filter(t => selectedHashes.value.includes(t.hash))
    .map(t => ({ hash: t.hash, name: t.name, size: t.size, save_path: t.save_path }))

  try {
    await seedConfigApi.batchFetch(items, props.clientId || '')
    fetching.value = true
    fetchDone.value = false
    progress.value = { active: true, total: items.length, done: 0, failed: 0, items: [] }
    pollProgress()
  } catch (e) {
    message.error('启动批量获取失败：' + (e as Error).message)
  }
}

let pollTimer: ReturnType<typeof setTimeout> | null = null

function pollProgress() {
  pollTimer = setTimeout(async () => {
    try {
      const resp = await seedConfigApi.batchFetchProgress()
      const data = resp.data?.data
      if (data) {
        progress.value = {
          active: data.active,
          total: data.total,
          done: data.done,
          failed: data.failed,
          items: data.items || [],
        }
      }
      if (data?.active) {
        pollProgress()
      } else {
        fetching.value = false
        fetchDone.value = true
        const success = (data?.done || 0) - (data?.failed || 0)
        message.success(`获取完成: ${success} 成功${(data?.failed || 0) > 0 ? ', ' + data.failed + ' 失败' : ''}`)
        emit('completed')
        await loadUnconfigured()
      }
    } catch {
      if (fetching.value) pollProgress()
    }
  }, 1500)
}

function handleClose() {
  if (pollTimer) clearTimeout(pollTimer)
  emit('update:open', false)
}

async function openPriorityDialog() {
  try {
    const resp = await seedConfigApi.getFetchPriority()
    priorityList.value = resp.data?.data?.priority || []
  } catch { /* silent */ }
  priorityDialogVisible.value = true
}

async function savePriority() {
  prioritySaving.value = true
  try {
    await seedConfigApi.setFetchPriority([...priorityList.value])
    message.success('优先级已保存')
    priorityDialogVisible.value = false
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    prioritySaving.value = false
  }
}

function onDrop(dropIndex: number) {
  if (draggedIndex.value === null) return
  const item = priorityList.value.splice(draggedIndex.value, 1)[0]
  priorityList.value.splice(dropIndex, 0, item)
  draggedIndex.value = null
}
</script>
