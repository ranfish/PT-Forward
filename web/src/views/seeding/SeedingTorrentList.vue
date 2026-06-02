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
        <a-select-option value="pending">{{ t('seeding.pendingStatus') }}</a-select-option>
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
    />
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted, watch } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { seedingApi } from '@/api/seeding'
import { downloadersApi } from '@/api/downloaders'
import { usePagination } from '@/composables/usePagination'
import { useEnumLabels } from '@/utils/enumLabels'
import { useTorrentColumns } from '@/composables/useTorrentColumns'
import { h } from 'vue'
import { Badge, Space, Button } from 'ant-design-vue'

const { t } = useI18n()
const { translateSeedingStatus } = useEnumLabels()

function handleResume(recordId: number) {
  return async () => {
    try {
      await seedingApi.resumeRecord(recordId)
      message.success(t('common.resumed'))
      pagination.fetch()
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }
}

function handlePause(recordId: number) {
  return async () => {
    try {
      await seedingApi.pauseRecord(recordId)
      message.success(t('common.paused'))
      pagination.fetch()
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }
}

const { columns } = useTorrentColumns({
  show: ['title', 'site_name', 'torrent_id', 'discount', 'is_free', 'has_hr', 'torrent_size', 'info_hash', 'client_id', 'source', 'status', 'flushed_at', 'updated_at', 'actions'],
  statusRender: (record) => h(Badge, {
    status: record.status === 'seeding' ? 'success' : record.status === 'downloading' ? 'processing' : 'warning',
    text: translateSeedingStatus(record.status as string),
  }),
  actionsRender: (record) => h(Space, () => [
    record.status !== 'seeding'
      ? h(Button, { type: 'link', size: 'small', onClick: handleResume(record.id as number) }, () => t('common.resume'))
      : h(Button, { type: 'link', size: 'small', onClick: handlePause(record.id as number) }, () => t('common.pause')),
  ]),
})

const downloaderOptions = ref<{label: string, value: string}[]>([])
const filters = reactive({
  search: '',
  clientId: undefined as string | undefined,
  site: undefined as string | undefined,
  status: undefined as string | undefined,
})

const pagination = usePagination((page, size) =>
  seedingApi.listRecords(page, size, { ...filters }),
)

watch(filters, () => {
  pagination.fetch(1)
}, { deep: true })

async function fetchDownloaders() {
  try {
    const resp = await downloadersApi.list(1, 100)
    const items = resp.data.data?.items || resp.data.data || []
    downloaderOptions.value = items.map((d: { name?: string; id?: number }) => ({
      label: String(d.name || d.id),
      value: String(d.name || d.id),
    }))
  } catch {}
}

onMounted(() => {
  fetchDownloaders()
  pagination.fetch()
})
</script>
