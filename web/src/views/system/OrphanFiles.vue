<template>
  <div>
    <a-page-header :title="t('orphan.title')">
      <template #extra>
        <a-space>
          <a-button
            type="primary"
            :disabled="selectedRowKeys.length === 0 || batchRecovering"
            :loading="batchRecovering"
            @click="batchRecover"
          >
            {{ t('orphan.batchRecover') }} ({{ selectedRowKeys.length }})
          </a-button>
          <a-button :loading="scanning" @click="scan">
            {{ t('orphan.scan') }}
          </a-button>
          <a-button size="small" @click="showIgnored = !showIgnored; if (showIgnored) fetchIgnored()">
            忽略列表 ({{ ignoredPaths.length }})
          </a-button>
        </a-space>
      </template>
    </a-page-header>

    <a-alert v-if="scannedAt" type="info" style="margin: 0 24px 16px" show-icon>
      {{ t('orphan.lastScan') }}: {{ formatTime(scannedAt.toISOString()) }} ·
      {{ orphans.length }} {{ t('orphan.itemsFound') }}
    </a-alert>

    <div style="padding: 0 24px; margin-bottom: 16px">
      <a-card v-if="showIgnored" size="small" title="忽略列表（这些路径不会被扫描）" style="margin-bottom: 16px">
        <div v-if="ignoredPaths.length === 0" style="color: #999">无忽略记录</div>
        <div v-for="path in ignoredPaths" :key="path" style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px">
          <code style="flex: 1; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ path }}</code>
          <a-button size="small" type="link" danger @click="deleteIgnored(path)">移除</a-button>
        </div>
      </a-card>
    </div>

    <div style="padding: 0 24px; margin-bottom: 16px">
      <a-card size="small" :title="t('orphan.recoverSettings')">
        <a-space>
          <span>{{ t('orphan.categoryLabel') }}:</span>
          <a-input v-model:value="recoverCategory" style="width: 200px" placeholder="orphan-recover" />
          <span>{{ t('orphan.tagsLabel') }}:</span>
          <a-input v-model:value="recoverTags" style="width: 300px" placeholder="orphan-recover,recovered" />
          <a-button size="small" type="primary" @click="saveSettings">{{ t('common.save') }}</a-button>
        </a-space>
      </a-card>
    </div>

    <div style="padding: 0 24px; margin-bottom: 16px">
      <a-card size="small" :title="t('orphan.scanConfigTitle')">
        <div v-if="scanConfigs.length > 0" style="margin-bottom: 12px">
          <a-tag v-for="cfg in scanConfigs" :key="cfg.id" closable @close="deleteScanConfig(cfg.id)" style="margin-bottom: 4px">
            {{ cfg.client_id }}: {{ cfg.scan_path }}
          </a-tag>
        </div>
        <a-space>
          <a-select v-model:value="newConfigClient" style="width: 130px" :placeholder="t('orphan.selectClient')">
            <a-select-option v-for="c in availableClients" :key="c" :value="c">{{ c }}</a-select-option>
          </a-select>
          <a-input v-model:value="newConfigPath" style="width: 300px" placeholder="/PT1/SSD" />
          <a-button size="small" type="primary" :disabled="!newConfigClient || !newConfigPath" @click="addScanConfig">
            {{ t('orphan.addScanPath') }}
          </a-button>
        </a-space>
      </a-card>
    </div>

    <div style="padding: 0 24px">
      <a-table
        :columns="columns"
        :data-source="orphans"
        :loading="scanning"
        :pagination="pagination"
        :row-selection="{ selectedRowKeys, onChange: onSelectChange }"
        row-key="path"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <span style="font-family:monospace;font-size:12px">{{ record.name }}</span>
          </template>
          <template v-if="column.key === 'size'">
            {{ formatBytes(record.size) }}
          </template>
          <template v-if="column.key === 'type'">
            <a-tag :color="record.is_dir ? 'blue' : 'green'">
              {{ record.is_dir ? t('orphan.directory') : t('orphan.file') }}
            </a-tag>
          </template>
          <template v-if="column.key === 'client'">
            <a-select
              v-model:value="record._selectedClient"
              size="small"
              style="width: 110px"
              :options="(record.client_ids || []).map((c: string) => ({ label: c, value: c }))"
            />
          </template>
          <template v-if="column.key === 'status'">
            <a-tag v-if="record._status === 'searching'" color="processing">搜索中 {{ record._elapsed || 0 }}s</a-tag>
            <a-tag v-else-if="record._status === 'found'" color="success">已恢复</a-tag>
            <a-tag v-else-if="record._status === 'notfound'" color="default">未找到</a-tag>
            <a-tag v-else-if="record._status === 'error'" color="error">失败</a-tag>
            <span v-else style="color:#999">-</span>
          </template>
          <template v-if="column.key === 'action'">
            <a-space size="small">
              <a-button
                size="small"
                type="link"
                :disabled="batchRecovering || record._status === 'found'"
                :loading="recovering === record.path"
                @click="recover(record)"
              >
                {{ t('orphan.recover') }}
              </a-button>
              <a-button size="small" type="link" :disabled="batchRecovering" @click="ignoreOrphan(record)">
                {{ t('orphan.ignore') }}
              </a-button>
              <a-button size="small" type="link" danger :disabled="batchRecovering" @click="confirmDelete(record)">
                {{ t('orphan.deleteFile') }}
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </div>

    <a-modal
      v-model:open="resultVisible"
      :title="t('orphan.recoverResult')"
      :footer="null"
    >
      <a-result
        v-if="recoverResult"
        :status="recoverResult.found ? 'success' : 'warning'"
        :title="recoverResult.found ? t('orphan.recoverSuccess') : t('orphan.recoverFailed')"
        :sub-title="recoverResult.message"
      />
    </a-modal>

    <a-modal
      v-model:open="batchResultVisible"
      :title="t('orphan.batchRecover')"
      :footer="null"
      width="600px"
    >
      <a-statistic
        :title="t('orphan.batchTotal') + ': ' + batchStats.total"
        :value="batchStats.found"
        suffix="/ " + batchStats.total
      >
        <template #title>
          <span>{{ t('orphan.batchFound') }}: {{ batchStats.found }} · {{ t('orphan.batchNotFound') }}: {{ batchStats.notFound }} · {{ t('orphan.batchError') }}: {{ batchStats.error }}</span>
        </template>
      </a-statistic>
      <div style="margin-top:16px;max-height:300px;overflow-y:auto">
        <div v-for="r in batchResults" :key="r.path" style="margin-bottom:4px;font-size:12px">
          <a-tag :color="r.found ? 'success' : 'default'" style="font-size:11px">
            {{ r.found ? '✅' : '❌' }}
          </a-tag>
          <span style="font-family:monospace">{{ r.name }}</span>
          <span v-if="r.site" style="color:#52c41a;margin-left:8px">{{ r.site }}</span>
        </div>
      </div>
    </a-modal>

    <a-modal
      v-model:open="deleteModalVisible"
      title="删除文件确认"
      :ok-text="t('common.confirm')"
      cancel-text="取消"
      :ok-button-props="{ danger: true, loading: deleting }"
      :ok-type="'default'"
      @ok="doDelete"
    >
      <a-alert type="error" style="margin-bottom: 16px" message="此操作将从磁盘永久删除以下文件，不可恢复！" />
      <div style="margin-bottom: 16px; font-size: 12px; color: #999; word-break: break-all">
        {{ deleteTarget?.path }}
      </div>
      <a-input-password
        v-model:value="deletePassword"
        placeholder="请输入登录密码以确认删除"
        @press-enter="doDelete"
      />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { formatTime, formatBytes } from '@/utils/format'

