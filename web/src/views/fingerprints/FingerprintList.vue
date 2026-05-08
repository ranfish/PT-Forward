<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; gap: 12px">
      <a-input
        v-model:value="searchForm.infoHash"
        :placeholder="t('fingerprint.searchInfoHash')"
        style="width: 320px"
        @pressEnter="handleSearch"
      />
      <a-input
        v-model:value="searchForm.piecesHash"
        :placeholder="t('fingerprint.searchPiecesHash')"
        style="width: 320px"
        @pressEnter="handleSearch"
      />
      <a-button type="primary" @click="handleSearch">{{ t('common.search') }}</a-button>
      <a-button @click="resetSearch">{{ t('common.reset') }}</a-button>
      <a-popconfirm title="确定清理指纹缓存？" @confirm="handleDeleteCache">
        <a-button danger>清理缓存</a-button>
      </a-popconfirm>
    </div>

    <a-table
      :columns="columns"
      :data-source="searchMode ? searchResults : pagination.data.value"
      :loading="searchMode ? searchLoading : pagination.loading.value"
      :pagination="searchMode ? false : {
        current: pagination.currentPage.value,
        pageSize: pagination.pageSize.value,
        total: pagination.total.value,
        showSizeChanger: true,
        showTotal: (total: number) => `共 ${total} 条`,
      }"
      row-key="id"
      @change="(pag: any) => { if (!searchMode) pagination.onPageChange(pag.current, pag.pageSize) }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'actions'">
          <a-popconfirm :title="t('fingerprint.deleteConfirm')" @confirm="handleDelete(record.id)">
            <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
          </a-popconfirm>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { fingerprintsApi } from '@/api/fingerprints'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()

const searchMode = ref(false)
const searchLoading = ref(false)
const searchResults = ref<any[]>([])

const searchForm = reactive({ infoHash: '', piecesHash: '' })

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: 'InfoHash', dataIndex: 'info_hash', key: 'info_hash', ellipsis: true },
  { title: 'PiecesHash', dataIndex: 'pieces_hash', key: 'pieces_hash', ellipsis: true },
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: '操作', key: 'actions', width: 80 },
]

const pagination = usePagination((page, size) => fingerprintsApi.list(page, size))

async function handleSearch() {
  if (!searchForm.infoHash && !searchForm.piecesHash) {
    searchMode.value = false
    return
  }
  searchMode.value = true
  searchLoading.value = true
  try {
    const params: any = {}
    if (searchForm.infoHash) params.infoHash = searchForm.infoHash
    if (searchForm.piecesHash) params.piecesHash = searchForm.piecesHash
    const resp = await fingerprintsApi.search(params)
    searchResults.value = resp.data.data || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    searchLoading.value = false
  }
}

function resetSearch() {
  searchForm.infoHash = ''
  searchForm.piecesHash = ''
  searchMode.value = false
  searchResults.value = []
}

async function handleDelete(id: number) {
  try {
    await fingerprintsApi.delete(id)
    message.success(t('common.deleteSuccess'))
    if (searchMode.value) {
      handleSearch()
    } else {
      pagination.fetch()
    }
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())

async function handleDeleteCache() {
  try {
    await fingerprintsApi.deleteCache()
    message.success(t('common.deleteSuccess'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}
</script>
