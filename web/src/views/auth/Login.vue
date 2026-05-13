<template>
  <div class="login-page">
    <a-card class="login-card" :bordered="false">
      <div class="login-header">
        <h1>PT-Forward</h1>
        <p>{{ isSetupMode ? t('auth.initialSetup') : t('auth.loginTitle') }}</p>
      </div>

      <a-form :model="form" layout="vertical" @finish="handleSubmit">
        <a-form-item
          name="username"
          :rules="[{ required: true, message: t('auth.usernameRequired') }]"
        >
          <a-input
            v-model:value="form.username"
            size="large"
            :placeholder="t('auth.username')"
          >
            <template #prefix><UserOutlined /></template>
          </a-input>
        </a-form-item>
        <a-form-item
          name="password"
          :rules="[{ required: true, message: t('auth.passwordRequired') }]"
        >
          <a-input-password
            v-model:value="form.password"
            size="large"
            :placeholder="t('auth.password')"
          >
            <template #prefix><LockOutlined /></template>
          </a-input-password>
        </a-form-item>
        <a-form-item v-if="isSetupMode">
          <a-input-password
            v-model:value="form.confirmPassword"
            size="large"
            :placeholder="t('auth.confirmPassword')"
          >
            <template #prefix><LockOutlined /></template>
          </a-input-password>
        </a-form-item>
        <a-form-item>
          <a-button
            type="primary"
            html-type="submit"
            size="large"
            :loading="loading"
            block
          >
            {{ isSetupMode ? t('auth.createAdmin') : t('auth.login') }}
          </a-button>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { UserOutlined, LockOutlined } from '@ant-design/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const loading = ref(false)
const checking = ref(true)
const isSetupMode = ref(false)
const form = reactive({ username: '', password: '', confirmPassword: '' })

onMounted(async () => {
  try {
    const resp = await authApi.status()
    const data = resp.data?.data
    if (data && !data.initialized) {
      isSetupMode.value = true
    }
  } catch {
    // status check failed, assume login mode
  } finally {
    checking.value = false
  }
})

async function handleSubmit() {
  if (isSetupMode.value) {
    await handleSetup()
  } else {
    await handleLogin()
  }
}

async function handleSetup() {
  if (form.password !== form.confirmPassword) {
    message.error(t('auth.passwordMismatch'))
    return
  }
  loading.value = true
  try {
    await authApi.setup({ username: form.username, password: form.password })
    message.success(t('auth.setupSuccess'))
    await authStore.login(form.username, form.password)
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } catch (e: unknown) {
    message.error((e as Error).message || t('auth.setupFailed'))
  } finally {
    loading.value = false
  }
}

async function handleLogin() {
  loading.value = true
  try {
    await authStore.login(form.username, form.password)
    message.success(t('auth.loginSuccess'))
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } catch (e: unknown) {
    message.error((e as Error).message || t('auth.wrongCredentials'))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
}

.login-card {
  width: 400px;
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.login-header h1 {
  font-size: 28px;
  font-weight: 700;
  margin: 0 0 8px;
}

.login-header p {
  color: #888;
  margin: 0;
}
</style>
