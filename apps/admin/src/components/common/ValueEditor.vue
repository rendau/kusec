<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { EditorView, minimalSetup } from 'codemirror'
import { Compartment, EditorState } from '@codemirror/state'
import { yaml } from '@codemirror/lang-yaml'
import { json, jsonParseLinter } from '@codemirror/lang-json'
import { lintGutter, linter } from '@codemirror/lint'
import type { Diagnostic } from '@codemirror/lint'
import { oneDark } from '@codemirror/theme-one-dark'
import { parseDocument } from 'yaml'
import { useOsTheme, useThemeVars } from 'naive-ui'

const props = withDefaults(
  defineProps<{
    value: string
    /** Editing mode: plain text, YAML or JSON (with highlighting + lint). */
    format?: 'text' | 'yaml' | 'json'
    minHeight?: string
    maxHeight?: string
    readonly?: boolean
  }>(),
  { format: 'text', minHeight: '180px', maxHeight: '420px', readonly: false },
)

const emit = defineEmits<{ 'update:value': [value: string] }>()

const osTheme = useOsTheme()
const themeVars = useThemeVars()

const container = ref<HTMLElement | null>(null)
let view: EditorView | null = null

const languageComp = new Compartment()
const themeComp = new Compartment()

/** YAML validity check surfaced as inline diagnostics. */
function yamlLinter() {
  return linter((v) => {
    const diagnostics: Diagnostic[] = []
    const text = v.state.doc.toString()
    if (!text.trim()) return diagnostics
    const docLen = text.length
    for (const err of parseDocument(text).errors) {
      const from = Math.min(err.pos?.[0] ?? 0, docLen)
      const to = Math.min(Math.max(err.pos?.[1] ?? from + 1, from + 1), docLen)
      diagnostics.push({ from, to, severity: 'error', message: err.message })
    }
    return diagnostics
  })
}

function languageExtensions(format: 'text' | 'yaml' | 'json') {
  if (format === 'yaml') return [yaml(), lintGutter(), yamlLinter()]
  if (format === 'json') return [json(), lintGutter(), linter(jsonParseLinter())]
  return []
}

function themeExtensions() {
  return osTheme.value === 'dark' ? [oneDark] : []
}

onMounted(() => {
  if (!container.value) return
  const state = EditorState.create({
    doc: props.value ?? '',
    extensions: [
      minimalSetup,
      EditorView.lineWrapping,
      EditorState.readOnly.of(props.readonly),
      EditorView.editable.of(!props.readonly),
      languageComp.of(languageExtensions(props.format)),
      themeComp.of(themeExtensions()),
      EditorView.theme({
        '&': {
          fontSize: '13px',
          border: '1px solid rgba(128, 128, 128, 0.28)',
          borderRadius: '3px',
          maxHeight: props.maxHeight,
        },
        // Focus border color follows the naive-ui theme (see scoped CSS).
        '&.cm-focused': { outline: 'none' },
        '.cm-content': {
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
        },
        '.cm-scroller': {
          minHeight: props.minHeight,
          overflow: 'auto',
        },
      }),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) emit('update:value', u.state.doc.toString())
      }),
    ],
  })
  view = new EditorView({ state, parent: container.value })
})

onBeforeUnmount(() => {
  view?.destroy()
  view = null
})

// External value change (form seeding) → sync editor content.
watch(
  () => props.value,
  (val) => {
    if (!view) return
    const current = view.state.doc.toString()
    if ((val ?? '') !== current) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: val ?? '' },
      })
    }
  },
)

// Format change → swap language/lint.
watch(
  () => props.format,
  (fmt) => {
    view?.dispatch({ effects: languageComp.reconfigure(languageExtensions(fmt)) })
  },
)

// OS theme change → swap editor theme.
watch(osTheme, () => {
  view?.dispatch({ effects: themeComp.reconfigure(themeExtensions()) })
})
</script>

<template>
  <div ref="container" class="value-editor" />
</template>

<style scoped>
.value-editor {
  width: 100%;
}

/* Focus border follows the active naive-ui theme. */
.value-editor :deep(.cm-editor.cm-focused) {
  border-color: v-bind('themeVars.primaryColor');
}
</style>
