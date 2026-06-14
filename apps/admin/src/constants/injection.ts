import type { InjectionKey, Ref } from 'vue'

/**
 * One-shot reveal/hide broadcast from the workspace "Show all / Hide all"
 * buttons to every expanded items panel. Each panel applies the command to
 * its own per-row flags, so rows stay individually togglable afterwards.
 */
export interface RevealCommand {
  action: 'show' | 'hide'
  seq: number
}

export const itemsRevealCommandKey: InjectionKey<Ref<RevealCommand>> =
  Symbol('itemsRevealCommand')

/**
 * Same one-shot reveal/hide broadcast as `itemsRevealCommandKey`, but for the
 * ConfigMaps section — kept separate so revealing a secret's items does not
 * also reveal a config map's, and vice versa.
 */
export const configItemsRevealCommandKey: InjectionKey<Ref<RevealCommand>> =
  Symbol('configItemsRevealCommand')
