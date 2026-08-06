import { useEffect, useState, type FormEvent } from 'react'
import {
  Eye,
  EyeOff,
  Loader2,
  Lock,
  Mail,
  ShieldCheck,
} from 'lucide-react'
import { Alert, AlertDescription } from '../components/ui/alert'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { TurnstileWidget } from '../components/ui/turnstile'

interface User {
  id: string
  username: string
  role: string
  email?: string
  nama?: string
}

interface LoginProps {
  onLogin: (token: string, user: User) => void
  requestFn: (
    path: string,
    token: string,
    method?: string,
    body?: unknown
  ) => Promise<any>
}

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

export function LoginView({ onLogin, requestFn }: LoginProps) {
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [turnstileToken, setTurnstileToken] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    // Surface an error passed back from the Google OAuth redirect (?google_error=...).
    const params = new URLSearchParams(window.location.search)
    const gErr = params.get('google_error')
    if (gErr) {
      setError(gErr)
      params.delete('google_error')
      params.delete('google')
      const clean = params.toString()
      window.history.replaceState({}, '', window.location.pathname + (clean ? '?' + clean : ''))
    }
  }, [])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const payload: Record<string, string> = { login, password }
      if (turnstileToken) {
        payload.turnstileToken = turnstileToken
      }

      const res = await requestFn('/auth/login', '', 'POST', payload)
      onLogin(res.accessToken, res.user)
    } catch (err: any) {
      setError(err?.message || String(err) || 'Gagal masuk. Periksa kembali username dan kata sandi Anda.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-[100dvh] items-center justify-center bg-gray-50 p-3 sm:p-4 md:p-8 dark:bg-gray-900">
      {/* TailAdmin Centered Auth Split Card */}
      <Card className="grid w-full max-w-5xl overflow-hidden rounded-xl border border-gray-200 bg-white shadow-theme-lg sm:rounded-2xl lg:grid-cols-2 dark:border-gray-800 dark:bg-gray-900">
        {/* Left Side: TailAdmin Hero Branding Panel */}
        <div className="hidden lg:flex flex-col justify-between bg-gradient-to-br from-brand-500 via-brand-600 to-brand-700 p-12 text-white relative overflow-hidden">
          <div className="absolute -top-20 -left-20 w-80 h-80 rounded-full bg-white/10 blur-2xl pointer-events-none" />
          <div className="absolute -bottom-20 -right-20 w-80 h-80 rounded-full bg-white/10 blur-2xl pointer-events-none" />

          {/* Logo Brand Header */}
          <div className="flex items-center gap-3.5 z-10">
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white text-brand-500 font-black text-xl shadow-theme-md">
              TI
            </div>
            <div>
              <h2 className="font-extrabold text-xl tracking-tight text-white">
                Tunas Ilmu Learn
              </h2>
              <p className="text-xs text-white/80 font-medium">
                PKBM Tunas Ilmu
              </p>
            </div>
          </div>

          {/* Hero Content */}
          <div className="space-y-4 my-auto z-10 py-8">
            <h1 className="text-3xl font-extrabold tracking-tight text-white leading-tight">
              Sistem Informasi Pembelajaran Terpadu & Modern.
            </h1>
            <p className="text-sm text-white/80 leading-relaxed">
              Kelola data rombel, presensi mingguan, tanda tangan digital tutor, dan laporan nilai peserta didik dengan antarmuka Tunas Ilmu Learn.
            </p>

            <div className="flex items-center gap-3 pt-4">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-white/15 backdrop-blur-sm text-white">
                <ShieldCheck className="h-5 w-5" />
              </div>
              <div>
                <p className="text-xs font-bold text-white">Akses Terlindungi</p>
                <p className="text-[11px] text-white/70">Otentikasi aman multi-peran administrator & tutor.</p>
              </div>
            </div>
          </div>

          {/* Footer Info */}
          <div className="text-xs text-white/70 font-medium z-10">
            © {new Date().getFullYear()} PKBM Tunas Ilmu • Tunas Ilmu Learn
          </div>
        </div>

        {/* Right Side: TailAdmin Form Elements */}
        <div className="flex flex-col justify-center space-y-5 p-5 sm:p-8 md:space-y-6 md:p-12">
          <div className="space-y-2">
            <span className="text-xs font-bold uppercase tracking-wider text-brand-500">
              Masuk Sesi
            </span>
            <h2 className="text-xl font-bold tracking-tight text-gray-900 sm:text-2xl md:text-3xl dark:text-white/90">
              Masuk ke Tunas Ilmu Learn
            </h2>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Masukkan akun institusi Anda untuk mengakses Dashboard LMS PKBM.
            </p>
          </div>

          <form onSubmit={submit} className="space-y-5">
            {/* Google SSO */}
            <Button
              type="button"
              variant="outline"
              className="w-full h-12 font-medium text-sm flex items-center justify-center gap-3"
              onClick={() => {
                window.location.href = apiBase + '/auth/google'
              }}
            >
              <GoogleIcon className="h-5 w-5" />
              Masuk dengan Google
            </Button>

            <div className="relative my-1">
              <div className="absolute inset-0 flex items-center">
                <span className="w-full border-t border-gray-200 dark:border-gray-800" />
              </div>
              <div className="relative flex justify-center text-[11px] uppercase tracking-wider">
                <span className="bg-white px-2 text-gray-400 font-semibold dark:bg-gray-900">atau</span>
              </div>
            </div>

            {/* Field 1: TailAdmin Form Element Username/Email Input */}
            <div className="space-y-2">
              <label className="text-xs font-bold text-gray-700 dark:text-gray-300 block">
                Nama Pengguna / Email
              </label>
              <div className="relative">
                <Input
                  type="text"
                  placeholder="Masukkan username atau email"
                  className="w-full h-12 pl-11"
                  value={login}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setLogin(e.target.value)}
                  required
                />
                <Mail className="absolute left-4 top-3.5 h-5 w-5 text-gray-400" />
              </div>
            </div>

            {/* Field 2: TailAdmin Form Element Password Input */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-xs font-bold text-gray-700 dark:text-gray-300 block">
                  Kata Sandi
                </label>
              </div>
              <div className="relative">
                <Input
                  type={showPassword ? 'text' : 'password'}
                  placeholder="Masukkan password"
                  className="w-full h-12 pl-11 pr-11"
                  value={password}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setPassword(e.target.value)}
                  required
                />
                <Lock className="absolute left-4 top-3.5 h-5 w-5 text-gray-400" />
                <button
                  type="button"
                  className="absolute right-3.5 top-3.5 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 focus:outline-none"
                  onClick={() => setShowPassword(!showPassword)}
                  title={showPassword ? 'Sembunyikan password' : 'Tampilkan password'}
                >
                  {showPassword ? (
                    <EyeOff className="h-5 w-5" />
                  ) : (
                    <Eye className="h-5 w-5" />
                  )}
                </button>
              </div>
            </div>

            {/* Cloudflare Turnstile */}
            <TurnstileWidget
              onSuccess={(token) => setTurnstileToken(token)}
              onError={() => setTurnstileToken('')}
              onExpire={() => setTurnstileToken('')}
            />

            {/* Error Feedback */}
            {error && (
              <Alert variant="destructive" className="py-2.5 px-3.5 rounded-lg text-xs">
                <AlertDescription className="font-semibold">{error}</AlertDescription>
              </Alert>
            )}

            {/* Submit Button TailAdmin Style */}
            <Button
              type="submit"
              className="w-full h-12 font-semibold text-sm"
              disabled={loading}
            >
              {loading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Memproses Sesi...
                </>
              ) : (
                'Masuk'
              )}
            </Button>
          </form>
        </div>
      </Card>
    </div>
  )
}

function GoogleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 48 48" aria-hidden="true">
      <path fill="#FFC107" d="M43.6 20.5H42V20H24v8h11.3c-1.6 4.7-6.1 8-11.3 8-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.9 1.2 8 3.1l5.7-5.7C34.9 6.5 29.8 4.5 24 4.5 13.2 4.5 4.5 13.2 4.5 24S13.2 43.5 24 43.5 43.5 34.8 43.5 24c0-1.2-.1-2.3-.4-3.5z" />
      <path fill="#FF3D00" d="M6.3 14.7l6.6 4.8C14.7 16 19 13 24 13c3.1 0 5.9 1.2 8 3.1l5.7-5.7C34.9 6.5 29.8 4.5 24 4.5 16.3 4.5 9.7 8.9 6.3 14.7z" />
      <path fill="#4CAF50" d="M24 43.5c5.8 0 10.8-2 14.6-5.4l-6.8-5.7c-2 1.5-4.6 2.4-7.8 2.4-5.2 0-9.6-3.3-11.3-7.9l-6.6 5.1C9.6 39 16.3 43.5 24 43.5z" />
      <path fill="#1976D2" d="M43.6 20.5H42V20H24v8h11.3c-.8 2.2-2.2 4.1-4 5.5l6.8 5.7c-.5.4 7.4-5.4 7.4-15.2 0-1.2-.1-2.3-.4-3.5z" />
    </svg>
  )
}
