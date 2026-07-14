<template>
  <div>
    <a-card title="评分日志">
      <a-table
        :columns="columns"
        :data-source="logs"
        :loading="loading"
        :pagination="{
          current: page,
          pageSize: pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t: number) => `共 ${t} 条`,
          size: 'small',
        }"
        row-key="id"
        size="small"
        @change="(p: any) => { page = p.current; pageSize = p.pageSize; fetchData() }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'subscription_id'">
            <a-tag>{{ record.subscription_id }}</a-tag>
          </template>
          <template v-if="column.key === 'effective_score'">
            <a-tag :color="(record.effective_score || 0) >= 60 ? 'green' : 'red'">
              {{ record.effective_score ?? '-' }}
            </a-tag>
          </template>
          <template v-if="column.key === 'created_at'">
            {{ formatTime(record.created_at) }}
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { seedingApi } from '@/api/seeding'
import { formatTime } from '@/utils/format'

const loading = ref(false)
const logs = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const columns = [
  { title: '订阅', key: 'subscription_id', width: 80 },
  { title: '种子ID', dataIndex: 'torrent_id', key: 'torrent_id', width: 100, ellipsis: true },
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 80 },
  { title: '评分', key: 'effective_score', width: 70 },
  { title: '决策', dataIndex: 'decision', key: 'decision', width: 80 },
  { title: '原因', dataIndex: 'reason', key: 'reason', ellipsis: true },
  { title: '时间', key: 'created_at', width: 150 },
]

async function fetchData() {
  loading.value = true
  try {
    const { data } = await seedingApi.listScoringLogs({
      page: page.value,
      pageSize: pageSize.value,
    })
    logs.value = data.data?.items || []
    total.value = data.data?.total || 0
  } catch {
    message.error('加载评分日志失败')
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>
