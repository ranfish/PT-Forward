<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; gap: 12px">
      <a-input-search
        v-model:value="filters.search"
        placeholder="搜索种子名称"
        style="width: 300px"
        @search="pagination.fetch(1)"
      />
      <a-select
        v-model:value="filters.site"
        placeholder="筛选站点"
        allow-clear
        style="width: 200px"
        @change="pagination.fetch(1)"
      />
      <a-select
        v-model:value="filters.status"
        placeholder="筛选状态"
        allow-clear
        style="width: 150px"
        @change="pagination.fetch(1)"
      >
        <a-select-option value="seeding">做种中</a-select-option>
        <a-select-option value="paused_free_end">免费到期暂停</a-select-option>
        <a-select-option value="paused_rule">规则暂停</a-select-option>
        <a-select-option value="downloading">下载中</a-select-option>
      </a-select>
    </div>

    <a-table
      :columns="columns"
      :data-source="pagination.data.value"
      :loading="pagination.loading.value"
      :pagination="{
        current: pagination.currentPage.value,
        pageSize: pagination.pageSize.value,
        total: pagination.total.value,
        showSizeChanger: true,
        showTotal: (total: number) => `共 ${total} 条`,
      }"
      row-key="id"
      @change="(pag: any) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-badge
            :status="record.status === 'seeding' ? 'success' : record.status === 'downloading' ? 'processing' : 'warning'"
            :text="record.status"
          />
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted } from 'vue'
import { seedingApi } from '@/api/seeding'
import { usePagination } from '@/composables/usePagination'

const filters = reactive({
  search: '',
  site: undefined as string | undefined,
  status: undefined as string | undefined,
})

const columns = [
  { title: 'Torrent ID', dataIndex: 'torrent_id', key: 'torrent_id', ellipsis: true },
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: '客户端', dataIndex: 'client_id', key: 'client_id', width: 100 },
  { title: 'InfoHash', dataIndex: 'info_hash', key: 'info_hash', ellipsis: true },
  { title: '状态', key: 'status', width: 120 },
  { title: '免费', dataIndex: 'is_free', key: 'is_free', width: 60 },
  { title: 'HR', dataIndex: 'has_hr', key: 'has_hr', width: 60 },
  { title: '来源', dataIndex: 'source', key: 'source', width: 80 },
  { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180 },
]

const pagination = usePagination((page, size) =>
  seedingApi.listRecords(page, size),
)

onMounted(() => pagination.fetch())
</script>
