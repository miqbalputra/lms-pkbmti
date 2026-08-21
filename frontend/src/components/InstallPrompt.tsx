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
      <DialogContent className="max-w-sm gap-5 p-6 sm:p-6">
        <DialogHeader className="items-center space-y-2 text-center">
          <img src="/icon-192.png" alt="PKBMTI LMS" className="h-16 w-16 rounded-2xl shadow-lg" />
          <DialogTitle className="pr-6 text-center text-xl">Install PKBMTI LMS</DialogTitle>
          <DialogDescription className="text-center">
            Pasang aplikasi ini di perangkat Anda agar lebih cepat diakses seperti aplikasi biasa.
          </DialogDescription>
        </DialogHeader>

        {!canUseNativePrompt && (
          <p className="rounded-xl bg-blue-50 px-4 py-3 text-sm leading-6 text-blue-900 dark:bg-blue-950/50 dark:text-blue-100">
            {instruction}
          </p>
        )}

        <DialogFooter className="gap-3 sm:flex-row">
          <button
            type="button"
            onClick={dismiss}
            className="rounded-lg bg-gray-100 px-4 py-2.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600"
          >
            Nanti saja
          </button>
          {canUseNativePrompt ? (
            <button
              type="button"
              onClick={() => void handleInstall()}
              className="flex items-center justify-center gap-2 rounded-lg bg-[#0B63CE] px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-[#0754B4]"
            >
              <Download className="h-4 w-4" />
              Install
            </button>
          ) : (
            <button
              type="button"
              onClick={dismiss}
              className="rounded-lg bg-[#0B63CE] px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-[#0754B4]"
            >
              Saya mengerti
            </button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
