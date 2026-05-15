<template>
  <div>
    <a-page-header :title="subscription.name || t('subscription.subscriptionDetail')" @back="$router.push('/subscriptions')" />

    <a-spin :spinning="loading">
      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('common.name')">{{ subscription.name }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.site')">{{ subscription.siteName }}</a-descriptions-item>
        <a-descriptions-item :label="t('subscription.rssAddress')" :span="2">{{ ((subscription.urls || []) as string[]).join(', ') }}</a-descriptions-item>
        <a-descriptions-item :label="t('subscription.cronExpression')">{{ subscription.cron }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.status')">
          <a-badge :status="subscription.enabled ? 'success' : 'default'" :text="subscription.enabled ? t('common.enabled') : t('common.disabled')" />
        </a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ subscription.createdAt || '-' }}</a-descriptions-item>
      </a-descriptions>

      <a-tabs v-model:active-key="activeTab">
        <a-tab-pane key="config" :tab="t('subscription.config')">
          <a-form :model="configForm" layout="vertical" style="max-width: 640px">
            <a-form-item :label="t('common.name')" name="name" :rules="[{ required: true, message: t('common.nameRequired') }]">
              <a-input v-model:value="configForm.name" />
            </a-form-item>
            <a-form-item :label="t('subscription.rssAddress')">
              <a-textarea v-model:value="configForm.urls" :rows="3" :placeholder="t('subscription.rssUrlPerLineShort')" />
            </a-form-item>
            <a-form-item :label="t('subscription.cronExpression')">
              <a-input v-model:value="configForm.cron" :placeholder="t('subscription.cronExample')" />
            </a-form-item>

            <a-divider>{{ t('subscription.downloaderSettings') }}</a-divider>
            <a-form-item :label="t('downloader.title')" name="clientId">
              <a-select v-model:value="configForm.clientId" :placeholder="t('subscription.selectDownloader')" :loading="downloadersLoading" allow-clear>
                <a-select-option v-for="d in downloaders" :key="d.name" :value="d.name">
                  {{ d.name }}（{{ d.type }}）
                </a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item :label="t('subscription.savePathHint')">
              <a-input v-model:value="configForm.savePath" placeholder="/downloads/..." />
            </a-form-item>
            <a-form-item :label="t('subscription.categoryLabel')">
              <a-input v-model:value="configForm.category" :placeholder="t('subscription.categoryPlaceholder')" />
            </a-form-item>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('subscription.addPaused')">
                  <a-switch v-model:checked="configForm.addPaused" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('subscription.autoTmm')">
                  <a-switch v-model:checked="configForm.autoTmm" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-form-item :label="t('subscription.tagsLabel')">
              <a-select v-model:value="configForm.tags" mode="tags" :placeholder="t('subscription.tagsPlaceholder')" style="width: 100%" />
            </a-form-item>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('subscription.uploadLimit')">
                  <a-input-number v-model:value="configForm.uploadLimitKb" :min="0" style="width: 100%" :placeholder="t('subscription.noLimit')" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('subscription.downloadLimit')">
                  <a-input-number v-model:value="configForm.downloadLimitKb" :min="0" style="width: 100%" :placeholder="t('subscription.noLimit')" />
                </a-form-item>
              </a-col>
            </a-row>

            <a-divider>{{ t('subscription.scrapeOptions') }}</a-divider>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('subscription.freeOnly')">
                  <a-switch v-model:checked="configForm.scrapeFree" />
                  <div style="color:#999;font-size:12px;margin-top:4px">{{ t('subscription.freeOnlyHint') }}</div>
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('subscription.scrapeHrLabel')">
                  <a-switch v-model:checked="configForm.scrapeHr" />
                  <div style="color:#999;font-size:12px;margin-top:4px">{{ t('subscription.scrapeHrHint') }}</div>
                </a-form-item>
              </a-col>
            </a-row>

            <a-divider>{{ t('subscription.automation') }}</a-divider>
            <a-row :gutter="16">
              <a-col :span="8">
                <a-form-item :label="t('subscription.enableAutoPublish')">
                  <a-switch v-model:checked="configForm.publishEnabled" />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item :label="t('subscription.pushNotifyLabel')">
                  <a-switch v-model:checked="configForm.pushNotify" />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item :label="t('subscription.autoReseedLabel')">
                  <a-switch v-model:checked="configForm.autoReseed" />
                </a-form-item>
              </a-col>
            </a-row>

            <a-collapse :bordered="false" style="margin-bottom: 24px">
              <a-collapse-panel key="advanced" header="高级选项">
                <a-divider>速率与过滤</a-divider>
                <a-row :gutter="16">
                  <a-col :span="12">
                    <a-form-item label="跳过同大小">
                      <a-switch v-model:checked="configForm.skipSameSize" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="12">
                    <a-form-item label="严格同大小">
                      <a-switch v-model:checked="configForm.skipSameSizeStrict" />
                    </a-form-item>
                  </a-col>
                </a-row>
                <a-row v-if="configForm.skipSameSize" :gutter="16">
                  <a-col :span="12">
                    <a-form-item label="同大小窗口(分钟)">
                      <a-input-number v-model:value="configForm.skipSameSizeWindowMin" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="12">
                    <a-form-item label="每小时最大添加数">
                      <a-input-number v-model:value="configForm.addCountPerHour" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                </a-row>

                <a-divider>自定义正则</a-divider>
                <a-form-item label="使用自定义正则">
                  <a-switch v-model:checked="configForm.useCustomRegex" />
                </a-form-item>
                <template v-if="configForm.useCustomRegex">
                  <a-form-item label="正则表达式">
                    <a-input v-model:value="configForm.regexStr" placeholder="(.*)" />
                  </a-form-item>
                  <a-form-item label="替换字符串">
                    <a-input v-model:value="configForm.replaceStr" placeholder="$1" />
                  </a-form-item>
                </template>

                <a-divider>免费等待策略</a-divider>
                <a-form-item label="启用免费等待">
                  <a-switch v-model:checked="configForm.freeWaitEnabled" />
                </a-form-item>
                <a-row v-if="configForm.freeWaitEnabled" :gutter="16">
                  <a-col :span="8">
                    <a-form-item label="最大等待(秒)">
                      <a-input-number v-model:value="configForm.freeWaitMaxWaitSec" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="8">
                    <a-form-item label="重检间隔(秒)">
                      <a-input-number v-model:value="configForm.freeWaitRecheckSec" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="8">
                    <a-form-item label="最小剩余(秒)">
                      <a-input-number v-model:value="configForm.freeWaitMinRemain" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                </a-row>

                <a-divider>复检配置</a-divider>
                <a-form-item label="启用复检">
                  <a-switch v-model:checked="configForm.recheckEnabled" />
                </a-form-item>
                <a-row v-if="configForm.recheckEnabled" :gutter="16">
                  <a-col :span="8">
                    <a-form-item label="复检间隔(小时)">
                      <a-input-number v-model:value="configForm.recheckIntervalH" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="8">
                    <a-form-item label="最大复检次数">
                      <a-input-number v-model:value="configForm.recheckMaxCount" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="8">
                    <a-form-item label="最大复检年龄(小时)">
                      <a-input-number v-model:value="configForm.recheckMaxAgeH" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                </a-row>

                <a-divider>可行性评估</a-divider>
                <a-form-item label="启用可行性评估">
                  <a-switch v-model:checked="configForm.feasibilityEnabled" />
                </a-form-item>
                <a-row v-if="configForm.feasibilityEnabled" :gutter="16">
                  <a-col :span="8">
                    <a-form-item label="速度限制">
                      <a-input-number v-model:value="configForm.feasibilitySpeedLimit" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="8">
                    <a-form-item label="大小限制">
                      <a-input-number v-model:value="configForm.feasibilitySizeLimit" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="8">
                    <a-form-item label="安全系数">
                      <a-input-number v-model:value="configForm.feasibilitySafety" :min="0" :max="1" :step="0.1" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                </a-row>

                <a-divider>磁盘预算</a-divider>
                <a-form-item label="启用磁盘预算">
                  <a-switch v-model:checked="configForm.diskBudgetEnabled" />
                </a-form-item>
                <a-row v-if="configForm.diskBudgetEnabled" :gutter="16">
                  <a-col :span="12">
                    <a-form-item label="最低磁盘GB">
                      <a-input-number v-model:value="configForm.diskBudgetMinGB" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                </a-row>

                <a-divider>客户端选择</a-divider>
                <a-row :gutter="16">
                  <a-col :span="12">
                    <a-form-item label="选择模式">
                      <a-select v-model:value="configForm.clientSelection">
                        <a-select-option value="fixed">fixed</a-select-option>
                        <a-select-option value="round_robin">round_robin</a-select-option>
                        <a-select-option value="least_connections">least_connections</a-select-option>
                      </a-select>
                    </a-form-item>
                  </a-col>
                  <a-col :span="12">
                    <a-form-item label="候选客户端列表">
                      <a-select v-model:value="configForm.candidateClients" mode="multiple" :placeholder="'选择候选客户端'" :loading="downloadersLoading" style="width: 100%">
                        <a-select-option v-for="d in downloaders" :key="d.name" :value="d.name">
                          {{ d.name }}（{{ d.type }}）
                        </a-select-option>
                      </a-select>
                    </a-form-item>
                  </a-col>
                </a-row>

                <a-divider>磁盘守卫</a-divider>
                <a-row :gutter="16">
                  <a-col :span="12">
                    <a-form-item label="启用磁盘守卫">
                      <a-switch v-model:checked="configForm.diskGuardEnabled" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="12">
                    <a-form-item label="磁盘阈值(字节)">
                      <a-input-number v-model:value="configForm.diskGuardThreshold" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                </a-row>

                <a-divider>生命周期管理</a-divider>
                <a-row :gutter="16">
                  <a-col :span="8">
                    <a-form-item label="暂停阈值 seeders">
                      <a-input-number v-model:value="configForm.lifecyclePauseSeeders" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="8">
                    <a-form-item label="删除阈值 seeders">
                      <a-input-number v-model:value="configForm.lifecycleDeleteSeeders" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="8">
                    <a-form-item label="删除阈值小时">
                      <a-input-number v-model:value="configForm.lifecycleDeleteSeedHours" :min="0" style="width: 100%" />
                    </a-form-item>
                  </a-col>
                </a-row>
              </a-collapse-panel>
            </a-collapse>

            <a-form-item>
              <a-button type="primary" :loading="configSaving" @click="saveConfig">{{ t('common.saveConfig') }}</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>
        <a-tab-pane key="dryrun" :tab="t('subscription.dryrun')">
          <a-button type="primary" :loading="dryrunLoading" style="margin-bottom: 16px" @click="runDryrun">
            {{ t('subscription.runDryrun') }}
          </a-button>
          <a-table
            :columns="dryrunColumns"
            :data-source="dryrunResults"
            :pagination="false"
            row-key="title"
            size="small"
          />
        </a-tab-pane>
        <a-tab-pane key="history" :tab="t('subscription.fetchHistory')">
          <a-table
            :columns="historyColumns"
            :data-source="history"
            :loading="historyLoading"
            :pagination="{ pageSize: 20 }"
            row-key="id"
            size="small"
          />
        </a-tab-pane>
        <a-tab-pane key="rules" :tab="t('subscription.rules')">
          <p style="color: #666; margin-bottom: 16px">
            {{ t('subscription.rulesHint') }}
          </p>
          <div v-for="(cond, idx) in ruleConditions" :key="idx" style="margin-bottom: 12px">
            <a-row :gutter="8" align="middle">
              <a-col :span="6">
                <a-select v-model:value="cond.key" :placeholder="t('subscription.selectField')" style="width:100%">
                  <a-select-option value="title">{{ t('subscription.title') }}</a-select-option>
                  <a-select-option value="size">{{ t('subscription.volumeSize') }}</a-select-option>
                  <a-select-option value="uploader">{{ t('subscription.uploader') }}</a-select-option>
                  <a-select-option value="site">{{ t('common.site') }}</a-select-option>
                  <a-select-option value="free">{{ t('subscription.isFree') }}</a-select-option>
                  <a-select-option value="discount_level">{{ t('subscription.discountLevel') }}</a-select-option>
                </a-select>
              </a-col>
              <a-col :span="6">
                <a-select v-model:value="cond.compareType" :placeholder="t('subscription.compareMethod')" style="width:100%">
                  <a-select-option value="equals">{{ t('subscription.equals') }}</a-select-option>
                  <a-select-option value="bigger">{{ t('subscription.greaterThan') }}</a-select-option>
                  <a-select-option value="smaller">{{ t('subscription.lessThan') }}</a-select-option>
                  <a-select-option value="contain">{{ t('subscription.contains') }}</a-select-option>
                  <a-select-option value="not_contain">{{ t('subscription.notContains') }}</a-select-option>
                  <a-select-option value="regexp">{{ t('subscription.regexpMatch') }}</a-select-option>
                </a-select>
              </a-col>
              <a-col :span="10">
                <a-input-number
                  v-if="cond.key === 'size'"
                  v-model:value="cond.numValue"
                  style="width:100%"
                  :min="0"
                  :placeholder="t('subscription.unitBytes')"
                />
                <a-select v-else-if="cond.key === 'free'" v-model:value="cond.value" style="width:100%">
                  <a-select-option value="true">{{ t('subscription.yesFree') }}</a-select-option>
                  <a-select-option value="false">{{ t('subscription.noNotFree') }}</a-select-option>
                </a-select>
                <a-input v-else v-model:value="cond.value" :placeholder="t('subscription.inputValue')" style="width:100%" />
              </a-col>
              <a-col :span="2">
                <a-button type="text" danger :disabled="ruleConditions.length <= 1" @click="ruleConditions.splice(idx, 1)">
                  {{ t('subscription.deleteShort') }}
                </a-button>
              </a-col>
            </a-row>
            <div v-if="cond.key === 'size'" style="margin-top:4px;color:#999;font-size:12px">
              {{ t('subscription.bytesHint') }}
            </div>
          </div>
          <a-button type="dashed" block style="margin-bottom:16px" @click="ruleConditions.push({key:'title',compareType:'contain',value:'',numValue:0})">
            {{ t('subscription.addCondition') }}
          </a-button>
          <a-form layout="vertical" style="max-width: 600px">
            <a-form-item>
              <a-button type="primary" :loading="rulesSaving" @click="saveRules">{{ t('subscription.saveRulesBtn') }}</a-button>
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
const subscription = ref<Record<string, unknown>>({})
const dryrunResults = ref<Record<string, unknown>[]>([])
const history = ref<Record<string, unknown>[]>([])
const activeTab = ref('config')
const downloaders = ref<Record<string, unknown>[]>([])
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
  skipSameSize: false,
  skipSameSizeWindowMin: 30,
  skipSameSizeStrict: false,
  addCountPerHour: 0,
  useCustomRegex: false,
  regexStr: '',
  replaceStr: '',
  freeWaitEnabled: false,
  freeWaitMaxWaitSec: 7200,
  freeWaitRecheckSec: 600,
  freeWaitMinRemain: 30,
  recheckEnabled: false,
  recheckIntervalH: 6,
  recheckMaxCount: 5,
  recheckMaxAgeH: 72,
  feasibilityEnabled: false,
  feasibilitySpeedLimit: 0,
  feasibilitySizeLimit: 0,
  feasibilitySafety: 0.8,
  diskBudgetEnabled: false,
  diskBudgetMinGB: 10,
  candidateClients: [] as string[],
  clientSelection: 'fixed',
  diskGuardEnabled: true,
  diskGuardThreshold: 1073741824,
  lifecyclePauseSeeders: 0,
  lifecycleDeleteSeeders: 0,
  lifecycleDeleteSeedHours: 0,
})
const ruleConditions = ref<RuleCond[]>([{ key: 'title', compareType: 'contain', value: '', numValue: 0 }])
const rulesSaving = ref(false)

