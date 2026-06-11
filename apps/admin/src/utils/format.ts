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
