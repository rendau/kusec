import { parseAllDocuments, Scalar, visit } from 'yaml'
import type { Document, ScalarTag } from 'yaml'

import type { ValueFormat } from '@/api/types'

/** Thrown when a value cannot be pretty-printed; `message` is user-facing. */
export class PrettifyError extends Error {}

const JSON_INDENT = '  '

/**
 * Re-indent JSON without re-serialising it: only whitespace between tokens
 * changes. `JSON.stringify(JSON.parse(x))` would silently rewrite numbers past
 * 2^53 (ids) and drop duplicate keys — not acceptable for stored values.
 */
function prettifyJson(value: string): string {
  try {
    JSON.parse(value)
  } catch (error) {
    throw new PrettifyError(`Invalid JSON: ${(error as Error).message}`)
  }

  let out = ''
  let depth = 0
  let inString = false
  const newline = (): void => {
    out += '\n' + JSON_INDENT.repeat(depth)
  }

  for (let i = 0; i < value.length; i++) {
    const ch = value[i]!
    if (inString) {
      out += ch
      if (ch === '\\') out += value[++i]
      else if (ch === '"') inString = false
      continue
    }
    switch (ch) {
      case '"':
        inString = true
        out += ch
        break
      case '{':
      case '[': {
        // Empty object/array stays on one line.
        const next = value.slice(i + 1).search(/[^ \t\n\r]/) + i + 1
        if (value[next] === (ch === '{' ? '}' : ']')) {
          out += ch + value[next]
          i = next
        } else {
          out += ch
          depth++
          newline()
        }
        break
      }
      case '}':
      case ']':
        depth--
        newline()
        out += ch
        break
      case ',':
        out += ch
        newline()
        break
      case ':':
        out += ': '
        break
      case ' ':
      case '\t':
      case '\n':
      case '\r':
        break
      default:
        out += ch
    }
  }
  return value.endsWith('\n') ? out + '\n' : out
}

/** Source text of a plain number, written back verbatim (see `rawNumberTag`). */
class RawNumber {
  readonly text: string

  constructor(text: string) {
    this.text = text
  }
}

/**
 * The yaml library normalises numbers on output: `0644` becomes `644` (an
 * octal file mode for YAML 1.1 readers such as Go's yaml.v2), `0x1F` → `0x1f`,
 * `1e3` → `1e+3`, long floats get truncated. This tag makes plain numbers keep
 * their source text. It is `default`, so no tag prefix is emitted, and has no
 * `test`, so it never takes part in parsing.
 */
const rawNumberTag: ScalarTag = {
  identify: (value) => value instanceof RawNumber,
  default: true,
  tag: 'tag:kusec,2026:raw-number',
  resolve: (source) => source,
  stringify: (item) => (item.value as RawNumber).text,
}

function parseYaml(value: string): Document.Parsed[] {
  const docs = parseAllDocuments(value, {
    intAsBigInt: true,
    customTags: [rawNumberTag],
  })
  for (const doc of docs) {
    const error = doc.errors[0]
    if (error) {
      // The first line is the reason; the rest is a code frame.
      const reason = error.message.split('\n')[0]!.replace(/:$/, '')
      throw new PrettifyError(`Invalid YAML: ${reason}`)
    }
  }
  return [...docs]
}

/** Data the documents resolve to, as a comparable string. */
function yamlFingerprint(docs: Document.Parsed[]): string {
  return JSON.stringify(
    docs.map((doc) => doc.toJS() as unknown),
    (_key, v: unknown) => {
      if (typeof v === 'bigint') return `bigint:${v}`
      if (typeof v === 'number' && !Number.isFinite(v)) return `number:${v}`
      return v
    },
  )
}

/**
 * Re-indent YAML keeping comments, key order, quoting style and number
 * literals. The result is parsed back and compared with the original data, so
 * a formatting quirk can never silently change a stored value.
 */
function prettifyYaml(value: string): string {
  const docs = parseYaml(value)
  if (!docs.length) return value

  let before: string
  try {
    before = yamlFingerprint(docs)
  } catch (error) {
    throw new PrettifyError(`Cannot prettify YAML: ${(error as Error).message}`)
  }

  for (const doc of docs) {
    visit(doc, {
      Scalar(_key, node) {
        const isNumber =
          typeof node.value === 'number' || typeof node.value === 'bigint'
        if (isNumber && node.type === Scalar.PLAIN && !node.tag && node.source) {
          node.value = new RawNumber(node.source)
        }
      },
    })
  }

  // lineWidth 0 — never fold long lines.
  let out = docs.map((doc) => doc.toString({ lineWidth: 0 })).join('')
  if (!value.endsWith('\n')) out = out.replace(/\n$/, '')

  if (yamlFingerprint(parseYaml(out)) !== before) {
    throw new PrettifyError('Prettify would change the data — value left as is')
  }
  return out
}

/**
 * Pretty-print `value` as `format`. Plain text has no structure to format and
 * is returned unchanged. Throws `PrettifyError` when the value is not valid.
 */
export function prettifyValue(value: string, format: ValueFormat): string {
  if (format === 'json') return prettifyJson(value)
  if (format === 'yaml') return prettifyYaml(value)
  return value
}
