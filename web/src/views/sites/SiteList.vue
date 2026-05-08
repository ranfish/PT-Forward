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
      @change="(pag: any) => pagination.onPageChange(pag.current, pag.pageSize)"
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
      @ok="handleSubmit"
      :confirm-loading="submitting"
      width="640px"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item label="镜像域名" name="mirrorDomain">
          <a-input v-model:value="form.mirrorDomain" placeholder="例如: mirror.pterclub.net（留空使用原始域名）" />
        </a-form-item>
        <a-form-item :label="t('site.authType')" name="authType">
          <a-select v-model:value="form.authType" placeholder="选择认证方式">
            <a-select-option value="cookie">Cookie</a-select-option>
            <a-select-option value="apikey">API Key</a-select-option>
            <a-select-option value="passkey">Passkey</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="showCookieField" label="Cookie" name="cookie">
          <a-textarea v-model:value="form.cookie" :rows="3" :placeholder="editingSite?.hasCookie ? '••••••••（已配置，留空保持不变）' : '站点 Cookie'" />
        </a-form-item>
        <a-form-item v-if="showPasskeyField" label="Passkey" name="passkey">
          <a-input-password v-model:value="form.passkey" :placeholder="editingSite?.hasPasskey ? '••••••••（已配置，留空保持不变）' : '站点 Passkey'" />
        </a-form-item>
        <a-form-item v-if="showApiKeyField" label="API Key" name="apiKey">
          <a-input-password v-model:value="form.apiKey" :placeholder="editingSite?.hasApiKey ? '••••••••（已配置，留空保持不变）' : '站点 API Key（馒头使用 x-api-key）'" />
        </a-form-item>

        <a-divider>角色与发布</a-divider>
        <a-form-item label="启用" name="enabled">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
        <a-form-item label="作为源站" name="isSource">
          <a-switch v-model:checked="form.isSource" />
        </a-form-item>
        <a-form-item label="作为目标站" name="isTarget">
          <a-switch v-model:checked="form.isTarget" />
        </a-form-item>
        <a-form-item label="参与自动发布" name="participateAutoPublish">
          <a-switch v-model:checked="form.participateAutoPublish" />
        </a-form-item>

        <a-divider v-if="isCookieAuth">CookieCloud 同步</a-divider>
        <template v-if="isCookieAuth">
          <a-form-item label="CookieCloud 同步" name="cookieCloudSync">
            <a-switch v-model:checked="form.cookieCloudSync" />
          </a-form-item>
          <a-form-item label="CookieCloud 域名" name="cookieCloudDomain">
            <a-input v-model:value="form.cookieCloudDomain" placeholder="CookieCloud 中对应的域名" />
          </a-form-item>
        </template>

        <a-divider>RSS / 保存路径覆盖</a-divider>
        <a-form-item label="覆盖 RSS 地址" name="overrideRssUrl">
          <a-input v-model:value="form.overrideRssUrl" placeholder="自定义 RSS URL（留空使用默认）" />
        </a-form-item>
        <a-form-item label="覆盖保存路径" name="overrideSavePath">
          <a-input v-model:value="form.overrideSavePath" placeholder="自定义保存路径（留空使用默认）" />
        </a-form-item>

        <a-divider>网络</a-divider>
        <a-form-item label="代理地址" name="proxyUrl">
          <a-input v-model:value="form.proxyUrl" placeholder="例如: socks5://127.0.0.1:1080" />
        </a-form-item>
        <a-form-item label="跳过 SSL 验证" name="skipSslVerify">
          <a-switch v-model:checked="form.skipSslVerify" />
        </a-form-item>

        <a-divider>解析策略</a-divider>
        <a-form-item label="Hash 策略" name="hashStrategy">
          <a-select v-model:value="form.hashStrategy" placeholder="默认: guid" allow-clear>
            <a-select-option value="guid">GUID</a-select-option>
            <a-select-option value="xml_tag">XML 标签</a-select-option>
            <a-select-option value="fake_from_id">根据 ID 生成</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="Size 策略" name="sizeStrategy">
          <a-select v-model:value="form.sizeStrategy" placeholder="默认: enclosure" allow-clear>
            <a-select-option value="enclosure">Enclosure</a-select-option>
            <a-select-option value="xml_tag">XML 标签</a-select-option>
            <a-select-option value="desc_regex">描述正则</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="ID 策略" name="idStrategy">
          <a-select v-model:value="form.idStrategy" placeholder="默认: query_param" allow-clear>
            <a-select-option value="query_param">查询参数</a-select-option>
            <a-select-option value="link_regex">链接正则</a-select-option>
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

const searchText = ref('')
const modalVisible = ref(false)
const submitting = ref(false)
const editingSite = ref<any>(null)

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

function hasAnyCredential(record: any): boolean {
  return record.hasCookie || record.hasApiKey || record.hasPasskey
}

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '启用', key: 'enabled', width: 80, align: 'center' as const },
  { title: '参与自动发布', key: 'participateAutoPublish', width: 130, align: 'center' as const },
  { title: '作为源站', key: 'isSource', width: 100, align: 'center' as const },
  { title: '作为目标站', key: 'isTarget', width: 110, align: 'center' as const },
  { title: '凭据状态', key: 'hasCookie', width: 100 },
  { title: '操作', key: 'actions', width: 180 },
]

const pagination = usePagination((page, size) => sitesApi.list(page, size, searchText.value))

async function toggleField(record: any, field: string, value: boolean) {
  try {
    await sitesApi.update(record.id, { [field]: value })
    record[field] = value
  } catch (e: any) {
    message.error(e.message)
  }
}

function openModal(record: any) {
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
  submitting.value = true
  try {
    await sitesApi.update(editingSite.value.id, form)
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
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
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
