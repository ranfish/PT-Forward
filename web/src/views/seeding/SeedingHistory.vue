<template>
  <div>
    <a-card title="刷流历史">
      <template #extra>
        <a-space>
          <a-select v-model:value="filters.client_id" style="width: 140px" allow-clear placeholder="客户端" @change="fetchData">
            <a-select-option v-for="c in clients" :key="c.client_id" :value="c.client_id">{{ c.client_id }}</a-select-option>
          </a-select>
          <a-select v-model:value="filters.site_name" style="width: 120px" allow-clear placeholder="站点" @change="fetchData">
            <a-select-option v-for="s in sites" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
          <a-input-search v-model:value="filters.search" placeholder="搜索标题/Hash" style="width: 200px" @search="fetchData" allow-clear />
        </a-space>
      </template>
      <a-table
        :columns="columns"
        :data-source="records"
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
          <template v-if="column.key === 'site_name'">
            <a-tag>{{ record.site_name }}</a-tag>
          </template>
          <template v-if="column.key === 'status'">
            <a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag>
          </template>
          <template v-if="column.key === 'uploaded'">
            {{ formatBytes(record.uploaded) }}
          </template>
          <template v-if="column.key === 'ratio'">
            {{ record.ratio ? record.ratio.toFixed(2) : '-' }}
          </template>
          <template v-if="column.key === 'last_action_by'">
            <span v-if="record.last_action_by" style="font-size: 11px; color: #999">{{ record.last_action_by }}</span>
            <span v-else>-</span>
          </template>
          <template v-if="column.key === 'updated_at'">
            {{ formatTime(record.updated_at) }}
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { seedingApi } from '@/api/seeding'
import { formatBytes, formatTime } from '@/utils/format'

const loading = ref(false)
const records = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const clients = ref<any[]>([])
const sites = ref<string[]>([])

const filters = reactive({
  client_id: undefined as string | undefined,
  site_name: undefined as string | undefined,
  search: '',
})

const columns = [
  { title: '站点', key: 'site_name', width: 80 },
  { title: '状态', key: 'status', width: 80 },
  { title: '上传', key: 'uploaded', width: 90 },
  { title: '比率', key: 'ratio', width: 70 },
  { title: '操作原因', dataIndex: 'last_action_by', key: 'last_action_by', ellipsis: true },
  { title: '更新时间', key: 'updated_at', width: 150 },
]

function statusColor(status: string): string {
  const map: Record<string, string> = {
    deleted: 'default', seeding: 'green', pending: 'blue',
    paused_free_end: 'orange', paused_rule: 'orange', deleting: 'red',
  }
  return map[status] || 'default'
}

// §59.43: 私有副本删除，改用全局 formatBytes（utils/format）

async function fetchData() {
  loading.value = true
  try {
    const { data } = await seedingApi.getHistory({
      page: page.value,
      pageSize: pageSize.value,
      ...filters,
    } as any)
    records.value = data.data?.items || []
    total.value = data.data?.total || 0
    const siteSet = new Set<string>()
    records.value.forEach((r: any) => { if (r.site_name) siteSet.add(r.site_name) })
    sites.value = [...siteSet].sort()
  } catch {
    message.error('加载历史失败')
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>
