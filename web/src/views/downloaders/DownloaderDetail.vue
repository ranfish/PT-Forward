<template>
  <div>
    <a-page-header :title="downloader.name || t('downloader.title')" @back="$router.push('/downloaders')">
      <template #tags>
        <a-badge
          :status="downloader.enabled ? 'success' : 'default'"
          :text="downloader.enabled ? t('common.enabled') : t('common.disabled')"
        />
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('common.name')">{{ downloader.name }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.type')">{{ downloader.type }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.address')">{{ downloader.url }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.status')">{{ downloader.enabled ? t('common.enabled') : t('common.disabled') }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.username')">{{ downloader.username || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ downloader.created_at || downloader.createdAt || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-tabs v-model:activeKey="activeTab" @change="onTabChange">
        <a-tab-pane key="torrents" :tab="t('downloader.torrents')">
          <div style="margin-bottom: 12px; display: flex; justify-content: space-between">
            <a-space>
              <a-input-search
                v-model:value="torrentSearch"
                :placeholder="t('downloader.searchTorrent')"
                style="width: 300px"
                @search="fetchTorrents"
              />
            </a-space>
            <a-button @click="fetchTorrents">
              <template #icon><ReloadOutlined /></template>
              {{ t('common.refresh') }}
            </a-button>
          </div>
          <a-table
            :columns="torrentColumns"
            :data-source="filteredTorrents"
            :loading="torrentsLoading"
            :pagination="{ pageSize: 20, showSizeChanger: true }"
            row-key="hash"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'total_size'">
                {{ formatSize(record.total_size) }}
              </template>
              <template v-if="column.key === 'progress'">
                <a-progress :percent="Math.round(record.progress * 100)" size="small" :stroke-width="4" style="width: 80px" />
              </template>
              <template v-if="column.key === 'upload_speed'">
                {{ formatSpeed(record.upload_speed) }}
              </template>
              <template v-if="column.key === 'download_speed'">
                {{ formatSpeed(record.download_speed) }}
              </template>
              <template v-if="column.key === 'state'">
                <a-tag :color="stateColor(record.state)">{{ record.state }}</a-tag>
              </template>
              <template v-if="column.key === 'actions'">
                <a-space>
                  <a-button
                    v-if="record.state === 'pausedUP' || record.state === 'pausedDL'"
                    type="link"
                    size="small"
                    @click="handleResume(record.hash)"
                  >
                    {{ t('common.resume') }}
                  </a-button>
                  <a-button
                    v-else
                    type="link"
                    size="small"
                    @click="handlePause(record.hash)"
                  >
                    {{ t('common.pause') }}
                  </a-button>
                  <a-popconfirm :title="t('common.deleteConfirm')" @confirm="handleDeleteTorrent(record.hash)">
                    <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-tab-pane>
        <a-tab-pane key="maindata" :tab="t('downloader.maindata')">
          <a-descriptions bordered :column="2" v-if="maindata">
            <a-descriptions-item :label="t('downloader.torrentCount')">{{ Object.keys(maindata.torrents || {}).length }}</a-descriptions-item>
            <a-descriptions-item :label="t('downloader.freeSpace')">{{ formatSize(maindata.free_space) }}</a-descriptions-item>
            <a-descriptions-item :label="t('downloader.categoryCount')">{{ Object.keys(maindata.categories || {}).length }}</a-descriptions-item>
            <a-descriptions-item :label="t('downloader.tagCount')">{{ (maindata.tags || []).length }}</a-descriptions-item>
          </a-descriptions>
          <a-empty v-else :description="t('common.noData')" />
        </a-tab-pane>
        <a-tab-pane key="publish-targets" :tab="t('downloader.publishTargets')">
          <div style="margin-bottom: 12px; display: flex; justify-content: flex-end">
            <a-button type="primary" @click="openTargetModal()">
              <template #icon><PlusOutlined /></template>
              {{ t('downloader.addPublishTarget') }}
            </a-button>
          </div>
          <a-table
            :columns="targetColumns"
            :data-source="publishTargets"
            :loading="targetsLoading"
            :pagination="false"
            row-key="id"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'auto_publish'">
                <a-tag :color="record.auto_publish ? 'green' : 'default'">{{ record.auto_publish ? t('common.yes') : t('common.no') }}</a-tag>
              </template>
              <template v-if="column.key === 'enabled'">
                <a-switch :checked="record.enabled" @change="(v: boolean) => toggleTarget(record, v)" />
              </template>
              <template v-if="column.key === 'actions'">
                <a-space>
                  <a-button type="link" size="small" @click="openTargetModal(record)">{{ t('common.edit') }}</a-button>
                  <a-popconfirm :title="t('downloader.deletePublishTargetConfirm')" @confirm="handleDeleteTarget(record.id)">
                    <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-tab-pane>
      </a-tabs>
    </a-spin>

    <a-modal
      v-model:open="targetModalVisible"
      :title="editingTarget ? t('downloader.editPublishTarget') : t('downloader.addPublishTarget')"
      @ok="handleTargetSubmit"
      :confirm-loading="targetSubmitting"
      width="560px"
    >
      <a-form :model="targetForm" layout="vertical">
        <a-form-item :label="t('downloader.targetSite')" :rules="[{ required: true, message: t('downloader.targetSite') }]">
          <a-input v-model:value="targetForm.site_name" placeholder="如 hdsky.me" :disabled="!!editingTarget" />
        </a-form-item>
        <a-form-item :label="t('downloader.categoryMapping')">
          <a-input v-model:value="targetForm.category_mapping" placeholder="如 Movie→电影,TV→剧集" />
        </a-form-item>
        <a-form-item :label="t('downloader.sourceMapping')">
          <a-input v-model:value="targetForm.source_mapping" placeholder="如 HDSky→天空" />
        </a-form-item>
        <a-form-item :label="t('downloader.codecMapping')">
          <a-input v-model:value="targetForm.codec_mapping" placeholder="如 x264→H.264,x265→H.265" />
        </a-form-item>
        <a-form-item :label="t('downloader.autoPublish')">
          <a-switch v-model:checked="targetForm.auto_publish" />
        </a-form-item>
        <a-form-item :label="t('downloader.notifyOnPublish')">
          <a-switch v-model:checked="targetForm.notify_on_publish" />
        </a-form-item>
        <a-form-item :label="t('common.enabled')">
          <a-switch v-model:checked="targetForm.enabled" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { ReloadOutlined, PlusOutlined } from '@ant-design/icons-vue'
import { downloadersApi } from '@/api/downloaders'

const { t } = useI18n()
const route = useRoute()
const id = Number(route.params.id)

const loading = ref(false)
const torrentsLoading = ref(false)
const targetsLoading = ref(false)
const downloader = ref<any>({})
const torrents = ref<any[]>([])
const maindata = ref<any>(null)
const publishTargets = ref<any[]>([])
const activeTab = ref('torrents')
const torrentSearch = ref('')

const targetModalVisible = ref(false)
const targetSubmitting = ref(false)
const editingTarget = ref<any>(null)
const targetForm = ref({
  site_name: '',
  category_mapping: '',
  source_mapping: '',
  codec_mapping: '',
  auto_publish: false,
  notify_on_publish: false,
  enabled: true,
})

const filteredTorrents = computed(() => {
  if (!torrentSearch.value) return torrents.value
  const q = torrentSearch.value.toLowerCase()
  return torrents.value.filter((t: any) => (t.name || '').toLowerCase().includes(q))
})

const torrentColumns = [
  { title: '名称', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '大小', key: 'total_size', width: 100 },
  { title: '进度', key: 'progress', width: 120 },
  { title: '状态', key: 'state', width: 100 },
  { title: '上传速度', key: 'upload_speed', width: 120 },
  { title: '下载速度', key: 'download_speed', width: 120 },
  { title: '操作', key: 'actions', width: 120 },
]

const targetColumns = [
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 140 },
  { title: '分类映射', dataIndex: 'category_mapping', key: 'category_mapping', ellipsis: true },
  { title: '来源映射', dataIndex: 'source_mapping', key: 'source_mapping', ellipsis: true },
  { title: '编码映射', dataIndex: 'codec_mapping', key: 'codec_mapping', ellipsis: true },
  { title: '自动发布', key: 'auto_publish', width: 90 },
  { title: '通知', dataIndex: 'notify_on_publish', key: 'notify_on_publish', width: 70 },
  { title: '启用', key: 'enabled', width: 80 },
  { title: '操作', key: 'actions', width: 140 },
]

function stateColor(state: string) {
  const colors: Record<string, string> = {
    uploading: 'green',
    stalledUP: 'green',
    downloading: 'blue',
    stalledDL: 'blue',
    pausedUP: 'orange',
    pausedDL: 'orange',
    error: 'red',
  }
  return colors[state] || 'default'
}

function formatSpeed(bytes: number) {
  if (!bytes) return '0 B/s'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

function formatSize(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

async function fetchDownloader() {
  loading.value = true
  try {
    const resp = await downloadersApi.get(id)
    downloader.value = resp.data.data || {}
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function fetchTorrents() {
  torrentsLoading.value = true
  try {
    const resp = await downloadersApi.getTorrents(id)
    const body = resp.data.data
    torrents.value = body?.items || body || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    torrentsLoading.value = false
  }
}

async function fetchMaindata() {
  try {
    const resp = await downloadersApi.getMaindata(id)
    maindata.value = resp.data.data || null
  } catch (e: any) {
    message.error(e.message)
  }
}

async function fetchPublishTargets() {
  targetsLoading.value = true
  try {
    const resp = await downloadersApi.listPublishTargets()
    const all = resp.data.data || []
    publishTargets.value = all.filter((t: any) => t.client_id === id)
  } catch (e: any) {
    message.error(e.message)
  } finally {
    targetsLoading.value = false
  }
}

async function handlePause(hash: string) {
  try {
    await downloadersApi.pauseTorrent(id, hash)
    message.success(t('common.paused'))
    fetchTorrents()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function handleResume(hash: string) {
  try {
    await downloadersApi.resumeTorrent(id, hash)
    message.success(t('common.resumed'))
    fetchTorrents()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function handleDeleteTorrent(hash: string) {
  try {
    await downloadersApi.deleteTorrent(id, hash)
    message.success(t('common.deleted'))
    fetchTorrents()
  } catch (e: any) {
    message.error(e.message)
  }
}

function openTargetModal(record?: any) {
  editingTarget.value = record || null
  if (record) {
    targetForm.value = {
      site_name: record.site_name || '',
      category_mapping: record.category_mapping || '',
      source_mapping: record.source_mapping || '',
      codec_mapping: record.codec_mapping || '',
      auto_publish: record.auto_publish || false,
      notify_on_publish: record.notify_on_publish || false,
      enabled: record.enabled !== false,
    }
  } else {
    targetForm.value = {
      site_name: '',
      category_mapping: '',
      source_mapping: '',
      codec_mapping: '',
      auto_publish: false,
      notify_on_publish: false,
      enabled: true,
    }
  }
  targetModalVisible.value = true
}

async function handleTargetSubmit() {
  if (!targetForm.value.site_name) {
    message.error(t('downloader.targetSite'))
    return
  }
  targetSubmitting.value = true
  try {
    if (editingTarget.value) {
      await downloadersApi.updatePublishTarget({
        id: editingTarget.value.id,
        category_mapping: targetForm.value.category_mapping,
        source_mapping: targetForm.value.source_mapping,
        codec_mapping: targetForm.value.codec_mapping,
        auto_publish: targetForm.value.auto_publish,
        notify_on_publish: targetForm.value.notify_on_publish,
        enabled: targetForm.value.enabled,
      })
    } else {
      await downloadersApi.createPublishTarget({
        client_id: id,
        site_name: targetForm.value.site_name,
        category_mapping: targetForm.value.category_mapping,
        source_mapping: targetForm.value.source_mapping,
        codec_mapping: targetForm.value.codec_mapping,
        auto_publish: targetForm.value.auto_publish,
        notify_on_publish: targetForm.value.notify_on_publish,
        enabled: targetForm.value.enabled,
      })
    }
    message.success(t('common.operationSuccess'))
    targetModalVisible.value = false
    fetchPublishTargets()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    targetSubmitting.value = false
  }
}

async function handleDeleteTarget(targetId: number) {
  try {
    await downloadersApi.deletePublishTarget(targetId)
    message.success(t('common.deleted'))
    fetchPublishTargets()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function toggleTarget(record: any, checked: boolean) {
  try {
    await downloadersApi.updatePublishTarget({ id: record.id, enabled: checked })
    message.success(t('common.statusUpdated'))
    fetchPublishTargets()
  } catch (e: any) {
    message.error(e.message)
  }
}

function onTabChange(key: string) {
  if (key === 'torrents' && torrents.value.length === 0) fetchTorrents()
  if (key === 'maindata' && !maindata.value) fetchMaindata()
  if (key === 'publish-targets') fetchPublishTargets()
}

onMounted(() => {
  fetchDownloader()
  fetchTorrents()
})
</script>
