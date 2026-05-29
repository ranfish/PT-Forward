<template>
  <div>
    <a-page-header title="PTGen" :sub-title="t('ptgen.subtitle')" />

    <a-card :title="t('ptgen.query')" style="margin-bottom: 16px">
      <a-space compact style="width: 100%">
        <a-input
          v-model:value="queryInput"
          :placeholder="t('ptgen.queryPlaceholder')"
          style="max-width: 500px"
          @press-enter="handleQuery"
        />
        <a-button type="primary" :loading="querying" @click="handleQuery">{{ t('ptgen.query') }}</a-button>
      </a-space>
      <div v-if="queryResult" style="margin-top: 16px">
        <a-descriptions bordered size="small" :column="2">
          <a-descriptions-item :label="t('ptgen.chineseTitle')">{{ queryResult.chineseTitle }}</a-descriptions-item>
          <a-descriptions-item :label="t('ptgen.foreignTitle')">{{ queryResult.foreignTitle }}</a-descriptions-item>
          <a-descriptions-item :label="t('ptgen.year')">{{ queryResult.year }}</a-descriptions-item>
          <a-descriptions-item :label="t('ptgen.region')">
            <a-tag v-for="r in (queryResult.region || [])" :key="r">{{ r }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item :label="t('ptgen.doubanRating')">{{ queryResult.doubanRating }}</a-descriptions-item>
          <a-descriptions-item :label="t('ptgen.imdbRating')">{{ queryResult.imdbRating }}</a-descriptions-item>
          <a-descriptions-item :label="t('ptgen.director')">
            <a-tag v-for="d in (queryResult.director || [])" :key="d">{{ d }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item :label="t('ptgen.genre')">
            <a-tag v-for="g in (queryResult.genre || [])" :key="g">{{ g }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item :label="t('ptgen.introduction')" :span="2">{{ queryResult.introduction }}</a-descriptions-item>
        </a-descriptions>
        <div v-if="queryResult.posterURL" style="margin-top: 12px">
          <a-image :src="queryResult.posterURL" :width="200" />
        </div>
        <a-tag v-if="queryResult.cached" color="blue" style="margin-top: 8px">{{ t('ptgen.fromCache') }}</a-tag>
      </div>
    </a-card>

    <a-card :title="t('ptgen.queryCache')">
      <template #extra>
        <a-popconfirm :title="t('ptgen.cleanCacheConfirm')" @confirm="handleCleanCache">
          <a-button size="small" danger>{{ t('ptgen.cleanExpiredCache') }}</a-button>
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
import { useI18n } from 'vue-i18n'
import { ptgenApi } from '@/api/ptgen'

const { t } = useI18n()

const queryInput = ref('')
const querying = ref(false)
const queryResult = ref<Record<string, unknown> | null>(null)
const cacheLoading = ref(false)
const caches = ref<Record<string, unknown>[]>([])

const cachePagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
})

const cacheColumns = [
  { title: t('ptgen.queryKey'), dataIndex: 'query_key', ellipsis: true },
  { title: t('ptgen.chineseTitleCache'), dataIndex: 'chinese_title', ellipsis: true },
  { title: t('ptgen.source'), dataIndex: 'source', width: 120 },
  { title: t('common.updatedAt'), key: 'updated_at', width: 180 },
]

import { formatTime } from '@/utils/format'

async function handleQuery() {
  if (!queryInput.value.trim()) {
    message.warning(t('ptgen.queryRequired'))
    return
  }
  querying.value = true
  queryResult.value = null
  try {
    const resp = await ptgenApi.query({ query: queryInput.value.trim() })
    queryResult.value = resp.data?.data || null
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    message.error(err?.response?.data?.message || t('ptgen.queryFailed'))
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
    const data = resp.data?.data || []
    caches.value = data
    cachePagination.total = data.length
  } catch {
  } finally {
    cacheLoading.value = false
  }
}

async function handleCleanCache() {
  try {
    const resp = await ptgenApi.cleanCache()
    message.success(t('ptgen.cacheCleaned', { count: resp.data?.data?.deleted || 0 }))
    fetchCache()
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    message.error(err?.response?.data?.message || t('ptgen.cleanFailed'))
  }
}

function handleCacheTableChange(pagination: { current: number; pageSize: number }) {
  cachePagination.current = pagination.current
  cachePagination.pageSize = pagination.pageSize
  fetchCache()
}

onMounted(() => {
  fetchCache()
})
</script>
