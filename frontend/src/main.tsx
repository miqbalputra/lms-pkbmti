import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { ThemeProvider } from './context/ThemeContext.tsx'
import { AccessibilityProvider } from './context/AccessibilityContext.tsx'
import UjianCBTView from './pages/UjianCBTView.tsx'

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker
      .register('/sw.js', { updateViaCache: 'none' })
      .then((registration) => registration.update())
      .catch(() => {})
  })
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AccessibilityProvider>
      <ThemeProvider>
        {window.location.pathname === '/ujian' ? <UjianCBTView /> : <App />}
      </ThemeProvider>
    </AccessibilityProvider>
  </StrictMode>,
)
