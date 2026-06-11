/**
 * Helpers for binary / file item values, which are stored base64-encoded in the
 * text `value` column (mirroring how Kubernetes keeps binary secret data).
 */

/** Read a File into a base64 string (without the data-URL prefix). */
export function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = String(reader.result)
      const comma = result.indexOf(',')
      resolve(comma >= 0 ? result.slice(comma + 1) : result)
    }
    reader.onerror = () => reject(reader.error ?? new Error('Failed to read file'))
    reader.readAsDataURL(file)
  })
}

/** Encode bytes to a base64 string (chunked to avoid call-stack limits). */
export function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return btoa(binary)
}

/** Encode arbitrary text to base64 (UTF-8 safe, unlike bare `btoa`). */
export function textToBase64(text: string): string {
  return bytesToBase64(new TextEncoder().encode(text))
}

/** Decode a base64 string to UTF-8 text. Throws on invalid base64. */
export function base64ToText(b64: string): string {
  return new TextDecoder().decode(base64ToBytes(b64))
}

/** Heuristic: the value looks like a base64 payload (used to offer decoding). */
export function isProbablyBase64(value: string): boolean {
  const clean = value.trim()
  if (clean.length < 8 || clean.length % 4 !== 0) return false
  if (!/^[A-Za-z0-9+/]+={0,2}$/.test(clean)) return false
  try {
    atob(clean)
    return true
  } catch {
    return false
  }
}

/** Heuristic: looks like UTF-8 text (no NUL bytes, decodes cleanly). */
export function isProbablyText(bytes: Uint8Array): boolean {
  const sample = bytes.subarray(0, 8192)
  if (sample.includes(0)) return false
  try {
    new TextDecoder('utf-8', { fatal: true }).decode(sample)
    return true
  } catch {
    return false
  }
}

/** Decode a base64 string to bytes. Throws on invalid input. */
export function base64ToBytes(b64: string): Uint8Array {
  const binary = atob(b64.trim())
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

/** Approximate decoded byte size of a base64 string. */
export function base64ByteSize(b64: string): number {
  const clean = b64.trim()
  if (!clean) return 0
  const padding = clean.endsWith('==') ? 2 : clean.endsWith('=') ? 1 : 0
  return Math.floor((clean.length * 3) / 4) - padding
}

/** Human-readable byte size. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB']
  let value = bytes / 1024
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${value.toFixed(value < 10 ? 1 : 0)} ${units[i]}`
}

/** Trigger a browser download of a base64-encoded value. */
export function downloadBase64(
  b64: string,
  fileName: string,
  contentType?: string,
): void {
  const bytes = base64ToBytes(b64)
  const blob = new Blob([bytes], {
    type: contentType || 'application/octet-stream',
  })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName || 'value'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
