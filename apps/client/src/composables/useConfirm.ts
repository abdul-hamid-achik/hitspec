import { ref } from 'vue'

export interface ConfirmOptions {
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'default' | 'destructive'
}

const visible = ref(false)
const options = ref<ConfirmOptions>({
  title: '',
  message: '',
})
let resolvePromise: ((value: boolean) => void) | null = null

export function useConfirm() {
  function confirm(opts: ConfirmOptions): Promise<boolean> {
    options.value = opts
    visible.value = true
    return new Promise<boolean>((resolve) => {
      resolvePromise = resolve
    })
  }

  function handleConfirm() {
    visible.value = false
    resolvePromise?.(true)
    resolvePromise = null
  }

  function handleCancel() {
    visible.value = false
    resolvePromise?.(false)
    resolvePromise = null
  }

  return { visible, options, confirm, handleConfirm, handleCancel }
}
