<script setup>
const props = defineProps({
  modelValue: {
    type: String,
    default: 'gateway',
  },
})

const emit = defineEmits(['update:modelValue'])

const items = [
  { key: 'gateway', title: '网关连接', subtitle: '可执行路径与连通状态' },
]

function selectItem(key) {
  emit('update:modelValue', key)
}
</script>

<template>
  <aside class="qq-settings-sidebar">
    <div class="qq-settings-sidebar-head">
      <div class="qq-sidebar-title">设置</div>
      <div class="qq-sidebar-subtitle">项目与连接配置</div>
    </div>
    <div class="qq-settings-nav">
      <button
        v-for="item in items"
        :key="item.key"
        class="qq-settings-nav-item"
        :class="{ 'is-active': modelValue === item.key }"
        @click="selectItem(item.key)"
      >
        <div class="qq-settings-nav-title">{{ item.title }}</div>
        <div class="qq-settings-nav-subtitle">{{ item.subtitle }}</div>
      </button>
    </div>
  </aside>
</template>

