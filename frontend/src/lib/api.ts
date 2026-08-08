// Fetch wrapper terpusat untuk semua panggilan API backend.
// Dipindahkan dari App.tsx agar tidak menimbulkan import sirkular
// (App mengimpor halaman via React.lazy, halaman mengimpor request →
// jika request tetap di App, siklus App↔halaman terbentuk).
//
// `signal` opsional memungkinkan pembatalan fetch (mis. saat unmount
// atau ganti halaman) agar tidak ada race / leak di state yang sudah lepas.

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

let onUnauthorized: (() => void) | null = null
let onTokenRefreshed: ((session: AuthSession) => void) | null = null
let refreshInFlight: Promise<AuthSession> | null = null

export type AuthSession = {
  accessToken: string
  user: Record<string, unknown>
}

export function setOnUnauthorized(fn: (() => void) | null) { onUnauthorized = fn }
export function setOnTokenRefreshed(fn: ((session: AuthSession) => void) | null) { onTokenRefreshed = fn }

// Refresh-token rotation is deliberately single-flight. Without this guard,
// two requests/tabs can rotate the same cookie at once; the second request
// receives 401 and the old client treated that as an intentional logout.
export function refreshSession(): Promise<AuthSession> {
  if (!refreshInFlight) {
    refreshInFlight = fetch(apiBase + '/auth/refresh', {
      method: 'POST',
      credentials: 'include',
    })
      .then(async (response) => {
        const result = await response.json().catch(() => ({}))
        if (!response.ok) {
          throw new Error(
            result?.error || result?.message || `Sesi tidak dapat diperbarui (${response.status}).`,
          )
        }
        return result as AuthSession
      })
      .finally(() => {
        refreshInFlight = null
      })
  }
  return refreshInFlight
}

function isAuthEndpoint(path: string) {
  return path.startsWith('/auth/')
}

function fallbackRequestError(status: number) {
  if (status === 413) {
    return 'Ukuran foto terlalu besar untuk dikirim. Pilih ulang foto agar dikompres otomatis.'
  }
  if (status === 502 || status === 503 || status === 504) {
    return `Server belum dapat memproses permintaan (${status}). Silakan coba simpan kembali.`
  }
  return `Permintaan gagal (${status}). Periksa kembali koneksi atau data yang diisi.`
}

async function fetchWithToken(
  path: string,
  token: string,
  method: string,
  body: unknown,
  signal?: AbortSignal,
) {
  return fetch(apiBase + path, {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
    signal,
  })
}

export async function request(
  path: string,
  token: string,
  method = 'GET',
  body?: unknown,
  signal?: AbortSignal,
) {
  let r = await fetchWithToken(path, token, method, body, signal)
  // A request may have started with an access token that was replaced by the
  // keep-alive refresh. Renew once and replay it before asking the app to log
  // out. Auth endpoints themselves are excluded to avoid refresh recursion.
  if (r.status === 401 && token && !isAuthEndpoint(path)) {
    let refreshFailed = false
    try {
      const session = await refreshSession()
      onTokenRefreshed?.(session)
      r = await fetchWithToken(path, session.accessToken, method, body, signal)
    } catch {
      refreshFailed = true
      if (onUnauthorized) onUnauthorized()
    }
    if (refreshFailed) {
      const x = await r.json().catch(() => ({}))
      throw new Error(x.error || x.message || `Permintaan gagal (${r.status}).`)
    }
  }
  if (!r.ok) {
    if (r.status === 401 && onUnauthorized && !isAuthEndpoint(path)) onUnauthorized()
    const x = await r.json().catch(() => ({}))
    throw new Error(
      x.error ||
        x.message ||
        fallbackRequestError(r.status),
    )
  }
  return r.status === 204 ? null : r.json()
}

// downloadFile mengunduh respons biner (xlsx/pdf/csv/file) dari endpoint
// terproteksi, lalu memicu unduhan browser. Nama file diambil dari header
// Content-Disposition bila ada, fallback ke argumen `fallback`.
export async function downloadFile(path: string, token: string, fallback: string) {
  let r = await fetch(apiBase + path, {
    credentials: 'include',
    headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}) },
  })
  if (r.status === 401 && token && !isAuthEndpoint(path)) {
    try {
      const session = await refreshSession()
      onTokenRefreshed?.(session)
      r = await fetch(apiBase + path, {
        credentials: 'include',
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
    } catch {
      // The final 401 branch below handles the user-facing logout path.
    }
  }
  if (!r.ok) {
    if (r.status === 401 && onUnauthorized && !isAuthEndpoint(path)) onUnauthorized()
    const x = await r.json().catch(() => ({}))
    throw new Error((x as { error?: string; message?: string })?.error || (x as { message?: string })?.message || `Gagal mengunduh (${r.status})`)
  }
  const blob = await r.blob()
  const cd = r.headers.get('Content-Disposition') || ''
  const m = /filename="?([^";]+)"?/.exec(cd)
  const name = m?.[1] || fallback
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export { apiBase }
