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
      <template #extra>
        <a-button :loading="detecting" @click="runDetect">{{ t('site.detect') }}</a-button>
      </template>
    </a-page-header>

    <a-modal v-model:open="showDetectResult" :title="t('site.detectResult')" :footer="null" width="520px">
      <a-descriptions bordered :column="1" size="small">
        <a-descriptions-item :label="t('site.detectFramework')">
          <a-tag color="blue">{{ detectResult.framework }}</a-tag>
        </a-descriptions-item>
        <a-descriptions-item :label="t('site.detectConfidence')">{{ ((detectResult.confidence ?? 0) * 100).toFixed(0) }}%</a-descriptions-item>
        <a-descriptions-item v-if="detectResult.detectionDetail" :label="t('site.detectDetail')">{{ detectResult.detectionDetail }}</a-descriptions-item>
      </a-descriptions>
    </a-modal>

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('common.name')">{{ site.name }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.framework')">
          <a-tag :color="frameworkColors[site.framework as string] || 'default'">
            {{ frameworkLabels[site.framework as string] || site.framework }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item :label="t('site.authType')">{{ authLabel }}</a-descriptions-item>
        <a-descriptions-item v-if="site.mirrorDomain" :label="t('site.mirrorDomain')">{{ site.mirrorDomain }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie" label="Cookie">{{ site.hasCookie ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsApiKey" label="API Key">{{ site.hasApiKey ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsPasskey" label="Passkey">{{ site.hasPasskey ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.enabledLabel')"><a-badge :status="site.enabled ? 'success' : 'default'" :text="site.enabled ? t('common.yes') : t('common.no')" /></a-descriptions-item>
        <a-descriptions-item :label="t('site.role')">{{ [site.isSource ? t('site.sourceSiteRole') : '', site.isTarget ? t('site.targetSiteRole') : ''].filter(Boolean).join(', ') || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.participateAutoPublishLabel')">{{ site.participateAutoPublish ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie" :label="t('site.cookieCloudSyncLabel')">{{ site.cookieCloudSync ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie && site.cookieCloudSync" :label="t('site.cookieCloudDomainLabel')">{{ site.cookieCloudDomain || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.hashStrategy')">{{ site.hashStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.sizeStrategy')">{{ site.sizeStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.idStrategy')">{{ site.idStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.overrideRssUrl')">{{ site.overrideRssUrl || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.overrideSavePath')">{{ site.overrideSavePath || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.proxyAddress')">{{ site.proxyUrl || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.skipSslVerify')">{{ site.skipSslVerify ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.lastSync')">{{ site.lastSyncAt || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ site.createdAt || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-card :title="t('site.siteStats')" style="margin-bottom: 24px">
        <a-spin :spinning="statsLoading">
          <a-row :gutter="16">
            <a-col :span="4"><a-statistic :title="t('site.uploadBytes')" :value="formatBytes(stats.uploadBytes)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.downloadBytes')" :value="formatBytes(stats.downloadBytes)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.ratio')" :value="stats.ratio?.toFixed(2) || '-'" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.seedingCount')" :value="stats.seedingCount ?? '-'" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.seedingPoints')" :value="stats.seedingPoints ?? '-'" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.bonusPoints')" :value="stats.bonusPoints ?? '-'" /></a-col>
          </a-row>
          <a-row :gutter="16" style="margin-top: 12px">
            <a-col :span="4"><a-statistic :title="t('site.seedingSize')" :value="formatBytes(stats.seedingSize)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.userClass')" :value="stats.userClass || '-'" /></a-col>
            <a-col :span="8"><a-statistic :title="t('site.statsSyncedAt')" :value="stats.statsSyncedAt || '-'" /></a-col>
          </a-row>
        </a-spin>
      </a-card>

      <a-card :title="t('site.siteSettings')" style="margin-bottom: 24px">
        <a-form :model="settingsForm" layout="vertical" style="max-width: 500px">
          <a-form-item :label="t('site.mirrorDomain')">
            <a-input v-model:value="settingsForm.mirrorDomain" :placeholder="t('site.mirrorDomainPlaceholder')" />
          </a-form-item>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.enabledLabel')">
                <a-switch v-model:checked="settingsForm.enabled" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.participateAutoPublishLabel')">
                <a-switch v-model:checked="settingsForm.participateAutoPublish" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.asSource')">
                <a-switch v-model:checked="settingsForm.isSource" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.asTarget')">
                <a-switch v-model:checked="settingsForm.isTarget" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row v-if="needsCookie" :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.cookieCloudSyncLabel')">
                <a-switch v-model:checked="settingsForm.cookieCloudSync" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.cookieCloudDomainLabel')">
                <a-input v-model:value="settingsForm.cookieCloudDomain" :placeholder="t('site.cookieCloudDomainShortPlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item :label="t('site.overrideRssUrl')">
            <a-input v-model:value="settingsForm.overrideRssUrl" :placeholder="t('site.customRssUrl')" />
          </a-form-item>
          <a-form-item :label="t('site.overrideSavePath')">
            <a-input v-model:value="settingsForm.overrideSavePath" :placeholder="t('site.customSavePath')" />
          </a-form-item>
          <a-form-item :label="t('site.proxyAddress')">
            <a-input v-model:value="settingsForm.proxyUrl" :placeholder="t('site.proxyPlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('site.skipSslVerify')">
            <a-switch v-model:checked="settingsForm.skipSslVerify" />
          </a-form-item>
          <a-row :gutter="16">
            <a-col :span="8">
              <a-form-item :label="t('site.hashStrategy')">
                <a-select v-model:value="settingsForm.hashStrategy" allow-clear>
                  <a-select-option value="guid">GUID</a-select-option>
                  <a-select-option value="xml_tag">{{ t('site.xmlTag') }}</a-select-option>
                  <a-select-option value="fake_from_id">{{ t('site.fakeFromId') }}</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('site.sizeStrategy')">
                <a-select v-model:value="settingsForm.sizeStrategy" allow-clear>
                  <a-select-option value="enclosure">Enclosure</a-select-option>
                  <a-select-option value="xml_tag">{{ t('site.xmlTag') }}</a-select-option>
                  <a-select-option value="desc_regex">{{ t('site.descRegex') }}</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('site.idStrategy')">
                <a-select v-model:value="settingsForm.idStrategy" allow-clear>
                  <a-select-option value="query_param">{{ t('site.queryParam') }}</a-select-option>
                  <a-select-option value="link_regex">{{ t('site.linkRegex') }}</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item>
            <a-button type="primary" @click="updateSettings">{{ t('site.saveSettings') }}</a-button>
          </a-form-item>
        </a-form>
      </a-card>

      <a-card :title="t('site.credentialManagement')" style="margin-bottom: 24px">
        <a-form :model="credForm" layout="vertical" style="max-width: 500px">
          <a-form-item v-if="needsCookie" label="Cookie">
            <a-textarea v-model:value="credForm.cookie" :rows="4" :placeholder="t('site.inputCookie')" />
          </a-form-item>
          <a-form-item v-if="needsApiKey" label="API Key">
            <a-input-password v-model:value="credForm.apiKey" :placeholder="t('site.inputApiKey')" />
          </a-form-item>
          <a-form-item v-if="needsPasskey" label="Passkey">
            <a-input v-model:value="credForm.passkey" :placeholder="t('site.inputPasskey')" />
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
        <a-textarea v-model:value="overrideJSON" :rows="12" placeholder="{&quot;upload_rule&quot;: &quot;...&quot;, &quot;download_prefix&quot;: &quot;...&quot;}" />
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

interface DetectResultData {
  framework?: string
  confidence?: number
  detectionDetail?: string
}

interface SiteStatsData {
  uploadBytes?: number
  downloadBytes?: number
  ratio?: number
  seedingCount?: number
  seedingPoints?: number
  bonusPoints?: number
  seedingSize?: number
  userClass?: string
  statsSyncedAt?: string
}

const { t } = useI18n()

const route = useRoute()
const siteId = Number(route.params.id)

const loading = ref(false)
const site = ref<Record<string, unknown>>({})

const credForm = reactive({ cookie: '', passkey: '', apiKey: '' })

const settingsForm = reactive({
  enabled: true,
  isSource: false,
  isTarget: false,
  participateAutoPublish: true,
  mirrorDomain: '',
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

const authTypeLabels: Record<string, string> = {
  cookie: 'Cookie',
  apikey: 'API Key',
  passkey: 'Passkey',
}

const needsCookie = computed(() => site.value.authType === 'cookie' || !site.value.authType)
const needsApiKey = computed(() => site.value.authType === 'apikey')
const needsPasskey = computed(() => site.value.authType === 'passkey')
const authLabel = computed(() => authTypeLabels[site.value.authType as string] || 'Cookie')

const overrideLoading = ref(false)
const overrideSaving = ref(false)
const overrideData = ref<Record<string, unknown>>({})
const overrideJSON = ref('{}')
const showOverrideEditor = ref(false)

const hasOverrides = computed(() => Object.keys(overrideData.value).length > 0)

const detecting = ref(false)
const showDetectResult = ref(false)
const detectResult = ref<DetectResultData>({})
const statsLoading = ref(false)
const stats = ref<SiteStatsData>({})

import { formatBytes } from '@/utils/format'
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
    Object.assign(settingsForm, {
      enabled: site.value.enabled !== undefined ? site.value.enabled : true,
      isSource: site.value.isSource || false,
      isTarget: site.value.isTarget || false,
      participateAutoPublish: site.value.participateAutoPublish !== undefined ? site.value.participateAutoPublish : true,
      mirrorDomain: site.value.mirrorDomain || '',
      cookieCloudSync: site.value.cookieCloudSync || false,
      cookieCloudDomain: site.value.cookieCloudDomain || '',
      overrideRssUrl: site.value.overrideRssUrl || '',
      overrideSavePath: site.value.overrideSavePath || '',
      proxyUrl: site.value.proxyUrl || '',
      skipSslVerify: site.value.skipSslVerify || false,
      hashStrategy: site.value.hashStrategy || '',
      sizeStrategy: site.value.sizeStrategy || '',
      idStrategy: site.value.idStrategy || '',
    })
  } catch (e: unknown) {
    message.error((e as Error).message)
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
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function updateSettings() {
  try {
    await sitesApi.update(siteId, {
      enabled: settingsForm.enabled,
      isSource: settingsForm.isSource,
      isTarget: settingsForm.isTarget,
      participateAutoPublish: settingsForm.participateAutoPublish,
      mirrorDomain: settingsForm.mirrorDomain,
      cookieCloudSync: settingsForm.cookieCloudSync,
      cookieCloudDomain: settingsForm.cookieCloudDomain,
      overrideRssUrl: settingsForm.overrideRssUrl,
      overrideSavePath: settingsForm.overrideSavePath,
      proxyUrl: settingsForm.proxyUrl,
      skipSslVerify: settingsForm.skipSslVerify,
      hashStrategy: settingsForm.hashStrategy,
      sizeStrategy: settingsForm.sizeStrategy,
      idStrategy: settingsForm.idStrategy,
    })
    message.success(t('common.configSaved'))
    fetchSite()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function fetchOverrides() {
  overrideLoading.value = true
  try {
    const resp = await sitesApi.getOverrides(siteId)
    const data = resp.data?.data || {}
    if (data && typeof data === 'object' && !Array.isArray(data)) {
      const filtered: Record<string, unknown> = {}
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
  } catch (e: unknown) {
    if (e instanceof SyntaxError) {
      message.error(t('common.jsonFormatError'))
    } else {
      message.error((e as { response?: { data?: { message?: string } } }).response?.data?.message || t('common.saveFailed'))
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
  } catch (e: unknown) {
    message.error((e as { response?: { data?: { message?: string } } }).response?.data?.message || t('common.deleteFailed'))
  }
}

async function runDetect() {
  detecting.value = true
  try {
    const resp = await sitesApi.detect(siteId)
    detectResult.value = resp.data.data || {}
    showDetectResult.value = true
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    detecting.value = false
  }
}

async function fetchStats() {
  statsLoading.value = true
  try {
    const resp = await sitesApi.getStats(siteId)
    stats.value = resp.data.data || {}
  } catch {
    stats.value = {}
  } finally {
    statsLoading.value = false
  }
}

onMounted(() => {
  fetchSite()
  fetchOverrides()
  fetchStats()
})
</script>
