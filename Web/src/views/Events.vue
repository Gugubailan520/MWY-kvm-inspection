<template>
  <a-card title="事件日志">
    <a-form layout="inline" :model="q" style="margin-bottom:16px">
      <a-form-item field="node_id"><a-input v-model="q.node_id" placeholder="节点ID" allow-clear style="width:160px" /></a-form-item>
      <a-form-item field="dst_ip"><a-input v-model="q.dst_ip" placeholder="目的IP" allow-clear style="width:140px" /></a-form-item>
      <a-form-item field="domain"><a-input v-model="q.domain" placeholder="域名" allow-clear style="width:180px" /></a-form-item>
      <a-form-item field="is_violation">
        <a-select v-model="q.is_violation" placeholder="违规" allow-clear style="width:120px">
          <a-option value="true">仅违规</a-option>
          <a-option value="false">仅正常</a-option>
        </a-select>
      </a-form-item>
      <a-form-item><a-button type="primary" @click="onSearch"><i class="fa-solid fa-magnifying-glass" /> 查询</a-button></a-form-item>
      <a-form-item><a-button @click="onReset"><i class="fa-solid fa-rotate" /> 重置</a-button></a-form-item>
    </a-form>

    <a-table :data="rows" :pagination="{ total, current: page, pageSize, showTotal: true }" @page-change="onPage" row-key="timestamp">
      <template #columns>
        <a-table-column title="时间">
          <template #cell="{ record }">{{ fmt(record.timestamp) }}</template>
        </a-table-column>
        <a-table-column title="节点" data-index="node_id" :width="120" />
        <a-table-column title="源IP:端口">
          <template #cell="{ record }">{{ record.src_ip }}:{{ record.src_port }}</template>
        </a-table-column>
        <a-table-column title="目的IP:端口">
          <template #cell="{ record }">{{ record.dst_ip }}:{{ record.dst_port }}</template>
        </a-table-column>
        <a-table-column title="协议" data-index="protocol" :width="70" />
        <a-table-column title="方向" data-index="direction" :width="80" />
        <a-table-column title="域名" data-index="domain" />
        <a-table-column title="标题" data-index="title" :ellipsis="true" />
        <a-table-column title="识别协议" data-index="detected_protocol" />
        <a-table-column title="违规">
          <template #cell="{ record }">
            <a-tag v-if="record.is_violation" color="red">{{ record.violation_type }}</a-tag>
            <span v-else>-</span>
          </template>
        </a-table-column>
      </template>
    </a-table>
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { listEvents } from '../api'

const rows = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const q = reactive({ node_id: '', dst_ip: '', domain: '', is_violation: '' })

async function refresh() {
  const { data } = await listEvents({ ...q, skip: (page.value - 1) * pageSize.value, limit: pageSize.value })
  rows.value = data.items || []
  total.value = data.total || 0
}
function onSearch() { page.value = 1; refresh() }
function onReset() { Object.assign(q, { node_id: '', dst_ip: '', domain: '', is_violation: '' }); onSearch() }
function onPage(p: number) { page.value = p; refresh() }
function fmt(t: string) { return new Date(t).toLocaleString() }
onMounted(refresh)
</script>
