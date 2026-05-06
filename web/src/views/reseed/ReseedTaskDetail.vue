<template>
  <div>
    <a-page-header :title="`辅种任务 #${taskId}`" @back="$router.push('/reseed')">
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

      <a-tabs v-model:activeKey="activeTab">
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
                  @click="retryMatch(record.id)"
                  :disabled="record.status !== 'failed'"
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

const loading = ref(false)
const matchesLoading = ref(false)
const task = ref<any>({})
const matches = ref<any[]>([])
const activeTab = ref('matches')

const negDeleteInfoHash = ref('')
const negDeleteSite = ref('')

const matchColumns = [
  { title: '源 InfoHash', dataIndex: 'source_info_hash', key: 'source_info_hash', ellipsis: true },
  { title: '目标站点', dataIndex: 'target_site', key: 'target_site', width: 120 },
  { title: '目标 InfoHash', dataIndex: 'target_info_hash', key: 'target_info_hash', ellipsis: true },
  { title: '匹配方法', dataIndex: 'match_method', key: 'match_method', width: 100 },
  { title: '置信度', dataIndex: 'confidence', key: 'confidence', width: 80 },
  { title: '状态', key: 'status', width: 100 },
  { title: '失败原因', dataIndex: 'fail_reason', key: 'fail_reason', ellipsis: true },
  { title: '操作', key: 'actions', width: 80 },
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
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function fetchMatches() {
  matchesLoading.value = true
  try {
    const resp = await reseedApi.getMatches(taskId)
    matches.value = resp.data.data || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    matchesLoading.value = false
  }
}

async function retryMatch(matchId: number) {
  try {
    await reseedApi.retryMatch(taskId, matchId)
    message.success(t('reseed.retryTriggered'))
    fetchMatches()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function deleteNegativeCache() {
  if (!negDeleteInfoHash.value) return
  try {
    await reseedApi.deleteNegativeCache(taskId, negDeleteInfoHash.value, negDeleteSite.value || undefined)
    message.success(t('common.deleted'))
    negDeleteInfoHash.value = ''
    negDeleteSite.value = ''
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => {
  fetchTask()
  fetchMatches()
})
</script>
