// Fetch wrapper terpusat untuk semua panggilan API backend.
// Dipindahkan dari App.tsx agar tidak menimbulkan import sirkular
// (App mengimpor halaman via React.lazy, halaman mengimpor request →
// jika request tetap di App, siklus App↔halaman terbentuk).
//
// `signal` opsional memungkinkan pembatalan fetch (mis. saat unmount
// atau ganti halaman) agar tidak ada race / leak di state yang sudah lepas.

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

export async function request(
  path: string,
  token: string,
  method = 'GET',
  body?: unknown,
  signal?: AbortSignal,
) {
  const r = await fetch(apiBase + path, {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
    signal,
  })
  if (!r.ok) {
    const x = await r.json().catch(() => ({}))
    throw new Error(
      x.error ||
        x.message ||
        `Permintaan gagal (${r.status}). Periksa kembali koneksi atau kredensial Anda.`,
    )
  }
  return r.status === 204 ? null : r.json()
}

// downloadFile mengunduh respons biner (xlsx/pdf/csv/file) dari endpoint
// terproteksi, lalu memicu unduhan browser. Nama file diambil dari header
// Content-Disposition bila ada, fallback ke argumen `fallback`.
export async function downloadFile(path: string, token: string, fallback: string) {
  const r = await fetch(apiBase + path, {
    credentials: 'include',
    headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}) },
  })
  if (!r.ok) {
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