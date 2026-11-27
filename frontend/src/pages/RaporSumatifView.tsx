import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select } from '../components/ui/select'
import { request } from '../lib/api'
import { automaticReportNarrative } from '../lib/report-narrative'
import { ReportAttendancePanel, type ReportAttendanceActions } from '../components/ReportAttendancePanel'
import { ReportGradeCPPanel, type ReportGradeCPActions } from '../components/ReportGradeCPPanel'
import { useUnsavedChanges } from '../context/useUnsavedChanges'
import { ReportWorkflowGuide } from '../components/ReportWorkflowGuide'
import { useNavigate } from 'react-router-dom'

type Student = { id: string; nama: string; nis: string; nisn: string }
type ClassRow = { id: string; jenjang: number; namaRombel: string; tahunAjaranId: string; waliKelasId?: string; tahunAjaran: { id: string; namaTahunAjaran: string; isAktif: boolean } }
type Subject = { mapelId: string; name: string; group: string }
type Config = { subjects: Subject[]; program: string; phase: string; city: string; date: string; teacher: string; teacherNip: string; principal: string; principalNip: string; principalSignature?: string; extrasEnabled?: boolean; extraActivities?: string[]; attendanceMode?: string }
type Period = { id: string; revision: number }
type PromotionRules = { completion: boolean; nextClass: string; validGrade: boolean }
type CP = { id: string; label: string; urutan: number }
type Score = { cpId: string; studentId: string; value: number | null }
type Narrative = { studentId: string; strength: string; growth: string; reviewedRevision?: number }
type Details = { note: string; promotion: string; nextClass: string; extras: { name: string; description: string }[]; identity?: Record<string, string> }
type DocumentSubject = Subject & { score: number | null; strength: string; growth: string; stale: boolean; complete: boolean }
type ReportDocument = { student: Student; subjects: DocumentSubject[]; attendance: { sakit: number; izin: number; alpa: number }; attendanceSource?: string; details: Details; issues: string[] }
type RecordRow = { studentId: string; status: string; version: number }
type ImportStudent = { row: number; name: string; nis: string; nisn: string; studentId: string }
type ImportSubject = { sheet: string; name: string; mapelId: string; labels: string[]; issues: string[] }
type ImportPreview = { id: string; preview: { students: ImportStudent[]; subjects: ImportSubject[]; issues: string[]; source?: Record<string, string> }; students: Student[]; subjects: Subject[] }
const base = import.meta.env.VITE_API_BASE_URL || '/api'
const textClass = 'min-h-24 w-full rounded-lg border border-input bg-background p-3 text-sm disabled:opacity-60'
const requiredClass = 'border-l-4 border-l-primary bg-primary/5 focus-visible:ring-primary/30'
const statusLabels: Record<string, string> = { draft: 'Draf · masih dapat dilengkapi', ready: 'Siap diperiksa wali kelas', final: 'Final · versi terkunci', published: 'Terbit · tersedia untuk orang tua' }
const emptyDetails: Details = { note: '', promotion: '', nextClass: '', extras: [] }