const { t } = useI18n()

interface OrphanEntry {
  path: string
  name: string
  size: number
  is_dir: boolean
  client_ids: string[]
  save_path: string
  detected_at: string
  _status?: string
  _selectedClient?: string
  _elapsed?: number
}

interface BatchResult {
  path: string
  name: string
  found: boolean
  site?: string
  message?: string
}

const orphans = ref<OrphanEntry[]>([])
const scanning = ref(false)
const scannedAt = ref<Date | null>(null)
const showIgnored = ref(false)
const ignoredPaths = ref<string[]>([])
const recovering = ref<string | null>(null)
const deleteModalVisible = ref(false)
const deleteTarget = ref<OrphanEntry | null>(null)
const deletePassword = ref('')
const deleting = ref(false)
const resultVisible = ref(false)
const recoverResult = ref<{ found: boolean; message: string } | null>(null)
const selectedRowKeys = ref<string[]>([])
const batchRecovering = ref(false)
const batchResultVisible = ref(false)
const batchResults = ref<BatchResult[]>([])
const batchStats = ref({ total: 0, found: 0, notFound: 0, error: 0 })
const recoverCategory = ref('orphan-recover')
const recoverTags = ref('orphan-recover')
const pageSize = ref(50)
const scanConfigs = ref<{id: number; client_id: string; scan_path: string; enabled: boolean}[]>([])
const newConfigClient = ref('')
const newConfigPath = ref('')
const availableClients = ref<string[]>([])

