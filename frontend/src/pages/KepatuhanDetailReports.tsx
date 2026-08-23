import { useEffect, useState } from 'react'
import { CalendarCheck, ChevronDown, ClipboardList, Image as ImageIcon, Loader2, Users } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import { EmptyState } from '../components/ui/page'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { formatWibDate } from '../lib/wib'
import { request } from '../lib/api'

type SharedFilters = {
  tahunAjaranId: string
  semesterId: string
  tutorId: string
  kelasId: string
}

type AttendanceMeeting = {
  meetingId?: string
  sequence: number
  plannedDate: string
  actualDate?: string
  statusPertemuan: string
  totalStudents: number
  filledStudents: number
  hasSignature: boolean
  photoCount: number
  status: 'lengkap' | 'belum_dibuat' | 'belum_lengkap' | 'libur' | 'tidak_dipantau'
  issues: string[]
}

type AttendanceClass = {
  classId: string
  classLabel: string
  tutorName: string
  meetings: AttendanceMeeting[]
}

type JournalEntry = {
  id: string
  date: string
  materi: string
  kegiatan: string
  tutorName: string
  hasPhoto: boolean
}

type JournalSubject = {
  mapelId: string
  mapelName: string
  tutorNames: string[]
  entryCount: number
  status: 'terisi' | 'belum_diisi'
  entries: JournalEntry[]
}

type JournalClass = {
  classId: string
  classLabel: string
  tutorName: string
  subjects: JournalSubject[]
}

type DetailSummary = {
  totalPertemuan: number
  pertemuanLengkap: number
  pertemuanBelumLengkap: number
  pertemuanBelumDibuat: number
  pertemuanLibur: number
  pertemuanTidakDipantau: number
  totalMapel: number
  mapelTerisi: number
  mapelBelumDiisi: number
  kelasTanpaMapel: number
}

type DetailResponse = {
  periodStart?: string
  periodEnd?: string
  summary: DetailSummary
  presensi: AttendanceClass[]
  jurnal: JournalClass[]
  warnings: Array<{ type: string; classId: string; classLabel: string; message: string }>
}

type PresensiDetail = {
  details?: Array<{
    id: string
    statusKehadiran: string
    catatan?: string
    pesertaDidik?: { nama?: string; nis?: string }
  }>
  buktiFoto?: string
}

const emptySummary: DetailSummary = {
  totalPertemuan: 0,
  pertemuanLengkap: 0,
  pertemuanBelumLengkap: 0,
  pertemuanBelumDibuat: 0,
  pertemuanLibur: 0,
  pertemuanTidakDipantau: 0,
  totalMapel: 0,
  mapelTerisi: 0,
  mapelBelumDiisi: 0,
  kelasTanpaMapel: 0,
}

const emptyDetail: DetailResponse = { summary: emptySummary, presensi: [], jurnal: [], warnings: [] }

function dateLabel(value?: string) {
  return value ? formatWibDate(value) : '—'
}

function attendanceStatusLabel(status: AttendanceMeeting['status']) {
  switch (status) {
    case 'lengkap': return 'Lengkap'
    case 'belum_dibuat': return 'Belum dibuat'
    case 'belum_lengkap': return 'Belum lengkap'
    case 'libur': return 'Libur'
    default: return 'Tidak dipantau'
  }
}

function attendanceStatusTone(status: AttendanceMeeting['status']) {
  switch (status) {
    case 'lengkap': return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-300'
    case 'libur': return 'bg-slate-100 text-slate-700 dark:bg-slate-500/15 dark:text-slate-300'
    case 'tidak_dipantau': return 'bg-slate-100 text-slate-700 dark:bg-slate-500/15 dark:text-slate-300'
    case 'belum_dibuat': return 'bg-rose-100 text-rose-800 dark:bg-rose-500/15 dark:text-rose-300'
    default: return 'bg-amber-100 text-amber-800 dark:bg-amber-500/15 dark:text-amber-300'
  }
}

function journalStatusTone(status: JournalSubject['status']) {
  return status === 'terisi'
    ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-300'
    : 'bg-rose-100 text-rose-800 dark:bg-rose-500/15 dark:text-rose-300'
}

