<template>
  <div class="login-wrap">
    <a-card class="login-card">
      <h2 class="title"><i class="fa-solid fa-shield-halved" /> KVM 违规巡查系统</h2>
      <a-form :model="form" layout="vertical" @submit="onSubmit">
        <a-form-item field="username" label="用户名">
          <a-input v-model="form.username" placeholder="admin" />
        </a-form-item>
        <a-form-item field="password" label="密码">
          <a-input-password v-model="form.password" placeholder="请输入密码" />
        </a-form-item>
        <a-button type="primary" long html-type="submit" :loading="loading">登录</a-button>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { login } from '../api'

const router = useRouter()
const loading = ref(false)
const form = reactive({ username: 'admin', password: '' })

async function onSubmit() {
  loading.value = true
  try {
    const { data } = await login(form.username, form.password)
    localStorage.setItem('token', data.token)
    localStorage.setItem('username', data.username)
    router.push('/dashboard')
  } catch (e: any) {
    Message.error(e.response?.data?.error || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  height: 100vh; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #1e3c72 0%, #2a5298 100%);
}
.login-card { width: 380px; }
.title { text-align: center; margin-bottom: 24px; color: #1e3c72; }
.title i { color: #1890ff; }
</style>
