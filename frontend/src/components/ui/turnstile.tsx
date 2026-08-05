import { Component, type ReactNode } from 'react'
import Turnstile from 'react-turnstile'

interface TurnstileWidgetProps {
  sitekey?: string
  onSuccess?: (token: string) => void
  onError?: () => void
  onExpire?: () => void
}

const TurnstileComp: any = (Turnstile as any)?.default || Turnstile

class TurnstileErrorBoundary extends Component<
  { children: ReactNode; fallback: ReactNode },
  { hasError: boolean }
> {
  constructor(props: { children: ReactNode; fallback: ReactNode }) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback
    }
    return this.props.children
  }
}

export function TurnstileWidget({
  sitekey,
  onSuccess,
  onError,
  onExpire,
}: TurnstileWidgetProps) {
  const resolvedSiteKey = sitekey || import.meta.env.VITE_TURNSTILE_SITE_KEY || (import.meta.env.DEV ? '1x00000000000000000000AA' : '')
  const isRenderable = typeof TurnstileComp === 'function' || typeof TurnstileComp === 'string'
  const canRender = Boolean(resolvedSiteKey) && isRenderable

  const fallbackUI = (
    <div className="text-xs text-muted-foreground bg-muted/40 px-3.5 py-2 rounded-lg border flex items-center justify-center gap-2">
      <span className={`w-2 h-2 rounded-full ${resolvedSiteKey ? 'bg-success' : 'bg-destructive'}`}></span>
      <span>{resolvedSiteKey ? 'Proteksi Keamanan Turnstile Aktif' : 'Proteksi Turnstile belum dikonfigurasi'}</span>
    </div>
  )

  return (
    <div className="my-3 flex justify-center min-h-[50px] items-center">
      {canRender ? (
        <TurnstileErrorBoundary fallback={fallbackUI}>
          <TurnstileComp
            sitekey={resolvedSiteKey}
            onVerify={(token: string) => {
              if (onSuccess) onSuccess(token)
            }}
            onError={() => {
              if (onError) onError()
            }}
            onExpire={() => {
              if (onExpire) onExpire()
            }}
          />
        </TurnstileErrorBoundary>
      ) : (
        fallbackUI
      )}
    </div>
  )
}
