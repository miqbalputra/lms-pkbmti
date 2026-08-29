import { useEffect, useState } from 'react'
import { CalendarCheck, ChevronDown, ClipboardList, Download, Image as ImageIcon, Loader2, Users } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import { EmptyState } from '../components/ui/page'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { formatWibDate, formatWibDateTime } from '../lib/wib'
import { downloadFile, request } from '../lib/api'
import { toast } from 'sonner'

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
  generatedAt?: string
  tahunAjaran?: { id: string; label: string }
  semester?: { id: string; label: string }
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

type AttendanceFollowUpRow = {
  classLabel: string
  tutorName: string
  plannedDate: string
  actualDate?: string
  status: 'belum_dibuat' | 'belum_lengkap'
  followUp: string
}

function dateLabel(value?: string) {
  return value ? formatWibDate(value) : '—'
}

function attendanceFollowUpText(meeting: AttendanceMeeting) {
  const issues: string[] = []
  if (meeting.status === 'belum_dibuat') issues.push('Presensi belum dibuat')
  for (const rawIssue of meeting.issues) {
    let issue = rawIssue
    if (issue === 'Kehadiran siswa belum lengkap' && meeting.totalStudents > 0) {
      issue = `Kehadiran siswa ${meeting.filledStudents}/${meeting.totalStudents} sudah diisi`
    }
    if (issue !== 'Belum ada data presensi' && issue !== 'Presensi belum dibuat') issues.push(issue)
  }
  if (meeting.actualDate) issues.push(`Dilaksanakan ${dateLabel(meeting.actualDate)}`)
  return issues.length > 0 ? issues.join('; ') : 'Perlu dilengkapi'
}

function pendingAttendanceRows(data: DetailResponse): AttendanceFollowUpRow[] {
  return data.presensi.flatMap((classRow) =>
    classRow.meetings
      .filter((meeting) => meeting.status === 'belum_dibuat' || meeting.status === 'belum_lengkap')
      .map((meeting) => ({
        classLabel: classRow.classLabel,
        tutorName: classRow.tutorName,
        plannedDate: dateLabel(meeting.plannedDate),
        actualDate: meeting.actualDate,
        status: meeting.status as AttendanceFollowUpRow['status'],
        followUp: attendanceFollowUpText(meeting),
      }))
  )
}

function wrapReportText(value: string, maxCharacters: number) {
  const words = value.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return ['']
  const lines: string[] = []
  let current = ''
  for (const word of words) {
    const next = current ? `${current} ${word}` : word
    if (current && next.length > maxCharacters) {
      lines.push(current)
      current = word
    } else {
      current = next
    }
  }
  if (current) lines.push(current)
  return lines
}

