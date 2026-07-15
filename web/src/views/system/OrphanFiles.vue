<template>
  <div>
    <a-page-header :title="t('orphan.title')">
      <template #extra>
        <a-button type="primary" :loading="scanning" @click="scan">
          {{ t('orphan.scan') }}
        </a-button>
      </template>
    </a-page-header>

    <a-alert v-if="scannedAt" type="info" style="margin: 0 24px 16px" show-icon>
      {{ t('orphan.lastScan') }}: {{ formatTime(scannedAt.toISOString()) }} ·
      {{ orphans.length }} {{ t('orphan.itemsFound') }}
    </a-alert>

    <div style="padding: 0 24px">
      <a-table
        :columns="columns"
        :data-source="orphans"
        :loading="scanning"
        :pagination="{ pageSize: 20 }"
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
          <template v-if="column.key === 'action'">
            <a-button
              size="small"
              type="link"
              :loading="recovering === record.path"
              @click="recover(record)"
            >
              {{ t('orphan.recover') }}
            </a-button>
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
  client_id: string
  save_path: string
  detected_at: string
}

const orphans = ref<OrphanEntry[]>([])
const scanning = ref(false)
const scannedAt = ref<Date | null>(null)
const recovering = ref<string | null>(null)
const resultVisible = ref(false)
const recoverResult = ref<{ found: boolean; message: string } | null>(null)

const columns = [
  { title: t('orphan.columnName'), key: 'name', ellipsis: true },
  { title: t('orphan.columnType'), key: 'type', width: 80 },
  { title: t('orphan.columnSize'), key: 'size', width: 100 },
  { title: t('orphan.columnClient'), dataIndex: 'client_id', key: 'client_id', width: 120 },
  { title: t('common.action'), key: 'action', width: 100 },
]

async function fetchOrphans() {
  try {
    const resp = await fetch('/api/v1/orphans', {
      headers: { Authorization: `Bearer ${localStorage.getItem('pt-forward-access-token')}` }
    })
    const data = await resp.json()
    if (data.code === 0) {
      orphans.value = data.data.orphans || []
      if (data.data.scanned_at) {
        scannedAt.value = new Date(data.data.scanned_at)
      }
    }
  } catch {
    // ignore
  }
}

async function scan() {
  scanning.value = true
  try {
    const resp = await fetch('/api/v1/orphans/scan', {
      method: 'POST',
      headers: { Authorization: `Bearer ${localStorage.getItem('pt-forward-access-token')}` }
    })
    const data = await resp.json()
    if (data.code === 0) {
      orphans.value = data.data.orphans || []
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

async function recover(orphan: OrphanEntry) {
  recovering.value = orphan.path
  resultVisible.value = false
  message.loading(t('orphan.recovering'), 0)
  try {
    const resp = await fetch('/api/v1/orphans/recover', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${localStorage.getItem('pt-forward-access-token')}`
      },
      body: JSON.stringify({ path: orphan.path })
    })
    const data = await resp.json()
    if (data.code !== 0) {
      message.destroy()
      message.error(data.message || 'Recovery failed')
      return
    }
    const taskID = data.data.task_id
    // Poll for result
    let pollCount = 0
    const poll = async () => {
      pollCount++
      try {
        const r = await fetch(`/api/v1/orphans/recover/${taskID}`, {
          headers: { Authorization: `Bearer ${localStorage.getItem('pt-forward-access-token')}` }
        })
        const d = await r.json()
        if (d.code === 0 && d.data) {
          const msg = d.data.message || ''
          if (msg !== 'searching...' && msg !== '') {
            message.destroy()
            recoverResult.value = d.data
            resultVisible.value = true
            if (d.data.found) {
              message.success(t('orphan.recoverSuccess'))
            }
            recovering.value = null
            return
          }
        }
      } catch { /* ignore poll errors */ }
      if (pollCount < 60) {
        setTimeout(poll, 3000)
      } else {
        message.destroy()
        message.error(t('orphan.timeout'))
        recovering.value = null
      }
    }
    setTimeout(poll, 2000)
  } catch (e: unknown) {
    message.destroy()
    message.error(e instanceof Error ? e.message : String(e))
    recovering.value = null
  }
}

onMounted(() => {
  fetchOrphans()
})
</script>
