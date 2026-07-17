<template>
  <div class="image-host-settings">
    <a-card title="图床配置" style="margin-bottom: 24px">
      <template #extra>
        <a-button type="primary" size="small" :loading="testing" @click="handleTest">测试上传</a-button>
      </template>
      <a-form layout="vertical" style="max-width: 600px" v-loading="loading">
        <a-form-item label="默认图床">
          <a-select v-model:value="config.default" style="width: 200px">
            <a-select-option v-for="h in config.hosts" :key="h" :value="h">{{ h }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="截图策略">
          <a-select v-model:value="config.strategy" style="width: 320px">
            <a-select-option value="auto">自动（源站优先+本地兜底）</a-select-option>
            <a-select-option value="local_upload">始终本地截图</a-select-option>
            <a-select-option value="source_rehost">源站截图转存</a-select-option>
            <a-select-option value="source_direct">源站截图直用</a-select-option>
          </a-select>
        </a-form-item>
        <a-divider>AGSVPT 配置</a-divider>
        <a-form-item label="AGSVPT 邮箱">
          <a-input v-model:value="config.agsvpt_email" placeholder="AGSVPT 注册邮箱" style="width: 300px" />
        </a-form-item>
        <a-form-item label="AGSVPT 密码">
          <a-input-password v-model:value="agsvptPassword" placeholder="留空不修改" style="width: 300px" />
        </a-form-item>
        <a-form-item label="AGSVPT 状态">
          <a-tag :color="config.agsvpt_configured ? 'green' : 'default'">
            {{ config.agsvpt_configured ? '已配置' : '未配置' }}
          </a-tag>
        </a-form-item>
        <a-divider>图床健康状态</a-divider>
        <a-space wrap>
          <a-tag v-for="(status, name) in config.health" :key="name"
            :color="status === 'ok' ? 'green' : 'red'">
            {{ name }}: {{ status === 'ok' ? '正常' : status }}
          </a-tag>
        </a-space>
        <div style="margin-top: 16px">
          <a-button type="primary" :loading="saving" @click="handleSave">保存</a-button>
        </div>
      </a-form>
    </a-card>

    <PublishLimitSettings />

    <a-card title="PTGen 配置" style="margin-top: 24px">
      <a-form layout="vertical" style="max-width: 600px">
        <a-form-item label="PTGen 端点（# 分隔故障转移）">
          <a-textarea
            v-model:value="ptgenEndpoints"
            :rows="3"
            placeholder="https://doubaninfo.com/api/v1_douban.php"
          />
        </a-form-item>
        <a-form-item label="API Key（豆影等需要 Key 的服务）">
          <a-input-password v-model:value="ptgenApiKey" placeholder="可选，仅部分服务需要" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" :loading="ptgenSaving" @click="savePtgen">保存</a-button>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { imageHostApi, type ImageHostConfig } from '@/api/image-host'
import { settingsApi } from '@/api/settings'
import PublishLimitSettings from './PublishLimitSettings.vue'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const agsvptPassword = ref('')
const ptgenEndpoints = ref('')
const ptgenApiKey = ref('')
const ptgenSaving = ref(false)

const config = reactive<ImageHostConfig>({
  hosts: [],
  default: 'pixhost',
  strategy: 'auto',
  agsvpt_email: '',
  agsvpt_configured: false,
  health: {},
})

async function loadConfig() {
  loading.value = true
  try {
    const { data } = await imageHostApi.get()
    const d = data.data
    config.hosts = d.hosts || []
    config.default = d.default || 'pixhost'
    config.strategy = d.strategy || 'auto'
    config.agsvpt_email = d.agsvpt_email || ''
    config.agsvpt_configured = d.agsvpt_configured || false
    config.health = d.health || {}
  } catch {
    message.error('加载图床配置失败')
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const payload: Record<string, string> = {
      default: config.default,
      strategy: config.strategy,
    }
    if (config.agsvpt_email) payload.agsvpt_email = config.agsvpt_email
    if (agsvptPassword.value) payload.agsvpt_password = agsvptPassword.value
    await imageHostApi.update(payload)
    message.success('保存成功')
    agsvptPassword.value = ''
    await loadConfig()
  } catch {
    message.error('保存失败')
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    const { data } = await imageHostApi.test(config.default)
    if (data.data.success) {
      message.success(`测试成功: ${data.data.url}`)
    } else {
      message.error('测试失败')
    }
  } catch (e: any) {
    message.error('测试失败: ' + (e?.message || '未知错误'))
  } finally {
    testing.value = false
  }
}

onMounted(loadConfig)

async function loadPtgen() {
  try {
    const [epResp, keyResp] = await Promise.all([
      settingsApi.get('ptgen_endpoints'),
      settingsApi.get('ptgen_api_key'),
    ])
    const epRaw = epResp.data.data as any
    if (epRaw && typeof epRaw === 'object') {
      ptgenEndpoints.value = (epRaw as any).ptgen_endpoints || (epRaw as any).value || ''
    }
    const keyRaw = keyResp.data.data as any
    if (keyRaw && typeof keyRaw === 'object') {
      ptgenApiKey.value = (keyRaw as any).ptgen_api_key || (keyRaw as any).value || ''
    }
  } catch {
    // ignore
  }
}

async function savePtgen() {
  ptgenSaving.value = true
  try {
    await settingsApi.update('ptgen_endpoints', { value: ptgenEndpoints.value })
    await settingsApi.update('ptgen_api_key', { value: ptgenApiKey.value })
    message.success('PTGen 配置保存成功')
  } catch {
    message.error('保存失败')
  } finally {
    ptgenSaving.value = false
  }
}

onMounted(loadPtgen)
</script>
