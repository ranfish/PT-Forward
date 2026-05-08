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
        <a-descriptions-item :label="t('site.detectConfidence')">{{ (detectResult.confidence * 100).toFixed(0) }}%</a-descriptions-item>
        <a-descriptions-item v-if="detectResult.detectionDetail" :label="t('site.detectDetail')">{{ detectResult.detectionDetail }}</a-descriptions-item>
      </a-descriptions>
    </a-modal>

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
        <a-descriptions-item label="启用"><a-badge :status="site.enabled ? 'success' : 'default'" :text="site.enabled ? '是' : '否'" /></a-descriptions-item>
        <a-descriptions-item label="角色">{{ [site.isSource ? '源站' : '', site.isTarget ? '目标站' : ''].filter(Boolean).join(', ') || '-' }}</a-descriptions-item>
        <a-descriptions-item label="参与自动发布">{{ site.participateAutoPublish ? '是' : '否' }}</a-descriptions-item>
        <a-descriptions-item label="CookieCloud 同步">{{ site.cookieCloudSync ? '是' : '否' }}</a-descriptions-item>
        <a-descriptions-item v-if="site.cookieCloudSync" label="CookieCloud 域名">{{ site.cookieCloudDomain || '-' }}</a-descriptions-item>
        <a-descriptions-item label="Hash 策略">{{ site.hashStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item label="Size 策略">{{ site.sizeStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item label="ID 策略">{{ site.idStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item label="覆盖 RSS 地址">{{ site.overrideRssUrl || '-' }}</a-descriptions-item>
        <a-descriptions-item label="覆盖保存路径">{{ site.overrideSavePath || '-' }}</a-descriptions-item>
        <a-descriptions-item label="代理地址">{{ site.proxyUrl || '-' }}</a-descriptions-item>
        <a-descriptions-item label="跳过 SSL 验证">{{ site.skipSslVerify ? '是' : '否' }}</a-descriptions-item>
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

      <a-card title="站点设置" style="margin-bottom: 24px">
        <a-form :model="settingsForm" layout="vertical" style="max-width: 500px">
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item label="启用">
                <a-switch v-model:checked="settingsForm.enabled" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item label="参与自动发布">
                <a-switch v-model:checked="settingsForm.participateAutoPublish" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item label="作为源站">
                <a-switch v-model:checked="settingsForm.isSource" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item label="作为目标站">
                <a-switch v-model:checked="settingsForm.isTarget" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item label="CookieCloud 同步">
                <a-switch v-model:checked="settingsForm.cookieCloudSync" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item label="CookieCloud 域名">
                <a-input v-model:value="settingsForm.cookieCloudDomain" placeholder="CookieCloud 域名" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item label="覆盖 RSS 地址">
            <a-input v-model:value="settingsForm.overrideRssUrl" placeholder="自定义 RSS URL" />
          </a-form-item>
          <a-form-item label="覆盖保存路径">
            <a-input v-model:value="settingsForm.overrideSavePath" placeholder="自定义保存路径" />
          </a-form-item>
          <a-form-item label="代理地址">
            <a-input v-model:value="settingsForm.proxyUrl" placeholder="例如: socks5://127.0.0.1:1080" />
          </a-form-item>
          <a-form-item label="跳过 SSL 验证">
            <a-switch v-model:checked="settingsForm.skipSslVerify" />
          </a-form-item>
          <a-row :gutter="16">
            <a-col :span="8">
              <a-form-item label="Hash 策略">
                <a-select v-model:value="settingsForm.hashStrategy" allow-clear>
                  <a-select-option value="guid">GUID</a-select-option>
                  <a-select-option value="xml_tag">XML 标签</a-select-option>
                  <a-select-option value="fake_from_id">根据 ID 生成</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item label="Size 策略">
                <a-select v-model:value="settingsForm.sizeStrategy" allow-clear>
                  <a-select-option value="enclosure">Enclosure</a-select-option>
                  <a-select-option value="xml_tag">XML 标签</a-select-option>
                  <a-select-option value="desc_regex">描述正则</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item label="ID 策略">
                <a-select v-model:value="settingsForm.idStrategy" allow-clear>
                  <a-select-option value="query_param">查询参数</a-select-option>
                  <a-select-option value="link_regex">链接正则</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item>
            <a-button type="primary" @click="updateSettings">保存设置</a-button>
          </a-form-item>
        </a-form>
      </a-card>

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

const settingsForm = reactive({
  enabled: true,
  isSource: false,
  isTarget: false,
  participateAutoPublish: true,
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
const authLabel = computed(() => authTypeLabels[site.value.authType] || 'Cookie')

const overrideLoading = ref(false)
const overrideSaving = ref(false)
const overrideData = ref<Record<string, any>>({})
const overrideJSON = ref('{}')
const showOverrideEditor = ref(false)

const hasOverrides = computed(() => Object.keys(overrideData.value).length > 0)

const detecting = ref(false)
const showDetectResult = ref(false)
const detectResult = ref<any>({})
const statsLoading = ref(false)
const stats = ref<any>({})

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

async function updateSettings() {
  try {
    await sitesApi.update(siteId, {
      enabled: settingsForm.enabled,
      isSource: settingsForm.isSource,
      isTarget: settingsForm.isTarget,
      participateAutoPublish: settingsForm.participateAutoPublish,
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

async function runDetect() {
  detecting.value = true
  try {
    const resp = await sitesApi.detect(siteId)
    detectResult.value = resp.data.data || {}
    showDetectResult.value = true
  } catch (e: any) {
    message.error(e.message)
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
