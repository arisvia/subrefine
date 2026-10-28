import { createSignal, Show, type Component, createEffect, onMount, onCleanup } from 'solid-js'
import { Portal } from 'solid-js/web'
import { Button } from '@/components/ui'
import { useI18n, t as translate } from '@/i18n'
import { IconAlertCircle, IconAlertTriangle, IconInfo } from '@/components/ui/icons'

export interface ConfirmOptions {
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  variant?: 'danger' | 'warning' | 'primary'
}

interface DialogState extends ConfirmOptions {
  isOpen: boolean
  isAlert?: boolean
  resolve: (value: boolean) => void
}

const [state, setState] = createSignal<DialogState>({
  isOpen: false,
  message: '',
  resolve: () => {},
})

export function confirm(opts: ConfirmOptions | string): Promise<boolean> {
  const options = typeof opts === 'string' ? { message: opts } : opts
  return new Promise<boolean>(resolve => {
    setState({
      ...options,
      isOpen: true,
      isAlert: false,
      resolve,
    })
  })
}

export function alert(message: string, title?: string): Promise<boolean> {
  return new Promise<boolean>(resolve => {
    setState({
      title: title || translate('confirm.titleInfo'),
      message,
      confirmText: translate('common.ok'),
      variant: 'primary',
      isOpen: true,
      isAlert: true,
      resolve,
    })
  })
}

export const ConfirmDialogHost: Component = () => {
  const s = () => state()
  const { t } = useI18n()
  const title = () => s().title || (s().variant === 'danger' ? t('confirm.titleDanger') : s().variant === 'warning' ? t('confirm.titleWarning') : t('confirm.titleInfo'))
  let prevFocus: HTMLElement | null = null
  let confirmBtnRef: HTMLButtonElement | undefined
  let cancelBtnRef: HTMLButtonElement | undefined

  const handleClose = (res: boolean) => {
    const fn = state().resolve
    setState(prev => ({ ...prev, isOpen: false }))
    fn(res)
    if (prevFocus && typeof prevFocus.focus === 'function') {
      prevFocus.focus()
      prevFocus = null
    }
  }

  createEffect(() => {
    if (s().isOpen) {
      prevFocus = document.activeElement as HTMLElement | null
      const previousOverflow = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      onCleanup(() => { document.body.style.overflow = previousOverflow })
      queueMicrotask(() => {
        if (!s().isAlert && (s().variant === 'danger' || s().variant === 'warning')) {
          cancelBtnRef?.focus()
        } else {
          confirmBtnRef?.focus()
        }
      })
    }
  })

  onMount(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (!state().isOpen) return
      if (e.key === 'Escape') {
        e.preventDefault()
        handleClose(false)
        return
      }
      if (e.key === 'Tab') {
        if (s().isAlert) {
          e.preventDefault()
          confirmBtnRef?.focus()
          return
        }
        if (e.shiftKey) {
          if (document.activeElement === cancelBtnRef) {
            e.preventDefault()
            confirmBtnRef?.focus()
          }
        } else {
          if (document.activeElement === confirmBtnRef) {
            e.preventDefault()
            cancelBtnRef?.focus()
          }
        }
      }
    }
    window.addEventListener('keydown', handleKey)
    onCleanup(() => window.removeEventListener('keydown', handleKey))
  })

  return (
    <Show when={s().isOpen}>
      <Portal>
        <div class="fixed inset-0 z-120 flex items-center justify-center p-4">
        <button
          type="button"
          tabIndex={-1}
          aria-label={t('confirm.closeOverlay')}
          class="absolute inset-0 w-full h-full bg-overlay backdrop-blur-sm animate-fade-in border-none cursor-default"
          onClick={() => handleClose(false)}
        />

        {/* 现代 Liquid Glass 对话框卡片 */}
        <div
          role="alertdialog"
          aria-modal="true"
          aria-label={title()}
          class="relative w-full max-w-md max-h-[calc(100dvh-2rem)] rounded-card glass-float p-5 sm:p-6 flex flex-col animate-scale-in select-none"
        >
          <div class="flex items-start gap-3 sm:gap-4 min-h-0 overflow-y-auto px-1 -mx-1">
            <div
              class={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${
                s().variant === 'danger'
                  ? 'bg-danger/15 text-danger'
                  : s().variant === 'warning'
                    ? 'bg-warning/15 text-warning'
                    : 'bg-accent/15 text-accent'
              }`}
            >
              <Show when={s().variant === 'danger'}>
                <IconAlertCircle size="md" class="w-5 h-5" />
              </Show>
              <Show when={s().variant === 'warning'}>
                <IconAlertTriangle size="md" class="w-5 h-5" />
              </Show>
              <Show when={s().variant !== 'danger' && s().variant !== 'warning'}>
                <IconInfo size="md" class="w-5 h-5" />
              </Show>
            </div>

            <div class="flex-1 min-w-0">
              <h3 class="text-base font-semibold text-text leading-snug break-words [overflow-wrap:anywhere]">
                {title()}
              </h3>
              <p class="text-sm text-muted mt-1.5 leading-relaxed whitespace-pre-wrap break-words [overflow-wrap:anywhere]">
                {s().message}
              </p>
            </div>
          </div>

          <div class="mt-6 flex flex-wrap items-center justify-end gap-2.5 shrink-0">
            <Show when={!s().isAlert}>
              <Button
                ref={cancelBtnRef}
                variant="secondary"
                size="sm"
                onClick={() => handleClose(false)}
              >
                {s().cancelText || t('common.cancel')}
              </Button>
            </Show>
            <Button
              ref={confirmBtnRef}
              variant={s().variant === 'danger' ? 'danger' : 'primary'}
              size="sm"
              onClick={() => handleClose(true)}
            >
              {s().confirmText || (s().variant === 'danger' ? t('common.confirmDelete') : t('common.confirm'))}
            </Button>
          </div>
        </div>
      </div>
      </Portal>
    </Show>
  )
}
