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
        <!-- v0.0.256 图床代理开关 -->
        <a-form-item label="图床代理">
          <a-switch v-model:checked="useProxy" @change="onProxyChange" />
          <span style="margin-left: 8px; color: #666; font-size: 12px">
            {{ useProxy ? '使用系统 HTTP 代理上传截图' : '直连（不使用代理）' }}
          </span>
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
      <a-form layout="vertical" style="max-width: 720px">
        <div class="ptgen-provider-list">
          <div v-for="(p, idx) in ptgenProviders" :key="idx" class="ptgen-provider-row">
            <a-input
              v-model:value="p.url"
              placeholder="端点 URL，如 https://doubaninfo.com/api/v1_douban.php 或 https://cspt.top"
              style="flex: 1"
            />
            <a-input-password
              v-model:value="p.key"
              placeholder="API Key / Token"
              style="width: 220px"
            />
            <a-button danger size="small" @click="ptgenProviders.splice(idx, 1)">删除</a-button>
          </div>
          <div v-if="ptgenProviders.length === 0" class="ptgen-empty">
            暂未配置任何 PTGen 端点
          </div>
        </div>
        <a-button type="dashed" size="small" @click="ptgenProviders.push({ url: '', key: '' })" style="margin-top: 8px">
          + 添加端点
        </a-button>
        <div style="margin-top: 16px">
          <a-button type="primary" :loading="ptgenSaving" @click="savePtgen">保存</a-button>
        </div>
        <div class="ptgen-hint">
          豆影：端点填 https://doubaninfo.com/api/v1_douban.php ，Key 填豆影 API Key<br>
          财神：端点填 https://cspt.top ，Key 填财神 API Token（在财神站点 ptgen.php 页面生成）<br>
          排在前面的端点优先使用，查询失败自动切换下一个。
        </div>
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
const useProxy = ref(false)  // v0.0.256 图床代理开关
interface PtgenProvider { url: string; key: string }
const ptgenProviders = ref<PtgenProvider[]>([])
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
    // v0.0.256 读图床代理开关
    try {
      const { data: sd } = await settingsApi.get('image_host_use_proxy')
      useProxy.value = sd.data?.['image_host_use_proxy'] === 'true'
    } catch { /* defaults to false */ }
  } catch {
    message.error('加载图床配置失败')
  } finally {
    loading.value = false
  }
}

// v0.0.256 图床代理开关变更
async function onProxyChange(checked: boolean) {
  try {
    await settingsApi.update('image_host_use_proxy', { value: String(checked) })
    message.success(checked ? '图床代理已开启（重启后生效）' : '图床代理已关闭（重启后生效）')
  } catch {
    message.error('保存失败')
    useProxy.value = !checked
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
    const epItems = (epResp.data.data as any)?.items || {}
    const keyItems = (keyResp.data.data as any)?.items || {}
    const epStr = epItems.ptgen_endpoints || ''
    const globalKey = keyItems.ptgen_api_key || ''

    const providers: PtgenProvider[] = []
    for (const line of epStr.split('#')) {
      const t = line.trim()
      if (!t) continue
      if (t.includes('|')) {
        const i = t.indexOf('|')
        providers.push({ url: t.slice(0, i).trim(), key: t.slice(i + 1).trim() })
      } else {
        providers.push({ url: t, key: globalKey })
      }
    }
    ptgenProviders.value = providers
  } catch {
    // ignore
  }
}

async function savePtgen() {
  ptgenSaving.value = true
  try {
    const epStr = ptgenProviders.value
      .filter(p => p.url.trim())
      .map(p => `${p.url.trim()}|${p.key.trim()}`)
      .join('#')
    await settingsApi.update('ptgen_endpoints', { value: epStr })
    await settingsApi.update('ptgen_api_key', { value: '' })
    message.success('PTGen 配置保存成功')
  } catch {
    message.error('保存失败')
  } finally {
    ptgenSaving.value = false
  }
}

onMounted(loadPtgen)
</script>

<style scoped>
.ptgen-provider-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.ptgen-provider-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.ptgen-empty {
  color: #999;
  font-size: 13px;
  padding: 8px 0;
}
.ptgen-hint {
  margin-top: 12px;
  color: #888;
  font-size: 12px;
  line-height: 1.8;
}
</style>
