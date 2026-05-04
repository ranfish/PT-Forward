<template>
  <div>
    <a-tabs v-model:activeKey="activeTab">
      <a-tab-pane key="candidates" tab="候选列表">
        <div style="margin-bottom: 16px; display: flex; justify-content: space-between">
          <a-space>
            <a-input-search
              v-model:value="candidateSearch"
              placeholder="搜索标题"
              style="width: 300px"
              @search="fetchCandidates"
            />
          </a-space>
        </div>

        <a-table
          :columns="candidateColumns"
          :data-source="candidates"
          :loading="candidatesLoading"
          :pagination="{ current: candidatePage, pageSize: 20, total: candidateTotal, showSizeChanger: true, showTotal: (t: number) => `共 ${t} 条` }"
          row-key="id"
          size="small"
          @change="(pag: any) => { candidatePage = pag.current; fetchCandidates() }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'publish_status'">
              <a-tag :color="record.publish_status === 'pending' ? 'blue' : record.publish_status === 'completed' ? 'green' : record.publish_status === 'failed' ? 'red' : 'default'">
                {{ record.publish_status }}
              </a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-space>
                <a-button type="link" size="small" @click="manualPublish(record.id)" :disabled="record.publish_status === 'completed'">
                  发布
                </a-button>
                <a-popconfirm title="确定删除？" @confirm="deleteCandidate(record.id)">
                  <a-button type="link" danger size="small">删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="groups" tab="发布组">
        <a-table
          :columns="groupColumns"
          :data-source="groups"
          :loading="groupsLoading"
          :pagination="false"
          row-key="id"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="record.status === 'active' ? 'green' : record.status === 'completed' ? 'default' : 'orange'">
                {{ record.status }}
              </a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-space>
                <a-button type="link" size="small" @click="$router.push(`/publish/groups/${record.id}`)">详情</a-button>
                <a-popconfirm title="确定删除？" @confirm="deleteGroup(record.id)">
                  <a-button type="link" danger size="small">删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { publishApi } from '@/api/publish'

const activeTab = ref('candidates')
const candidateSearch = ref('')
const candidatesLoading = ref(false)
const candidates = ref<any[]>([])
const candidatePage = ref(1)
const candidateTotal = ref(0)

const groupsLoading = ref(false)
const groups = ref<any[]>([])

const candidateColumns = [
  { title: '种子名称', dataIndex: 'torrent_name', key: 'torrent_name', ellipsis: true },
  { title: '源站点', dataIndex: 'source_site', key: 'source_site', width: 120 },
  { title: '大小', dataIndex: 'size', key: 'size', width: 100 },
  { title: '发布状态', key: 'publish_status', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: '操作', key: 'actions', width: 120 },
]

const groupColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: '源站点', dataIndex: 'source_site', key: 'source_site', width: 120 },
  { title: '源 Hash', dataIndex: 'source_hash', key: 'source_hash', ellipsis: true },
  { title: '状态', key: 'status', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: '操作', key: 'actions', width: 120 },
]

async function fetchCandidates() {
  candidatesLoading.value = true
  try {
    const resp = await publishApi.listCandidates({ page: candidatePage.value, size: 20, search: candidateSearch.value || undefined })
    const body = resp.data.data
    candidates.value = body?.items || body || []
    candidateTotal.value = body?.total || 0
  } catch (e: any) {
    message.error(e.message)
  } finally {
    candidatesLoading.value = false
  }
}

async function fetchGroups() {
  groupsLoading.value = true
  try {
    const resp = await publishApi.listGroups()
    groups.value = resp.data.data || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    groupsLoading.value = false
  }
}

async function manualPublish(id: number) {
  try {
    await publishApi.manualPublish(id)
    message.success('已触发发布')
    fetchCandidates()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function deleteCandidate(id: number) {
  try {
    await publishApi.deleteCandidate(id)
    message.success('已删除')
    fetchCandidates()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function deleteGroup(id: number) {
  try {
    await publishApi.deleteGroup(id)
    message.success('已删除')
    fetchGroups()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => {
  fetchCandidates()
  fetchGroups()
})
</script>
