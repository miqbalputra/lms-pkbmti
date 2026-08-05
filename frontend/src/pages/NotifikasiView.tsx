import { useEffect, useState } from 'react'
import { Bell, Check, CheckCheck } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { EmptyState, PageToolbar } from '../components/ui/page'
import { request } from '../lib/api'

type Notif = Record<string, unknown> & { id: string }

function fmtTime(v: unknown): string {
  if (!v) return ''
  const d = new Date(String(v))
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return 'Baru saja'
  if (diff < 3600000) return `${Math.floor(diff / 60000)} menit lalu`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} jam lalu`
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
}

const tipeColor: Record<string, string> = {
  ujian: 'bg-blue-50 text-blue-600',
  tugas: 'bg-amber-50 text-amber-600',
  presensi: 'bg-green-50 text-green-600',
  rapor: 'bg-purple-50 text-purple-600',
  umum: 'bg-gray-50 text-gray-600',
}

export function NotifikasiView({ token }: { token: string }) {
  const [notifs, setNotifs] = useState<Notif[]>([])
  const [unread, setUnread] = useState(0)
  const [loading, setLoading] = useState(true)

  const load = () => {
    setLoading(true)
    Promise.all([
      request('/notifikasi', token).then((d) => setNotifs(Array.isArray(d) ? d : [])),
      request('/notifikasi/unread-count', token).then((d) => setUnread(Number(d.count || 0))),
    ])
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
  }, [token])

  const tandaiBaca = (id: string) => {
    request(`/notifikasi/${id}/baca`, token, 'PUT')
      .then(() => {
        setNotifs((prev) =>
          prev.map((n) => (n.id === id ? { ...n, isRead: true } : n))
        )
        setUnread((u) => Math.max(0, u - 1))
      })
      .catch(() => {})
  }

  const tandaiSemua = () => {
    request('/notifikasi/baca-all', token, 'PUT')
      .then(() => {
        setNotifs((prev) => prev.map((n) => ({ ...n, isRead: true })))
        setUnread(0)
      })
      .catch(() => {})
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Pusat Notifikasi"
        description={`${unread} notifikasi belum dibaca.`}
        actions={
          unread > 0 ? (
            <Button variant="outline" onClick={tandaiSemua}>
              <CheckCheck className="h-4 w-4" /> Tandai Semua Dibaca
            </Button>
          ) : undefined
        }
      />

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground">
            <div className="h-4 w-4 rounded-full border-2 border-primary border-t-transparent animate-spin" />
            Memuat notifikasi...
          </div>
        ) : notifs.length === 0 ? (
          <EmptyState
            icon={<Bell className="h-12 w-12 text-muted-foreground/40" />}
            title="Tidak ada notifikasi"
            description="Anda tidak memiliki notifikasi saat ini."
          />
        ) : (
          <div className="divide-y divide-border">
            {notifs.map((n) => (
              <div
                key={n.id}
                className={`flex items-start gap-3 p-4 transition-colors ${
                  n.isRead ? 'bg-background' : 'bg-primary/5'
                }`}
              >
                <div
                  className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-bold ${
                    tipeColor[String(n.tipe)] || 'bg-gray-50 text-gray-600'
                  }`}
                >
                  {String(n.tipe).charAt(0).toUpperCase()}
                </div>
                <div className="flex-1 min-w-0">
                  <p className={`text-sm ${n.isRead ? 'font-normal' : 'font-semibold text-foreground'}`}>
                    {String(n.judul)}
                  </p>
                  <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">
                    {String(n.isi)}
                  </p>
                  <p className="text-[10px] text-muted-foreground/60 mt-1 font-mono">
                    {fmtTime(n.createdAt)}
                  </p>
                </div>
                {!n.isRead && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => tandaiBaca(n.id)}
                    className="shrink-0"
                  >
                    <Check className="h-3 w-3" />
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
