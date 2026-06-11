import { useMessage } from 'naive-ui'

/**
 * Clipboard access with user feedback baked in. Centralises the
 * try/catch + message pattern repeated across panels, drawers and modals.
 */
export function useClipboard() {
  const message = useMessage()

  /** Copy `value`, reporting success/failure via toast. */
  async function copy(value: string, successText = 'Value copied'): Promise<void> {
    try {
      await navigator.clipboard.writeText(value)
      message.success(successText)
    } catch {
      message.error('Clipboard unavailable')
    }
  }

  /** Read clipboard text; returns '' when unavailable (no toast). */
  async function readSilently(): Promise<string> {
    try {
      return await navigator.clipboard.readText()
    } catch {
      return ''
    }
  }

  return { copy, readSilently }
}
