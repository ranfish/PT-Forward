<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: space-between">
      <a-space>
        <a-input-search
          v-model:value="searchText"
          :placeholder="t('common.search')"
          style="width: 240px"
          @search="pagination.fetch(1)"
        />
      </a-space>
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('site.addSite') }}
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
        showTotal: (total: number) => t('common.totalCount', { total }),
      }"
      row-key="id"
      @change="(pag: any) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'framework'">
          <a-tag :color="frameworkColors[record.framework] || 'default'">
            {{ frameworkLabels[record.framework] || record.framework }}
          </a-tag>
        </template>
        <template v-if="column.key === 'authType'">
          <a-tag>{{ authTypeLabels[record.authType] || record.authType || 'Cookie' }}</a-tag>
        </template>
        <template v-if="column.key === 'hasCookie'">
          <a-badge
            :status="hasAnyCredential(record) ? 'success' : 'default'"
            :text="hasAnyCredential(record) ? t('common.configured') : t('common.notConfigured')"
          />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/sites/${record.id}`)">{{ t('common.detail') }}</a-button>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" @click="testConnection(record.id)">{{ t('common.test') }}</a-button>
            <a-popconfirm :title="t('site.deleteSiteConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingSite ? t('site.editSite') : t('site.addSite')"
      @ok="handleSubmit"
      :confirm-loading="submitting"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('site.domain')" name="domain" :rules="[{ required: true, message: t('site.domainRequired') }]">
          <a-input v-model:value="form.domain" :disabled="!!editingSite" placeholder="例如: pterclub.net" />
        </a-form-item>
        <a-form-item :label="t('common.name')" name="name" :rules="[{ required: true, message: t('common.nameRequired') }]">
          <a-input v-model:value="form.name" placeholder="站点显示名称" />
        </a-form-item>
        <a-form-item :label="t('site.siteUrl')" name="baseUrl" :rules="[{ required: true, message: t('site.siteUrlRequired') }]">
          <a-input v-model:value="form.baseUrl" placeholder="例如: https://pterclub.net" />
        </a-form-item>
        <a-form-item :label="t('site.framework')" name="framework">
          <a-select v-model:value="form.framework" placeholder="选择框架">
            <a-select-opt-group :label="t('site.mainstreamFrameworks')">
              <a-select-option value="nexusphp">NexusPHP</a-select-option>
              <a-select-option value="unit3d">UNIT3D</a-select-option>
              <a-select-option value="gazelle">Gazelle</a-select-option>
            </a-select-opt-group>
            <a-select-opt-group :label="t('site.specialFrameworks')">
              <a-select-option value="mteam">M-Team (馒头)</a-select-option>
              <a-select-option value="rousi">Rousi (肉丝)</a-select-option>
              <a-select-option value="tnode">TNode (朱雀)</a-select-option>
              <a-select-option value="luminance">Luminance</a-select-option>
            </a-select-opt-group>
            <a-select-opt-group :label="t('site.otherFrameworks')">
              <a-select-option value="generic">通用 (Generic)</a-select-option>
            </a-select-opt-group>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('site.authType')" name="authType">
          <a-select v-model:value="form.authType" placeholder="选择认证方式">
            <a-select-option value="cookie">Cookie</a-select-option>
            <a-select-option value="apikey">API Key</a-select-option>
            <a-select-option value="passkey">Passkey</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="showCookieField" label="Cookie" name="cookie">
          <a-textarea v-model:value="form.cookie" :rows="3" placeholder="站点 Cookie" />
        </a-form-item>
        <a-form-item v-if="showPasskeyField" label="Passkey" name="passkey">
          <a-input v-model:value="form.passkey" placeholder="站点 Passkey" />
        </a-form-item>
        <a-form-item v-if="showApiKeyField" label="API Key" name="apiKey">
          <a-input-password v-model:value="form.apiKey" placeholder="站点 API Key（馒头使用 x-api-key）" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { useI18n } from 'vue-i18n'
import { sitesApi } from '@/api/sites'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()

const searchText = ref('')
const modalVisible = ref(false)
const submitting = ref(false)
const editingSite = ref<any>(null)

const form = reactive({
  domain: '',
  name: '',
  baseUrl: '',
  framework: '',
  authType: 'cookie',
  cookie: '',
  passkey: '',
  apiKey: '',
})

const showCookieField = computed(() => form.authType === 'cookie')
const showPasskeyField = computed(() => form.authType === 'passkey')
const showApiKeyField = computed(() => form.authType === 'apikey')

const frameworkColors: Record<string, string> = {
  nexusphp: 'blue',
  unit3d: 'green',
  gazelle: 'purple',
  mteam: 'orange',
  rousi: 'pink',
  tnode: 'cyan',
  luminance: 'magenta',
  generic: 'default',
}

const frameworkLabels: Record<string, string> = {
  nexusphp: 'NexusPHP',
  unit3d: 'UNIT3D',
  gazelle: 'Gazelle',
  mteam: 'M-Team',
  rousi: 'Rousi',
  tnode: 'TNode',
  luminance: 'Luminance',
  generic: 'Generic',
}

const authTypeLabels: Record<string, string> = {
  cookie: 'Cookie',
  apikey: 'API Key',
  passkey: 'Passkey',
}

function hasAnyCredential(record: any): boolean {
  return record.hasCookie || record.hasApiKey || record.hasPasskey
}

const columns = [
  { title: '域名', dataIndex: 'domain', key: 'domain' },
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '框架', dataIndex: 'framework', key: 'framework' },
  { title: '认证方式', key: 'authType' },
  { title: '凭据状态', key: 'hasCookie' },
  { title: '操作', key: 'actions', width: 260 },
]

const pagination = usePagination((page, size) => sitesApi.list(page, size))

function openModal(record?: any) {
  editingSite.value = record || null
  if (record) {
    Object.assign(form, {
      domain: record.domain,
      name: record.name,
      baseUrl: record.baseUrl || '',
      framework: record.framework,
      authType: record.authType || 'cookie',
      cookie: '',
      passkey: '',
      apiKey: '',
    })
  } else {
    Object.assign(form, { domain: '', name: '', baseUrl: '', framework: '', authType: 'cookie', cookie: '', passkey: '', apiKey: '' })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingSite.value) {
      await sitesApi.update(editingSite.value.id, form)
    } else {
      await sitesApi.create(form)
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
    await sitesApi.delete(id)
    message.success(t('common.deleteSuccess'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function testConnection(id: number) {
  try {
    await sitesApi.testConnection(id)
    message.success(t('site.connectionTestSuccess'))
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
