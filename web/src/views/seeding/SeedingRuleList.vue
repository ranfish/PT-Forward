<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        添加规则
      </a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="rules"
      :loading="loading"
      :pagination="false"
      row-key="id"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" @change="toggleRule(record)" />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="openModal(record)">编辑</a-button>
            <a-button type="link" size="small" @click="testRule(record.id)">测试</a-button>
            <a-popconfirm title="确定删除该规则？" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRule ? '编辑规则' : '添加规则'"
      @ok="handleSubmit"
      :confirm-loading="submitting"
      width="640px"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item label="规则别名" name="alias" :rules="[{ required: true, message: '请输入规则别名' }]">
          <a-input v-model:value="form.alias" placeholder="规则别名" />
        </a-form-item>
        <a-form-item label="类型" name="type">
          <a-select v-model:value="form.type" placeholder="选择类型">
            <a-select-option value="normal">普通</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="条件 (JSON)" name="conditions">
          <a-textarea v-model:value="form.conditions" :rows="3" placeholder='如 [{"field":"seed_time","op":"gt","value":720}]' />
        </a-form-item>
        <a-form-item label="表达式" name="expr">
          <a-input v-model:value="form.expr" placeholder="CEL 表达式（可选）" />
        </a-form-item>
        <a-form-item label="动作" name="action">
          <a-select v-model:value="form.action" placeholder="选择动作">
            <a-select-option value="delete">删除种子</a-select-option>
            <a-select-option value="pause">暂停种子</a-select-option>
            <a-select-option value="limit_speed">限速</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="优先级" name="priority">
          <a-input-number v-model:value="form.priority" :min="0" style="width: 100%" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { deleteRulesApi } from '@/api/seeding'

const loading = ref(false)
const modalVisible = ref(false)
const submitting = ref(false)
const editingRule = ref<any>(null)
const rules = ref<any[]>([])

const form = reactive({
  alias: '',
  type: 'normal',
  conditions: '',
  expr: '',
  action: 'delete',
  priority: 0,
})

const columns = [
  { title: '别名', dataIndex: 'alias', key: 'alias' },
  { title: '类型', dataIndex: 'type', key: 'type', width: 80 },
  { title: '条件', dataIndex: 'conditions', key: 'conditions', ellipsis: true },
  { title: '动作', dataIndex: 'action', key: 'action', width: 100 },
  { title: '优先级', dataIndex: 'priority', key: 'priority', width: 80 },
  { title: '启用', key: 'enabled', width: 80 },
  { title: '操作', key: 'actions', width: 200 },
]

async function fetchRules() {
  loading.value = true
  try {
    const resp = await deleteRulesApi.list()
    rules.value = resp.data.data?.items || resp.data.data || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

function openModal(record?: any) {
  editingRule.value = record || null
  if (record) {
    Object.assign(form, {
      alias: record.alias || '',
      type: record.type || 'normal',
      conditions: record.conditions || '',
      expr: record.expr || '',
      action: record.action || 'delete',
      priority: record.priority || 0,
    })
  } else {
    Object.assign(form, { alias: '', type: 'normal', conditions: '', expr: '', action: 'delete', priority: 0 })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingRule.value) {
      await deleteRulesApi.update(editingRule.value.id, form)
    } else {
      await deleteRulesApi.create(form)
    }
    message.success('操作成功')
    modalVisible.value = false
    fetchRules()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await deleteRulesApi.delete(id)
    message.success('删除成功')
    fetchRules()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function testRule(id: number) {
  try {
    const resp = await deleteRulesApi.test(id)
    const result = resp.data.data
    message.success(`匹配 ${result?.matched?.length || 0} / ${result?.total || 0} 个种子`)
  } catch (e: any) {
    message.error(e.message)
  }
}

async function toggleRule(record: any) {
  try {
    await deleteRulesApi.update(record.id, { ...record, enabled: !record.enabled })
    message.success('状态已切换')
    fetchRules()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(fetchRules)
</script>
