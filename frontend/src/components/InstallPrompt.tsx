import { useEffect, useRef, useState } from 'react'
import { Download } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'

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
    // Some privacy modes can block localStorage. The popup still works for the current visit.
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

      const elapsed = Date.now() - dismissedAt
      const wait = elapsed >= REMINDER_INTERVAL
        ? INITIAL_DELAY
        : Math.max(0, REMINDER_INTERVAL - elapsed)

      popupTimer.current = setTimeout(() => {
        popupTimer.current = null
        if (!isStandalone()) setShowPopup(true)
      }, wait)
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
      // The old prompt used permanent dismissal. Its value must not suppress the new 3-hour reminder.
      window.localStorage.removeItem(LEGACY_DISMISSED_KEY)
    } catch {
      // Continue when browser storage is unavailable.
    }

    window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
    window.addEventListener('appinstalled', handleAppInstalled)
    schedulePopup()

    return () => {
      clearPopupTimer()
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
      // The reminder has already been scheduled by the dismissal timestamp.
    }
  }

  if (installed) return null

  const ios = isIOS()
  const canUseNativePrompt = Boolean(deferredPrompt)
  const instruction = ios
    ? 'Di Safari, ketuk tombol Bagikan lalu pilih “Tambah ke Layar Utama”.'
    : 'Gunakan menu browser lalu pilih “Install aplikasi” atau “Tambahkan ke layar utama”.'

  return (
    <Dialog open={showPopup} onOpenChange={(open) => (open ? setShowPopup(true) : dismiss())}>
      <DialogContent className="max-w-[26rem] gap-0 overflow-hidden border-0 p-0 shadow-2xl [&>button]:right-4 [&>button]:top-4 [&>button]:z-10 [&>button]:rounded-full [&>button]:p-1.5 [&>button]:text-slate-500 [&>button]:transition-colors [&>button]:hover:bg-slate-100 [&>button]:hover:text-slate-900 dark:[&>button]:hover:bg-slate-800 dark:[&>button]:hover:text-white">
        <div className="relative overflow-hidden px-6 pb-6 pt-8 sm:px-8 sm:pt-9">
          <div className="pointer-events-none absolute inset-x-0 top-0 h-32 bg-gradient-to-b from-blue-50 to-transparent dark:from-blue-950/40" />
          <DialogHeader className="relative items-center space-y-3 text-center">
            <div className="rounded-[1.4rem] bg-white p-2.5 shadow-lg shadow-blue-900/10 ring-1 ring-blue-100 dark:bg-slate-950 dark:ring-blue-900/70">
              <img src="/icon-192.png" alt="PKBMTI LMS" className="h-14 w-14 rounded-2xl" />
            </div>
            <span className="rounded-full bg-blue-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.13em] text-[#0B63CE] dark:bg-blue-950/50 dark:text-blue-200">
              PKBM Tunas Ilmu
            </span>
            <DialogTitle className="pr-6 text-center text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
              Install PKBMTI LMS
            </DialogTitle>
            <DialogDescription className="max-w-[21rem] text-center text-[15px] leading-6 text-slate-500 dark:text-slate-400">
              Akses lebih cepat langsung dari layar utama perangkat Anda.
            </DialogDescription>
          </DialogHeader>

          {!canUseNativePrompt && (
            <p className="relative mt-5 rounded-xl border border-blue-100 bg-blue-50/70 px-4 py-3 text-left text-sm leading-6 text-blue-900 dark:border-blue-900/60 dark:bg-blue-950/40 dark:text-blue-100">
              {instruction}
            </p>
          )}
        </div>

        <DialogFooter className="border-t border-slate-100 bg-slate-50 px-6 py-4 sm:flex-row sm:space-x-0 sm:px-8 dark:border-slate-800 dark:bg-slate-900/70">
          <button
            type="button"
            onClick={dismiss}
            className="rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-sm font-semibold text-slate-600 transition-colors hover:border-slate-300 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
          >
            Nanti saja
          </button>
          {canUseNativePrompt ? (
            <button
              type="button"
              onClick={() => void handleInstall()}
              className="flex items-center justify-center gap-2 rounded-xl bg-[#0B63CE] px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-blue-600/20 transition-all hover:-translate-y-0.5 hover:bg-[#0754B4] hover:shadow-blue-600/30"
            >
              <Download className="h-4 w-4" />
              Install sekarang
            </button>
          ) : (
            <button
              type="button"
              onClick={dismiss}
              className="rounded-xl bg-[#0B63CE] px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-blue-600/20 transition-all hover:-translate-y-0.5 hover:bg-[#0754B4] hover:shadow-blue-600/30"
            >
              Saya mengerti
            </button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