const pagination = {
  pageSize: pageSize.value,
  showSizeChanger: true,
  pageSizeOptions: ['50', '100', '200'],
  onChange: (page: number, size: number) => {
    pageSize.value = size
    pagination.pageSize = size
  },
  onShowSizeChange: (_current: number, size: number) => {
    pageSize.value = size
    pagination.pageSize = size
  },
}

const columns = [
  { title: t('orphan.columnName'), key: 'name', ellipsis: true },
  { title: t('orphan.columnType'), key: 'type', width: 80 },
  { title: t('orphan.columnSize'), key: 'size', width: 100 },
  { title: t('orphan.columnClient'), key: 'client', width: 130 },
  { title: t('orphan.columnStatus'), key: 'status', width: 100 },
  { title: t('common.action'), key: 'action', width: 180 },
]

function onSelectChange(keys: string[]) {
  selectedRowKeys.value = keys
}

async function fetchOrphans() {
  try {
    const resp = await fetch('/api/v1/orphans', {
      headers: authHeaders()
    })
    const data = await resp.json()
    if (data.code === 0) {
      orphans.value = (data.data.orphans || []).map((o: OrphanEntry) => ({ ...o, _selectedClient: o.client_ids?.[0] }))
      if (data.data.scanned_at) {
        scannedAt.value = new Date(data.data.scanned_at)
      }
    }
  } catch {
    // ignore
  }
}

async function loadSettings() {
  try {
    for (const [key, ref_] of [['orphan_recover_category', recoverCategory], ['orphan_recover_tags', recoverTags]] as [string, { value: string }][]) {
      const resp = await fetch(`/api/v1/settings/${key}`, { headers: authHeaders() })
      const data = await resp.json()
      if (data.code === 0 && data.data?.value) {
        ref_.value = data.data.value
      }
    }
  } catch { /* ignore */ }
}

async function saveSettings() {
  try {
    await fetch('/api/v1/settings/orphan_recover_category', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ value: recoverCategory.value })
    })
    await fetch('/api/v1/settings/orphan_recover_tags', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ value: recoverTags.value })
    })
    message.success(t('common.saved'))
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function scan() {
  scanning.value = true
  selectedRowKeys.value = []
  try {
    const resp = await fetch('/api/v1/orphans/scan', {
      method: 'POST',
      headers: { Authorization: `Bearer ${localStorage.getItem('pt-forward-access-token')}` }
    })
    const data = await resp.json()
    if (data.code === 0) {
      orphans.value = (data.data.orphans || []).map((o: OrphanEntry) => ({ ...o, _selectedClient: o.client_ids?.[0] }))
      scannedAt.value = new Date(data.data.scanned_at)
      message.success(`${data.data.count} ${t('orphan.itemsFound')}`)
    } else {
      message.error(data.message || 'Scan failed')
    }
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    scanning.value = false
  }
}

