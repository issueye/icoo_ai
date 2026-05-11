<script setup>
import { computed, onMounted } from 'vue'
import { RefreshCcw, Search, Sparkles } from 'lucide-vue-next'
import { useSkillsStore } from '@/stores/skills'

const skills = useSkillsStore()

const query = computed({
  get: () => skills.query,
  set: (value) => skills.setQuery(value),
})

const filteredItems = computed(() => skills.filteredItems)
const totalCount = computed(() => skills.items.length)

const lastLoadedLabel = computed(() => {
  if (!skills.lastLoadedAt) return '未刷新'
  const date = new Date(skills.lastLoadedAt)
  if (Number.isNaN(date.getTime())) return '未刷新'
  return date.toLocaleString('zh-CN', { hour12: false })
})

onMounted(async () => {
  if (!skills.items.length && !skills.loading) {
    await skills.loadSkills()
  }
})

async function refreshSkills() {
  await skills.loadSkills()
}
</script>

<template>
  <section class="qq-chat-workspace qq-skills-workspace">
    <header class="qq-chat-header qq-skills-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">技能管理</h2>
        <p class="qq-sidebar-subtitle">当前可用技能列表与状态</p>
      </div>
      <button class="qq-icon-button" :disabled="skills.loading" aria-label="刷新技能列表" @click="refreshSkills">
        <RefreshCcw class="h-4 w-4" :class="{ 'qq-skills-spinning': skills.loading }" />
      </button>
    </header>

    <div class="qq-skills-body">
      <div class="qq-skills-toolbar">
        <label class="qq-search-box qq-skills-search text-sm">
          <Search class="h-4 w-4" />
          <input v-model="query" class="w-full bg-transparent outline-none" placeholder="搜索技能名 / ID / 描述" />
        </label>
        <div class="qq-skills-meta">
          <span>总数 {{ totalCount }}</span>
          <span>更新时间 {{ lastLoadedLabel }}</span>
        </div>
      </div>

      <div class="qq-skills-content">
        <div v-if="skills.loading" class="qq-event-card text-sm">技能列表加载中...</div>
        <div v-else-if="skills.error" class="qq-event-card is-warning text-sm">{{ skills.error }}</div>
        <div v-else-if="!filteredItems.length" class="qq-event-card text-sm">
          <Sparkles class="mr-1 inline h-4 w-4" />
          没有匹配到技能
        </div>
        <div v-else class="qq-skills-grid">
          <article v-for="item in filteredItems" :key="item.id" class="qq-skills-card">
            <div class="qq-skills-card-head">
              <h3 class="qq-skills-card-title">{{ item.name || item.id }}</h3>
              <span class="qq-skills-id">{{ item.id }}</span>
            </div>
            <p class="qq-skills-desc">{{ item.description || '暂无描述' }}</p>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>
