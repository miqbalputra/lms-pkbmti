import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { FileImage, FileText, FileType2, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Signature } from '../components/ui/Signature'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'
import { formatWibDate, wibToday } from '../lib/wib'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }
type Assignment = Row & { tutorId: string; kelasId: string; mapelId: string; tutor?: Row; kelas?: Row; mapel?: Row }
type JournalLine = { jamKe: number; mapelId: string; materi: string }
type SheetLine = { id: string; batchId: string; jamKe: number; tutorId: string; tutorNama: string; mapelId: string; mapelNama: string; materi: string; kegiatan?: string; tandaTangan?: string; fotoPath?: string; canEdit: boolean }
type AbsentStudent = { pesertaDidikId: string; nama: string; statusKehadiran: string }
type Sheet = { kelasId: string; kelasLabel: string; tanggal: string; attendanceStatus: 'belum_ada' | 'sebagian' | 'terisi'; absentStudents: AbsentStudent[]; lines: SheetLine[] }

function latestSaturday(): string {
  const date = new Date(`${wibToday()}T12:00:00+07:00`)
  date.setUTCDate(date.getUTCDate() - ((date.getUTCDay() + 1) % 7))
  return date.toISOString().slice(0, 10)
}

function isAllowedSaturday(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || value > wibToday()) return false
  return new Date(`${value}T12:00:00+07:00`).getUTCDay() === 6
}

function absenceText(absences: AbsentStudent[]): string {
  return absences.length ? absences.map((student) => `${student.nama} (${student.statusKehadiran})`).join(', ') : '—'
}

function defaultLine(mapelId = ''): JournalLine { return { jamKe: 1, mapelId, materi: '' } }
function assignmentClassLabel(assignment: Assignment): string { return `Kelas ${String(assignment.kelas?.jenjang || '')}${String(assignment.kelas?.namaRombel || '')}` }
function assignmentMapelLabel(assignment: Assignment): string { return String(assignment.mapel?.namaMapel || '-') }

