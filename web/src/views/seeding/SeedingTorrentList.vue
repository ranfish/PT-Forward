<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; gap: 12px">
      <a-input-search
        v-model:value="filters.search"
        :placeholder="t('downloader.searchTorrent')"
        style="width: 300px"
        @search="pagination.fetch(1)"
      />
      <a-select
        v-model:value="filters.site"
        :placeholder="t('seeding.filterSite')"
        allow-clear
        style="width: 200px"
        @change="pagination.fetch(1)"
      />
      <a-select
        v-model:value="filters.status"
        :placeholder="t('seeding.filterStatus')"
        allow-clear
        style="width: 150px"
        @change="pagination.fetch(1)"
      >
        <a-select-option value="seeding">{{ t('seeding.seedingStatus') }}</a-select-option>
        <a-select-option value="paused_free_end">{{ t('seeding.pausedFreeEnd') }}</a-select-option>
        <a-select-option value="paused_rule">{{ t('seeding.pausedRule') }}</a-select-option>
        <a-select-option value="downloading">{{ t('seeding.downloadingStatus') }}</a-select-option>
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
        showTotal: (total: number) => t('common.totalCount', { total }),
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
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button
              v-if="record.status !== 'seeding'"
              type="link"
              size="small"
              @click="handleResume(record.id)"
            >
              {{ t('common.resume') }}
            </a-button>
            <a-button
              v-if="record.status === 'seeding'"
              type="link"
              size="small"
              @click="handlePause(record.id)"
            >
              {{ t('common.pause') }}
            </a-button>
          </a-space>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { seedingApi } from '@/api/seeding'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()
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
  { title: '操作', key: 'actions', width: 100 },
]

const pagination = usePagination((page, size) =>
  seedingApi.listRecords(page, size),
)

async function handleResume(recordId: number) {
  try {
    await seedingApi.resumeRecord(recordId)
    message.success(t('common.resumed'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function handlePause(recordId: number) {
  try {
    await seedingApi.pauseRecord(recordId)
    message.success(t('common.paused'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
