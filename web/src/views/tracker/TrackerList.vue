<template>
  <div>
    <a-card :title="t('tracker.title')">
      <a-table
        :columns="memberColumns"
        :data-source="members"
        :loading="loading"
        :pagination="{ pageSize: 20, showSizeChanger: true, showTotal: (t: number) => `共 ${t} 条` }"
        row-key="id"
        size="small"
        @change="(pag: any) => { page = pag.current; fetchMembers() }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <a-tag :color="memberStatusColor(record.status)">{{ record.status }}</a-tag>
          </template>
          <template v-if="column.key === 'paused'">
            <a-tag :color="record.paused ? 'orange' : 'green'">{{ record.paused ? t('tracker.pausedStatus') : t('tracker.active') }}</a-tag>
          </template>
          <template v-if="column.key === 'info_hash'">
            <a-button type="link" size="small" @click="viewMember(record.info_hash)">{{ record.info_hash?.substring(0, 16) }}...</a-button>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-card :title="t('tracker.statusHistory')" style="margin-top: 16px">
      <a-table
        :columns="historyColumns"
        :data-source="histories"
        :loading="historyLoading"
        :pagination="{ pageSize: 20 }"
        row-key="id"
        size="small"
      />
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { trackerApi } from '@/api/tracker'

const { t } = useI18n()

const loading = ref(false)
const historyLoading = ref(false)
const members = ref<any[]>([])
const histories = ref<any[]>([])
const page = ref(1)

const memberColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: 'InfoHash', key: 'info_hash', width: 180 },
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: '大小', dataIndex: 'size', key: 'size', width: 100 },
  { title: '角色', dataIndex: 'role', key: 'role', width: 80 },
  { title: '状态', key: 'status', width: 100 },
  { title: '暂停', key: 'paused', width: 80 },
  { title: '做种数', dataIndex: 'seeders', key: 'seeders', width: 80 },
  { title: '下载数', dataIndex: 'leechers', key: 'leechers', width: 80 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
]

const historyColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: '组 ID', dataIndex: 'publish_group_id', key: 'publish_group_id', width: 80 },
  { title: '成员 Hash', dataIndex: 'member_hash', key: 'member_hash', ellipsis: true },
  { title: '旧状态', dataIndex: 'old_status', key: 'old_status', width: 100 },
  { title: '新状态', dataIndex: 'new_status', key: 'new_status', width: 100 },
  { title: '原因', dataIndex: 'reason', key: 'reason', ellipsis: true },
  { title: '时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
]

function memberStatusColor(status: string) {
  const map: Record<string, string> = { new: 'blue', completed: 'green', failed: 'red', active: 'cyan' }
  return map[status] || 'default'
}

async function fetchMembers() {
  loading.value = true
  try {
    const resp = await trackerApi.listMembers()
    const body = resp.data.data
    members.value = body?.items || body || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function fetchHistory() {
  historyLoading.value = true
  try {
    const resp = await trackerApi.getHistory()
    const body = resp.data.data
    histories.value = body?.items || body || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    historyLoading.value = false
  }
}

async function viewMember(hash: string) {
  if (!hash) return
  try {
    const resp = await trackerApi.getHistory()
    const body = resp.data.data
    histories.value = body?.items || body || []
    message.info(`查看 InfoHash: ${hash.substring(0, 16)}...`)
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => {
  fetchMembers()
  fetchHistory()
})
</script>
