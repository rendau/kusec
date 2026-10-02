import { useMessage } from 'naive-ui'

import type { ValueFormat } from '@/api/types'
import { PrettifyError, prettifyValue } from '@/utils/prettify'

/**
 * Pretty-printing of an item value with user feedback baked in — shared by
 * the item / config item forms and the inline value editor.
 */
export function useValuePrettify() {
  const message = useMessage()

  /**
   * Pretty-print `value` as `format`, reporting the outcome via toast.
   * Returns the formatted value, or `null` when it is not valid `format`.
   */
  function prettify(value: string, format: ValueFormat): string | null {
    try {
      const result = prettifyValue(value, format)
      if (result === value) message.info('Already formatted')
      else message.success(`Formatted as ${format.toUpperCase()}`)
      return result
    } catch (error) {
      message.error(
        error instanceof PrettifyError ? error.message : 'Failed to format the value',
      )
      return null
    }
  }

  return { prettify }
}
