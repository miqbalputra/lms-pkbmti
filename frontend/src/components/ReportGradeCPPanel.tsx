import { useEffect, useRef, useState, type MutableRefObject } from 'react'
import { toast } from 'sonner'
import { request } from '../lib/api'
import { useReportAutosave } from '../lib/useReportAutosave'
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
export type ReportGradeCPEntry = 'edit' | 'add' | 'other-subject'
export const maxReportCPs = 100

function sameCPs(left: Definition[], right: Definition[]) {
  return left.length === right.length && left.every((cp, index) => cp.id === right[index].id && cp.label.trim() === right[index].label.trim())
}

export function ReportGradeCPPanel({ token, initialGrade, initialSubject, initialSemester, entry, busy, actionsRef, setBusy, setDirty, leave }: {
  token: string
  initialGrade?: number
  initialSubject?: string
  initialSemester: string
  entry?: ReportGradeCPEntry
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
  const [lastSave, setLastSave] = useState<'automatic' | 'manual'>()
  const [metadataRetry, setMetadataRetry] = useState(0)
  const [templateRetry, setTemplateRetry] = useState(0)
  const [focusCP, setFocusCP] = useState<number | null>(null)
  const initialEntry = useRef(entry)
  const subjectSelect = useRef<HTMLSelectElement>(null)
  const cpInput = useRef<HTMLInputElement>(null)
  const latestCPs = useRef(cps)
  latestCPs.current = cps
  const saveInFlight = useRef<Promise<void> | null>(null)
  const tokenRef = useRef(token)
  tokenRef.current = token
  const selected = metadata?.grades.find(row => String(row.jenjang) === grade)
  const subjectName = selected?.subjects?.find(row => row.id === subject)?.namaMapel
  const year = metadata?.year.id || ''
  const path = year && grade && subject
    ? `/rapor-sumatif/cp-templates/${grade}?${new URLSearchParams({ tahunAjaranId: year, semester, mapelId: subject })}`
    : ''
  const templateDirty = !!template && !sameCPs(cps, template.cps)
  const missingDescriptions = !cps.length || cps.some(cp => !cp.label.trim())
  const removesSavedCP = !!template?.cps.some(saved => !cps.some(cp => cp.id === saved.id))
  const assignmentDirty = !!metadata?.isAdmin && !!selected && teacher !== selected.userId
  const canAutosaveTemplate = templateDirty && !template?.archived && !missingDescriptions && !removesSavedCP
  useReportAutosave(!busy && !error && (assignmentDirty || canAutosaveTemplate), metadata?.isAdmin ? teacher : cps, () => save(true))

  useEffect(() => {
    const controller = new AbortController()
    void request('/rapor-sumatif/cp-assignments', tokenRef.current, 'GET', undefined, controller.signal)
      .then((data: ReportCPAssignments) => {
        if (controller.signal.aborted) return
        const row = data.grades.find(item => item.jenjang === initialGrade) || data.grades[0]
        setMetadata(data)
        setGrade(String(row?.jenjang || ''))
        setTeacher(row?.userId || '')
        setSubject(row?.subjects?.some(item => item.id === initialSubject) ? initialSubject || '' : '')
      })
      .catch(e => { if (!controller.signal.aborted) setError(e.message) })
    return () => controller.abort()
  }, [initialGrade, initialSubject, metadataRetry])

  useEffect(() => {
    if (metadata && initialEntry.current === 'other-subject') {
      subjectSelect.current?.focus()
      initialEntry.current = undefined
    }
  }, [metadata])

  useEffect(() => {
    if (!path) return
    const controller = new AbortController()
    void request(path, tokenRef.current, 'GET', undefined, controller.signal)
      .then((data: Template) => {
        if (controller.signal.aborted) return
        const action = initialEntry.current
        initialEntry.current = undefined
        setTemplate(data)
        if (action === 'add' && !data.archived && data.cps.length < maxReportCPs) {
          setCPs([...data.cps, { id: '', label: '' }])
          setFocusCP(data.cps.length)
          setDirty(true)
        } else {
          setCPs(data.cps)
          if (action === 'edit' && !data.archived && data.cps.length) setFocusCP(0)
        }
      })
      .catch(e => { if (!controller.signal.aborted) setError(e.message) })
    return () => controller.abort()
  }, [path, templateRetry, setDirty])

  useEffect(() => {
    if (focusCP !== null) {
      cpInput.current?.focus()
      setFocusCP(null)
    }
  }, [focusCP])

  function changeSelection(change: () => void, retry = false) {
    leave(() => {
      if (!retry) initialEntry.current = undefined
      setTemplate(null); setCPs([]); setError(''); setLastSave(undefined); change()
    })
  }

  function editCPs(next: Definition[]) {
    latestCPs.current = next
    setCPs(next)
    setDirty(!!template && !sameCPs(next, template.cps))
  }

  function addCP() {
    editCPs([...cps, { id: '', label: '' }])
    setFocusCP(cps.length)
  }

  function save(automatic = false) {
    if (!saveInFlight.current) {
      saveInFlight.current = persist(automatic).finally(() => { saveInFlight.current = null })
    }
    return saveInFlight.current
  }

  async function persist(automatic: boolean) {
    if (template && missingDescriptions) throw new Error('Isi deskripsi setiap CP sebelum menyimpan')
    setBusy(true)
    setError('')
    try {
      if (metadata?.isAdmin && selected) {
        await request(`/rapor-sumatif/cp-assignments/${grade}?${new URLSearchParams({ tahunAjaranId: year })}`, token, 'PUT', { userId: teacher })
        setMetadata({ ...metadata, grades: metadata.grades.map(row => row === selected ? { ...row, userId: teacher } : row) })
        setLastSave(automatic ? 'automatic' : 'manual')
        if (!automatic) toast.success(teacher ? `Guru CP Kelas ${grade} ditetapkan` : `Penugasan CP Kelas ${grade} dilepas`)
        setDirty(false)
      } else if (template && path) {
        const data: Template = await request(path, token, 'PUT', { revision: template.revision, cps: latestCPs.current })
        // The row order stays fixed while saving. Keep edits typed during the
        // request and copy back the server IDs, including IDs for new CPs.
        const next = latestCPs.current.map((cp, index) => ({ ...cp, id: data.cps[index].id }))
        latestCPs.current = next
        setTemplate(data)
        setCPs(next)
        setDirty(!sameCPs(next, data.cps))
        setLastSave(automatic ? 'automatic' : 'manual')
        if (!automatic) toast.success(`CP Kelas ${grade} tersimpan`)
      }
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

  return <Card id="report-grade-cp" tabIndex={-1} className="space-y-4 p-4" role="region" aria-labelledby="report-grade-cp-title">
    <div>
      <h3 id="report-grade-cp-title" className="text-lg font-bold">{metadata?.isAdmin ? 'Penugasan guru penyusun CP' : 'Kelola Capaian Pembelajaran (CP)'}</h3>
      {year && <p className="text-sm text-muted-foreground">Tahun ajaran {metadata?.year.namaTahunAjaran}</p>}
    </div>
    {error && !template && <p role="alert" className="rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-800">{error}</p>}
    {!metadata && !error && <p role="status">Memuat penugasan CP…</p>}
    {metadata && !metadata.grades.length && <p className="text-sm text-muted-foreground">{metadata.isAdmin ? 'Belum ada jenjang kelas pada tahun ajaran aktif.' : 'Admin belum menugaskan Anda untuk menyusun CP pada tahun ajaran aktif.'}</p>}
    {!!metadata?.grades.length && <>
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
          <div><Label htmlFor="report-cp-subject">Mata pelajaran CP</Label><Select ref={subjectSelect} id="report-cp-subject" disabled={busy || !selected?.subjects?.length} value={subject} onChange={e => { const value = e.target.value; changeSelection(() => setSubject(value)) }}>
            <option value="">Pilih mata pelajaran</option>{selected?.subjects?.map(row => <option key={row.id} value={row.id}>{row.namaMapel}</option>)}
          </Select></div>
        </>}
      </div>
      {metadata.isAdmin && <>
        <p className="text-sm text-muted-foreground">Satu guru menyusun CP untuk jenjang ini di semester Ganjil dan Genap. Saat guru diganti, daftar CP yang sudah tersimpan tetap tersedia bagi guru pengganti.</p>
        <Button disabled={busy || teacher === selected?.userId} onClick={() => void save().catch(() => {})}>Simpan penugasan</Button>
        <p role="status" className="text-sm text-muted-foreground">{busy ? 'Menyimpan penugasan…' : assignmentDirty ? 'Perubahan penugasan belum tersimpan.' : lastSave ? 'Penugasan tersimpan.' : 'Perubahan penugasan disimpan otomatis.'}</p>
      </>}
      {!metadata.isAdmin && !selected?.subjects?.length && <p className="text-sm text-muted-foreground">Minta admin menambahkan mata pelajaran pada rombel jenjang ini.</p>}
      {!metadata.isAdmin && !!selected?.subjects?.length && !subject && <p className="rounded-lg border bg-muted/40 p-3 text-sm">Pilih <strong>Mata pelajaran CP</strong> untuk menambah atau mengedit daftar CP mapel tersebut.</p>}
    </>}
    {path && !template && !error && <p role="status">Memuat daftar CP…</p>}
    {template && <section className="space-y-3 border-t pt-4" aria-labelledby="report-grade-cp-list-title">
      <div>
        <h4 id="report-grade-cp-list-title" className="font-semibold">Daftar CP {subjectName}</h4>
        <p className="mt-1 text-sm text-muted-foreground">Kelas {grade} · Semester {semester}. Daftar CP dipakai bersama oleh rombel satu jenjang. Rombel dengan daftar CP lama yang berbeda tetap memakai daftar lamanya.</p>
      </div>
      {template.archived && <p role="status" className="text-sm text-muted-foreground">Semester diarsipkan. Daftar CP hanya dapat dilihat.</p>}
      {cps.length === 0 && <p className="text-sm text-muted-foreground">Belum ada CP untuk mata pelajaran ini.</p>}
      {cps.map((cp, index) => <div key={index} className="flex items-end gap-2">
        <div className="min-w-0 flex-1"><Label htmlFor={`report-grade-label-${index}`}>CP {index + 1}</Label><Input ref={index === focusCP ? cpInput : undefined} id={`report-grade-label-${index}`} maxLength={500} placeholder="Tulis deskripsi capaian pembelajaran" value={cp.label} disabled={template.archived} onChange={e => editCPs(latestCPs.current.map((row, i) => i === index ? { ...row, label: lowercaseCPStart(e.target.value.trimStart()) } : row))} /></div>
        {!template.archived && <Button variant="ghost" disabled={busy} aria-label={`Hapus CP ${index + 1}`} onClick={() => editCPs(cps.filter((_, i) => i !== index))}>Hapus</Button>}
      </div>)}
      {!template.archived && <>
        <div className="flex flex-wrap gap-2">
          {cps.length < maxReportCPs && <Button variant="outline" disabled={busy} onClick={addCP}>Tambah CP {cps.length + 1}</Button>}
          <Button disabled={busy || !templateDirty || missingDescriptions} onClick={() => void save().catch(() => {})}>Simpan CP jenjang</Button>
        </div>
        {cps.length >= maxReportCPs && <p className="text-sm text-muted-foreground">Daftar mencapai batas {maxReportCPs} CP.</p>}
        {error ? <p role="alert" className="rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-800">CP belum tersimpan. {error}</p>
          : <p role="status" className="text-sm text-muted-foreground">{busy ? 'Menyimpan CP…'
            : templateDirty && missingDescriptions ? 'Isi deskripsi setiap CP sebelum menyimpan.'
            : templateDirty && removesSavedCP ? 'Klik Simpan CP jenjang untuk menghapus CP dari daftar.'
            : templateDirty ? 'Menunggu simpan otomatis…'
            : lastSave === 'automatic' ? 'Tersimpan otomatis.'
            : lastSave === 'manual' ? 'CP tersimpan.'
            : 'Perubahan CP disimpan otomatis setelah Anda berhenti mengetik.'}</p>}
      </>}
    </section>}
    {error && (!metadata || path) && <Button variant="outline" disabled={busy} onClick={() => changeSelection(() => {
      if (path) setTemplateRetry(value => value + 1)
      else setMetadataRetry(value => value + 1)
    }, true)}>Muat ulang</Button>}
  </Card>
}
