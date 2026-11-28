import { useEffect, useRef, useState, type RefObject } from 'react'
import { toast } from 'sonner'
import { request } from '../lib/api'
import { useReportAutosave } from '../lib/useReportAutosave'
import { Button } from './ui/button'
import { Card } from './ui/card'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Select } from './ui/select'

type Counts = { sakit: number; izin: number; alpa: number }
type Row = { studentId: string; status: string; source: string; attendance: Counts & { conflicts: string[] }; manual: Counts | null }
type Grid = { period: { revision: number }; mode: string; rows: Row[]; readOnly: boolean }
type Draft = Record<keyof Counts, string>
const fields = [['sakit', 'Sakit'], ['izin', 'Izin'], ['alpa', 'Alpa']] as const
const sourceLabels: Record<string, string> = { automatic: 'Otomatis dari presensi', manual: 'Rekap manual', historical: 'Rekap impor Excel', frozen: 'Rapor final / terbit' }
const attendanceValidation = 'Lengkapi Sakit, Izin, dan Alpa dengan bilangan bulat 0–366 hari. Isi 0 jika nihil; total maksimal 366 hari.'
export type ReportAttendanceActions = { save: () => Promise<void>; discard: () => Promise<void> }

function attendanceCounts(draft: Draft): Counts | null {
  const values = fields.map(([key]) => draft[key].trim() === '' ? NaN : Number(draft[key]))
  if (values.some(value => !Number.isInteger(value) || value < 0 || value > 366) || values.reduce((sum, value) => sum + value, 0) > 366) return null
  return { sakit: values[0], izin: values[1], alpa: values[2] }
}