function parsePhotos(value?: string) {
  if (!value) return [] as string[]
  try {
    const parsed: unknown = JSON.parse(value)
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
  } catch {
    return [value]
  }
}

function ReportMetrics({ kind, summary }: { kind: 'presensi' | 'jurnal'; summary: DetailSummary }) {
  const metrics = kind === 'presensi'
    ? [
        { label: 'Pertemuan dipantau', value: summary.totalPertemuan, tone: 'text-foreground' },
        { label: 'Lengkap', value: summary.pertemuanLengkap, tone: 'text-emerald-600' },
        { label: 'Belum lengkap', value: summary.pertemuanBelumLengkap, tone: 'text-amber-600' },
        { label: 'Belum dibuat', value: summary.pertemuanBelumDibuat, tone: 'text-rose-600' },
      ]
    : [
        { label: 'Mapel dipantau', value: summary.totalMapel, tone: 'text-foreground' },
        { label: 'Sudah diisi', value: summary.mapelTerisi, tone: 'text-emerald-600' },
        { label: 'Belum diisi', value: summary.mapelBelumDiisi, tone: 'text-rose-600' },
        { label: 'Kelas tanpa mapel', value: summary.kelasTanpaMapel, tone: 'text-amber-600' },
      ]
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {metrics.map((metric) => (
        <Card key={metric.label} className="rounded-2xl border border-border bg-card p-4 shadow-2xs">
          <p className={`text-2xl font-extrabold tracking-tight ${metric.tone}`}>{metric.value}</p>
          <p className="mt-1 text-xs font-medium text-muted-foreground">{metric.label}</p>
        </Card>
      ))}
    </div>
  )
}

