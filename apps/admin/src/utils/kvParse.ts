/**
 * Parser for key/value text pasted from the clipboard (bulk item import).
 *
 * Supported formats:
 *   - one pair per line:        `key: value` or `key=value`
 *   - single line, comma:       `key: value, key: value`
 *   - single line, semicolon:   `key=value;key=value`
 *   - .env style:               `export KEY="value"`, `#`/`//` comments are skipped
 *   - tab-separated:            `key<TAB>value` (paste from a spreadsheet)
 *   - JSON object:              `{"key": "value", ...}` (non-string values are
 *                               serialized back to JSON)
 *
 * Values may be wrapped in single or double quotes — quotes are stripped.
 * Duplicate keys are collapsed, the last occurrence wins.
 */

export interface ParsedPair {
  key: string
  value: string
}

export interface ParseResult {
  pairs: ParsedPair[]
  /** Raw fragments that could not be parsed into a pair. */
  invalid: string[]
  /** Keys that appeared more than once (the last value won). */
  duplicates: string[]
}

const COMMENT_RE = /^(#|\/\/)/
const EXPORT_RE = /^export\s+/

function stripQuotes(s: string): string {
  if (
    s.length >= 2 &&
    ((s.startsWith('"') && s.endsWith('"')) || (s.startsWith("'") && s.endsWith("'")))
  ) {
    return s.slice(1, -1)
  }
  return s
}

/** Split one fragment at the earliest of `=`, `:` or TAB. */
function splitPair(fragment: string): ParsedPair | null {
  const candidates = [
    fragment.indexOf('='),
    fragment.indexOf(':'),
    fragment.indexOf('\t'),
  ].filter((i) => i > 0)
  if (!candidates.length) return null
  const sep = Math.min(...candidates)

  const key = stripQuotes(fragment.slice(0, sep).trim())
  const value = stripQuotes(fragment.slice(sep + 1).trim())
  if (!key) return null
  return { key, value }
}

/**
 * Split a line into pair fragments. A delimiter (`;`, then `,`) is honored
 * only when every resulting fragment still parses as a pair — so values that
 * merely contain a comma are not torn apart.
 */
function lineFragments(line: string): string[] {
  for (const delimiter of [';', ',']) {
    if (!line.includes(delimiter)) continue
    const parts = line
      .split(delimiter)
      .map((p) => p.trim())
      .filter(Boolean)
    if (parts.length && parts.every((p) => splitPair(p) !== null)) return parts
  }
  return [line]
}

/** Parse the whole input as a flat JSON object, or return null. */
function tryParseJsonObject(text: string): ParsedPair[] | null {
  if (!text.startsWith('{')) return null
  let obj: unknown
  try {
    obj = JSON.parse(text)
  } catch {
    return null
  }
  if (obj === null || typeof obj !== 'object' || Array.isArray(obj)) return null
  return Object.entries(obj).map(([key, v]) => ({
    key,
    value: typeof v === 'string' ? v : JSON.stringify(v),
  }))
}

export function parseKeyValueText(input: string): ParseResult {
  const result: ParseResult = { pairs: [], invalid: [], duplicates: [] }
  const text = input.replace(/^\uFEFF/, '').trim()
  if (!text) return result

  const raw: ParsedPair[] = []
  const fromJson = tryParseJsonObject(text)
  if (fromJson) {
    raw.push(...fromJson)
  } else {
    for (const rawLine of text.split(/\r?\n/)) {
      const line = rawLine.trim()
      if (!line || COMMENT_RE.test(line)) continue
      for (const fragment of lineFragments(line)) {
        const pair = splitPair(fragment.replace(EXPORT_RE, ''))
        if (pair) raw.push(pair)
        else result.invalid.push(fragment)
      }
    }
  }

  const byKey = new Map<string, string>()
  for (const pair of raw) {
    if (byKey.has(pair.key) && !result.duplicates.includes(pair.key)) {
      result.duplicates.push(pair.key)
    }
    byKey.set(pair.key, pair.value)
  }
  result.pairs = [...byKey].map(([key, value]) => ({ key, value }))
  return result
}
