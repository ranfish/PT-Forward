<template>
  <div>
    <a-alert type="info" show-icon style="margin-bottom: 16px">
      <template #message>全局排除规则对所有订阅生效。匹配到这些规则的种子将被直接跳过，不会推送到下载器。</template>
    </a-alert>

    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        添加排除规则
      </a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="pagination.data.value"
      :loading="pagination.loading.value"
      :pagination="{
        current: pagination.currentPage.value,
        pageSize: pagination.pageSize.value,
        total: pagination.total.value,
        showSizeChanger: true,
        showTotal: (total: number) => `共 ${total} 条`,
      }"
      row-key="id"
      @change="(pag: any) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" @change="toggleEnabled(record)" />
        </template>
        <template v-if="column.key === 'conditions'">
          <span v-for="(c, i) in (record.conditions || [])" :key="i">
            <a-tag v-if="i < 3">{{ keyLabels[c.key] || c.key }} {{ compareLabels[c.compare_type || c.compareType] || c.compare_type || c.compareType }} {{ formatValue(c) }}</a-tag>
          </span>
          <span v-if="(record.conditions || []).length > 3" style="color: #999">+{{ record.conditions.length - 3 }} 条</span>
          <span v-if="!(record.conditions || []).length" style="color: #999">无</span>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="openModal(record)">编辑</a-button>
            <a-popconfirm title="确定删除此规则？" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRule ? '编辑排除规则' : '添加排除规则'"
      @ok="handleSubmit"
      :confirm-loading="submitting"
      width="720px"
    >
      <a-form :model="form" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="16">
            <a-form-item label="规则名称" name="name" :rules="[{ required: true, message: '请输入规则名称' }]">
              <a-input v-model:value="form.name" placeholder="如：排除禁转资源" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="启用">
              <a-switch v-model:checked="form.enabled" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>匹配条件（全部满足则排除该种子）</a-divider>

        <div v-for="(cond, idx) in form.conditionList" :key="idx" style="margin-bottom: 12px">
          <a-row :gutter="8" align="middle">
            <a-col :span="6">
              <a-select v-model:value="cond.key" placeholder="选择字段" style="width:100%">
                <a-select-option value="title">标题</a-select-option>
                <a-select-option value="size">体积大小</a-select-option>
                <a-select-option value="uploader">发布者</a-select-option>
                <a-select-option value="site">站点</a-select-option>
                <a-select-option value="category">分类</a-select-option>
                <a-select-option value="free">是否免费</a-select-option>
                <a-select-option value="tags">标签</a-select-option>
              </a-select>
            </a-col>
            <a-col :span="6">
              <a-select v-model:value="cond.compareType" placeholder="比较方式" style="width:100%">
                <a-select-option v-for="opt in compareOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</a-select-option>
              </a-select>
            </a-col>
            <a-col :span="10">
              <a-input-number
                v-if="cond.key === 'size'"
                v-model:value="cond.numValue"
                style="width:100%"
                :min="0"
                placeholder="输入数值"
              />
              <a-select
                v-else-if="cond.key === 'free'"
                v-model:value="cond.value"
                style="width:100%"
                placeholder="选择"
              >
                <a-select-option value="true">是（免费）</a-select-option>
                <a-select-option value="false">否（非免费）</a-select-option>
              </a-select>
              <a-input v-else v-model:value="cond.value" placeholder="输入值" style="width:100%" />
            </a-col>
            <a-col :span="2">
              <a-button type="text" danger @click="removeCondition(idx)" :disabled="form.conditionList.length <= 1">
                <template #icon><DeleteOutlined /></template>
              </a-button>
            </a-col>
          </a-row>
          <div v-if="cond.key === 'size'" style="margin-top: 4px; color: #999; font-size: 12px; padding-left: 4px">
            单位：字节（1 GB = 1073741824，100 MB = 104857600）
          </div>
        </div>

        <a-button type="dashed" block @click="addCondition" style="margin-bottom: 16px">
          <template #icon><PlusOutlined /></template>
          添加条件
        </a-button>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons-vue'
