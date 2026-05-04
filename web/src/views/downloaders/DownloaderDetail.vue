<template>
  <div>
    <a-page-header :title="downloader.name || '下载器详情'" @back="$router.push('/downloaders')">
      <template #tags>
        <a-badge
          :status="downloader.enabled ? 'success' : 'default'"
          :text="downloader.enabled ? '已启用' : '已禁用'"
        />
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item label="名称">{{ downloader.name }}</a-descriptions-item>
        <a-descriptions-item label="类型">{{ downloader.type }}</a-descriptions-item>
        <a-descriptions-item label="地址">{{ downloader.url }}</a-descriptions-item>
        <a-descriptions-item label="状态">{{ downloader.enabled ? '已启用' : '已禁用' }}</a-descriptions-item>
        <a-descriptions-item label="用户名">{{ downloader.username || '-' }}</a-descriptions-item>
        <a-descriptions-item label="创建时间">{{ downloader.created_at || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-tabs v-model:activeKey="activeTab" @change="onTabChange">
        <a-tab-pane key="torrents" tab="种子列表">
          <a-table
            :columns="torrentColumns"
            :data-source="torrents"
            :loading="torrentsLoading"
            :pagination="{ pageSize: 20, showSizeChanger: true }"
            row-key="hash"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'total_size'">
                {{ formatSize(record.total_size) }}
              </template>
              <template v-if="column.key === 'progress'">
                {{ (record.progress * 100).toFixed(1) }}%
              </template>
              <template v-if="column.key === 'upload_speed'">
                {{ formatSpeed(record.upload_speed) }}
              </template>
              <template v-if="column.key === 'download_speed'">
                {{ formatSpeed(record.download_speed) }}
              </template>
            </template>
          </a-table>
        </a-tab-pane>
        <a-tab-pane key="maindata" tab="主数据">
          <a-descriptions bordered :column="2" v-if="maindata">
            <a-descriptions-item label="种子数">{{ Object.keys(maindata.torrents || {}).length }}</a-descriptions-item>
            <a-descriptions-item label="可用空间">{{ formatSize(maindata.free_space) }}</a-descriptions-item>
            <a-descriptions-item label="分类数">{{ Object.keys(maindata.categories || {}).length }}</a-descriptions-item>
            <a-descriptions-item label="标签数">{{ (maindata.tags || []).length }}</a-descriptions-item>
          </a-descriptions>
          <a-empty v-else description="暂无数据" />
        </a-tab-pane>
      </a-tabs>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { downloadersApi } from '@/api/downloaders'

const route = useRoute()
const id = Number(route.params.id)

const loading = ref(false)
const torrentsLoading = ref(false)
const downloader = ref<any>({})
const torrents = ref<any[]>([])
const maindata = ref<any>(null)
const activeTab = ref('torrents')

const torrentColumns = [
  { title: '名称', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '大小', key: 'total_size', width: 100 },
  { title: '进度', key: 'progress', width: 100 },
  { title: '状态', dataIndex: 'state', key: 'state', width: 100 },
  { title: '上传速度', key: 'upload_speed', width: 120 },
  { title: '下载速度', key: 'download_speed', width: 120 },
]

function formatSpeed(bytes: number) {
  if (!bytes) return '0 B/s'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

function formatSize(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

async function fetchDownloader() {
  loading.value = true
  try {
    const resp = await downloadersApi.get(id)
    downloader.value = resp.data.data || {}
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function fetchTorrents() {
  torrentsLoading.value = true
  try {
    const resp = await downloadersApi.getTorrents(id)
    const body = resp.data.data
    torrents.value = body?.items || body || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    torrentsLoading.value = false
  }
}

async function fetchMaindata() {
  try {
    const resp = await downloadersApi.getMaindata(id)
    maindata.value = resp.data.data || null
  } catch (e: any) {
    message.error(e.message)
  }
}

function onTabChange(key: string) {
  if (key === 'torrents' && torrents.value.length === 0) fetchTorrents()
  if (key === 'maindata' && !maindata.value) fetchMaindata()
}

onMounted(() => {
  fetchDownloader()
  fetchTorrents()
})
</script>
