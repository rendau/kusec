import { onMounted, onUnmounted, ref } from 'vue'

/**
 * Reactive viewport breakpoint helper. `isMobile` tracks a `max-width` media
 * query (default 768px) so layouts can switch the sidebar to an overlay drawer
 * and let dense tables scroll horizontally on small screens.
 */
export function useBreakpoint(maxWidth = 768) {
  const isMobile = ref(false)

  let media: MediaQueryList | null = null
  const update = (event: MediaQueryList | MediaQueryListEvent): void => {
    isMobile.value = event.matches
  }

  onMounted(() => {
    media = window.matchMedia(`(max-width: ${maxWidth}px)`)
    update(media)
    media.addEventListener('change', update)
  })

  onUnmounted(() => {
    media?.removeEventListener('change', update)
  })

  return { isMobile }
}
