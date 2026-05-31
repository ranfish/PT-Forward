<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: space-between">
      <a-space wrap>
        <a-input
          v-model:value="searchText"
          :placeholder="t('common.search')"
          style="width: 200px"
          allow-clear
        />
        <a-select v-model:value="filters.enabled" :placeholder="t('site.filterEnabled')" style="width: 110px" allow-clear>
          <a-select-option value="true">{{ t('site.filterEnabledOnly') }}</a-select-option>
          <a-select-option value="false">{{ t('site.filterDisabledOnly') }}</a-select-option>
        </a-select>
        <a-select v-model:value="filters.isSource" :placeholder="t('site.filterSource')" style="width: 110px" allow-clear>
          <a-select-option value="true">{{ t('site.filterYes') }}</a-select-option>
          <a-select-option value="false">{{ t('site.filterNo') }}</a-select-option>
        </a-select>
        <a-select v-model:value="filters.isTarget" :placeholder="t('site.filterTarget')" style="width: 110px" allow-clear>
          <a-select-option value="true">{{ t('site.filterYes') }}</a-select-option>
          <a-select-option value="false">{{ t('site.filterNo') }}</a-select-option>
        </a-select>
        <a-button :loading="syncing" @click="syncAllStats">{{ t('site.syncAllStats') }}</a-button>
        <template v-if="selectedRowKeys.length > 0">
          <a-divider type="vertical" />
          <a-tag color="blue" closable @close="selectedRowKeys = []">{{ t('site.selectedCount', { count: selectedRowKeys.length }) }}</a-tag>
          <a-button size="small" :loading="batchSyncing" @click="batchSyncSelected">{{ t('site.batchSync') }}</a-button>
          <a-button size="small" @click="batchUpdate('enabled', true)">{{ t('site.batchEnable') }}</a-button>
          <a-button size="small" @click="batchUpdate('enabled', false)">{{ t('site.batchDisable') }}</a-button>
          <a-button size="small" @click="batchUpdate('is_source', true)">{{ t('site.batchSetSource') }}</a-button>
          <a-button size="small" @click="batchUpdate('is_source', false)">{{ t('site.batchUnsetSource') }}</a-button>
          <a-button size="small" @click="batchUpdate('is_target', true)">{{ t('site.batchSetTarget') }}</a-button>
          <a-button size="small" @click="batchUpdate('is_target', false)">{{ t('site.batchUnsetTarget') }}</a-button>
          <a-button size="small" @click="batchUpdate('participate_auto_publish', true)">{{ t('site.batchSetPublish') }}</a-button>
          <a-button size="small" @click="batchUpdate('participate_auto_publish', false)">{{ t('site.batchUnsetPublish') }}</a-button>
        </template>
      </a-space>
      <a-button type="primary" @click="openCreateModal">{{ t('common.create') }}</a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="filteredData"
      :loading="loading"
      :row-selection="{ selectedRowKeys, onChange: onSelectionChange }"
      :pagination="{
        current: currentPage,
        pageSize: pageSize,
        total: filteredData.length,
        showSizeChanger: true,
        showTotal: (total: number) => t('common.totalCount', { count: total }),
      }"
      row-key="id"
      @change="(pag: { current: number; pageSize: number }) => { currentPage = pag.current; pageSize = pag.pageSize }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" size="small" @change="(v: boolean) => toggleField(record, 'enabled', v)" />
        </template>
        <template v-if="column.key === 'participateAutoPublish'">
          <a-switch :checked="record.participateAutoPublish" size="small" @change="(v: boolean) => toggleField(record, 'participateAutoPublish', v)" />
        </template>
        <template v-if="column.key === 'isSource'">
          <a-switch :checked="record.isSource" size="small" @change="(v: boolean) => toggleField(record, 'isSource', v)" />
        </template>
        <template v-if="column.key === 'isTarget'">
          <a-switch :checked="record.isTarget" size="small" @change="(v: boolean) => toggleField(record, 'isTarget', v)" />
        </template>
        <template v-if="column.key === 'hasCookie'">
          <a-badge
            :status="hasAnyCredential(record) ? 'success' : 'default'"
            :text="hasAnyCredential(record) ? t('common.configured') : t('common.notConfigured')"
          />
        </template>
        <template v-if="column.key === 'username'">
          <span>{{ record.username || '-' }}</span>
        </template>
        <template v-if="column.key === 'userClass'">
          <span>{{ record.userClass || '-' }}</span>
        </template>
        <template v-if="column.key === 'uploadBytes'">
          <span>{{ formatBytes(Number(record.uploadBytes)) }}</span>
        </template>
        <template v-if="column.key === 'downloadBytes'">
          <span>{{ formatBytes(Number(record.downloadBytes)) }}</span>
        </template>
        <template v-if="column.key === 'ratio'">
          <span>{{ Number(record.uploadBytes) > 0 && Number(record.downloadBytes) === 0 ? '∞' : (record.ratio ? record.ratio.toFixed(2) : '-') }}</span>
        </template>
        <template v-if="column.key === 'seedingCount'">
          <span>{{ record.seedingCount || '-' }}</span>
        </template>
        <template v-if="column.key === 'seedingSize'">
          <span>{{ record.seedingSize ? formatBytes(Number(record.seedingSize)) : '-' }}</span>
        </template>
        <template v-if="column.key === 'bonusPoints'">
          <span>{{ record.bonusPoints ? Number(record.bonusPoints).toLocaleString('en', { maximumFractionDigits: 0 }) : '-' }}</span>
        </template>
        <template v-if="column.key === 'syncTime'">
          <span>{{ formatTime(record.statsSyncedAt) }}</span>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/sites/${record.id}`)">{{ t('common.detail') }}</a-button>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" :loading="syncingSingleId === record.id" @click="syncSingleStats(record.id)">{{ t('site.syncStats') }}</a-button>
            <a-button type="link" size="small" @click="testConnection(record.id)">{{ t('common.test') }}</a-button>
            <a-popconfirm :title="t('common.deleteConfirm')" @confirm="deleteSite(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="isCreateMode ? t('site.addSite') : t('site.editSite')"
      :confirm-loading="submitting"
      width="640px"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <template v-if="isCreateMode">
          <a-form-item :label="t('site.siteName')" name="name" :rules="[{ required: true, message: t('site.nameRequired') }]">
            <a-input v-model:value="form.name" :placeholder="t('site.siteName')" />
          </a-form-item>
          <a-form-item :label="t('site.domain')" name="domain">
            <a-input v-model:value="form.domain" :placeholder="t('site.domainPlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('site.baseUrl')" name="baseUrl">
            <a-input v-model:value="form.baseUrl" :placeholder="t('site.baseUrlPlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('site.framework')" name="framework">
            <a-select v-model:value="form.framework" :placeholder="t('site.framework')" allow-clear>
              <a-select-option value="nexusphp">NexusPHP</a-select-option>
              <a-select-option value="unit3d">UNIT3D</a-select-option>
              <a-select-option value="gazelle">Gazelle</a-select-option>
              <a-select-option value="mteam">M-Team</a-select-option>
              <a-select-option value="tnode">TNode</a-select-option>
              <a-select-option value="luminance">Luminance</a-select-option>
              <a-select-option value="rousi">ROUSI</a-select-option>
              <a-select-option value="generic">{{ t('site.frameworkGeneric') }}</a-select-option>
            </a-select>
          </a-form-item>
        </template>
        <a-form-item :label="t('site.authType')" name="authType">
          <a-select v-model:value="form.authType" :placeholder="t('site.selectAuthType')">
            <a-select-option value="cookie">Cookie</a-select-option>
            <a-select-option value="apikey">{{ t('site.authTypeApiKey') }}</a-select-option>
            <a-select-option value="passkey">Passkey</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="showCookieField" :label="t('sites.cookie')" name="cookie">
          <a-textarea v-model:value="form.cookie" :rows="3" :placeholder="editingSite?.hasCookie ? t('site.placeholderConfigured') : t('site.placeholderCookie')" />
        </a-form-item>
        <a-form-item v-if="showPasskeyField" :label="t('sites.passkey')" name="passkey">
          <a-input-password v-model:value="form.passkey" :placeholder="editingSite?.hasPasskey ? t('site.placeholderConfigured') : t('site.placeholderPasskey')" />
        </a-form-item>
        <a-form-item v-if="showApiKeyField" :label="t('sites.apiKey')" name="apiKey">
          <a-input-password v-model:value="form.apiKey" :placeholder="editingSite?.hasApiKey ? t('site.placeholderConfigured') : t('site.placeholderApiKey')" />
        </a-form-item>

        <a-divider>{{ t('site.roleAndPublish') }}</a-divider>
        <a-form-item :label="t('site.enabledLabel')" name="enabled">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
        <a-form-item :label="t('site.asSource')" name="isSource">
          <a-switch v-model:checked="form.isSource" />
        </a-form-item>
        <a-form-item :label="t('site.asTarget')" name="isTarget">
          <a-switch v-model:checked="form.isTarget" />
        </a-form-item>
        <a-form-item :label="t('site.participateAutoPublishLabel')" name="participateAutoPublish">
          <a-switch v-model:checked="form.participateAutoPublish" />
        </a-form-item>

        <a-divider v-if="isCookieAuth">{{ t('site.cookieCloudSyncLabel') }}</a-divider>
        <template v-if="isCookieAuth">
          <a-form-item :label="t('site.cookieCloudSyncLabel')" name="cookieCloudSync">
            <a-switch v-model:checked="form.cookieCloudSync" />
          </a-form-item>
          <a-form-item :label="t('site.cookieCloudDomainLabel')" name="cookieCloudDomain">
            <a-input v-model:value="form.cookieCloudDomain" :placeholder="t('site.cookieCloudDomainPlaceholder')" />
          </a-form-item>
        </template>

        <a-divider>{{ t('site.rssSavePathOverride') }}</a-divider>
        <a-form-item :label="t('site.overrideRssUrl')" name="overrideRssUrl">
          <a-input v-model:value="form.overrideRssUrl" :placeholder="t('site.overrideRssUrlPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('site.overrideSavePath')" name="overrideSavePath">
          <a-input v-model:value="form.overrideSavePath" :placeholder="t('site.overrideSavePathPlaceholder')" />
        </a-form-item>

        <a-divider>{{ t('site.network') }}</a-divider>
        <a-form-item :label="t('site.proxyAddress')" name="proxyUrl">
          <a-input v-model:value="form.proxyUrl" :placeholder="t('site.proxyPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('site.skipSslVerify')" name="skipSslVerify">
          <a-switch v-model:checked="form.skipSslVerify" />
        </a-form-item>

        <a-divider>{{ t('site.parseStrategy') }}</a-divider>
        <a-form-item :label="t('site.hashStrategy')" name="hashStrategy">
          <a-select v-model:value="form.hashStrategy" :placeholder="t('site.defaultGuid')" allow-clear>
            <a-select-option value="guid">GUID</a-select-option>
            <a-select-option value="xml_tag">{{ t('site.xmlTag') }}</a-select-option>
            <a-select-option value="fake_from_id">{{ t('site.fakeFromId') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('site.sizeStrategy')" name="sizeStrategy">
          <a-select v-model:value="form.sizeStrategy" :placeholder="t('site.defaultEnclosure')" allow-clear>
            <a-select-option value="enclosure">{{ t('site.sizeStrategyEnclosure') }}</a-select-option>
            <a-select-option value="xml_tag">{{ t('site.xmlTag') }}</a-select-option>
            <a-select-option value="desc_regex">{{ t('site.descRegex') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('site.idStrategy')" name="idStrategy">
          <a-select v-model:value="form.idStrategy" :placeholder="t('site.defaultQueryParam')" allow-clear>
            <a-select-option value="query_param">{{ t('site.queryParam') }}</a-select-option>
            <a-select-option value="link_regex">{{ t('site.linkRegex') }}</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { sitesApi } from '@/api/sites'
import { formatBytes, formatTime } from '@/utils/format'

const { t } = useI18n()

interface SiteListItem {
  [key: string]: string | number | boolean
  id: number
  name: string
  enabled: boolean
  participateAutoPublish: boolean
  isSource: boolean
  isTarget: boolean
  hasCookie: boolean
  hasPasskey: boolean
  hasApiKey: boolean
  authType: string
  cookieCloudSync: boolean
  cookieCloudDomain: string
  overrideRssUrl: string
  overrideSavePath: string
  proxyUrl: string
  skipSslVerify: boolean
  hashStrategy: string
  sizeStrategy: string
  idStrategy: string
  username: string
  userClass: string
  uploadBytes: string | number
  downloadBytes: string | number
  ratio: number
  bonusPoints: number
  seedingCount: number
  statsSyncedAt: string
  framework: string
  domain: string
}

const allSites = ref<SiteListItem[]>([])
const loading = ref(false)
const searchText = ref('')
const modalVisible = ref(false)
const submitting = ref(false)
const syncing = ref(false)
const batchSyncing = ref(false)
const syncingSingleId = ref<number | null>(null)
const editingSite = ref<SiteListItem | null>(null)
const isCreateMode = ref(false)
const selectedRowKeys = ref<number[]>([])
const currentPage = ref(1)
const pageSize = ref(20)

const filters = reactive({
  enabled: undefined as string | undefined,
  isSource: undefined as string | undefined,
  isTarget: undefined as string | undefined,
})

const filteredData = computed(() => {
  let data = allSites.value
  const q = searchText.value.trim().toLowerCase()
  if (q) {
    data = data.filter(s => {
      const name = (s.name || '').toLowerCase()
      const domain = (s.domain || '').toLowerCase()
      const username = (s.username || '').toLowerCase()
      return name.includes(q) || domain.includes(q) || username.includes(q)
    })
  }
  if (filters.enabled === 'true') data = data.filter(s => s.enabled)
  else if (filters.enabled === 'false') data = data.filter(s => !s.enabled)
  if (filters.isSource === 'true') data = data.filter(s => s.isSource)
  else if (filters.isSource === 'false') data = data.filter(s => !s.isSource)
  if (filters.isTarget === 'true') data = data.filter(s => s.isTarget)
  else if (filters.isTarget === 'false') data = data.filter(s => !s.isTarget)
  return data
})

const form = reactive({
  name: '',
  domain: '',
  baseUrl: '',
  framework: '',
  authType: 'cookie',
  cookie: '',
  passkey: '',
  apiKey: '',
  isSource: false,
  isTarget: false,
  participateAutoPublish: true,
  enabled: true,
  cookieCloudSync: false,
  cookieCloudDomain: '',
  overrideRssUrl: '',
  overrideSavePath: '',
  proxyUrl: '',
  skipSslVerify: false,
  hashStrategy: '',
  sizeStrategy: '',
  idStrategy: '',
})

const showCookieField = computed(() => form.authType === 'cookie')
const showPasskeyField = computed(() => form.authType === 'passkey')
const showApiKeyField = computed(() => form.authType === 'apikey')
const isCookieAuth = computed(() => form.authType === 'cookie')

function hasAnyCredential(record: SiteListItem): boolean {
  return record.hasCookie || record.hasApiKey || record.hasPasskey
}

const columns = [
  { title: t('common.name'), dataIndex: 'name', key: 'name', width: 100 },
  { title: t('site.enabledLabel'), key: 'enabled', width: 70, align: 'center' as const },
  { title: t('site.participateAutoPublishLabel'), key: 'participateAutoPublish', width: 120, align: 'center' as const },
  { title: t('site.asSource'), key: 'isSource', width: 80, align: 'center' as const },
  { title: t('site.asTarget'), key: 'isTarget', width: 80, align: 'center' as const },
  { title: t('site.credentialStatus'), key: 'hasCookie', width: 90 },
  { title: t('site.username'), key: 'username', width: 90 },
  { title: t('site.userClass'), key: 'userClass', width: 90 },
  { title: t('site.uploadBytes'), key: 'uploadBytes', width: 100 },
  { title: t('site.downloadBytes'), key: 'downloadBytes', width: 100 },
  { title: t('site.ratio'), key: 'ratio', width: 70 },
  { title: t('site.seedingCount'), key: 'seedingCount', width: 80 },
  { title: t('site.seedingSize'), key: 'seedingSize', width: 100 },
  { title: t('site.bonusPoints'), key: 'bonusPoints', width: 100 },
  { title: t('site.syncTime'), key: 'syncTime', width: 120 },
  { title: t('common.actions'), key: 'actions', width: 240 },
]

async function fetchAll() {
  loading.value = true
  try {
    const resp = await sitesApi.list(1, 200)
    allSites.value = (resp.data?.data?.items || []) as unknown as SiteListItem[]
  } catch {
    allSites.value = []
  } finally {
    loading.value = false
  }
}

function onSelectionChange(keys: number[]) {
  selectedRowKeys.value = keys
}

async function batchUpdate(field: string, value: boolean) {
  if (selectedRowKeys.value.length === 0) return
  try {
    const resp = await sitesApi.batchUpdate(selectedRowKeys.value, { [field]: value })
    const updated = resp.data?.data?.updated ?? 0
    message.success(t('site.batchUpdateSuccess', { count: updated }))
    selectedRowKeys.value = []
    await fetchAll()
  } catch (e: unknown) {
    message.error(t('site.batchUpdateFailed', { error: e instanceof Error ? e.message : String(e) }))
  }
}

async function toggleField(record: SiteListItem, field: string, value: boolean) {
  try {
    await sitesApi.update(record.id, { [field]: value })
    record[field] = value
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

function openModal(record: SiteListItem) {
  isCreateMode.value = false
  editingSite.value = record
  Object.assign(form, {
    name: record.name || '',
    domain: record.domain || '',
    baseUrl: (record as Record<string, string | number | boolean>).baseUrl as string || '',
    framework: record.framework || '',
    authType: record.authType || 'cookie',
    cookie: '',
    passkey: '',
    apiKey: '',
    isSource: record.isSource || false,
    isTarget: record.isTarget || false,
    participateAutoPublish: record.participateAutoPublish !== undefined ? record.participateAutoPublish : true,
    enabled: record.enabled !== undefined ? record.enabled : true,
    cookieCloudSync: record.cookieCloudSync || false,
    cookieCloudDomain: record.cookieCloudDomain || '',
    overrideRssUrl: record.overrideRssUrl || '',
    overrideSavePath: record.overrideSavePath || '',
    proxyUrl: record.proxyUrl || '',
    skipSslVerify: record.skipSslVerify || false,
    hashStrategy: record.hashStrategy || '',
    sizeStrategy: record.sizeStrategy || '',
    idStrategy: record.idStrategy || '',
  })
  modalVisible.value = true
}

function openCreateModal() {
  isCreateMode.value = true
  editingSite.value = null
  Object.assign(form, {
    name: '',
    domain: '',
    baseUrl: '',
    framework: '',
    authType: 'cookie',
    cookie: '',
    passkey: '',
    apiKey: '',
    isSource: false,
    isTarget: false,
    participateAutoPublish: true,
    enabled: true,
    cookieCloudSync: false,
    cookieCloudDomain: '',
    overrideRssUrl: '',
    overrideSavePath: '',
    proxyUrl: '',
    skipSslVerify: false,
    hashStrategy: '',
    sizeStrategy: '',
    idStrategy: '',
  })
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (isCreateMode.value) {
      await sitesApi.create(form)
    } else if (editingSite.value) {
      const payload: Record<string, unknown> = {}
      const skipIfEmpty = ['name', 'domain', 'baseUrl', 'framework']
      for (const [key, value] of Object.entries(form)) {
        if (skipIfEmpty.includes(key) && !value) continue
        payload[key] = value
      }
      await sitesApi.update(editingSite.value.id, payload)
    }
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    await fetchAll()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    submitting.value = false
  }
}

async function deleteSite(id: number) {
  try {
    await sitesApi.delete(id)
    message.success(t('common.deleted'))
    await fetchAll()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function testConnection(id: number) {
  try {
    const resp = await sitesApi.testConnection(id)
    const data = resp.data?.data
    if (data?.success) {
      message.success(data.message || t('site.connectionTestSuccess'))
    } else {
      message.warning(data?.message || t('common.operationFailed'))
    }
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function syncAllStats() {
  syncing.value = true
  try {
    const resp = await sitesApi.syncAllStats()
    const data = resp.data?.data as Record<string, unknown>
    const synced = (data?.synced as number) ?? 0
    const failed = (data?.failed as number) ?? 0
    const failedSites = (data?.failedSites as string[]) ?? []
    if (failed > 0) {
      message.warning(`${t('site.syncAllStatsSuccess', { synced, failed })} ${failedSites.join(', ')}`)
    } else {
      message.success(t('site.syncAllStatsSuccess', { synced, failed }))
    }
    await fetchAll()
  } catch (e: unknown) {
    message.error(t('site.syncAllStatsFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    syncing.value = false
  }
}

async function batchSyncSelected() {
  if (selectedRowKeys.value.length === 0) return
  batchSyncing.value = true
  try {
    const resp = await sitesApi.batchSyncStats(selectedRowKeys.value)
    const data = resp.data?.data
    const synced = data?.synced ?? 0
    const failed = data?.failed ?? 0
    const failedSites = data?.failedSites ?? []
    if (failed > 0) {
      message.warning(`${t('site.batchSyncSuccess', { synced, failed })} ${failedSites.join(', ')}`)
    } else {
      message.success(t('site.batchSyncSuccess', { synced, failed }))
    }
    selectedRowKeys.value = []
    await fetchAll()
  } catch (e: unknown) {
    message.error(t('site.batchSyncFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    batchSyncing.value = false
  }
}

async function syncSingleStats(id: number) {
  syncingSingleId.value = id
  try {
    await sitesApi.syncSiteStats(id)
    message.success(t('site.syncSingleSuccess'))
    await fetchAll()
  } catch (e: unknown) {
    message.error(t('site.syncSingleFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    syncingSingleId.value = null
  }
}

onMounted(() => fetchAll())
</script>
