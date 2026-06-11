import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import type { FormInst } from 'naive-ui'

import { apiErrorMessage } from '@/api/http'

interface UseEntityFormOptions<E> {
  /** Modal visibility (the `show` prop). */
  show: () => boolean
  /** The entity being edited, or `null` when creating. */
  entity: () => E | null
  /** Seed the form model from the entity (or defaults when creating). */
  seed: (entity: E | null) => void | Promise<void>
  /** Persist a new entity. */
  create: () => Promise<unknown>
  /** Persist changes to an existing entity. */
  update: (entity: E) => Promise<unknown>
  messages: { created: string; updated: string }
  /** Called after a successful save (emit `saved`, close the modal). */
  onSaved: () => void
}

/**
 * Shared lifecycle of an entity create/edit modal: re-seed the form each
 * time it opens, validate + persist on submit, report errors via toast.
 */
export function useEntityForm<E>(options: UseEntityFormOptions<E>) {
  const message = useMessage()

  const formRef = ref<FormInst | null>(null)
  const submitting = ref(false)
  const isEdit = computed(() => options.entity() !== null)

  watch(options.show, async (show) => {
    if (!show) return
    await options.seed(options.entity())
    formRef.value?.restoreValidation()
  })

  async function submit(): Promise<void> {
    try {
      await formRef.value?.validate()
    } catch {
      return
    }

    submitting.value = true
    try {
      const entity = options.entity()
      if (entity !== null) {
        await options.update(entity)
        message.success(options.messages.updated)
      } else {
        await options.create()
        message.success(options.messages.created)
      }
      options.onSaved()
    } catch (error) {
      message.error(apiErrorMessage(error, 'Unexpected error, please try again'))
    } finally {
      submitting.value = false
    }
  }

  return { formRef, submitting, isEdit, submit }
}
