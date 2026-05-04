<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic title="队列深度" :value="backpressure.queue_depth || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="活跃发布" :value="backpressure.active_publishes || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="最大并发" :value="backpressure.max_concurrent_publishes || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="是否限流">
            <template #formatter>
              <a-tag :color="backpressure.is_throttled ? 'red' : 'green'">{{ backpressure.is_throttled ? '是' : '否' }}</a-tag>
            </template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <a-card title="生命周期配置">
      <a-spin :spinning="loading">
        <a-form :model="config" layout="vertical">
          <a-form-item label="暂停做种者">
            <a-switch v-model:checked="config.pauseSeeders" />
          </a-form-item>
          <a-form-item label="删除做种者">
            <a-switch v-model:checked="config.deleteSeeders" />
          </a-form-item>
          <a-form-item label="做种保留时间（小时）">
            <a-input-number v-model:value="config.deleteSeedHours" :min="0" style="width: 200px" />
          </a-form-item>
          <a-form-item label="检查间隔">
            <a-input v-model:value="config.checkInterval" placeholder="如 5m" style="width: 200px" />
          </a-form-item>
          <a-form-item label="最大并发检查数">
            <a-input-number v-model:value="config.maxConcurrentChecks" :min="1" style="width: 200px" />
          </a-form-item>
          <a-form-item>
            <a-button type="primary" @click="saveConfig" :loading="saving">保存配置</a-button>
          </a-form-item>
        </a-form>
      </a-spin>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { lifecycleApi } from '@/api/lifecycle'

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
    message.success('配置已保存')
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
