<template>
  <div class="step-content">
    <div class="step-toolbar">
      <a-select
        v-model:value="selectedClientId"
        style="width: 240px"
        :loading="clientsLoading"
        placeholder="选择下载器"
        @change="onClientChange"
      >
        <a-select-option v-for="c in clients" :key="c.id" :value="c.id">
          {{ c.name }} ({{ c.type }})
        </a-select-option>
      </a-select>
      <a-select
        v-model:value="selectedSavePath"
        style="width: 260px; margin-left: 12px"
        placeholder="资源路径"
        allow-clear
        show-search
        :options="savePathOptions"
      />
      <a-input-search
        v-model:value="torrentSearch"
        placeholder="搜索种子名称"
        style="width: 240px; margin-left: 12px"
        allow-clear
      />
    </div>

    <a-table
      :columns="torrentColumns"
      :data-source="filteredTorrents"
      :loading="torrentsLoading"
      :pagination="{ pageSize: 20, size: 'small' }"
      row-key="info_hash"
      size="small"
      :row-selection="{
        type: 'radio',
        selectedRowKeys: selectedTorrent ? [selectedTorrent.info_hash] : [],
        onSelect: (r: unknown) => { selectedTorrent = r as SeededTorrent }
      }"
      :scroll="{ y: 420 }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'source_site'">
          <a-tag v-if="record.source_site" color="blue">{{ record.source_site }}</a-tag>
          <span v-else style="color: #999">-</span>
        </template>
        <template v-if="column.key === 'size'">
          {{ formatBytes(record.size) }}
        </template>
        <template v-if="column.key === 'state'">
          <a-tag :color="qbStateColor(record.state)">{{ translateQbState(record.state) }}</a-tag>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { downloadersApi } from '@/api/downloaders'
import { manualForwardApi } from '@/api/publish'
import { useEnumLabels } from '@/utils/enumLabels'
import { formatBytes } from '@/utils/format'

export interface SeededTorrent {
  info_hash: string
  name: string
  size: number
  save_path: string
  upload_speed: number
  seeders: number
  state: string
  client_id: number
  source_site?: string
}

const props = defineProps<{
  modelValue: SeededTorrent | null
  presetClientId?: number
}>()
const emit = defineEmits<{
  'update:modelValue': [value: SeededTorrent | null]
}>()

const { translateQbState } = useEnumLabels()

const selectedTorrent = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const clients = ref<{ id: number; name: string; type: string }[]>([])
const clientsLoading = ref(false)
const selectedClientId = ref<number | undefined>(undefined)
const seededTorrents = ref<SeededTorrent[]>([])
const torrentsLoading = ref(false)
const torrentSearch = ref('')
const selectedSavePath = ref<string | undefined>(undefined)

const savePathOptions = computed(() => {
  const set = new Set<string>()
  for (const t of seededTorrents.value) {
    if (t.save_path) {
      set.add(t.save_path)
    }
  }
  return Array.from(set).sort().map(p => ({ label: p, value: p }))
})

const filteredTorrents = computed(() => {
  let result = seededTorrents.value
  if (selectedSavePath.value) {
    result = result.filter(t => t.save_path === selectedSavePath.value)
  }
  if (torrentSearch.value) {
    const q = torrentSearch.value.toLowerCase()
    result = result.filter(t => t.name.toLowerCase().includes(q))
  }
  return result
})

const torrentColumns = [
  { title: '种子名称', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '路径', dataIndex: 'save_path', key: 'save_path', width: 200, ellipsis: true },
  { title: '来源站', key: 'source_site', width: 80 },
  { title: '大小', key: 'size', width: 90 },
  { title: '状态', key: 'state', width: 90 },
]

function qbStateColor(state: string): string {
  const map: Record<string, string> = {
    uploadingUP: 'green', stalledUP: 'cyan', pausedUP: 'orange',
    queuedUP: 'blue', checkingUP: 'geekblue', forcedUP: 'green',
  }
  return map[state] || 'default'
}

function onClientChange() {
  selectedSavePath.value = undefined
  fetchSeededTorrents()
}

async function fetchClients() {
  clientsLoading.value = true
  try {
    const resp = await downloadersApi.listLight(1, 200)
    const data = resp.data?.data
    const items = (data?.items || data || []) as { id: number; name: string; type: string }[]
    clients.value = items
    if (props.presetClientId) {
      selectedClientId.value = props.presetClientId
    } else if (clients.value.length > 0 && !selectedClientId.value) {
      selectedClientId.value = clients.value[0].id
    }
    if (selectedClientId.value) {
      await fetchSeededTorrents()
    }
  } finally {
    clientsLoading.value = false
  }
}

async function fetchSeededTorrents() {
  if (!selectedClientId.value) return
  torrentsLoading.value = true
  try {
    const resp = await manualForwardApi.seededTorrents(selectedClientId.value)
    seededTorrents.value = ((resp.data?.data as unknown[]) || []) as SeededTorrent[]
  } finally {
    torrentsLoading.value = false
  }
}

onMounted(fetchClients)

watch(() => props.presetClientId, (v) => {
  if (v && v !== selectedClientId.value) {
    selectedClientId.value = v
    fetchSeededTorrents()
  }
})
</script>