export function RaporSumatifView({ token, readOnly, nilaiOnly = false }: { token: string; readOnly: boolean; nilaiOnly?: boolean }) {
  const navigate = useNavigate()
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
  const [cpSource, setCPSource] = useState<'legacy' | 'shared' | 'pending'>('pending')
  const [scores, setScores] = useState<Score[]>([])
  const [narratives, setNarratives] = useState<Narrative[]>([])
  const [narrativeEdits, setNarrativeEdits] = useState<Set<string>>(new Set())
  const [doc, setDoc] = useState<ReportDocument | null>(null)
  const [record, setRecord] = useState<RecordRow | null>(null)
  const [details, setDetails] = useState<Details>(emptyDetails)
  const [promotionRules, setPromotionRules] = useState<PromotionRules | null>(null)
  const [promotionEdited, setPromotionEdited] = useState(false)
  const [tab, setTab] = useState(nilaiOnly ? 'nilai' : 'rapor')
  const [busy, setBusy] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [aiDirty, setAIDirty] = useState(false)
  const [saved, setSaved] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [semesterArchived, setSemesterArchived] = useState(false)
  const [periodConfigLocked, setPeriodConfigLocked] = useState(false)
  const [docLoading, setDocLoading] = useState(false)
  const [docError, setDocError] = useState('')
  const [gridLoading, setGridLoading] = useState(false)
  const [gridError, setGridError] = useState('')
  const [focusTarget, setFocusTarget] = useState('')
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
  const tokenRef = useRef(token)
  tokenRef.current = token
  const attendanceActions = useRef<ReportAttendanceActions | null>(null)
  const gradeCPActions = useRef<ReportGradeCPActions | null>(null)
  const documentRequest = useRef(0)
  const documentController = useRef<AbortController | null>(null)
  const qs = new URLSearchParams({ kelasId, semester, tahunAjaranId: year }).toString()
  const scopeRef = useRef(qs)
  scopeRef.current = qs
  const locked = (id: string) => records.some(r => r.studentId === id && ['final', 'published'].includes(r.status))
  const canWrite = !readOnly && canInput.includes(mapelId)
  const isArchived = semesterArchived || !!year && !years.find(y => y.id === year)?.isAktif
  const settingsLocked = periodConfigLocked || records.some(r => ['final', 'published'].includes(r.status))
  const hasChanges = dirty || aiDirty
  const leave = useUnsavedChanges({ dirty: hasChanges, busy, save: saveCurrent, discard: discardCurrent })
  const gradeCPOpen = tab === 'cp'

  useEffect(() => {
    void Promise.all([request('/rapor-sumatif/classes', tokenRef.current), request('/rapor-sumatif/ai/settings', tokenRef.current)])
      .then(([k, a]) => { const y = [...new Map<string, ClassRow['tahunAjaran']>((k as ClassRow[]).map(c => [c.tahunAjaranId, c.tahunAjaran])).values()]; setClasses(k); setYears(y); setYear(y.find(r => r.isAktif)?.id || y[0]?.id || ''); setAI(a) })
      .catch(e => toast.error(e.message))
  }, [])
  useEffect(() => () => { if (previewRef.current) URL.revokeObjectURL(previewRef.current); aiController.current?.abort(); documentController.current?.abort() }, [])
  useEffect(() => {
    if (!focusTarget) return
    const target = document.getElementById(focusTarget)
    if (target) { target.scrollIntoView({ behavior: 'smooth', block: 'center' }); target.focus(); setFocusTarget('') }
  }, [tab, focusTarget, config, cps, doc])

  async function refreshPeriod() {
    const data = await request('/rapor-sumatif/period?' + qs, token)
    if (scopeRef.current !== qs) return data
    setPeriod(data.period); setConfig(data.config); setStudents(data.students); setRecords(data.records)
    setPromotionRules(data.promotionRules)
    setSemesterArchived(!!data.semesterArchived); setPeriodConfigLocked(!!data.configLocked)
    setCanManage(data.canManage); setCanInput(data.canInput); setIsAdmin(data.isAdmin)
    return data
  }
  useEffect(() => {
    const current = ++generation.current
    setConfig(null); setDoc(null); setCPs([]); setScores([]); setNarratives([]); setMapelId(''); setStudentId(''); setDirty(false); setImportData(null); setLedger([])
    setPromotionRules(null); setPromotionEdited(false)
    setSemesterArchived(false); setPeriodConfigLocked(false); setSaved(false); setSaveError('')
    if (!kelasId) return
    void request('/rapor-sumatif/period?' + qs, tokenRef.current).then(data => {
      if (current !== generation.current) return
      setPeriod(data.period); setConfig(data.config); setStudents(data.students); setRecords(data.records)
      setPromotionRules(data.promotionRules)
      setSemesterArchived(!!data.semesterArchived); setPeriodConfigLocked(!!data.configLocked)
      setCanManage(data.canManage); setCanInput(data.canInput); setIsAdmin(data.isAdmin)
    }).catch(e => { if (current === generation.current) toast.error(e.message) })
  }, [kelasId, semester, year]) // eslint-disable-line react-hooks/exhaustive-deps -- token refresh must not erase unsaved inputs

  async function loadGrid() {
    if (!mapelId) return
    setGridLoading(true); setGridError('')
    try {
      const data = await request('/rapor-sumatif/grid?' + qs + '&mapelId=' + mapelId, token)
      if (scopeRef.current !== qs) return
      setPeriod(data.period); setCPs(data.cps); setCPSource(data.cpSource); setScores(data.scores); setNarratives(data.narratives); setNarrativeEdits(new Set()); setDirty(false)
    } catch (e) { setGridError(e instanceof Error ? e.message : 'Nilai gagal dimuat'); throw e }
    finally { if (scopeRef.current === qs) setGridLoading(false) }
  }
  useEffect(() => {
    if (gradeCPOpen) return
    let cancelled = false
    const controller = new AbortController()
    setCPs([]); setCPSource('pending'); setScores([]); setNarratives([]); setNarrativeEdits(new Set()); setSuggestion(null)
    setGridError(''); setGridLoading(!!mapelId)
    if (!mapelId) return
    void request('/rapor-sumatif/grid?' + qs + '&mapelId=' + mapelId, tokenRef.current, 'GET', undefined, controller.signal).then(data => {
      if (cancelled) return
      setPeriod(data.period); setCPs(data.cps); setCPSource(data.cpSource); setScores(data.scores); setNarratives(data.narratives); setDirty(false)
    }).catch(e => { if (!cancelled) setGridError(e.message) }).finally(() => { if (!cancelled) setGridLoading(false) })
    return () => { cancelled = true; controller.abort() }
  }, [mapelId, qs, gradeCPOpen])

  async function loadDocument() {
    if (!studentId || !canManage && !readOnly) return
    const current = ++documentRequest.current
    documentController.current?.abort()
    const controller = new AbortController(); documentController.current = controller
    setDoc(null); setDocError(''); setDocLoading(true)
    try {
      const data = await request('/rapor-sumatif/document/' + studentId + '?' + qs, tokenRef.current, 'GET', undefined, controller.signal)
      if (current !== documentRequest.current || scopeRef.current !== qs) return
      setDoc(data.document); setRecord(data.record); setPeriod(data.period); setDetails({ ...emptyDetails, ...data.document.details, extras: data.document.details.extras || [] })
      setPromotionEdited(false)
    } catch (e) {
      if (current === documentRequest.current && !controller.signal.aborted) setDocError(e instanceof Error ? e.message : 'Pemeriksaan gagal dimuat')
      throw e
    } finally { if (current === documentRequest.current) setDocLoading(false) }
  }
  useEffect(() => {
    setDoc(null); setRecord(null); setDocError(''); setDocLoading(false)
    if (tab === 'rapor') void loadDocument().catch(() => undefined)
    return cancelDocumentRequest
  }, [studentId, qs, canManage, readOnly, tab]) // eslint-disable-line react-hooks/exhaustive-deps -- refresh on entry, not access-token rotation

  async function run(action: () => Promise<void>) {
    setBusy(true)
    setSaveError('')
    try { await action() } catch (e) { const message = e instanceof Error ? e.message : 'Permintaan gagal'; setSaveError(message); toast.error(message) } finally { setBusy(false) }
  }
  function changeFilter(action: () => void) {
    leave(() => { setDirty(false); setSaved(false); action() })
  }
  function cancelDocumentRequest() { ++documentRequest.current; documentController.current?.abort() }
  function invalidateReport() {
    ++documentRequest.current; documentController.current?.abort(); setDoc(null); setLedger([])
    if (previewRef.current) URL.revokeObjectURL(previewRef.current)
    previewRef.current = ''; setPreviewURL('')
  }
  function goTo(nextTab: string, target: string) {
    const show = () => { if (nextTab) setTab(nextTab); setFocusTarget(target === 'report-scores' && !mapelId ? 'report-cp' : target === 'report-status' && !studentId ? 'report-student' : target) }
    if (!nextTab || nextTab === tab) show()
    else leave(show)
  }
  function openIssue(issue: string) {
    if (issue === 'Belum ada mapel kelas') {
      if (isAdmin) leave(() => navigate('/kelas-mapel'))
      else toast.info('Minta admin menetapkan mata pelajaran untuk kelas ini melalui menu Mapel Kelas.')
      return
    }
    if (/Konflik presensi/i.test(issue)) { leave(() => navigate('/presensi')); return }
    if (/tanggal|penandatangan|Pilih mata pelajaran|Daftar mapel/i.test(issue)) { goTo('pengaturan', !config?.date ? 'report-config-date' : !config.teacher ? 'report-config-teacher' : semester === 'Genap' && !config.principal ? 'report-config-principal' : 'report-config'); return }
    if (/presensi/i.test(issue)) { goTo('presensi', 'report-attendance'); return }
    if (/keputusan/i.test(issue)) { goTo('rapor', 'report-promotion'); return }
    if (/Catatan wali/i.test(issue)) { goTo('rapor', 'report-note'); return }
    if (/ekstrakurikuler/i.test(issue)) { goTo('rapor', 'report-extras'); return }
    const subject = doc?.subjects.find(m => issue.startsWith(m.name + ':'))
    if (subject) leave(() => { setMapelId(subject.mapelId); setTab('nilai'); setFocusTarget('report-scores') })
  }
  async function saveDetails() {
    await request('/rapor-sumatif/document/' + studentId + '?' + qs, token, 'PUT', { revision: period.revision, updatePromotion: promotionEdited, details: { note: details.note, promotion: details.promotion, nextClass: details.nextClass, extras: details.extras, identity: details.identity } })
    invalidateReport(); await refreshPeriod(); await loadDocument(); setDirty(false); setSaved(true); toast.success('Catatan tersimpan')
  }
  async function saveConfig() {
    await request('/rapor-sumatif/config?' + qs, token, 'PUT', { revision: period.revision, config })
    invalidateReport(); await refreshPeriod(); setDirty(false); setSaved(true); toast.success('Pengaturan rapor tersimpan')
  }
  async function saveAI() {
    setAI(await request('/rapor-sumatif/ai/settings', token, 'PUT', { provider: ai.provider, model: ai.model }))
    setAIDirty(false); setSaved(true); toast.success('Pengaturan AI tersimpan')
  }
  async function saveCurrent() {
    setBusy(true); setSaveError('')
    try {
      if (dirty) {
        if (tab === 'nilai') await saveGrid()
        else if (tab === 'rapor') await saveDetails()
        else if (tab === 'pengaturan') await saveConfig()
        else if (tab === 'presensi' && attendanceActions.current) { await attendanceActions.current.save(); setSaved(true) }
        else if (gradeCPOpen && gradeCPActions.current) await gradeCPActions.current.save()
        else throw new Error('Gunakan tombol simpan pada bagian yang sedang diisi')
      }
      if (aiDirty) await saveAI()
    } catch (e) { setSaveError(e instanceof Error ? e.message : 'Penyimpanan gagal. Isian tetap ada.'); throw e }
    finally { setBusy(false) }
  }
  async function discardCurrent() {
    if (dirty) {
      if (tab === 'nilai') await loadGrid()
      else if (tab === 'rapor') await loadDocument()
      else if (tab === 'pengaturan') await refreshPeriod()
      else if (tab === 'presensi') await attendanceActions.current?.discard()
      else if (gradeCPOpen) await gradeCPActions.current?.discard()
    }
    if (aiDirty) { setAI(await request('/rapor-sumatif/ai/settings', token)); setAIDirty(false) }
    setDirty(false); setSaveError('')
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
    if (!cps.length) throw new Error('Daftar CP belum tersedia')
    if (scores.some(s => s.value !== null && (!Number.isFinite(s.value) || s.value < 0 || s.value > 100))) throw new Error('Nilai harus berupa angka 0–100')
    await request('/rapor-sumatif/grid?' + qs + '&mapelId=' + mapelId, token, 'PUT', {
      revision: period.revision, cps, scores: scores.filter(s => !locked(s.studentId)),
      narratives: narratives.filter(n => narrativeEdits.has(n.studentId)),
    })
    invalidateReport(); await refreshPeriod(); await loadGrid(); setSaved(true); toast.success('Nilai dan deskripsi tersimpan.')
  }
  async function suggest(id: string) {
    if (dirty) throw new Error('Simpan perubahan sebelum meminta saran AI')
    const result = await request('/rapor-sumatif/ai/suggest?' + qs + '&mapelId=' + mapelId, token, 'POST', { studentId: id })
    setSuggestion({ studentId: id, strength: result.strength, growth: result.growth })
  }
  async function showPDF(kind: string, all = false, version = 0) {
    if (hasChanges) throw new Error('Simpan perubahan sebelum membuka cetakan')
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
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div><h2 className="text-xl font-bold">{nilaiOnly ? 'Nilai Sumatif CP' : 'Rapor Semester'}</h2><p className="text-sm text-muted-foreground">Nilai CP, deskripsi capaian, pemeriksaan, dan cetak sesuai blangko PKBM Tunas Ilmu.</p></div>
      {!readOnly && <Button variant={gradeCPOpen ? 'default' : 'outline'} disabled={busy} onClick={() => leave(() => {
        if (gradeCPOpen) invalidateReport()
        setTab(gradeCPOpen ? 'nilai' : 'cp')
      })}>{gradeCPOpen ? 'Kembali ke nilai per rombel' : 'CP per jenjang'}</Button>}
    </div>
    {gradeCPOpen ? <ReportGradeCPPanel token={token} initialSemester={semester} busy={busy} actionsRef={gradeCPActions} setBusy={setBusy} setDirty={setDirty} leave={leave} /> : <>
    <ReportWorkflowGuide current={!kelasId ? 1 : tab === 'nilai' ? cps.length ? 3 : 2 : ['ready', 'final', 'published'].includes(record?.status || '') ? 5 : 4} ready={!!config} canReview={canManage || readOnly} busy={busy} onStep={goTo} />
    {aiProgress && <div className="flex items-center gap-3 rounded-lg border p-3 text-sm" role="status">Membuat saran AI: {aiProgress.done}/{aiProgress.total} siswa<Button variant="outline" onClick={() => aiController.current?.abort()}>Hentikan</Button></div>}
    <Card id="report-scope" tabIndex={-1} className="grid gap-3 p-4 sm:grid-cols-3">
      <div><Label>Tahun ajaran <span className="text-xs text-primary">Wajib</span></Label><Select aria-label="Tahun ajaran rapor" aria-required="true" value={year} disabled={busy} onChange={e => { const value = e.target.value; changeFilter(() => { setYear(value); setKelasId('') }) }}><option value="">Pilih tahun</option>{years.map(y => <option key={y.id} value={y.id}>{y.namaTahunAjaran}{y.isAktif ? '' : ' (arsip)'}</option>)}</Select></div>
      <div><Label>Kelas <span className="text-xs text-primary">Wajib</span></Label><Select aria-label="Kelas rapor" aria-required="true" value={kelasId} disabled={busy} onChange={e => { const value = e.target.value; changeFilter(() => setKelasId(value)) }}><option value="">Pilih kelas</option>{classes.filter(k => k.tahunAjaranId === year).map(k => <option key={k.id} value={k.id}>Kelas {k.jenjang} {k.namaRombel}</option>)}</Select></div>
      <div><Label>Semester <span className="text-xs text-primary">Wajib</span></Label><Select aria-label="Semester rapor" aria-required="true" value={semester} disabled={busy} onChange={e => { const value = e.target.value; changeFilter(() => setSemester(value)) }}><option>Ganjil</option><option>Genap</option></Select></div>
    </Card>
    {config && <>
      <div className="flex flex-wrap gap-2" role="tablist" aria-label="Bagian rapor">{[['nilai', 'Nilai & deskripsi'], ['rapor', 'Pemeriksaan & cetak'], ...(canManage || readOnly ? [['presensi', 'Presensi rapor']] : []), ['leger', 'Leger'], ['pengaturan', 'Pengaturan'], ...(isAdmin ? [['impor', 'Impor workbook']] : [])].map(([id, label]) => <Button key={id} variant={tab === id ? 'default' : 'outline'} disabled={busy} onClick={() => { if (id !== tab) leave(() => setTab(id)) }}>{label}</Button>)}</div>
      <div role="status" aria-live="polite" className={`flex flex-wrap items-center justify-between gap-3 rounded-xl border p-3 text-sm ${hasChanges ? 'border-amber-300 bg-amber-50 text-amber-950' : 'border-primary/20 bg-primary/5'}`}><span><strong>{busy ? 'Sedang memproses…' : hasChanges ? 'Belum disimpan' : saved ? 'Tersimpan' : 'Belum ada perubahan'}</strong>{hasChanges && ' · Simpan sebelum melanjutkan ke bagian lain.'}</span>{hasChanges && <Button size="sm" disabled={busy} onClick={() => void run(saveCurrent)}>Simpan perubahan</Button>}</div>
      {saveError && <p role="alert" className="rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-800">{saveError}</p>}
      {isArchived && <p role="status" className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950">{semesterArchived ? 'Semester ini sudah diarsipkan.' : 'Tahun ajaran ini sudah diarsipkan.'} Nilai, CP, catatan, dan pengaturan hanya dapat dilihat. Admin dapat mengimpor data historis melalui pratinjau.</p>}
      {tab === 'presensi' && (canManage || readOnly) && <div id="report-attendance" tabIndex={-1}><ReportAttendancePanel key={qs} actionsRef={attendanceActions} token={token} qs={qs} students={students} isAdmin={isAdmin} canEdit={canManage && !readOnly && !isArchived} setDirty={setDirty} setParentBusy={setBusy} onSaved={async () => { invalidateReport(); await refreshPeriod(); setSaved(true) }} /></div>}
      {tab === 'nilai' && <>
        <Card id="report-cp" tabIndex={-1} className="space-y-3 p-4"><Label>Mata pelajaran <span className="text-xs text-primary">Wajib</span></Label><Select aria-label="Mata pelajaran rapor" aria-required="true" value={mapelId} disabled={busy} onChange={e => { const value = e.target.value; changeFilter(() => setMapelId(value)) }}><option value="">Pilih mapel</option>{config.subjects.map(m => <option value={m.mapelId} key={m.mapelId}>{m.name}</option>)}</Select>
          {gridLoading && <p role="status">Memuat CP dan nilai tersimpan…</p>}
          {gridError && <p role="alert" className="text-red-700">{gridError} <Button variant="outline" onClick={() => void run(loadGrid)}>Muat ulang nilai</Button></p>}
          {mapelId && !canWrite && !gridLoading && <p className="rounded-lg bg-muted/50 p-3 text-sm">Bagian ini hanya dapat dilihat. Pengisian dilakukan tutor yang ditugaskan pada mapel ini atau admin.</p>}
          {mapelId && !gridLoading && !gridError && cpSource === 'pending' && <p role="status" className="rounded-lg bg-muted/50 p-3 text-sm">CP jenjang belum tersedia untuk mapel dan semester ini. Hubungi admin atau guru penyusun CP.</p>}
          {mapelId && !gridLoading && cpSource === 'legacy' && <p className="rounded-lg bg-muted/50 p-3 text-sm">Rombel ini memakai daftar CP lama untuk menjaga keterkaitan nilai dan deskripsi yang sudah tersimpan.</p>}
          {mapelId && !gridLoading && cpSource === 'shared' && <p className="text-sm text-muted-foreground">Daftar CP jenjang dikelola oleh guru yang ditugaskan admin.</p>}
          {mapelId && cps.length > 0 && <><p className="text-sm text-muted-foreground">Nilai rapor adalah rata-rata CP yang terisi, dibulatkan. Tempel blok angka dari Excel ke sel nilai pertama.</p>
            <details className="rounded-lg border bg-muted/30 p-3 text-sm"><summary className="cursor-pointer font-semibold">Bantuan deskripsi otomatis dan manual</summary><p className="mt-2 text-sm text-muted-foreground">Deskripsi otomatis memakai 1 CP tertinggi dan 1 terendah untuk 1–4 CP terisi, atau masing-masing 2 untuk lebih dari 4 CP. Khusus 1 CP, hanya capaian yang ditampilkan. Awali deskripsi CP dengan huruf kecil; antar-CP digabung dengan “serta”. Deskripsi manual yang sudah tersimpan tetap dipertahankan.</p>
            <p className="text-sm text-muted-foreground">Untuk memperbarui deskripsi draf, pilih “Susun ulang dari CP” di baris siswa, periksa hasilnya, lalu klik “Simpan nilai”. Rapor final dan terbit tetap memakai versi yang sudah dibekukan.</p></details>
            <div className="flex flex-wrap gap-2">{canWrite && !isArchived && !gridLoading && !gridError && <><Button variant="outline" disabled={busy || dirty} onClick={() => void run(suggestClass)}>Bantu AI satu kelas</Button><Button disabled={busy || !dirty} onClick={() => void run(saveGrid)}>Simpan nilai</Button></>}</div>
            <dl className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">{cps.map((cp, i) => <div key={cp.id} className="rounded-md border p-3 text-sm"><dt className="font-semibold">CP {i + 1}</dt><dd>{cp.label}</dd></div>)}</dl>
          </>}
        </Card>
        {mapelId && cps.length > 0 && <Card id="report-scores" tabIndex={-1} className="overflow-auto p-3"><p className="mb-3 text-sm text-muted-foreground">Nilai CP <strong className="text-primary">Wajib sebelum finalisasi</strong>. Kolom kosong dapat disimpan sebagai draf; isi 0 hanya jika nilainya memang nol.</p><table className="w-full text-sm"><thead><tr><th className="min-w-44 p-2 text-left">Peserta didik</th>{cps.map((cp, i) => <th key={cp.id || i} className="min-w-24 p-2">CP {i + 1}</th>)}<th className="p-2">Nilai rapor</th><th className="min-w-80 p-2">Deskripsi akhir</th></tr></thead><tbody>{students.map((student, ri) => {
          const values = cps.map(cp => scoreValue(student.id, cp.id)).filter((v): v is number => v !== null)
          const narrative: Narrative = narratives.find(n => n.studentId === student.id) || { studentId: student.id, ...automaticNarrative(student.id) }
          const disabled = !canWrite || busy || isArchived || locked(student.id)
          return <tr key={student.id} className="border-t align-top"><td className="p-2 font-medium">{student.nama}<div className="text-xs text-muted-foreground">NIS {student.nis}{locked(student.id) ? ' · final' : ''}</div></td>{cps.map((cp, ci) => <td key={cp.id || ci} className="p-1"><Input aria-required="true" className={scoreValue(student.id, cp.id) === null ? requiredClass + ' bg-amber-50 dark:bg-amber-950/20' : 'border-l-4 border-l-primary/25'} aria-label={`${student.nama} CP ${ci + 1}`} type="number" min="0" max="100" step="0.01" disabled={disabled} value={scoreValue(student.id, cp.id) ?? ''} onChange={e => updateScore(student.id, cp.id, e.target.value)} onPaste={e => {
            const raw = e.clipboardData.getData('text/plain'); if (!raw.includes('\t') && !raw.includes('\n')) return
            e.preventDefault(); const rows = raw.trimEnd().split(/\r?\n/); const updates: Score[] = []
            let invalid = false
            rows.forEach((row, r) => row.split('\t').forEach((cell, c) => { const st = students[ri + r], target = cps[ci + c]; if (!st || !target || locked(st.id)) return; const value = cell.trim() === '' ? null : Number(cell.replace(',', '.')); if (value !== null && (!Number.isFinite(value) || value < 0 || value > 100)) { invalid = true; return }; updates.push({ studentId: st.id, cpId: target.id, value }) }))
            if (invalid) { setSaveError('Blok berisi nilai tidak valid. Gunakan angka 0–100; isian sebelumnya tetap ada.'); return }
            setScores(prev => [...prev.filter(v => !updates.some(u => u.cpId === v.cpId && u.studentId === v.studentId)), ...updates]); setDirty(true)
          }} /></td>)}<td className="p-2 text-center font-semibold">{values.length ? Math.round(values.reduce((a, b) => a + b, 0) / values.length) : '—'}<div className="text-xs font-normal">{values.length}/{cps.length} CP</div></td><td className="space-y-2 p-2"><textarea aria-label={'Capaian ' + student.nama} placeholder="Kosong: gunakan narasi otomatis dari CP tertinggi" className={textClass} disabled={disabled} value={narrative?.strength || ''} onChange={e => editNarrative(student.id, 'strength', e.target.value)} /><textarea aria-label={'Penguatan ' + student.nama} placeholder="Penguatan capaian lainnya" className={textClass} disabled={disabled} value={narrative?.growth || ''} onChange={e => editNarrative(student.id, 'growth', e.target.value)} />{narrative?.reviewedRevision === -1 && <p className="text-amber-700">Nilai berubah. Tinjau deskripsi dan simpan kembali.</p>}<div className="flex flex-wrap gap-2">{narratives.some(row => row.studentId === student.id) && <Button size="sm" variant="outline" aria-label={'Susun ulang deskripsi ' + student.nama} title="Mengganti deskripsi pada draf siswa ini dari nilai CP. Klik Simpan nilai / CP untuk menyimpan." disabled={disabled || values.length === 0} onClick={() => { const automatic = automaticNarrative(student.id); editNarrative(student.id, 'strength', automatic.strength); editNarrative(student.id, 'growth', automatic.growth) }}>Susun ulang dari CP</Button>}<Button size="sm" variant="outline" disabled={disabled || dirty} onClick={() => void run(() => suggest(student.id))}>Bantu AI</Button>{narrative && <Button size="sm" variant="outline" disabled={disabled} onClick={() => { setNarrativeEdits(prev => new Set([...prev, student.id])); setDirty(true) }}>Tandai ditinjau</Button>}</div></td></tr>
        })}</tbody></table></Card>}
      </>}
      {tab === 'rapor' && <>
        {!canManage && !readOnly ? <p>Pengisian nilai tersedia pada tab Nilai & deskripsi. Pemeriksaan rapor dilakukan wali kelas atau admin.</p> : <>
          <Card id="report-student" tabIndex={-1} className="space-y-3 p-4"><Label>Peserta didik <span className="text-xs text-primary">Pilih untuk memeriksa rapor</span></Label><Select aria-label="Peserta didik rapor" value={studentId} disabled={busy} onChange={e => { const value = e.target.value; changeFilter(() => setStudentId(value)) }}><option value="">Pilih siswa</option>{students.map(s => <option key={s.id} value={s.id}>{s.nama} · {records.find(r => r.studentId === s.id)?.status || 'draft'}</option>)}</Select>
            <div className="flex flex-wrap gap-2">{['rapor', 'cover', 'identitas', 'lengkap'].map(kind => <Button key={kind} variant="outline" disabled={busy || docLoading || !studentId || !doc} onClick={() => void run(() => showPDF(kind))}>Cetak {kind}</Button>)}<Button variant="outline" disabled={busy || hasChanges || students.length === 0} onClick={() => void run(() => showPDF('lengkap', true))}>Cetak lengkap satu kelas</Button></div>
          </Card>
          {docLoading && <p role="status" className="rounded-lg border p-4">Memuat nilai dan deskripsi terbaru untuk pemeriksaan…</p>}
          {docError && <div role="alert" className="space-y-2 rounded-lg border border-red-300 p-4 text-sm"><p>{docError}</p><Button variant="outline" disabled={busy} onClick={() => void run(loadDocument)}>Muat ulang pemeriksaan</Button></div>}
          {doc && <Card className="space-y-4 p-4"><div><h3 className="font-bold">{doc.student.nama}</h3><p className="mt-1 text-sm font-semibold text-primary">{statusLabels[record?.status || 'draft'] || 'Draf'}</p><p className="text-xs text-muted-foreground">Status: {record?.status || 'draft'} · Versi {record?.version || 0}</p>{record && record.version > 0 && <details className="mt-2"><summary className="cursor-pointer text-sm">Riwayat versi final</summary><div className="mt-2 flex flex-wrap gap-2">{Array.from({ length: record.version }, (_, i) => <Button key={i} variant="outline" size="sm" disabled={busy || dirty} onClick={() => void run(() => showPDF('rapor', false, i + 1))}>Lihat versi {i + 1}</Button>)}</div></details>}</div>
            <table className="w-full text-sm"><thead><tr><th className="text-left">Mata pelajaran</th><th>Nilai</th><th className="text-left">Capaian kompetensi</th></tr></thead><tbody>{doc.subjects.map(m => <tr key={m.mapelId} className="border-t"><td className="p-2">{m.name}</td><td className="p-2 text-center">{m.score ?? '—'}</td><td className="space-y-2 p-2"><p>{m.strength}</p><p>{m.growth}</p>{m.stale && <p className="text-amber-700">Perlu ditinjau</p>}</td></tr>)}</tbody></table>
            <p className="text-sm">Ketidakhadiran: Sakit {doc.attendance.sakit} hari · Izin {doc.attendance.izin} hari · Tanpa keterangan {doc.attendance.alpa} hari</p>
            <p className="text-xs text-muted-foreground">Sumber presensi: {['final', 'published'].includes(record?.status || '') ? 'versi rapor yang sudah dibekukan' : doc.attendanceSource === 'manual' ? 'rekap manual dari catatan kertas' : doc.attendanceSource === 'historical' ? 'rekap historis hasil impor Excel' : 'otomatis dari presensi aplikasi'}. {doc.attendanceSource === 'manual' && canManage && !locked(studentId) && <Button size="sm" variant="outline" disabled={busy || dirty} onClick={() => setTab('presensi')}>Isi / periksa rekap presensi</Button>}</p>
            <fieldset disabled={!canManage || busy || isArchived || locked(studentId)} className="space-y-3"><Label htmlFor="report-note">Catatan wali kelas <span className="text-xs text-muted-foreground">Opsional</span></Label><textarea id="report-note" aria-label="Catatan wali kelas" className={textClass} value={details.note} onChange={e => { setDetails({ ...details, note: e.target.value }); setDirty(true) }} />
              <details className="rounded-lg border p-3"><summary className="cursor-pointer font-medium">Identitas untuk lembar cetak</summary><p className="my-2 text-xs text-muted-foreground">Nama, NIS, dan NISN mengikuti master peserta didik. Data pelengkap disimpan pada rapor semester ini.</p><div className="grid gap-3 sm:grid-cols-2">{[['birthPlace', 'Tempat lahir'], ['birthDate', 'Tanggal lahir'], ['religion', 'Agama'], ['phone', 'Telepon'], ['address', 'Alamat'], ['father', 'Nama ayah'], ['mother', 'Nama ibu'], ['fatherJob', 'Pekerjaan ayah'], ['motherJob', 'Pekerjaan ibu'], ['guardian', 'Nama wali'], ['guardianJob', 'Pekerjaan wali'], ['admissionClass', 'Kelas saat diterima'], ['admissionYear', 'Tahun masuk'], ['admissionStatus', 'Status saat diterima']].map(([key, label]) => <div key={key}><Label>{label}</Label><Input aria-label={label} type={key === 'birthDate' ? 'date' : 'text'} value={details.identity?.[key] || ''} onChange={e => { setDetails({ ...details, identity: { ...details.identity, [key]: e.target.value } }); setDirty(true) }} /></div>)}</div></details>
              {semester === 'Genap' && promotionRules && <div id="report-promotion" tabIndex={-1} className="space-y-3 rounded-lg border border-primary/25 bg-primary/5 p-4">
                <div><Label>{promotionRules.completion ? 'Keputusan kelulusan' : 'Keputusan kenaikan'} <span className="text-xs text-primary">Wajib</span></Label><Select aria-required="true" aria-label={promotionRules.completion ? 'Keputusan kelulusan' : 'Keputusan kenaikan'} value={details.promotion} disabled={!promotionRules.validGrade} onChange={e => { setDetails({ ...details, promotion: e.target.value, nextClass: e.target.value === 'naik' ? promotionRules.nextClass : '' }); setPromotionEdited(true); setDirty(true) }}>
                  <option value="">Belum ditentukan</option>
                  {promotionRules.completion ? <><option value="lulus">Lulus</option><option value="belum_lulus">Belum lulus</option>{['naik', 'tinggal'].includes(details.promotion) && <option value={details.promotion} disabled>{details.promotion === 'naik' ? 'Naik kelas' : 'Tinggal kelas'} (keputusan lama)</option>}</> : <><option value="naik">Naik kelas</option><option value="tinggal">Tinggal kelas</option>{['lulus', 'belum_lulus'].includes(details.promotion) && <option value={details.promotion} disabled>{details.promotion === 'lulus' ? 'Lulus' : 'Belum lulus'} (keputusan lama)</option>}</>}
                </Select></div>
                {!promotionRules.validGrade ? <p className="text-sm text-amber-700">Jenjang kelas belum valid. Hubungi admin untuk memeriksa data kelas.</p> : promotionRules.completion ? <p className="text-sm text-muted-foreground">Kelas 6, 9, dan 12 merupakan kelas akhir program. Pilih keputusan kelulusan sesuai hasil penetapan; tidak ada kelas tujuan kenaikan otomatis.</p> : <p className="text-sm text-muted-foreground">Cukup pilih keputusan. Jika naik kelas, kelas tujuan terisi otomatis berdasarkan kelas pada rapor ini.</p>}
                {details.promotion === 'naik' && <div className="rounded-lg bg-muted/50 p-3 text-sm"><span>{promotionRules.completion ? 'Kelas tujuan pada isian lama' : 'Naik ke kelas'}: </span><output aria-label="Kelas tujuan kenaikan" className="font-semibold">{details.nextClass || promotionRules.nextClass || '—'}</output>
                  {!promotionRules.completion && details.nextClass && details.nextClass !== promotionRules.nextClass && !locked(studentId) && <div className="mt-2 space-y-2"><p className="text-muted-foreground">Tujuan lama tetap tersimpan. Kelas tujuan otomatis untuk rapor ini adalah {promotionRules.nextClass}.</p><Button size="sm" variant="outline" onClick={() => { setDetails({ ...details, nextClass: promotionRules.nextClass }); setPromotionEdited(true); setDirty(true) }}>Gunakan kelas tujuan otomatis</Button></div>}
                </div>}
                {promotionRules.completion && ['naik', 'tinggal'].includes(details.promotion) && !locked(studentId) && <p className="text-sm text-amber-700">Keputusan lama tetap tersimpan. Sebelum finalisasi, pilih Lulus atau Belum lulus untuk kelas akhir program.</p>}
              </div>}
              <section id="report-extras" tabIndex={-1} className="space-y-3 rounded-xl border p-4" aria-label="Ekstrakurikuler siswa">
                <h4 className="font-semibold">Ekstrakurikuler</h4>
                {config.extrasEnabled === false ? <p className="text-sm text-muted-foreground">Ekstrakurikuler tidak diaktifkan untuk kelas dan semester ini. Pengaturannya ada pada tab Pengaturan. Isian lama tetap tersimpan.</p> : <>
                  <p className="text-sm text-muted-foreground">Tambahkan kegiatan yang diikuti siswa, lalu isi keterangan perkembangannya. Siswa yang tidak mengikuti kegiatan boleh dikosongkan.</p>
                  {details.extras.length === 0 && <p className="text-sm">Belum ada kegiatan untuk siswa ini.</p>}
                  {details.extras.map((extra, i) => <div key={i} className="space-y-2 rounded-lg bg-muted/40 p-3">
                    <div className="flex items-center justify-between gap-2"><Label htmlFor={'extra-name-' + i}>Kegiatan {i + 1}</Label><Button variant="ghost" aria-label={'Hapus ekstrakurikuler ' + (i + 1)} onClick={() => { setDetails({ ...details, extras: details.extras.filter((_, j) => i !== j) }); setDirty(true) }}>Hapus</Button></div>
                    {config.extraActivities?.length ? <Select id={'extra-name-' + i} aria-label={'Nama ekstrakurikuler ' + (i + 1)} value={extra.name} onChange={e => { setDetails({ ...details, extras: details.extras.map((v, j) => i === j ? { ...v, name: e.target.value } : v) }); setDirty(true) }}><option value="">Pilih kegiatan...</option>{extra.name && !config.extraActivities.includes(extra.name) && <option value={extra.name}>{extra.name} (isian sebelumnya)</option>}{config.extraActivities.map(name => <option key={name} value={name} disabled={details.extras.some((v, j) => j !== i && v.name === name)}>{name}</option>)}</Select> : <Input id={'extra-name-' + i} aria-label={'Nama ekstrakurikuler ' + (i + 1)} maxLength={100} placeholder="Contoh: Pramuka" value={extra.name} onChange={e => { setDetails({ ...details, extras: details.extras.map((v, j) => i === j ? { ...v, name: e.target.value } : v) }); setDirty(true) }} />}
                    <Label htmlFor={'extra-description-' + i}>Keterangan perkembangan</Label><textarea id={'extra-description-' + i} aria-label={'Keterangan ekstrakurikuler ' + (i + 1)} className={textClass} maxLength={1000} placeholder="Contoh: Aktif mengikuti latihan dan mampu bekerja sama dalam regu." value={extra.description} onChange={e => { setDetails({ ...details, extras: details.extras.map((v, j) => i === j ? { ...v, description: e.target.value } : v) }); setDirty(true) }} /><p className="text-right text-xs text-muted-foreground">{extra.description.length}/1.000 karakter</p>
                  </div>)}
                  <Button variant="outline" disabled={details.extras.length >= 10} onClick={() => { setDetails({ ...details, extras: [...details.extras, { name: '', description: '' }] }); setDirty(true) }}>Tambah ekstrakurikuler</Button>
                </>}
              </section>
              <Button disabled={!dirty} onClick={() => void run(saveDetails)}>Simpan catatan</Button>
            </fieldset>
            {doc.issues.length > 0 ? <div className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950"><strong>Perlu dilengkapi sebelum final:</strong><p className="mt-1">Pilih bagian di bawah untuk menuju isian yang perlu diperiksa.</p><ul className="mt-2 space-y-2">{doc.issues.map((issue, i) => <li key={i}><button type="button" className="min-h-10 w-full rounded-lg border border-amber-300 bg-white px-3 py-2 text-left font-medium underline decoration-amber-400 underline-offset-4 hover:bg-amber-100 disabled:opacity-60" disabled={busy} onClick={() => openIssue(issue)}>{issue} →</button></li>)}</ul></div> : <p className="rounded-lg border border-green-300 bg-green-50 p-3 text-sm text-green-900">Isian wajib lengkap. Periksa nilai, deskripsi, dan identitas sebelum memfinalkan rapor.</p>}
            {canManage && !isArchived && <div id="report-status" tabIndex={-1} className="flex flex-wrap gap-2">{[['ready', 'Siap diperiksa'], ['final', 'Finalkan rapor'], ['published', 'Terbitkan untuk orang tua'], ['draft', 'Buka / revisi draf']].map(([status, label]) => <Button key={status} variant="outline" disabled={busy || dirty || status === 'final' && (record?.status !== 'ready' || doc.issues.length > 0) || status === 'published' && record?.status !== 'final' || status === 'ready' && locked(studentId)} onClick={() => void run(async () => { let reason = ''; if (status === 'draft' && locked(studentId)) { const answer = window.prompt('Alasan pembukaan revisi rapor:'); if (!answer?.trim()) return; reason = answer } await request('/rapor-sumatif/document/' + studentId + '/status?' + qs, token, 'POST', { revision: period.revision, status, reason }); await refreshPeriod(); await loadDocument(); toast.success('Status rapor diperbarui') })}>{label}</Button>)}</div>}
          </Card>}
        </>}
      </>}
      {tab === 'leger' && <Card className="space-y-3 overflow-auto p-4"><div className="flex gap-2"><Button disabled={busy} onClick={() => void run(async () => setLedger(await request('/rapor-sumatif/leger?' + qs, token)))}>Muat leger</Button><Button variant="outline" disabled={busy} onClick={() => void run(() => exportLedger('xlsx'))}>Unduh Excel</Button><Button variant="outline" disabled={busy} onClick={() => void run(() => exportLedger('pdf'))}>Unduh PDF</Button></div><table className="w-full text-sm"><thead><tr><th>Nama</th>{config.subjects.map(m => <th key={m.mapelId} className="p-2">{m.name}</th>)}<th>Jumlah</th><th>Peringkat</th></tr></thead><tbody>{ledger.map(row => <tr key={row.student.id} className="border-t"><td className="p-2">{row.student.nama}</td>{row.subjects.map(m => <td key={m.mapelId} className="p-2 text-center">{m.score ?? '—'}</td>)}<td>{row.total ?? '—'}</td><td>{row.rank ?? '—'}</td></tr>)}</tbody></table></Card>}
      {tab === 'pengaturan' && <>
        <Card id="report-config" tabIndex={-1} className="space-y-4 p-4"><h3 className="font-bold">Identitas dan cetak rapor</h3><p className="text-sm text-muted-foreground">Lembar rapor semester 1 ditandatangani orang tua/wali dan wali kelas. Semester 2 ditambah tanda tangan Kepala PKBM tersendiri di bagian paling akhir tengah, di bawah orang tua/wali dan wali kelas.</p>{settingsLocked && <div role="status" className="space-y-2 rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950"><strong>Pengaturan kelas dikunci</strong><p>Sudah ada rapor final / terbit pada periode ini. Pengaturan digunakan bersama oleh satu kelas. Untuk koreksi, wali kelas atau admin perlu membuka revisi rapor final / terbit yang terkait, disertai alasan.</p><Button variant="outline" disabled={busy} onClick={() => leave(() => { setStudentId(records.find(r => ['final', 'published'].includes(r.status))?.studentId || ''); setTab('rapor') })}>Lihat rapor yang terkunci</Button></div>}<p className="text-sm text-muted-foreground">Kolom bertanda Wajib harus lengkap sebelum finalisasi. Draf pengaturan dapat disimpan bertahap.</p><fieldset disabled={!canManage || busy || isArchived || settingsLocked} className="grid gap-3 sm:grid-cols-2">{[['program', 'Program'], ['phase', 'Fase'], ['city', 'Tempat penetapan'], ['date', 'Tanggal rapor'], ['teacher', 'Nama wali kelas'], ['teacherNip', 'NIP wali kelas'], ['principal', 'Nama kepala PKBM'], ['principalNip', 'NIP kepala PKBM']].map(([key, label]) => { const mandatory = key === 'date' || key === 'teacher' || key === 'principal' && semester === 'Genap'; return <div key={key}><Label htmlFor={'report-config-' + key}>{label} {mandatory ? <span className="text-xs font-semibold text-primary">Wajib</span> : key.endsWith('Nip') ? <span className="text-xs text-muted-foreground">Opsional</span> : null}</Label><Input id={'report-config-' + key} className={mandatory ? requiredClass : undefined} required={mandatory} aria-required={mandatory} aria-label={label} type={key === 'date' ? 'date' : 'text'} value={String(config[key as keyof Config] || '')} onChange={e => { setConfig({ ...config, [key]: e.target.value }); setDirty(true) }} /></div> })}
          <div><Label>Tanda tangan kepala PKBM (semester 2, opsional, PNG)</Label><Input aria-label="Gambar tanda tangan kepala PKBM" type="file" accept="image/png" onChange={e => { const file = e.target.files?.[0]; if (!file) return; if (file.size > 300000) { toast.error('Gambar maksimal 300 KB'); return }; const reader = new FileReader(); reader.onload = () => { setConfig({ ...config, principalSignature: String(reader.result) }); setDirty(true) }; reader.readAsDataURL(file) }} />{config.principalSignature && <Button variant="ghost" onClick={() => { setConfig({ ...config, principalSignature: '' }); setDirty(true) }}>Hapus gambar tanda tangan</Button>}</div><div className="space-y-2 sm:col-span-2"><Label>Urutan dan kelompok mata pelajaran</Label>{config.subjects.map((m, i) => <div key={m.mapelId} className="flex flex-wrap gap-2"><Input className="min-w-56 flex-1" aria-label="Nama cetak mapel" value={m.name} onChange={e => { setConfig({ ...config, subjects: config.subjects.map((v, j) => i === j ? { ...v, name: e.target.value } : v) }); setDirty(true) }} /><Select className="w-48" value={m.group} onChange={e => { setConfig({ ...config, subjects: config.subjects.map((v, j) => i === j ? { ...v, group: e.target.value } : v) }); setDirty(true) }}><option value="umum">Mapel umum</option><option value="pemberdayaan">Pemberdayaan / keterampilan</option></Select><Button variant="outline" disabled={i === 0} onClick={() => { const subjects = [...config.subjects]; [subjects[i - 1], subjects[i]] = [subjects[i], subjects[i - 1]]; setConfig({ ...config, subjects }); setDirty(true) }}>Naik</Button></div>)}</div>
          <section className="space-y-3 rounded-xl border p-4 sm:col-span-2" aria-label="Pengaturan ekstrakurikuler kelas">
            <h4 className="font-semibold">Ekstrakurikuler kelas ini</h4><p className="text-sm text-muted-foreground">Berlaku hanya untuk kelas, tahun ajaran, dan semester yang dipilih. Keterangan diisi per siswa pada Pemeriksaan &amp; cetak.</p>
            <label className="flex items-center gap-2"><input type="checkbox" checked={config.extrasEnabled !== false} onChange={e => { setConfig({ ...config, extrasEnabled: e.target.checked }); setDirty(true) }} />Kelas ini memiliki ekstrakurikuler</label>
            {config.extrasEnabled !== false && <><p className="text-sm">Daftar kegiatan menjadi pilihan saat mengisi rapor siswa. Jika daftar kosong, nama kegiatan dapat ditulis langsung per siswa.</p>{(config.extraActivities || []).map((name, i) => <div key={i} className="flex gap-2"><Input aria-label={'Kegiatan kelas ' + (i + 1)} maxLength={100} placeholder="Contoh: Pramuka, Seni Tari, Futsal" value={name} onChange={e => { setConfig({ ...config, extraActivities: config.extraActivities!.map((v, j) => i === j ? e.target.value : v) }); setDirty(true) }} /><Button variant="ghost" aria-label={'Hapus kegiatan kelas ' + (i + 1)} onClick={() => { setConfig({ ...config, extraActivities: config.extraActivities!.filter((_, j) => i !== j) }); setDirty(true) }}>Hapus</Button></div>)}<Button variant="outline" disabled={(config.extraActivities?.length || 0) >= 20} onClick={() => { setConfig({ ...config, extraActivities: [...(config.extraActivities || []), ''] }); setDirty(true) }}>Tambah kegiatan kelas</Button></>}
          </section>
          <Button disabled={!dirty} onClick={() => void run(saveConfig)}>Simpan pengaturan</Button>
        </fieldset></Card>
        {isAdmin && <Card className="space-y-3 p-4"><h3 className="font-bold">Bantuan AI</h3><p className="text-sm">API key dikonfigurasi pada server. Gemini: {ai.geminiConfigured ? 'tersedia' : 'belum tersedia'} · OpenAI: {ai.openaiConfigured ? 'tersedia' : 'belum tersedia'}</p><Label>Penyedia</Label><Select aria-label={"Penyedia"} value={ai.provider} disabled={busy || isArchived} onChange={e => { setAI({ ...ai, provider: e.target.value }); setAIDirty(true) }}><option value="">Nonaktif</option><option value="gemini">Gemini</option><option value="openai">OpenAI</option></Select><Label>ID model</Label><Input aria-label={"ID model"} value={ai.model} disabled={busy || isArchived} onChange={e => { setAI({ ...ai, model: e.target.value }); setAIDirty(true) }} placeholder="Model yang tersedia pada akun penyedia" /><Button disabled={busy || isArchived || !aiDirty} onClick={() => void run(saveAI)}>Simpan pengaturan AI</Button></Card>}
      </>}
      {tab === 'impor' && isAdmin && <Card className="space-y-4 p-4"><h3 className="font-bold">Impor workbook semester lalu</h3><p className="text-sm">Pilih kelas dan semester tujuan di atas. Data baru disimpan setelah pencocokan dan konfirmasi. Data yang sudah ada tidak ditimpa.</p><Input aria-label="Workbook rapor" type="file" accept=".xlsm,.xlsx" disabled={busy} onChange={e => void run(() => upload(e.target.files?.[0]))} />
        {importData && <><p className="text-sm">Sumber: {importData.preview.source?.year || 'tahun tidak dicantumkan'} · Semester {importData.preview.source?.semester || '—'} · Kelas {importData.preview.source?.class || '—'}</p><p>{importData.preview.students.length} siswa · {importData.preview.subjects.length} sheet mapel</p>{importData.preview.issues.map((v, i) => <p className="text-red-700" key={i}>{v}</p>)}<div className="space-y-2">{importData.preview.students.map(s => <div key={s.row} className="grid gap-2 sm:grid-cols-2"><span>{s.name} · NIS {s.nis} · NISN {s.nisn}</span><Select value={importStudents[s.row] || ''} onChange={e => setImportStudents({ ...importStudents, [s.row]: e.target.value })}><option value="">Cocokkan siswa...</option><option value="skip">Lewati siswa ini</option>{importData.students.map(pd => <option key={pd.id} value={pd.id}>{pd.nama} · {pd.nis}</option>)}</Select></div>)}</div><div className="space-y-2">{importData.preview.subjects.map(s => <div key={s.sheet}><Label>{s.sheet} · {s.labels.length} CP</Label><Select value={importSubjects[s.sheet] || ''} onChange={e => setImportSubjects({ ...importSubjects, [s.sheet]: e.target.value })}><option value="">Cocokkan mapel...</option><option value="skip">Lewati sheet ini</option>{importData.subjects.map(mp => <option key={mp.mapelId} value={mp.mapelId}>{mp.name}</option>)}</Select>{s.issues.map((v, i) => <p key={i} className="text-sm text-red-700">{v}</p>)}</div>)}</div><Button disabled={busy || dirty} onClick={() => void run(async () => { await request('/rapor-sumatif/import/' + importData.id + '/confirm?' + qs, token, 'POST', { revision: period.revision, students: importStudents, subjects: importSubjects }); setImportData(null); await refreshPeriod(); toast.success('Data historis berhasil diimpor') })}>Konfirmasi dan simpan impor</Button></>}
      </Card>}
    </>}
    </>}
    {suggestion && <div role="dialog" aria-modal="true" aria-label="Tinjau saran AI" className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"><Card className="w-full max-w-xl space-y-4 p-5"><p className="text-xs">{suggestionQueue.length + 1} saran menunggu tinjauan</p><h3 className="font-bold">Tinjau saran AI · {students.find(s => s.id === suggestion.studentId)?.nama}</h3><Label>Capaian yang dikuasai</Label><textarea aria-label={"Capaian yang dikuasai"} className={textClass} value={suggestion.strength} onChange={e => setSuggestion({ ...suggestion, strength: e.target.value })} /><Label>Penguatan</Label><textarea aria-label={"Penguatan"} className={textClass} value={suggestion.growth} onChange={e => setSuggestion({ ...suggestion, growth: e.target.value })} /><p className="text-xs text-muted-foreground">Penerapan mengganti deskripsi pada draf. Periksa isinya lalu simpan nilai.</p><div className="flex gap-2"><Button onClick={() => { editNarrative(suggestion.studentId, 'strength', suggestion.strength); editNarrative(suggestion.studentId, 'growth', suggestion.growth); setSuggestion(suggestionQueue[0] || null); setSuggestionQueue(rows => rows.slice(1)) }}>Terapkan pada draf</Button><Button variant="outline" onClick={() => { setSuggestion(null); setSuggestionQueue([]) }}>Batal</Button></div></Card></div>}
    {previewURL && <div role="dialog" aria-modal="true" aria-label="Pratinjau cetak rapor" className="fixed inset-0 z-50 flex flex-col bg-background p-4"><div className="mb-3 flex items-center gap-3"><strong>Pratinjau PDF</strong><a href={previewURL} download="rapor.pdf" className="underline">Unduh PDF</a><Button className="ml-auto" variant="outline" onClick={() => setPreviewURL('')}>Tutup</Button></div><iframe title="PDF rapor" src={previewURL} className="w-full flex-1 rounded-lg border" /></div>}
  </div>
}
