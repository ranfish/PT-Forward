<template>
  <div>
    <a-page-header :title="t('publish.publishGroup', { id: groupId })" @back="$router.push('/publish')">
      <template v-if="group" #tags>
        <a-tag :color="group.status === 'active' || group.status === 'monitoring' ? 'green' : group.status === 'deleted' ? 'default' : group.status === 'publishing' ? 'blue' : group.status === 'publish_failed' ? 'red' : 'orange'">
          {{ translatePublishStatus(group.status) }}
        </a-tag>
      </template>
      <template v-if="group" #extra>
        <a-space>
          <a-button :disabled="group.status !== 'active' && group.status !== 'publishing' && group.status !== 'monitoring'" @click="pauseGroup">{{ t('common.pause') }}</a-button>
          <a-button type="primary" :disabled="group.status !== 'partially_paused' && group.status !== 'all_paused'" @click="resumeGroup">{{ t('common.resume') }}</a-button>
          <a-popconfirm :title="t('common.deleteConfirm')" @confirm="lifecycleDeleteGroup">
            <a-button danger :disabled="group.status === 'deleting'">{{ t('common.delete') }}</a-button>
          </a-popconfirm>
        </a-space>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <template v-if="group">
        <a-descriptions bordered :column="2" style="margin-bottom: 24px">
          <a-descriptions-item :label="t('publish.sourceSite')">{{ group.source_site }}</a-descriptions-item>
          <a-descriptions-item :label="t('publish.sourceHash')">{{ group.source_hash }}</a-descriptions-item>
          <a-descriptions-item :label="t('publish.candidateId')">{{ group.candidate_id || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.status')">{{ translatePublishStatus(group.status) }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.createdAt')">{{ formatTime(group.created_at) }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.updatedAt')">{{ formatTime(group.updated_at) }}</a-descriptions-item>
        </a-descriptions>

        <a-card :title="t('publish.memberList')">
          <a-table
            :columns="memberColumns"
            :data-source="members"
            :loading="membersLoading"
            :pagination="{ pageSize: 20 }"
            row-key="id"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'info_hash'">
                <span style="cursor:pointer;font-family:monospace;font-size:12px" @click="copyHash(record.info_hash)">{{ record.info_hash }}</span>
              </template>
              <template v-if="column.key === 'status'">
                <a-tag :color="record.status === 'completed' ? 'green' : record.status === 'failed' ? 'red' : 'blue'">
                  {{ translatePublishStatus(record.status) }}
                </a-tag>
              </template>
            </template>
          </a-table>
        </a-card>
      </template>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { publishApi } from '@/api/publish'
import { formatTime, copyToClipboard } from '@/utils/format'
import { useEnumLabels } from '@/utils/enumLabels'
import type { PublishGroup } from '@/api/types'

const route = useRoute()
const groupId = Number(route.params.id)
const { t } = useI18n()
const { translatePublishStatus } = useEnumLabels()

function copyHash(text: string) {
  copyToClipboard(text)
  message.success(t('common.copied'))
}

interface GroupMember {
  id: number
  site_name: string
  info_hash: string
  size: number
  role: string
  status: string
  created_at: string
}

const loading = ref(false)
const membersLoading = ref(false)
const group = ref<PublishGroup | null>(null)
const members = ref<GroupMember[]>([])

const memberColumns = [
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: 'InfoHash', dataIndex: 'info_hash', key: 'info_hash', ellipsis: true },
  { title: t('common.size'), dataIndex: 'size', key: 'size', width: 100 },
  { title: t('publish.columnRole'), dataIndex: 'role', key: 'role', width: 80 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
]

async function fetchGroup() {
  loading.value = true
  try {
    const resp = await publishApi.getGroup(groupId)
    group.value = resp.data.data || null
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function fetchMembers() {
  membersLoading.value = true
  try {
    const resp = await publishApi.listCandidates({ groupId })
    members.value = (resp.data.data?.items || []) as unknown as GroupMember[]
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    membersLoading.value = false
  }
}

async function pauseGroup() {
  try {
    await publishApi.pauseGroup(groupId)
    message.success(t('common.paused'))
    fetchGroup()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function resumeGroup() {
  try {
    await publishApi.resumeGroup(groupId)
    message.success(t('common.resumed'))
    fetchGroup()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function lifecycleDeleteGroup() {
  try {
    await publishApi.lifecycleDeleteGroup(groupId)
    message.success(t('common.deleted'))
    fetchGroup()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

onMounted(() => {
  fetchGroup()
  fetchMembers()
})
</script>
