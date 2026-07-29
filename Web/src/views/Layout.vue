<template>
  <a-layout class="layout">
    <a-layout-sider :width="220" :collapsed="collapsed" breakpoint="lg" @collapse="collapsed = $event">
      <div class="logo">
        <i class="fa-solid fa-shield-halved" />
        <span v-show="!collapsed">违规巡查系统</span>
      </div>
      <a-menu :selected-keys="[route.name as string]" @menu-item-click="onMenu">
        <a-menu-item key="dashboard"><i class="fa-solid fa-gauge" /> 总览</a-menu-item>
        <a-menu-item key="nodes"><i class="fa-solid fa-server" /> 节点管理</a-menu-item>
        <a-menu-item key="events"><i class="fa-solid fa-list" /> 事件日志</a-menu-item>
        <a-menu-item key="traffic"><i class="fa-solid fa-chart-line" /> 流量监控</a-menu-item>
        <a-menu-item key="rules"><i class="fa-solid fa-gavel" /> 违规规则</a-menu-item>
        <a-menu-item key="blacklist"><i class="fa-solid fa-ban" /> 域名黑名单</a-menu-item>
      </a-menu>
    </a-layout-sider>

    <a-layout>
      <a-layout-header class="header">
        <a-button type="text" @click="collapsed = !collapsed">
          <i class="fa-solid fa-bars" />
        </a-button>
        <div class="header-right">
          <span class="user">{{ user }}</span>
          <a-button type="text" @click="logout"><i class="fa-solid fa-right-from-bracket" /> 退出</a-button>
        </div>
      </a-layout-header>
      <a-layout-content class="content">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const collapsed = ref(false)
const user = computed(() => localStorage.getItem('username') || 'admin')

function onMenu(key: string) {
  router.push({ name: key })
}
function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  router.push('/login')
}
</script>

<style scoped>
.layout { height: 100vh; }
.logo {
  height: 56px; display: flex; align-items: center; justify-content: center;
  color: #fff; gap: 8px; font-size: 16px; background: #001529;
}
.logo i { font-size: 22px; color: #1890ff; }
.header {
  background: #fff; display: flex; align-items: center; justify-content: space-between;
  padding: 0 16px; border-bottom: 1px solid #eee;
}
.header-right { display: flex; align-items: center; gap: 12px; }
.user { color: #555; }
.content { padding: 16px; background: #f0f2f5; overflow: auto; }
:deep(.arco-layout-sider) { background: #001529; }
:deep(.arco-menu) { background: transparent; }
:deep(.arco-menu-item) { color: #b3c0d1; }
:deep(.arco-menu-item.arco-menu-selected) { color: #fff; background: #1890ff; }
</style>
