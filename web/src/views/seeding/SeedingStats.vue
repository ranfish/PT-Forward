<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('seeding.totalUpload')" :value="formatSize(overview.totalUploadBytes || 0)" :value-style="{ color: '#52c41a' }">
            <template #prefix><CloudUploadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('seeding.totalDownload')" :value="formatSize(overview.totalDownloadBytes || 0)" :value-style="{ color: '#1890ff' }">
            <template #prefix><CloudDownloadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('seeding.todayAdded')" :value="overview.todayAdded || 0" :value-style="{ color: '#faad14' }">
            <template #prefix><ClockCircleOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('seeding.globalRatio')" :value="overview.globalRatio || 0" :precision="2" :value-style="{ color: '#722ed1' }">
            <template #prefix><PieChartOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('seeding.siteStats')" style="margin-bottom: 24px">
      <a-table
        :columns="siteColumns"
        :data-source="siteStats"
        :loading="siteLoading"
        :pagination="false"
        row-key="siteName"
        size="small"
      />
    </a-card>

    <a-card :title="t('seeding.torrentRanking')">
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
import { useI18n } from 'vue-i18n'
import {
  CloudUploadOutlined,
  CloudDownloadOutlined,
  ClockCircleOutlined,
  PieChartOutlined,
} from '@ant-design/icons-vue'
import { seedingApi } from '@/api/seeding'

interface StatsOverviewData {
  totalUploadBytes?: number
  totalDownloadBytes?: number
  todayAdded?: number
  globalRatio?: number
}

const { t } = useI18n()
const siteLoading = ref(false)
const torrentLoading = ref(false)
const overview = ref<StatsOverviewData>({})
const siteStats = ref<Record<string, unknown>[]>([])
const torrentStats = ref<Record<string, unknown>[]>([])

const siteColumns = [
  { title: t('common.site'), dataIndex: 'siteName', key: 'siteName' },
  { title: t('seeding.seedingCount'), dataIndex: 'count', key: 'count', width: 80 },
]

const torrentColumns = [
  { title: 'Torrent ID', dataIndex: 'torrent_id', key: 'torrent_id', ellipsis: true },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: t('common.status'), dataIndex: 'status', key: 'status', width: 100 },
  { title: t('seeding.free'), dataIndex: 'is_free', key: 'is_free', width: 60 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180 },
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
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

onMounted(fetchAll)
</script>