export function KepatuhanDetailReports({
  token,
  kind,
  active,
  filters,
}: {
  token: string
  kind: 'presensi' | 'jurnal'
  active: boolean
  filters: SharedFilters
}) {
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [data, setData] = useState<DetailResponse>(emptyDetail)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [expandedMeeting, setExpandedMeeting] = useState('')
  const [meetingDetails, setMeetingDetails] = useState<Record<string, PresensiDetail>>({})
  const [loadingMeeting, setLoadingMeeting] = useState('')
  const [expandedJournal, setExpandedJournal] = useState('')
  const [photoPreview, setPhotoPreview] = useState('')

  useEffect(() => {
    setFrom('')
    setTo('')
  }, [filters.semesterId, filters.tahunAjaranId])

  useEffect(() => {
    if (!active) return
    const controller = new AbortController()
    const params = new URLSearchParams({ jenis: kind })
    if (filters.tahunAjaranId) params.set('tahunAjaranId', filters.tahunAjaranId)
    if (filters.semesterId) params.set('semesterId', filters.semesterId)
    if (filters.tutorId) params.set('tutorId', filters.tutorId)
    if (filters.kelasId) params.set('kelasId', filters.kelasId)
    if (from) params.set('from', from)
    if (to) params.set('to', to)
    setLoading(true)
    setError('')
    void request(`/dashboard/kepatuhan-pembelajaran/detail?${params.toString()}`, token, 'GET', undefined, controller.signal)
      .then((result) => {
        if (!controller.signal.aborted) setData(result as DetailResponse)
      })
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) {
          setData(emptyDetail)
          setError(reason instanceof Error ? reason.message : 'Laporan detail belum dapat dimuat.')
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [active, filters.kelasId, filters.semesterId, filters.tahunAjaranId, filters.tutorId, from, kind, to, token])

  async function toggleMeeting(meetingID: string) {
    if (expandedMeeting === meetingID) {
      setExpandedMeeting('')
      return
    }
    setExpandedMeeting(meetingID)
    if (meetingDetails[meetingID]) return
    setLoadingMeeting(meetingID)
    try {
      const detail = await request(`/presensi/${meetingID}`, token)
      setMeetingDetails((current) => ({ ...current, [meetingID]: detail as PresensiDetail }))
    } catch (reason: unknown) {
      setError(reason instanceof Error ? reason.message : 'Detail presensi belum dapat dimuat.')
      setExpandedMeeting('')
    } finally {
      setLoadingMeeting('')
    }
  }

  const title = kind === 'presensi' ? 'Detail Presensi per Kelas' : 'Detail Jurnal per Kelas & Mapel'
  const description = kind === 'presensi'
    ? 'Pantau urutan pertemuan, kelengkapan siswa, tanda tangan, dan foto bukti KBM.'
    : 'Mapel dianggap belum diisi bila tidak memiliki jurnal sama sekali pada periode terpilih.'

  return (
    <div className="space-y-4">
      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs sm:p-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h3 className="flex items-center gap-2 text-sm font-bold text-foreground">
              {kind === 'presensi' ? <CalendarCheck className="h-4 w-4 text-primary" /> : <ClipboardList className="h-4 w-4 text-primary" />}
              {title}
            </h3>
            <p className="mt-1 text-xs text-muted-foreground">{description}</p>
          </div>
          <div className="grid grid-cols-2 gap-2 sm:w-[320px]">
            <div className="grid gap-1">
              <label className="text-xs font-medium text-muted-foreground">Dari tanggal</label>
              <Input type="date" value={from} onChange={(event) => setFrom(event.target.value)} />
            </div>
            <div className="grid gap-1">
              <label className="text-xs font-medium text-muted-foreground">Sampai tanggal</label>
              <Input type="date" value={to} onChange={(event) => setTo(event.target.value)} />
            </div>
          </div>
        </div>
        <p className="mt-3 text-xs text-muted-foreground">
          {data.periodStart && data.periodEnd
            ? `Periode laporan: ${dateLabel(data.periodStart)} s/d ${dateLabel(data.periodEnd)} (WIB).`
            : 'Kosongkan rentang untuk memakai bagian semester yang sudah berjalan.'}
        </p>
      </Card>

      {error && <div className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>}

      {loading ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => <Card key={index} className="h-24 animate-pulse rounded-2xl border border-border bg-muted" />)}
        </div>
      ) : (
        <ReportMetrics kind={kind} summary={data.summary} />
      )}

      {kind === 'presensi' ? (
        <div className="space-y-4">
          {loading ? <ReportTableSkeleton columns={6} /> : data.presensi.length === 0 ? (
          <EmptyState title="Tidak ada kelas atau pertemuan" description="Ubah filter atau rentang tanggal untuk melihat laporan presensi." />
          ) : data.presensi.map((classRow) => (
            <Card key={classRow.classId} className="overflow-hidden rounded-2xl border border-border bg-card shadow-2xs">
              <div className="flex flex-col gap-1 border-b border-border/70 px-5 py-4 sm:px-6">
                <h4 className="font-bold text-foreground">{classRow.classLabel}</h4>
                <p className="text-xs text-muted-foreground">Wali kelas: {classRow.tutorName}</p>
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Pertemuan</TableHead>
                    <TableHead>Tanggal</TableHead>
                    <TableHead>Kelompok siswa</TableHead>
                    <TableHead>Bukti</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Detail</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {classRow.meetings.map((meeting) => (
                    <AttendanceRow
                      key={`${classRow.classId}-${meeting.sequence}`}
                      meeting={meeting}
                      expanded={expandedMeeting === meeting.meetingId}
                      loading={loadingMeeting === meeting.meetingId}
                      detail={meeting.meetingId ? meetingDetails[meeting.meetingId] : undefined}
                      onToggle={() => meeting.meetingId && void toggleMeeting(meeting.meetingId)}
                      onPreview={setPhotoPreview}
                    />
                  ))}
                </TableBody>
              </Table>
            </Card>
          ))}
        </div>
      ) : (
        <div className="space-y-4">
          {data.warnings.map((warning) => (
            <div key={warning.classId} className="rounded-xl border border-warning/30 bg-warning/10 px-4 py-3 text-sm text-warning-foreground">
              <strong>{warning.classLabel}:</strong> {warning.message}
            </div>
          ))}
          {loading ? <ReportTableSkeleton columns={5} /> : data.jurnal.length === 0 ? (
          <EmptyState title="Tidak ada kelas atau mapel" description="Ubah filter atau rentang tanggal untuk melihat laporan jurnal." />
          ) : data.jurnal.map((classRow) => (
            <Card key={classRow.classId} className="overflow-hidden rounded-2xl border border-border bg-card shadow-2xs">
              <div className="flex flex-col gap-1 border-b border-border/70 px-5 py-4 sm:px-6">
                <h4 className="font-bold text-foreground">{classRow.classLabel}</h4>
                <p className="text-xs text-muted-foreground">Wali kelas: {classRow.tutorName}</p>
              </div>
              {classRow.subjects.length === 0 ? (
                <div className="px-5 py-5 text-sm text-muted-foreground">Belum ada mapel yang dikonfigurasi pada kelas ini.</div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Mata pelajaran</TableHead>
                      <TableHead>Pengajar ditugaskan</TableHead>
                      <TableHead>Jumlah jurnal</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">Detail</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {classRow.subjects.map((subject) => {
                      const subjectKey = `${classRow.classId}-${subject.mapelId}`
                      const expanded = expandedJournal === subjectKey
                      return (
                        <JournalRow
                          key={subjectKey}
                          subject={subject}
                          expanded={expanded}
                          onToggle={() => setExpandedJournal(expanded ? '' : subjectKey)}
                        />
                      )
                    })}
                  </TableBody>
                </Table>
              )}
            </Card>
          ))}
        </div>
      )}

      <Dialog open={!!photoPreview} onOpenChange={(open) => !open && setPhotoPreview('')}>
        <DialogContent className="max-w-3xl gap-2 p-2">
          <DialogTitle className="sr-only">Pratinjau foto bukti KBM</DialogTitle>
          <DialogDescription className="sr-only">Foto dokumentasi dari detail presensi.</DialogDescription>
          {photoPreview && <img src={photoPreview} alt="Foto bukti KBM" className="max-h-[80vh] w-full rounded-xl object-contain" />}
        </DialogContent>
      </Dialog>
    </div>
  )
}

