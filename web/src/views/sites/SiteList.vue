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
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="$router.push(`/sites/${record.id}`)">{{ t('common.detail') }}</a-button>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" @click="testConnection(record.id)">{{ t('common.test') }}</a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="t('site.editSite')"
      :confirm-loading="submitting"
      width="640px"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('site.mirrorDomain')" name="mirrorDomain">
          <a-input v-model:value="form.mirrorDomain" :placeholder="t('site.mirrorDomainPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('site.authType')" name="authType">
          <a-select v-model:value="form.authType" :placeholder="t('site.selectAuthType')">
            <a-select-option value="cookie">Cookie</a-select-option>
            <a-select-option value="apikey">API Key</a-select-option>
            <a-select-option value="passkey">Passkey</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="showCookieField" label="Cookie" name="cookie">
          <a-textarea v-model:value="form.cookie" :rows="3" :placeholder="editingSite?.hasCookie ? t('site.placeholderConfigured') : t('site.placeholderCookie')" />
        </a-form-item>
        <a-form-item v-if="showPasskeyField" label="Passkey" name="passkey">
          <a-input-password v-model:value="form.passkey" :placeholder="editingSite?.hasPasskey ? t('site.placeholderConfigured') : t('site.placeholderPasskey')" />
        </a-form-item>
        <a-form-item v-if="showApiKeyField" label="API Key" name="apiKey">
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
            <a-select-option value="enclosure">Enclosure</a-select-option>
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
import { usePagination } from '@/composables/usePagination'

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
  mirrorDomain: string
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
}

const searchText = ref('')
const modalVisible = ref(false)
const submitting = ref(false)
const editingSite = ref<SiteListItem | null>(null)

const form = reactive({
  mirrorDomain: '',
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
  { title: t('common.name'), dataIndex: 'name', key: 'name' },
  { title: t('site.enabledLabel'), key: 'enabled', width: 80, align: 'center' as const },
  { title: t('site.participateAutoPublishLabel'), key: 'participateAutoPublish', width: 130, align: 'center' as const },
  { title: t('site.asSource'), key: 'isSource', width: 100, align: 'center' as const },
  { title: t('site.asTarget'), key: 'isTarget', width: 110, align: 'center' as const },
  { title: t('site.credentialStatus'), key: 'hasCookie', width: 100 },
  { title: t('common.actions'), key: 'actions', width: 180 },
]

const pagination = usePagination((page, size) => sitesApi.list(page, size, searchText.value))

async function toggleField(record: SiteListItem, field: string, value: boolean) {
  try {
    await sitesApi.update(record.id, { [field]: value })
    record[field] = value
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

function openModal(record: SiteListItem) {
  editingSite.value = record
  Object.assign(form, {
    mirrorDomain: record.mirrorDomain || '',
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

async function handleSubmit() {
  if (!editingSite.value) return
  submitting.value = true
  try {
    await sitesApi.update(editingSite.value.id, form)
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    pagination.fetch()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    submitting.value = false
  }
}

async function testConnection(id: number) {
  try {
    const resp = await sitesApi.testConnection(id)
    const data = resp.data?.data
    if (data?.ok) {
      message.success(data.message || t('site.connectionTestSuccess'))
    } else {
      message.warning(data?.message || t('common.operationFailed'))
    }
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

onMounted(() => pagination.fetch())
</script>
