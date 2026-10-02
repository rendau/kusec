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

/**
 * Случайное целое в диапазоне `[0, max)` без смещения: значения, попавшие в
 * неполный «хвост» диапазона uint32, отбрасываются (rejection sampling), а не
 * сворачиваются через `%`.
 */
export function randomInt(max: number): number {
  const range = 0x1_0000_0000
  const limit = range - (range % max)
  const buf = new Uint32Array(1)
  for (;;) {
    crypto.getRandomValues(buf)
    const value = buf[0]!
    if (value < limit) return value % max
  }
}
