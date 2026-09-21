import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

export type FontScale = 1 | 1.1 | 1.25 | 1.5

type AccessibilityContextValue = {
  fontScale: FontScale
  highContrast: boolean
  setFontScale: (value: FontScale) => void
  setHighContrast: (value: boolean) => void
}

const STORAGE_KEY = 'pkbmti-accessibility'
const AccessibilityContext = createContext<AccessibilityContextValue | null>(null)

function readPreferences(): { fontScale: FontScale; highContrast: boolean } {
  try {
    const value = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}') as { fontScale?: number; highContrast?: boolean }
    const allowed: FontScale[] = [1, 1.1, 1.25, 1.5]
    return {
      fontScale: allowed.includes(value.fontScale as FontScale) ? value.fontScale as FontScale : 1,
      highContrast: Boolean(value.highContrast),
    }
  } catch {
    return { fontScale: 1, highContrast: false }
  }
}

export function AccessibilityProvider({ children }: { children: ReactNode }) {
  const initial = readPreferences()
  const [fontScale, setFontScale] = useState<FontScale>(initial.fontScale)
  const [highContrast, setHighContrast] = useState(initial.highContrast)

  useEffect(() => {
    document.documentElement.style.setProperty('--app-font-scale', String(fontScale))
    document.documentElement.classList.toggle('high-contrast', highContrast)
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ fontScale, highContrast }))
  }, [fontScale, highContrast])

  const value = useMemo(() => ({ fontScale, highContrast, setFontScale, setHighContrast }), [fontScale, highContrast])
  return <AccessibilityContext.Provider value={value}>{children}</AccessibilityContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAccessibility() {
  const context = useContext(AccessibilityContext)
  if (!context) throw new Error('useAccessibility harus digunakan di dalam AccessibilityProvider')
  return context
}