function AttendanceRow({
  meeting,
  expanded,
  loading,
  detail,
  onToggle,
  onPreview,
}: {
  meeting: AttendanceMeeting
  expanded: boolean
  loading: boolean
  detail?: PresensiDetail
  onToggle: () => void
  onPreview: (url: string) => void
}) {
  const photos = parsePhotos(detail?.buktiFoto)
  return (
    <>
      <TableRow>
        <TableCell className="font-semibold">#{meeting.sequence}</TableCell>
        <TableCell>
          <div>{dateLabel(meeting.plannedDate)}</div>
          {meeting.actualDate && <div className="text-xs text-muted-foreground">Dilaksanakan: {dateLabel(meeting.actualDate)}</div>}
        </TableCell>
        <TableCell>{meeting.filledStudents}/{meeting.totalStudents}</TableCell>
        <TableCell>
          <div className="text-xs">{meeting.hasSignature ? 'Tanda tangan ada' : 'Tanda tangan kosong'}</div>
          <div className="text-xs text-muted-foreground">{meeting.photoCount} foto KBM</div>
        </TableCell>
        <TableCell>
          <span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ${attendanceStatusTone(meeting.status)}`}>{attendanceStatusLabel(meeting.status)}</span>
          {meeting.issues.length > 0 && <p className="mt-1 max-w-xs text-xs text-muted-foreground">{meeting.issues.join(' · ')}</p>}
        </TableCell>
        <TableCell className="text-right">
          {meeting.meetingId ? (
            <Button size="sm" variant="outline" onClick={onToggle} disabled={loading}>
              {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ChevronDown className={`h-3.5 w-3.5 transition-transform ${expanded ? 'rotate-180' : ''}`} />}
              {expanded ? 'Tutup' : 'Lihat'}
            </Button>
          ) : '—'}
        </TableCell>
      </TableRow>
      {expanded && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={6} className="bg-muted/30">
            {!detail ? <div className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 className="h-4 w-4 animate-spin" /> Memuat detail presensi...</div> : (
              <div className="grid gap-5 lg:grid-cols-[1fr_280px]">
                <div>
                  <h5 className="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground"><Users className="h-4 w-4 text-primary" /> Detail kehadiran siswa</h5>
                  {detail.details?.length ? (
                    <div className="grid gap-2 sm:grid-cols-2">
                      {detail.details.map((student) => (
                        <div key={student.id} className="rounded-xl border border-border bg-card px-3 py-2">
                          <p className="font-medium text-foreground">{student.pesertaDidik?.nama || 'Peserta didik'}</p>
                          <p className="text-xs text-muted-foreground">{student.pesertaDidik?.nis || '—'} · {student.statusKehadiran}</p>
                          {student.catatan && <p className="mt-1 text-xs text-muted-foreground">{student.catatan}</p>}
                        </div>
                      ))}
                    </div>
                  ) : <p className="text-sm text-muted-foreground">Belum ada detail kehadiran siswa.</p>}
                </div>
                <div>
                  <h5 className="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground"><ImageIcon className="h-4 w-4 text-primary" /> Foto bukti KBM</h5>
                  {photos.length ? (
                    <div className="grid grid-cols-2 gap-2">
                      {photos.map((photo, index) => (
                        <button key={`${photo.slice(0, 24)}-${index}`} type="button" className="aspect-[4/3] overflow-hidden rounded-xl border border-border bg-card" onClick={() => onPreview(photo)}>
                          <img src={photo} alt={`Foto KBM ${index + 1}`} className="h-full w-full object-cover" />
                        </button>
                      ))}
                    </div>
                  ) : <p className="text-sm text-muted-foreground">Belum ada foto bukti KBM.</p>}
                </div>
              </div>
            )}
          </TableCell>
        </TableRow>
      )}
    </>
  )
}

function JournalRow({ subject, expanded, onToggle }: { subject: JournalSubject; expanded: boolean; onToggle: () => void }) {
  return (
    <>
      <TableRow>
        <TableCell className="font-semibold text-foreground">{subject.mapelName}</TableCell>
        <TableCell>{subject.tutorNames.length ? subject.tutorNames.join(', ') : <span className="text-muted-foreground">Belum ditugaskan</span>}</TableCell>
        <TableCell>{subject.entryCount}</TableCell>
        <TableCell><span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ${journalStatusTone(subject.status)}`}>{subject.status === 'terisi' ? 'Terisi' : 'Belum diisi'}</span></TableCell>
        <TableCell className="text-right">
          {subject.entryCount ? <Button size="sm" variant="outline" onClick={onToggle}><ChevronDown className={`h-3.5 w-3.5 transition-transform ${expanded ? 'rotate-180' : ''}`} />{expanded ? 'Tutup' : 'Lihat'}</Button> : '—'}
        </TableCell>
      </TableRow>
      {expanded && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={5} className="bg-muted/30">
            <div className="space-y-3">
              {subject.entries.map((entry) => (
                <div key={entry.id} className="rounded-xl border border-border bg-card p-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="font-semibold text-foreground">{dateLabel(entry.date)}</p>
                    <span className="text-xs text-muted-foreground">{entry.tutorName} · {entry.hasPhoto ? 'Ada foto dokumentasi' : 'Tanpa foto'}</span>
                  </div>
                  <p className="mt-2 text-sm text-foreground"><strong>Materi:</strong> {entry.materi || '—'}</p>
                  <p className="mt-1 text-sm text-muted-foreground"><strong>Kegiatan:</strong> {entry.kegiatan || '—'}</p>
                </div>
              ))}
            </div>
          </TableCell>
        </TableRow>
      )}
    </>
  )
}

function ReportTableSkeleton({ columns }: { columns: number }) {
  return (
    <Card className="overflow-hidden rounded-2xl border border-border bg-card shadow-2xs">
      <Table>
        <TableBody>
          {Array.from({ length: 5 }).map((_, index) => <TableRow key={index}><TableCell colSpan={columns}><div className="h-4 w-full animate-pulse rounded bg-muted" /></TableCell></TableRow>)}
        </TableBody>
      </Table>
    </Card>
  )
}
