import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select } from '../components/ui/select'
import { request } from '../lib/api'
import { automaticReportNarrative, lowercaseCPStart } from '../lib/report-narrative'

type Student = { id: string; nama: string; nis: string; nisn: string }
type ClassRow = { id: string; jenjang: number; namaRombel: string; tahunAjaranId: string; waliKelasId?: string; tahunAjaran: { id: string; namaTahunAjaran: string; isAktif: boolean } }
type Subject = { mapelId: string; name: string; group: string }
type Config = { subjects: Subject[]; program: string; phase: string; city: string; date: string; teacher: string; teacherNip: string; principal: string; principalNip: string; principalSignature?: string; extrasEnabled?: boolean; extraActivities?: string[] }
type Period = { id: string; revision: number }
type CP = { id: string; label: string; urutan: number }
type Score = { cpId: string; studentId: string; value: number | null }
type Narrative = { studentId: string; strength: string; growth: string; reviewedRevision?: number }
type Details = { note: string; promotion: string; nextClass: string; extras: { name: string; description: string }[]; identity?: Record<string, string> }
type DocumentSubject = Subject & { score: number | null; strength: string; growth: string; stale: boolean; complete: boolean }
type ReportDocument = { student: Student; subjects: DocumentSubject[]; attendance: { sakit: number; izin: number; alpa: number }; details: Details; issues: string[] }
type RecordRow = { studentId: string; status: string; version: number }
type ImportStudent = { row: number; name: string; nis: string; nisn: string; studentId: string }
type ImportSubject = { sheet: string; name: string; mapelId: string; labels: string[]; issues: string[] }
type ImportPreview = { id: string; preview: { students: ImportStudent[]; subjects: ImportSubject[]; issues: string[]; source?: Record<string, string> }; students: Student[]; subjects: Subject[] }
const base = import.meta.env.VITE_API_BASE_URL || '/api'
const textClass = 'min-h-24 w-full rounded-lg border border-input bg-background p-3 text-sm disabled:opacity-60'
const emptyDetails: Details = { note: '', promotion: '', nextClass: '', extras: [] }

