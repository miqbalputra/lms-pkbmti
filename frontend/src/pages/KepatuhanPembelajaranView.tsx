import { useEffect, useState } from 'react'
import { ArrowRight, CalendarCheck, ClipboardList, Filter, School, Users } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import type { User } from '../App'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { EmptyState, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs'
import { request } from '../lib/api'
import { pathFor } from '../lib/router'
import { formatWibDate } from '../lib/wib'
import { KepatuhanDetailReports } from './KepatuhanDetailReports'

type Option = { id: string; label: string }

type Filters = {
  tahunAjaranId: string
  semesterId: string
  tutorId: string
  kelasId: string
  status: string
}

type Summary = {
  presensiTertunda: number
  jurnalTertunda: number
  totalTertunda: number
  tutorDenganTugas: number
  kelasDenganTugas: number
}

type ClassSummary = {
  classId: string
  classLabel: string
  tutorId: string
  tutorName: string
  presensiTertunda: number
  jurnalTertunda: number
  totalTertunda: number
  tanggalTertua?: string
}

type Task = {
  type: 'presensi' | 'jurnal'
  classId: string
  classLabel: string
  tutorId: string
  tutorName: string
  date: string
  actualDate?: string
  reason: string
  meetingId?: string
}

type ComplianceData = {
  tahunAjaran: Option
  semester: Option
  filters: Filters
  summary: Summary
  ringkasanKelas: ClassSummary[]
  tasks: Task[]
  options: {
    tahunAjaran: Option[]
    semester: Option[]
    tutor: Option[]
    kelas: Option[]
  }
}

const emptyData: ComplianceData = {
  tahunAjaran: { id: '', label: '' },
  semester: { id: '', label: '' },
  filters: { tahunAjaranId: '', semesterId: '', tutorId: '', kelasId: '', status: 'all' },
  summary: { presensiTertunda: 0, jurnalTertunda: 0, totalTertunda: 0, tutorDenganTugas: 0, kelasDenganTugas: 0 },
  ringkasanKelas: [],
  tasks: [],
  options: { tahunAjaran: [], semester: [], tutor: [], kelas: [] },
}

function formatDate(value: string) {
  return value ? formatWibDate(value) : '—'
}

function taskLabel(type: Task['type']) {
  return type === 'presensi' ? 'Presensi Kelas' : 'Jurnal Kelas'
}

export function KepatuhanPembelajaranView({ token, user }: { token: string; user: User }) {
  const navigate = useNavigate()
  const [data, setData] = useState<ComplianceData>(emptyData)
  const [filters, setFilters] = useState<Filters>({ ...emptyData.filters })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [tab, setTab] = useState<'ringkasan' | 'presensi' | 'jurnal'>('ringkasan')

  useEffect(() => {
    const controller = new AbortController()
    const params = new URLSearchParams()
    if (filters.tahunAjaranId) params.set('tahunAjaranId', filters.tahunAjaranId)
    if (filters.semesterId) params.set('semesterId', filters.semesterId)
    if (filters.tutorId) params.set('tutorId', filters.tutorId)
    if (filters.kelasId) params.set('kelasId', filters.kelasId)
    if (filters.status !== 'all') params.set('status', filters.status)
    const suffix = params.toString()

    setLoading(true)
    setError('')
    void request(`/dashboard/kepatuhan-pembelajaran${suffix ? `?${suffix}` : ''}`, token, 'GET', undefined, controller.signal)
      .then((result) => {
        if (!controller.signal.aborted) setData(result as ComplianceData)
      })
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) {
          setData(emptyData)
          setError(reason instanceof Error ? reason.message : 'Data kepatuhan belum dapat dimuat.')
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [filters.kelasId, filters.semesterId, filters.status, filters.tahunAjaranId, filters.tutorId, token])

  function updateYear(tahunAjaranId: string) {
    setFilters((current) => ({ ...current, tahunAjaranId, semesterId: '', tutorId: '', kelasId: '' }))
  }

  function updateTutor(tutorId: string) {
    setFilters((current) => ({ ...current, tutorId, kelasId: '' }))
  }

  function resetFilters() {
    setFilters({ ...emptyData.filters })
  }

  function openTask(task: Task) {
    const params = new URLSearchParams()
    if (task.type === 'presensi') {
      if (task.meetingId) params.set('presensiId', task.meetingId)
      else {
        params.set('kelasId', task.classId)
        params.set('tanggal', task.date)
      }
      navigate(`${pathFor('presensi')}?${params.toString()}`)
      return
    }
    params.set('kelasId', task.classId)
    params.set('tanggal', task.date)
    // Admin can add a journal on behalf of its wali kelas. Kepala sekolah
    // remains read-only in the destination page, preserving data ownership.
    if (user.role === 'admin') params.set('tutorId', task.tutorId)
    navigate(`${pathFor('jurnal-mengajar')}?${params.toString()}`)
  }

  const period = [data.tahunAjaran.label, data.semester.label].filter(Boolean).join(' · ')
  const kpis = [
    { label: 'Total tugas tertunda', value: data.summary.totalTertunda, icon: ClipboardList, tone: 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300' },
    { label: 'Presensi Kelas', value: data.summary.presensiTertunda, icon: CalendarCheck, tone: 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-300' },
    { label: 'Jurnal Kelas', value: data.summary.jurnalTertunda, icon: ClipboardList, tone: 'bg-violet-50 text-violet-700 dark:bg-violet-500/10 dark:text-violet-300' },
    { label: 'Wali kelas perlu tindak lanjut', value: data.summary.tutorDenganTugas, icon: Users, tone: 'bg-rose-50 text-rose-700 dark:bg-rose-500/10 dark:text-rose-300' },
  ]

  return (
    <div className="space-y-6">
      <PageToolbar
        title="Kepatuhan Pembelajaran"
        description={period ? `Pemantauan Presensi Kelas dan Jurnal Kelas · ${period}` : 'Pemantauan Presensi Kelas dan Jurnal Kelas.'}
        actions={
          <Button variant="outline" size="sm" onClick={resetFilters} disabled={loading}>
            Reset filter
          </Button>
        }
      />

      <Tabs value={tab} onValueChange={(value) => setTab(value as 'ringkasan' | 'presensi' | 'jurnal')}>
        <TabsList className="grid h-auto w-full grid-cols-3 sm:w-auto sm:inline-grid">
          <TabsTrigger value="ringkasan">Ringkasan</TabsTrigger>
          <TabsTrigger value="presensi">Detail Presensi</TabsTrigger>
          <TabsTrigger value="jurnal">Detail Jurnal</TabsTrigger>
        </TabsList>

        <TabsContent value="ringkasan" className="space-y-6">
      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs sm:p-5">
        <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-foreground">
          <Filter className="h-4 w-4 text-primary" /> Filter pemantauan
        </div>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <Select aria-label="Tahun ajaran" value={filters.tahunAjaranId} onChange={(event) => updateYear(event.target.value)}>
            <option value="">Tahun ajaran aktif</option>
            {data.options.tahunAjaran.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
          </Select>
          <Select aria-label="Semester" value={filters.semesterId} onChange={(event) => setFilters((current) => ({ ...current, semesterId: event.target.value }))}>
            <option value="">Semester aktif</option>
            {data.options.semester.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
          </Select>
          <Select aria-label="Wali kelas" value={filters.tutorId} onChange={(event) => updateTutor(event.target.value)}>
            <option value="">Semua wali kelas</option>
            {data.options.tutor.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
          </Select>
          <Select aria-label="Kelas" value={filters.kelasId} onChange={(event) => setFilters((current) => ({ ...current, kelasId: event.target.value }))}>
            <option value="">Semua kelas</option>
            {data.options.kelas.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
          </Select>
          <Select aria-label="Status tugas" value={filters.status} onChange={(event) => setFilters((current) => ({ ...current, status: event.target.value }))}>
            <option value="all">Semua tugas</option>
            <option value="presensi">Presensi Kelas</option>
            <option value="jurnal">Jurnal Kelas</option>
          </Select>
        </div>
      </Card>

      {error && (
        <div className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {kpis.map((kpi) => {
          const Icon = kpi.icon
          return (
            <Card key={kpi.label} className="rounded-2xl border border-border bg-card p-5 shadow-2xs">
              {loading ? (
                <div className="space-y-3 animate-pulse"><div className="h-10 w-10 rounded-xl bg-muted" /><div className="h-7 w-14 rounded bg-muted" /><div className="h-3 w-32 rounded bg-muted" /></div>
              ) : (
                <>
                  <div className={`flex h-10 w-10 items-center justify-center rounded-xl ${kpi.tone}`}><Icon className="h-5 w-5" /></div>
                  <p className="mt-4 text-3xl font-extrabold tracking-tight text-foreground">{kpi.value}</p>
                  <p className="mt-1 text-xs font-medium text-muted-foreground">{kpi.label}</p>
                </>
              )}
            </Card>
          )
        })}
      </div>

      <Card className="rounded-2xl border border-border bg-card shadow-2xs">
        <div className="flex flex-col gap-1 border-b border-border/70 px-5 py-4 sm:px-6">
          <h3 className="flex items-center gap-2 text-sm font-bold text-foreground"><School className="h-4 w-4 text-primary" /> Ringkasan per kelas</h3>
          <p className="text-xs text-muted-foreground">Jumlah tugas yang belum lengkap dan tanggal tugas paling lama pada kelas wali.</p>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Kelas</TableHead>
              <TableHead>Wali kelas</TableHead>
              <TableHead className="text-center">Presensi</TableHead>
              <TableHead className="text-center">Jurnal</TableHead>
              <TableHead>Tertua</TableHead>
              <TableHead>Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? Array.from({ length: 4 }).map((_, index) => <TableRow key={index}><TableCell colSpan={6}><div className="h-4 w-full animate-pulse rounded bg-muted" /></TableCell></TableRow>) : data.ringkasanKelas.length === 0 ? (
              <EmptyState colSpan={6} label="Tidak ada kelas wali pada filter ini." />
            ) : data.ringkasanKelas.map((row) => (
              <TableRow key={row.classId}>
                <TableCell className="font-semibold text-foreground">{row.classLabel}</TableCell>
                <TableCell>{row.tutorName}</TableCell>
                <TableCell className="text-center">{row.presensiTertunda}</TableCell>
                <TableCell className="text-center">{row.jurnalTertunda}</TableCell>
                <TableCell>{row.tanggalTertua ? formatDate(row.tanggalTertua) : '—'}</TableCell>
                <TableCell>
                  <span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ${row.totalTertunda > 0 ? 'bg-amber-100 text-amber-800 dark:bg-amber-500/15 dark:text-amber-300' : 'bg-emerald-100 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-300'}`}>
                    {row.totalTertunda > 0 ? `${row.totalTertunda} tertunda` : 'Lengkap'}
                  </span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>

      <Card className="rounded-2xl border border-border bg-card shadow-2xs">
        <div className="flex flex-col gap-1 border-b border-border/70 px-5 py-4 sm:px-6">
          <h3 className="flex items-center gap-2 text-sm font-bold text-foreground"><ClipboardList className="h-4 w-4 text-primary" /> Tugas tertunda</h3>
          <p className="text-xs text-muted-foreground">Buka data terkait untuk melihat atau melengkapi tugas tanpa alur persetujuan.</p>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Jenis</TableHead>
              <TableHead>Kelas</TableHead>
              <TableHead>Wali kelas</TableHead>
              <TableHead>Tanggal</TableHead>
              <TableHead>Keterangan</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? Array.from({ length: 5 }).map((_, index) => <TableRow key={index}><TableCell colSpan={6}><div className="h-4 w-full animate-pulse rounded bg-muted" /></TableCell></TableRow>) : data.tasks.length === 0 ? (
              <EmptyState colSpan={6} title="Tidak ada tugas tertunda" description="Presensi Kelas dan Jurnal Kelas pada filter ini sudah lengkap." icon={<CalendarCheck className="h-8 w-8 text-emerald-600" />} />
            ) : data.tasks.map((task) => (
              <TableRow key={`${task.type}-${task.classId}-${task.date}`}>
                <TableCell>
                  <span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ${task.type === 'presensi' ? 'bg-blue-100 text-blue-800 dark:bg-blue-500/15 dark:text-blue-300' : 'bg-violet-100 text-violet-800 dark:bg-violet-500/15 dark:text-violet-300'}`}>
                    {taskLabel(task.type)}
                  </span>
                </TableCell>
                <TableCell className="font-semibold text-foreground">{task.classLabel}</TableCell>
                <TableCell>{task.tutorName}</TableCell>
                <TableCell>{formatDate(task.date)}</TableCell>
                <TableCell className="max-w-xs text-xs text-muted-foreground">{task.reason}{task.actualDate ? ` · dilaksanakan ${formatDate(task.actualDate)}` : ''}</TableCell>
                <TableCell className="text-right">
                  <Button size="sm" variant="outline" onClick={() => openTask(task)}>
                    {user.role === 'admin' ? 'Buka formulir' : 'Buka data'} <ArrowRight className="h-3.5 w-3.5" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
        </TabsContent>

        <TabsContent value="presensi" className="space-y-6">
          <KepatuhanDetailReports
            token={token}
            kind="presensi"
            active={tab === 'presensi'}
            filters={filters}
          />
        </TabsContent>

        <TabsContent value="jurnal" className="space-y-6">
          <KepatuhanDetailReports
            token={token}
            kind="jurnal"
            active={tab === 'jurnal'}
            filters={filters}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}
