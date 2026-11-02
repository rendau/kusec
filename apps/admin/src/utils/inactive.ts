import type { InactiveKind } from '@/api/types'

const KIND_LABELS: Record<InactiveKind, string> = {
  app: 'Application',
  secret: 'Secret',
  item: 'Secret item',
  configmap: 'Config map',
  config_item: 'Config item',
}

export function inactiveKindLabel(kind: InactiveKind): string {
  return KIND_LABELS[kind]
}

export const inactiveKindOptions = (Object.keys(KIND_LABELS) as InactiveKind[]).map((value) => ({
  label: KIND_LABELS[value],
  value,
}))
