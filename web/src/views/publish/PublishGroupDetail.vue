<template>
  <div>
    <a-page-header :title="`发布组 #${groupId}`" @back="$router.push('/publish')">
      <template #tags>
        <a-tag :color="group.status === 'active' || group.status === 'monitoring' ? 'green' : group.status === 'deleted' ? 'default' : group.status === 'publishing' ? 'blue' : group.status === 'publish_failed' ? 'red' : 'orange'">
          {{ group.status }}
        </a-tag>
      </template>
      <template #extra>
        <a-space>
          <a-button @click="pauseGroup" :disabled="group.status !== 'active' && group.status !== 'publishing' && group.status !== 'monitoring'">{{ t('common.pause') }}</a-button>
          <a-button type="primary" @click="resumeGroup" :disabled="group.status !== 'partially_paused' && group.status !== 'all_paused'">{{ t('common.resume') }}</a-button>
          <a-popconfirm :title="t('common.deleteConfirm')" @confirm="lifecycleDeleteGroup">
            <a-button danger :disabled="group.status === 'deleting'">{{ t('common.delete') }}</a-button>
          </a-popconfirm>
        </a-space>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('publish.sourceSite')">{{ group.sourceSite }}</a-descriptions-item>
        <a-descriptions-item :label="t('publish.sourceHash')">{{ group.sourceHash }}</a-descriptions-item>
        <a-descriptions-item :label="t('publish.candidateId')">{{ group.candidateId || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.status')">{{ group.status }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ group.createdAt || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.updatedAt')">{{ group.updatedAt || '-' }}</a-descriptions-item>
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
            <template v-if="column.key === 'status'">
              <a-tag :color="record.status === 'completed' ? 'green' : record.status === 'failed' ? 'red' : 'blue'">
                {{ record.status }}
              </a-tag>
            </template>
          </template>
        </a-table>
      </a-card>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { publishApi } from '@/api/publish'

const route = useRoute()
const groupId = Number(route.params.id)
const { t } = useI18n()

const loading = ref(false)
const membersLoading = ref(false)
const group = ref<any>({})
const members = ref<any[]>([])

const memberColumns = [
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: 'InfoHash', dataIndex: 'info_hash', key: 'info_hash', ellipsis: true },
  { title: '大小', dataIndex: 'size', key: 'size', width: 100 },
  { title: '角色', dataIndex: 'role', key: 'role', width: 80 },
  { title: '状态', key: 'status', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
]

async function fetchGroup() {
  loading.value = true
  try {
    const resp = await publishApi.getGroup(groupId)
    group.value = resp.data.data || {}
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function fetchMembers() {
  membersLoading.value = true
  try {
    const resp = await publishApi.listCandidates({ groupId })
    members.value = resp.data.data?.items || resp.data.data || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    membersLoading.value = false
  }
}

async function pauseGroup() {
  try {
    await publishApi.pauseGroup(groupId)
    message.success(t('common.paused'))
    fetchGroup()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function resumeGroup() {
  try {
    await publishApi.resumeGroup(groupId)
    message.success(t('common.resumed'))
    fetchGroup()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function lifecycleDeleteGroup() {
  try {
    await publishApi.lifecycleDeleteGroup(groupId)
    message.success(t('common.deleted'))
    fetchGroup()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => {
  fetchGroup()
  fetchMembers()
})
</script>