function authHeaders(): HeadersInit {
  return { Authorization: `Bearer ${localStorage.getItem('pt-forward-access-token')}` }
}

async function loadScanConfigs() {
  try {
    const resp = await fetch('/api/v1/orphans/scan-configs', { headers: authHeaders() })
    const data = await resp.json()
    if (data.code === 0) {
      scanConfigs.value = data.data || []
    }
  } catch { /* ignore */ }
}

async function loadAvailableClients() {
  try {
    const resp = await fetch('/api/v1/downloaders', { headers: authHeaders() })
    const data = await resp.json()
    const clients = data.data?.items || data.data?.clients || []
    if (Array.isArray(clients)) {
      availableClients.value = clients
        .filter((c: { enabled?: boolean; connected?: boolean }) => c.enabled !== false)
        .map((c: { name?: string }) => c.name || '')
        .filter(Boolean)
    }
  } catch { /* ignore */ }
}

async function addScanConfig() {
  if (!newConfigClient.value || !newConfigPath.value) return
  try {
    const resp = await fetch('/api/v1/orphans/scan-configs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ client_id: newConfigClient.value, scan_path: newConfigPath.value, enabled: true })
    })
    const data = await resp.json()
    if (data.code === 0) {
      scanConfigs.value.push(data.data)
      newConfigPath.value = ''
      message.success(t('orphan.added'))
      fetchOrphans()
    }
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function deleteScanConfig(id: number) {
  try {
    await fetch(`/api/v1/orphans/scan-configs?id=${id}`, {
      method: 'DELETE',
      headers: authHeaders()
    })
    scanConfigs.value = scanConfigs.value.filter(c => c.id !== id)
  } catch { /* ignore */ }
}

async function recoverOrphan(
  path: string,
  clientID?: string,
  onProgress?: (elapsedSec: number) => void
): Promise<{ found: boolean; site: string; message: string }> {
  const startTime = Date.now()
  const timer = onProgress ? setInterval(() => {
    onProgress(Math.floor((Date.now() - startTime) / 1000))
  }, 1000) : undefined
  if (onProgress) onProgress(0)

  try {
    const resp = await fetch('/api/v1/orphans/recover', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ path, client_id: clientID })
    })
    const data = await resp.json()
    if (data.code !== 0) {
      return { found: false, site: '', message: data.message || 'failed' }
    }
    const taskID = data.data.task_id
    for (let i = 0; i < 100; i++) {
      await new Promise(r => setTimeout(r, 3000))
      try {
        const r = await fetch(`/api/v1/orphans/recover/${taskID}`, { headers: authHeaders() })
        const d = await r.json()
        if (d.code === 0 && d.data) {
          const msg = d.data.message || ''
          if (msg !== 'searching...' && msg !== '') {
            return { found: !!d.data.found, site: d.data.site_name || '', message: msg }
          }
        }
      } catch { /* ignore */ }
    }
    return { found: false, site: '', message: '恢复超时（5分钟），请重试' }
  } finally {
    if (timer) clearInterval(timer)
  }
}

async function recover(orphan: OrphanEntry) {
  recovering.value = orphan.path
  resultVisible.value = false
  orphan._elapsed = 0
  message.loading(t('orphan.recovering'), 0)
  try {
    const result = await recoverOrphan(orphan.path, orphan._selectedClient, (sec) => {
      orphan._elapsed = sec
    })
    message.destroy()
    if (result.found) {
      orphan._status = 'found'
      message.success(t('orphan.recoverSuccess') + ': ' + result.site)
      setTimeout(() => fetchOrphans(), 2000)
    } else {
      orphan._status = 'notfound'
      message.warning(result.message || t('orphan.recoverFailed'))
    }
  } catch (e: unknown) {
    message.destroy()
    message.error(e instanceof Error ? e.message : String(e))
    orphan._status = 'error'
  } finally {
    recovering.value = null
  }
}

