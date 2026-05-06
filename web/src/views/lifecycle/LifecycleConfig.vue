<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('lifecycle.queueDepth')" :value="backpressure.queue_depth || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('lifecycle.activePublishes')" :value="backpressure.active_publishes || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('lifecycle.maxConcurrent')" :value="backpressure.max_concurrent_publishes || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('lifecycle.throttled')">
            <template #formatter>
              <a-tag :color="backpressure.is_throttled ? 'red' : 'green'">{{ backpressure.is_throttled ? t('common.yes') : t('common.no') }}</a-tag>
            </template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('lifecycle.title')">
      <a-spin :spinning="loading">
        <a-form :model="config" layout="vertical">
          <a-form-item :label="t('lifecycle.pauseSeeders')">
            <a-switch v-model:checked="config.pauseSeeders" />
          </a-form-item>
          <a-form-item :label="t('lifecycle.deleteSeeders')">
            <a-switch v-model:checked="config.deleteSeeders" />
          </a-form-item>
          <a-form-item :label="t('lifecycle.seedRetentionHours')">
            <a-input-number v-model:value="config.deleteSeedHours" :min="0" style="width: 200px" />
          </a-form-item>
          <a-form-item :label="t('lifecycle.checkInterval')">
            <a-input v-model:value="config.checkInterval" placeholder="如 5m" style="width: 200px" />
          </a-form-item>
          <a-form-item :label="t('lifecycle.maxConcurrentChecks')">
            <a-input-number v-model:value="config.maxConcurrentChecks" :min="1" style="width: 200px" />
          </a-form-item>
          <a-form-item>
            <a-button type="primary" @click="saveConfig" :loading="saving">{{ t('common.saveConfig') }}</a-button>
          </a-form-item>
        </a-form>
      </a-spin>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { lifecycleApi } from '@/api/lifecycle'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const backpressure = ref<any>({})

const config = reactive({
  pauseSeeders: true,
  deleteSeeders: false,
  deleteSeedHours: 720,
  checkInterval: '5m',
  maxConcurrentChecks: 10,
})

async function fetchConfig() {
  loading.value = true
  try {
    const resp = await lifecycleApi.getConfig()
    const data = resp.data.data || {}
    config.pauseSeeders = data.pauseSeeders ?? true
    config.deleteSeeders = data.deleteSeeders ?? false
    config.deleteSeedHours = data.deleteSeedHours ?? 720
    config.checkInterval = data.checkInterval ?? '5m'
    config.maxConcurrentChecks = data.maxConcurrentChecks ?? 10
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function fetchBackpressure() {
  try {
    const resp = await lifecycleApi.getBackpressure()
    backpressure.value = resp.data.data || {}
  } catch (e: any) {
    message.error(e.message)
  }
}

async function saveConfig() {
  saving.value = true
  try {
    await lifecycleApi.updateConfig(config)
    message.success(t('common.configSaved'))
  } catch (e: any) {
    message.error(e.message)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  fetchConfig()
  fetchBackpressure()
})
</script>
