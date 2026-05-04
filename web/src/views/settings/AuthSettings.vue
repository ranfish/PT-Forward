<template>
  <div>
    <a-card title="修改密码" style="max-width: 500px">
      <a-form :model="form" @finish="handleSubmit" layout="vertical">
        <a-form-item
          label="旧密码"
          name="oldPassword"
          :rules="[{ required: true, message: '请输入旧密码' }]"
        >
          <a-input-password v-model:value="form.oldPassword" placeholder="请输入旧密码" />
        </a-form-item>
        <a-form-item
          label="新密码"
          name="newPassword"
          :rules="[
            { required: true, message: '请输入新密码' },
            { min: 8, message: '密码至少 8 个字符' },
          ]"
        >
          <a-input-password v-model:value="form.newPassword" placeholder="请输入新密码" />
        </a-form-item>
        <a-form-item
          label="确认密码"
          name="confirmPassword"
          :rules="[
            { required: true, message: '请确认新密码' },
            { validator: validateConfirmPassword },
          ]"
        >
          <a-input-password v-model:value="form.confirmPassword" placeholder="请再次输入新密码" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" :loading="submitting">
            修改密码
          </a-button>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { message } from 'ant-design-vue'
import type { Rule } from 'ant-design-vue/es/form'
import { authApi } from '@/api/auth'

const submitting = ref(false)

const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const validateConfirmPassword = async (_rule: Rule, value: string) => {
  if (value && value !== form.newPassword) {
    throw new Error('两次输入的密码不一致')
  }
}

async function handleSubmit() {
  submitting.value = true
  try {
    await authApi.changePassword(form.oldPassword, form.newPassword)
    message.success('密码修改成功')
    Object.assign(form, { oldPassword: '', newPassword: '', confirmPassword: '' })
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}
</script>