export function RaporSumatifView({ token, readOnly, nilaiOnly = false }: { token: string; readOnly: boolean; nilaiOnly?: boolean }) {
  const [classes, setClasses] = useState<ClassRow[]>([])
  const [years, setYears] = useState<{ id: string; namaTahunAjaran: string; isAktif: boolean }[]>([])
  const [year, setYear] = useState('')
  const [kelasId, setKelasId] = useState('')
  const [semester, setSemester] = useState('Ganjil')
  const [period, setPeriod] = useState<Period>({ id: '', revision: 0 })
  const [config, setConfig] = useState<Config | null>(null)
  const [students, setStudents] = useState<Student[]>([])
  const [records, setRecords] = useState<RecordRow[]>([])
  const [mapelId, setMapelId] = useState('')
  const [studentId, setStudentId] = useState('')
  const [cps, setCPs] = useState<CP[]>([])
  const [scores, setScores] = useState<Score[]>([])
  const [narratives, setNarratives] = useState<Narrative[]>([])
  const [narrativeEdits, setNarrativeEdits] = useState<Set<string>>(new Set())
  const [doc, setDoc] = useState<ReportDocument | null>(null)
  const [record, setRecord] = useState<RecordRow | null>(null)
  const [details, setDetails] = useState<Details>(emptyDetails)
  const [tab, setTab] = useState(nilaiOnly ? 'nilai' : 'rapor')
  const [busy, setBusy] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [canManage, setCanManage] = useState(false)
  const [canInput, setCanInput] = useState<string[]>([])
  const [isAdmin, setIsAdmin] = useState(false)
  const [previewURL, setPreviewURL] = useState('')
  const previewRef = useRef('')
  const [ai, setAI] = useState({ provider: '', model: '', geminiConfigured: false, openaiConfigured: false })
  const [suggestion, setSuggestion] = useState<{ studentId: string; strength: string; growth: string } | null>(null)
  const [suggestionQueue, setSuggestionQueue] = useState<{ studentId: string; strength: string; growth: string }[]>([])
  const [aiProgress, setAIProgress] = useState<{ done: number; total: number } | null>(null)
  const aiController = useRef<AbortController | null>(null)
  const [importData, setImportData] = useState<ImportPreview | null>(null)
  const [importStudents, setImportStudents] = useState<Record<number, string>>({})
  const [importSubjects, setImportSubjects] = useState<Record<string, string>>({})
  const [ledger, setLedger] = useState<{ student: Student; subjects: DocumentSubject[]; total: number | null; rank: number | null }[]>([])
  const generation = useRef(0)
  const qs = new URLSearchParams({ kelasId, semester, tahunAjaranId: year }).toString()
  const locked = (id: string) => records.some(r => r.studentId === id && ['final', 'published'].includes(r.status))
  const canWrite = !readOnly && canInput.includes(mapelId)
  const isArchived = !!year && !years.find(y => y.id === year)?.isAktif

  useEffect(() => {
    void Promise.all([request('/rapor-sumatif/classes', token), request('/rapor-sumatif/ai/settings', token)])
      .then(([k, a]) => { const y = [...new Map<string, ClassRow['tahunAjaran']>((k as ClassRow[]).map(c => [c.tahunAjaranId, c.tahunAjaran])).values()]; setClasses(k); setYears(y); setYear(y.find(r => r.isAktif)?.id || y[0]?.id || ''); setAI(a) })
      .catch(e => toast.error(e.message))
  }, [token])
  useEffect(() => {
    const handle = (e: BeforeUnloadEvent) => { if (dirty) e.preventDefault() }
    window.addEventListener('beforeunload', handle)
    return () => window.removeEventListener('beforeunload', handle)
  }, [dirty])
  useEffect(() => () => { if (previewRef.current) URL.revokeObjectURL(previewRef.current); aiController.current?.abort() }, [])

  async function refreshPeriod() {
    const data = await request('/rapor-sumatif/period?' + qs, token)
    setPeriod(data.period); setConfig(data.config); setStudents(data.students); setRecords(data.records)
    setCanManage(data.canManage); setCanInput(data.canInput); setIsAdmin(data.isAdmin)
    return data
  }
  useEffect(() => {
    const current = ++generation.current
    setConfig(null); setDoc(null); setCPs([]); setScores([]); setNarratives([]); setMapelId(''); setStudentId(''); setDirty(false); setImportData(null); setLedger([])
    if (!kelasId) return
    void request('/rapor-sumatif/period?' + qs, token).then(data => {
      if (current !== generation.current) return
      setPeriod(data.period); setConfig(data.config); setStudents(data.students); setRecords(data.records)
      setCanManage(data.canManage); setCanInput(data.canInput); setIsAdmin(data.isAdmin)
    }).catch(e => { if (current === generation.current) toast.error(e.message) })
  }, [kelasId, semester, year, token]) // eslint-disable-line react-hooks/exhaustive-deps

  async function loadGrid() {
    if (!mapelId) return
    const data = await request('/rapor-sumatif/grid?' + qs + '&mapelId=' + mapelId, token)
    setPeriod(data.period); setCPs(data.cps); setScores(data.scores); setNarratives(data.narratives); setNarrativeEdits(new Set()); setDirty(false)
  }
  useEffect(() => {
    let cancelled = false
    setCPs([]); setScores([]); setNarratives([]); setNarrativeEdits(new Set()); setSuggestion(null)
    if (!mapelId) return
    void request('/rapor-sumatif/grid?' + qs + '&mapelId=' + mapelId, token).then(data => {
      if (cancelled) return
      setPeriod(data.period); setCPs(data.cps); setScores(data.scores); setNarratives(data.narratives); setDirty(false)
    }).catch(e => { if (!cancelled) toast.error(e.message) })
    return () => { cancelled = true }
  }, [mapelId, qs, token])

  async function loadDocument() {
    if (!studentId) return
    const data = await request('/rapor-sumatif/document/' + studentId + '?' + qs, token)
    setDoc(data.document); setRecord(data.record); setPeriod(data.period); setDetails({ ...emptyDetails, ...data.document.details, extras: data.document.details.extras || [] })
  }
  useEffect(() => {
    let cancelled = false
    setDoc(null); setRecord(null)
    if (!studentId || !canManage && !readOnly) return
    void request('/rapor-sumatif/document/' + studentId + '?' + qs, token).then(data => {
      if (cancelled) return
      setDoc(data.document); setRecord(data.record); setPeriod(data.period); setDetails({ ...emptyDetails, ...data.document.details, extras: data.document.details.extras || [] })
    }).catch(e => { if (!cancelled) toast.error(e.message) })
    return () => { cancelled = true }
  }, [studentId, qs, token, canManage, readOnly])

  async function run(action: () => Promise<void>) {
    setBusy(true)
    try { await action() } catch (e) { toast.error(e instanceof Error ? e.message : 'Permintaan gagal') } finally { setBusy(false) }
  }
  function changeFilter(action: () => void) {
    if (dirty && !window.confirm('Ada perubahan belum disimpan. Buang perubahan dan ganti pilihan?')) return
    setDirty(false); action()
  }
  function scoreValue(id: string, cpId: string) { return scores.find(s => s.studentId === id && s.cpId === cpId)?.value ?? null }
  function updateScore(id: string, cpId: string, raw: string) {
    const value = raw.trim() === '' ? null : Number(raw)
    setScores(rows => [...rows.filter(r => !(r.studentId === id && r.cpId === cpId)), { studentId: id, cpId, value }]); setDirty(true)
  }
  function editNarrative(id: string, field: 'strength' | 'growth', value: string) {
    setNarratives(rows => { const prev = rows.find(r => r.studentId === id) || { studentId: id, ...automaticNarrative(id) }; return [...rows.filter(r => r.studentId !== id), { ...prev, [field]: value }] })
    setNarrativeEdits(rows => new Set([...rows, id])); setDirty(true)
  }
  function automaticNarrative(id: string) {
    return automaticReportNarrative(cps, scores.filter(score => score.studentId === id))
  }
  async function suggestClass() {
    if (dirty) throw new Error('Simpan perubahan sebelum meminta saran AI')
    const queue: { studentId: string; strength: string; growth: string }[] = []
    const targets = students.filter(s => !locked(s.id) && cps.some(cp => scoreValue(s.id, cp.id) !== null))
    const controller = new AbortController(); aiController.current = controller; setAIProgress({ done: 0, total: targets.length })
    for (const student of targets) {
      if (controller.signal.aborted) break
      try { const result = await request('/rapor-sumatif/ai/suggest?' + qs + '&mapelId=' + mapelId, token, 'POST', { studentId: student.id }, controller.signal); queue.push({ studentId: student.id, strength: result.strength, growth: result.growth }); setAIProgress({ done: queue.length, total: targets.length }) }
      catch (e) { toast.error(e instanceof Error ? e.message : 'Layanan AI gagal'); break }
    }
    aiController.current = null; setAIProgress(null)
    if (queue.length) { setSuggestion(queue[0]); setSuggestionQueue(queue.slice(1)) }
  }
  async function saveGrid() {
    if (scores.some(s => s.value !== null && (!Number.isFinite(s.value) || s.value < 0 || s.value > 100))) throw new Error('Nilai harus berupa angka 0–100')
    const hasNew = cps.some(cp => !cp.id)
    await request('/rapor-sumatif/grid?' + qs + '&mapelId=' + mapelId, token, 'PUT', {
      revision: period.revision, cps, scores: scores.filter(s => !!s.cpId && !locked(s.studentId)),
      narratives: narratives.filter(n => narrativeEdits.has(n.studentId)),
    })
    await refreshPeriod(); await loadGrid(); toast.success(hasNew ? 'CP tersimpan. Silakan isi nilainya.' : 'Nilai dan deskripsi tersimpan.')
  }
  async function suggest(id: string) {
    if (dirty) throw new Error('Simpan perubahan sebelum meminta saran AI')
    const result = await request('/rapor-sumatif/ai/suggest?' + qs + '&mapelId=' + mapelId, token, 'POST', { studentId: id })
    setSuggestion({ studentId: id, strength: result.strength, growth: result.growth })
  }
  async function showPDF(kind: string, all = false, version = 0) {
    if (dirty) throw new Error('Simpan perubahan sebelum membuka cetakan')
    const path = '/rapor-sumatif/print?' + qs + '&kind=' + kind + (!all && studentId ? '&studentId=' + studentId : '') + (version ? '&version=' + version : '')
    const r = await fetch(base + path, { credentials: 'include', headers: { Authorization: 'Bearer ' + token } })
    if (!r.ok) { const e = await r.json().catch(() => ({})); throw new Error(e.message || e.error || 'Gagal membuat PDF') }
    const url = URL.createObjectURL(await r.blob()); if (previewRef.current) URL.revokeObjectURL(previewRef.current); previewRef.current = url; setPreviewURL(url)
  }
  async function exportLedger(format: string) {
    const r = await fetch(base + '/rapor-sumatif/leger?' + qs + '&format=' + format, { credentials: 'include', headers: { Authorization: 'Bearer ' + token } })
    if (!r.ok) throw new Error('Gagal mengunduh leger')
    const url = URL.createObjectURL(await r.blob()); const a = document.createElement('a'); a.href = url; a.download = 'leger.' + format; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  async function upload(file?: File) {
    if (!file) return
    const form = new FormData(); form.append('file', file)
    const r = await fetch(base + '/rapor-sumatif/import/preview?' + qs, { method: 'POST', credentials: 'include', headers: { Authorization: 'Bearer ' + token }, body: form })
    const data = await r.json(); if (!r.ok) throw new Error(data.message || data.error || 'Impor gagal')
    setImportData(data); setImportStudents(Object.fromEntries(data.preview.students.map((s: ImportStudent) => [s.row, s.studentId]))); setImportSubjects(Object.fromEntries(data.preview.subjects.map((s: ImportSubject) => [s.sheet, s.mapelId])))
  }

  return <div className="space-y-4">
    <div><h2 className="text-xl font-bold">{nilaiOnly ? 'Nilai Sumatif CP' : 'Rapor Semester'}</h2><p className="text-sm text-muted-foreground">Nilai CP, deskripsi capaian, pemeriksaan, dan cetak sesuai blangko PKBM Tunas Ilmu.</p></div>
    {aiProgress && <div className="flex items-center gap-3 rounded-lg border p-3 text-sm" role="status">Membuat saran AI: {aiProgress.done}/{aiProgress.total} siswa<Button variant="outline" onClick={() => aiController.current?.abort()}>Hentikan</Button></div>}
    <Card className="grid gap-3 p-4 sm:grid-cols-3">
      <div><Label>Tahun ajaran</Label><Select aria-label="Tahun ajaran rapor" value={year} disabled={busy} onChange={e => changeFilter(() => { setYear(e.target.value); setKelasId('') })}><option value="">Pilih tahun</option>{years.map(y => <option key={y.id} value={y.id}>{y.namaTahunAjaran}{y.isAktif ? '' : ' (arsip)'}</option>)}</Select></div>
      <div><Label>Kelas</Label><Select aria-label="Kelas rapor" value={kelasId} disabled={busy} onChange={e => changeFilter(() => setKelasId(e.target.value))}><option value="">Pilih kelas</option>{classes.filter(k => k.tahunAjaranId === year).map(k => <option key={k.id} value={k.id}>Kelas {k.jenjang} {k.namaRombel}</option>)}</Select></div>
      <div><Label>Semester</Label><Select aria-label="Semester rapor" value={semester} disabled={busy} onChange={e => changeFilter(() => setSemester(e.target.value))}><option>Ganjil</option><option>Genap</option></Select></div>
    </Card>
    {config && <>
      <div className="flex flex-wrap gap-2" role="tablist" aria-label="Bagian rapor">{[['nilai', 'Nilai & deskripsi'], ['rapor', 'Pemeriksaan & cetak'], ['leger', 'Leger'], ['pengaturan', 'Pengaturan'], ...(isAdmin ? [['impor', 'Impor workbook']] : [])].map(([id, label]) => <Button key={id} variant={tab === id ? 'default' : 'outline'} disabled={busy} onClick={() => { if (dirty) { toast.error('Simpan perubahan sebelum berpindah bagian'); return }; setTab(id) }}>{label}</Button>)}{dirty && <span className="self-center text-sm text-amber-700">Perubahan belum disimpan</span>}</div>
      {isArchived && <p className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950">Tahun ajaran arsip bersifat baca saja. Admin dapat mengimpor data historis melalui pratinjau.</p>}
      {tab === 'nilai' && <>
        <Card className="space-y-3 p-4"><Label>Mata pelajaran</Label><Select aria-label="Mata pelajaran rapor" value={mapelId} disabled={busy} onChange={e => changeFilter(() => setMapelId(e.target.value))}><option value="">Pilih mapel</option>{config.subjects.map(m => <option value={m.mapelId} key={m.mapelId}>{m.name}</option>)}</Select>
          {mapelId && <><p className="text-sm text-muted-foreground">Nilai rapor = rata-rata CP yang terisi, dibulatkan. Simpan daftar CP sebelum mengisi nilai. Tempel blok angka dari Excel ke sel nilai pertama.</p>
            <p className="text-sm text-muted-foreground">Deskripsi otomatis memakai 1 CP tertinggi dan 1 terendah untuk 1–4 CP terisi, atau masing-masing 2 untuk lebih dari 4 CP. Khusus 1 CP, hanya capaian yang ditampilkan. Awali deskripsi CP dengan huruf kecil; antar-CP digabung dengan “serta”. Deskripsi manual yang sudah tersimpan tetap dipertahankan.</p>
            <p className="text-sm text-muted-foreground">Untuk memakai format baru pada deskripsi tersimpan, pilih “Susun ulang dari CP” di baris siswa, periksa hasilnya, lalu klik “Simpan nilai / CP”. Perubahan hanya pada draf; rapor final dan terbit tetap memakai versi yang sudah dibekukan.</p>
            <div className="flex flex-wrap gap-2">{canWrite && !isArchived && <><Button variant="outline" disabled={busy || records.some(r => ['final', 'published'].includes(r.status))} onClick={() => { setCPs(rows => [...rows, { id: '', label: '', urutan: rows.length + 1 }]); setDirty(true) }}>Tambah CP</Button><Button variant="outline" disabled={busy || dirty || cps.length === 0} onClick={() => void run(suggestClass)}>Bantu AI satu kelas</Button><Button disabled={busy || !dirty || cps.length === 0} onClick={() => void run(saveGrid)}>Simpan nilai / CP</Button></>}</div>
            <div className="flex flex-wrap gap-3">{cps.map((cp, i) => <div key={cp.id || i} className="w-56 space-y-1"><Label>CP {i + 1}</Label><Input aria-label={'Label CP ' + (i + 1)} value={cp.label} placeholder="contoh: membaca huruf, suku kata, dan kata" disabled={!canWrite || busy || isArchived || records.some(r => ['final', 'published'].includes(r.status))} onChange={e => { setCPs(rows => rows.map((v, j) => i === j ? { ...v, label: lowercaseCPStart(e.target.value.trimStart()) } : v)); setDirty(true) }} />{canWrite && <Button size="sm" variant="ghost" disabled={busy || isArchived || records.some(r => ['final', 'published'].includes(r.status))} onClick={() => { if (window.confirm('Hapus CP ini beserta nilainya?')) { setCPs(rows => rows.filter((_, j) => i !== j)); setScores(rows => rows.filter(r => r.cpId !== cp.id)); setDirty(true) } }}>Hapus CP</Button>}</div>)}</div>
          </>}
        </Card>
        {mapelId && cps.length > 0 && <Card className="overflow-auto p-3"><table className="w-full text-sm"><thead><tr><th className="min-w-44 p-2 text-left">Peserta didik</th>{cps.map((cp, i) => <th key={cp.id || i} className="min-w-24 p-2">CP {i + 1}</th>)}<th className="p-2">Nilai rapor</th><th className="min-w-80 p-2">Deskripsi akhir</th></tr></thead><tbody>{students.map((student, ri) => {
          const values = cps.map(cp => scoreValue(student.id, cp.id)).filter((v): v is number => v !== null)
          const narrative: Narrative = narratives.find(n => n.studentId === student.id) || { studentId: student.id, ...automaticNarrative(student.id) }
          const disabled = !canWrite || busy || isArchived || locked(student.id) || cps.some(cp => !cp.id)
          return <tr key={student.id} className="border-t align-top"><td className="p-2 font-medium">{student.nama}<div className="text-xs text-muted-foreground">NIS {student.nis}{locked(student.id) ? ' · final' : ''}</div></td>{cps.map((cp, ci) => <td key={cp.id || ci} className="p-1"><Input aria-label={`${student.nama} CP ${ci + 1}`} type="number" min="0" max="100" step="0.01" disabled={disabled} value={scoreValue(student.id, cp.id) ?? ''} onChange={e => updateScore(student.id, cp.id, e.target.value)} onPaste={e => {
            const raw = e.clipboardData.getData('text/plain'); if (!raw.includes('\t') && !raw.includes('\n')) return
            e.preventDefault(); const rows = raw.trimEnd().split(/\r?\n/); const updates: Score[] = []
            rows.forEach((row, r) => row.split('\t').forEach((cell, c) => { const st = students[ri + r], target = cps[ci + c]; if (!st || !target || locked(st.id)) return; const value = cell.trim() === '' ? null : Number(cell.replace(',', '.')); if (value !== null && (!Number.isFinite(value) || value < 0 || value > 100)) throw new Error('Blok berisi nilai tidak valid'); updates.push({ studentId: st.id, cpId: target.id, value }) }))
            setScores(prev => [...prev.filter(v => !updates.some(u => u.cpId === v.cpId && u.studentId === v.studentId)), ...updates]); setDirty(true)
          }} /></td>)}<td className="p-2 text-center font-semibold">{values.length ? Math.round(values.reduce((a, b) => a + b, 0) / values.length) : '—'}<div className="text-xs font-normal">{values.length}/{cps.length} CP</div></td><td className="space-y-2 p-2"><textarea aria-label={'Capaian ' + student.nama} placeholder="Kosong: gunakan narasi otomatis dari CP tertinggi" className={textClass} disabled={disabled} value={narrative?.strength || ''} onChange={e => editNarrative(student.id, 'strength', e.target.value)} /><textarea aria-label={'Penguatan ' + student.nama} placeholder="Penguatan capaian lainnya" className={textClass} disabled={disabled} value={narrative?.growth || ''} onChange={e => editNarrative(student.id, 'growth', e.target.value)} />{narrative?.reviewedRevision === -1 && <p className="text-amber-700">Nilai berubah. Tinjau deskripsi dan simpan kembali.</p>}<div className="flex flex-wrap gap-2">{narratives.some(row => row.studentId === student.id) && <Button size="sm" variant="outline" aria-label={'Susun ulang deskripsi ' + student.nama} title="Mengganti deskripsi pada draf siswa ini dari nilai CP. Klik Simpan nilai / CP untuk menyimpan." disabled={disabled || values.length === 0} onClick={() => { const automatic = automaticNarrative(student.id); editNarrative(student.id, 'strength', automatic.strength); editNarrative(student.id, 'growth', automatic.growth) }}>Susun ulang dari CP</Button>}<Button size="sm" variant="outline" disabled={disabled || dirty} onClick={() => void run(() => suggest(student.id))}>Bantu AI</Button>{narrative && <Button size="sm" variant="outline" disabled={disabled} onClick={() => { setNarrativeEdits(prev => new Set([...prev, student.id])); setDirty(true) }}>Tandai ditinjau</Button>}</div></td></tr>
        })}</tbody></table></Card>}
      </>}
      {tab === 'rapor' && <>
        {!canManage && !readOnly ? <p>Pengisian nilai tersedia pada tab Nilai & deskripsi. Pemeriksaan rapor dilakukan wali kelas atau admin.</p> : <>
          <Card className="space-y-3 p-4"><Label>Peserta didik</Label><Select aria-label="Peserta didik rapor" value={studentId} disabled={busy} onChange={e => changeFilter(() => setStudentId(e.target.value))}><option value="">Pilih siswa</option>{students.map(s => <option key={s.id} value={s.id}>{s.nama} · {records.find(r => r.studentId === s.id)?.status || 'draft'}</option>)}</Select>
            <div className="flex flex-wrap gap-2">{['rapor', 'cover', 'identitas', 'lengkap'].map(kind => <Button key={kind} variant="outline" disabled={busy || !studentId} onClick={() => void run(() => showPDF(kind))}>Cetak {kind}</Button>)}<Button variant="outline" disabled={busy || students.length === 0} onClick={() => void run(() => showPDF('lengkap', true))}>Cetak lengkap satu kelas</Button></div>
          </Card>
          {doc && <Card className="space-y-4 p-4"><div><h3 className="font-bold">{doc.student.nama}</h3><p className="text-sm">Status: {record?.status || 'draft'} · Versi {record?.version || 0}</p>{!!record?.version && <details className="mt-2"><summary className="cursor-pointer text-sm">Riwayat versi final</summary><div className="mt-2 flex flex-wrap gap-2">{Array.from({ length: record.version }, (_, i) => <Button key={i} variant="outline" size="sm" disabled={busy || dirty} onClick={() => void run(() => showPDF('rapor', false, i + 1))}>Lihat versi {i + 1}</Button>)}</div></details>}</div>
            <table className="w-full text-sm"><thead><tr><th className="text-left">Mata pelajaran</th><th>Nilai</th><th className="text-left">Capaian kompetensi</th></tr></thead><tbody>{doc.subjects.map(m => <tr key={m.mapelId} className="border-t"><td className="p-2">{m.name}</td><td className="p-2 text-center">{m.score ?? '—'}</td><td className="space-y-2 p-2"><p>{m.strength}</p><p>{m.growth}</p>{m.stale && <p className="text-amber-700">Perlu ditinjau</p>}</td></tr>)}</tbody></table>
            <p className="text-sm">Ketidakhadiran: Sakit {doc.attendance.sakit} hari · Izin {doc.attendance.izin} hari · Tanpa keterangan {doc.attendance.alpa} hari</p>
            <fieldset disabled={!canManage || busy || isArchived || locked(studentId)} className="space-y-3"><Label>Catatan wali kelas</Label><textarea aria-label="Catatan wali kelas" className={textClass} value={details.note} onChange={e => { setDetails({ ...details, note: e.target.value }); setDirty(true) }} />
              <details className="rounded-lg border p-3"><summary className="cursor-pointer font-medium">Identitas untuk lembar cetak</summary><p className="my-2 text-xs text-muted-foreground">Nama, NIS, dan NISN mengikuti master peserta didik. Data pelengkap disimpan pada rapor semester ini.</p><div className="grid gap-3 sm:grid-cols-2">{[['birthPlace', 'Tempat lahir'], ['birthDate', 'Tanggal lahir'], ['religion', 'Agama'], ['phone', 'Telepon'], ['address', 'Alamat'], ['father', 'Nama ayah'], ['mother', 'Nama ibu'], ['fatherJob', 'Pekerjaan ayah'], ['motherJob', 'Pekerjaan ibu'], ['guardian', 'Nama wali'], ['guardianJob', 'Pekerjaan wali'], ['admissionClass', 'Kelas saat diterima'], ['admissionYear', 'Tahun masuk'], ['admissionStatus', 'Status saat diterima']].map(([key, label]) => <div key={key}><Label>{label}</Label><Input aria-label={label} type={key === 'birthDate' ? 'date' : 'text'} value={details.identity?.[key] || ''} onChange={e => { setDetails({ ...details, identity: { ...details.identity, [key]: e.target.value } }); setDirty(true) }} /></div>)}</div></details>
              {semester === 'Genap' && <div className="grid gap-3 sm:grid-cols-2"><div><Label>Keputusan kenaikan</Label><Select aria-label={"Keputusan kenaikan"} value={details.promotion} onChange={e => { setDetails({ ...details, promotion: e.target.value }); setDirty(true) }}><option value="">Belum ditentukan</option><option value="naik">Naik kelas</option><option value="tinggal">Tinggal kelas</option></Select></div><div><Label>Naik ke kelas</Label><Input aria-label={"Naik ke kelas"} value={details.nextClass} placeholder="II (Dua)" onChange={e => { setDetails({ ...details, nextClass: e.target.value }); setDirty(true) }} /></div></div>}
              <section className="space-y-3 rounded-xl border p-4" aria-label="Ekstrakurikuler siswa">
                <h4 className="font-semibold">Ekstrakurikuler</h4>
                {config.extrasEnabled === false ? <p className="text-sm text-muted-foreground">Ekstrakurikuler tidak diaktifkan untuk kelas dan semester ini. Pengaturannya ada pada tab Pengaturan. Isian lama tetap tersimpan.</p> : <>
                  <p className="text-sm text-muted-foreground">Tambahkan kegiatan yang diikuti siswa, lalu isi keterangan perkembangannya. Siswa yang tidak mengikuti kegiatan boleh dikosongkan.</p>
                  {details.extras.length === 0 && <p className="text-sm">Belum ada kegiatan untuk siswa ini.</p>}
                  {details.extras.map((extra, i) => <div key={i} className="space-y-2 rounded-lg bg-muted/40 p-3">
                    <div className="flex items-center justify-between gap-2"><Label htmlFor={'extra-name-' + i}>Kegiatan {i + 1}</Label><Button variant="ghost" aria-label={'Hapus ekstrakurikuler ' + (i + 1)} onClick={() => { setDetails({ ...details, extras: details.extras.filter((_, j) => i !== j) }); setDirty(true) }}>Hapus</Button></div>
                    {!!config.extraActivities?.length ? <Select id={'extra-name-' + i} aria-label={'Nama ekstrakurikuler ' + (i + 1)} value={extra.name} onChange={e => { setDetails({ ...details, extras: details.extras.map((v, j) => i === j ? { ...v, name: e.target.value } : v) }); setDirty(true) }}><option value="">Pilih kegiatan...</option>{extra.name && !config.extraActivities.includes(extra.name) && <option value={extra.name}>{extra.name} (isian sebelumnya)</option>}{config.extraActivities.map(name => <option key={name} value={name} disabled={details.extras.some((v, j) => j !== i && v.name === name)}>{name}</option>)}</Select> : <Input id={'extra-name-' + i} aria-label={'Nama ekstrakurikuler ' + (i + 1)} maxLength={100} placeholder="Contoh: Pramuka" value={extra.name} onChange={e => { setDetails({ ...details, extras: details.extras.map((v, j) => i === j ? { ...v, name: e.target.value } : v) }); setDirty(true) }} />}
                    <Label htmlFor={'extra-description-' + i}>Keterangan perkembangan</Label><textarea id={'extra-description-' + i} aria-label={'Keterangan ekstrakurikuler ' + (i + 1)} className={textClass} maxLength={1000} placeholder="Contoh: Aktif mengikuti latihan dan mampu bekerja sama dalam regu." value={extra.description} onChange={e => { setDetails({ ...details, extras: details.extras.map((v, j) => i === j ? { ...v, description: e.target.value } : v) }); setDirty(true) }} /><p className="text-right text-xs text-muted-foreground">{extra.description.length}/1.000 karakter</p>
                  </div>)}
                  <Button variant="outline" disabled={details.extras.length >= 10} onClick={() => { setDetails({ ...details, extras: [...details.extras, { name: '', description: '' }] }); setDirty(true) }}>Tambah ekstrakurikuler</Button>
                </>}
              </section>
              <Button disabled={!dirty} onClick={() => void run(async () => { await request('/rapor-sumatif/document/' + studentId + '?' + qs, token, 'PUT', { revision: period.revision, details: { note: details.note, promotion: details.promotion, nextClass: details.nextClass, extras: details.extras, identity: details.identity } }); await refreshPeriod(); await loadDocument(); setDirty(false); toast.success('Catatan tersimpan') })}>Simpan catatan</Button>
            </fieldset>
            {!!doc.issues.length && <div className="rounded-lg bg-amber-50 p-3 text-sm text-amber-950"><strong>Perlu dilengkapi sebelum final:</strong><ul className="list-disc pl-5">{doc.issues.map((issue, i) => <li key={i}>{issue}</li>)}</ul></div>}
            {canManage && !isArchived && <div className="flex flex-wrap gap-2">{[['ready', 'Siap diperiksa'], ['final', 'Finalkan rapor'], ['published', 'Terbitkan untuk orang tua'], ['draft', 'Buka / revisi draf']].map(([status, label]) => <Button key={status} variant="outline" disabled={busy || dirty || status === 'final' && (record?.status !== 'ready' || doc.issues.length > 0) || status === 'published' && record?.status !== 'final' || status === 'ready' && locked(studentId)} onClick={() => void run(async () => { let reason = ''; if (status === 'draft' && locked(studentId)) { const answer = window.prompt('Alasan pembukaan revisi rapor:'); if (!answer?.trim()) return; reason = answer } await request('/rapor-sumatif/document/' + studentId + '/status?' + qs, token, 'POST', { revision: period.revision, status, reason }); await refreshPeriod(); await loadDocument(); toast.success('Status rapor diperbarui') })}>{label}</Button>)}</div>}
          </Card>}
        </>}
      </>}
      {tab === 'leger' && <Card className="space-y-3 overflow-auto p-4"><div className="flex gap-2"><Button disabled={busy} onClick={() => void run(async () => setLedger(await request('/rapor-sumatif/leger?' + qs, token)))}>Muat leger</Button><Button variant="outline" disabled={busy} onClick={() => void run(() => exportLedger('xlsx'))}>Unduh Excel</Button><Button variant="outline" disabled={busy} onClick={() => void run(() => exportLedger('pdf'))}>Unduh PDF</Button></div><table className="w-full text-sm"><thead><tr><th>Nama</th>{config.subjects.map(m => <th key={m.mapelId} className="p-2">{m.name}</th>)}<th>Jumlah</th><th>Peringkat</th></tr></thead><tbody>{ledger.map(row => <tr key={row.student.id} className="border-t"><td className="p-2">{row.student.nama}</td>{row.subjects.map(m => <td key={m.mapelId} className="p-2 text-center">{m.score ?? '—'}</td>)}<td>{row.total ?? '—'}</td><td>{row.rank ?? '—'}</td></tr>)}</tbody></table></Card>}
      {tab === 'pengaturan' && <>
        <Card className="space-y-4 p-4"><h3 className="font-bold">Identitas dan cetak rapor</h3><p className="text-sm text-muted-foreground">Lembar rapor semester 1 ditandatangani orang tua/wali dan wali kelas. Semester 2 ditambah tanda tangan Kepala PKBM tersendiri di bagian paling akhir tengah, di bawah orang tua/wali dan wali kelas.</p><fieldset disabled={!canManage || busy || isArchived} className="grid gap-3 sm:grid-cols-2">{[['program', 'Program'], ['phase', 'Fase'], ['city', 'Tempat penetapan'], ['date', 'Tanggal rapor'], ['teacher', 'Nama wali kelas'], ['teacherNip', 'NIP wali kelas'], ['principal', 'Nama kepala PKBM'], ['principalNip', 'NIP kepala PKBM']].map(([key, label]) => <div key={key}><Label>{label}</Label><Input aria-label={label} type={key === 'date' ? 'date' : 'text'} value={String(config[key as keyof Config] || '')} onChange={e => { setConfig({ ...config, [key]: e.target.value }); setDirty(true) }} /></div>)}
          <div><Label>Tanda tangan kepala PKBM (semester 2, opsional, PNG)</Label><Input aria-label="Gambar tanda tangan kepala PKBM" type="file" accept="image/png" onChange={e => { const file = e.target.files?.[0]; if (!file) return; if (file.size > 300000) { toast.error('Gambar maksimal 300 KB'); return }; const reader = new FileReader(); reader.onload = () => { setConfig({ ...config, principalSignature: String(reader.result) }); setDirty(true) }; reader.readAsDataURL(file) }} />{config.principalSignature && <Button variant="ghost" onClick={() => { setConfig({ ...config, principalSignature: '' }); setDirty(true) }}>Hapus gambar tanda tangan</Button>}</div><div className="space-y-2 sm:col-span-2"><Label>Urutan dan kelompok mata pelajaran</Label>{config.subjects.map((m, i) => <div key={m.mapelId} className="flex flex-wrap gap-2"><Input className="min-w-56 flex-1" aria-label="Nama cetak mapel" value={m.name} onChange={e => { setConfig({ ...config, subjects: config.subjects.map((v, j) => i === j ? { ...v, name: e.target.value } : v) }); setDirty(true) }} /><Select className="w-48" value={m.group} onChange={e => { setConfig({ ...config, subjects: config.subjects.map((v, j) => i === j ? { ...v, group: e.target.value } : v) }); setDirty(true) }}><option value="umum">Mapel umum</option><option value="pemberdayaan">Pemberdayaan / keterampilan</option></Select><Button variant="outline" disabled={i === 0} onClick={() => { const subjects = [...config.subjects]; [subjects[i - 1], subjects[i]] = [subjects[i], subjects[i - 1]]; setConfig({ ...config, subjects }); setDirty(true) }}>Naik</Button></div>)}</div>
          <section className="space-y-3 rounded-xl border p-4 sm:col-span-2" aria-label="Pengaturan ekstrakurikuler kelas">
            <h4 className="font-semibold">Ekstrakurikuler kelas ini</h4><p className="text-sm text-muted-foreground">Berlaku hanya untuk kelas, tahun ajaran, dan semester yang dipilih. Keterangan diisi per siswa pada Pemeriksaan &amp; cetak.</p>
            <label className="flex items-center gap-2"><input type="checkbox" checked={config.extrasEnabled !== false} onChange={e => { setConfig({ ...config, extrasEnabled: e.target.checked }); setDirty(true) }} />Kelas ini memiliki ekstrakurikuler</label>
            {config.extrasEnabled !== false && <><p className="text-sm">Daftar kegiatan menjadi pilihan saat mengisi rapor siswa. Jika daftar kosong, nama kegiatan dapat ditulis langsung per siswa.</p>{(config.extraActivities || []).map((name, i) => <div key={i} className="flex gap-2"><Input aria-label={'Kegiatan kelas ' + (i + 1)} maxLength={100} placeholder="Contoh: Pramuka, Seni Tari, Futsal" value={name} onChange={e => { setConfig({ ...config, extraActivities: config.extraActivities!.map((v, j) => i === j ? e.target.value : v) }); setDirty(true) }} /><Button variant="ghost" aria-label={'Hapus kegiatan kelas ' + (i + 1)} onClick={() => { setConfig({ ...config, extraActivities: config.extraActivities!.filter((_, j) => i !== j) }); setDirty(true) }}>Hapus</Button></div>)}<Button variant="outline" disabled={(config.extraActivities?.length || 0) >= 20} onClick={() => { setConfig({ ...config, extraActivities: [...(config.extraActivities || []), ''] }); setDirty(true) }}>Tambah kegiatan kelas</Button></>}
          </section>
          <Button disabled={!dirty} onClick={() => void run(async () => { await request('/rapor-sumatif/config?' + qs, token, 'PUT', { revision: period.revision, config }); await refreshPeriod(); setDirty(false); toast.success('Pengaturan rapor tersimpan') })}>Simpan pengaturan</Button>
        </fieldset></Card>
        {isAdmin && <Card className="space-y-3 p-4"><h3 className="font-bold">Bantuan AI</h3><p className="text-sm">API key dikonfigurasi pada server. Gemini: {ai.geminiConfigured ? 'tersedia' : 'belum tersedia'} · OpenAI: {ai.openaiConfigured ? 'tersedia' : 'belum tersedia'}</p><Label>Penyedia</Label><Select aria-label={"Penyedia"} value={ai.provider} onChange={e => setAI({ ...ai, provider: e.target.value })}><option value="">Nonaktif</option><option value="gemini">Gemini</option><option value="openai">OpenAI</option></Select><Label>ID model</Label><Input aria-label={"ID model"} value={ai.model} onChange={e => setAI({ ...ai, model: e.target.value })} placeholder="Model yang tersedia pada akun penyedia" /><Button disabled={busy} onClick={() => void run(async () => { setAI(await request('/rapor-sumatif/ai/settings', token, 'PUT', { provider: ai.provider, model: ai.model })); toast.success('Pengaturan AI tersimpan') })}>Simpan pengaturan AI</Button></Card>}
      </>}
      {tab === 'impor' && isAdmin && <Card className="space-y-4 p-4"><h3 className="font-bold">Impor workbook semester lalu</h3><p className="text-sm">Pilih kelas dan semester tujuan di atas. Data baru disimpan setelah pencocokan dan konfirmasi. Data yang sudah ada tidak ditimpa.</p><Input aria-label="Workbook rapor" type="file" accept=".xlsm,.xlsx" disabled={busy} onChange={e => void run(() => upload(e.target.files?.[0]))} />
        {importData && <><p className="text-sm">Sumber: {importData.preview.source?.year || 'tahun tidak dicantumkan'} · Semester {importData.preview.source?.semester || '—'} · Kelas {importData.preview.source?.class || '—'}</p><p>{importData.preview.students.length} siswa · {importData.preview.subjects.length} sheet mapel</p>{importData.preview.issues.map((v, i) => <p className="text-red-700" key={i}>{v}</p>)}<div className="space-y-2">{importData.preview.students.map(s => <div key={s.row} className="grid gap-2 sm:grid-cols-2"><span>{s.name} · NIS {s.nis} · NISN {s.nisn}</span><Select value={importStudents[s.row] || ''} onChange={e => setImportStudents({ ...importStudents, [s.row]: e.target.value })}><option value="">Cocokkan siswa...</option><option value="skip">Lewati siswa ini</option>{importData.students.map(pd => <option key={pd.id} value={pd.id}>{pd.nama} · {pd.nis}</option>)}</Select></div>)}</div><div className="space-y-2">{importData.preview.subjects.map(s => <div key={s.sheet}><Label>{s.sheet} · {s.labels.length} CP</Label><Select value={importSubjects[s.sheet] || ''} onChange={e => setImportSubjects({ ...importSubjects, [s.sheet]: e.target.value })}><option value="">Cocokkan mapel...</option><option value="skip">Lewati sheet ini</option>{importData.subjects.map(mp => <option key={mp.mapelId} value={mp.mapelId}>{mp.name}</option>)}</Select>{s.issues.map((v, i) => <p key={i} className="text-sm text-red-700">{v}</p>)}</div>)}</div><Button disabled={busy || dirty} onClick={() => void run(async () => { await request('/rapor-sumatif/import/' + importData.id + '/confirm?' + qs, token, 'POST', { revision: period.revision, students: importStudents, subjects: importSubjects }); setImportData(null); await refreshPeriod(); toast.success('Data historis berhasil diimpor') })}>Konfirmasi dan simpan impor</Button></>}
      </Card>}
    </>}
    {suggestion && <div role="dialog" aria-modal="true" aria-label="Tinjau saran AI" className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"><Card className="w-full max-w-xl space-y-4 p-5"><p className="text-xs">{suggestionQueue.length + 1} saran menunggu tinjauan</p><h3 className="font-bold">Tinjau saran AI · {students.find(s => s.id === suggestion.studentId)?.nama}</h3><Label>Capaian yang dikuasai</Label><textarea aria-label={"Capaian yang dikuasai"} className={textClass} value={suggestion.strength} onChange={e => setSuggestion({ ...suggestion, strength: e.target.value })} /><Label>Penguatan</Label><textarea aria-label={"Penguatan"} className={textClass} value={suggestion.growth} onChange={e => setSuggestion({ ...suggestion, growth: e.target.value })} /><p className="text-xs text-muted-foreground">Penerapan mengganti deskripsi pada draf. Periksa isinya lalu simpan nilai.</p><div className="flex gap-2"><Button onClick={() => { editNarrative(suggestion.studentId, 'strength', suggestion.strength); editNarrative(suggestion.studentId, 'growth', suggestion.growth); setSuggestion(suggestionQueue[0] || null); setSuggestionQueue(rows => rows.slice(1)) }}>Terapkan pada draf</Button><Button variant="outline" onClick={() => { setSuggestion(null); setSuggestionQueue([]) }}>Batal</Button></div></Card></div>}
    {previewURL && <div role="dialog" aria-modal="true" aria-label="Pratinjau cetak rapor" className="fixed inset-0 z-50 flex flex-col bg-background p-4"><div className="mb-3 flex items-center gap-3"><strong>Pratinjau PDF</strong><a href={previewURL} download="rapor.pdf" className="underline">Unduh PDF</a><Button className="ml-auto" variant="outline" onClick={() => setPreviewURL('')}>Tutup</Button></div><iframe title="PDF rapor" src={previewURL} className="w-full flex-1 rounded-lg border" /></div>}
  </div>
}
