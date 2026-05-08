<template>
  <div>
    <a-page-header :title="subscription.name || t('subscription.subscriptionDetail')" @back="$router.push('/subscriptions')" />

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item label="名称">{{ subscription.name }}</a-descriptions-item>
        <a-descriptions-item label="站点">{{ subscription.siteName }}</a-descriptions-item>
        <a-descriptions-item label="RSS 地址" :span="2">{{ (subscription.urls || []).join(', ') }}</a-descriptions-item>
        <a-descriptions-item label="定时表达式">{{ subscription.cron }}</a-descriptions-item>
        <a-descriptions-item label="状态">
          <a-badge :status="subscription.enabled ? 'success' : 'default'" :text="subscription.enabled ? '已启用' : '已禁用'" />
        </a-descriptions-item>
        <a-descriptions-item label="创建时间">{{ subscription.createdAt || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-tabs v-model:activeKey="activeTab">
        <a-tab-pane key="config" tab="配置">
          <a-form :model="configForm" layout="vertical" style="max-width: 640px">
            <a-form-item label="名称" name="name" :rules="[{ required: true, message: '请输入名称' }]">
              <a-input v-model:value="configForm.name" />
            </a-form-item>
            <a-form-item label="RSS 地址">
              <a-textarea v-model:value="configForm.urls" :rows="3" placeholder="每行一个 RSS URL" />
            </a-form-item>
            <a-form-item label="定时表达式">
              <a-input v-model:value="configForm.cron" placeholder="如 */5 * * * *" />
            </a-form-item>

            <a-divider>下载器设置</a-divider>
            <a-form-item label="下载器" name="clientId">
              <a-select v-model:value="configForm.clientId" placeholder="选择下载器" :loading="downloadersLoading" allow-clear>
                <a-select-option v-for="d in downloaders" :key="d.name" :value="d.name">
                  {{ d.name }}（{{ d.type }}）
                </a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="保存路径（留空使用下载器默认值）">
              <a-input v-model:value="configForm.savePath" placeholder="/downloads/..." />
            </a-form-item>
            <a-form-item label="分类">
              <a-input v-model:value="configForm.category" placeholder="下载器中的分类" />
            </a-form-item>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="暂停添加">
                  <a-switch v-model:checked="configForm.addPaused" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="自动种子管理">
                  <a-switch v-model:checked="configForm.autoTmm" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-form-item label="标签">
              <a-select v-model:value="configForm.tags" mode="tags" placeholder="输入标签后回车" style="width: 100%" />
            </a-form-item>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="上传限速 (KB/s)">
                  <a-input-number v-model:value="configForm.uploadLimitKb" :min="0" style="width: 100%" placeholder="0 = 不限" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="下载限速 (KB/s)">
                  <a-input-number v-model:value="configForm.downloadLimitKb" :min="0" style="width: 100%" placeholder="0 = 不限" />
                </a-form-item>
              </a-col>
            </a-row>

            <a-divider>抓取选项</a-divider>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="只接受免费种子">
                  <a-switch v-model:checked="configForm.scrapeFree" />
                  <div style="color:#999;font-size:12px;margin-top:4px">开启后仅推送免费/折扣种子到下载器</div>
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="检测 HR 信息">
                  <a-switch v-model:checked="configForm.scrapeHr" />
                  <div style="color:#999;font-size:12px;margin-top:4px">检测站点 HR（保种时间要求）</div>
                </a-form-item>
              </a-col>
            </a-row>

            <a-divider>自动化</a-divider>
            <a-row :gutter="16">
              <a-col :span="8">
                <a-form-item label="启用自动发布">
                  <a-switch v-model:checked="configForm.publishEnabled" />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item label="推送通知">
                  <a-switch v-model:checked="configForm.pushNotify" />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item label="自动辅种">
                  <a-switch v-model:checked="configForm.autoReseed" />
                </a-form-item>
              </a-col>
            </a-row>

            <a-form-item>
              <a-button type="primary" @click="saveConfig" :loading="configSaving">{{ t('common.saveConfig') }}</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>
        <a-tab-pane key="dryrun" tab="试运行">
          <a-button type="primary" @click="runDryrun" :loading="dryrunLoading" style="margin-bottom: 16px">
            执行试运行
          </a-button>
          <a-table
            :columns="dryrunColumns"
            :data-source="dryrunResults"
            :pagination="false"
            row-key="title"
            size="small"
          />
        </a-tab-pane>
        <a-tab-pane key="history" tab="抓取历史">
          <a-table
            :columns="historyColumns"
            :data-source="history"
            :loading="historyLoading"
            :pagination="{ pageSize: 20 }"
            row-key="id"
            size="small"
          />
        </a-tab-pane>
        <a-tab-pane key="rules" tab="过滤规则">
          <p style="color: #666; margin-bottom: 16px">
            此处配置订阅级别的自定义条件（如按标题、体积筛选）。「全局排除规则」在「设置 → 全局排除规则」中管理，对所有订阅生效。
          </p>
          <div v-for="(cond, idx) in ruleConditions" :key="idx" style="margin-bottom: 12px">
            <a-row :gutter="8" align="middle">
              <a-col :span="6">
                <a-select v-model:value="cond.key" placeholder="选择字段" style="width:100%">
                  <a-select-option value="title">标题</a-select-option>
                  <a-select-option value="size">体积大小</a-select-option>
                  <a-select-option value="uploader">发布者</a-select-option>
                  <a-select-option value="site">站点</a-select-option>
                  <a-select-option value="free">是否免费</a-select-option>
                </a-select>
              </a-col>
              <a-col :span="6">
                <a-select v-model:value="cond.compareType" placeholder="比较方式" style="width:100%">
                  <a-select-option value="equals">等于</a-select-option>
                  <a-select-option value="bigger">大于</a-select-option>
                  <a-select-option value="smaller">小于</a-select-option>
                  <a-select-option value="contain">包含</a-select-option>
                  <a-select-option value="not_contain">不包含</a-select-option>
                  <a-select-option value="regexp">正则匹配</a-select-option>
                </a-select>
              </a-col>
              <a-col :span="10">
                <a-input-number
                  v-if="cond.key === 'size'"
                  v-model:value="cond.numValue"
                  style="width:100%"
                  :min="0"
                  placeholder="单位：字节"
                />
                <a-select v-else-if="cond.key === 'free'" v-model:value="cond.value" style="width:100%">
                  <a-select-option value="true">是（免费）</a-select-option>
                  <a-select-option value="false">否（非免费）</a-select-option>
                </a-select>
                <a-input v-else v-model:value="cond.value" placeholder="输入值" style="width:100%" />
              </a-col>
              <a-col :span="2">
                <a-button type="text" danger @click="ruleConditions.splice(idx, 1)" :disabled="ruleConditions.length <= 1">
                  删
                </a-button>
              </a-col>
            </a-row>
            <div v-if="cond.key === 'size'" style="margin-top:4px;color:#999;font-size:12px">
              1 GB = 1073741824，100 MB = 104857600
            </div>
          </div>
          <a-button type="dashed" block @click="ruleConditions.push({key:'title',compareType:'contain',value:'',numValue:0})" style="margin-bottom:16px">
            添加条件
          </a-button>
          <a-form layout="vertical" style="max-width: 600px">
            <a-form-item>
              <a-button type="primary" @click="saveRules" :loading="rulesSaving">保存规则</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>
      </a-tabs>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { subscriptionsApi } from '@/api/subscriptions'
import { downloadersApi } from '@/api/downloaders'

interface RuleCond {
  key: string
  compareType: string
  value: string
  numValue: number
}

const { t } = useI18n()

const route = useRoute()
const id = Number(route.params.id)

const loading = ref(false)
const configSaving = ref(false)
const dryrunLoading = ref(false)
const historyLoading = ref(false)
const subscription = ref<any>({})
const dryrunResults = ref<any[]>([])
const history = ref<any[]>([])
const activeTab = ref('config')
const downloaders = ref<any[]>([])
const downloadersLoading = ref(false)

const configForm = reactive({
  name: '',
  urls: '',
  cron: '',
  clientId: '',
  savePath: '',
  category: '',
  addPaused: false,
  autoTmm: false,
  tags: [] as string[],
  uploadLimitKb: 0,
  downloadLimitKb: 0,
  scrapeFree: false,
  scrapeHr: false,
  publishEnabled: false,
  pushNotify: false,
  autoReseed: false,
})
const ruleConditions = ref<RuleCond[]>([{ key: 'title', compareType: 'contain', value: '', numValue: 0 }])
const rulesSaving = ref(false)

const dryrunColumns = [
  { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '大小', dataIndex: 'size', key: 'size', width: 100 },
  { title: '匹配', dataIndex: 'matched', key: 'matched', width: 80 },
  { title: '原因', dataIndex: 'reason', key: 'reason' },
]

const historyColumns = [
  { title: '时间', dataIndex: 'fetchedAt', key: 'fetchedAt', width: 180 },
  { title: '新种子数', dataIndex: 'newCount', key: 'newCount', width: 100 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
]

async function fetchDownloaders() {
  downloadersLoading.value = true
  try {
    const resp = await downloadersApi.list(1, 200)
    downloaders.value = resp.data?.data?.items || resp.data?.data || []
  } catch {
    downloaders.value = []
  } finally {
    downloadersLoading.value = false
  }
}

async function fetchSubscription() {
  loading.value = true
  try {
    const resp = await subscriptionsApi.get(id)
    subscription.value = resp.data.data || {}
    Object.assign(configForm, {
      name: subscription.value.name || '',
      urls: Array.isArray(subscription.value.urls) ? subscription.value.urls.join('\n') : (subscription.value.urls || ''),
      cron: subscription.value.cron || '',
      clientId: subscription.value.clientId || '',
      savePath: subscription.value.savePath || '',
      category: subscription.value.category || '',
      addPaused: subscription.value.addPaused || false,
      autoTmm: subscription.value.autoTmm || false,
      tags: subscription.value.tags || [],
      uploadLimitKb: subscription.value.uploadLimitKb || 0,
      downloadLimitKb: subscription.value.downloadLimitKb || 0,
      scrapeFree: subscription.value.scrapeFree || false,
      scrapeHr: subscription.value.scrapeHr || false,
      publishEnabled: subscription.value.publishEnabled || false,
      pushNotify: subscription.value.pushNotify || false,
      autoReseed: subscription.value.autoReseed || false,
    })
    const conds = subscription.value.conditions || []
    if (Array.isArray(conds) && conds.length) {
      ruleConditions.value = conds.map((c: any) => ({
        key: c.key || 'title',
        compareType: c.compare_type || c.compareType || 'contain',
        value: c.value || '',
        numValue: c.key === 'size' ? Number(c.value || 0) : 0,
      }))
    }
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  configSaving.value = true
  try {
    const payload: any = {
      name: configForm.name,
      urls: typeof configForm.urls === 'string' ? configForm.urls.split('\n').map((u: string) => u.trim()).filter(Boolean) : configForm.urls,
      cron: configForm.cron,
      clientId: configForm.clientId,
      savePath: configForm.savePath,
      category: configForm.category,
      addPaused: configForm.addPaused,
      autoTmm: configForm.autoTmm,
      tags: configForm.tags,
      uploadLimitKb: configForm.uploadLimitKb,
      downloadLimitKb: configForm.downloadLimitKb,
      scrapeFree: configForm.scrapeFree,
      scrapeHr: configForm.scrapeHr,
      publishEnabled: configForm.publishEnabled,
      pushNotify: configForm.pushNotify,
      autoReseed: configForm.autoReseed,
    }
    await subscriptionsApi.update(id, payload)
    message.success(t('common.configSaved'))
    fetchSubscription()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    configSaving.value = false
  }
}

async function runDryrun() {
  dryrunLoading.value = true
  try {
    const resp = await subscriptionsApi.dryrun(id)
    const data = resp.data.data || {}
    dryrunResults.value = (data.recentTorrents || []).map((torrent: any) => ({
      title: torrent.title || torrent.name || '-',
      size: torrent.size ? (torrent.size / 1073741824).toFixed(2) + ' GB' : '-',
      matched: torrent.matched ? t('common.yes') : t('common.no'),
      reason: torrent.reason || '-',
    }))
    message.success(t('subscription.dryrunComplete', { count: data.total || 0 }))
  } catch (e: any) {
    message.error(e.message)
  } finally {
    dryrunLoading.value = false
  }
}

async function fetchHistory() {
  historyLoading.value = true
  try {
    const resp = await subscriptionsApi.get(id)
    const sub = resp.data.data || {}
    history.value = (sub.recentFetches || []).map((f: any, idx: number) => ({
      id: idx + 1,
      fetchedAt: f.fetchedAt || f.createdAt || '-',
      newCount: f.newCount ?? 0,
      status: f.status || 'ok',
    }))
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

async function saveRules() {
  rulesSaving.value = true
  try {
    const conditions = ruleConditions.value.map(c => ({
      key: c.key,
      compare_type: c.compareType,
      value: c.key === 'size' ? String(c.numValue || 0) : c.value,
    }))
    await subscriptionsApi.updateRules(id, { conditions })
    message.success('规则已保存')
  } catch (e: any) {
    message.error(e.message)
  } finally {
    rulesSaving.value = false
  }
}

onMounted(() => {
  fetchSubscription()
  fetchHistory()
  fetchDownloaders()
})
</script>
