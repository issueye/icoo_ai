<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Check, ChevronDown } from 'lucide-vue-next'

const props = defineProps({
  label: { type: String, required: true },
  modelValue: { type: String, default: '' },
  options: { type: Array, default: () => [] },
  icon: { type: [Object, Function], default: null },
})

const emit = defineEmits(['update:modelValue'])

const root = ref(null)
const listbox = ref(null)
const open = ref(false)
const activeIndex = ref(0)

const selectedOption = computed(() => props.options.find((item) => item.id === props.modelValue) ?? props.options[0])

function setActiveToSelected() {
  const index = props.options.findIndex((item) => item.id === selectedOption.value?.id)
  activeIndex.value = Math.max(index, 0)
}

async function toggle() {
  open.value = !open.value
  if (open.value) {
    setActiveToSelected()
    await nextTick()
    listbox.value?.focus()
  }
}

function close() {
  open.value = false
}

function selectOption(option) {
  if (!option) return
  emit('update:modelValue', option.id)
  close()
}

function moveActive(delta) {
  if (!props.options.length) return
  activeIndex.value = (activeIndex.value + delta + props.options.length) % props.options.length
}

function handleButtonKeydown(event) {
  if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    if (!open.value) toggle()
  }
}

function handleListKeydown(event) {
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
    root.value?.querySelector('button')?.focus()
    return
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    moveActive(1)
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    moveActive(-1)
    return
  }
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    selectOption(props.options[activeIndex.value])
  }
}

function handleDocumentPointerDown(event) {
  if (!root.value?.contains(event.target)) close()
}

onMounted(() => {
  document.addEventListener('pointerdown', handleDocumentPointerDown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handleDocumentPointerDown)
})
</script>

<template>
  <div ref="root" class="qq-context-dropdown" :class="{ 'is-open': open }">
    <button class="qq-context-trigger" type="button" :aria-expanded="open" aria-haspopup="listbox" @click="toggle" @keydown="handleButtonKeydown">
      <component :is="icon" v-if="icon" class="h-3.5 w-3.5" />
      <span class="qq-context-label">{{ label }}</span>
      <span class="qq-context-value">{{ selectedOption?.label }}</span>
      <ChevronDown class="qq-context-chevron h-3.5 w-3.5" />
    </button>
    <div v-if="open" ref="listbox" class="qq-context-menu" role="listbox" tabindex="-1" @keydown="handleListKeydown">
      <button
        v-for="(option, index) in options"
        :key="option.id"
        class="qq-context-option"
        :class="{ 'is-selected': option.id === selectedOption?.id, 'is-active': index === activeIndex }"
        type="button"
        role="option"
        :aria-selected="option.id === selectedOption?.id"
        @mouseenter="activeIndex = index"
        @click="selectOption(option)"
      >
        <span class="min-w-0 truncate">{{ option.label }}</span>
        <Check v-if="option.id === selectedOption?.id" class="h-3.5 w-3.5" />
      </button>
    </div>
  </div>
</template>