async function batchRecover() {
  const selected = orphans.value.filter(o => selectedRowKeys.value.includes(o.path) && o._status !== 'found')
  if (selected.length === 0) return

  batchRecovering.value = true
  batchResults.value = []
  batchStats.value = { total: selected.length, found: 0, notFound: 0, error: 0 }

  const MAX_CONCURRENT = 5
  let index = 0
  let done = 0

  message.loading({ content: `${t('orphan.batchRecovering')} 0/${selected.length}`, key: 'batch-recover', duration: 0 })

  const worker = async () => {
    while (index < selected.length) {
      const orphan = selected[index++]
      orphan._status = 'searching'
      orphan._elapsed = 0
      try {
        const result = await recoverOrphan(orphan.path, orphan._selectedClient, (sec) => {
          orphan._elapsed = sec
        })
        batchResults.value.push({
          path: orphan.path,
          name: orphan.name,
          found: result.found,
          site: result.site,
          message: result.message
        })
        if (result.found) {
          orphan._status = 'found'
          batchStats.value.found++
        } else {
          orphan._status = 'notfound'
          batchStats.value.notFound++
        }
      } catch {
        batchResults.value.push({
          path: orphan.path,
          name: orphan.name,
          found: false,
          message: 'error'
        })
        orphan._status = 'error'
        batchStats.value.error++
      }
      done++
      message.loading({ content: `${t('orphan.batchRecovering')} ${done}/${selected.length}`, key: 'batch-recover', duration: 0 })
    }
  }

  const workers = []
  for (let i = 0; i < Math.min(MAX_CONCURRENT, selected.length); i++) {
    workers.push(worker())
  }
  await Promise.all(workers)

  message.destroy('batch-recover')
  batchResultVisible.value = true
  selectedRowKeys.value = []
  batchRecovering.value = false

  const found = batchStats.value.found
  if (found > 0) {
    message.success(`${found} ${t('orphan.recoverSuccess')}`)
    setTimeout(() => fetchOrphans(), 2000)
  }
}

async function ignoreOrphan(orphan: OrphanEntry) {
  try {
    const resp = await fetch('/api/v1/orphans/ignore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ path: orphan.path })
    })
    const data = await resp.json()
    if (data.code === 0) {
      orphans.value = orphans.value.filter(o => o.path !== orphan.path)
      message.success(t('orphan.ignored'))
    }
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function fetchIgnored() {
  try {
    const resp = await fetch('/api/v1/orphans/ignored', { headers: authHeaders() })
    const data = await resp.json()
    if (data.code === 0) {
      ignoredPaths.value = data.data.ignored || []
    }
  } catch { /* silent */ }
}

async function deleteIgnored(path: string) {
  try {
    const resp = await fetch(`/api/v1/orphans/ignored?path=${encodeURIComponent(path)}`, {
      method: 'DELETE',
      headers: authHeaders()
    })
    const data = await resp.json()
    if (data.code === 0) {
      ignoredPaths.value = ignoredPaths.value.filter(p => p !== path)
      message.success('已移除忽略记录')
    }
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

function confirmDelete(orphan: OrphanEntry) {
  deleteTarget.value = orphan
  deletePassword.value = ''
  deleteModalVisible.value = true
}

async function doDelete() {
  if (!deleteTarget.value || !deletePassword.value) return
  deleting.value = true
  try {
    const resp = await fetch('/api/v1/orphans/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ path: deleteTarget.value.path, password: deletePassword.value })
    })
    const data = await resp.json()
    if (data.code === 0) {
      orphans.value = orphans.value.filter(o => o.path !== deleteTarget.value?.path)
      message.success(t('orphan.deleted'))
      deleteModalVisible.value = false
    } else {
      message.error(data.message || 'Delete failed')
    }
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  fetchOrphans()
  loadSettings()
  loadScanConfigs()
  loadAvailableClients()
})
</script>