function escapeSvgText(value: string) {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;')
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

async function downloadAttendanceJpg(data: DetailResponse) {
  const rows = pendingAttendanceRows(data)
  const width = 1400
  const padding = 56
  const tableWidth = width - padding * 2
  const columnWidths = [70, 240, 300, 190, 488]
  const title = 'Laporan Tindak Lanjut Presensi'
  const period = [data.tahunAjaran?.label, data.semester?.label].filter(Boolean).join(' · ') || 'Periode belum tersedia'
  const generatedAt = data.generatedAt ? formatWibDateTime(data.generatedAt) : formatWibDateTime(new Date())
  const bodyFontSize = 18
  const lineHeight = 26
  const headerHeight = 58
  const rowLayouts = rows.map((row) => {
    const values = [
      String(rows.indexOf(row) + 1),
      row.classLabel,
      row.tutorName,
      row.plannedDate,
      `${row.status === 'belum_dibuat' ? 'Belum dibuat' : 'Belum lengkap'}: ${row.followUp}`,
    ]
    const lines = values.map((value, index) => wrapReportText(value, Math.max(8, Math.floor((columnWidths[index] - 26) / 9))))
    return { values, lines, height: Math.max(72, Math.max(...lines.map((value) => value.length)) * lineHeight + 26) }
  })
  const emptyHeight = 84
  const tableHeight = headerHeight + (rows.length > 0 ? rowLayouts.reduce((total, row) => total + row.height, 0) : emptyHeight)
  const height = 190 + tableHeight + padding
  const svg: string[] = []
  const text = (value: string, x: number, y: number, size: number, color: string, anchor: 'start' | 'middle' = 'start', weight = 400) => {
    svg.push(`<text x="${x}" y="${y}" text-anchor="${anchor}" font-family="Arial, sans-serif" font-size="${size}px" font-weight="${weight}" fill="${color}">${escapeSvgText(value)}</text>`)
  }
  const multiline = (lines: string[], x: number, y: number, maxWidth: number, anchor: 'start' | 'middle' = 'start', color = '#17211b', weight = 400) => {
    const textX = anchor === 'middle' ? x + maxWidth / 2 : x
    lines.forEach((line, index) => text(line, textX, y + index * lineHeight, bodyFontSize, color, anchor, weight))
  }

  svg.push(`<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">`)
  svg.push('<rect width="100%" height="100%" fill="#ffffff"/>')
  text(title, width / 2, 64, 34, '#1c5740', 'middle', 700)
  text('PKBM Tunas Ilmu', width / 2, 98, 20, '#17211b', 'middle', 600)
  text(period, width / 2, 130, 18, '#526158', 'middle')
  text(`Dibuat ${generatedAt} WIB · ${rows.length} item perlu ditindaklanjuti`, width / 2, 158, 17, '#526158', 'middle')

  let y = 190
  let x = padding
  columnWidths.forEach((columnWidth, index) => {
    svg.push(`<rect x="${x}" y="${y}" width="${columnWidth}" height="${headerHeight}" fill="#1c5740" stroke="#ffffff" stroke-width="2"/>`)
    text(['No', 'Rombel', 'Tutor', 'Tanggal', 'Status / tindak lanjut'][index], x + columnWidth / 2, y + 36, 17, '#ffffff', 'middle', 700)
    x += columnWidth
  })
  y += headerHeight

  if (rows.length === 0) {
    svg.push(`<rect x="${padding}" y="${y}" width="${tableWidth}" height="${emptyHeight}" fill="#ffffff" stroke="#d6dfd9" stroke-width="2"/>`)
    text('Tidak ada presensi yang perlu ditindaklanjuti.', width / 2, y + 50, bodyFontSize, '#526158', 'middle', 600)
  } else {
    rowLayouts.forEach((layout, rowIndex) => {
      x = padding
      const fill = rowIndex % 2 === 0 ? '#f7faf8' : '#ffffff'
      layout.lines.forEach((lines, columnIndex) => {
        svg.push(`<rect x="${x}" y="${y}" width="${columnWidths[columnIndex]}" height="${layout.height}" fill="${fill}" stroke="#d6dfd9" stroke-width="2"/>`)
        multiline(lines, x + (columnIndex === 0 || columnIndex === 3 ? 0 : 14), y + 32, columnWidths[columnIndex] - (columnIndex === 0 || columnIndex === 3 ? 0 : 28), columnIndex === 0 || columnIndex === 3 ? 'middle' : 'start', columnIndex === 4 ? '#8a4b00' : '#17211b', columnIndex === 4 ? 600 : 400)
        x += columnWidths[columnIndex]
      })
      y += layout.height
    })
  }
  svg.push('</svg>')

  const sourceUrl = URL.createObjectURL(new Blob([svg.join('')], { type: 'image/svg+xml;charset=utf-8' }))
  try {
    const image = new Image()
    await new Promise<void>((resolve, reject) => {
      image.onload = () => resolve()
      image.onerror = () => reject(new Error('JPG gagal dibuat.'))
      image.src = sourceUrl
    })
    const scale = Math.min(2, 30000 / height)
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(width * scale))
    canvas.height = Math.max(1, Math.round(height * scale))
    const context = canvas.getContext('2d')
    if (!context) throw new Error('Browser tidak mendukung pembuatan JPG.')
    context.fillStyle = '#ffffff'
    context.fillRect(0, 0, canvas.width, canvas.height)
    context.drawImage(image, 0, 0, canvas.width, canvas.height)
    const blob = await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob((value) => value ? resolve(value) : reject(new Error('JPG gagal dibuat.')), 'image/jpeg', 0.92)
    })
    downloadBlob(blob, `laporan-tindak-lanjut-presensi-${new Date().toISOString().slice(0, 10)}.jpg`)
  } finally {
    URL.revokeObjectURL(sourceUrl)
  }
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
  const [exporting, setExporting] = useState<'pdf' | 'jpg' | ''>('')

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

  function exportQuery(format: 'pdf') {
    const params = new URLSearchParams({ jenis: 'presensi', format })
    if (filters.tahunAjaranId) params.set('tahunAjaranId', filters.tahunAjaranId)
    if (filters.semesterId) params.set('semesterId', filters.semesterId)
    if (filters.tutorId) params.set('tutorId', filters.tutorId)
    if (filters.kelasId) params.set('kelasId', filters.kelasId)
    if (from) params.set('from', from)
    if (to) params.set('to', to)
    return params.toString()
  }

  async function exportPdf() {
    setExporting('pdf')
    try {
      await downloadFile(
        `/dashboard/kepatuhan-pembelajaran/export?${exportQuery('pdf')}`,
        token,
        'laporan-tindak-lanjut-presensi.pdf',
      )
      toast.success('Laporan PDF diunduh.')
    } catch (reason: unknown) {
      toast.error(reason instanceof Error ? reason.message : 'PDF gagal dibuat.')
    } finally {
      setExporting('')
    }
  }

  async function exportJpg() {
    setExporting('jpg')
    try {
      await downloadAttendanceJpg(data)
      toast.success('Laporan JPG diunduh.')
    } catch (reason: unknown) {
      toast.error(reason instanceof Error ? reason.message : 'JPG gagal dibuat.')
    } finally {
      setExporting('')
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
          <div className="flex flex-wrap items-end justify-end gap-2 sm:max-w-[470px]">
            <div className="grid min-w-[140px] flex-1 gap-1">
              <label className="text-xs font-medium text-muted-foreground">Dari tanggal</label>
              <Input type="date" value={from} onChange={(event) => setFrom(event.target.value)} />
            </div>
            <div className="grid min-w-[140px] flex-1 gap-1">
              <label className="text-xs font-medium text-muted-foreground">Sampai tanggal</label>
              <Input type="date" value={to} onChange={(event) => setTo(event.target.value)} />
            </div>
            {kind === 'presensi' && (
              <div className="flex w-full flex-wrap gap-2 sm:justify-end">
                <Button size="sm" variant="outline" onClick={() => void exportPdf()} disabled={loading || exporting !== ''}>
                  <Download className="h-4 w-4" /> {exporting === 'pdf' ? 'Menyiapkan...' : 'Unduh PDF'}
                </Button>
                <Button size="sm" variant="outline" onClick={() => void exportJpg()} disabled={loading || exporting !== ''}>
                  <ImageIcon className="h-4 w-4" /> {exporting === 'jpg' ? 'Menyiapkan...' : 'Unduh JPG'}
                </Button>
              </div>
            )}
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