const dryrunColumns = [
  { title: t('subscription.title'), dataIndex: 'title', key: 'title', ellipsis: true },
  { title: t('common.size'), dataIndex: 'size', key: 'size', width: 100 },
  { title: t('subscription.matched'), dataIndex: 'matched', key: 'matched', width: 80 },
  { title: t('subscription.reason'), dataIndex: 'reason', key: 'reason' },
]

const historyColumns = [
  { title: t('subscription.time'), dataIndex: 'fetchedAt', key: 'fetchedAt', width: 180 },
  { title: t('subscription.newTorrents'), dataIndex: 'newCount', key: 'newCount', width: 100 },
  { title: t('common.status'), dataIndex: 'status', key: 'status', width: 100 },
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
      skipSameSize: subscription.value.skipSameSize || false,
      skipSameSizeWindowMin: subscription.value.skipSameSizeWindowMin || 30,
      skipSameSizeStrict: subscription.value.skipSameSizeStrict || false,
      addCountPerHour: subscription.value.addCountPerHour || 0,
      useCustomRegex: subscription.value.useCustomRegex || false,
      regexStr: subscription.value.regexStr || '',
      replaceStr: subscription.value.replaceStr || '',
      freeWaitEnabled: subscription.value.freeWaitEnabled || false,
      freeWaitMaxWaitSec: subscription.value.freeWaitMaxWaitSec || 7200,
      freeWaitRecheckSec: subscription.value.freeWaitRecheckSec || 600,
      freeWaitMinRemain: subscription.value.freeWaitMinRemain || 30,
      recheckEnabled: subscription.value.recheckEnabled || false,
      recheckIntervalH: subscription.value.recheckIntervalH || 6,
      recheckMaxCount: subscription.value.recheckMaxCount || 5,
      recheckMaxAgeH: subscription.value.recheckMaxAgeH || 72,
      feasibilityEnabled: subscription.value.feasibilityEnabled || false,
      feasibilitySpeedLimit: subscription.value.feasibilitySpeedLimit || 0,
      feasibilitySizeLimit: subscription.value.feasibilitySizeLimit || 0,
      feasibilitySafety: subscription.value.feasibilitySafety ?? 0.8,
      diskBudgetEnabled: subscription.value.diskBudgetEnabled || false,
      diskBudgetMinGB: subscription.value.diskBudgetMinGB || 10,
      candidateClients: subscription.value.candidateClients || [],
      clientSelection: subscription.value.clientSelection || 'fixed',
      diskGuardEnabled: subscription.value.diskGuardEnabled ?? true,
      diskGuardThreshold: subscription.value.diskGuardThreshold || 1073741824,
      lifecyclePauseSeeders: subscription.value.lifecyclePauseSeeders || 0,
      lifecycleDeleteSeeders: subscription.value.lifecycleDeleteSeeders || 0,
      lifecycleDeleteSeedHours: subscription.value.lifecycleDeleteSeedHours || 0,
    })
    const conds = subscription.value.conditions || []
    if (Array.isArray(conds) && conds.length) {
      ruleConditions.value = conds.map((c: Record<string, unknown>) => ({
        key: String(c.key || 'title'),
        compareType: String(c.compare_type || c.compareType || 'contain'),
        value: String(c.value || ''),
        numValue: c.key === 'size' ? Number(c.value || 0) : 0,
      }))
    }
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  configSaving.value = true
  try {
    const payload: Record<string, unknown> = {
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
      skipSameSize: configForm.skipSameSize,
      skipSameSizeWindowMin: configForm.skipSameSizeWindowMin,
      skipSameSizeStrict: configForm.skipSameSizeStrict,
      addCountPerHour: configForm.addCountPerHour,
      useCustomRegex: configForm.useCustomRegex,
      regexStr: configForm.regexStr,
      replaceStr: configForm.replaceStr,
      freeWaitEnabled: configForm.freeWaitEnabled,
      freeWaitMaxWaitSec: configForm.freeWaitMaxWaitSec,
      freeWaitRecheckSec: configForm.freeWaitRecheckSec,
      freeWaitMinRemain: configForm.freeWaitMinRemain,
      recheckEnabled: configForm.recheckEnabled,
      recheckIntervalH: configForm.recheckIntervalH,
      recheckMaxCount: configForm.recheckMaxCount,
      recheckMaxAgeH: configForm.recheckMaxAgeH,
      feasibilityEnabled: configForm.feasibilityEnabled,
      feasibilitySpeedLimit: configForm.feasibilitySpeedLimit,
      feasibilitySizeLimit: configForm.feasibilitySizeLimit,
      feasibilitySafety: configForm.feasibilitySafety,
      diskBudgetEnabled: configForm.diskBudgetEnabled,
      diskBudgetMinGB: configForm.diskBudgetMinGB,
      candidateClients: configForm.candidateClients,
      clientSelection: configForm.clientSelection,
      diskGuardEnabled: configForm.diskGuardEnabled,
      diskGuardThreshold: configForm.diskGuardThreshold,
      lifecyclePauseSeeders: configForm.lifecyclePauseSeeders,
      lifecycleDeleteSeeders: configForm.lifecycleDeleteSeeders,
      lifecycleDeleteSeedHours: configForm.lifecycleDeleteSeedHours,
    }
    await subscriptionsApi.update(id, payload)
    message.success(t('common.configSaved'))
    fetchSubscription()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    configSaving.value = false
  }
}

async function runDryrun() {
  dryrunLoading.value = true
  try {
    const resp = await subscriptionsApi.dryrun(id)
    const data = resp.data.data || {}
    dryrunResults.value = (data.recentTorrents || []).map((torrent: Record<string, unknown>) => ({
      title: torrent.title || torrent.name || '-',
      size: torrent.size ? (Number(torrent.size) / 1073741824).toFixed(2) + ' GB' : '-',
      matched: torrent.matched ? t('common.yes') : t('common.no'),
      reason: torrent.reason || '-',
    }))
    message.success(t('subscription.dryrunComplete', { count: data.total || 0 }))
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    dryrunLoading.value = false
  }
}

async function fetchHistory() {
  historyLoading.value = true
  try {
    const resp = await subscriptionsApi.get(id)
    const sub = resp.data.data || {}
    history.value = (sub.recentFetches || []).map((f: Record<string, unknown>, idx: number) => ({
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
    message.success(t('subscription.rulesSaved'))
  } catch (e: unknown) {
    message.error((e as Error).message)
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
