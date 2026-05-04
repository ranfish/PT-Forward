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
        <template v-if="column.key === 'ruleType'">
          <a-tag :color="record.ruleType === 'accept' ? 'green' : record.ruleType === 'reject' ? 'red' : 'blue'">
            {{ record.ruleType }}
          </a-tag>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="openModal(record)">编辑</a-button>
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
        <a-form-item label="规则名称" name="name" :rules="[{ required: true, message: '请输入规则名称' }]">
          <a-input v-model:value="form.name" placeholder="规则名称" />
        </a-form-item>
        <a-form-item label="规则类型" name="ruleType">
          <a-select v-model:value="form.ruleType" placeholder="选择规则类型">
            <a-select-option value="accept">接受</a-select-option>
            <a-select-option value="reject">拒绝</a-select-option>
            <a-select-option value="accept_and_reject">接受且拒绝</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="条件 (JSON)" name="conditions">
          <a-textarea v-model:value="form.conditions" :rows="3" placeholder='如 [{"field":"title","op":"contains","value":"关键词"}]' />
        </a-form-item>
        <a-form-item label="保存路径" name="savePath">
          <a-input v-model:value="form.savePath" placeholder="留空使用默认" />
        </a-form-item>
        <a-form-item label="分类" name="category">
          <a-input v-model:value="form.category" placeholder="留空使用默认" />
        </a-form-item>
        <a-form-item label="标签" name="tags">
          <a-input v-model:value="form.tags" placeholder="逗号分隔" />
        </a-form-item>
        <a-form-item label="优先级" name="priority">
          <a-input-number v-model:value="form.priority" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="启用" name="enabled">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { filterRulesApi } from '@/api/filter-rules'
import { usePagination } from '@/composables/usePagination'

const modalVisible = ref(false)
const submitting = ref(false)
const editingRule = ref<any>(null)

const form = reactive({
  name: '',
  ruleType: 'reject',
  conditions: '',
  savePath: '',
  category: '',
  tags: '',
  priority: 0,
  enabled: true,
})

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '类型', key: 'ruleType', width: 120 },
  { title: '条件', dataIndex: 'conditions', key: 'conditions', ellipsis: true },
  { title: '保存路径', dataIndex: 'savePath', key: 'savePath', ellipsis: true },
  { title: '分类', dataIndex: 'category', key: 'category', width: 100 },
  { title: '优先级', dataIndex: 'priority', key: 'priority', width: 80 },
  { title: '操作', key: 'actions', width: 120 },
]

const pagination = usePagination((page, size) => filterRulesApi.list(page, size))

function openModal(record?: any) {
  editingRule.value = record || null
  if (record) {
    Object.assign(form, {
      name: record.name || '',
      ruleType: record.ruleType || 'reject',
      conditions: typeof record.conditions === 'string' ? record.conditions : JSON.stringify(record.conditions || []),
      savePath: record.savePath || '',
      category: record.category || '',
      tags: record.tags || '',
      priority: record.priority || 0,
      enabled: record.enabled ?? true,
    })
  } else {
    Object.assign(form, { name: '', ruleType: 'reject', conditions: '', savePath: '', category: '', tags: '', priority: 0, enabled: true })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingRule.value) {
      await filterRulesApi.update(editingRule.value.id, form)
    } else {
      await filterRulesApi.create(form)
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

onMounted(() => pagination.fetch())
</script>
