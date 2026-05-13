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
        v-model:value="filters.clientId"
        :placeholder="t('seeding.filterClient')"
        allow-clear
        style="width: 200px"
        :options="downloaderOptions"
        show-search
        :filter-option="(input: string, option: { label: string }) => option.label.toLowerCase().includes(input.toLowerCase())"
        @change="pagination.fetch(1)"
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
        showTotal: (total: number) => t('common.totalCount', { count: total }),
      }"
      row-key="id"
      @change="(pag: { current: number; pageSize: number }) => pagination.onPageChange(pag.current, pag.pageSize)"
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
import { reactive, ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { seedingApi } from '@/api/seeding'
import { downloadersApi } from '@/api/downloaders'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()
const downloaderOptions = ref<{label: string, value: string}[]>([])
const filters = reactive({
  search: '',
  clientId: undefined as string | undefined,
  site: undefined as string | undefined,
  status: undefined as string | undefined,
})

const columns = [
  { title: 'Torrent ID', dataIndex: 'torrent_id', key: 'torrent_id', ellipsis: true },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: t('seeding.client'), dataIndex: 'client_id', key: 'client_id', width: 100 },
  { title: 'InfoHash', dataIndex: 'info_hash', key: 'info_hash', ellipsis: true },
  { title: t('common.status'), key: 'status', width: 120 },
  { title: t('seeding.free'), dataIndex: 'is_free', key: 'is_free', width: 60 },
  { title: 'HR', dataIndex: 'has_hr', key: 'has_hr', width: 60 },
  { title: t('seeding.source'), dataIndex: 'source', key: 'source', width: 80 },
  { title: t('common.updatedAt'), dataIndex: 'updated_at', key: 'updated_at', width: 180 },
  { title: t('common.actions'), key: 'actions', width: 100 },
]

const pagination = usePagination((page, size) =>
  seedingApi.listRecords(page, size),
)

async function handleResume(recordId: number) {
  try {
    await seedingApi.resumeRecord(recordId)
    message.success(t('common.resumed'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function handlePause(recordId: number) {
  try {
    await seedingApi.pauseRecord(recordId)
    message.success(t('common.paused'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function fetchDownloaders() {
  try {
    const resp = await downloadersApi.list(1, 100)
    const items = resp.data.data?.items || resp.data.data || []
    downloaderOptions.value = items.map((d: Record<string, unknown>) => ({
      label: String(d.name || d.id),
      value: String(d.name || d.id),
    }))
  } catch (_e: unknown) {}
}

onMounted(() => {
  fetchDownloaders()
  pagination.fetch()
})
</script>
