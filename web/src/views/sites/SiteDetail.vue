<template>
  <div>
    <a-page-header :title="site.name || site.domain" @back="$router.push('/sites')">
      <template #tags>
        <a-tag v-if="needsCookie && site.hasCookie" color="green">{{ t('site.cookieValid') }}</a-tag>
        <a-tag v-else-if="needsCookie" color="red">{{ t('site.cookieNotConfigured') }}</a-tag>
        <a-tag v-if="needsApiKey && site.hasApiKey" color="green">{{ t('site.apiKeyValid') }}</a-tag>
        <a-tag v-else-if="needsApiKey" color="red">{{ t('site.apiKeyNotConfigured') }}</a-tag>
        <a-tag v-if="needsPasskey && site.hasPasskey" color="green">{{ t('site.passkeyValid') }}</a-tag>
        <a-tag v-else-if="needsPasskey" color="red">{{ t('site.passkeyNotConfigured') }}</a-tag>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('site.domain')">{{ site.domain }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.name')">{{ site.name }}</a-descriptions-item>
        <a-descriptions-item label="URL">{{ site.baseUrl }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.framework')">
          <a-tag :color="frameworkColors[site.framework] || 'default'">
            {{ frameworkLabels[site.framework] || site.framework }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item :label="t('site.authType')">{{ authLabel }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie" label="Cookie">{{ site.hasCookie ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsApiKey" label="API Key">{{ site.hasApiKey ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsPasskey" label="Passkey">{{ site.hasPasskey ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.lastSync')">{{ site.lastSyncAt || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ site.createdAt || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-card :title="t('site.credentialManagement')" style="margin-bottom: 24px">
        <a-form :model="credForm" layout="vertical" style="max-width: 500px">
          <a-form-item v-if="needsCookie" label="Cookie">
            <a-textarea v-model:value="credForm.cookie" :rows="4" placeholder="输入新的 Cookie" />
          </a-form-item>
          <a-form-item v-if="needsApiKey" label="API Key">
            <a-input-password v-model:value="credForm.apiKey" placeholder="输入 API Key" />
          </a-form-item>
          <a-form-item v-if="needsPasskey" label="Passkey">
            <a-input v-model:value="credForm.passkey" placeholder="输入 Passkey" />
          </a-form-item>
          <a-form-item>
            <a-button type="primary" @click="updateCredentials">{{ t('site.saveCredentials') }}</a-button>
          </a-form-item>
        </a-form>
      </a-card>

      <a-card :title="t('site.siteConfigOverride')" style="margin-bottom: 24px">
        <template #extra>
          <a-space>
            <a-button size="small" @click="showOverrideEditor = true">{{ t('site.editOverrideConfig') }}</a-button>
            <a-popconfirm v-if="hasOverrides" :title="t('site.deleteAllOverrideConfirm')" @confirm="deleteOverrides">
              <a-button size="small" danger>{{ t('site.deleteOverride') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
        <div v-if="overrideLoading">
          <a-spin />
        </div>
        <div v-else-if="hasOverrides">
          <a-descriptions bordered size="small" :column="2">
            <a-descriptions-item v-for="(val, key) in overrideData" :key="key" :label="String(key)">
              {{ typeof val === 'object' ? JSON.stringify(val) : String(val) }}
            </a-descriptions-item>
          </a-descriptions>
        </div>
        <a-empty v-else :description="t('site.noOverrideConfig')" />
      </a-card>

      <a-modal
        v-model:open="showOverrideEditor"
        :title="t('site.editOverrideConfig')"
        :confirm-loading="overrideSaving"
        width="640px"
        @ok="saveOverrides"
      >
        <a-alert :message="t('site.overrideConfigHint')" type="info" show-icon style="margin-bottom: 16px" />
        <a-textarea v-model:value="overrideJSON" :rows="12" placeholder='{"upload_rule": "...", "download_prefix": "..."}' />
      </a-modal>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { sitesApi } from '@/api/sites'

const { t } = useI18n()

const route = useRoute()
const siteId = Number(route.params.id)

const loading = ref(false)
const site = ref<any>({})

const credForm = reactive({ cookie: '', passkey: '', apiKey: '' })

const authTypeLabels: Record<string, string> = {
  cookie: 'Cookie',
  apikey: 'API Key',
  passkey: 'Passkey',
}

const needsCookie = computed(() => site.value.authType === 'cookie' || !site.value.authType)
const needsApiKey = computed(() => site.value.authType === 'apikey')
const needsPasskey = computed(() => site.value.authType === 'passkey')
const authLabel = computed(() => authTypeLabels[site.value.authType] || 'Cookie')

const overrideLoading = ref(false)
const overrideSaving = ref(false)
const overrideData = ref<Record<string, any>>({})
const overrideJSON = ref('{}')
const showOverrideEditor = ref(false)

const hasOverrides = computed(() => Object.keys(overrideData.value).length > 0)

const frameworkColors: Record<string, string> = {
  nexusphp: 'blue', unit3d: 'green', gazelle: 'purple',
  mteam: 'orange', rousi: 'pink', tnode: 'cyan', luminance: 'magenta', generic: 'default',
}

const frameworkLabels: Record<string, string> = {
  nexusphp: 'NexusPHP', unit3d: 'UNIT3D', gazelle: 'Gazelle',
  mteam: 'M-Team', rousi: 'Rousi', tnode: 'TNode', luminance: 'Luminance', generic: 'Generic',
}

async function fetchSite() {
  loading.value = true
  try {
    const resp = await sitesApi.get(siteId)
    site.value = resp.data.data || {}
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function updateCredentials() {
  try {
    const payload: Record<string, string> = {}
    if (credForm.cookie) payload.cookie = credForm.cookie
    if (credForm.passkey) payload.passkey = credForm.passkey
    if (credForm.apiKey) payload.apiKey = credForm.apiKey
    await sitesApi.updateCredentials(siteId, payload)
    message.success(t('site.credentialsUpdated'))
    credForm.cookie = ''
    credForm.passkey = ''
    credForm.apiKey = ''
    fetchSite()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function fetchOverrides() {
  overrideLoading.value = true
  try {
    const resp = await sitesApi.getOverrides(siteId)
    const data = resp.data?.data || {}
    if (data && typeof data === 'object' && !Array.isArray(data)) {
      const filtered: Record<string, any> = {}
      for (const [k, v] of Object.entries(data)) {
        if (k !== 'id' && k !== 'site_id' && k !== 'created_at' && k !== 'updated_at') {
          filtered[k] = v
        }
      }
      overrideData.value = filtered
      overrideJSON.value = Object.keys(filtered).length > 0 ? JSON.stringify(filtered, null, 2) : '{}'
    } else {
      overrideData.value = {}
      overrideJSON.value = '{}'
    }
  } catch {
    overrideData.value = {}
    overrideJSON.value = '{}'
  } finally {
    overrideLoading.value = false
  }
}

async function saveOverrides() {
  overrideSaving.value = true
  try {
    const parsed = JSON.parse(overrideJSON.value)
    await sitesApi.updateOverrides(siteId, parsed)
    message.success(t('site.overrideConfigSaved'))
    showOverrideEditor.value = false
    fetchOverrides()
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      message.error(t('common.jsonFormatError'))
    } else {
      message.error(e?.response?.data?.message || t('common.saveFailed'))
    }
  } finally {
    overrideSaving.value = false
  }
}

async function deleteOverrides() {
  try {
    await sitesApi.deleteOverrides(siteId)
    message.success(t('site.overrideConfigDeleted'))
    overrideData.value = {}
    overrideJSON.value = '{}'
  } catch (e: any) {
    message.error(e?.response?.data?.message || t('common.deleteFailed'))
  }
}

onMounted(() => {
  fetchSite()
  fetchOverrides()
})
</script>
