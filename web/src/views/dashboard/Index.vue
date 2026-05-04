<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic
            title="站点 (在线/总数)"
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
            title="下载器 (在线/总数)"
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
            title="做种中"
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
            title="待发布"
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
          <a-statistic title="辅种今日/累计" :value="`${overview.reseed?.todayCount || 0} / ${overview.reseed?.totalCount || 0}`" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="发布今日/累计" :value="`${overview.publish?.todayCount || 0} / ${overview.publish?.totalCount || 0}`" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="下载中种子" :value="overview.torrents?.downloading || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="Goroutines" :value="overview.system?.goroutines || 0" />
        </a-card>
      </a-col>
    </a-row>

    <a-card title="近 7 天趋势" style="margin-bottom: 24px">
      <div ref="chartRef" style="height: 320px; width: 100%"></div>
    </a-card>

    <a-card title="最近活动">
      <a-table
        :columns="activityColumns"
        :data-source="activities"
        :loading="loading"
        :pagination="false"
        row-key="id"
        size="small"
      />
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { message } from 'ant-design-vue'
import * as echarts from 'echarts'
import {
  GlobalOutlined,
  CloudDownloadOutlined,
  ThunderboltOutlined,
  SendOutlined,
} from '@ant-design/icons-vue'
import { dashboardApi } from '@/api/dashboard'
import { useWebSocketStore } from '@/stores/websocket'

const loading = ref(false)
const overview = ref<any>({})
const activities = ref<any[]>([])
const chartRef = ref<HTMLElement>()
let chartInstance: echarts.ECharts | null = null
let resizeTimer: ReturnType<typeof setTimeout> | null = null
const wsStore = useWebSocketStore()

const activityColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: '种子标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: 'InfoHash', dataIndex: 'info_hash', key: 'info_hash', ellipsis: true },
  { title: '大小', dataIndex: 'size', key: 'size', width: 100 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
]

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(2) + ' ' + units[i]
}

function initChart(trends: any[]) {
  if (!chartRef.value) return
  if (chartInstance) {
    chartInstance.dispose()
  }
  chartInstance = echarts.init(chartRef.value)
  const dates = trends.map((t: any) => t.date)
  chartInstance.setOption({
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross', label: { backgroundColor: '#6a7985' } },
    },
    legend: { data: ['事件', 'RSS', '发布', '辅种'], bottom: 0 },
    grid: { left: 50, right: 30, top: 20, bottom: 40 },
    xAxis: { type: 'category', data: dates, boundaryGap: true },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      {
        name: '事件', type: 'bar', stack: 'total',
        data: trends.map((t: any) => t.events),
        itemStyle: { color: '#1890ff' },
        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(24,144,255,0.5)' } },
      },
      {
        name: 'RSS', type: 'bar', stack: 'total',
        data: trends.map((t: any) => t.rss),
        itemStyle: { color: '#52c41a' },
        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(82,196,26,0.5)' } },
      },
      {
        name: '发布', type: 'line', smooth: true,
        data: trends.map((t: any) => t.publish),
        itemStyle: { color: '#722ed1' },
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.1 },
      },
      {
        name: '辅种', type: 'line', smooth: true,
        data: trends.map((t: any) => t.reseed),
        itemStyle: { color: '#faad14' },
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.1 },
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

function handleWSMessage(msg: any) {
  if (!msg || !msg.type) return
  if (msg.type === 'torrent.added') {
    const p = msg.payload || {}
    activities.value.unshift({
      id: p.eventId || Date.now(),
      title: p.title || '',
      site_name: p.siteName || '',
      info_hash: '',
      size: typeof p.size === 'number' ? formatSize(p.size) : p.size || '',
      status: p.discount || '',
      created_at: msg.timestamp || new Date().toISOString(),
    })
    if (activities.value.length > 20) {
      activities.value = activities.value.slice(0, 20)
    }
  }
  if (msg.type === 'system.site.frozen') {
    fetchData()
  }
}

async function fetchData() {
  loading.value = true
  try {
    const [overviewResp, activityResp, trendsResp] = await Promise.all([
      dashboardApi.getOverview(),
      dashboardApi.getActivities(1, 20),
      dashboardApi.getTrends(7),
    ])
    overview.value = overviewResp.data.data || {}
    activities.value = activityResp.data.data?.items || activityResp.data.data || []
    const trends = trendsResp.data.data?.trends || []
    await nextTick()
    initChart(trends)
  } catch (e: any) {
    message.error(e.message)
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
