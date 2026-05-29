<template>
  <div>
    <a-card :title="t('tracker.title')">
      <a-table
        :columns="memberColumns"
        :data-source="members"
        :loading="loading"
        :pagination="{ pageSize: 20, showSizeChanger: true, showTotal: (total: number) => t('common.totalCount', { count: total }) }"
        row-key="id"
        size="small"
        @change="(pag: { current: number }) => { page = pag.current; fetchMembers() }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <a-tag :color="memberStatusColor(record.status)">{{ record.status }}</a-tag>
          </template>
          <template v-if="column.key === 'paused'">
            <a-tag :color="record.paused ? 'orange' : 'green'">{{ record.paused ? t('tracker.pausedStatus') : t('tracker.active') }}</a-tag>
          </template>
          <template v-if="column.key === 'info_hash'">
            <span style="cursor:pointer;font-family:monospace;font-size:12px" @click="copyHash(record.info_hash)">{{ record.info_hash }}</span>
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

    <a-modal v-model:open="memberDetailVisible" :title="t('tracker.memberDetail')" width="600px" :footer="null">
      <a-spin :spinning="memberDetailLoading">
        <a-descriptions v-if="memberDetail" bordered :column="1" size="small">
          <a-descriptions-item v-for="(val, key) in memberDetail" :key="key" :label="String(key)">{{ val ?? '-' }}</a-descriptions-item>
        </a-descriptions>
      </a-spin>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { trackerApi } from '@/api/tracker'
import { formatTime, copyToClipboard } from '@/utils/format'

const { t } = useI18n()

function copyHash(text: string) {
  copyToClipboard(text)
  message.success(t('common.copied'))
}

const loading = ref(false)
const historyLoading = ref(false)
const members = ref<Record<string, unknown>[]>([])
const histories = ref<Record<string, unknown>[]>([])
const page = ref(1)
const memberDetailVisible = ref(false)
const memberDetailLoading = ref(false)
const memberDetail = ref<Record<string, unknown> | null>(null)

const memberColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: 'InfoHash', key: 'info_hash', width: 180 },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: t('common.size'), dataIndex: 'size', key: 'size', width: 100 },
  { title: t('tracker.role'), dataIndex: 'role', key: 'role', width: 80 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('tracker.pausedStatus'), key: 'paused', width: 80 },
  { title: t('tracker.seeders'), dataIndex: 'seeders', key: 'seeders', width: 80 },
  { title: t('tracker.leechers'), dataIndex: 'leechers', key: 'leechers', width: 80 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
]

const historyColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('tracker.groupId'), dataIndex: 'publish_group_id', key: 'publish_group_id', width: 80 },
  { title: t('tracker.memberHash'), dataIndex: 'member_hash', key: 'member_hash', ellipsis: true },
  { title: t('tracker.oldStatus'), dataIndex: 'old_status', key: 'old_status', width: 100 },
  { title: t('tracker.newStatus'), dataIndex: 'new_status', key: 'new_status', width: 100 },
  { title: t('tracker.reason'), dataIndex: 'reason', key: 'reason', ellipsis: true },
  { title: t('tracker.colTime'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
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
    members.value = (body?.items ?? []) as Record<string, unknown>[]
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

async function fetchHistory() {
  historyLoading.value = true
  try {
    const resp = await trackerApi.getHistory()
    const body = resp.data.data
    histories.value = (body?.items ?? []) as Record<string, unknown>[]
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    historyLoading.value = false
  }
}

async function viewMember(hash: string) {
  if (!hash) return
  memberDetailVisible.value = true
  memberDetailLoading.value = true
  memberDetail.value = null
  try {
    const resp = await trackerApi.getMember(hash)
    memberDetail.value = (resp.data.data || null) as Record<string, unknown> | null
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    memberDetailLoading.value = false
  }
}

onMounted(() => {
  fetchMembers()
  fetchHistory()
})
</script>
