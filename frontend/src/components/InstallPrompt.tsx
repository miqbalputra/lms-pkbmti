import { useEffect, useRef, useState } from 'react'
import { Download, X } from 'lucide-react'

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const DISMISSED_AT_KEY = 'pkbmti-lms-install-prompt:last-dismissed-at'
const LEGACY_DISMISSED_KEY = 'pwa-installed-dismissed'
const REMINDER_INTERVAL = 3 * 60 * 60 * 1000
const INITIAL_DELAY = 3 * 1000

function getStoredDismissedAt() {
  try {
    const value = Number(window.localStorage.getItem(DISMISSED_AT_KEY))
    return Number.isFinite(value) && value > 0 ? value : 0
  } catch {
    return 0
  }
}

function persistDismissedAt(dismissedAt: number) {
  try {
    window.localStorage.setItem(DISMISSED_AT_KEY, String(dismissedAt))
  } catch {
    // Some privacy modes can block localStorage. The reminder still works for this visit.
  }
}

function isStandalone() {
  const navigatorWithStandalone = navigator as Navigator & { standalone?: boolean }
  return window.matchMedia('(display-mode: standalone)').matches || navigatorWithStandalone.standalone === true
}

function isIOS() {
  return /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
}

export function InstallPrompt() {
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null)
  const [showPopup, setShowPopup] = useState(false)
  const [installed, setInstalled] = useState(() => isStandalone())
  const [dismissedAt, setDismissedAt] = useState(() => getStoredDismissedAt())
  const popupTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const suppressedRef = useRef(false)

  useEffect(() => {
    const clearPopupTimer = () => {
      if (popupTimer.current) {
        clearTimeout(popupTimer.current)
        popupTimer.current = null
      }
    }

    const schedulePopup = () => {
      clearPopupTimer()
      if (isStandalone()) {
        setInstalled(true)
        return
      }
      if (suppressedRef.current) return

      const elapsed = Date.now() - dismissedAt
      const wait = elapsed >= REMINDER_INTERVAL
        ? INITIAL_DELAY
        : Math.max(0, REMINDER_INTERVAL - elapsed)

      popupTimer.current = setTimeout(() => {
        popupTimer.current = null
        if (!isStandalone() && !suppressedRef.current) setShowPopup(true)
      }, wait)
    }

    const syncSuppression = () => {
      const shouldSuppress = Boolean(document.querySelector('[data-assessment-workspace]'))
      if (shouldSuppress === suppressedRef.current) return
      suppressedRef.current = shouldSuppress
      if (shouldSuppress) {
        clearPopupTimer()
        setShowPopup(false)
      } else {
        schedulePopup()
      }
    }

    const handleBeforeInstallPrompt = (event: Event) => {
      event.preventDefault()
      setDeferredPrompt(event as BeforeInstallPromptEvent)
    }

    const handleAppInstalled = () => {
      clearPopupTimer()
      setInstalled(true)
      setDeferredPrompt(null)
      setShowPopup(false)
    }

    try {
      window.localStorage.removeItem(LEGACY_DISMISSED_KEY)
    } catch {
      // Continue when browser storage is unavailable.
    }

    window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
    window.addEventListener('appinstalled', handleAppInstalled)
    const workspaceObserver = new MutationObserver(syncSuppression)
    workspaceObserver.observe(document.documentElement, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-assessment-workspace'] })
    syncSuppression()
    schedulePopup()

    return () => {
      clearPopupTimer()
      workspaceObserver.disconnect()
      window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
      window.removeEventListener('appinstalled', handleAppInstalled)
    }
  }, [dismissedAt])

  const dismiss = () => {
    const now = Date.now()
    persistDismissedAt(now)
    setDismissedAt(now)
    setShowPopup(false)
  }

  const handleInstall = async () => {
    if (!deferredPrompt) return

    const prompt = deferredPrompt
    dismiss()
    setDeferredPrompt(null)

    try {
      await prompt.prompt()
      const { outcome } = await prompt.userChoice
      if (outcome === 'accepted') setInstalled(true)
    } catch {
      // The reminder is rescheduled from the dismissal timestamp.
    }
  }

  if (installed || !showPopup) return null

  const instruction = isIOS()
    ? 'Di Safari, ketuk Bagikan lalu pilih “Tambah ke Layar Utama”.'
    : 'Gunakan menu browser lalu pilih “Install aplikasi” atau “Tambahkan ke layar utama”.'

  return (
    <aside
      aria-label="Instal aplikasi PKBM Tunas Ilmu"
      aria-live="polite"
      className="pointer-events-none fixed inset-x-3 bottom-[max(0.75rem,env(safe-area-inset-bottom))] z-[100] flex justify-center sm:inset-x-auto sm:right-5 sm:w-[min(25rem,calc(100vw-2rem))] sm:justify-end"
    >
      <section className="pointer-events-auto w-full max-w-md rounded-2xl border border-blue-100 bg-white p-4 shadow-[0_18px_55px_rgba(15,23,42,.2)] ring-1 ring-black/5 dark:border-slate-700 dark:bg-slate-900 dark:ring-white/10">
        <div className="flex items-start gap-3">
          <img src="/pkbmti-lms-book-192.png" alt="" className="h-11 w-11 shrink-0 rounded-xl ring-1 ring-blue-100 dark:ring-slate-700" />
          <div className="min-w-0 flex-1 pr-1">
            <h2 className="font-semibold text-slate-900 dark:text-white">Install PKBMTI LMS</h2>
            <p className="mt-1 text-sm leading-5 text-slate-600 dark:text-slate-300">Akses lebih cepat langsung dari layar utama perangkat Anda.</p>
            {!deferredPrompt && <p className="mt-2 rounded-lg bg-blue-50 px-3 py-2 text-xs leading-5 text-blue-900 dark:bg-blue-950/50 dark:text-blue-100">{instruction}</p>}
          </div>
          <button type="button" onClick={dismiss} aria-label="Tutup ajakan instal aplikasi" className="grid h-11 w-11 shrink-0 place-items-center rounded-xl text-slate-500 hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:hover:bg-slate-800">
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="mt-3 flex flex-wrap justify-end gap-2">
          <button type="button" onClick={dismiss} className="min-h-11 rounded-xl border border-slate-200 bg-white px-4 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800">
            Nanti saja
          </button>
          {deferredPrompt && <button type="button" onClick={() => void handleInstall()} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-[#0B63CE] px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-[#0754B4] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2">
            <Download className="h-4 w-4" />Install sekarang
          </button>}
        </div>
      </section>
    </aside>
  )
}
