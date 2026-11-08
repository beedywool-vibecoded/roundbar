// App-wide snackbar and confirmation dialog, driven from any view.
import { reactive } from 'vue'
import { ApiError } from './api'

export const snackbar = reactive({ text: '', error: false, id: 0 })

export function toast(text: string, error = false) {
  snackbar.text = text
  snackbar.error = error
  snackbar.id++
}

export function toastError(e: unknown) {
  toast(e instanceof ApiError || e instanceof Error ? e.message : String(e), true)
}

interface ConfirmRequest {
  title: string
  body: string
  action: string
  danger?: boolean
}

export const confirmState = reactive<{ open: boolean; req: ConfirmRequest | null; resolve: ((v: boolean) => void) | null }>({
  open: false,
  req: null,
  resolve: null,
})

export function confirm(req: ConfirmRequest): Promise<boolean> {
  return new Promise((resolve) => {
    confirmState.req = req
    confirmState.resolve = resolve
    confirmState.open = true
  })
}

export function settleConfirm(v: boolean) {
  confirmState.resolve?.(v)
  confirmState.open = false
  confirmState.resolve = null
}

// Runs an action with a pending flag, a toast on success and on failure.
export async function run(pending: { value: boolean }, fn: () => Promise<unknown>, done?: string) {
  pending.value = true
  try {
    await fn()
    if (done) toast(done)
  } catch (e) {
    toastError(e)
  } finally {
    pending.value = false
  }
}
