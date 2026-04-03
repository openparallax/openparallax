<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import { useRoute } from 'vitepress'
import { computed } from 'vue'

const route = useRoute()

const sectionClass = computed(() => {
  const path = route.path
  if (path.startsWith('/shield')) return 'section-shield'
  if (path.startsWith('/memory')) return 'section-memory'
  if (path.startsWith('/audit')) return 'section-audit'
  if (path.startsWith('/sandbox')) return 'section-sandbox'
  if (path.startsWith('/channels')) return 'section-channels'
  if (path.startsWith('/modules/chronicle')) return 'section-chronicle'
  if (path.startsWith('/modules/llm')) return 'section-llm'
  return ''
})
</script>

<template>
  <div :class="sectionClass">
    <DefaultTheme.Layout />
  </div>
</template>
