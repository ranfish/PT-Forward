<template>
  <a-card title="加种限制">
    <template #extra>
      <a-button type="primary" size="small" @click="handleAdd">添加限制</a-button>
    </template>
    <a-table :data-source="limits" :loading="loading" :pagination="false" row-key="site_name" size="small">
      <a-table-column title="站点" data-index="site_name" :width="150" />
      <a-table-column title="状态" :width="80">
        <template #default="{ record }">
          <a-tag :color="record.enabled ? 'green' : 'default'">{{ record.enabled ? '启用' : '关闭' }}</a-tag>
        </template>
      </a-table-column>
      <a-table-column title="周期" :width="100">
        <template #default="{ record }">{{ record.window_hours }}h</template>
      </a-table-column>
      <a-table-column title="上限" :width="80">
        <template #default="{ record }">{{ record.max_count }}</template>
      </a-table-column>
      <a-table-column title="当前" :width="100">
        <template #default="{ record }">
          <span :style="{ color: (record.current_count || 0) >= record.max_count ? '#ff4d4f' : '' }">
            {{ record.current_count || 0 }} / {{ record.max_count }}
          </span>
        </template>
      </a-table-column>
      <a-table-column title="操作" :width="120">
        <template #default="{ record }">
          <a-button size="small" type="link" @click="handleEdit(record)">编辑</a-button>
          <a-button size="small" type="link" danger @click="handleDelete(record)">删除</a-button>
        </template>
      </a-table-column>
    </a-table>

    <a-modal v-model:open="dialogVisible" :title="editingLimit.id ? '编辑限制' : '添加限制'" @ok="handleSave" :confirm-loading="saving" width="420px">
      <a-form layout="vertical">
        <a-form-item label="站点">
          <a-select
            v-model:value="editingLimit.site_name"
            show-search
            placeholder="选择或输入站点名"
            :disabled="!!editingLimit.id"
          >
            <a-select-option v-for="s in siteOptions" :key="s" :value="s">{{ s }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="启用">
          <a-switch v-model:checked="editingLimit.enabled" />
        </a-form-item>
        <a-form-item label="最大数量">
          <a-input-number v-model:value="editingLimit.max_count" :min="1" :max="999" />
        </a-form-item>
        <a-form-item label="周期">
          <DurationInput v-model:display="windowDisplay" v-model:unit="windowUnit" :min="1" :max="168" />
        </a-form-item>
      </a-form>
    </a-modal>
  </a-card>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { publishLimitApi, type PublishLimit } from '@/api/image-host'
import { sitesApi } from '@/api/sites'
import DurationInput from '@/components/DurationInput.vue'
import { durationStoreValue, durationDisplayValue, smartDurationUnit } from '@/utils/duration'
import type { DurationUnit } from '@/utils/duration'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const limits = ref<PublishLimit[]>([])
const siteOptions = ref<string[]>([])

// §59.596 刀2: window_hours 智能回显（底层小时不动）
const windowDisplay = ref(24)
const windowUnit = ref<DurationUnit>('hours')
const editingLimit = reactive<PublishLimit>({
  site_name: '',
  enabled: true,
  max_count: 20,
  window_hours: 24,
})

async function loadLimits() {
  loading.value = true
  try {
    const { data } = await publishLimitApi.list()
    limits.value = data.data.items || []
  } catch {
    message.error('加载限制列表失败')
  } finally {
    loading.value = false
  }
}

async function loadSites() {
  try {
    const { data } = await sitesApi.list(1, 200)
    const items = (data.data as any)?.items || (data.data as any) || []
    siteOptions.value = items.map((s: any) => s.name).filter(Boolean)
  } catch {
    // ignore
  }
}

function handleAdd() {
  Object.assign(editingLimit, { id: undefined, site_name: '', enabled: true, max_count: 20, window_hours: 24 })
  windowUnit.value = 'hours' // §59.596 刀2: 新建默认（window 语义小时惯例）
  windowDisplay.value = 24
  dialogVisible.value = true
}

function handleEdit(row: PublishLimit) {
  Object.assign(editingLimit, row)
  // §59.596 刀2: 智能回显同步（底层小时→显示）
  const h = row.window_hours ?? 24
  windowUnit.value = smartDurationUnit(h * 3600)
  windowDisplay.value = durationDisplayValue(h, windowUnit.value, 3600)
  dialogVisible.value = true
}

async function handleSave() {
  if (!editingLimit.site_name) {
    message.warning('请选择站点')
    return
  }
  saving.value = true
  try {
    // §59.596 刀2: 显示值换算落底层小时
    await publishLimitApi.upsert({ ...editingLimit, window_hours: durationStoreValue(windowDisplay.value, windowUnit.value, 3600) })
    message.success('保存成功')
    dialogVisible.value = false
    await loadLimits()
  } catch {
    message.error('保存失败')
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: PublishLimit) {
  try {
    await publishLimitApi.delete(row.site_name)
    message.success('删除成功')
    await loadLimits()
  } catch {
    message.error('删除失败')
  }
}

onMounted(() => {
  loadLimits()
  loadSites()
})
</script>
