<template>
  <div>
    <a-page-header :title="t('reseed.taskDetail', { id: taskId })" @back="$router.push('/reseed')">
      <template #tags>
        <a-tag :color="statusColor(task.status)">{{ task.status }}</a-tag>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('common.name')">{{ task.name }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.status')">{{ task.status }}</a-descriptions-item>
        <a-descriptions-item :label="t('reseed.sourceSite')">{{ task.source_site_ids }}</a-descriptions-item>
        <a-descriptions-item :label="t('reseed.targetSite')">{{ task.target_site_ids }}</a-descriptions-item>
        <a-descriptions-item :label="t('reseed.client')">{{ task.client_ids }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ task.created_at || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-tabs v-model:active-key="activeTab">
        <a-tab-pane key="matches" :tab="t('reseed.matchResults')">
          <a-table
            :columns="matchColumns"
            :data-source="matches"
            :loading="matchesLoading"
            :pagination="{ pageSize: 20 }"
            row-key="id"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'status'">
                <a-tag :color="record.status === 'injected' ? 'green' : record.status === 'failed' ? 'red' : 'blue'">
                  {{ record.status }}
                </a-tag>
              </template>
              <template v-if="column.key === 'actions'">
                <a-button
                  type="link"
                  size="small"
                  :disabled="record.status !== 'failed'"
                  @click="retryMatch(record.id)"
                >
                  {{ t('reseed.retry') }}
                </a-button>
              </template>
            </template>
          </a-table>
        </a-tab-pane>
        <a-tab-pane key="negative" :tab="t('reseed.negativeCache')">
          <div style="margin-bottom: 16px; display: flex; gap: 12px; align-items: center">
            <a-input v-model:value="negDeleteInfoHash" placeholder="InfoHash" style="width: 320px" />
            <a-input v-model:value="negDeleteSite" :placeholder="t('reseed.siteOptional')" style="width: 200px" />
            <a-popconfirm :title="t('reseed.deleteNegativeCacheConfirm')" @confirm="deleteNegativeCache">
              <a-button type="primary" danger :disabled="!negDeleteInfoHash">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </div>
          <a-empty :description="t('reseed.deleteNegativeCacheDesc')" />
        </a-tab-pane>
      </a-tabs>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { reseedApi } from '@/api/reseed'

const route = useRoute()
const taskId = Number(route.params.id)
const { t } = useI18n()

interface ReseedTaskInfo {
  name: string
  status: string
  source_site_ids: string
  target_site_ids: string
  client_ids: string
  created_at: string
}

interface ReseedMatchItem {
  id: number
  source_info_hash: string
  target_site: string
  target_info_hash: string
  match_method: string
  confidence: number
  status: string
  fail_reason: string
}

const loading = ref(false)
const matchesLoading = ref(false)
const task = ref<ReseedTaskInfo>({} as ReseedTaskInfo)
const matches = ref<ReseedMatchItem[]>([])
const activeTab = ref('matches')

const negDeleteInfoHash = ref('')
const negDeleteSite = ref('')

const matchColumns = [
  { title: t('reseed.sourceInfoHash'), dataIndex: 'source_info_hash', key: 'source_info_hash', ellipsis: true },
  { title: t('reseed.targetSite'), dataIndex: 'target_site', key: 'target_site', width: 120 },
  { title: t('reseed.targetInfoHash'), dataIndex: 'target_info_hash', key: 'target_info_hash', ellipsis: true },
  { title: t('reseed.matchMethod'), dataIndex: 'match_method', key: 'match_method', width: 100 },
  { title: t('reseed.confidence'), dataIndex: 'confidence', key: 'confidence', width: 80 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('reseed.failReason'), dataIndex: 'fail_reason', key: 'fail_reason', ellipsis: true },
  { title: t('common.actions'), key: 'actions', width: 80 },
]

function statusColor(status: string) {
  const map: Record<string, string> = { idle: 'blue', running: 'green', completed: 'default', failed: 'red' }
  return map[status] || 'default'
}

async function fetchTask() {
  loading.value = true
  try {
    const resp = await reseedApi.getTask(taskId)
    task.value = resp.data.data || {}
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function fetchMatches() {
  matchesLoading.value = true
  try {
    const resp = await reseedApi.getMatches(taskId)
    matches.value = resp.data.data || []
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    matchesLoading.value = false
  }
}

async function retryMatch(matchId: number) {
  try {
    await reseedApi.retryMatch(taskId, matchId)
    message.success(t('reseed.retryTriggered'))
    fetchMatches()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function deleteNegativeCache() {
  if (!negDeleteInfoHash.value) return
  try {
    await reseedApi.deleteNegativeCache(taskId, negDeleteInfoHash.value, negDeleteSite.value || undefined)
    message.success(t('common.deleted'))
    negDeleteInfoHash.value = ''
    negDeleteSite.value = ''
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

onMounted(() => {
  fetchTask()
  fetchMatches()
})
</script>
