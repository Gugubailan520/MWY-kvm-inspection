<template>
  <div>
    <!-- 统计卡片 -->
    <a-row :gutter="16" style="margin-bottom:16px">
      <a-col :span="4" v-for="c in cards" :key="c.label">
        <a-card>
          <a-statistic :title="c.label" :value="c.value" :value-style="{ fontSize: '20px' }">
            <template #prefix><i :class="c.icon" :style="{ color: c.color }" /></template>
            <template #suffix v-if="c.suffix">{{ c.suffix }}</template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <!-- 接口类型筛选 -->
    <a-card>
      <template #title><i class="fa-solid fa-network-wired" /> 宿主机网络接口</template>
      <template #extra>
        <a-select v-model="nodeId" placeholder="选择节点" style="width:220px" allow-search @change="refreshLatest">
          <a-option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }}</a-option>
        </a-select>
        <a-button status="text" @click="refreshLatest"><i class="fa-solid fa-rotate" /></a-button>
      </template>

      <div style="margin-bottom:12px">
        <a-space wrap>
          <a-tag v-for="t in typeList" :key="t.key" :checkable="true" :checked="showType[t.key]"
                 @check="(v) => toggleType(t.key, v)" :color="showType[t.key] ? typeColor(t.key) : ''">
            <i :class="typeIcon(t.key)" /> {{ t.label }}
          </a-tag>
        </a-space>
      </div>

      <a-table :data="filtered" :pagination="false" row-key="name" :scroll="{ x: 1100 }">
        <template #columns>
          <a-table-column title="接口" data-index="name" :width="140" />
          <a-table-column title="类型" :width="100">
            <template #cell="{ record }">
              <a-tag :color="typeColor(record.type)" size="small">{{ typeLabel(record.type) }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="状态" :width="80">
            <template #cell="{ record }">
              <a-badge :status="record.up ? 'processing' : 'default'" :text="record.up ? 'UP' : 'DOWN'" />
            </template>
          </a-table-column>
          <a-table-column title="MAC" data-index="mac" :width="160" />
          <a-table-column title="IPv4" data-index="ipv4" :width="140" />
          <a-table-column title="速率 (RX/TX)" :width="180">
            <template #cell="{ record }">
              <span style="color:#1890ff">↓{{ fmtSpeed(record.rx_speed) }}</span>
              <span style="color:#f7ba1e;margin-left:12px">↑{{ fmtSpeed(record.tx_speed) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="累计 (RX/TX)" :width="180">
            <template #cell="{ record }">
              <span>{{ fmtBytes(record.rx_bytes) }}</span>
              <span style="margin-left:12px">{{ fmtBytes(record.tx_bytes) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="丢包/错误" :width="110">
            <template #cell="{ record }">
              <span :style="{ color: (record.rx_dropped + record.tx_dropped) > 0 ? '#f53f3f' : '#999' }">
                {{ record.rx_dropped + record.tx_dropped }}/{{ record.rx_errors + record.tx_errors }}
              </span>
            </template>
          </a-table-column>
          <a-table-column title="操作" :width="90" :fixed="'right'">
            <template #cell="{ record }">
              <a-button type="text" @click="openDetail(record)"><i class="fa-solid fa-chart-line" /></a-button>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 接口详情 + 趋势图弹窗 -->
    <a-modal v-model:visible="detailVisible" :title="`接口详情 · ${detail?.name || ''}`" width="780px" :footer="false">
      <div v-if="detail">
        <a-descriptions :column="3" size="small" style="margin-bottom:16px">
          <a-descriptions-item label="类型">{{ typeLabel(detail.type) }}</a-descriptions-item>
          <a-descriptions-item label="MAC">{{ detail.mac || '-' }}</a-descriptions-item>
          <a-descriptions-item label="MTU">{{ detail.mtu || '-' }}</a-descriptions-item>
          <a-descriptions-item label="IPv4">{{ detail.ipv4 || '-' }}</a-descriptions-item>
          <a-descriptions-item label="IPv6">{{ detail.ipv6 || '-' }}</a-descriptions-item>
          <a-descriptions-item label="链路速率">{{ detail.link_speed ? detail.link_speed + ' Mbit/s' : '-' }}</a-descriptions-item>
          <a-descriptions-item label="RX 速率">{{ fmtSpeed(detail.rx_speed) }}</a-descriptions-item>
          <a-descriptions-item label="TX 速率">{{ fmtSpeed(detail.tx_speed) }}</a-descriptions-item>
          <a-descriptions-item label="累计">{{ fmtBytes(detail.rx_bytes + detail.tx_bytes) }}</a-descriptions-item>
        </a-descriptions>

        <div style="margin-bottom:8px;font-weight:600">
          <i class="fa-solid fa-clock-rotate-left" /> 历史流量趋势（近 {{ rangeSec / 60 }} 分钟）
          <a-radio-group v-model="rangeSec" size="mini" style="margin-left:12px" @change="loadHistory">
            <a-radio :value="300">5m</a-radio>
            <a-radio :value="1800">30m</a-radio>
            <a-radio :value="3600">1h</a-radio>
            <a-radio :value="21600">6h</a-radio>
          </a-radio-group>
        </div>
        <canvas ref="chartRef" width="740" height="260" style="width:100%;height:260px"></canvas>
        <a-empty v-if="!history.length && !historyLoading" description="暂无历史数据" style="margin-top:40px" />
      </div>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { ifStatsHistory, ifStatsLatest, listNodes } from '../api'

// 节点列表
const nodes = ref<any[]>([])
const nodeId = ref<string>('')
const stats = ref<any[]>([])

// 类型开关（参考 cockpit-traffic-monitor 的接口类型过滤）
const typeList = [
  { key: 'ethernet', label: '物理网卡' },
  { key: 'bridge', label: '网桥' },
  { key: 'veth', label: 'vEth' },
  { key: 'tap', label: 'TAP/TUN' },
  { key: 'firewall', label: '防火墙' },
  { key: 'bond', label: 'Bond' },
  { key: 'vlan', label: 'VLAN' },
  { key: 'virtual', label: '虚拟' },
  { key: 'wireless', label: '无线' },
  { key: 'loopback', label: '回环' },
]
const showType = reactive<Record<string, boolean>>({
  ethernet: true, bridge: true, veth: true, tap: true, firewall: true,
  bond: true, vlan: true, virtual: true, wireless: true, loopback: false,
})
function toggleType(k: string, v: any) { showType[k] = v }

const filtered = computed(() => stats.value.filter((s) => showType[s.type]))

// 统计卡片
const cards = computed(() => {
  const list = filtered.value
  const up = list.filter((s) => s.up).length
  const tx = list.reduce((a, b) => a + (b.tx_bytes || 0), 0)
  const rx = list.reduce((a, b) => a + (b.rx_bytes || 0), 0)
  const spd = list.reduce((a, b) => a + (b.rx_speed || 0) + (b.tx_speed || 0), 0)
  return [
    { label: '接口总数', value: list.length, icon: 'fa-solid fa-server', color: '#1890ff', suffix: '' },
    { label: '活跃接口', value: up, icon: 'fa-solid fa-circle-check', color: '#00b42a', suffix: '' },
    { label: '总发送', value: tx, icon: 'fa-solid fa-arrow-up', color: '#f7ba1e', suffix: 'B' },
    { label: '总接收', value: rx, icon: 'fa-solid fa-arrow-down', color: '#1890ff', suffix: 'B' },
    { label: '当前速率', value: spd, icon: 'fa-solid fa-bolt', color: '#ff7d00', suffix: 'B/s' },
    { label: '在线节点', value: nodes.value.filter((n) => n.status === 'online').length, icon: 'fa-solid fa-wifi', color: '#722ed1', suffix: '' },
  ]
})

// WebSocket 实时推送
let ws: WebSocket | null = null
function connectWS() {
  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(token)}`)
  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      // 同时接收流量推送与事件推送
      if (msg.type === 'ifstats_push' && msg.payload?.stats) {
        mergeStats(msg.payload.stats)
      }
    } catch {}
  }
  ws.onclose = () => { setTimeout(connectWS, 3000) }
}

// 用最新推送更新当前展示的接口列表（保持累计值与速率实时刷新）
function mergeStats(incoming: any[]) {
  for (const s of incoming) {
    const idx = stats.value.findIndex((x) => x.name === s.name)
    if (idx >= 0) stats.value[idx] = s
    else stats.value.push(s)
  }
}

async function refreshLatest() {
  if (!nodeId.value) return
  const { data } = await ifStatsLatest(nodeId.value)
  stats.value = data.items || []
}

// ---------- 详情弹窗 + 历史趋势图 ----------
const detailVisible = ref(false)
const detail = ref<any>(null)
const rangeSec = ref(1800)
const chartRef = ref<HTMLCanvasElement | null>(null)
const history = ref<any[]>([])
const historyLoading = ref(false)

async function openDetail(record: any) {
  detail.value = record
  detailVisible.value = true
  history.value = []
  await nextTick()
  await loadHistory()
}

async function loadHistory() {
  if (!detail.value) return
  historyLoading.value = true
  try {
    const now = new Date()
    const from = new Date(now.getTime() - rangeSec.value * 1000)
    const { data } = await ifStatsHistory({
      node_id: detail.value.node_id || nodeId.value,
      name: detail.value.name,
      from: from.toISOString(),
      to: now.toISOString(),
      limit: 2000,
    })
    history.value = data.items || []
    drawChart()
  } finally { historyLoading.value = false }
}

watch(rangeSec, loadHistory)

function drawChart() {
  const cv = chartRef.value
  if (!cv || !history.value.length) return
  const ctx = cv.getContext('2d')!
  const W = cv.width, H = cv.height
  ctx.clearRect(0, 0, W, H)
  const pts = history.value
  const maxV = Math.max(1, ...pts.map((p) => p.rx_speed + p.tx_speed))
  const padL = 48, padR = 12, padT = 12, padB = 28
  const plotW = W - padL - padR, plotH = H - padT - padB

  // 网格
  ctx.strokeStyle = '#eee'; ctx.fillStyle = '#999'; ctx.font = '11px sans-serif'
  for (let i = 0; i <= 4; i++) {
    const y = padT + (plotH / 4) * i
    ctx.beginPath(); ctx.moveTo(padL, y); ctx.lineTo(W - padR, y); ctx.stroke()
    ctx.fillText(fmtSpeed(maxV * (1 - i / 4)), 4, y + 4)
  }
  const tMin = new Date(pts[0].timestamp).getTime()
  const tMax = new Date(pts[pts.length - 1].timestamp).getTime()
  const span = Math.max(1, tMax - tMin)
  const X = (t: number) => padL + ((t - tMin) / span) * plotW
  const Y = (v: number) => padT + plotH - (v / maxV) * plotH

  // RX 线（蓝）
  drawSeries(ctx, pts.map((p) => [X(new Date(p.timestamp).getTime()), Y(p.rx_speed)]), '#1890ff', 'rgba(24,144,255,0.08)')
  // TX 线（橙）
  drawSeries(ctx, pts.map((p) => [X(new Date(p.timestamp).getTime()), Y(p.tx_speed)]), '#f7ba1e', 'rgba(247,186,30,0.08)')
}

function drawSeries(ctx: CanvasRenderingContext2D, pts: number[][], color: string, fill: string) {
  if (!pts.length) return
  ctx.strokeStyle = color; ctx.fillStyle = fill; ctx.lineWidth = 1.5
  ctx.beginPath()
  pts.forEach(([x, y], i) => (i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y)))
  ctx.lineTo(pts[pts.length - 1][0], 260 - 28)
  ctx.lineTo(pts[0][0], 260 - 28)
  ctx.closePath(); ctx.fill()
  ctx.beginPath()
  pts.forEach(([x, y], i) => (i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y)))
  ctx.stroke()
}

// ---------- 格式化 ----------
function fmtBytes(b: number): string {
  if (!b) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0; let v = b
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return v.toFixed(i === 0 ? 0 : 1) + ' ' + u[i]
}
function fmtSpeed(bps: number): string {
  return fmtBytes(bps || 0) + '/s'
}
function typeLabel(t: string): string {
  return ({ ethernet: '网卡', bridge: '网桥', veth: 'vEth', tap: 'TAP', firewall: '防火墙', bond: 'Bond', vlan: 'VLAN', virtual: '虚拟', wireless: '无线', loopback: '回环' } as any)[t] || t
}
function typeColor(t: string): string {
  return ({ ethernet: 'blue', bridge: 'green', veth: 'orange', tap: 'purple', firewall: 'red', bond: 'cyan', vlan: 'magenta', virtual: 'gray', wireless: 'pink', loopback: '' } as any)[t] || ''
}
function typeIcon(t: string): string {
  return ({ ethernet: 'fa-solid fa-ethernet', bridge: 'fa-solid fa-bridge-water', veth: 'fa-solid fa-link', tap: 'fa-solid fa-tower-broadcast', firewall: 'fa-solid fa-fire', bond: 'fa-solid fa-layer-group', vlan: 'fa-solid fa-network-wired', virtual: 'fa-solid fa-cube', wireless: 'fa-solid fa-wifi', loopback: 'fa-solid fa-rotate' } as any)[t] || 'fa-solid fa-question'
}

// ---------- 生命周期 ----------
let poller: any
async function init() {
  const { data } = await listNodes()
  nodes.value = data.items || []
  if (nodes.value.length) {
    const online = nodes.value.find((n) => n.status === 'online') || nodes.value[0]
    nodeId.value = online.id
    await refreshLatest()
  }
  connectWS()
  poller = setInterval(refreshLatest, 5000)
}
onMounted(init)
onUnmounted(() => { if (ws) ws.close(); clearInterval(poller) })
</script>

<style scoped>
:deep(.arco-statistic-value) { font-size: 20px; }
</style>