export function JurnalMengajarView({ token, user, readOnly }: { token: string; user: User; readOnly: boolean }) {
  const [searchParams, setSearchParams] = useSearchParams()
  const [assignments, setAssignments] = useState<Assignment[]>([])
  const [tutors, setTutors] = useState<Row[]>([])
  const [selectedTutorID, setSelectedTutorID] = useState(user.tutorId || '')
  const [classID, setClassID] = useState(searchParams.get('kelasId') || '')
  const [date, setDate] = useState(searchParams.get('tanggal') || latestSaturday())
  const [prefillMapelID, setPrefillMapelID] = useState(searchParams.get('mapelId') || '')
  const [sheet, setSheet] = useState<Sheet | null>(null)
  const [loadingSheet, setLoadingSheet] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editingBatchID, setEditingBatchID] = useState('')
  const [lines, setLines] = useState<JournalLine[]>([defaultLine()])
  const [signature, setSignature] = useState('')
  const [photo, setPhoto] = useState<File | null>(null)
  const [saving, setSaving] = useState(false)
  const loadVersion = useRef(0)

  const isAdmin = user.role === 'admin'
  const activeTutorID = isAdmin ? selectedTutorID : user.tutorId || ''
  const tutorAssignments = useMemo(
    () => assignments.filter((assignment) => assignment.tutorId === activeTutorID && assignment.mapel?.isActive !== false),
    [assignments, activeTutorID],
  )
  const classAssignments = useMemo(() => tutorAssignments.filter((assignment) => assignment.kelasId === classID), [tutorAssignments, classID])
  const classOptions = useMemo(() => {
    const seen = new Set<string>()
    return tutorAssignments.filter((assignment) => !seen.has(assignment.kelasId) && !!seen.add(assignment.kelasId))
  }, [tutorAssignments])
  const selectedTutorName = isAdmin ? String(tutors.find((tutor) => tutor.id === selectedTutorID)?.nama || 'Tutor') : String(user.nama || user.username)
  const existingSignature = editingBatchID ? sheet?.lines.find((line) => line.batchId === editingBatchID)?.tandaTangan || '' : ''
  const selectedClass = classOptions.find((assignment) => assignment.kelasId === classID)?.kelas as Row | undefined
  const sheetDateInvalid = !isAllowedSaturday(date)

  const loadSheet = useCallback(async () => {
    if (!classID || !/^\d{4}-\d{2}-\d{2}$/.test(date)) { setSheet(null); return }
    const version = ++loadVersion.current
    setLoadingSheet(true)
    try {
      const result = await request(`/jurnal/sheet?kelasId=${encodeURIComponent(classID)}&tanggal=${encodeURIComponent(date)}`, token) as Sheet
      if (version === loadVersion.current) setSheet(result)
    } catch (error) {
      if (version === loadVersion.current) { setSheet(null); toast.error(error instanceof Error ? error.message : 'Gagal memuat lembar jurnal.') }
    } finally { if (version === loadVersion.current) setLoadingSheet(false) }
  }, [classID, date, token])

  useEffect(() => {
    void request('/penugasan', token).then((value: Assignment[]) => setAssignments(Array.isArray(value) ? value : [])).catch(() => setAssignments([]))
    if (isAdmin) void request('/tutor', token).then((value: Row[]) => setTutors(Array.isArray(value) ? value : [])).catch(() => setTutors([]))
  }, [isAdmin, token])

  useEffect(() => {
    if (!classID && classOptions.length) setClassID(classOptions[0].kelasId)
    if (classID && !classOptions.some((assignment) => assignment.kelasId === classID)) setClassID('')
  }, [classID, classOptions])
  useEffect(() => { void loadSheet() }, [loadSheet])
  useEffect(() => {
    if (!classID) return
    const timer = window.setInterval(() => void loadSheet(), 60_000)
    return () => window.clearInterval(timer)
  }, [classID, loadSheet])
  useEffect(() => {
    if (!searchParams.size) return
    const next = new URLSearchParams(searchParams)
    next.delete('kelasId'); next.delete('tanggal'); next.delete('mapelId')
    setSearchParams(next, { replace: true })
  }, [searchParams, setSearchParams])

  function openNewBatch() {
    if (!activeTutorID || !classID) { toast.error(isAdmin ? 'Pilih tutor dan kelas terlebih dahulu.' : 'Pilih kelas terlebih dahulu.'); return }
    if (sheetDateInvalid) { toast.error('Pengisian jurnal hanya dibuka untuk hari Sabtu dari semester aktif sampai hari ini.'); return }
    const availableMapel = classAssignments.some((assignment) => assignment.mapelId === prefillMapelID) ? prefillMapelID : classAssignments[0]?.mapelId || ''
    setEditingBatchID(''); setLines([defaultLine(availableMapel)]); setSignature(''); setPhoto(null); setFormOpen(true)
  }

  function openEdit(batchID: string) {
    if (sheetDateInvalid) { toast.error('Tanggal jurnal tidak dapat diedit dari formulir ini.'); return }
    const selectedLines = sheet?.lines.filter((line) => line.batchId === batchID) || []
    if (!selectedLines.length) return
    setEditingBatchID(batchID); setLines(selectedLines.map((line) => ({ jamKe: line.jamKe, mapelId: line.mapelId, materi: line.materi })))
    setSignature(selectedLines[0].tandaTangan || ''); setPhoto(null); setFormOpen(true)
  }

  function updateLine(index: number, patch: Partial<JournalLine>) { setLines((current) => current.map((line, lineIndex) => lineIndex === index ? { ...line, ...patch } : line)) }
  function addLine() {
    const nextJam = Math.max(0, ...lines.map((line) => line.jamKe || 0), ...((sheet?.lines || []).map((line) => line.jamKe))) + 1
    setLines((current) => [...current, { ...defaultLine(classAssignments[0]?.mapelId || ''), jamKe: nextJam }])
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!classID || !activeTutorID) return toast.error('Tutor dan kelas wajib dipilih.')
    if (sheetDateInvalid) return toast.error('Pilih hari Sabtu dari semester aktif sampai hari ini.')
    if (lines.length < 1 || lines.some((line) => !line.jamKe || !line.mapelId || !line.materi.trim())) return toast.error('Lengkapi Jam Ke, Mata Pelajaran, dan Materi pada setiap baris.')
    const signatureToSave = signature || existingSignature
    if (!signatureToSave) return toast.error('Paraf tutor wajib diisi sebelum menyimpan jurnal.')
    const data = new FormData()
    if (isAdmin) data.append('tutorId', selectedTutorID)
    data.append('kelasId', classID); data.append('tanggal', date); data.append('tandaTangan', signatureToSave); data.append('lines', JSON.stringify(lines))
    if (photo) data.append('foto', photo)
    setSaving(true)
    try {
      const response = await fetch(`${apiBase}/jurnal/batches${editingBatchID ? `/${editingBatchID}` : ''}`, { method: editingBatchID ? 'PUT' : 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: data })
      const result = await response.json().catch(() => ({}))
      if (!response.ok) throw new Error((result as { error?: string }).error || `Permintaan gagal (${response.status}).`)
      toast.success(editingBatchID ? 'Jurnal diperbarui.' : 'Jurnal tersimpan.')
      setFormOpen(false); setEditingBatchID(''); setPhoto(null); setPrefillMapelID(''); await loadSheet()
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Gagal menyimpan jurnal.') } finally { setSaving(false) }
  }

  async function exportSheet(format: 'pdf' | 'docx' | 'jpg') {
    if (!classID || !date) return
    try {
      const response = await fetch(`${apiBase}/jurnal/export?kelasId=${encodeURIComponent(classID)}&tanggal=${encodeURIComponent(date)}&format=${format}`, { credentials: 'include', headers: { Authorization: `Bearer ${token}` } })
      if (!response.ok) throw new Error('Ekspor jurnal gagal.')
      const blob = await response.blob(); const url = URL.createObjectURL(blob); const anchor = document.createElement('a')
      anchor.href = url; anchor.download = `jurnal-${date}.${format}`; anchor.click(); URL.revokeObjectURL(url)
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Ekspor jurnal gagal.') }
  }

  const batchCounts = useMemo(() => {
    const counts = new Map<string, number>()
    for (const line of sheet?.lines || []) counts.set(line.batchId, (counts.get(line.batchId) || 0) + 1)
    return counts
  }, [sheet])
  const firstLineIDs = useMemo(() => {
    const seen = new Set<string>(); const first = new Set<string>()
    for (const line of sheet?.lines || []) {
      if (!seen.has(line.batchId)) { seen.add(line.batchId); first.add(line.id) }
    }
    return first
  }, [sheet])

  return <div className="space-y-4">
    <PageToolbar title="Jurnal Mengajar" description="Lembar jurnal Sabtu per kelas. Ketidakhadiran tersinkron langsung dari Presensi Kelas." actions={<div className="flex flex-wrap gap-2">
      <Button variant="outline" disabled={!classID || !date} onClick={() => void exportSheet('pdf')}><FileText className="h-4 w-4" /> PDF</Button>
      <Button variant="outline" disabled={!classID || !date} onClick={() => void exportSheet('docx')}><FileType2 className="h-4 w-4" /> Word</Button>
      <Button variant="outline" disabled={!classID || !date} onClick={() => void exportSheet('jpg')}><FileImage className="h-4 w-4" /> JPG</Button>
      {!readOnly && <Button onClick={openNewBatch}><Plus className="h-4 w-4" /> Isi jurnal</Button>}
    </div>} />

    <Card className="grid gap-4 p-4 md:grid-cols-3">
      {isAdmin && <div className="grid gap-2"><Label>Tutor</Label><Select value={selectedTutorID} onChange={(event) => setSelectedTutorID(event.target.value)}><option value="">Pilih tutor</option>{tutors.map((tutor) => <option key={tutor.id} value={tutor.id}>{String(tutor.nama || '-')}</option>)}</Select></div>}
      <div className="grid gap-2"><Label>Kelas / Rombel</Label><Select value={classID} onChange={(event) => setClassID(event.target.value)} disabled={!activeTutorID}><option value="">Pilih kelas</option>{classOptions.map((assignment) => <option key={assignment.kelasId} value={assignment.kelasId}>{assignmentClassLabel(assignment)}</option>)}</Select></div>
      <div className="grid gap-2"><Label>Tanggal jurnal</Label><Input type="date" value={date} max={wibToday()} onChange={(event) => setDate(event.target.value)} />{sheetDateInvalid && <p className="text-xs text-destructive">Hanya Sabtu dari semester aktif sampai hari ini yang dapat disimpan.</p>}</div>
      <div className="flex items-end"><Button variant="outline" disabled={!classID || loadingSheet} onClick={() => void loadSheet()}><RefreshCw className="h-4 w-4" /> Muat ulang</Button></div>
    </Card>

    {formOpen && !readOnly && <FormCard title={editingBatchID ? 'Edit jurnal saya' : 'Isi jurnal saya'} description={`Tutor: ${selectedTutorName}. Paraf berlaku untuk seluruh baris pada kiriman ini.`}>
      <form className="space-y-4" onSubmit={submit}>
        <div className="overflow-x-auto rounded-xl border border-border"><Table className="min-w-[960px]"><TableHeader><TableRow><TableHead className="w-24">Jam Ke</TableHead><TableHead className="w-48">Nama Tutor</TableHead><TableHead className="w-56">Mata Pelajaran</TableHead><TableHead>Materi</TableHead><TableHead className="w-64">Peserta Didik Tidak Hadir</TableHead><TableHead className="w-12" /></TableRow></TableHeader><TableBody>{lines.map((line, index) => <TableRow key={index}><TableCell><Input type="number" min="1" value={line.jamKe} onChange={(event) => updateLine(index, { jamKe: Number(event.target.value) })} required /></TableCell><TableCell className="font-medium">{selectedTutorName}</TableCell><TableCell><Select value={line.mapelId} onChange={(event) => updateLine(index, { mapelId: event.target.value })} required><option value="">Pilih mapel</option>{classAssignments.map((assignment) => <option key={assignment.mapelId} value={assignment.mapelId}>{assignmentMapelLabel(assignment)}</option>)}</Select></TableCell><TableCell><textarea className="min-h-20 w-full rounded-lg border border-input bg-background px-3 py-2 text-sm" value={line.materi} onChange={(event) => updateLine(index, { materi: event.target.value })} placeholder="Materi yang diajarkan" required /></TableCell><TableCell className="text-xs text-muted-foreground">{sheet?.attendanceStatus === 'belum_ada' ? 'Belum ada presensi; akan tersinkron otomatis.' : absenceText(sheet?.absentStudents || [])}</TableCell><TableCell>{lines.length > 1 && <Button type="button" size="icon" variant="ghost" aria-label="Hapus baris" onClick={() => setLines((current) => current.filter((_, lineIndex) => lineIndex !== index))}><Trash2 className="h-4 w-4" /></Button>}</TableCell></TableRow>)}</TableBody></Table></div>
        <Button type="button" variant="outline" onClick={addLine}><Plus className="h-4 w-4" /> Tambah baris</Button>
        <div className="grid gap-2"><Label>Foto dokumentasi (opsional)</Label><Input type="file" accept="image/png,image/jpeg" onChange={(event) => setPhoto(event.target.files?.[0] || null)} /></div>
        <Signature value={signature || existingSignature} onChange={setSignature} userName={selectedTutorName} label="Paraf Tutor" />
        <div className="flex gap-2"><Button type="submit" disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan Jurnal'}</Button><Button type="button" variant="outline" onClick={() => { setFormOpen(false); setEditingBatchID('') }}>Batal</Button></div>
      </form>
    </FormCard>}

    <Card className="overflow-hidden rounded-2xl border border-border">
      <div className="border-b border-border px-5 py-4"><p className="font-semibold">Hari/Tanggal: Sabtu, {formatWibDate(`${date}T00:00:00+07:00`)}</p><p className="mt-1 text-sm text-muted-foreground">{selectedClass ? `Kelas ${String(selectedClass.jenjang || '')}${String(selectedClass.namaRombel || '')}` : 'Pilih kelas'} · {sheet?.attendanceStatus === 'belum_ada' ? 'Presensi belum diisi' : sheet?.attendanceStatus === 'sebagian' ? 'Presensi sedang dilengkapi' : 'Presensi tersinkron'}</p></div>
      <div className="overflow-x-auto"><Table className="min-w-[960px]"><TableHeader><TableRow><TableHead>Jam Ke</TableHead><TableHead>Nama Tutor</TableHead><TableHead>Mata Pelajaran</TableHead><TableHead>Materi</TableHead><TableHead>Peserta Didik Tidak Hadir</TableHead><TableHead>Paraf</TableHead></TableRow></TableHeader><TableBody>{(sheet?.lines || []).map((line) => {
        const isFirstBatchLine = firstLineIDs.has(line.id)
        return <TableRow key={line.id}><TableCell className="text-center font-medium">{line.jamKe}</TableCell><TableCell>{line.tutorNama}</TableCell><TableCell>{line.mapelNama}</TableCell><TableCell className="whitespace-pre-wrap">{line.materi}{line.kegiatan ? <div className="mt-1 text-xs text-muted-foreground">{line.kegiatan}</div> : null}</TableCell><TableCell className="max-w-xs text-sm">{absenceText(sheet?.absentStudents || [])}</TableCell>{isFirstBatchLine && <TableCell rowSpan={batchCounts.get(line.batchId) || 1} className="min-w-32 align-middle text-center">{line.tandaTangan ? <img src={line.tandaTangan} alt={`Paraf ${line.tutorNama}`} className="mx-auto max-h-16 max-w-28 object-contain" /> : <span className="text-muted-foreground">—</span>}{line.canEdit && !readOnly && <Button size="sm" variant="ghost" className="mt-1" onClick={() => openEdit(line.batchId)}><Pencil className="h-3.5 w-3.5" /> Edit</Button>}</TableCell>}</TableRow>
      })}{!loadingSheet && !(sheet?.lines || []).length && <EmptyState colSpan={6} label="Belum ada jurnal untuk kelas dan tanggal ini." />}</TableBody></Table></div>
    </Card>
  </div>
}
