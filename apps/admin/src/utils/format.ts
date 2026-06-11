import type { ValueFormat } from '@/api/types'

/** Locale-formatted date-time; falls back to the raw value, '—' when empty. */
export function formatDate(value: string | undefined): string {
  if (!value) return '—'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}

/** Narrow an arbitrary server value to a supported editor format. */
export function normalizeValueFormat(value: string | undefined): ValueFormat {
  return value === 'yaml' || value === 'json' ? value : 'text'
}

/**
 * Strip the leading indent shared by all non-blank lines (dedent).
 *
 * YAML pasted from a nested section of another file keeps the original
 * indentation; the common prefix is computed literally (spaces/tabs), so
 * relative indentation inside the document is preserved. Whitespace-only
 * lines are ignored when computing the prefix and emptied in the output.
 */
export function stripCommonIndent(value: string): string {
  const lines = value.split('\n')

  let prefix: string | null = null
  for (const line of lines) {
    if (!line.trim()) continue
    const indent = line.slice(0, line.length - line.trimStart().length)
    if (prefix === null) {
      prefix = indent
    } else {
      let common = 0
      while (
        common < prefix.length &&
        common < indent.length &&
        prefix[common] === indent[common]
      ) {
        common++
      }
      prefix = prefix.slice(0, common)
    }
    if (prefix === '') return value
  }
  if (!prefix) return value

  const cut = prefix.length
  return lines
    .map((line) => (line.startsWith(prefix) ? line.slice(cut) : line.trimEnd()))
    .join('\n')
}
