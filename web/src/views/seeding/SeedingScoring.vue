<template>
  <div>
    <a-page-header :title="t('seeding.scoring.title')" @back="$router.push('/seeding')" />

    <a-row :gutter="16">
      <a-col :span="12">
        <a-card :title="t('seeding.scoring.config')" style="margin-bottom: 16px">
          <a-form-item :label="t('seeding.scoring.linkedSubscription')" style="margin-bottom: 16px">
            <a-select
              v-model:value="selectedSubId"
              :placeholder="t('seeding.scoring.selectSubscription')"
              allow-clear
              style="width: 100%"
              @change="fetchScoringConfig"
            >
              <a-select-option v-for="sub in subscriptions" :key="sub.id" :value="sub.id">{{ sub.name }} (ID: {{ sub.id }})</a-select-option>
            </a-select>
          </a-form-item>
          <a-spin :spinning="configLoading">
            <a-alert v-if="!selectedSubId" :message="t('seeding.scoring.selectSubscriptionHint')" type="info" show-icon style="margin-bottom: 16px" />
            <div :style="{ opacity: selectedSubId ? 1 : 0.4, pointerEvents: selectedSubId ? 'auto' : 'none' }">
              <a-form :model="scoringConfig" layout="vertical">
                <a-form-item :label="t('seeding.scoring.enabled')">
                  <a-switch v-model:checked="scoringConfig.enabled" @change="saveScoringConfig" />
                </a-form-item>
                <a-form-item :label="t('seeding.scoring.halfLifeHours')">
                  <a-input-number v-model:value="scoringConfig.halfLifeHours" :min="0.1" :step="0.5" style="width: 100%" />
                </a-form-item>
                <a-form-item :label="t('seeding.scoring.minScore')">
                  <a-input-number v-model:value="scoringConfig.minScore" :min="0" :step="0.1" style="width: 100%" />
                </a-form-item>
                <a-form-item :label="t('seeding.scoring.maxCandidates')">
                  <a-input-number v-model:value="scoringConfig.maxCandidates" :min="1" style="width: 100%" />
                </a-form-item>
                <a-form-item :label="t('seeding.scoring.maxActiveSeeding')">
                  <a-input-number v-model:value="scoringConfig.maxActiveSeeding" :min="1" style="width: 100%" />
                </a-form-item>
                <a-form-item :label="t('seeding.scoring.topNConfirm')">
                  <a-input-number v-model:value="scoringConfig.topNConfirm" :min="1" style="width: 100%" />
                </a-form-item>
                <a-form-item :label="t('seeding.scoring.include2xUp')">
                  <a-switch v-model:checked="scoringConfig.include2xUp" @change="saveScoringConfig" />
                </a-form-item>
                <a-form-item :label="t('seeding.scoring.batchLimit')">
                  <a-input-number v-model:value="scoringConfig.batchLimit" :min="1" style="width: 100%" />
                  <div style="font-size: 11px; color: #999; margin-top: 2px">{{ t('seeding.scoring.batchLimitHint') }}</div>
                </a-form-item>
                <a-form-item :label="t('seeding.scoring.pushIntervalMs')">
                  <a-input-number v-model:value="scoringConfig.pushIntervalMs" :min="0" :step="100" style="width: 100%" />
                  <div style="font-size: 11px; color: #999; margin-top: 2px">{{ t('seeding.scoring.pushIntervalMsHint') }}</div>
                </a-form-item>
                <a-form-item>
                  <a-button type="primary" :loading="saving" @click="saveScoringConfig">{{ t('common.saveConfig') }}</a-button>
                </a-form-item>
              </a-form>
            </div>
          </a-spin>
        </a-card>

        <a-card :title="t('seeding.scoring.dryrun')">
          <a-form :model="dryrunForm" layout="vertical">
            <a-row :gutter="12">
              <a-col :span="12">
                <a-form-item :label="t('seeding.scoring.seeders')">
                  <a-input-number v-model:value="dryrunForm.seeders" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('seeding.scoring.leechers')">
                  <a-input-number v-model:value="dryrunForm.leechers" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="12">
              <a-col :span="12">
                <a-form-item :label="t('seeding.scoring.ageHours')">
                  <a-input-number v-model:value="dryrunForm.ageHours" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('seeding.scoring.sizeGB')">
                  <a-input-number v-model:value="dryrunForm.sizeGB" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="12">
              <a-col :span="12">
                <a-form-item :label="t('seeding.scoring.discount')">
                  <a-select v-model:value="dryrunForm.discount" style="width: 100%">
                    <a-select-option value="">{{ t('seeding.scoring.discountNone') }}</a-select-option>
                    <a-select-option value="FREE">{{ t('seeding.scoring.discountFree') }}</a-select-option>
                    <a-select-option value="2XFREE">{{ t('seeding.scoring.discount2xFree') }}</a-select-option>
                    <a-select-option value="2XUP">{{ t('seeding.scoring.discount2xUp') }}</a-select-option>
                    <a-select-option value="PERCENT_50">{{ t('seeding.scoring.discountHalfDown') }}</a-select-option>
                    <a-select-option value="2X50">{{ t('seeding.scoring.discount2xHalfDown') }}</a-select-option>
                    <a-select-option value="PERCENT_25">{{ t('seeding.scoring.discount25Down') }}</a-select-option>
                    <a-select-option value="PERCENT_70">{{ t('seeding.scoring.discount70Down') }}</a-select-option>
                    <a-select-option value="PERCENT_75">{{ t('seeding.scoring.discount75Down') }}</a-select-option>
                    <a-select-option value="CUSTOM">{{ t('seeding.scoring.discountCustom') }}</a-select-option>
                  </a-select>
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('seeding.scoring.siteWeight')">
                  <a-input-number v-model:value="dryrunForm.siteWeight" :min="0" :step="0.1" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-form-item>
              <a-button type="primary" :loading="dryrunLoading" @click="runDryrun">{{ t('seeding.scoring.runDryrun') }}</a-button>
            </a-form-item>
          </a-form>
          <a-descriptions v-if="dryrunResult" bordered :column="2" size="small">
            <a-descriptions-item :label="t('seeding.scoring.score')">{{ Number.isFinite(dryrunResult.score) ? dryrunResult.score!.toFixed(4) : '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('seeding.scoring.effectiveScore')">{{ Number.isFinite(dryrunResult.effectiveScore) ? dryrunResult.effectiveScore!.toFixed(4) : '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('seeding.scoring.demandScore')">{{ Number.isFinite(dryrunResult.demandScore) ? dryrunResult.demandScore!.toFixed(4) : '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('seeding.scoring.recencyFactor')">{{ Number.isFinite(dryrunResult.recencyFactor) ? dryrunResult.recencyFactor!.toFixed(4) : '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('seeding.scoring.shouldCleanup')">
              <a-tag :color="dryrunResult.shouldCleanup ? 'red' : 'green'">{{ dryrunResult.shouldCleanup ? t('common.yes') : t('common.no') }}</a-tag>
            </a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>

      <a-col :span="12">
        <a-card :title="t('seeding.scoring.logs')">
          <a-table
            :columns="logColumns"
            :data-source="scoringLogs"
            :loading="logsLoading"
            :pagination="{ pageSize: 15 }"
            row-key="id"
            size="small"
          />
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { seedingApi } from '@/api/seeding'
import { subscriptionsApi } from '@/api/subscriptions'
import { formatTime, copyToClipboard } from '@/utils/format'

interface DryrunResult {
  score?: number
  effectiveScore?: number
  demandScore?: number
  recencyFactor?: number
  shouldCleanup?: boolean
}

interface ScoringLogItem {
  id: number
  title: string
  site_name: string
  size: string
  created_at: string
}

const { t } = useI18n()

function copyHash(text: string) {
  copyToClipboard(text)
  message.success(t('common.copied'))
}

const configLoading = ref(false)
const saving = ref(false)
const dryrunLoading = ref(false)
const logsLoading = ref(false)
const dryrunResult = ref<DryrunResult | null>(null)
const scoringLogs = ref<ScoringLogItem[]>([])
const subscriptions = ref<{ id: number; name: string }[]>([])
const selectedSubId = ref<number | undefined>(undefined)
const scoringConfig = reactive({
  enabled: false,
  halfLifeHours: 2,
  minScore: 1.0,
  maxCandidates: 50,
  maxActiveSeeding: 100,
  topNConfirm: 10,
  include2xUp: false,
  batchLimit: 5,
  pushIntervalMs: 0,
})

const dryrunForm = reactive({
  seeders: 100,
  leechers: 50,
  ageHours: 24,
  sizeGB: 10,
  discount: '',
  siteWeight: 1.0,
  halfLifeHours: 2,
})

const logColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('seeding.infoHash'), dataIndex: 'info_hash', key: 'info_hash', ellipsis: true, customRender: ({ text }: { text: string }) => h('span', { style: 'cursor:pointer;font-family:monospace;font-size:12px', onClick: () => copyHash(text) }, text) },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 100 },
  { title: t('seeding.scoringScore'), dataIndex: 'score', key: 'score', width: 80 },
  { title: t('common.time'), dataIndex: 'created_at', key: 'created_at', width: 170, customRender: ({ text }: { text: string }) => formatTime(text) },
]

async function fetchScoringConfig() {
  configLoading.value = true
  try {
    const resp = await seedingApi.getScoringConfig(selectedSubId.value)
    const data = resp.data.data || {}
    Object.assign(scoringConfig, {
      enabled: data.enabled ?? false,
      halfLifeHours: data.half_life_hours ?? 2,
      minScore: data.min_score ?? 1.0,
      maxCandidates: data.max_candidates ?? 50,
      maxActiveSeeding: data.max_active_seeding ?? 100,
      topNConfirm: data.top_n_confirm ?? 10,
      include2xUp: data.include_2xup ?? false,
      batchLimit: data.batch_limit ?? 5,
      pushIntervalMs: data.push_interval_ms ?? 0,
    })
    dryrunForm.halfLifeHours = scoringConfig.halfLifeHours
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    configLoading.value = false
  }
}

async function saveScoringConfig() {
  saving.value = true
  try {
    await seedingApi.updateScoringConfig(scoringConfig, selectedSubId.value)
    message.success(t('common.configSaved'))
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

async function runDryrun() {
  dryrunLoading.value = true
  try {
    const resp = await seedingApi.scoringDryrun({
      seeders: dryrunForm.seeders,
      leechers: dryrunForm.leechers,
      ageHours: dryrunForm.ageHours,
      size: dryrunForm.sizeGB * 1073741824,
      discount: dryrunForm.discount,
      halfLifeHours: dryrunForm.halfLifeHours,
      siteWeight: dryrunForm.siteWeight,
    })
    dryrunResult.value = resp.data.data
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    dryrunLoading.value = false
  }
}

async function fetchScoringLogs() {
  logsLoading.value = true
  try {
    const resp = await seedingApi.listScoringLogs({ limit: 30 })
    scoringLogs.value = (resp.data.data?.items || []) as unknown as ScoringLogItem[]
  } catch {
    scoringLogs.value = []
  } finally {
    logsLoading.value = false
  }
}

async function fetchSubscriptions() {
  try {
    const resp = await subscriptionsApi.list(1, 200)
    subscriptions.value = resp.data.data?.items || resp.data.data || []
  } catch (e: unknown) {
    console.warn('fetchSubscriptions failed:', e instanceof Error ? e.message : String(e))
  }
}

onMounted(() => {
  fetchSubscriptions()
  fetchScoringConfig()
  fetchScoringLogs()
})
</script>
