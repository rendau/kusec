import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /** Document title segment shown in the browser tab. */
    title?: string
  }
}
