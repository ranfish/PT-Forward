<template>
  <div>
    <a-page-header title="PTGen" sub-title="影片信息查询与缓存管理" />

    <a-card title="查询" style="margin-bottom: 16px">
      <a-space compact style="width: 100%">
        <a-input
          v-model:value="queryInput"
          placeholder="输入豆瓣链接、IMDb 链接或影片名称"
          style="max-width: 500px"
          @press-enter="handleQuery"
        />
        <a-button type="primary" :loading="querying" @click="handleQuery">查询</a-button>
      </a-space>
      <div v-if="queryResult" style="margin-top: 16px">
        <a-descriptions bordered size="small" :column="2">
          <a-descriptions-item label="中文名">{{ queryResult.chineseTitle }}</a-descriptions-item>
          <a-descriptions-item label="外文名">{{ queryResult.foreignTitle }}</a-descriptions-item>
          <a-descriptions-item label="年份">{{ queryResult.year }}</a-descriptions-item>
          <a-descriptions-item label="地区">
            <a-tag v-for="r in (queryResult.region || [])" :key="r">{{ r }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="豆瓣评分">{{ queryResult.doubanRating }}</a-descriptions-item>
          <a-descriptions-item label="IMDb评分">{{ queryResult.imdbRating }}</a-descriptions-item>
          <a-descriptions-item label="导演">
            <a-tag v-for="d in (queryResult.director || [])" :key="d">{{ d }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="类型">
            <a-tag v-for="g in (queryResult.genre || [])" :key="g">{{ g }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="简介" :span="2">{{ queryResult.introduction }}</a-descriptions-item>
        </a-descriptions>
        <div v-if="queryResult.posterURL" style="margin-top: 12px">
          <a-image :src="queryResult.posterURL" :width="200" />
        </div>
        <a-tag v-if="queryResult.cached" color="blue" style="margin-top: 8px">来自缓存</a-tag>
      </div>
    </a-card>

    <a-card title="查询缓存">
      <template #extra>
        <a-popconfirm title="确认清理过期缓存？" @confirm="handleCleanCache">
          <a-button size="small" danger>清理过期缓存</a-button>
        </a-popconfirm>
      </template>
      <a-table
        :columns="cacheColumns"
        :data-source="caches"
        :loading="cacheLoading"
        :pagination="cachePagination"
        row-key="id"
        size="small"
        @change="handleCacheTableChange"
      >
        <template #bodyCell="{ column, record }">
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
import { ptgenApi } from '@/api/ptgen'

const queryInput = ref('')
const querying = ref(false)
const queryResult = ref<any>(null)
const cacheLoading = ref(false)
const caches = ref<any[]>([])

const cachePagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
})

const cacheColumns = [
  { title: '查询关键词', dataIndex: 'query_key', ellipsis: true },
  { title: '中文标题', dataIndex: 'chinese_title', ellipsis: true },
  { title: '来源', dataIndex: 'source', width: 120 },
  { title: '更新时间', key: 'updated_at', width: 180 },
]

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

async function handleQuery() {
  if (!queryInput.value.trim()) {
    message.warning('请输入查询内容')
    return
  }
  querying.value = true
  queryResult.value = null
  try {
    const resp = await ptgenApi.query({ query: queryInput.value.trim() })
    queryResult.value = resp.data?.data || null
  } catch (e: any) {
    message.error(e?.response?.data?.message || '查询失败')
  } finally {
    querying.value = false
  }
}

async function fetchCache() {
  cacheLoading.value = true
  try {
    const resp = await ptgenApi.listCache({
      page: cachePagination.current,
      size: cachePagination.pageSize,
    })
    const data = resp.data?.data || {}
    caches.value = data.items || []
    cachePagination.total = data.total || 0
  } catch {
  } finally {
    cacheLoading.value = false
  }
}

async function handleCleanCache() {
  try {
    const resp = await ptgenApi.cleanCache()
    message.success(`已清理 ${resp.data?.data?.deleted || 0} 条过期缓存`)
    fetchCache()
  } catch (e: any) {
    message.error(e?.response?.data?.message || '清理失败')
  }
}

function handleCacheTableChange(pagination: any) {
  cachePagination.current = pagination.current
  cachePagination.pageSize = pagination.pageSize
  fetchCache()
}

onMounted(() => {
  fetchCache()
})
</script>
