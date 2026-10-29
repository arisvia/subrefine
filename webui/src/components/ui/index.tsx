// Vendored from cyrene-gateway webui (src/components/ui/index.tsx) — shared
// design-system primitives. Adapt locally only when subrefine truly diverges.
import { type Component, type JSX, For, Show, createSignal, createMemo, createEffect, createUniqueId, on, onMount, onCleanup, splitProps } from 'solid-js'
import { Portal } from 'solid-js/web'
import { useToast, dismiss } from '@/lib/toast'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/lib/format'
import { IconClose, IconCheck, IconChevronDown, IconChevronUp, IconAlertCircle, IconAlertTriangle, IconInfo, IconUpload, IconFile, IconImage } from './icons'
import type { BadgeTone } from '@/types/domain'
export * from './SubrefineLogo'
export * from './icons'
export { ConfirmDialogHost, confirm, alert } from '@/lib/confirm'
export const Card: Component<{
  class?: string
  hover?: boolean
  onClick?: () => void
  children?: JSX.Element
  ref?: HTMLDivElement | ((el: HTMLDivElement) => void)
}> = props => (
  <div
    ref={el => {
      if (typeof props.ref === 'function') props.ref(el)
      else if (props.ref) (props as unknown as { ref: HTMLDivElement }).ref = el
    }}
    class={`rounded-card glass-card ${props.hover ? 'hover:bg-card-hover hover:border-glass-border-hover hover:shadow-glass-hover' : ''} transition-[background-color,border-color,box-shadow] duration-200 ${props.class ?? ''}`}
    onClick={props.onClick}
  >
    {props.children}
  </div>
)
export interface PageHeaderProps {
  title: string | JSX.Element
  subtitle?: string | JSX.Element
  badge?: JSX.Element
  actions?: JSX.Element
  children?: JSX.Element
  class?: string
  sticky?: boolean
}

export const PageHeader: Component<PageHeaderProps> = props => {
  const isSticky = () => props.sticky === true
  return (
    <header class={`${isSticky() ? 'sticky top-19 z-20 rounded-2xl glass-sticky px-5 py-4' : 'relative'} space-y-3 ${props.class ?? ''}`}>
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div class="min-w-0">
          <div class="flex items-center gap-2.5 flex-wrap">
            <h1 class="text-2xl font-semibold tracking-tight text-foreground wrap-break-word">{props.title}</h1>
            {props.badge}
          </div>
          <Show when={props.subtitle}>
            <p class="text-sm text-muted mt-1 wrap-break-word">{props.subtitle}</p>
          </Show>
        </div>
        <Show when={props.actions}>
          <div class="flex items-center gap-2.5 flex-wrap sm:justify-end">
            {props.actions}
          </div>
        </Show>
      </div>
      <Show when={props.children}>
        <div class="pt-0.5">
          {props.children}
        </div>
      </Show>
    </header>
  )
}


export const Badge: Component<{ tone?: BadgeTone; class?: string; children?: JSX.Element }> = props => {
  const tones: Record<BadgeTone, string> = {
    green: 'text-success bg-success/12 border-success/30',
    amber: 'text-warning bg-warning/12 border-warning/30',
    red: 'text-danger bg-danger/12 border-danger/30',
    blue: 'text-info bg-info/12 border-info/30',
    purple: 'text-accent bg-accent/12 border-accent/30',
    violet: 'text-accent bg-accent/12 border-accent/30',
    teal: 'text-accent-2 bg-accent-2/12 border-accent-2/30',
    rose: 'text-danger bg-danger/12 border-danger/30',
    orange: 'text-warning bg-warning/12 border-warning/30',
    indigo: 'text-accent bg-accent/12 border-accent/30',
    cyan: 'text-accent-2 bg-accent-2/12 border-accent-2/30',
    pink: 'text-danger bg-danger/12 border-danger/30',
    gray: 'text-muted bg-surface-inset border-subtle',
  }
  return (
    <span class={`inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium border ${tones[props.tone ?? 'gray']} ${props.class ?? ''}`}>
      {props.children}
    </span>
  )
}

export interface EmptyProps {
  message?: string
  title?: string
  description?: string
  icon?: JSX.Element
  action?: JSX.Element
  class?: string
  children?: JSX.Element
}

export const Empty: Component<EmptyProps> = props => {
  return (
    <Show
      when={props.icon || props.title || props.description || props.action}
      fallback={
        <div class={`py-12 text-center text-sm text-faint ${props.class ?? ''}`}>
          {props.message}
          {props.children}
        </div>
      }
    >
      <div class={`text-center space-y-4 ${props.class ?? ''}`}>
        <Show when={props.icon}>
          <div class="w-12 h-12 rounded-2xl bg-accent/10 text-accent flex items-center justify-center mx-auto shadow-glass">
            {props.icon}
          </div>
        </Show>
        <div class="space-y-1">
          <Show when={props.title || props.message}>
            <h3 class="text-base font-semibold text-foreground">
              {props.title || props.message}
            </h3>
          </Show>
          <Show when={props.description}>
            <p class="text-xs text-faint max-w-md mx-auto leading-relaxed">
              {props.description}
            </p>
          </Show>
        </div>
        <Show when={props.action}>
          <div class="flex items-center justify-center gap-3 pt-2">
            {props.action}
          </div>
        </Show>
        {props.children}
      </div>
    </Show>
  )
}

export const Spinner: Component<{ size?: 'sm' | 'md' | 'lg' }> = props => {
  const sizes = { sm: 'h-3.5 w-3.5', md: 'h-4 w-4', lg: 'h-5 w-5' }
  return (
    <div
      class={`${sizes[props.size ?? 'md']} shrink-0 rounded-full border-2 border-subtle border-t-accent animate-spin`}
      aria-hidden="true"
    />
  )
}
export const StatusPulse: Component<{
  status?: 'active' | 'paused' | 'error' | 'idle'
  tone?: 'green' | 'amber' | 'red' | 'blue' | 'accent' | 'gray'
  size?: 'xs' | 'sm' | 'md'
  class?: string
}> = props => {
  const isPulsing = () => props.status === 'active' || (!props.status && props.tone !== 'gray')
  const tone = () => {
    if (props.tone) return props.tone
    if (props.status === 'active') return 'accent'
    if (props.status === 'error') return 'red'
    if (props.status === 'paused') return 'gray'
    return 'green'
  }

  const dotColors: Record<string, string> = {
    accent: 'bg-accent',
    green: 'bg-success',
    amber: 'bg-warning',
    red: 'bg-danger',
    blue: 'bg-info',
    gray: 'bg-muted/60',
  }

  const pingColors: Record<string, string> = {
    accent: 'bg-accent/75',
    green: 'bg-success/75',
    amber: 'bg-warning/75',
    red: 'bg-danger/75',
    blue: 'bg-info/75',
    gray: 'bg-muted/40',
  }

  const sizeClasses: Record<string, string> = {
    xs: 'w-1.5 h-1.5',
    sm: 'w-2 h-2',
    md: 'w-2.5 h-2.5',
  }

  const sz = () => sizeClasses[props.size ?? 'sm']

  return (
    <span class={`relative inline-flex shrink-0 items-center justify-center self-center ${sz()} ${props.class ?? ''}`}>
      <Show when={isPulsing()}>
        <span class={`absolute inline-flex h-full w-full rounded-full animate-ping opacity-75 ${pingColors[tone()]}`} />
      </Show>
      <span class={`relative inline-flex rounded-full ${sz()} ${dotColors[tone()]}`} />
    </span>
  )
}