import { filterRulesApi } from '@/api/filter-rules'
import { usePagination } from '@/composables/usePagination'

const modalVisible = ref(false)
const submitting = ref(false)
const editingRule = ref<any>(null)

interface ConditionForm {
  key: string
  compareType: string
  value: string
  numValue: number
}

const form = reactive({
  name: '',
  conditionList: [] as ConditionForm[],
  enabled: true,
})

const keyLabels: Record<string, string> = {
  title: '标题', size: '体积', uploader: '发布者', site: '站点',
  category: '分类', free: '免费', tags: '标签',
}

const compareLabels: Record<string, string> = {
  equals: '等于', bigger: '大于', smaller: '小于',
  contain: '包含', not_contain: '不包含',
  include_in: '属于', not_include_in: '不属于',
  regexp: '正则匹配', not_regexp: '正则不匹配',
}

const compareOptions = [
  { value: 'equals', label: '等于' },
  { value: 'bigger', label: '大于' },
  { value: 'smaller', label: '小于' },
  { value: 'contain', label: '包含' },
  { value: 'not_contain', label: '不包含' },
  { value: 'include_in', label: '属于（逗号分隔）' },
  { value: 'not_include_in', label: '不属于（逗号分隔）' },
  { value: 'regexp', label: '正则匹配' },
  { value: 'not_regexp', label: '正则不匹配' },
]

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '排除条件', key: 'conditions' },
  { title: '启用', key: 'enabled', width: 80, align: 'center' as const },
  { title: '操作', key: 'actions', width: 140 },
]

const pagination = usePagination((page, size) => filterRulesApi.list(page, size))

function formatValue(c: any) {
  const v = c.value || ''
  if (c.key === 'size' && v) {
    const n = Number(v)
    if (n >= 1073741824) return (n / 1073741824).toFixed(1) + ' GB'
    if (n >= 1048576) return (n / 1048576).toFixed(0) + ' MB'
    return v + ' B'
  }
  return v
}

function addCondition() {
  form.conditionList.push({ key: 'title', compareType: 'contain', value: '', numValue: 0 })
}

function removeCondition(idx: number) {
  form.conditionList.splice(idx, 1)
}

function conditionsToForm(conditions: any[]) {
  return (conditions || []).map((c: any) => ({
    key: c.key || 'title',
    compareType: c.compare_type || c.compareType || 'contain',
    value: c.value || '',
    numValue: c.key === 'size' ? Number(c.value || 0) : 0,
  }))
}

function formToConditions() {
  return form.conditionList.map(c => ({
    key: c.key,
    compare_type: c.compareType,
    value: c.key === 'size' ? String(c.numValue || 0) : c.value,
  }))
}

function openModal(record?: any) {
  editingRule.value = record || null
  if (record) {
    form.conditionList = conditionsToForm(record.conditions)
    if (!form.conditionList.length) form.conditionList.push({ key: 'title', compareType: 'contain', value: '', numValue: 0 })
    Object.assign(form, {
      name: record.name || '',
      enabled: record.enabled ?? true,
    })
  } else {
    Object.assign(form, {
      name: '', conditionList: [{ key: 'title', compareType: 'contain', value: '', numValue: 0 }], enabled: true,
    })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  if (!form.name.trim()) {
    message.error('请输入规则名称')
    return
  }
  if (!form.conditionList.length) {
    message.error('至少需要一个条件')
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.name,
      ruleType: 'reject',
      conditions: formToConditions(),
      enabled: form.enabled,
    }
    if (editingRule.value) {
      await filterRulesApi.update(editingRule.value.id, payload)
    } else {
      await filterRulesApi.create(payload)
    }
    message.success('操作成功')
    modalVisible.value = false
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await filterRulesApi.delete(id)
    message.success('删除成功')
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function toggleEnabled(record: any) {
  try {
    await filterRulesApi.update(record.id, { ...record, enabled: !record.enabled })
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