export function ReportAttendancePanel({ token, qs, students, isAdmin, canEdit, setDirty, setParentBusy, onSaved, actionsRef }: {
  token: string; qs: string; students: { id: string; nama: string; nis: string }[]
  isAdmin: boolean; canEdit: boolean; setDirty: (dirty: boolean) => void; setParentBusy: (busy: boolean) => void; onSaved: (kind: 'manual' | 'automatic') => Promise<void>
  actionsRef: RefObject<ReportAttendanceActions | null>
}) {
  const [grid, setGrid] = useState<Grid | null>(null)
  const [mode, setMode] = useState('automatic')
  const [drafts, setDrafts] = useState<Record<string, Draft>>({})
  const [saving, setSaving] = useState<'manual' | 'automatic' | null>(null)
  const busy = !!saving
  const [error, setError] = useState('')
  const [saveError, setSaveError] = useState('')
  const [saved, setSaved] = useState<'manual' | 'automatic' | null>(null)
  const [reload, setReload] = useState(0)
  const latestDrafts = useRef(drafts)
  latestDrafts.current = drafts
  const saveInFlight = useRef<Promise<void> | null>(null)
  const tokenRef = useRef(token)
  tokenRef.current = token
  const modeChanged = !!grid && mode !== grid.mode
  const hasDrafts = Object.keys(drafts).length > 0
  const hasFinal = grid?.rows.some(row => ['final', 'published'].includes(row.status))
  const writable = !!grid && canEdit && !grid.readOnly
  const readyRows = Object.entries(drafts).flatMap(([studentId, draft]) => {
    const counts = attendanceCounts(draft)
    return counts ? [{ studentId, ...counts }] : []
  })
  const incompleteRows = Object.keys(drafts).length > readyRows.length
  const saveMode = isAdmin && modeChanged && !hasFinal && !hasDrafts
  useReportAutosave(writable && !busy && !saveError && (saveMode || !modeChanged && readyRows.length > 0),
    saveMode ? mode : drafts, () => save(saveMode, true))
  useEffect(() => {
    let cancelled = false
    setGrid(null); setDrafts({}); setError(''); setSaveError(''); setSaved(null)
    void request('/rapor-sumatif/attendance?' + qs, tokenRef.current).then(data => {
      if (cancelled) return
      setGrid(data); setMode(data.mode)
    }).catch(e => { if (!cancelled) setError(e instanceof Error ? e.message : 'Rekap presensi gagal dimuat') })
    return () => { cancelled = true }
  }, [qs, reload])

  function save(changeMode: boolean, automatic = false) {
    if (!saveInFlight.current) {
      saveInFlight.current = persist(changeMode, automatic).finally(() => { saveInFlight.current = null })
    }
    return saveInFlight.current
  }
  async function persist(changeMode: boolean, automatic: boolean) {
    if (!grid) throw new Error('Tunggu hingga rekap presensi selesai dimuat')
    if (!changeMode && !automatic && incompleteRows) throw new Error(attendanceValidation)
    const kind = automatic ? 'automatic' : 'manual'
    setSaving(kind); setParentBusy(true); setSaveError('')
    try {
      if (changeMode) {
        await request('/rapor-sumatif/attendance-mode?' + qs, token, 'PUT', { revision: grid.period.revision, mode })
        setDirty(false)
        try {
          const updated: Grid = await request('/rapor-sumatif/attendance?' + qs, token)
          setGrid(updated); setMode(updated.mode)
        } catch {
          setGrid(null); setError('Sumber tersimpan. Muat ulang untuk melihat rekap terbaru.')
        }
      } else {
        const period: Grid['period'] = await request('/rapor-sumatif/attendance?' + qs, token, 'PUT', { revision: grid.period.revision, rows: readyRows })
        const remaining = { ...latestDrafts.current }
        const savedRows = new Map(readyRows.map(({ studentId, ...counts }) => [studentId, counts]))
        for (const studentId of savedRows.keys()) {
          if (remaining[studentId] === drafts[studentId]) delete remaining[studentId]
        }
        latestDrafts.current = remaining
        setDrafts(remaining); setDirty(Object.keys(remaining).length > 0)
        setGrid({ ...grid, period, rows: grid.rows.map(row => {
          const counts = savedRows.get(row.studentId)
          return counts ? { ...row, status: 'draft', manual: counts, attendance: { ...counts, conflicts: [] } } : row
        }) })
      }
      await onSaved(kind)
      setSaved(kind)
      if (!automatic) toast.success(changeMode ? 'Sumber presensi rapor tersimpan untuk kelas dan semester ini.' : 'Rekap presensi tersimpan dan digunakan pada rapor.')
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Penyimpanan gagal. Isian tetap ada.')
      throw e
    }
    finally { setSaving(null); setParentBusy(false) }
  }
  useEffect(() => {
    const actions = {
      save: async () => { await save(mode !== grid?.mode) },
      discard: async () => { setDrafts({}); setMode(grid?.mode || 'automatic'); setDirty(false); setSaveError('') },
    }
    actionsRef.current = actions
    return () => { if (actionsRef.current === actions) actionsRef.current = null }
  })
  function submit(changeMode: boolean) {
    void save(changeMode).catch(e => toast.error(e instanceof Error ? e.message : 'Penyimpanan gagal. Isian tetap ada.'))
  }
  if (error) return <Card className="space-y-3 p-4"><p role="alert">{error}</p><Button variant="outline" onClick={() => setReload(v => v + 1)}>Muat ulang rekap</Button></Card>
  if (!grid) return <Card className="p-4" role="status">Memuat rekap presensi…</Card>
  return <div className="space-y-4">
    <Card className="space-y-4 p-4">
      <div><h3 className="text-lg font-bold">Presensi untuk rapor</h3><p className="text-sm text-muted-foreground">Satu rekap untuk satu siswa dalam semester yang dipilih. Jumlah di bawah digunakan pada bagian ketidakhadiran rapor.</p></div>
      <ol className="grid gap-3 text-sm sm:grid-cols-3">
        {['Admin memilih sumber rekap kelas.', 'Wali kelas menyalin jumlah hari dari catatan kertas.', 'Rekap disimpan otomatis. Periksa rapor sebelum finalisasi.'].map((step, i) => <li key={step} className="flex items-start gap-2 rounded-lg bg-muted/50 p-3"><span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">{i + 1}</span><span>{step}</span></li>)}
      </ol>
      {isAdmin ? <div className="space-y-2 rounded-lg border p-3">
        <Label htmlFor="report-attendance-mode">Sumber presensi kelas dan semester ini</Label>
        <div className="flex flex-wrap items-center gap-2"><Select id="report-attendance-mode" aria-label="Sumber presensi rapor" className="sm:max-w-md" value={mode} disabled={!writable || busy || hasDrafts || hasFinal} onChange={e => { setMode(e.target.value); setDirty(e.target.value !== grid.mode); setSaveError('') }}><option value="automatic">Otomatis dari presensi aplikasi</option><option value="manual">Rekap manual dari catatan kertas</option></Select><Button variant="outline" disabled={!writable || busy || hasDrafts || hasFinal || !modeChanged} onClick={() => submit(true)}>Simpan sumber presensi</Button></div>
        <p className="text-xs text-muted-foreground">Pengaturan hanya berlaku untuk kelas dan semester yang dipilih. Mengganti sumber tidak menghapus presensi harian atau rekap manual yang sudah disimpan.</p>
        {hasFinal && <p className="text-sm text-amber-700">Sumber dikunci karena sudah ada rapor final / terbit.</p>}
      </div> : <p className="rounded-lg bg-muted/50 p-3 text-sm">Sumber kelas: <strong>{grid.mode === 'manual' ? 'Rekap manual dari catatan kertas' : 'Otomatis dari presensi aplikasi'}</strong>. Pengaturan sumber dilakukan admin.</p>}
      {saveError ? <p role="alert" className="text-sm text-destructive">Presensi belum tersimpan. {saveError}</p>
        : writable && <p role="status" className="text-sm text-muted-foreground">{busy ? 'Menyimpan presensi…' : modeChanged ? 'Pilihan sumber akan disimpan otomatis.' : incompleteRows ? 'Lengkapi Sakit, Izin, dan Alpa. Isi 0 jika nihil.' : hasDrafts ? 'Menunggu simpan otomatis…' : saved === 'automatic' ? 'Presensi tersimpan otomatis.' : saved ? 'Presensi tersimpan.' : ''}</p>}
      {grid.readOnly && <p className="text-sm text-amber-700">Tahun ajaran atau semester ini sudah diarsipkan. Rekap hanya dapat dilihat.</p>}
      {grid.mode === 'manual' ? <p className="text-sm">Isi jumlah <strong>hari</strong> Sakit, Izin, dan Alpa. Tulis <strong>0</strong> jika tidak ada; kolom kosong berarti belum diisi. Hanya baris yang Anda ubah yang akan disimpan.</p> : <p className="text-sm">Rekap dihitung dari presensi aplikasi per hari. Untuk memakai catatan kertas, minta admin memilih sumber rekap manual.</p>}
    </Card>
    <Card className="overflow-auto p-3">
      <table className="w-full text-sm" aria-label="Rekap presensi rapor"><thead><tr><th className="min-w-44 p-2 text-left">Peserta didik</th>{fields.map(([key, label]) => <th key={key} className="min-w-24 p-2">{label} (hari)</th>)}<th className="min-w-44 p-2 text-left">Keterangan</th></tr></thead><tbody>{grid.rows.map(row => {
        const student = students.find(s => s.id === row.studentId)
        const editable = writable && grid.mode === 'manual' && row.source === 'manual' && !['final', 'published'].includes(row.status)
        const draft = drafts[row.studentId]
        return <tr key={row.studentId} className="border-t"><td className="p-2 font-medium">{student?.nama || 'Siswa'}<p className="text-xs font-normal text-muted-foreground">NIS {student?.nis || '—'}</p></td>{fields.map(([key, label]) => <td key={key} className="p-2">{editable ? <Input type="number" min="0" max="366" step="1" inputMode="numeric" aria-required="true" className="w-24 border-l-4 border-l-primary bg-primary/5" aria-label={`${label} ${student?.nama || row.studentId}`} placeholder="Belum diisi" value={draft ? draft[key] : row.manual?.[key] ?? ''} disabled={saving === 'manual' || modeChanged} onChange={e => {
          if (e.target.validity.badInput) return
          const current = latestDrafts.current[row.studentId] || { sakit: String(row.manual?.sakit ?? ''), izin: String(row.manual?.izin ?? ''), alpa: String(row.manual?.alpa ?? '') }
          const next = { ...latestDrafts.current, [row.studentId]: { ...current, [key]: e.target.value } }
          latestDrafts.current = next
          setDrafts(next); setDirty(true); setSaveError('')
        }} /> : <span className="block text-center">{row.source === 'manual' && !row.manual ? '—' : row.attendance[key]}</span>}</td>)}<td className="space-y-1 p-2"><p>{sourceLabels[row.source] || row.source}</p>{row.source === 'manual' && !row.manual && !draft && <p className="text-xs text-amber-700">Belum diisi</p>}{draft && <p className="text-xs text-amber-700">Perubahan belum disimpan</p>}{row.attendance.conflicts?.length > 0 && <p className="text-xs text-red-700">Konflik status pada tanggal yang sama. Periksa presensi harian.</p>}</td></tr>
      })}</tbody></table>
      {grid.rows.length === 0 && <p className="p-3 text-sm">Belum ada peserta didik pada kelas ini.</p>}
    </Card>
    {writable && grid.mode === 'manual' && <div className="flex flex-wrap items-center gap-3"><Button disabled={busy || modeChanged || !hasDrafts} onClick={() => submit(false)}>Simpan rekap presensi</Button><span className="text-xs text-muted-foreground">Rapor final / terbit dan rekap hasil impor dikunci untuk menjaga data lama.</span></div>}
  </div>
}
