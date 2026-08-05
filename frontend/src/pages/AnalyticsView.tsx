import { useEffect, useState, useMemo } from 'react'
import { BarChart3, Users, School, BookOpen, CalendarCheck, Award, TrendingUp, Filter } from 'lucide-react'
import { Card, CardContent } from '../components/ui/card'
import { PageToolbar } from '../components/ui/page'
import { Button } from '../components/ui/button'
import { request } from '../lib/api'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  LineChart,
  Line,
  Legend,
} from 'recharts'

type DashboardData = Record<string, unknown>

const COLORS = ['#1c5740', '#d4af37', '#2563eb', '#dc2626', '#9333ea', '#f59e0b', '#16a34a', '#0891b2']

const SEMESTER_OPTIONS = [
  { label: 'Semua', value: '' },
  { label: 'Semester 1 (Jul–Des)', value: '1' },
  { label: 'Semester 2 (Jan–Jun)', value: '2' },
]

export function AnalyticsView({ token }: { token: string }) {
  const [data, setData] = useState<DashboardData>({})
  const [mapel, setMapel] = useState<Record<string, unknown>[]>([])
  const [kelas, setKelas] = useState<Record<string, unknown>[]>([])
  const [siswa, setSiswa] = useState<Record<string, unknown>[]>([])
  const [loading, setLoading] = useState(true)
  const [semester, setSemester] = useState('')
  const [year, setYear] = useState(new Date().getFullYear())
  const [ujian, setUjian] = useState<Record<string, unknown>[]>([])

  useEffect(() => {
    setLoading(true)
    const params = new URLSearchParams()
    if (semester) params.set('semester', semester)
    if (year) params.set('year', String(year))
    const qs = params.toString()
    const base = '/dashboard' + (qs ? '?' + qs : '')
    Promise.all([
      request(base, token).then(setData).catch(() => ({})),
      request('/mapel', token).then(d => setMapel(Array.isArray(d) ? d : [])).catch(() => {}),
      request('/kelas', token).then(d => setKelas(Array.isArray(d) ? d : [])).catch(() => {}),
      request('/peserta-didik', token).then(d => setSiswa(Array.isArray(d) ? d : [])).catch(() => {}),
      request('/ujian', token).then(d => setUjian(Array.isArray(d) ? d : [])).catch(() => {}),
    ]).finally(() => setLoading(false))
  }, [token, semester, year])

  const filteredSiswa = useMemo(() => {
    if (!semester) return siswa
    return siswa.filter(s => {
      const created = s.createdAt ? new Date(String(s.createdAt)) : null
      if (!created) return true
      const sem = semester === '1' ? [7, 8, 9, 10, 11, 12] : [1, 2, 3, 4, 5, 6]
      return created.getFullYear() === year && sem.includes(created.getMonth() + 1)
    })
  }, [siswa, semester, year])

  const perPokjar = (data.perPokjar as { label: string; total: number }[]) || []
  const perKelas = (data.perKelas as { label: string; total: number }[]) || []

  // Compute analytics using filtered siswa
  const siswaByGender = [
    { name: 'Laki-laki', value: filteredSiswa.filter(s => s.jenisKelamin === 'Laki-laki' || s.jenisKelamin === 'L').length },
    { name: 'Perempuan', value: filteredSiswa.filter(s => s.jenisKelamin === 'Perempuan' || s.jenisKelamin === 'P').length },
  ]

  const siswaByStatus = [
    { name: 'Aktif', value: filteredSiswa.filter(s => s.status === 'aktif').length },
    { name: 'Lulus', value: filteredSiswa.filter(s => s.status === 'lulus').length },
    { name: 'Nonaktif', value: filteredSiswa.filter(s => s.status !== 'aktif' && s.status !== 'lulus').length },
  ]

  const kelasSize = kelas.map(k => ({
    name: `Kelas ${String(k.jenjang || '')}${String(k.namaRombel || '')}`,
    siswa: filteredSiswa.filter(s => s.kelasId === k.id).length,
  })).sort((a, b) => b.siswa - a.siswa).slice(0, 10)

  // Ujian completion stats
  const ujianStats = useMemo(() => {
    if (!Array.isArray(ujian) || ujian.length === 0) return []
    return ujian.slice(0, 10).map((u: any) => ({
      name: String(u.judul || '-').slice(0, 20),
      soal: Number(u.jumlahSoal || 0),
    }))
  }, [ujian])

  const kpis = [
    { label: 'Total Peserta Didik', value: filteredSiswa.length, icon: Users },
    { label: 'Total Rombel', value: kelas.length, icon: School },
    { label: 'Mata Pelajaran', value: mapel.length, icon: BookOpen },
    { label: 'Kehadiran Tercatat', value: Number(data.hadir) || 0, icon: CalendarCheck },
  ]

  return (
    <div className="space-y-6">
      <PageToolbar
        title="Analytics Dashboard"
        description="Analitik dan ringkasan data LMS PKBM Tunas Ilmu."
        actions={
          <div className="flex items-center gap-2">
            <div className="flex items-center gap-2 bg-card border border-border rounded-xl px-3 py-2">
              <Filter className="h-4 w-4 text-muted-foreground" />
              <select
                value={semester}
                onChange={(e) => setSemester(e.target.value)}
                className="bg-transparent text-sm border-none outline-none text-foreground cursor-pointer"
              >
                {SEMESTER_OPTIONS.map(o => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
              <select
                value={year}
                onChange={(e) => setYear(Number(e.target.value))}
                className="bg-transparent text-sm border-none outline-none text-foreground cursor-pointer"
              >
                {Array.from({ length: 5 }, (_, i) => new Date().getFullYear() - 2 + i).map(y => (
                  <option key={y} value={y}>{y}</option>
                ))}
              </select>
            </div>
          </div>
        }
      />

      {/* KPI Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {loading
          ? Array.from({ length: 4 }).map((_, i) => (
              <Card key={i} className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
                <div className="h-12 w-12 bg-muted animate-pulse rounded-full" />
                <div className="h-8 w-20 bg-muted animate-pulse rounded mt-4" />
                <div className="h-4 w-28 bg-muted animate-pulse rounded mt-2" />
              </Card>
            ))
          : kpis.map((kpi) => {
              const Icon = kpi.icon
              return (
                <Card key={kpi.label} className="rounded-2xl border border-border bg-card p-6 shadow-2xs transition-all hover:shadow-md">
                  <div className="flex h-12 w-12 items-center justify-center rounded-full bg-secondary/80 text-primary shadow-xs">
                    <Icon className="h-6 w-6" />
                  </div>
                  <div className="mt-4">
                    <h4 className="text-2xl md:text-3xl font-extrabold text-foreground tracking-tight">{kpi.value}</h4>
                    <span className="text-xs font-semibold text-muted-foreground block mt-0.5">{kpi.label}</span>
                  </div>
                </Card>
              )
            })}
      </div>

      {/* Charts Grid */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Siswa by Kelas */}
        <Card className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
          <h3 className="text-sm font-bold text-foreground mb-4 flex items-center gap-2">
            <BarChart3 className="h-4 w-4 text-primary" /> Distribusi Siswa per Rombel
          </h3>
          {loading ? (
            <div className="h-64 bg-muted animate-pulse rounded-lg" />
          ) : kelasSize.length === 0 ? (
            <div className="h-64 flex items-center justify-center text-sm text-muted-foreground">Tidak ada data</div>
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={kelasSize}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
                <XAxis dataKey="name" tick={{ fontSize: 10 }} angle={-30} textAnchor="end" height={60} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Bar dataKey="siswa" fill="#1c5740" radius={[6, 6, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </Card>

        {/* Siswa by Gender (Pie) */}
        <Card className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
          <h3 className="text-sm font-bold text-foreground mb-4 flex items-center gap-2">
            <Users className="h-4 w-4 text-primary" /> Komposisi Jenis Kelamin
          </h3>
          {loading ? (
            <div className="h-64 bg-muted animate-pulse rounded-lg" />
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <PieChart>
                <Pie
                  data={siswaByGender}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={100}
                  paddingAngle={5}
                  dataKey="value"
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                >
                  {siswaByGender.map((_, i) => (
                    <Cell key={i} fill={COLORS[i % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          )}
        </Card>

        {/* Siswa by Pokjar (Bar) */}
        <Card className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
          <h3 className="text-sm font-bold text-foreground mb-4 flex items-center gap-2">
            <School className="h-4 w-4 text-primary" /> Siswa per Pokjar
          </h3>
          {loading ? (
            <div className="h-64 bg-muted animate-pulse rounded-lg" />
          ) : perPokjar.length === 0 ? (
            <div className="h-64 flex items-center justify-center text-sm text-muted-foreground">Tidak ada data</div>
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={perPokjar}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
                <XAxis dataKey="label" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Bar dataKey="total" fill="#d4af37" radius={[6, 6, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </Card>

        {/* Siswa by Status (Pie) */}
        <Card className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
          <h3 className="text-sm font-bold text-foreground mb-4 flex items-center gap-2">
            <TrendingUp className="h-4 w-4 text-primary" /> Status Peserta Didik
          </h3>
          {loading ? (
            <div className="h-64 bg-muted animate-pulse rounded-lg" />
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <PieChart>
                <Pie
                  data={siswaByStatus}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={100}
                  paddingAngle={5}
                  dataKey="value"
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                >
                  {siswaByStatus.map((_, i) => (
                    <Cell key={i} fill={COLORS[i % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          )}
        </Card>
      </div>
    </div>
  )
}
