<template>
  <a-row :gutter="16">
    <a-col :span="6" v-for="c in cards" :key="c.label">
      <a-card>
        <a-statistic :title="c.label" :value="c.value">
          <template #prefix><i :class="c.icon" :style="{ color: c.color }" /></template>
        </a-statistic>
      </a-card>
    </a-col>
  </a-row>

  <a-card title="实时违规告警" style="margin-top:16px">
    <template #extra><a-tag color="green"><i class="fa-solid fa-circle" style="font-size:8px" /> 实时</a-tag></template>
    <a-list v-if="alerts.length" :data="alerts" :bordered="false">
      <a-list-item v-for="(a, i) in alerts" :key="i">
        <a-list-item-meta :title="a.violation_type + ' · ' + (a.domain || a.dst_ip)" :description="a.violation_detail">
          <template #avatar><i class="fa-solid fa-triangle-exclamation" style="color:#f53f3f" /></template>
        </a-list-item-meta>
        <template #actions><span class="ts">{{ fmt(a.timestamp) }}</span></template>
      </a-list-item>
    </a-list>
    <a-empty v-else description="暂无实时告警" />
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { dashboard } from '../api'

const cards = ref([
  { label: '节点总数', value: 0, icon: 'fa-solid fa-server', color: '#1890ff' },
  { label: '在线节点', value: 0, icon: 'fa-solid fa-circle-check', color: '#00b42a' },
  { label: '违规总数', value: 0, icon: 'fa-solid fa-triangle-exclamation', color: '#f53f3f' },
  { label: '24h 违规', value: 0, icon: 'fa-solid fa-clock', color: '#ff7d00' },
])
const alerts = ref<any[]>([])
let ws: WebSocket | null = null
let timer: any

async function refresh() {
  const { data } = await dashboard()
  cards.value[0].value = data.node_total
  cards.value[1].value = data.node_online
  cards.value[2].value = data.vio_total
  cards.value[3].value = data.vio_24h
}

function connectWS() {
  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(token)}`)
  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'event' && msg.payload?.is_violation) {
        alerts.value.unshift(msg.payload)
        if (alerts.value.length > 50) alerts.value.pop()
      }
    } catch {}
  }
  ws.onclose = () => { setTimeout(connectWS, 3000) }
}

function fmt(t: string) { return new Date(t).toLocaleTimeString() }

onMounted(() => { refresh(); connectWS(); timer = setInterval(refresh, 10000) })
onUnmounted(() => { if (ws) ws.close(); clearInterval(timer) })
</script>

<style scoped>
.ts { color: #999; font-size: 12px; }
</style>
