<template>
  <div>
    <a-card :title="t('auth.profile')" style="max-width: 500px; margin-bottom: 16px">
      <a-form :model="profileForm" layout="vertical" @finish="handleProfileSubmit">
        <a-form-item :label="t('auth.username')">
          <a-input :value="profile.username" disabled />
        </a-form-item>
        <a-form-item :label="t('auth.displayName')" name="displayName">
          <a-input v-model:value="profileForm.displayName" :placeholder="t('auth.displayNamePlaceholder')" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" :loading="profileSubmitting">
            {{ t('common.save') }}
          </a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card :title="t('auth.changePassword')" style="max-width: 500px">
      <a-form :model="form" layout="vertical" @finish="handleSubmit">
        <a-form-item
          :label="t('auth.oldPassword')"
          name="oldPassword"
          :rules="[{ required: true, message: t('auth.oldPasswordRequired') }]"
        >
          <a-input-password v-model:value="form.oldPassword" :placeholder="t('auth.oldPasswordPlaceholder')" />
        </a-form-item>
        <a-form-item
          :label="t('auth.newPassword')"
          name="newPassword"
          :rules="[
            { required: true, message: t('auth.newPasswordRequired') },
            { min: 8, message: t('auth.passwordMinLength') },
          ]"
        >
          <a-input-password v-model:value="form.newPassword" :placeholder="t('auth.newPasswordPlaceholder')" />
        </a-form-item>
        <a-form-item
          :label="t('auth.confirmPassword')"
          name="confirmPassword"
          :rules="[
            { required: true, message: t('auth.confirmPasswordRequired') },
            { validator: validateConfirmPassword },
          ]"
        >
          <a-input-password v-model:value="form.confirmPassword" :placeholder="t('auth.confirmPasswordPlaceholder')" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" :loading="submitting">
            {{ t('auth.changePassword') }}
          </a-button>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import type { Rule } from 'ant-design-vue/es/form'
import { authApi } from '@/api/auth'

const { t } = useI18n()

const submitting = ref(false)
const profileSubmitting = ref(false)

const profile = reactive({ username: '', displayName: '' })
const profileForm = reactive({ displayName: '' })

const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const validateConfirmPassword = async (_rule: Rule, value: string) => {
  if (value && value !== form.newPassword) {
    throw new Error(t('auth.passwordMismatch'))
  }
}

async function fetchProfile() {
  try {
    const resp = await authApi.getProfile()
    const data = resp.data?.data || resp.data
    profile.username = data?.username || ''
    profile.displayName = data?.displayName || ''
    profileForm.displayName = data?.displayName || ''
  } catch { /* ignore */ }
}

async function handleProfileSubmit() {
  profileSubmitting.value = true
  try {
    await authApi.updateProfile({ displayName: profileForm.displayName })
    profile.displayName = profileForm.displayName
    message.success(t('common.saveSuccess'))
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    profileSubmitting.value = false
  }
}

async function handleSubmit() {
  submitting.value = true
  try {
    await authApi.changePassword(form.oldPassword, form.newPassword)
    message.success(t('auth.passwordChanged'))
    Object.assign(form, { oldPassword: '', newPassword: '', confirmPassword: '' })
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    submitting.value = false
  }
}

onMounted(fetchProfile)
</script>
