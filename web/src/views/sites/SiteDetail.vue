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
        <a-tag v-if="showAuthKey && site.hasAuthKey" color="green">{{ t('site.authKeyValid') }}</a-tag>
        <a-tag v-if="showRssKey && site.hasRssKey" color="green">{{ t('site.rssKeyValid') }}</a-tag>
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
      <div style="margin-bottom: 16px; display: flex; gap: 8px; align-items: center;">
        <a-button v-if="siteFrozen" type="primary" danger size="small" :loading="freezeLoading" @click="handleUnfreeze">{{ t('site.unfreeze') }}</a-button>
        <a-button v-else size="small" :loading="freezeLoading" @click="showFreezeModal = true">{{ t('site.freeze') }}</a-button>
        <a-tag v-if="siteFrozen" color="red">{{ t('site.frozen') }}</a-tag>
      </div>

      <a-modal v-model:open="showFreezeModal" :title="t('site.freezeTitle')" @ok="handleFreeze">
        <a-form layout="vertical">
          <a-form-item :label="t('site.freezeDuration')">
            <a-input v-model:value="freezeForm.duration" placeholder="1h / 30m / 2d" />
          </a-form-item>
          <a-form-item :label="t('site.freezeReason')">
            <a-textarea v-model:value="freezeForm.reason" :rows="2" />
          </a-form-item>
        </a-form>
      </a-modal>

      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('common.name')">{{ site.name }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.framework')">
          <a-tag :color="frameworkColors[site.framework as string] || 'default'">
            {{ frameworkLabels[site.framework as string] || site.framework }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item :label="t('site.authType')">{{ authLabel }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie" :label="t('sites.cookie')">{{ site.hasCookie ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsApiKey" :label="apiKeyLabel">{{ site.hasApiKey ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsPasskey" :label="t('sites.passkey')">{{ site.hasPasskey ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="showAuthKey && site.hasAuthKey" :label="t('sites.authKey')">{{ t('common.configured') }}</a-descriptions-item>
        <a-descriptions-item v-if="showRssKey && site.hasRssKey" :label="rssKeyLabel">{{ t('common.configured') }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.enabledLabel')"><a-badge :status="site.enabled ? 'success' : 'default'" :text="site.enabled ? t('common.yes') : t('common.no')" /></a-descriptions-item>
        <a-descriptions-item :label="t('site.role')">{{ [site.isSource ? t('site.sourceSiteRole') : '', site.isTarget ? t('site.targetSiteRole') : ''].filter(Boolean).join(', ') || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.participateAutoPublishLabel')">{{ site.participateAutoPublish ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie" :label="t('site.cookieCloudSyncLabel')">{{ site.cookieCloudSync ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.proxyAddress')">{{ site.proxyUrl || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.skipSslVerify')">{{ site.skipSslVerify ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.maxConcurrent')">{{ site.maxConcurrent || 2 }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ formatTime(site.createdAt) }}</a-descriptions-item>
      </a-descriptions>

      <a-card :title="t('site.siteStats')" style="margin-bottom: 24px">
        <template #extra>
          <a-button size="small" :loading="statsSyncing" @click="syncStats">{{ t('site.syncStats') }}</a-button>
        </template>
        <a-spin :spinning="statsLoading">
          <a-row :gutter="16">
            <a-col :span="4"><a-statistic :title="t('site.uploadBytes')" :value="formatBytes(stats.uploadBytes)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.downloadBytes')" :value="formatBytes(stats.downloadBytes)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.ratio')" :value="Number(stats.uploadBytes ?? 0) > 0 && Number(stats.downloadBytes ?? 0) === 0 ? '∞' : (Number.isFinite(stats.ratio) ? stats.ratio!.toFixed(2) : '-')" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.seedingCount')" :value="stats.seedingCount ?? '-'" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.seedingPoints')" :value="stats.seedingPoints ?? '-'" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.bonusPoints')" :value="stats.bonusPoints ?? '-'" /></a-col>
          </a-row>
          <a-row :gutter="16" style="margin-top: 12px">
            <a-col :span="4"><a-statistic :title="t('site.seedingSize')" :value="formatBytes(stats.seedingSize)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.userClass')" :value="stats.userClass || '-'" /></a-col>
            <a-col :span="8"><a-statistic :title="t('site.statsSyncedAt')" :value="formatTime(stats.statsSyncedAt)" /></a-col>
          </a-row>
        </a-spin>
      </a-card>

      <a-card :title="t('site.siteSettings')" style="margin-bottom: 24px">
        <a-form :model="settingsForm" layout="vertical">
          <div class="section-title">{{ t('site.basicInfo') }}</div>
          <a-row :gutter="24">
            <a-col :span="8">
              <a-form-item :label="t('common.name')">
                <a-input v-model:value="settingsForm.name" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('site.baseUrl')">
                <a-input v-model:value="settingsForm.baseUrl" :placeholder="t('site.baseUrlPlaceholder')" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('site.alternativeDomains')">
                <a-input v-model:value="settingsForm.alternativeDomains" :placeholder="t('site.alternativeDomainsPlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>

          <div class="section-title">{{ t('site.roleAndPublish') }}</div>
          <a-row :gutter="24">
            <a-col :span="6">
              <a-form-item :label="t('site.enabledLabel')">
                <a-switch v-model:checked="settingsForm.enabled" />
              </a-form-item>
            </a-col>
            <a-col :span="6">
              <a-form-item :label="t('site.participateAutoPublishLabel')">
                <a-switch v-model:checked="settingsForm.participateAutoPublish" />
              </a-form-item>
            </a-col>
            <a-col :span="6">
              <a-form-item :label="t('site.asSource')">
                <a-switch v-model:checked="settingsForm.isSource" />
              </a-form-item>
            </a-col>
            <a-col :span="6">
              <a-form-item :label="t('site.asTarget')">
                <a-switch v-model:checked="settingsForm.isTarget" />
              </a-form-item>
            </a-col>
          </a-row>

          <div v-if="needsCookie" class="section-title">{{ t('site.cookieCloudSyncLabel') }}</div>
          <a-row v-if="needsCookie" :gutter="24">
            <a-col :span="6">
              <a-form-item :label="t('site.cookieCloudSyncLabel')">
                <a-switch v-model:checked="settingsForm.cookieCloudSync" />
              </a-form-item>
            </a-col>
          </a-row>

          <div class="section-title">{{ t('site.network') }}</div>
          <a-row :gutter="24">
            <a-col :span="8">
              <a-form-item :label="t('site.proxyAddress')">
                <a-input v-model:value="settingsForm.proxyUrl" :placeholder="t('site.proxyPlaceholder')" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('site.skipSslVerify')">
                <a-switch v-model:checked="settingsForm.skipSslVerify" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('site.maxConcurrent')">
                <a-input-number v-model:value="settingsForm.maxConcurrent" :min="1" :max="100" style="width: 100%" />
                <div style="font-size: 11px; color: #999; margin-top: 2px">{{ t('site.maxConcurrentHint') }}</div>
              </a-form-item>
            </a-col>
          </a-row>

          <div class="section-title">{{ t('site.hrStrategy') }}</div>
          <a-form-item :label="t('site.hrStrategy')">
            <a-select v-model:value="settingsForm.hrStrategy" allow-clear :placeholder="t('site.hrStrategyPlaceholder')" style="max-width: 400px">
              <a-select-option value="protect">{{ t('site.hrProtect') }}</a-select-option>
              <a-select-option value="ignore">{{ t('site.hrIgnore') }}</a-select-option>
              <a-select-option value="skip">{{ t('site.hrStrict') }}</a-select-option>
            </a-select>
          </a-form-item>

          <a-form-item>
            <a-button type="primary" @click="updateSettings">{{ t('site.saveSettings') }}</a-button>
          </a-form-item>
        </a-form>
      </a-card>

      <a-card :title="needsCookie ? t('site.credentialManagementHint') : t('site.credentialManagement')" style="margin-bottom: 24px">
        <a-form :model="credForm" layout="vertical">
          <div class="section-title">{{ t('site.basicCredentials') }}</div>
          <a-row :gutter="24">
            <a-col v-if="needsCookie" :span="24">
              <a-form-item :label="t('sites.cookie')">
                <a-textarea v-model:value="credForm.cookie" :rows="3" :placeholder="site.cookieMasked || t('site.inputCookie')" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="24">
            <a-col v-if="needsApiKey" :span="12">
              <a-form-item :label="apiKeyLabel">
                <a-input v-model:value="credForm.apiKey" :placeholder="apiKeyHint" />
              </a-form-item>
            </a-col>
            <a-col v-if="showPasskeyField" :span="12">
              <a-form-item :label="site.passkeyAlias || t('sites.passkey')">
                <template v-if="site.passkeyHint" #extra>
                  <span style="color: rgba(0,0,0,0.45); font-size: 12px;">{{ site.passkeyHint }}</span>
                </template>
                <a-input
                  v-model:value="credForm.passkey"
                  :placeholder="site.passkeyMasked || (site.passkeyAlias ? `${t('site.inputPasskey')}${site.passkeyAlias}` : t('site.inputPasskey'))"
                />
              </a-form-item>
            </a-col>
          </a-row>

          <template v-if="showAdvancedCredentials">
            <div class="section-title">{{ t('site.advancedCredentials') }}</div>
            <a-row :gutter="24">
              <a-col v-if="showBearerToken" :span="12">
                <a-form-item :label="t('site.bearerToken')">
                  <a-input v-model:value="credForm.bearerToken" :placeholder="t('site.bearerTokenPlaceholder')" />
                </a-form-item>
              </a-col>
              <a-col v-if="showAuthKey" :span="6">
                <a-form-item :label="t('site.authKey')">
                  <a-input v-model:value="credForm.authKey" :placeholder="site.authKeyMasked || t('site.authKeyPlaceholder')" />
                </a-form-item>
              </a-col>
              <a-col v-if="showAuthHash" :span="6">
                <a-form-item :label="t('site.authHash')">
                  <a-input v-model:value="credForm.authHash" :placeholder="t('site.authHashPlaceholder')" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="24">
              <a-col v-if="showUserId" :span="6">
                <a-form-item :label="t('site.userId')">
                  <a-input-number v-model:value="credForm.userId" :placeholder="t('site.userIdPlaceholder')" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col v-if="showRssKey" :span="6">
                <a-form-item :label="rssKeyLabel">
                  <a-input v-model:value="credForm.rssKey" :placeholder="site.rssKeyMasked || rssKeyHint" />
                </a-form-item>
              </a-col>
            </a-row>
          </template>

          <a-form-item>
            <a-button type="primary" @click="updateCredentials">{{ t('site.saveCredentials') }}</a-button>
          </a-form-item>
        </a-form>
      </a-card>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { sitesApi } from '@/api/sites'
import { ensureSupportedSitesCache, getSiteFieldOverride } from '@/api/supported-sites'
import type { Site, SiteCredentials } from '@/api/types'

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
const site = ref<Partial<Site>>({})
const credForm = reactive({ cookie: '', passkey: '', apiKey: '', bearerToken: '', authKey: '', authHash: '', userId: undefined as number | undefined, rssKey: '' })

const settingsForm = reactive({
  name: '',
  baseUrl: '',
  alternativeDomains: '',
  enabled: true,
  isSource: false,
  isTarget: false,
  participateAutoPublish: true,
  cookieCloudSync: false,
  proxyUrl: '',
  skipSslVerify: false,
  maxConcurrent: 2,
  hrStrategy: '',
})

const authTypeLabels: Record<string, string> = {
  cookie: 'Cookie',
  apikey: 'API Key',
  passkey: 'Passkey',
}

const fw = computed(() => site.value.framework as string)
// supported_sites 缓存加载状态（触发 override 重新计算）
const supportedSitesLoaded = ref(false)
const override = computed(() => {
  if (!supportedSitesLoaded.value) return undefined
  return getSiteFieldOverride(site.value.domain as string)
})

const needsCookie = computed(() => {
  if (override.value?.showCookie !== undefined) return override.value.showCookie
  return site.value.authType === 'cookie' || !site.value.authType
})
const needsApiKey = computed(() => {
  if (override.value?.showApiKey !== undefined) return override.value.showApiKey
  return site.value.authType === 'apikey'
})
const needsPasskey = computed(() => {
  if (override.value?.showPasskey !== undefined) return override.value.showPasskey
  return site.value.authType === 'passkey'
})
const showPasskeyField = computed(() => {
  if (override.value?.showPasskey !== undefined) return override.value.showPasskey
  const f = site.value.framework as string
  return needsPasskey.value || (f === 'nexusphp' || f === 'generic' || f === 'unit3d')
})
const authLabel = computed(() => authTypeLabels[site.value.authType as string] || 'Cookie')

const apiKeyLabel = computed(() => override.value?.apiKeyLabel || t('sites.apiKey'))
const apiKeyHint = computed(() => site.value.apiKeyMasked || (override.value?.apiKeyHint || t('site.inputApiKey')))
const rssKeyLabel = computed(() => override.value?.rssKeyLabel || t('site.rssKey'))
const rssKeyHint = computed(() => override.value?.rssKeyHint || t('site.rssKeyPlaceholder'))

const showBearerToken = computed(() => {
  if (override.value?.showBearerToken !== undefined) return override.value.showBearerToken
  return false
})
const showAuthKey = computed(() => {
  if (override.value?.showAuthKey !== undefined) return override.value.showAuthKey
  return fw.value === 'gazelle'
})
const showAuthHash = computed(() => {
  if (override.value?.showAuthHash !== undefined) return override.value.showAuthHash
  return false
})
const showUserId = computed(() => {
  if (override.value?.showUserId !== undefined) return override.value.showUserId
  return false
})
const showRssKey = computed(() => {
  if (override.value?.showRssKey !== undefined) return override.value.showRssKey
  return fw.value === 'gazelle' || fw.value === 'tnode' || fw.value === 'unit3d'
})
const showAdvancedCredentials = computed(() => showBearerToken.value || showAuthKey.value || showAuthHash.value || showUserId.value || showRssKey.value)

const detecting = ref(false)
const showDetectResult = ref(false)
const detectResult = ref<DetectResultData>({})
const statsLoading = ref(false)
const statsSyncing = ref(false)
const stats = ref<SiteStatsData>({})

import { formatBytes, formatTime } from '@/utils/format'
const frameworkColors: Record<string, string> = {
  nexusphp: 'blue', unit3d: 'green', gazelle: 'purple',
  mteam: 'orange', rousi: 'pink', tnode: 'cyan', luminance: 'magenta', generic: 'default',
}

const frameworkLabels: Record<string, string> = {
  nexusphp: 'NexusPHP', unit3d: 'UNIT3D', gazelle: 'Gazelle',
  mteam: 'M-Team', rousi: 'Rousi', tnode: 'TNode', luminance: 'Luminance', generic: 'Generic',
}

const siteFrozen = ref(false)
const freezeLoading = ref(false)
const showFreezeModal = ref(false)
const freezeForm = reactive({ duration: '1h', reason: '' })

async function checkFreezeStatus() {
  try {
    const resp = await sitesApi.getFreezeStatus(siteId)
    siteFrozen.value = resp.data.data?.frozen ?? false
  } catch {
    siteFrozen.value = false
  }
}

async function handleFreeze() {
  freezeLoading.value = true
  try {
    await sitesApi.freezeSite(siteId, freezeForm)
    message.success(t('site.freezeSuccess'))
    siteFrozen.value = true
    showFreezeModal.value = false
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    freezeLoading.value = false
  }
}

async function handleUnfreeze() {
  freezeLoading.value = true
  try {
    await sitesApi.unfreezeSite(siteId)
    message.success(t('site.unfreezeSuccess'))
    siteFrozen.value = false
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    freezeLoading.value = false
  }
}

async function fetchSite() {
  loading.value = true
  try {
    const resp = await sitesApi.get(siteId)
    site.value = resp.data.data || {}
    Object.assign(settingsForm, {
      name: site.value.name || '',
      baseUrl: site.value.baseUrl || '',
      alternativeDomains: altDomainsToText(site.value.alternativeDomains),
      enabled: site.value.enabled !== undefined ? site.value.enabled : true,
      isSource: site.value.isSource || false,
      isTarget: site.value.isTarget || false,
      participateAutoPublish: site.value.participateAutoPublish !== undefined ? site.value.participateAutoPublish : true,
      cookieCloudSync: site.value.cookieCloudSync || false,
      proxyUrl: site.value.proxyUrl || '',
      skipSslVerify: site.value.skipSslVerify || false,
      maxConcurrent: site.value.maxConcurrent || 2,
      hrStrategy: site.value.hrStrategy || '',
    })
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

async function updateCredentials() {
  try {
    const payload: SiteCredentials = {}
    if (credForm.passkey) payload.passkey = credForm.passkey
    if (credForm.apiKey) payload.apiKey = credForm.apiKey
    if (credForm.bearerToken) payload.bearerToken = credForm.bearerToken
    if (credForm.authKey) payload.authKey = credForm.authKey
    if (credForm.authHash) payload.authHash = credForm.authHash
    if (credForm.userId) payload.userId = credForm.userId
    if (credForm.rssKey) payload.rssKey = credForm.rssKey
    if (credForm.cookie) payload.cookie = credForm.cookie
    await sitesApi.updateCredentials(siteId, payload)
    message.success(t('site.credentialsUpdated'))
    credForm.cookie = ''
    credForm.passkey = ''
    credForm.apiKey = ''
    credForm.bearerToken = ''
    credForm.authKey = ''
    credForm.authHash = ''
    credForm.userId = undefined
    credForm.rssKey = ''
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
      cookieCloudSync: settingsForm.cookieCloudSync,
      proxyUrl: settingsForm.proxyUrl,
      skipSslVerify: settingsForm.skipSslVerify,
      maxConcurrent: settingsForm.maxConcurrent,
      hrStrategy: settingsForm.hrStrategy,
      name: settingsForm.name,
      baseUrl: settingsForm.baseUrl,
      alternativeDomains: altDomainsToJson(settingsForm.alternativeDomains),
    })
    message.success(t('common.configSaved'))
    fetchSite()
  } catch (e: unknown) {
    message.error((e as Error).message)
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

async function syncStats() {
  statsSyncing.value = true
  try {
    await sitesApi.syncSiteStats(siteId)
    message.success(t('site.statsSynced'))
    fetchStats()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    statsSyncing.value = false
  }
}

function altDomainsToText(val: string | undefined): string {
  if (!val) return ''
  try {
    const arr: string[] = JSON.parse(val)
    if (Array.isArray(arr)) return arr.join(', ')
  } catch { /* not JSON, return as-is */ }
  return val
}

function altDomainsToJson(val: string): string {
  const trimmed = val.trim()
  if (!trimmed) return ''
  const items = trimmed.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean)
  if (items.length === 0) return ''
  return JSON.stringify(items)
}

onMounted(() => {
  fetchSite()
  fetchStats()
  checkFreezeStatus()
  // 预加载白名单缓存，加载完成后触发 override 重算
  ensureSupportedSitesCache()
    .then(() => { supportedSitesLoaded.value = true })
    .catch(() => {/* 缓存加载失败不阻塞页面，使用框架默认显示 */})
})
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 500;
  color: #595959;
  padding-bottom: 10px;
  margin: 32px 0 18px;
  border-bottom: 1px solid #d9d9d9;
}
.section-title:first-child {
  margin-top: 0;
}
.section-title::before {
  content: '';
  display: inline-block;
  width: 3px;
  height: 14px;
  background: #1890ff;
  margin-right: 8px;
  vertical-align: middle;
  border-radius: 2px;
}
</style>
