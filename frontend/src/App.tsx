import { Component, type ReactNode, useCallback, useEffect, useState, lazy, Suspense } from 'react'
import {
  Bell,
  CalendarCheck,
  CalendarClock,
  School,
  UserCheck,
  Users,
} from 'lucide-react'
import { Alert, AlertDescription } from './components/ui/alert'
import { Card, CardContent } from './components/ui/card'
import { Toaster } from './components/ui/sonner'
import { AppShell } from './components/layout/AppShell'
import { LoginView } from './pages/Login'
import { InstallPrompt } from './components/InstallPrompt'
import { TutorEmailPrompt } from './components/TutorEmailPrompt'
import { refreshSession, request, setOnTokenRefreshed, setOnUnauthorized } from './lib/api'

// Re-export agar halaman yang masih mengimpor { request } from '../App' tetap
// berfungsi (sumber kebenaran kini di ./lib/api, tanpa import sirkular).
export { request }

class ErrorBoundary extends Component<{ children: ReactNode }, { error: string | null }> {
  state = { error: null as string | null }
  static getDerivedStateFromError(err: unknown) {
    return { error: err instanceof Error ? err.message : String(err) }
  }
  render() {
    if (this.state.error) {
      return (
        <div className="p-6">
          <div className="rounded-md bg-destructive/10 border border-destructive/30 p-4">
            <p className="text-sm font-medium text-destructive">Terjadi kesalahan saat menampilkan halaman.</p>
            <p className="text-xs text-muted-foreground mt-1">{this.state.error}</p>
            <button className="text-xs underline mt-2" onClick={() => this.setState({ error: null })}>Coba lagi</button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

// Code-splitting: semua halaman kerja di-lazy-load agar bundle awal ringan.
// Tiap halaman jadi chunk terpisah yang baru diunduh saat pertama dibuka.
const Accounts = lazy(() => import('./Accounts').then((m) => ({ default: m.Accounts })))
const AttendanceWorkspace = lazy(() => import('./Attendance').then((m) => ({ default: m.AttendanceWorkspace })))
const AuditLogs = lazy(() => import('./AuditLogs').then((m) => ({ default: m.AuditLogs })))
const ClassSubjects = lazy(() => import('./ClassSubjects').then((m) => ({ default: m.ClassSubjects })))
const MasterData = lazy(() => import('./MasterData').then((m) => ({ default: m.MasterData })))
const Nilai = lazy(() => import('./Nilai').then((m) => ({ default: m.Nilai })))
const PengaturanNilai = lazy(() => import('./PengaturanNilai').then((m) => ({ default: m.PengaturanNilai })))
const PromotionWizard = lazy(() => import('./Promotion').then((m) => ({ default: m.PromotionWizard })))
const ScheduleSettings = lazy(() => import('./ScheduleSettings').then((m) => ({ default: m.ScheduleSettings })))
const ArchiveView = lazy(() => import('./OperationalViews').then((m) => ({ default: m.ArchiveView })))
const AssignmentsView = lazy(() => import('./OperationalViews').then((m) => ({ default: m.AssignmentsView })))
const ClassesView = lazy(() => import('./OperationalViews').then((m) => ({ default: m.ClassesView })))
const StudentsView = lazy(() => import('./OperationalViews').then((m) => ({ default: m.StudentsView })))
const BukuKelasView = lazy(() => import('./pages/BukuKelasView').then((m) => ({ default: m.BukuKelasView })))
const JurnalMengajarView = lazy(() => import('./pages/JurnalMengajarView').then((m) => ({ default: m.JurnalMengajarView })))
const KelasVirtualView = lazy(() => import('./pages/KelasVirtualView').then((m) => ({ default: m.KelasVirtualView })))
const MateriView = lazy(() => import('./pages/MateriView').then((m) => ({ default: m.MateriView })))
const RppView = lazy(() => import('./pages/RppView').then((m) => ({ default: m.RppView })))
const PeminjamanBuku = lazy(() => import('./pages/PeminjamanBuku').then((m) => ({ default: m.PeminjamanBuku })))
const PengumumanView = lazy(() => import('./pages/PengumumanView').then((m) => ({ default: m.PengumumanView })))
const RekapBuku = lazy(() => import('./pages/RekapBuku').then((m) => ({ default: m.RekapBuku })))
const TugasView = lazy(() => import('./pages/TugasView').then((m) => ({ default: m.TugasView })))
const BankSoalView = lazy(() => import('./pages/BankSoalView').then((m) => ({ default: m.BankSoalView })))
const UjianView = lazy(() => import('./pages/UjianView').then((m) => ({ default: m.UjianView })))
const SertifikatView = lazy(() => import('./pages/SertifikatView').then((m) => ({ default: m.SertifikatView })))
const KartuPelajarView = lazy(() => import('./pages/KartuPelajarView').then((m) => ({ default: m.KartuPelajarView })))
const PerilakuView = lazy(() => import('./pages/PerilakuView').then((m) => ({ default: m.PerilakuView })))
const RaporView = lazy(() => import('./pages/RaporView').then((m) => ({ default: m.RaporView })))
const SumberNilaiView = lazy(() => import('./pages/SumberNilaiView').then((m) => ({ default: m.SumberNilaiView })))
const ModulBelajarView = lazy(() => import('./pages/ModulBelajarView').then((m) => ({ default: m.ModulBelajarView })))
const KompetensiView = lazy(() => import('./pages/KompetensiView').then((m) => ({ default: m.KompetensiView })))
const NilaiKompetensiView = lazy(() => import('./pages/NilaiKompetensiView').then((m) => ({ default: m.NilaiKompetensiView })))
const LaporanView = lazy(() => import('./pages/LaporanView').then((m) => ({ default: m.LaporanView })))
const ImportView = lazy(() => import('./pages/ImportView').then((m) => ({ default: m.ImportView })))
const TutorDocumentsView = lazy(() => import('./pages/TutorDocumentsView').then((m) => ({ default: m.TutorDocumentsView })))
const SuratSiswaView = lazy(() => import('./pages/SuratSiswaView').then((m) => ({ default: m.SuratSiswaView })))
const RelasiOrangTua = lazy(() => import('./pages/RelasiOrangTua').then((m) => ({ default: m.RelasiOrangTua })))
const BackupView = lazy(() => import('./pages/BackupView').then((m) => ({ default: m.BackupView })))
const UjianOnlineView = lazy(() => import('./pages/UjianOnlineView').then((m) => ({ default: m.UjianOnlineView })))
const UjianMonitorView = lazy(() => import('./pages/UjianMonitorView').then((m) => ({ default: m.UjianMonitorView })))
const NotifikasiView = lazy(() => import('./pages/NotifikasiView').then((m) => ({ default: m.NotifikasiView })))
const KalenderView = lazy(() => import('./pages/KalenderView').then((m) => ({ default: m.KalenderView })))
const AnalyticsView = lazy(() => import('./pages/AnalyticsView').then((m) => ({ default: m.AnalyticsView })))
const DashboardCharts = lazy(() =>
  import('./DashboardCharts').then((m) => ({ default: m.DashboardCharts }))
)

export type User = { id: string; username: string; role: string; tutorId?: string; email?: string; nama?: string }

function PageFallback() {
  return (
    <div className="grid place-items-center py-20 text-sm text-muted-foreground">
      <div className="flex items-center gap-2 font-medium">
        <div className="h-4 w-4 rounded-full border-2 border-primary border-t-transparent animate-spin" />
        Memuat halaman...
      </div>
    </div>
  )
}

export default function App() {
  const [token, setToken] = useState('')
  const [user, setUser] = useState<User | null>(null)
  const [ready, setReady] = useState(false)
  const [page, setPage] = useState('dashboard')
  const [tutorAccountOpen, setTutorAccountOpen] = useState(false)

  useEffect(() => {
    void refreshSession()
      .then((r) => {
        setToken(r.accessToken)
        setUser(r.user as User)
      })
      .catch(() => undefined)
      .finally(() => setReady(true))
  }, [])

  const handleLogout = useCallback(() => {
    void request('/auth/logout', token, 'POST').catch(() => undefined)
    setToken('')
    setUser(null)
    setTutorAccountOpen(false)
  }, [token])

  useEffect(() => {
    setOnTokenRefreshed((session) => {
      setToken(session.accessToken)
      setUser(session.user as User)
    })
    if (token) {
      setOnUnauthorized(handleLogout)
    } else {
      setOnUnauthorized(null)
    }
    return () => {
      setOnUnauthorized(null)
      setOnTokenRefreshed(null)
    }
  }, [handleLogout, token])

  // Keep-alive: refresh before the normal 15-minute access-token expiry.
  // refreshSession() is single-flight, so a transient race cannot log the
  // admin out while they are typing or working in multiple requests.
  useEffect(() => {
    if (!token) return
    const REFRESH_MS = 10 * 60 * 1000
    const timer = setInterval(() => {
      void refreshSession()
        .then((r) => {
          setToken(r.accessToken)
          setUser(r.user as User)
        })
        // Keep the current session on a transient network error. A later API
        // call will retry refresh; only an actual unauthorized API response
        // ends the session.
        .catch(() => undefined)
    }, REFRESH_MS)

    return () => {
      clearInterval(timer)
    }
  }, [token])

  if (!ready) {
    return (
      <div className="grid min-h-screen place-items-center bg-background text-sm text-muted-foreground">
        <div className="flex items-center gap-2 font-medium">
          <div className="h-4 w-4 rounded-full border-2 border-primary border-t-transparent animate-spin" />
          Memulihkan sesi aman...
        </div>
      </div>
    )
  }

  if (!token || !user) {
    return (
      <>
        <Toaster position="top-right" />
        <LoginView
          onLogin={(t, u) => {
            setToken(t)
            setUser(u)
          }}
          requestFn={request}
        />
      </>
    )
  }

  return (
    <>
      <Toaster position="top-right" />
      <InstallPrompt />
      <AppShell
        user={user}
        token={token}
        page={page}
        setPage={setPage}
        onLogout={handleLogout}
        onOpenTutorAccount={() => setTutorAccountOpen(true)}
      >
        <Suspense fallback={<PageFallback />}>
          <ErrorBoundary key={page}>
            <Workspace
              page={page}
              setPage={setPage}
              token={token}
              user={user}
              readOnly={user.role !== 'admin'}
              attendanceReadOnly={user.role === 'kepala_sekolah'}
            />
          </ErrorBoundary>
        </Suspense>
      </AppShell>
      <TutorEmailPrompt
        token={token}
        user={user}
        required={user.role === 'guru' && !String(user.email || '').trim()}
        open={tutorAccountOpen || (user.role === 'guru' && !String(user.email || '').trim())}
        onOpenChange={setTutorAccountOpen}
        onSaved={(email) => {
          setUser((current) => (current ? { ...current, email } : current))
          setTutorAccountOpen(false)
        }}
      />
    </>
  )
}

function Workspace({
  page,
  setPage,
  token,
  user,
  readOnly,
  attendanceReadOnly,
}: {
  page: string
  setPage: (p: string) => void
  token: string
  user: User
  readOnly: boolean
  attendanceReadOnly: boolean
}) {
  if (page === 'dashboard') return <Dashboard token={token} />
  if (page === 'arsip') return <ArchiveView token={token} />
  if (page === 'kenaikan-kelas') return readOnly ? <Restricted /> : <PromotionWizard token={token} />
  if (page === 'akun') return readOnly ? <Restricted /> : <Accounts token={token} />
  if (page === 'pengaturan-jadwal') return readOnly ? <Restricted /> : <ScheduleSettings token={token} />
  if (page === 'audit-log') return readOnly ? <Restricted /> : <AuditLogs token={token} />
  if (page === 'kelas-mapel') return readOnly ? <Restricted /> : <ClassSubjects token={token} />
  if (page === 'kelas') return <ClassesView token={token} readOnly={readOnly} />
  if (page === 'peserta-didik') return <StudentsView token={token} readOnly={readOnly} />
  if (page === 'relasi-orang-tua') return <RelasiOrangTua token={token} readOnly={readOnly} />
  if (page === 'penugasan') return <AssignmentsView token={token} readOnly={readOnly} />
  if (page === 'presensi') return <AttendanceWorkspace token={token} readOnly={attendanceReadOnly} userName={user?.username} />
  if (page === 'nilai') return <Nilai token={token} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'pengaturan-nilai') return readOnly ? <Restricted /> : <PengaturanNilai token={token} />
  if (page === 'buku-kelas') return readOnly ? <Restricted /> : <BukuKelasView token={token} readOnly={readOnly} />
  if (page === 'peminjaman-buku') return user.role === 'guru' ? <PeminjamanBuku token={token} readOnly={false} user={user} /> : <Restricted />
  if (page === 'rekap-buku') return <RekapBuku token={token} />
  if (page === 'pengumuman') return <PengumumanView token={token} user={user} readOnly={user.role !== 'admin' && user.role !== 'guru'} />
  if (page === 'jurnal-mengajar') return <JurnalMengajarView token={token} user={user} readOnly={user.role !== 'admin' && user.role !== 'guru'} />
  if (page === 'tugas') return <TugasView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'materi') return <MateriView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'rpp') return <RppView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'kelas-virtual') return <KelasVirtualView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'bank-soal') return <BankSoalView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'ujian') return <UjianView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} setPage={setPage} />
  if (page === 'sertifikat') return <SertifikatView token={token} readOnly={user.role !== 'admin'} />
  if (page === 'kartu-pelajar') return <KartuPelajarView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'perilaku') return <PerilakuView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'rapor') return <RaporView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'sumber-nilai') return <SumberNilaiView token={token} readOnly={user.role !== 'admin'} />
  if (page === 'modul-belajar') return <ModulBelajarView token={token} readOnly={user.role !== 'admin'} />
  if (page === 'kompetensi') return user.role === 'guru' ? <KompetensiView token={token} user={user} readOnly={false} /> : <Restricted />
  if (page === 'nilai-kompetensi') return <NilaiKompetensiView token={token} user={user} readOnly={user.role === 'kepala_sekolah'} />
  if (page === 'laporan') return <LaporanView token={token} />
  if (page === 'import') return user.role === 'admin' || user.role === 'guru' ? <ImportView token={token} user={user} /> : <Restricted />
  if (page === 'dokumen-tutor') return user.role === 'admin' || user.role === 'guru' ? <TutorDocumentsView token={token} role={user.role} /> : <Restricted />
  if (page === 'surat-siswa') return user.role === 'admin' ? <SuratSiswaView token={token} /> : <Restricted />
  if (page === 'backup') return user.role === 'admin' ? <BackupView token={token} /> : <Restricted />
  if (page === 'ujian-online') return <UjianOnlineView token={token} />
  if (page === 'ujian-monitor') return <UjianMonitorView token={token} />
  if (page === 'notifikasi') return <NotifikasiView token={token} />
  if (page === 'kalender') return <KalenderView token={token} readOnly={user.role === 'kepala_sekolah' || user.role === 'guru'} />
  if (page === 'analytics') return <AnalyticsView token={token} />
  if (page === 'portal-ortu') {
    window.open('/orangtua', '_blank')
    setPage('dashboard')
    return null
  }
  return <MasterData resource={page} token={token} readOnly={readOnly} />
}

