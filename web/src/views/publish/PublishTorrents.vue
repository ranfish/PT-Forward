<template>
  <div>
    <div class="page-toolbar">
      <a-select
        v-model:value="selectedClientId"
        style="width: 260px"
        :loading="clientsLoading"
        placeholder="选择下载器"
        @change="fetchTorrents"
      >
        <a-select-option v-for="c in clients" :key="c.id" :value="c.id">
          {{ c.name }} ({{ c.type }})
        </a-select-option>
      </a-select>
      <a-input-search
        v-if="torrents.length"
        v-model:value="searchText"
        placeholder="搜索种子名称..."
        style="width: 280px; margin-left: 12px"
        allow-clear
      />
      <a-tag v-if="torrents.length" color="blue" style="margin-left: 8px">
        共 {{ filteredTorrents.length }} 个种子
      </a-tag>
    </div>

    <a-table
      :columns="columns"
      :data-source="filteredTorrents"
      :loading="loading"
      :pagination="{ pageSize: 15, showSizeChanger: false, size: 'small' }"
      row-key="info_hash"
      size="small"
      :scroll="{ y: 520 }"
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
              <div v-else style="color: #999">暂无覆盖数据，点击"查询覆盖"</div>
            </template>
            <div class="coverage-cell">
              <span class="coverage-has">{{ record.coverage?.has_count ?? 0 }}</span>
              <span class="coverage-sep">/</span>
              <span class="coverage-total">{{ record.coverage?.total_sites ?? 0 }}</span>
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
              :disabled="(record.coverage?.target_count ?? 0) === 0 && !queryingHash"
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
import { ref, computed, onMounted } from 'vue'
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
const queryingHash = ref('')

const wizardOpen = ref(false)
const presetTorrent = ref<{ info_hash: string; name: string; size: number; save_path: string; client_id: number; state: string } | null>(null)

const columns = [
  { title: '种子名称', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '大小', key: 'size', width: 90 },
  { title: '覆盖', key: 'coverage', width: 100, align: 'center' as const },
  { title: '可转', key: 'target_count', width: 100 },
  { title: '操作', key: 'actions', width: 180 },
]

const filteredTorrents = computed(() => {
  if (!searchText.value) return torrents.value
  const q = searchText.value.toLowerCase()
  return torrents.value.filter(t => t.name.toLowerCase().includes(q))
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
    torrents.value = resp.data?.data?.items || []
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

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
</script>

<style scoped>
.page-toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}
.coverage-cell {
  font-size: 16px;
  font-weight: 600;
  cursor: default;
}
.coverage-has {
  color: #52c41a;
}
.coverage-sep {
  color: #d9d9d9;
  margin: 0 2px;
}
.coverage-total {
  color: #999;
}
</style>
