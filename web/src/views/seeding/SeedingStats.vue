<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic title="总上传量" :value="formatSize(overview.totalUploadBytes || 0)" :value-style="{ color: '#52c41a' }">
            <template #prefix><CloudUploadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="总下载量" :value="formatSize(overview.totalDownloadBytes || 0)" :value-style="{ color: '#1890ff' }">
            <template #prefix><CloudDownloadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="今日新增" :value="overview.todayAdded || 0" :value-style="{ color: '#faad14' }">
            <template #prefix><ClockCircleOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="全局分享率" :value="overview.globalRatio || 0" :precision="2" :value-style="{ color: '#722ed1' }">
            <template #prefix><PieChartOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <a-card title="站点统计" style="margin-bottom: 24px">
      <a-table
        :columns="siteColumns"
        :data-source="siteStats"
        :loading="siteLoading"
        :pagination="false"
        row-key="siteName"
        size="small"
      />
    </a-card>

    <a-card title="种子排行">
      <a-table
        :columns="torrentColumns"
        :data-source="torrentStats"
        :loading="torrentLoading"
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
import {
  CloudUploadOutlined,
  CloudDownloadOutlined,
  ClockCircleOutlined,
  PieChartOutlined,
} from '@ant-design/icons-vue'
import { seedingApi } from '@/api/seeding'

const siteLoading = ref(false)
const torrentLoading = ref(false)
const overview = ref<any>({})
const siteStats = ref<any[]>([])
const torrentStats = ref<any[]>([])

const siteColumns = [
  { title: '站点', dataIndex: 'siteName', key: 'siteName' },
  { title: '做种数', dataIndex: 'count', key: 'count', width: 80 },
]

const torrentColumns = [
  { title: 'Torrent ID', dataIndex: 'torrent_id', key: 'torrent_id', ellipsis: true },
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '免费', dataIndex: 'is_free', key: 'is_free', width: 60 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
]

function formatSize(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

async function fetchAll() {
  try {
    const [overviewResp, siteResp, torrentResp] = await Promise.all([
      seedingApi.getStatsOverview(),
      seedingApi.getStatsBySite(),
      seedingApi.getStatsTorrents(1, 20),
    ])
    overview.value = overviewResp.data.data || {}
    siteStats.value = siteResp.data.data?.items || siteResp.data.data || []
    torrentStats.value = torrentResp.data.data?.items || torrentResp.data.data || []
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(fetchAll)
</script>
