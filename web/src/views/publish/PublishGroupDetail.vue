<template>
  <div>
    <a-page-header :title="`发布组 #${groupId}`" @back="$router.push('/publish')">
      <template #tags>
        <a-tag :color="group.status === 'active' ? 'green' : group.status === 'completed' ? 'default' : 'orange'">
          {{ group.status }}
        </a-tag>
      </template>
      <template #extra>
        <a-space>
          <a-button @click="pauseGroup" :disabled="group.status !== 'active'">暂停</a-button>
          <a-button type="primary" @click="resumeGroup" :disabled="group.status !== 'paused'">恢复</a-button>
        </a-space>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item label="源站点">{{ group.source_site }}</a-descriptions-item>
        <a-descriptions-item label="源 Hash">{{ group.source_hash }}</a-descriptions-item>
        <a-descriptions-item label="候选 ID">{{ group.candidate_id }}</a-descriptions-item>
        <a-descriptions-item label="状态">{{ group.status }}</a-descriptions-item>
        <a-descriptions-item label="创建时间">{{ group.created_at || '-' }}</a-descriptions-item>
        <a-descriptions-item label="更新时间">{{ group.updated_at || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-card title="成员列表">
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
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { publishApi } from '@/api/publish'

const route = useRoute()
const groupId = Number(route.params.id)

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
    message.success('已暂停')
    fetchGroup()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function resumeGroup() {
  try {
    await publishApi.resumeGroup(groupId)
    message.success('已恢复')
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
