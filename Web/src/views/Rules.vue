<template>
  <a-card title="违规规则">
    <template #extra>
      <a-button type="primary" @click="openAdd"><i class="fa-solid fa-plus" /> 新建规则</a-button>
    </template>
    <a-table :data="rows" :pagination="false" row-key="id">
      <template #columns>
        <a-table-column title="名称" data-index="name" />
        <a-table-column title="类型" data-index="type" />
        <a-table-column title="匹配条件" data-index="pattern" :ellipsis="true" />
        <a-table-column title="启用">
          <template #cell="{ record }">
            <a-switch :model-value="record.enabled" @change="(v) => onToggle(record, v)" />
          </template>
        </a-table-column>
        <a-table-column title="操作">
          <template #cell="{ record }">
            <a-button type="text" @click="openEdit(record)"><i class="fa-solid fa-pen" /></a-button>
            <a-popconfirm content="确认删除？" @ok="onDelete(record.id)">
              <a-button type="text" status="danger"><i class="fa-solid fa-trash" /></a-button>
            </a-popconfirm>
          </template>
        </a-table-column>
      </template>
    </a-table>

    <a-modal v-model:visible="visible" :title="editing.id ? '编辑规则' : '新建规则'" @ok="onSave" :ok-loading="saving">
      <a-form :model="editing" layout="vertical">
        <a-form-item field="name" label="规则名称"><a-input v-model="editing.name" /></a-form-item>
        <a-form-item field="type" label="类型">
          <a-select v-model="editing.type">
            <a-option value="blacklist_ip">黑名单IP</a-option>
            <a-option value="blacklist_domain">黑名单域名</a-option>
            <a-option value="port">违规端口</a-option>
            <a-option value="keyword">标题关键词</a-option>
            <a-option value="protocol">代理协议</a-option>
          </a-select>
        </a-form-item>
        <a-form-item field="pattern" label="匹配条件（逗号分隔）"><a-textarea v-model="editing.pattern" /></a-form-item>
        <a-form-item field="enabled" label="启用"><a-switch v-model="editing.enabled" /></a-form-item>
      </a-form>
    </a-modal>
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { createRule, deleteRule, listRules, updateRule } from '../api'

const rows = ref<any[]>([])
const visible = ref(false)
const saving = ref(false)
const editing = reactive<any>({ name: '', type: 'keyword', pattern: '', enabled: true })

async function refresh() {
  const { data } = await listRules()
  rows.value = data.items || []
}
function openAdd() { Object.assign(editing, { id: undefined, name: '', type: 'keyword', pattern: '', enabled: true }); visible.value = true }
function openEdit(r: any) { Object.assign(editing, r); visible.value = true }
async function onSave() {
  saving.value = true
  try {
    if (editing.id) await updateRule(editing.id, editing)
    else await createRule(editing)
    Message.success('已保存')
    visible.value = false
    refresh()
  } catch (e: any) { Message.error(e.response?.data?.error || '保存失败') }
  finally { saving.value = false }
}
async function onToggle(r: any, v: any) {
  await updateRule(r.id, { ...r, enabled: v })
  refresh()
}
async function onDelete(id: number) { await deleteRule(id); Message.success('已删除'); refresh() }
onMounted(refresh)
</script>
