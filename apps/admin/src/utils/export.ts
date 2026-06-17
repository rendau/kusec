import { stringify as stringifyYaml } from 'yaml'

/**
 * Serialising a secret/config map as flat key/value pairs for copy/download.
 * Binary (encoding=base64) values are emitted verbatim as their base64 string,
 * mirroring how Kubernetes stores them.
 */

export type ExportFormat = 'env' | 'json' | 'yaml'

export interface ExportPair {
  key: string
  value: string
}

/** Display label + file extension for each format (UI + download name). */
export const EXPORT_FORMATS: Record<
  ExportFormat,
  { label: string; extension: string }
> = {
  env: { label: '.env', extension: 'env' },
  json: { label: 'JSON', extension: 'json' },
  yaml: { label: 'YAML', extension: 'yaml' },
}

/** Quote a dotenv value when it contains characters that break `KEY=value`. */
function dotenvQuote(value: string): string {
  if (value === '') return ''
  if (/[\s"'#\\]|^\s|\s$/.test(value) || value.includes('\n')) {
    const escaped = value
      .replace(/\\/g, '\\\\')
      .replace(/"/g, '\\"')
      .replace(/\n/g, '\\n')
    return `"${escaped}"`
  }
  return value
}

/** Render key/value pairs in the requested format. */
export function formatKeyValues(pairs: ExportPair[], format: ExportFormat): string {
  switch (format) {
    case 'env':
      return pairs.map((p) => `${p.key}=${dotenvQuote(p.value)}`).join('\n')
    case 'json':
      return JSON.stringify(
        Object.fromEntries(pairs.map((p) => [p.key, p.value])),
        null,
        2,
      )
    case 'yaml':
      return stringifyYaml(Object.fromEntries(pairs.map((p) => [p.key, p.value])))
  }
}

/** Trigger a browser download of `text` as `baseName.<ext>`. */
export function downloadText(
  text: string,
  baseName: string,
  format: ExportFormat,
): void {
  const { extension } = EXPORT_FORMATS[format]
  const fileName = format === 'env' ? `${baseName}.env` : `${baseName}.${extension}`
  const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
