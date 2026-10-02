import { useMessage } from 'naive-ui'

/**
 * Clipboard access with user feedback baked in. Centralises the
 * try/catch + message pattern repeated across panels, drawers and modals.
 */
export function useClipboard() {
  const message = useMessage()

  /** Write `value` without any toast; resolves to whether it succeeded. */
  async function write(value: string): Promise<boolean> {
    try {
      await navigator.clipboard.writeText(value)
      return true
    } catch {
      return false
    }
  }

  /** Copy `value`, reporting success/failure via toast. */
  async function copy(value: string, successText = 'Value copied'): Promise<void> {
    if (await write(value)) message.success(successText)
    else message.error('Clipboard unavailable')
  }

  /** Read clipboard text; returns '' when unavailable (no toast). */
  async function readSilently(): Promise<string> {
    try {
      return await navigator.clipboard.readText()
    } catch {
      return ''
    }
  }

  return { copy, write, readSilently }
}
