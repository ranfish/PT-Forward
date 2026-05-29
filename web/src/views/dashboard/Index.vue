<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic
            :title="t('dashboard.sitesOnline')"
            :value="`${overview.sites?.online || 0} / ${overview.sites?.total || 0}`"
            :value-style="{ color: '#1890ff' }"
          >
            <template #prefix><GlobalOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic
            :title="t('dashboard.downloadersOnline')"
            :value="`${overview.downloaders?.online || 0} / ${overview.downloaders?.total || 0}`"
            :value-style="{ color: '#52c41a' }"
          >
            <template #prefix><CloudDownloadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic
            :title="t('dashboard.seedingTorrents')"
            :value="overview.torrents?.seeding || 0"
            :value-style="{ color: '#faad14' }"
          >
            <template #prefix><ThunderboltOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic
            :title="t('dashboard.pendingPublish')"
            :value="overview.publish?.pendingCount || 0"
            :value-style="{ color: '#722ed1' }"
          >
            <template #prefix><SendOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('dashboard.reseedTodayTotal')" :value="`${overview.reseed?.todayCount || 0} / ${overview.reseed?.totalCount || 0}`" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('dashboard.publishTodayTotal')" :value="`${overview.publish?.todayCount || 0} / ${overview.publish?.totalCount || 0}`" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('dashboard.downloadingTorrents')" :value="overview.torrents?.downloading || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('dashboard.goroutines')" :value="overview.system?.goroutines || 0" />
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('dashboard.trends7d')" style="margin-bottom: 24px">
      <div ref="chartRef" style="height: 320px; width: 100%" />
    </a-card>

    <a-card :title="t('dashboard.recentActivity')">
      <a-table
        :columns="activityColumns"
        :data-source="activities"
        :loading="loading"
        :pagination="{
          current: activityPage,
          pageSize: activitySize,
          total: activityTotal,
          showSizeChanger: true,
          showTotal: (total: number) => t('common.totalCount', { count: total }),
        }"
        row-key="id"
        size="small"
        @change="(pag: { current: number; pageSize: number }) => { activityPage = pag.current; activitySize = pag.pageSize; fetchActivities() }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <a v-if="record.detail_url" :href="record.detail_url" target="_blank" style="color: #1890ff">{{ record.title }}</a>
            <span v-else>{{ record.title }}</span>
          </template>
          <template v-if="column.key === 'info_hash'">
            <span v-if="/^[0-9a-f]{40}$/i.test(record.info_hash)">{{ record.info_hash }}</span>
            <span v-else style="color: #999">-</span>
          </template>
          <template v-if="column.key === 'size'">
            {{ formatSize(record.size) }}
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import * as echarts from 'echarts'
import {
  GlobalOutlined,
  CloudDownloadOutlined,
  ThunderboltOutlined,
  SendOutlined,
} from '@ant-design/icons-vue'
import { dashboardApi, type TrendPoint } from '@/api/dashboard'
import { useWebSocketStore } from '@/stores/websocket'
import { formatTime } from '@/utils/format'

interface ActivityItem {
  id: number
  title: string
  site_name: string
  info_hash: string
  detail_url: string
  size: number
  status: string
  created_at: string
}

interface DashboardOverview {
  sites?: { online?: number; total?: number }
  downloaders?: { online?: number; total?: number }
  torrents?: { seeding?: number; downloading?: number }
  publish?: { pendingCount?: number; todayCount?: number; totalCount?: number }
  reseed?: { todayCount?: number; totalCount?: number }
  system?: { goroutines?: number }
}

interface WSMessage {
  type?: string
  payload?: Record<string, unknown>
  timestamp?: string
}

const { t } = useI18n()
const loading = ref(false)
const overview = ref<DashboardOverview>({})
const activities = ref<ActivityItem[]>([])
const activityPage = ref(1)
const activitySize = ref(20)
const activityTotal = ref(0)
const chartRef = ref<HTMLElement>()
let chartInstance: echarts.ECharts | null = null
let resizeTimer: ReturnType<typeof setTimeout> | null = null
const wsStore = useWebSocketStore()

const activityColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('dashboard.torrentTitle'), dataIndex: 'title', key: 'title', ellipsis: true, width: 220 },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 80 },
  { title: '种子ID', dataIndex: 'torrent_id', key: 'torrent_id', width: 100 },
  { title: 'InfoHash', key: 'info_hash', width: 280 },
  { title: t('common.size'), key: 'size', width: 100 },
  { title: t('common.status'), dataIndex: 'status', key: 'status', width: 100 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
]

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(2) + ' ' + units[i]
}

function initChart(points: TrendPoint[]) {
  if (!chartRef.value) return
  if (chartInstance) {
    chartInstance.dispose()
  }
  chartInstance = echarts.init(chartRef.value)
  const dates = points.map(p => p.date)
  chartInstance.setOption({
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross', label: { backgroundColor: '#6a7985' } },
    },
    legend: { data: [t('dashboard.chartEvents'), t('dashboard.chartRSS'), t('dashboard.chartPublish'), t('dashboard.chartReseed')], bottom: 0 },
    grid: { left: 50, right: 30, top: 20, bottom: 40 },
    xAxis: { type: 'category', data: dates, boundaryGap: true },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      {
        name: t('dashboard.chartEvents'), type: 'line', smooth: true,
        data: points.map(p => p.events),
        itemStyle: { color: '#1890ff' },
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.1 },
      },
      {
        name: t('dashboard.chartRSS'), type: 'line', smooth: true,
        data: points.map(p => p.rss),
        itemStyle: { color: '#52c41a' },
        lineStyle: { width: 2 },
      },
      {
        name: t('dashboard.chartPublish'), type: 'line', smooth: true,
        data: points.map(p => p.publish),
        itemStyle: { color: '#722ed1' },
        lineStyle: { width: 2 },
      },
      {
        name: t('dashboard.chartReseed'), type: 'line', smooth: true,
        data: points.map(p => p.reseed),
        itemStyle: { color: '#faad14' },
        lineStyle: { width: 2 },
      },
    ],
  })
}

function handleResize() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    chartInstance?.resize()
  }, 200)
}

function handleWSMessage(msg: WSMessage) {
  if (!msg || !msg.type) return
  if (msg.type === 'torrent.added') {
    const p = msg.payload || {}
    activities.value.unshift({
      id: (p.eventId as number) || Date.now(),
      title: (p.title as string) || '',
      site_name: (p.siteName as string) || '',
      info_hash: '',
      detail_url: '',
      size: typeof p.size === 'number' ? p.size : 0,
      status: (p.discount as string) || '',
      created_at: msg.timestamp || new Date().toISOString(),
    })
    if (activities.value.length > activitySize.value) {
      activities.value = activities.value.slice(0, activitySize.value)
    }
  }
  if (msg.type === 'system.site.frozen') {
    fetchData()
  }
}

async function fetchActivities() {
  try {
    const resp = await dashboardApi.getActivities(activityPage.value, activitySize.value)
    const body = resp.data.data || { items: [], total: 0 }
    activities.value = (body.items || []) as ActivityItem[]
    activityTotal.value = body.total || 0
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function fetchData() {
  loading.value = true
  try {
    const [overviewResp, trendsResp] = await Promise.all([
      dashboardApi.getOverview(),
      dashboardApi.getTrends(7),
    ])
    overview.value = overviewResp.data.data || {}
    const trendData = trendsResp.data.data || { trends: [] }
    await nextTick()
    initChart(trendData.trends || [])
    await fetchActivities()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
  window.addEventListener('resize', handleResize)
  wsStore.subscribe(['dashboard', 'torrent', 'system'])
})

watch(() => wsStore.lastMessage, (msg) => {
  if (msg) handleWSMessage(msg)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (resizeTimer) clearTimeout(resizeTimer)
  chartInstance?.dispose()
  wsStore.unsubscribe(['dashboard', 'torrent', 'system'])
})
</script>
