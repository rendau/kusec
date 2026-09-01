import type { Ref } from 'vue'

/**
 * Reveal a long list of expanded-row keys a few rows per animation frame, so
 * mounting hundreds of item panels doesn't block the main thread in a single
 * task ("Expand all" on a big app froze the tab in Firefox).
 */
export function useChunkedExpand(expandedKeys: Ref<string[]>, chunkSize = 3) {
  let seq = 0

  /** Abort an in-flight progressive expand (collapse, app switch, …). */
  function cancel(): void {
    seq++
  }

  async function expandAll(ids: string[]): Promise<void> {
    const run = ++seq
    for (let i = 0; i < ids.length; i += chunkSize) {
      if (run !== seq) return
      expandedKeys.value = ids.slice(0, i + chunkSize)
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    }
  }

  return { expandAll, cancel }
}
