<template>
  <div>
    <a-page-header :title="site.name || site.domain" @back="$router.push('/sites')">
      <template #tags>
        <a-tag v-if="needsCookie && site.hasCookie" color="green">Cookie 有效</a-tag>
        <a-tag v-else-if="needsCookie" color="red">Cookie 未配置</a-tag>
        <a-tag v-if="needsApiKey && site.hasApiKey" color="green">API Key 有效</a-tag>
        <a-tag v-else-if="needsApiKey" color="red">API Key 未配置</a-tag>
        <a-tag v-if="needsPasskey && site.hasPasskey" color="green">Passkey 有效</a-tag>
        <a-tag v-else-if="needsPasskey" color="red">Passkey 未配置</a-tag>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item label="域名">{{ site.domain }}</a-descriptions-item>
        <a-descriptions-item label="名称">{{ site.name }}</a-descriptions-item>
        <a-descriptions-item label="URL">{{ site.baseUrl }}</a-descriptions-item>
        <a-descriptions-item label="框架">
          <a-tag :color="frameworkColors[site.framework] || 'default'">
            {{ frameworkLabels[site.framework] || site.framework }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item label="认证方式">{{ authLabel }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie" label="Cookie">{{ site.hasCookie ? '已配置' : '未配置' }}</a-descriptions-item>
        <a-descriptions-item v-if="needsApiKey" label="API Key">{{ site.hasApiKey ? '已配置' : '未配置' }}</a-descriptions-item>
        <a-descriptions-item v-if="needsPasskey" label="Passkey">{{ site.hasPasskey ? '已配置' : '未配置' }}</a-descriptions-item>
        <a-descriptions-item label="最后同步">{{ site.lastSyncAt || '-' }}</a-descriptions-item>
        <a-descriptions-item label="创建时间">{{ site.createdAt || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-card title="凭证管理" style="margin-bottom: 24px">
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
            <a-button type="primary" @click="updateCredentials">保存凭证</a-button>
          </a-form-item>
        </a-form>
      </a-card>

      <a-card title="站点配置覆盖" style="margin-bottom: 24px">
        <template #extra>
          <a-space>
            <a-button size="small" @click="showOverrideEditor = true">编辑覆盖配置</a-button>
            <a-popconfirm v-if="hasOverrides" title="确认删除所有覆盖配置？" @confirm="deleteOverrides">
              <a-button size="small" danger>删除覆盖</a-button>
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
        <a-empty v-else description="暂无覆盖配置（使用全局默认值）" />
      </a-card>

      <a-modal
        v-model:open="showOverrideEditor"
        title="编辑站点配置覆盖"
        :confirm-loading="overrideSaving"
        width="640px"
        @ok="saveOverrides"
      >
        <a-alert message="覆盖配置为 JSON 格式，仅修改需要覆盖的字段" type="info" show-icon style="margin-bottom: 16px" />
        <a-textarea v-model:value="overrideJSON" :rows="12" placeholder='{"upload_rule": "...", "download_prefix": "..."}' />
      </a-modal>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { sitesApi } from '@/api/sites'

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
    message.success('凭证已更新')
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
    message.success('覆盖配置已保存')
    showOverrideEditor.value = false
    fetchOverrides()
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      message.error('JSON 格式错误')
    } else {
      message.error(e?.response?.data?.message || '保存失败')
    }
  } finally {
    overrideSaving.value = false
  }
}

async function deleteOverrides() {
  try {
    await sitesApi.deleteOverrides(siteId)
    message.success('覆盖配置已删除')
    overrideData.value = {}
    overrideJSON.value = '{}'
  } catch (e: any) {
    message.error(e?.response?.data?.message || '删除失败')
  }
}

onMounted(() => {
  fetchSite()
  fetchOverrides()
})
</script>
