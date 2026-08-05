import { useState, useEffect } from 'react'
import { Download, X } from 'lucide-react'

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const STORAGE_KEY = 'pwa-installed-dismissed'

export function InstallPrompt() {
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null)
  const [showPopup, setShowPopup] = useState(false)

  useEffect(() => {
    const dismissed = localStorage.getItem(STORAGE_KEY)
    if (dismissed) return

    const handler = (e: Event) => {
      e.preventDefault()
      setDeferredPrompt(e as BeforeInstallPromptEvent)
      setTimeout(() => setShowPopup(true), 3000)
    }

    window.addEventListener('beforeinstallprompt', handler)
    return () => window.removeEventListener('beforeinstallprompt', handler)
  }, [])

  const handleInstall = async () => {
    if (!deferredPrompt) return
    await deferredPrompt.prompt()
    const { outcome } = await deferredPrompt.userChoice
    if (outcome === 'accepted') {
      localStorage.setItem(STORAGE_KEY, 'true')
      setShowPopup(false)
    }
    setDeferredPrompt(null)
  }

  const handleDismiss = () => {
    localStorage.setItem(STORAGE_KEY, 'true')
    setShowPopup(false)
  }

  if (!showPopup || !deferredPrompt) return null

  return (
    <div className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/50 p-4">
      <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl max-w-sm w-full p-6 relative animate-in fade-in zoom-in-95 duration-200">
        <button
          onClick={handleDismiss}
          className="absolute top-3 right-3 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
        >
          <X className="w-5 h-5" />
        </button>

        <div className="flex flex-col items-center text-center">
          <img src="/icon-192.png" alt="Tunas Ilmu" className="w-16 h-16 rounded-2xl mb-4 shadow-lg" />
          <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-2">
            Install Tunas Ilmu
          </h3>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-6">
            Pasang aplikasi ini di perangkat Anda untuk akses yang lebih cepat dan mudah.
          </p>

          <div className="flex gap-3 w-full">
            <button
              onClick={handleDismiss}
              className="flex-1 px-4 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
            >
              Nanti Saja
            </button>
            <button
              onClick={handleInstall}
              className="flex-1 px-4 py-2.5 text-sm font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 transition-colors flex items-center justify-center gap-2"
            >
              <Download className="w-4 h-4" />
              Install
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
