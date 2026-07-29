<template>
  <a-card title="节点管理">
    <template #extra>
      <a-button type="primary" @click="showAdd = true"><i class="fa-solid fa-plus" /> 添加节点</a-button>
    </template>
    <a-table :data="rows" :pagination="false" row-key="id">
      <template #columns>
        <a-table-column title="名称" data-index="name" />
        <a-table-column title="IP" data-index="ip" />
        <a-table-column title="系统" data-index="os" />
        <a-table-column title="虚拟化" data-index="virt" />
        <a-table-column title="API Key">
          <template #cell="{ record }">
            <a-typography-text copyable>{{ record.api_key }}</a-typography-text>
          </template>
        </a-table-column>
        <a-table-column title="状态">
          <template #cell="{ record }">
            <a-badge :status="record.status === 'online' ? 'processing' : 'default'" :text="record.status" />
          </template>
        </a-table-column>
        <a-table-column title="最后心跳">
          <template #cell="{ record }">{{ record.last_heartbeat ? fmt(record.last_heartbeat) : '-' }}</template>
        </a-table-column>
        <a-table-column title="操作">
          <template #cell="{ record }">
            <a-popconfirm content="确认删除该节点？" @ok="onDelete(record.id)">
              <a-button type="text" status="danger"><i class="fa-solid fa-trash" /></a-button>
            </a-popconfirm>
          </template>
        </a-table-column>
      </template>
    </a-table>

    <a-modal v-model:visible="showAdd" title="添加节点" @ok="onAdd" :ok-loading="adding">
      <a-form :model="form" layout="vertical">
        <a-form-item field="name" label="名称"><a-input v-model="form.name" /></a-form-item>
        <a-form-item field="ip" label="IP"><a-input v-model="form.ip" /></a-form-item>
        <a-form-item field="os" label="系统">
          <a-select v-model="form.os">
            <a-option value="CentOS 7.9">CentOS 7.9</a-option>
            <a-option value="CentOS 8">CentOS 8</a-option>
            <a-option value="Debian 12">Debian 12</a-option>
            <a-option value="Debian 13">Debian 13</a-option>
          </a-select>
        </a-form-item>
        <a-form-item field="virt" label="虚拟化平台">
          <a-select v-model="form.virt">
            <a-option value="KVM">KVM (魔方云)</a-option>
            <a-option value="LXC">LXC (PVE)</a-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { createNode, deleteNode, listNodes } from '../api'

const rows = ref<any[]>([])
const showAdd = ref(false)
const adding = ref(false)
const form = reactive({ name: '', ip: '', os: 'Debian 12', virt: 'KVM' })

async function refresh() {
  const { data } = await listNodes()
  rows.value = data.items || []
}
async function onAdd() {
  adding.value = true
  try {
    await createNode({ ...form })
    Message.success('添加成功')
    showAdd.value = false
    refresh()
  } catch (e: any) {
    Message.error(e.response?.data?.error || '添加失败')
  } finally { adding.value = false }
}
async function onDelete(id: string) {
  await deleteNode(id)
  Message.success('已删除')
  refresh()
}
function fmt(t: string) { return new Date(t).toLocaleString() }
onMounted(refresh)
</script>
