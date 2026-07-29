<template>
  <a-card title="域名/IP 黑名单（联动 iptables）">
    <template #extra>
      <a-button type="primary" @click="openAdd"><i class="fa-solid fa-plus" /> 添加黑名单</a-button>
    </template>
    <a-table :data="rows" :pagination="false" row-key="id">
      <template #columns>
        <a-table-column title="目标" data-index="target" />
        <a-table-column title="类型" data-index="kind" />
        <a-table-column title="动作" data-index="action" />
        <a-table-column title="状态">
          <template #cell="{ record }">
            <a-tag :color="record.status === 'active' ? 'red' : 'gray'">{{ record.status }}</a-tag>
          </template>
        </a-table-column>
        <a-table-column title="创建时间">
          <template #cell="{ record }">{{ fmt(record.created_at) }}</template>
        </a-table-column>
        <a-table-column title="操作">
          <template #cell="{ record }">
            <a-popconfirm :content="`确认删除并解封 ${record.target}？`" @ok="onDelete(record.id)">
              <a-button type="text" status="danger"><i class="fa-solid fa-trash" /></a-button>
            </a-popconfirm>
          </template>
        </a-table-column>
      </template>
    </a-table>

    <a-alert type="info" style="margin-top:12px">
      <i class="fa-solid fa-circle-info" /> 仅 IP 类型会实时下发到 Agent 的 iptables 进行封禁；域名类型需 Agent 解析为 IP 后封禁。
    </a-alert>

    <a-modal v-model:visible="visible" title="添加黑名单" @ok="onAdd" :ok-loading="saving">
      <a-form :model="form" layout="vertical">
        <a-form-item field="kind" label="类型">
          <a-radio-group v-model="form.kind">
            <a-radio value="ip">IP</a-radio>
            <a-radio value="domain">域名</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item field="target" label="目标">
          <a-input v-model="form.target" :placeholder="form.kind === 'ip' ? '1.2.3.4 或 CIDR' : '*.example.com'" />
        </a-form-item>
        <a-form-item field="action" label="动作">
          <a-select v-model="form.action">
            <a-option value="drop">DROP</a-option>
            <a-option value="reject">REJECT</a-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { addBlacklist, deleteBlacklist, listBlacklist } from '../api'

const rows = ref<any[]>([])
const visible = ref(false)
const saving = ref(false)
const form = reactive({ kind: 'ip', target: '', action: 'drop' })

async function refresh() {
  const { data } = await listBlacklist()
  rows.value = data.items || []
}
function openAdd() { Object.assign(form, { kind: 'ip', target: '', action: 'drop' }); visible.value = true }
async function onAdd() {
  saving.value = true
  try {
    await addBlacklist({ ...form })
    Message.success('已添加并下发封禁')
    visible.value = false
    refresh()
  } catch (e: any) { Message.error(e.response?.data?.error || '添加失败') }
  finally { saving.value = false }
}
async function onDelete(id: number) { await deleteBlacklist(id); Message.success('已解封'); refresh() }
function fmt(t: string) { return new Date(t).toLocaleString() }
onMounted(refresh)
</script>
