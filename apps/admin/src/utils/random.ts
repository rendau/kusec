/** Helpers for generating cryptographically-random secret values. */

/**
 * Сгенерировать случайную hex-строку из `bytes` случайных байт
 * (длина строки — `bytes * 2` символов). Использует `crypto.getRandomValues`.
 */
export function randomHex(bytes = 16): string {
  const buf = new Uint8Array(bytes)
  crypto.getRandomValues(buf)
  let hex = ''
  for (const b of buf) {
    hex += b.toString(16).padStart(2, '0')
  }
  return hex
}
