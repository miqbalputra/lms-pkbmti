import { useEffect, useRef, useState, type MutableRefObject } from 'react'
import { toast } from 'sonner'
import { request } from '../lib/api'
import { lowercaseCPStart } from '../lib/report-narrative'
import { Button } from './ui/button'
import { Card } from './ui/card'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Select } from './ui/select'

type Grade = { jenjang: number; userId: string; subjects?: { id: string; namaMapel: string }[] }
export type ReportCPAssignments = {
  year: { id: string; namaTahunAjaran: string }
  isAdmin: boolean
  grades: Grade[]
  teachers: { id: string; nama: string }[]
}
type Definition = { id: string; label: string }
type Template = { revision: number; cps: Definition[]; archived: boolean }
export type ReportGradeCPActions = { save: () => Promise<void>; discard: () => Promise<void> }

export function ReportGradeCPPanel({ token, initialGrade, initialSubject, initialSemester, busy, actionsRef, setBusy, setDirty, leave }: {
  token: string
  initialGrade?: number
  initialSubject?: string
  initialSemester: string
  busy: boolean
  actionsRef: MutableRefObject<ReportGradeCPActions | null>
  setBusy: (busy: boolean) => void
  setDirty: (dirty: boolean) => void
  leave: (action: () => void) => void
}) {
  const [metadata, setMetadata] = useState<ReportCPAssignments | null>(null)
  const [grade, setGrade] = useState('')
  const [teacher, setTeacher] = useState('')
  const [semester, setSemester] = useState(initialSemester)
  const [subject, setSubject] = useState('')
  const [template, setTemplate] = useState<Template | null>(null)
  const [cps, setCPs] = useState<Definition[]>([])
  const [error, setError] = useState('')
  const [metadataRetry, setMetadataRetry] = useState(0)
  const [templateRetry, setTemplateRetry] = useState(0)
  const tokenRef = useRef(token)
  tokenRef.current = token
  const selected = metadata?.grades.find(row => String(row.jenjang) === grade)
  const year = metadata?.year.id || ''
  const path = year && grade && subject
    ? `/rapor-sumatif/cp-templates/${grade}?${new URLSearchParams({ tahunAjaranId: year, semester, mapelId: subject })}`
    : ''
  const templateDirty = !!template && JSON.stringify(cps) !== JSON.stringify(template.cps)

  useEffect(() => {
    const controller = new AbortController()
    void request('/rapor-sumatif/cp-assignments', tokenRef.current, 'GET', undefined, controller.signal)
      .then((data: ReportCPAssignments) => {
        if (controller.signal.aborted) return
        const row = data.grades.find(item => item.jenjang === initialGrade) || data.grades[0]
        setMetadata(data)
        setGrade(String(row?.jenjang || ''))
        setTeacher(row?.userId || '')
        setSubject(row?.subjects?.find(item => item.id === initialSubject)?.id || '')
      })
      .catch(e => { if (!controller.signal.aborted) setError(e.message) })
    return () => controller.abort()
  }, [initialGrade, initialSubject, metadataRetry])

  useEffect(() => {
    if (!path) return
    const controller = new AbortController()
    void request(path, tokenRef.current, 'GET', undefined, controller.signal)
      .then((data: Template) => { if (!controller.signal.aborted) { setTemplate(data); setCPs(data.cps) } })
      .catch(e => { if (!controller.signal.aborted) setError(e.message) })
    return () => controller.abort()
  }, [path, templateRetry])

  function changeSelection(change: () => void) {
    leave(() => { setTemplate(null); setCPs([]); setError(''); change() })
  }

  function editCPs(next: Definition[]) {
    setCPs(next)
    setDirty(JSON.stringify(next) !== JSON.stringify(template?.cps))
  }

  async function save() {
    setBusy(true)
    setError('')
    try {
      if (metadata?.isAdmin && selected) {
        await request(`/rapor-sumatif/cp-assignments/${grade}?${new URLSearchParams({ tahunAjaranId: year })}`, token, 'PUT', { userId: teacher })
        setMetadata({ ...metadata, grades: metadata.grades.map(row => row === selected ? { ...row, userId: teacher } : row) })
        toast.success(teacher ? `Guru CP Kelas ${grade} ditetapkan` : `Penugasan CP Kelas ${grade} dilepas`)
      } else if (template && path) {
        if (!cps.length || cps.some(cp => !cp.label.trim())) throw new Error('Isi deskripsi setiap CP sebelum menyimpan')
        const data: Template = await request(path, token, 'PUT', { revision: template.revision, cps })
        setTemplate(data)
        setCPs(data.cps)
        toast.success(`CP Kelas ${grade} tersimpan`)
      }
      setDirty(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'CP gagal disimpan')
      throw e
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    actionsRef.current = { save, discard: async () => {
      setTeacher(selected?.userId || '')
      setCPs(template?.cps || [])
      setError('')
      setDirty(false)
    } }
    return () => { actionsRef.current = null }
  })

  return <Card id="report-grade-cp" tabIndex={-1} className="space-y-4 p-4" aria-label="CP per jenjang">
    <div>
      <h3 className="font-bold">{metadata?.isAdmin ? 'Penugasan guru CP per jenjang' : 'CP per jenjang'}</h3>
      {year && <p className="text-sm text-muted-foreground">Tahun ajaran {metadata?.year.namaTahunAjaran}</p>}
    </div>
    {error && <p role="alert" className="rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-800">{error}</p>}
    {!metadata && !error && <p role="status">Memuat penugasan CP…</p>}
    {metadata && !metadata.grades.length && <p className="text-sm text-muted-foreground">{metadata.isAdmin ? 'Belum ada jenjang kelas pada tahun ajaran aktif.' : 'Admin belum menugaskan Anda untuk menyusun CP pada tahun ajaran aktif.'}</p>}
    {!!metadata?.grades.length && <>
      {!metadata.isAdmin && <p className="text-sm text-muted-foreground">Pilih jenjang, semester, dan mapel. Tambahkan deskripsi CP, lalu klik <strong>Simpan CP jenjang</strong>. Nilai siswa diisi dari tampilan nilai per rombel.</p>}
      <div className="grid gap-3 md:grid-cols-3">
        <div><Label htmlFor="report-cp-grade">Jenjang</Label><Select id="report-cp-grade" disabled={busy} value={grade} onChange={e => {
          const value = e.target.value
          changeSelection(() => { setGrade(value); setSubject(''); setTeacher(metadata.grades.find(row => String(row.jenjang) === value)?.userId || '') })
        }}>{metadata.grades.map(row => <option key={row.jenjang} value={row.jenjang}>Kelas {row.jenjang}</option>)}</Select></div>
        {metadata.isAdmin ? <div><Label htmlFor="report-cp-teacher">Guru penyusun CP</Label><Select id="report-cp-teacher" disabled={busy} value={teacher} onChange={e => { setTeacher(e.target.value); setDirty(e.target.value !== selected?.userId) }}>
          <option value="">Belum ditugaskan</option>
          {!!selected?.userId && !metadata.teachers.some(row => row.id === selected.userId) && <option value={selected.userId} disabled>Akun guru tidak aktif</option>}
          {metadata.teachers.map(row => <option key={row.id} value={row.id}>{row.nama}</option>)}
        </Select></div> : <>
          <div><Label htmlFor="report-cp-semester">Semester CP</Label><Select id="report-cp-semester" disabled={busy} value={semester} onChange={e => { const value = e.target.value; changeSelection(() => setSemester(value)) }}><option>Ganjil</option><option>Genap</option></Select></div>
          <div><Label htmlFor="report-cp-subject">Mata pelajaran CP</Label><Select id="report-cp-subject" disabled={busy} value={subject} onChange={e => { const value = e.target.value; changeSelection(() => setSubject(value)) }}>
            <option value="">Pilih mapel</option>{selected?.subjects?.map(row => <option key={row.id} value={row.id}>{row.namaMapel}</option>)}
          </Select></div>
        </>}
      </div>
      {metadata.isAdmin && <>
        <p className="text-sm text-muted-foreground">Satu guru menyusun CP untuk jenjang ini di semester Ganjil dan Genap. Saat guru diganti, daftar CP yang sudah tersimpan tetap tersedia bagi guru pengganti.</p>
        <Button disabled={busy || teacher === selected?.userId} onClick={() => void save().catch(() => {})}>Simpan penugasan</Button>
      </>}
      {!metadata.isAdmin && !selected?.subjects?.length && <p className="text-sm text-muted-foreground">Minta admin menambahkan mata pelajaran pada rombel jenjang ini.</p>}
    </>}
    {path && !template && !error && <p role="status">Memuat daftar CP…</p>}
    {template && <>
      <p className="text-sm text-muted-foreground">Daftar ini digunakan oleh rombel satu jenjang untuk mapel dan semester ini. Rombel dengan daftar CP lama yang berbeda tetap memakai daftar lamanya.</p>
      {template.archived && <p role="status" className="text-sm text-muted-foreground">Semester diarsipkan. Daftar CP hanya dapat dilihat.</p>}
      {cps.map((cp, index) => <div key={cp.id || index} className="flex items-end gap-2">
        <div className="flex-1"><Label htmlFor={`report-grade-label-${index}`}>CP {index + 1}</Label><Input id={`report-grade-label-${index}`} maxLength={500} value={cp.label} disabled={busy || template.archived} onChange={e => editCPs(cps.map((row, i) => i === index ? { ...row, label: lowercaseCPStart(e.target.value.trimStart()) } : row))} /></div>
        {!template.archived && <Button variant="ghost" disabled={busy} aria-label={`Hapus CP ${index + 1}`} onClick={() => editCPs(cps.filter((_, i) => i !== index))}>Hapus</Button>}
      </div>)}
      {!template.archived && <div className="flex flex-wrap gap-2">
        <Button variant="outline" disabled={busy || cps.length >= 100} onClick={() => editCPs([...cps, { id: '', label: '' }])}>Tambah CP</Button>
        <Button disabled={busy || !templateDirty || !cps.length} onClick={() => void save().catch(() => {})}>Simpan CP jenjang</Button>
      </div>}
    </>}
    {error && (!metadata || path) && <Button variant="outline" disabled={busy} onClick={() => changeSelection(() => {
      if (path) setTemplateRetry(value => value + 1)
      else setMetadataRetry(value => value + 1)
    })}>Muat ulang</Button>}
  </Card>
}
