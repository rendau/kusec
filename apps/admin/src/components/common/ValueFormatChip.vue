<script setup lang="ts">
import { computed } from 'vue'
import { useThemeVars } from 'naive-ui'

/**
 * Tiny "TEXT / YAML / JSON" badge pinned to the top-right corner of a value
 * viewer. Colored from the active naive-ui theme (follows primaryColor).
 */
const props = defineProps<{ format?: string }>()

const themeVars = useThemeVars()
const style = computed(() => ({
  color: themeVars.value.primaryColor,
  // 0x24 alpha suffix on the theme's #RRGGBB primary color.
  background: `${themeVars.value.primaryColor}24`,
}))

const label = computed(() => props.format || 'text')
</script>

<template>
  <span class="value-format-chip" :style="style">{{ label }}</span>
</template>

<style scoped>
.value-format-chip {
  position: absolute;
  top: 6px;
  right: 8px;
  z-index: 1;
  padding: 2px 7px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1.4;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  border-radius: 999px;
  pointer-events: none;
}
</style>