export function ToastHost() {
  const { toasts } = useToast()
  const { t } = useI18n()

  const kindStyles = {
    success: {
      card: 'glass-float',
      iconWrap: 'bg-success/15 text-success',
    },
    error: {
      card: 'glass-float',
      iconWrap: 'bg-danger/15 text-danger',
    },
    warning: {
      card: 'glass-float',
      iconWrap: 'bg-warning/15 text-warning',
    },
    info: {
      card: 'glass-float',
      iconWrap: 'bg-accent/15 text-accent',
    },
  }

  return (
    <div class="fixed top-4 right-4 z-130 w-[calc(100%-2rem)] max-w-sm pointer-events-none flex flex-col gap-2.5" role="status" aria-live="polite">
      <For each={toasts()}>
        {toastItem => {
          const style = () => kindStyles[toastItem.kind] || kindStyles.info
          return (
            <div
              class={`pointer-events-auto relative w-full p-3 rounded-xl shadow-glass-hover backdrop-blur-lg transition-all duration-200 animate-slide-up flex items-start gap-3 select-none ${style().card}`}
            >
              <div class={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 mt-0.5 ${style().iconWrap}`}>
                <Show when={toastItem.kind === 'success'}>
                  <IconCheck size="sm" class="w-3.5 h-3.5" />
                </Show>
                <Show when={toastItem.kind === 'error'}>
                  <IconAlertCircle size="sm" class="w-3.5 h-3.5" />
                </Show>
                <Show when={toastItem.kind === 'warning'}>
                  <IconAlertTriangle size="sm" class="w-3.5 h-3.5" />
                </Show>
                <Show when={toastItem.kind === 'info'}>
                  <IconInfo size="sm" class="w-3.5 h-3.5" />
                </Show>
              </div>

              <div class="flex-1 min-w-0 pt-0.5">
                <Show when={toastItem.title}>
                  <div class="text-xs font-semibold text-foreground mb-0.5 leading-snug">
                    {toastItem.title}
                  </div>
                </Show>
                <div class="text-xs text-muted leading-relaxed wrap-break-word whitespace-pre-wrap">
                  {toastItem.message}
                </div>
              </div>

              <button
                type="button"
                aria-label={t('common.close')}
                onClick={() => dismiss(toastItem.id)}
                class="text-faint hover:text-foreground p-1 rounded-md hover:bg-hover transition-colors shrink-0 -mr-1 -mt-1 cursor-pointer"
              >
                <IconClose size="xs" class="w-3.5 h-3.5" />
              </button>
            </div>
          )
        }}
      </For>
    </div>
  )
}
export interface BackToTopProps {
  threshold?: number
  class?: string
}

export const BackToTop: Component<BackToTopProps> = props => {
  const { t } = useI18n()
  const [visible, setVisible] = createSignal(false)
  const threshold = () => props.threshold ?? 280

  onMount(() => {
    const handleScroll = () => {
      setVisible(window.scrollY > threshold())
    }
    window.addEventListener('scroll', handleScroll, { passive: true })
    handleScroll()
    onCleanup(() => window.removeEventListener('scroll', handleScroll))
  })

  return (
    <Show when={visible()}>
      <div class={`fixed bottom-6 right-6 z-30 animate-fade-in pointer-events-auto ${props.class ?? ''}`}>
        <IconButton
          size="lg"
          variant="secondary"
          class="rounded-full shadow-lg glass-card hover:scale-110 active:scale-95 transition-all duration-200 cursor-pointer"
          onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
          title={t('common.backToTop')}
          aria-label={t('common.backToTop')}
        >
          <IconChevronUp size={16} />
        </IconButton>
      </div>
    </Show>
  )
}

export interface UnsavedChangesBarProps {
  show: boolean
  loading?: boolean
  message?: string
  saveLabel?: string
  resetLabel?: string
  onSave: () => void | Promise<void>
  onReset?: () => void
  class?: string
}

export const UnsavedChangesBar: Component<UnsavedChangesBarProps> = props => {
  const { t } = useI18n()

  // 绑定全局快捷键 Ctrl+S / Cmd+S
  createEffect(() => {
    if (!props.show) return
    const handleKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        e.preventDefault()
        if (!props.loading) {
          props.onSave()
        }
      }
    }
    window.addEventListener('keydown', handleKey)
    onCleanup(() => window.removeEventListener('keydown', handleKey))
  })

  return (
    <Show when={props.show}>
      <div class={`fixed bottom-6 inset-x-0 z-40 flex justify-center pointer-events-none px-4 ${props.class ?? ''}`}>
        <div class="pointer-events-auto flex items-center justify-between sm:justify-start gap-3.5 px-4 py-2.5 rounded-2xl glass-float border border-accent/40 shadow-2xl backdrop-blur-xl animate-slide-up max-w-lg w-full sm:w-auto">
          <div class="flex items-center gap-2.5 min-w-0">
            <span class="w-2 h-2 rounded-full bg-warning animate-pulse shrink-0" />
            <span class="text-xs font-medium text-foreground truncate">
              {props.message ?? t('common.unsavedChanges')}
            </span>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <Show when={props.onReset}>
              <Button
                variant="ghost"
                size="sm"
                onClick={props.onReset}
                disabled={props.loading}
                class="text-xs text-muted hover:text-foreground cursor-pointer"
              >
                {props.resetLabel ?? t('common.discard')}
              </Button>
            </Show>
            <Button
              variant="primary"
              size="sm"
              loading={props.loading}
              onClick={props.onSave}
              class="gap-1.5 shadow-xs text-xs font-semibold cursor-pointer"
            >
              <IconCheck size={14} />
              <span>{props.saveLabel ?? t('common.saveChanges')}</span>
              <kbd class="hidden sm:inline-block text-[10px] font-mono px-1 py-0.2 rounded bg-white/20 text-on-accent leading-none">
                Ctrl+S
              </kbd>
            </Button>
          </div>
        </div>
      </div>
    </Show>
  )
}

export const Alert: Component<{
  variant?: 'info' | 'success' | 'warning' | 'danger'
  title?: string
  children?: JSX.Element
  closable?: boolean
  onClose?: () => void
  class?: string
}> = props => {
  const [closed, setClosed] = createSignal(false)
  const variant = () => props.variant ?? 'info'

  const styles = {
    info: {
      box: 'bg-info/10 text-foreground shadow-xs',
      iconWrap: 'text-info',
    },
    success: {
      box: 'bg-success/10 text-foreground shadow-xs',
      iconWrap: 'text-success',
    },
    warning: {
      box: 'bg-warning/10 text-foreground shadow-xs',
      iconWrap: 'text-warning',
    },
    danger: {
      box: 'bg-danger/10 text-foreground shadow-xs',
      iconWrap: 'text-danger',
    },
  }

  const s = () => styles[variant()] || styles.info

  return (
    <Show when={!closed()}>
      <div
        role="alert"
        class={`relative flex items-start gap-3 p-3.5 rounded-xl backdrop-blur-md text-xs transition-all duration-150 ${s().box} ${props.class ?? ''}`}
      >
        <div class={`shrink-0 mt-0.5 ${s().iconWrap}`}>
          <Show when={variant() === 'success'}>
            <IconCheck size="sm" class="w-4 h-4" />
          </Show>
          <Show when={variant() === 'danger'}>
            <IconAlertCircle size="sm" class="w-4 h-4" />
          </Show>
          <Show when={variant() === 'warning'}>
            <IconAlertTriangle size="sm" class="w-4 h-4" />
          </Show>
          <Show when={variant() === 'info'}>
            <IconInfo size="sm" class="w-4 h-4" />
          </Show>
        </div>

        <div class="flex-1 min-w-0">
          <Show when={props.title}>
            <div class="font-semibold text-foreground mb-0.5 leading-snug">
              {props.title}
            </div>
          </Show>
          <div class="text-muted leading-relaxed wrap-break-word">
            {props.children}
          </div>
        </div>

        <Show when={props.closable}>
          <button
            type="button"
            aria-label={useI18n().t('common.close')}
            onClick={() => {
              setClosed(true)
              props.onClose?.()
            }}
            class="text-faint hover:text-foreground p-1 rounded-md hover:bg-hover transition-colors shrink-0 -mr-1 -mt-1 cursor-pointer"
          >
            <IconClose size="xs" class="w-3.5 h-3.5" />
          </button>
        </Show>
      </div>
    </Show>
  )
}

export const controlSizes = {
  sm: 'h-8 text-xs',
  md: 'h-9 text-sm',
  lg: 'h-11 text-base',
} as const

export type ControlSize = keyof typeof controlSizes

const buttonBase = 'ui-control inline-flex items-center justify-center border font-medium leading-none [&>svg]:block [&>svg]:shrink-0 select-none whitespace-nowrap shrink-0 disabled:opacity-50 disabled:pointer-events-none cursor-pointer rounded-control'
const buttonVariants = {
  primary: 'bg-accent border-transparent text-on-accent hover:bg-accent-hover shadow-accent',
  secondary: 'bg-control border-control-border text-text hover:bg-hover hover:border-glass-border-hover',
  ghost: 'border-transparent text-muted hover:text-foreground hover:bg-hover',
  danger: 'bg-danger/10 border-danger/20 text-danger hover:bg-danger/20',
}

export interface ButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'ghost' | 'danger' | 'secondary'
  size?: 'sm' | 'md' | 'lg'
  disabled?: boolean
  loading?: boolean
  onClick?: (e: MouseEvent) => void
  type?: 'button' | 'submit' | 'reset'
  title?: string
  children?: JSX.Element
  class?: string
  ref?: HTMLButtonElement | ((el: HTMLButtonElement) => void)
}

export const Button: Component<ButtonProps> = props => {
  const [local, others] = splitProps(props, ['variant', 'size', 'disabled', 'loading', 'children', 'class', 'ref', 'type'])
  const sizes: Record<ControlSize, string> = {
    sm: `${controlSizes.sm} px-3 min-w-fit gap-1.5`,
    md: `${controlSizes.md} px-4 min-w-fit gap-2`,
    lg: `${controlSizes.lg} px-5 min-w-fit gap-2.5`,
  }
  return (
    <button
      ref={local.ref}
      type={local.type ?? 'button'}
      class={`${buttonBase} ${sizes[local.size ?? 'md']} ${buttonVariants[local.variant ?? 'secondary']} ${local.class ?? ''}`}
      disabled={local.disabled || local.loading}
      aria-busy={local.loading || undefined}
      {...others}
    >
      <Show when={local.loading}>
        <Spinner size={local.size} />
      </Show>
      {local.children}
    </button>
  )
}

export interface IconButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'ghost' | 'danger' | 'secondary'
  size?: 'xs' | 'sm' | 'md' | 'lg'
  disabled?: boolean
  loading?: boolean
  onClick?: (e: MouseEvent) => void
  type?: 'button' | 'submit' | 'reset'
  title?: string
  children?: JSX.Element
  class?: string
  ref?: HTMLButtonElement | ((el: HTMLButtonElement) => void)
}

export const IconButton: Component<IconButtonProps> = props => {
  const [local, others] = splitProps(props, ['variant', 'size', 'disabled', 'loading', 'children', 'class', 'ref', 'type'])
  const iconSizes: Record<'xs' | 'sm' | 'md' | 'lg', string> = {
    xs: 'w-6 h-6 text-xs',
    sm: `w-8 ${controlSizes.sm}`,
    md: `w-9 ${controlSizes.md}`,
    lg: `w-11 ${controlSizes.lg}`,
  }
  return (
    <button
      ref={local.ref}
      type={local.type ?? 'button'}
      class={`${buttonBase} ${iconSizes[local.size ?? 'md']} ${buttonVariants[local.variant ?? 'ghost']} ${local.class ?? ''}`}
      disabled={local.disabled || local.loading}
      aria-busy={local.loading || undefined}
      {...others}
    >
      <Show when={local.loading} fallback={local.children}>
        <Spinner size={local.size === 'xs' || local.size === 'sm' ? 'sm' : 'md'} />
      </Show>
    </button>
  )
}
export const Input: Component<{
  value?: string | number
  placeholder?: string
  type?: string
  size?: 'sm' | 'md' | 'lg'
  disabled?: boolean
  min?: number | string
  max?: number | string
  step?: number | string
  onInput?: (v: string) => void
  onKeyDown?: (e: KeyboardEvent) => void
  class?: string
  ariaLabel?: string
}> = props => {
  const sizes: Record<ControlSize, string> = {
    sm: `${controlSizes.sm} min-h-8 px-2.5`,
    md: `${controlSizes.md} min-h-9 px-3`,
    lg: `${controlSizes.lg} min-h-11 px-4`,
  }
  return (
    <input
      type={props.type ?? 'text'}
      value={props.value ?? ''}
      placeholder={props.placeholder}
      disabled={props.disabled}
      min={props.min}
      max={props.max}
      step={props.step}
      aria-label={props.ariaLabel}
      onInput={e => props.onInput?.(e.currentTarget.value)}
      onKeyDown={props.onKeyDown}
      class={`w-full min-w-0 ${sizes[props.size ?? 'md']} ui-field rounded-control disabled:opacity-50 disabled:cursor-not-allowed ${props.class ?? ''}`}
    />
  )
}
export interface TextareaProps {
  value?: string | number
  placeholder?: string
  rows?: number
  disabled?: boolean
  onInput?: (v: string) => void
  onKeyDown?: (e: KeyboardEvent) => void
  class?: string
  ariaLabel?: string
}

export const Textarea: Component<TextareaProps> = props => {
  return (
    <textarea
      value={props.value ?? ''}
      placeholder={props.placeholder}
      rows={props.rows}
      disabled={props.disabled}
      aria-label={props.ariaLabel}
      onInput={e => props.onInput?.(e.currentTarget.value)}
      onKeyDown={props.onKeyDown}
      class={`w-full px-3 py-2 text-sm ui-field rounded-control disabled:opacity-50 disabled:cursor-not-allowed ${props.class ?? ''}`}
    />
  )
}
export interface SelectOption {
  value: string
  label: string
  description?: string
  badge?: string
}

export const Select: Component<{
  value?: string
  options: (SelectOption | { value: string; label: string })[]
  onChange?: (v: string) => void
  size?: 'sm' | 'md' | 'lg'
  class?: string
  ariaLabel?: string
  placeholder?: string
  disabled?: boolean
  align?: 'left' | 'right'
}> = props => {
  const [open, setOpen] = createSignal(false)
  const [search, setSearch] = createSignal('')
  const [popoverStyle, setPopoverStyle] = createSignal<Record<string, string>>({})
  const listboxId = createUniqueId()
  let rootRef: HTMLDivElement | undefined
  let triggerRef: HTMLButtonElement | undefined
  let popoverRef: HTMLDivElement | undefined
  let searchInputRef: HTMLInputElement | undefined

  function updatePosition() {
    if (!rootRef) return
    const rect = rootRef.getBoundingClientRect()
    const margin = 8
    const maxWidth = Math.max(0, Math.min(520, window.innerWidth - margin * 2))
    const width = Math.min(Math.max(rect.width, 240), maxWidth)
    const desiredLeft = props.align === 'right' ? rect.right - width : rect.left
    const left = Math.max(margin, Math.min(desiredLeft, window.innerWidth - width - margin))
    const bottomAnchor = Math.max(margin, Math.min(rect.bottom + 6, window.innerHeight - margin))
    const topAnchor = Math.max(margin, Math.min(rect.top - 6, window.innerHeight - margin))
    const spaceBelow = Math.max(0, window.innerHeight - margin - bottomAnchor)
    const spaceAbove = Math.max(0, topAnchor - margin)
    const placeUp = spaceBelow < 280 && spaceAbove > spaceBelow

    setPopoverStyle({
      position: 'fixed',
      'z-index': '110',
      width: `${width}px`,
      'min-width': `${width}px`,
      'max-width': `${maxWidth}px`,
      'max-height': `${placeUp ? spaceAbove : spaceBelow}px`,
      left: `${left}px`,
      ...(placeUp
        ? { bottom: `${window.innerHeight - topAnchor}px` }
        : { top: `${bottomAnchor}px` }),
    })
  }

  function closeAndFocus() {
    setOpen(false)
    triggerRef?.focus()
  }

  function toggleOpen() {
    if (props.disabled) return
    if (!open()) {
      updatePosition()
      setOpen(true)
      queueMicrotask(() => {
        if (open()) searchInputRef?.focus()
      })
    } else {
      closeAndFocus()
    }
  }

  const selectedOption = () => (props.options as SelectOption[]).find(o => o.value === (props.value ?? ''))
  const isSelected = () => !!selectedOption()
  const displayLabel = () => selectedOption()?.label || props.placeholder || (props.options[0]?.label ?? '')
  const filteredOptions = createMemo(() => {
    const q = search().trim().toLowerCase()
    const opts = props.options as SelectOption[]
    if (!q) return opts
    return opts.filter(o =>
      o.label.toLowerCase().includes(q) ||
      o.value.toLowerCase().includes(q) ||
      (o.description && o.description.toLowerCase().includes(q)) ||
      (o.badge && o.badge.toLowerCase().includes(q))
    )
  })

  const INITIAL_LIMIT = 40
  const [displayLimit, setDisplayLimit] = createSignal(INITIAL_LIMIT)

  createEffect(() => {
    search()
    open()
    setDisplayLimit(INITIAL_LIMIT)
  })

  const visibleOptions = createMemo(() => {
    const list = filteredOptions()
    if (list.length <= INITIAL_LIMIT) return list
    return list.slice(0, displayLimit())
  })

  const handleListScroll = (e: Event) => {
    const el = e.currentTarget as HTMLElement
    if (el.scrollHeight - el.scrollTop - el.clientHeight < 120) {
      if (displayLimit() < filteredOptions().length) {
        setDisplayLimit(l => l + 40)
      }
    }
  }

  onMount(() => {
    const handleOutsideClick = (e: MouseEvent | PointerEvent) => {
      const target = e.target as Node
      if (rootRef?.contains(target) || popoverRef?.contains(target)) {
        return
      }
      setOpen(false)
    }

    const handleScrollOrResize = () => {
      if (open()) {
        updatePosition()
      }
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      if (!open()) {
        if ((e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Enter') && rootRef?.contains(document.activeElement)) {
          e.preventDefault()
          toggleOpen()
        }
        return
      }

      if (props.disabled || (!rootRef?.contains(document.activeElement) && !popoverRef?.contains(document.activeElement))) return

      if (e.key === 'Tab') {
        closeAndFocus()
      } else if (e.key === 'Escape') {
        e.preventDefault()
        e.stopPropagation()
        closeAndFocus()
      } else if (e.key === 'ArrowDown') {
        e.preventDefault()
        const opts = visibleOptions()
        const idx = opts.findIndex(o => o.value === (props.value ?? ''))
        const next = idx < opts.length - 1 ? idx + 1 : 0
        if (opts[next]) props.onChange?.(opts[next].value)
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        const opts = visibleOptions()
        const idx = opts.findIndex(o => o.value === (props.value ?? ''))
        const prev = idx > 0 ? idx - 1 : opts.length - 1
        if (opts[prev]) props.onChange?.(opts[prev].value)
      } else if (e.key === 'Enter' || (e.key === ' ' && e.target !== searchInputRef)) {
        if (e.target instanceof HTMLElement && e.target.closest('[role="option"]')) return
        e.preventDefault()
        closeAndFocus()
      }
    }

    window.addEventListener('pointerdown', handleOutsideClick)
    window.addEventListener('keydown', handleKeyDown, true)
    window.addEventListener('scroll', handleScrollOrResize, true)
    window.addEventListener('resize', handleScrollOrResize)
    onCleanup(() => {
      window.removeEventListener('pointerdown', handleOutsideClick)
      window.removeEventListener('keydown', handleKeyDown, true)
      window.removeEventListener('scroll', handleScrollOrResize, true)
      window.removeEventListener('resize', handleScrollOrResize)
    })
  })
  const triggerSizes: Record<ControlSize, string> = {
    sm: `${controlSizes.sm} min-h-8 pl-3 pr-2.5 gap-2`,
    md: `${controlSizes.md} min-h-9 pl-3.5 pr-3 gap-2.5`,
    lg: `${controlSizes.lg} min-h-11 pl-4 pr-3.5 gap-3`,
  }

  const chevronSizes = {
    sm: 'w-3 h-3',
    md: 'w-3.5 h-3.5',
    lg: 'w-4 h-4',
  }

  const optionSizes = {
    sm: 'px-2.5 py-1.5 text-xs',
    md: 'px-3 py-2 text-sm',
    lg: 'px-3.5 py-2.5 text-base',
  }

  return (
    <div
      ref={rootRef}
      class={`ui-select ${props.class ?? ''}`}
    >
      <button
        ref={triggerRef}
        type="button"
        role="combobox"
        aria-expanded={open()}
        aria-haspopup="listbox"
        aria-controls={open() ? listboxId : undefined}
        aria-label={props.ariaLabel || displayLabel()}
        title={selectedOption()?.description || displayLabel()}
        disabled={props.disabled}
        onClick={toggleOpen}
        class={`ui-field w-full min-w-0 flex items-center justify-between ${triggerSizes[props.size ?? 'md']} rounded-control hover:border-glass-border-hover ${
          open() ? 'ring-2 ring-accent/30' : ''
        } disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer`}
      >
        <div class="flex items-center gap-1.5 min-w-0 flex-1 truncate text-left">
          <Show when={selectedOption()?.badge}>
            <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-surface-inset text-muted shrink-0 border border-subtle/50">
              {selectedOption()!.badge}
            </span>
          </Show>
          <span class={`truncate ${isSelected() ? 'text-text font-medium' : 'text-faint'}`}>
            {displayLabel()}
          </span>
        </div>
        <div class="flex items-center justify-center shrink-0 text-faint ml-1">
          <IconChevronDown
            class={`${chevronSizes[props.size ?? 'md']} transition-transform duration-200 ${
              open() ? 'rotate-180 text-accent' : ''
            }`}
          />
        </div>
      </button>

      <Show when={open()}>
        <Portal>
          <div
            ref={popoverRef}
            style={popoverStyle()}
            class="rounded-control glass-float p-1.5 flex flex-col gap-1 overflow-hidden animate-fade-in"
          >
            <Show when={props.options.length > 8}>
              <div class="shrink-0 px-1 pt-0.5 pb-1 border-b border-subtle/60">
                <input
                  ref={searchInputRef}
                  type="text"
                  value={search()}
                  onInput={e => setSearch(e.currentTarget.value)}
                  placeholder={useI18n().t('ui.searchOptions')}
                  aria-label={useI18n().t('ui.searchOptions')}
                  class="ui-field w-full px-2.5 py-1.5 text-xs rounded-md"
                  onClick={e => e.stopPropagation()}
                />
              </div>
            </Show>
            <div
              id={listboxId}
              role="listbox"
              aria-label={props.ariaLabel || displayLabel()}
              class="min-h-0 max-h-60 overflow-y-auto space-y-0.5 px-1"
              onScroll={handleListScroll}
            >
              <For each={visibleOptions()}>
                {o => {
                  const active = () => (props.value ?? '') === o.value
                  return (
                    <button
                      type="button"
                      role="option"
                      aria-selected={active()}
                      disabled={props.disabled}
                      class={`ui-control border border-transparent w-full flex items-center justify-between gap-2.5 ${optionSizes[props.size ?? 'md']} rounded-lg text-left font-medium cursor-pointer ${
                        active()
                          ? 'bg-accent/15 text-accent'
                          : 'text-text hover:bg-hover hover:text-text'
                      }`}
                      onClick={() => {
                        if (props.disabled) return
                        props.onChange?.(o.value)
                        closeAndFocus()
                        setSearch('')
                      }}
                    >
                      <div class="min-w-0 flex-1 flex flex-col py-0.5">
                        <div class="flex items-center gap-1.5 min-w-0">
                          <Show when={o.badge}>
                            <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-surface-inset text-muted shrink-0 border border-subtle/40">
                              {o.badge}
                            </span>
                          </Show>
                          <span class="truncate font-medium text-xs">{o.label}</span>
                        </div>
                        <Show when={o.description}>
                          <span class="text-[10px] font-mono text-faint truncate mt-0.5">{o.description}</span>
                        </Show>
                      </div>
                      <Show when={active()}>
                        <IconCheck class="w-4 h-4 text-accent shrink-0 animate-scale-in" stroke-width="2.5" />
                      </Show>
                    </button>
                  )
                }}
              </For>
              <Show when={displayLimit() < filteredOptions().length}>
                <div class="px-2 py-1.5 text-center text-[10px] text-faint border-t border-subtle/30">
                  {useI18n().t('ui.showingOptions', { visible: visibleOptions().length, total: filteredOptions().length })}
                </div>
              </Show>
              <Show when={filteredOptions().length === 0}>
                <div class="px-3 py-3 text-center text-xs text-faint">{useI18n().t('ui.noMatchingOptions')}</div>
              </Show>
            </div>
          </div>
        </Portal>
      </Show>
    </div>
  )
}

export const Toggle: Component<{
  ariaLabel: string
  checked?: boolean
  disabled?: boolean
  size?: 'sm' | 'md'
  onChange?: (v: boolean) => void
}> = props => {
  const isSm = () => props.size === 'sm'
  return (
    <button
      type="button"
      role="switch"
      aria-label={props.ariaLabel}
      aria-checked={props.checked ?? false}
      disabled={props.disabled}
      onClick={e => {
        e.stopPropagation()
        props.onChange?.(!props.checked)
      }}
      class={`ui-control relative inline-flex items-center shrink-0 p-0.5 rounded-full disabled:opacity-50 disabled:cursor-not-allowed shadow-inner transition-colors duration-200 ${
        isSm() ? 'w-7 h-4' : 'w-9 h-5'
      } ${
        props.checked ? 'bg-accent' : 'bg-control-border'
      }`}
    >
      <span
        class={`pointer-events-none block rounded-full bg-switch-thumb shadow-sm transition-transform duration-200 ${
          isSm() ? 'w-3 h-3' : 'w-4 h-4'
        } ${
          props.checked ? (isSm() ? 'translate-x-3' : 'translate-x-4') : 'translate-x-0'
        }`}
      />
    </button>
  )
}
export interface CheckboxProps {
  checked?: boolean
  disabled?: boolean
  onChange?: (checked: boolean) => void
  label?: JSX.Element
  description?: JSX.Element
  class?: string
  children?: JSX.Element
}

export const Checkbox: Component<CheckboxProps> = props => (
  <label
    class={`inline-flex items-center gap-1.5 select-none cursor-pointer group ${
      props.disabled ? 'opacity-50 cursor-not-allowed pointer-events-none' : ''
    } ${props.class ?? ''}`}
  >
    <div class="relative flex items-center justify-center shrink-0">
      <input
        type="checkbox"
        class="sr-only peer"
        checked={props.checked ?? false}
        disabled={props.disabled}
        onChange={e => props.onChange?.(e.currentTarget.checked)}
      />
      <div
        class={`w-4 h-4 rounded-[5px] border transition-[background-color,border-color,box-shadow] duration-200 flex items-center justify-center shadow-xs peer-focus-visible:ring-2 peer-focus-visible:ring-accent/40 ${
          props.checked
            ? 'bg-accent border-accent text-on-accent'
            : 'border-control-border bg-control group-hover:border-glass-border-hover group-hover:bg-hover'
        }`}
      >
        <Show when={props.checked}>
          <IconCheck class="w-3 h-3" stroke-width="3" />
        </Show>
      </div>
    </div>
    <Show when={props.label || props.description || props.children}>
      <div class="min-w-0 text-xs leading-snug">
        <Show when={props.label}>
          <div class="font-medium text-foreground group-hover:text-accent transition-colors">{props.label}</div>
        </Show>
        <Show when={props.description}>
          <div class="text-[11px] text-faint mt-0.5">{props.description}</div>
        </Show>
        {props.children}
      </div>
    </Show>
  </label>
)
export interface SegmentedControlOption<T extends string = string> {
  value: T
  label: JSX.Element
  icon?: JSX.Element
  disabled?: boolean
}

export interface SegmentedControlProps<T extends string = string> {
  value: T
  onChange: (value: T) => void
  options: Array<SegmentedControlOption<T>>
  size?: 'sm' | 'md'
  class?: string
}

export function SegmentedControl<T extends string = string>(props: SegmentedControlProps<T>): JSX.Element {
  const size = () => props.size ?? 'sm'
  return (
    <div
      role="tablist"
      class={`inline-flex flex-nowrap shrink-0 max-w-full overflow-x-auto no-scrollbar items-center gap-1 p-1 rounded-xl bg-surface-inset border border-subtle select-none ${props.class ?? ''}`}
    >
      <For each={props.options}>
        {opt => {
          const isActive = () => props.value === opt.value
          return (
            <button
              type="button"
              role="tab"
              aria-selected={isActive()}
              disabled={opt.disabled}
              onClick={() => {
                if (!opt.disabled && props.value !== opt.value) {
                  props.onChange(opt.value)
                }
              }}
              class={`ui-control border font-medium rounded-lg flex items-center gap-1.5 whitespace-nowrap cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed ${
                size() === 'md' ? 'px-4 py-2 text-sm' : 'px-3.5 py-1.5 text-xs'
              } ${
                isActive()
                  ? 'glass-segment text-foreground'
                  : 'text-muted hover:text-foreground hover:bg-hover border-transparent'
              }`}
            >
              <Show when={opt.icon}>
                <span class="shrink-0">{opt.icon}</span>
              </Show>
              <span>{opt.label}</span>
            </button>
          )
        }}
      </For>
    </div>
  )
}

export interface ModalProps {
  open: boolean
  title?: string
  header?: JSX.Element
  size?: 'sm' | 'md' | 'lg' | 'xl' | '2xl'
  class?: string
  bodyClass?: string
  footer?: JSX.Element
  onClose: () => void
  children?: JSX.Element
}

export const Modal: Component<ModalProps> = props => {
  const [panel, setPanel] = createSignal<HTMLDivElement>()
  const { t } = useI18n()

  createEffect(() => {
    if (!props.open) return
    const previousFocus = document.activeElement
    const previousOverflow = document.body.style.overflow
    let active = true
    const focusable = () => Array.from(panel()?.querySelectorAll<HTMLElement>(
      'a[href], button, input, select, textarea, [tabindex], [contenteditable="true"]',
    ) ?? []).filter(el => el.tabIndex >= 0 && !el.matches(':disabled') &&
      !el.closest('[hidden], [inert], [aria-hidden="true"]') &&
      getComputedStyle(el).display !== 'none' && getComputedStyle(el).visibility !== 'hidden')

    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented) return
      if (e.key === 'Escape') {
        e.preventDefault()
        props.onClose()
      } else if (e.key === 'Tab') {
        const elements = focusable()
        const first = elements[0]
        const last = elements[elements.length - 1]
        if (!first || !panel()?.contains(document.activeElement)) {
          e.preventDefault()
          const target = (e.shiftKey ? last : first) ?? panel()
          target?.focus()
        } else if (e.shiftKey && (document.activeElement === first || document.activeElement === panel())) {
          e.preventDefault()
          last?.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }
    document.addEventListener('keydown', onKey)
    document.body.style.overflow = 'hidden'

    queueMicrotask(() => {
      if (active) (focusable()[0] ?? panel())?.focus()
    })

    onCleanup(() => {
      active = false
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = previousOverflow
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected) previousFocus.focus()
    })
  })

  return (
    <Show when={props.open}>
      <Portal>
        <div class="fixed inset-0 z-100 flex items-center justify-center p-4 sm:p-6">
          <button
            type="button"
            tabIndex={-1}
            aria-label={t('common.close')}
            class="absolute inset-0 w-full h-full bg-overlay backdrop-blur-md animate-fade-in border-none cursor-default"
            onClick={props.onClose}
          />
          <div
            ref={setPanel}
            role="dialog"
            tabIndex={-1}
            aria-modal="true"
            aria-label={props.title || 'Dialog'}
            class={`relative flex flex-col max-h-[calc(100dvh-2rem)] sm:max-h-[calc(100dvh-3rem)] w-full ${props.size === '2xl' ? 'max-w-2xl' : props.size === 'xl' ? 'max-w-xl' : props.size === 'md' ? 'max-w-md' : props.size === 'sm' ? 'max-w-sm' : 'max-w-lg'} rounded-2xl glass-float animate-scale-in z-10 shadow-2xl ${props.class ?? ''}`}
          >
            <div class="shrink-0 flex items-center justify-between gap-3 px-5 py-3.5 border-b border-subtle/50 bg-surface-inset/50 rounded-t-2xl">
              <Show when={props.header} fallback={<h3 class="min-w-0 text-sm font-semibold wrap-break-word">{props.title}</h3>}>
                {props.header}
              </Show>
              <IconButton
                size="sm"
                class="shrink-0"
                onClick={props.onClose}
                aria-label={t('common.close')}
              >
                <IconClose size={14} />
              </IconButton>
            </div>
            <div class={`min-h-0 overflow-y-auto ${props.bodyClass ?? 'px-5 py-5'}`}>{props.children}</div>
            <Show when={props.footer}>
              <div class="min-h-14 px-5 py-3 border-t border-subtle/50 flex items-center justify-end shrink-0 bg-surface-inset/30 rounded-b-2xl">
                {props.footer}
              </div>
            </Show>
          </div>
        </div>
      </Portal>
    </Show>
  )
}

export const Skeleton: Component<{ class?: string }> = props => (
  <div class={`animate-pulse rounded-control bg-hover ${props.class ?? 'h-4 w-full'}`} aria-hidden="true" />
)

export const LoadState: Component<{
  ready: boolean
  error: boolean
  onRetry: () => void
  fallback: JSX.Element
  children?: JSX.Element
}> = props => {
  const { t } = useI18n()
  return (
    <>
      <Show when={props.error}>
        <Alert variant="danger" class="mb-3">
          <div class="flex items-center justify-between gap-3">
            <span>{t('common.loadFailed')}</span>
            <Button size="sm" onClick={props.onRetry}>{t('common.retry')}</Button>
          </div>
        </Alert>
      </Show>
      <Show when={props.ready} fallback={
        <Show when={!props.error}>
          <div role="status" aria-label={t('common.loading')} aria-busy="true">{props.fallback}</div>
        </Show>
      }>
        {props.children}
      </Show>
    </>
  )
}

export const Field: Component<{ label: string; hint?: string; children?: JSX.Element }> = props => (
  <label class="block space-y-2">
    <span class="block text-xs font-medium text-muted/90 select-none">{props.label}</span>
    <div class="mt-1">{props.children}</div>
    <Show when={props.hint}>
      <span class="block text-[11px] text-faint mt-1.5 leading-relaxed">{props.hint}</span>
    </Show>
  </label>
)

export interface SliderProps {
  value: number
  min?: number
  max?: number
  step?: number
  disabled?: boolean
  label?: string
  hint?: string
  valueDisplay?: string | number
  unit?: string
  onChange?: (val: number) => void
  onInput?: (val: number) => void
  class?: string
}

export const Slider: Component<SliderProps> = props => {
  const min = () => props.min ?? 0
  const max = () => props.max ?? 100
  const step = () => props.step ?? 1
  const pct = () => {
    const range = max() - min()
    if (range <= 0) return 0
    return Math.min(100, Math.max(0, ((props.value - min()) / range) * 100))
  }

  return (
    <div class={`space-y-1.5 w-full select-none ${props.class ?? ''}`}>
      <Show when={props.label || props.valueDisplay !== undefined || props.unit}>
        <div class="flex items-center justify-between text-xs">
          <Show when={props.label}>
            <span class="font-medium text-muted">{props.label}</span>
          </Show>
          <span class="font-mono text-[11px] font-semibold text-foreground px-2 py-0.5 rounded-md bg-hover border border-subtle/50 shadow-2xs">
            {props.valueDisplay ?? `${props.value}${props.unit ?? ''}`}
          </span>
        </div>
      </Show>
      <div class="relative flex items-center h-6 w-full">
        <div class="absolute inset-x-0 h-1.5 rounded-full bg-control-border pointer-events-none overflow-hidden">
          <div
            class="h-full bg-gradient-to-r from-accent to-accent/90 rounded-full transition-[width] duration-75"
            style={{ width: `${pct()}%` }}
          />
        </div>
        <input
          type="range"
          aria-label={props.label}
          min={min()}
          max={max()}
          step={step()}
          disabled={props.disabled}
          value={props.value}
          onInput={e => {
            const val = Number(e.currentTarget.value)
            props.onInput?.(val)
            props.onChange?.(val)
          }}
          onChange={e => {
            props.onChange?.(Number(e.currentTarget.value))
          }}
          class="relative w-full z-10 appearance-none bg-transparent [&::-webkit-slider-runnable-track]:bg-transparent [&::-moz-range-track]:bg-transparent [&::-moz-range-progress]:bg-transparent focus-visible:ring-2 focus-visible:ring-accent/30 rounded-full cursor-pointer disabled:cursor-not-allowed disabled:opacity-40"
        />
      </div>
      <Show when={props.hint}>
        <p class="text-[11px] text-faint leading-relaxed">{props.hint}</p>
      </Show>
    </div>
  )
}

export interface FileUploadProps {
  accept?: string
  value?: File | null
  onChange?: (file: File | null) => void
  label?: string
  hint?: string
  placeholder?: string
  disabled?: boolean
  compact?: boolean
  class?: string
}

export const FileUpload: Component<FileUploadProps> = props => {
  let fileInputRef: HTMLInputElement | undefined
  const [dragOver, setDragOver] = createSignal(false)
  const { t } = useI18n()

  function handleFile(f?: File) {
    if (props.disabled || !f) return
    props.onChange?.(f)
  }

  function openPicker() {
    if (!props.disabled) fileInputRef?.click()
  }

  return (
    <div class={`w-full ${props.class ?? ''}`}>
      <input
        ref={fileInputRef}
        type="file"
        accept={props.accept}
        disabled={props.disabled}
        class="hidden"
        onChange={e => {
          const f = e.currentTarget.files?.[0]
          handleFile(f)
        }}
      />
      <Show
        when={!props.compact}
        fallback={
          <Button
            disabled={props.disabled}
            onClick={openPicker}
            class="w-full min-w-0 group"
          >
            <IconImage size={15} class="text-muted group-hover:text-foreground transition-colors shrink-0" />
            <span class="truncate font-medium">
              {props.value ? props.value.name : (props.placeholder ?? t('common.chooseFile'))}
            </span>
          </Button>
        }
      >
        <Show
          when={props.value}
          fallback={
            <div
              role="button"
              tabIndex={props.disabled ? -1 : 0}
              aria-disabled={props.disabled ?? false}
              aria-label={props.label ?? props.placeholder ?? t('common.chooseFile')}
              onClick={openPicker}
              onKeyDown={e => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  openPicker()
                }
              }}
              onDragOver={e => {
                e.preventDefault()
                if (!props.disabled) setDragOver(true)
              }}
              onDragLeave={() => setDragOver(false)}
              onDrop={e => {
                e.preventDefault()
                setDragOver(false)
                const f = e.dataTransfer?.files?.[0]
                handleFile(f)
              }}
              class={`ui-control rounded-2xl border-2 border-dashed p-5 text-center ${
                props.disabled
                  ? 'opacity-50 cursor-not-allowed border-control-border bg-surface-inset'
                  : dragOver()
                    ? 'cursor-pointer border-accent bg-accent/10'
                    : 'cursor-pointer border-control-border hover:border-glass-border-hover bg-surface-inset hover:bg-hover'
              }`}
            >
              <div class="flex flex-col items-center gap-2">
                <div class="w-10 h-10 rounded-xl bg-accent/10 border border-accent/20 flex items-center justify-center text-accent">
                  <IconUpload size={20} />
                </div>
                <div class="text-xs font-semibold text-foreground">
                  {props.placeholder ?? t('common.dragOrClickFile')}
                </div>
                <div class="text-[11px] text-faint">
                  {props.hint ?? (props.accept ? `${t('common.supportedFormats')}: ${props.accept}` : '')}
                </div>
              </div>
            </div>
          }
        >
          <div class="flex items-center justify-between p-3.5 rounded-2xl bg-card/60 border border-accent/30 shadow-2xs">
            <div class="flex items-center gap-3 min-w-0">
              <div class="w-9 h-9 rounded-xl bg-accent/10 border border-accent/20 flex items-center justify-center text-accent shrink-0">
                <IconFile size={18} />
              </div>
              <div class="min-w-0">
                <div class="text-xs font-semibold text-foreground truncate">{props.value!.name}</div>
                <div class="text-[11px] text-faint font-mono mt-0.5">{formatBytes(props.value!.size)}</div>
              </div>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <Button
                size="sm"
                variant="ghost"
                disabled={props.disabled}
                onClick={openPicker}
              >
                {t('common.change')}
              </Button>
              <IconButton
                size="sm"
                variant="danger"
                disabled={props.disabled}
                onClick={() => {
                  if (props.disabled) return
                  if (fileInputRef) fileInputRef.value = ''
                  props.onChange?.(null)
                }}
                title={t('common.clear')}
                aria-label={t('common.clear')}
              >
                <IconClose size={14} />
              </IconButton>
            </div>
          </div>
        </Show>
      </Show>
    </div>
  )
}

export interface TabTransitionProps<T extends string = string> {
  value: T
  order?: T[]
  class?: string
  views?: Partial<Record<T, () => JSX.Element>>
  children?: (tab: T) => JSX.Element
}

export function TabTransition<T extends string = string>(props: TabTransitionProps<T>) {
  const [current, setCurrent] = createSignal<T>(props.value)
  const [direction, setDirection] = createSignal<'forward' | 'backward'>('forward')
  const [isTransitioning, setIsTransitioning] = createSignal(false)
  const [outgoingSnapshot, setOutgoingSnapshot] = createSignal<{ dir: 'forward' | 'backward'; el: HTMLElement } | null>(null)
  let incomingRef: HTMLDivElement | undefined
  let timer: number | undefined

  createEffect(on(() => props.value, (newVal, oldVal = newVal) => {
    if (newVal === oldVal) return

    if (timer) clearTimeout(timer)
    timer = undefined

    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
      setOutgoingSnapshot(null)
      setIsTransitioning(false)
      setCurrent(() => newVal)
      return
    }

    // Freeze outgoing DOM before the live tab changes.
    const clone = incomingRef?.cloneNode(true) as HTMLElement | undefined
    clone?.classList.remove('animate-tab-slide-in-right', 'animate-tab-slide-in-left')

    // Determine direction.
    let dir: 'forward' | 'backward' = 'forward'
    if (props.order && props.order.length > 0) {
      const oldIdx = props.order.indexOf(oldVal)
      const newIdx = props.order.indexOf(newVal)
      dir = newIdx >= oldIdx ? 'forward' : 'backward'
    }
    setDirection(dir)
    setOutgoingSnapshot(clone ? { dir, el: clone } : null)
    setCurrent(() => newVal)
    setIsTransitioning(true)

    timer = window.setTimeout(() => {
      setOutgoingSnapshot(null)
      setIsTransitioning(false)
      timer = undefined
    }, 160)
  }))

  onCleanup(() => {
    if (timer) clearTimeout(timer)
  })

  const renderTab = (tab: T) => {
    if (props.views && props.views[tab]) {
      return props.views[tab]!()
    }
    if (typeof props.children === 'function') {
      return props.children(tab)
    }
    return null
  }

  return (
    <div class={`grid grid-cols-1 ${isTransitioning() ? 'overflow-x-hidden' : ''} ${props.class ?? ''}`}>
      <Show when={outgoingSnapshot()} keyed>
        {snap => (
          <div
            aria-hidden="true"
            inert
            class={`col-start-1 row-start-1 w-full pointer-events-none ${
              snap.dir === 'forward' ? 'animate-tab-slide-out-left' : 'animate-tab-slide-out-right'
            }`}
            ref={el => {
              el.appendChild(snap.el)
            }}
          />
        )}
      </Show>
      {/* Only a tab change remounts the live view; animation cleanup preserves its state. */}
      <Show when={{ tab: current() }} keyed>
        {item => (
          <div
            ref={incomingRef}
            class={`col-start-1 row-start-1 w-full ${
              isTransitioning()
                ? (direction() === 'forward' ? 'animate-tab-slide-in-right' : 'animate-tab-slide-in-left')
                : ''
            }`}
          >
            {renderTab(item.tab)}
          </div>
        )}
      </Show>
    </div>
  )
}

export { Show }
