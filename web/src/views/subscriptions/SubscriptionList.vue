<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('subscription.addSubscription') }}
      </a-button>
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
      @change="(pag: any) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" @change="toggleSubscription(record)" />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/subscriptions/${record.id}`)">{{ t('common.detail') }}</a-button>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" @click="triggerFetch(record.id)">{{ t('common.trigger') }}</a-button>
            <a-popconfirm :title="t('subscription.deleteSubscriptionConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRecord ? t('subscription.editSubscription') : t('subscription.addSubscription')"
      @ok="handleSubmit"
      :confirm-loading="submitting"
      width="640px"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('common.name')" name="name" :rules="[{ required: true, message: t('common.nameRequired') }]">
          <a-input v-model:value="form.name" placeholder="订阅名称" />
        </a-form-item>
        <a-form-item label="所属站点" name="siteName" :rules="[{ required: true, message: '请选择站点' }]">
          <a-select v-model:value="form.siteName" placeholder="选择站点" show-search :filter-option="filterSite" :loading="sitesLoading">
            <a-select-option v-for="s in sites" :key="s.name" :value="s.name">{{ s.name }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="RSS 地址" name="urls" :rules="[{ required: true, message: '请输入 RSS 地址' }]">
          <a-textarea v-model:value="form.urls" placeholder="每行一个 RSS 地址" :rows="3" />
        </a-form-item>
        <a-form-item label="定时表达式" name="cron">
          <a-input v-model:value="form.cron" placeholder="例如: */15 * * * *（每15分钟）" />
        </a-form-item>

        <a-divider>下载器设置</a-divider>
        <a-form-item label="下载器" name="clientId">
          <a-select v-model:value="form.clientId" placeholder="选择下载器" :loading="downloadersLoading" allow-clear>
            <a-select-option v-for="d in downloaders" :key="d.name" :value="d.name">
              {{ d.name }}（{{ d.type }}）
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="保存路径（留空使用下载器默认值）" name="savePath">
          <a-input v-model:value="form.savePath" placeholder="/downloads/..." />
        </a-form-item>
        <a-form-item label="分类" name="category">
          <a-input v-model:value="form.category" placeholder="下载器中的分类" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="暂停添加" name="addPaused">
              <a-switch v-model:checked="form.addPaused" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="自动种子管理" name="autoTmm">
              <a-switch v-model:checked="form.autoTmm" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="标签" name="tags">
          <a-select v-model:value="form.tags" mode="tags" placeholder="输入标签后回车" style="width: 100%" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="上传限速 (KB/s)" name="uploadLimitKb">
              <a-input-number v-model:value="form.uploadLimitKb" :min="0" style="width: 100%" placeholder="0 = 不限" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="下载限速 (KB/s)" name="downloadLimitKb">
              <a-input-number v-model:value="form.downloadLimitKb" :min="0" style="width: 100%" placeholder="0 = 不限" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>抓取选项</a-divider>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="只接受免费种子" name="scrapeFree">
              <a-switch v-model:checked="form.scrapeFree" />
              <div style="color:#999;font-size:12px;margin-top:4px">开启后仅推送免费/折扣种子到下载器</div>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="检测 HR 信息" name="scrapeHr">
              <a-switch v-model:checked="form.scrapeHr" />
              <div style="color:#999;font-size:12px;margin-top:4px">检测站点 HR（保种时间要求）</div>
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>自动化</a-divider>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="启用自动发布" name="publishEnabled">
              <a-switch v-model:checked="form.publishEnabled" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="推送通知" name="pushNotify">
              <a-switch v-model:checked="form.pushNotify" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="自动辅种" name="autoReseed">
              <a-switch v-model:checked="form.autoReseed" />
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { useI18n } from 'vue-i18n'
import { subscriptionsApi } from '@/api/subscriptions'
import { sitesApi } from '@/api/sites'
import { downloadersApi } from '@/api/downloaders'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()

const modalVisible = ref(false)
const submitting = ref(false)
const editingRecord = ref<any>(null)

const sites = ref<any[]>([])
const sitesLoading = ref(false)
const downloaders = ref<any[]>([])
const downloadersLoading = ref(false)

const form = reactive({
  name: '',
  siteName: '',
  urls: '',
  cron: '*/15 * * * *',
  clientId: '',
  savePath: '',
  category: '',
  addPaused: false,
  autoTmm: false,
  tags: [] as string[],
  scrapeFree: false,
  scrapeHr: false,
  uploadLimitKb: 0,
  downloadLimitKb: 0,
  publishEnabled: false,
  pushNotify: false,
  autoReseed: false,
  enabled: true,
})

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '站点', dataIndex: 'siteName', key: 'siteName' },
  { title: '定时表达式', dataIndex: 'cron', key: 'cron', width: 140 },
  { title: '启用', key: 'enabled', width: 80, align: 'center' as const },
  { title: '操作', key: 'actions', width: 220 },
]

const pagination = usePagination((page, size) => subscriptionsApi.list(page, size))

function filterSite(input: string, option: any) {
  return option.key?.toLowerCase().includes(input.toLowerCase())
}

async function fetchSites() {
  sitesLoading.value = true
  try {
    const resp = await sitesApi.list(1, 200)
    sites.value = resp.data?.data?.items || resp.data?.data || []
  } catch {
    sites.value = []
  } finally {
    sitesLoading.value = false
  }
}

async function fetchDownloaders() {
  downloadersLoading.value = true
  try {
    const resp = await downloadersApi.list(1, 200)
    downloaders.value = resp.data?.data?.items || resp.data?.data || []
  } catch {
    downloaders.value = []
  } finally {
    downloadersLoading.value = false
  }
}

function openModal(record?: any) {
  editingRecord.value = record || null
  if (record) {
    Object.assign(form, {
      name: record.name, siteName: record.siteName,
      urls: (record.urls || []).join('\n'), cron: record.cron || '*/15 * * * *',
      clientId: record.clientId || '',
      savePath: record.savePath || '',
      category: record.category || '',
      addPaused: record.addPaused || false,
      autoTmm: record.autoTmm || false,
      tags: record.tags || [],
      scrapeFree: record.scrapeFree || false,
      scrapeHr: record.scrapeHr || false,
      uploadLimitKb: record.uploadLimitKb || 0,
      downloadLimitKb: record.downloadLimitKb || 0,
      publishEnabled: record.publishEnabled || false,
      pushNotify: record.pushNotify || false,
      autoReseed: record.autoReseed || false,
      enabled: record.enabled ?? true,
    })
  } else {
    Object.assign(form, {
      name: '', siteName: '', urls: '', cron: '*/15 * * * *',
      clientId: '', savePath: '', category: '', addPaused: false, autoTmm: false,
      tags: [], scrapeFree: false, scrapeHr: false,
      uploadLimitKb: 0, downloadLimitKb: 0,
      publishEnabled: false, pushNotify: false, autoReseed: false, enabled: true,
    })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingRecord.value) {
      await subscriptionsApi.update(editingRecord.value.id, { ...form, urls: form.urls.split('\n').filter(u => u.trim()) })
    } else {
      await subscriptionsApi.create({ ...form, urls: form.urls.split('\n').filter(u => u.trim()) })
    }
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await subscriptionsApi.delete(id)
    message.success(t('common.deleteSuccess'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function toggleSubscription(record: any) {
  try {
    if (record.enabled) {
      await subscriptionsApi.pause(record.id)
    } else {
      await subscriptionsApi.resume(record.id)
    }
    message.success(t('subscription.statusToggled'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function triggerFetch(id: number) {
  try {
    await subscriptionsApi.trigger(id)
    message.success(t('subscription.fetchTriggered'))
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => {
  pagination.fetch()
  fetchSites()
  fetchDownloaders()
})
</script>