function Restricted() {
  return (
    <Alert>
      <AlertDescription>Akses ini hanya tersedia untuk Admin.</AlertDescription>
    </Alert>
  )
}

function Dashboard({ token }: { token: string }) {
  const [data, setData] = useState<Record<string, unknown>>({})
  const [tutorsCount, setTutorsCount] = useState<number>(0)
  const [loading, setLoading] = useState<boolean>(true)

  useEffect(() => {
    const ctrl = new AbortController()
    setLoading(true)
    Promise.all([
      request('/dashboard', token, 'GET', undefined, ctrl.signal)
        .then(setData)
        .catch(() => ({})),
      request('/tutor', token, 'GET', undefined, ctrl.signal)
        .then((res) => {
          if (Array.isArray(res)) setTutorsCount(res.length)
        })
        .catch(() => setTutorsCount(0)),
    ])
      .finally(() => setLoading(false))
    return () => ctrl.abort()
  }, [token])

  const upcomingEvents = (data.upcomingEvents as { id: string; judul: string; jenis: string; tanggal_mulai: string; tanggal_selesai: string }[]) || []
  const unreadNotif = Number(data.unreadNotif) || 0

  const kpis = [
    {
      label: 'Peserta Didik Aktif',
      value: Number(data.pesertaDidik) || 0,
      icon: Users,
    },
    {
      label: 'Rombel Terdaftar',
      value: Number(data.kelas) || 0,
      icon: School,
    },
    {
      label: 'Tutor Terdaftar',
      value: tutorsCount,
      icon: UserCheck,
    },
    {
      label: 'Kehadiran Tercatat',
      value: Number(data.hadir) || 0,
      icon: CalendarCheck,
    },
    {
      label: 'Notifikasi Belum Dibaca',
      value: unreadNotif,
      icon: Bell,
    },
    {
      label: 'Acara Mendatang',
      value: upcomingEvents.length,
      icon: CalendarClock,
    },
  ]

  const eventColors: Record<string, string> = {
    ujian: 'bg-orange-100 text-orange-700',
    libur: 'bg-red-100 text-red-700',
    kegiatan: 'bg-blue-100 text-blue-700',
    ppdb: 'bg-green-100 text-green-700',
  }

  return (
    <div className="space-y-6">
      {/* Stat Cards Grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 md:gap-6">
        {loading
          ? Array.from({ length: 6 }).map((_, i) => (
              <Card key={i} className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
                <div className="h-12 w-12 bg-muted animate-pulse rounded-full" />
                <div className="h-8 w-20 bg-muted animate-pulse rounded mt-4" />
                <div className="h-4 w-28 bg-muted animate-pulse rounded mt-2" />
              </Card>
            ))
          : kpis.map((kpi) => {
              const Icon = kpi.icon
              return (
                <Card
                  key={kpi.label}
                  className="rounded-2xl border border-border bg-card p-6 shadow-2xs transition-all hover:shadow-md"
                >
                  <div className="flex h-12 w-12 items-center justify-center rounded-full bg-secondary/80 text-primary shadow-xs">
                    <Icon className="h-6 w-6" />
                  </div>
                  <div className="mt-4 flex items-end justify-between">
                    <div>
                      <h4 className="text-2xl md:text-3xl font-extrabold text-foreground tracking-tight">
                        {kpi.value}
                      </h4>
                      <span className="text-xs font-semibold text-muted-foreground block mt-0.5">
                        {kpi.label}
                      </span>
                    </div>
                  </div>
                </Card>
              )
            })}
      </div>

      {/* Upcoming Events Section */}
      {!loading && upcomingEvents.length > 0 && (
        <Card className="rounded-2xl border border-border bg-card shadow-2xs">
          <div className="px-6 py-4 border-b border-border/60">
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-xl bg-primary/10 text-primary">
                <CalendarClock className="h-4 w-4" />
              </div>
              <div>
                <h3 className="text-base font-bold text-foreground">Acara Mendatang (30 Hari)</h3>
                <p className="text-xs text-muted-foreground">Kalender akademik yang akan datang</p>
              </div>
            </div>
          </div>
          <CardContent className="p-4">
            <div className="space-y-2">
              {upcomingEvents.map((evt) => (
                <div
                  key={evt.id}
                  className="flex items-center justify-between rounded-xl border border-border/60 px-4 py-3 hover:bg-muted/50 transition-colors"
                >
                  <div className="flex items-center gap-3">
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-semibold ${eventColors[evt.jenis] || 'bg-gray-100 text-gray-700'}`}>
                      {evt.jenis}
                    </span>
                    <span className="text-sm font-medium text-foreground">{evt.judul}</span>
                  </div>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    {new Date(evt.tanggal_mulai).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })}
                    {evt.tanggal_selesai && evt.tanggal_selesai !== evt.tanggal_mulai && (
                      <> — {new Date(evt.tanggal_selesai).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })}</>
                    )}
                  </span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Charts */}
      <Suspense
        fallback={
          <Card className="rounded-2xl border p-6">
            <CardContent className="p-6 text-sm text-muted-foreground">
              Memuat grafik...
            </CardContent>
          </Card>
        }
      >
        <DashboardCharts
          perPokjar={(data.perPokjar as { label: string; total: number }[]) || []}
          perKelas={(data.perKelas as { label: string; total: number }[]) || []}
          loading={loading}
        />
      </Suspense>
    </div>
  )
}
