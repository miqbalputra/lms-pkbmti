import { cloneElement, isValidElement, useCallback, useEffect, useId, useMemo, useRef, useState, type DragEvent, type FormEvent, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowDown, ArrowUp, BookOpen, CalendarDays, CheckCircle2, CircleDot, ClipboardCheck, Copy, Download, ExternalLink, FileText, FileUp, GripVertical, Grid2X2, ImageIcon, KeyRound, Link2, ListChecks, ListOrdered, Plus, Redo2, Save, Send, Star, Table2, TextCursorInput, Trash2, Undo2, UsersRound, Wifi, WifiOff, X } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Checkbox } from '../components/ui/checkbox'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../components/ui/dialog'
import { QuestionAnswerControl, StimulusContent, StimulusImage } from '../components/simulasi/QuestionAnswerControl'
import { AssessmentCollaboratorManager } from '../components/AssessmentCollaborators'
import { configExample, questionTypes, type QuestionType } from '../components/simulasi/questionTypes'
import type { User } from '../App'
import { apiBase, downloadFile, request, type ApiError } from '../lib/api'
import { countDrafts, enqueueDraft, listDrafts, removeDraft } from '../lib/draftQueue'

type Row = Record<string, any> & { id: string }
type CanvasPackageRef = Omit<Row, 'id'> & { id?: string }
const emptyQuestion = () => ({ jenjang: 'SD/MI', kelasFase: 'Kelas 5-6 / Fase C', mode: 'anbk_akm', domain: '', topik: '', kompetensi: '', levelKognitif: '', tingkatKesulitan: 'sedang', tags: '', tipe: 'pg_tunggal' as QuestionType, pertanyaan: '', wajibDijawab: true, konfigurasi: configExample('pg_tunggal'), pembahasan: '', bobot: '1', status: 'draf', stimulus: [] as Row[], mapelId: '' })
const emptyPaket = () => ({ nama: '', deskripsi: '', mode: 'anbk_akm', jenjang: 'SD/MI', mapelId: '', durasiMenit: '60', instruksi: 'Baca setiap soal dengan teliti. Kamu boleh menandai soal untuk diperiksa kembali.', nilaiLulus: '', waktuMulai: '', waktuSelesai: '', maksPercobaan: '1', acakUrutan: true, tampilkanNilai: true, tampilkanRingkasan: true, tampilkanPembahasan: false, izinkanEditRespons: false, temaWarna: '#1c5d94', pesanKonfirmasi: 'Jawaban kamu sudah berhasil dikirim.' })

export function SimulasiView({ token, user }: { token: string; user: User }) {
  const navigate = useNavigate()
  const [questions, setQuestions] = useState<Row[]>([])
  const [legacyQuestions, setLegacyQuestions] = useState<Row[]>([])
  const [packages, setPackages] = useState<Row[]>([])
  const [materials, setMaterials] = useState<Row[]>([])
  const [mapel, setMapel] = useState<Row[]>([])
  const [classes, setClasses] = useState<Row[]>([])
  const [students, setStudents] = useState<Row[]>([])
  const [questionOpen, setQuestionOpen] = useState(false)
  const [packageOpen, setPackageOpen] = useState(false)

  const [canvasPaket, setCanvasPaket] = useState<CanvasPackageRef | null | undefined>(undefined)
  const [packageForm, setPackageForm] = useState(emptyPaket)
  const [selectedPackage, setSelectedPackage] = useState<Row | null>(null)
  const [items, setItems] = useState<Row[]>([])
  const [assigned, setAssigned] = useState<string[]>([])
  const [results, setResults] = useState<Row[]>([])
  const [shareTokens, setShareTokens] = useState<Row[]>([])
  const [attemptDetail, setAttemptDetail] = useState<Row | null>(null)
  const [saving, setSaving] = useState(false)
  const [activeTab, setActiveTab] = useState(() => new URLSearchParams(window.location.search).get('tab') || 'paket')
  const [online, setOnline] = useState(() => typeof navigator === 'undefined' ? true : navigator.onLine)
  const [offlineDrafts, setOfflineDrafts] = useState(0)

  const canWrite = user.role === 'admin' || user.role === 'guru'
  const load = () => {
    void Promise.all([
      request('/simulasi/soal', token), request('/bank-soal', token).catch(() => []), request('/simulasi/paket', token), request('/simulasi/bahan', token).catch(() => []), request('/simulasi/mapel', token), request('/kelas', token).catch(() => []), request('/peserta-didik', token).catch(() => []),
    ]).then(([q, legacy, p, b, m, k, s]) => { setQuestions(Array.isArray(q) ? q : []); setLegacyQuestions(Array.isArray(legacy) ? legacy : []); setPackages(Array.isArray(p) ? p : []); setMaterials(Array.isArray(b) ? b : []); setMapel(Array.isArray(m) ? m : []); setClasses(Array.isArray(k) ? k : []); setStudents(Array.isArray(s) ? s : []) }).catch((error) => toast.error(String(error.message || error)))
  }
  useEffect(load, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  const syncOffline = async () => {
    if (!navigator.onLine) return
    const entries = (await listDrafts().catch(() => [])).filter((entry) => !entry.key.startsWith('builder-'))
    for (const entry of entries) {
      try {
        await request(entry.path, token, entry.method, entry.payload)
        await removeDraft(entry.key)
      } catch (error) {
        if ((error as ApiError).status === 409) toast.error('Draf offline konflik dengan versi server. Buka kanvas untuk memilih versi.')
        break
      }
    }
    setOfflineDrafts(await countDrafts())
    if (entries.length) load()
  }
  useEffect(() => {
    const onlineHandler = () => { setOnline(true); void syncOffline() }
    const offlineHandler = () => setOnline(false)
    window.addEventListener('online', onlineHandler)
    window.addEventListener('offline', offlineHandler)
    void countDrafts().then(setOfflineDrafts)
    if (navigator.onLine) void syncOffline()
    return () => { window.removeEventListener('online', onlineHandler); window.removeEventListener('offline', offlineHandler) }
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  async function savePackage(event: FormEvent) {
    event.preventDefault(); setSaving(true)
    try {
      const toTime = (value: string) => value ? new Date(value).toISOString() : null
      await request('/simulasi/paket', token, 'POST', { ...packageForm, mapelId: packageForm.mapelId || null, durasiMenit: Number(packageForm.durasiMenit), maksPercobaan: Number(packageForm.maksPercobaan), nilaiLulus: packageForm.nilaiLulus ? Number(packageForm.nilaiLulus) : null, waktuMulai: toTime(packageForm.waktuMulai), waktuSelesai: toTime(packageForm.waktuSelesai) })
      toast.success('Paket draf dibuat. Tambahkan soal dan peserta sebelum menerbitkan.'); setPackageOpen(false); setPackageForm(emptyPaket()); load()
    } catch (error: any) { toast.error(error.message || 'Paket belum dapat disimpan.') } finally { setSaving(false) }
  }
  async function copyLegacyQuestion(id: string) {
    try { await request(`/simulasi/soal/from-bank/${id}`, token, 'POST'); toast.success('Soal lama disalin sebagai draf visual.'); load() } catch (error: any) { toast.error(error.message || 'Soal lama belum dapat disalin.') }
  }
  async function archiveQuestion(id: string) {
    if (!window.confirm('Arsipkan soal ini? Soal tidak akan dihapus dari riwayat paket yang sudah terbit.')) return
    try { await request(`/simulasi/soal/${id}`, token, 'DELETE'); toast.success('Soal diarsipkan dari Bank Soal.'); load() } catch (error: any) { toast.error(error.message || 'Soal belum dapat diarsipkan.') }
  }
  async function selectPackage(row: Row) {
    setSelectedPackage(row); setResults([])
    try {
      const [packetItems, assignments, packetResults, tokens] = await Promise.all([request(`/simulasi/paket/${row.id}/soal`, token), request(`/simulasi/paket/${row.id}/penugasan`, token), request(`/simulasi/paket/${row.id}/hasil`, token), request(`/simulasi/paket/${row.id}/share-token`, token).catch(() => [])])
      setItems(Array.isArray(packetItems) ? packetItems : []); setAssigned(Array.isArray(assignments) ? assignments.map((item: Row) => String(item.pesertaDidikId)) : []); setResults(Array.isArray(packetResults) ? packetResults : [])
      setShareTokens(Array.isArray(tokens) ? tokens : [])
    } catch (error: any) { toast.error(error.message || 'Detail paket tidak dapat dimuat.') }
  }
  async function attachQuestion(question: Row) {
    if (!selectedPackage) return
    try { await request(`/simulasi/paket/${selectedPackage.id}/soal`, token, 'POST', { soalId: question.id, bobot: question.bobot }); toast.success('Soal ditambahkan.'); void selectPackage(selectedPackage) } catch (error: any) { toast.error(error.message || 'Soal tidak dapat ditambahkan.') }
  }
  async function assignStudents() {
    if (!selectedPackage) return
    try { await request(`/simulasi/paket/${selectedPackage.id}/penugasan`, token, 'PUT', { pesertaDidikIds: assigned }); toast.success('Penugasan peserta disimpan.') } catch (error: any) { toast.error(error.message || 'Penugasan gagal.') }
  }
  async function publish() {
    if (!selectedPackage) return
    try { const updated = await request(`/simulasi/paket/${selectedPackage.id}/publikasi`, token, 'POST'); toast.success('Paket diterbitkan dan soal dibekukan.'); setSelectedPackage(updated); load() } catch (error: any) { toast.error(error.message || 'Paket belum siap diterbitkan.') }
  }
  async function duplicate() {
    if (!selectedPackage) return
    try { const copied = await request(`/simulasi/paket/${selectedPackage.id}/duplikasi`, token, 'POST'); toast.success('Salinan draf dibuat.'); load(); void selectPackage(copied) } catch (error: any) { toast.error(error.message || 'Paket tidak dapat diduplikasi.') }
  }
  async function createShareToken(options?: { prefill?: Record<string, unknown>; embedOrigins?: string[] }) {
    if (!selectedPackage) return
    try {
      const hasPrefill = Boolean(options?.prefill && Object.keys(options.prefill).length)
      const created = await request(`/simulasi/paket/${selectedPackage.id}/share-token`, token, 'POST', { label: options?.embedOrigins?.length ? 'Sematan siswa' : hasPrefill ? 'Tautan siswa dengan isian awal' : 'Tautan siswa', ...(hasPrefill ? { prefill: options?.prefill } : {}), ...(options?.embedOrigins?.length ? { embedOrigins: options.embedOrigins } : {}) })
      setShareTokens((current) => [created, ...current]);
      try {
        await navigator.clipboard?.writeText(String(created.url || ''))
        toast.success('Tautan pengerjaan dibuat dan disalin.')
      } catch {
        toast.success('Tautan pengerjaan dibuat. Salin dari daftar tautan.')
      }
      return created
    } catch (error: any) { toast.error(error.message || 'Tautan belum dapat dibuat.'); return undefined }
  }
  async function revokeShareToken(id: string) {
    if (!selectedPackage) return
    try { await request(`/simulasi/paket/${selectedPackage.id}/share-token/${id}`, token, 'DELETE'); setShareTokens((current) => current.map((row) => row.id === id ? { ...row, status: 'dicabut', revokedAt: new Date().toISOString() } : row)); toast.success('Tautan dicabut.') } catch (error: any) { toast.error(error.message || 'Tautan belum dapat dicabut.') }
  }
  function openPackagePreview(id: string) {
    window.open(`/simulasi/preview/${encodeURIComponent(id)}`, '_blank', 'noopener,noreferrer')
  }
  async function inspectAttempt(attempt: Row) {
    try { setAttemptDetail(await request(`/simulasi/upaya/${attempt.id}/detail`, token)) } catch (error: any) { toast.error(error.message || 'Detail jawaban tidak dapat dimuat.') }
  }
  async function gradeEssay(answer: Row, skor: number, komentar: string) {
    if (!attemptDetail) return
    try {
      await request(`/simulasi/upaya/${attemptDetail.upaya.id}/jawaban/${answer.id}/nilai`, token, 'POST', { skor, komentar })
      toast.success('Nilai uraian disimpan.'); await inspectAttempt(attemptDetail.upaya); if (selectedPackage) void selectPackage(selectedPackage)
    } catch (error: any) { toast.error(error.message || 'Nilai uraian belum tersimpan.') }
  }
  async function importQuestions(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]; if (!file) return
    const form = new FormData(); form.append('file', file)
    try {
      const response = await fetch(apiBase + '/simulasi/soal/import', { method: 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: form })
      const body = await response.json().catch(() => ({})); if (!response.ok) throw new Error(body.error || 'Impor gagal')
      toast.success(`${body.created || 0} soal diimpor.`); load()
    } catch (error: any) { toast.error(error.message || 'Impor gagal.') } finally { event.target.value = '' }
  }
  const availableQuestions = useMemo(() => questions.filter((question) => question.status === 'terbit' && !items.some((item) => item.soalId === question.id)), [questions, items])

  return <div data-assessment-workspace="staff" className="space-y-4">
    {canvasPaket !== undefined && <BuilderCanvas token={token} students={students} classes={classes} bank={questions} materials={materials} initialPaket={canvasPaket} onClose={() => { setCanvasPaket(undefined); load() }} />}
    {canvasPaket === undefined && <PageToolbar title="Simulasi & Bank Soal" description="Satu workspace visual untuk bahan reusable, soal, paket, dan hasil ANBK/TKA SD." actions={<div className="flex flex-wrap items-center gap-2">{online ? <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-3 py-1.5 text-xs font-medium text-emerald-700"><Wifi className="h-3.5 w-3.5" /> Online</span> : <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-3 py-1.5 text-xs font-medium text-amber-700"><WifiOff className="h-3.5 w-3.5" /> Offline — draf lokal</span>}{offlineDrafts > 0 && <span className="rounded-full bg-muted px-3 py-1.5 text-xs font-medium">{offlineDrafts} antrean</span>}<Button variant="outline" onClick={() => navigate('/bank-soal-ujian')}><BookOpen className="h-4 w-4" /> Bank soal Ujian Online</Button>{canWrite && <><Button variant="outline" onClick={() => void downloadFile('/simulasi/soal/template', token, 'template-bank-soal-simulasi.xlsx')}><Download className="h-4 w-4" /> Template XLSX</Button><label className="inline-flex"><input className="sr-only" type="file" accept=".xlsx" onChange={importQuestions} /><Button type="button" variant="outline" asChild><span><FileUp className="h-4 w-4" /> Impor bank</span></Button></label><Button onClick={() => setQuestionOpen(true)}><Plus className="h-4 w-4" /> Buat soal</Button><Button variant="secondary" onClick={() => setCanvasPaket(null)}><ClipboardCheck className="h-4 w-4" /> Buat paket</Button></>}</div>} />}
    {questionOpen && <VisualQuestionForm token={token} mapel={mapel} onCancel={() => setQuestionOpen(false)} onSaved={() => { setQuestionOpen(false); load() }} />}
    {packageOpen && <PackageForm form={packageForm} setForm={setPackageForm} mapel={mapel} saving={saving} onCancel={() => setPackageOpen(false)} onSubmit={savePackage} />}
    <Tabs value={activeTab} onValueChange={setActiveTab}>
      <TabsList className="grid h-auto w-full grid-cols-2 gap-1 sm:inline-flex sm:w-auto sm:grid-cols-none"><TabsTrigger className="min-h-11" value="bahan">Bahan</TabsTrigger><TabsTrigger className="min-h-11" value="soal">Soal</TabsTrigger><TabsTrigger className="min-h-11" value="paket">Paket</TabsTrigger><TabsTrigger className="min-h-11" value="hasil">Hasil</TabsTrigger></TabsList>
      <TabsContent value="bahan"><MaterialsPanel token={token} materials={materials} canWrite={canWrite} onReload={load} /></TabsContent>
    <TabsContent value="paket"><div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(380px,1.1fr)]"><Card className="overflow-hidden"><div className="overflow-x-auto"><Table><TableHeader><TableRow><TableHead>Nama paket</TableHead><TableHead>Mode</TableHead><TableHead>Status</TableHead><TableHead /></TableRow></TableHeader><TableBody>{packages.map((row) => <TableRow key={row.id}><TableCell><div className="font-semibold">{row.nama || 'Paket tanpa judul'}</div><div className="text-xs text-muted-foreground">{row.durasiMenit} menit · maks. {row.maksPercobaan} percobaan</div></TableCell><TableCell>{row.mode === 'tka_sd' ? 'TKA SD' : 'ANBK/AKM'}</TableCell><TableCell><Badge variant={row.status === 'terbit' ? 'default' : 'secondary'}>{row.status}</Badge></TableCell><TableCell><Button className="min-h-11" size="sm" variant="outline" onClick={() => row.status === 'draf' ? setCanvasPaket(row) : void selectPackage(row)}>{row.status === 'draf' ? 'Buka kanvas' : 'Kelola'}</Button></TableCell></TableRow>)}{!packages.length && <EmptyState colSpan={4} label="Belum ada paket simulasi." />}</TableBody></Table></div></Card><Card className="p-4">{selectedPackage ? <PackageDetail packet={selectedPackage} token={token} currentUser={user} items={items} questions={availableQuestions} classes={classes} students={students} assigned={assigned} setAssigned={setAssigned} results={results} shareTokens={shareTokens} detail={attemptDetail} writable={canWrite && selectedPackage.status === 'draf'} canGrade={canWrite} onAttach={attachQuestion} onAssign={assignStudents} onPublish={publish} onDuplicate={duplicate} onInspect={inspectAttempt} onGrade={gradeEssay} onCreateShare={createShareToken} onRevokeShare={revokeShareToken} onPreview={() => openPackagePreview(selectedPackage.id)} onExport={() => void downloadFile(`/simulasi/paket/${selectedPackage.id}/export`, token, 'hasil-simulasi.csv')} onExportXlsx={() => void downloadFile(`/simulasi/paket/${selectedPackage.id}/export?format=xlsx`, token, 'hasil-simulasi.xlsx')} /> : <EmptyState title="Pilih paket" description="Pilih paket di sebelah kiri untuk menyusun soal, menugaskan siswa, dan melihat hasil." />}</Card></div></TabsContent>
      <TabsContent value="soal"><div className="space-y-4"><Card className="overflow-hidden"><div className="overflow-x-auto"><Table><TableHeader><TableRow><TableHead>Soal</TableHead><TableHead>Mode</TableHead><TableHead>Tipe</TableHead><TableHead>Status</TableHead><TableHead>Bobot</TableHead><TableHead>Aksi</TableHead></TableRow></TableHeader><TableBody>{questions.map((row) => <TableRow key={row.id}><TableCell><div className="max-w-xl font-medium line-clamp-2">{row.pertanyaan}</div><div className="text-xs text-muted-foreground">{row.topik || row.domain || 'Tanpa topik'}</div></TableCell><TableCell>{row.mode === 'tka_sd' ? 'TKA SD' : 'ANBK/AKM'}</TableCell><TableCell>{questionTypes.find(([id]) => id === row.tipe)?.[1] || row.tipe}</TableCell><TableCell><Badge variant={row.status === 'terbit' ? 'default' : 'secondary'}>{row.status}</Badge></TableCell><TableCell>{row.bobot}</TableCell><TableCell><div className="flex flex-wrap gap-2"><Button size="sm" className="min-h-11" variant="outline" onClick={() => setCanvasPaket({ prefillQuestion: row })}>Pakai di paket</Button>{canWrite && <Button size="sm" className="min-h-11" variant="outline" onClick={() => void archiveQuestion(row.id)} aria-label={`Arsipkan soal ${row.pertanyaan || row.id}`}><Trash2 className="h-4 w-4" />Hapus</Button>}</div></TableCell></TableRow>)}{!questions.length && <EmptyState colSpan={6} label="Belum ada soal simulasi." />}</TableBody></Table></div></Card>{legacyQuestions.length > 0 && <Card className="overflow-hidden"><div className="border-b bg-amber-50 p-4"><h2 className="font-bold">Soal Bank Soal lama</h2><p className="mt-1 text-sm text-amber-900/70">Data lama tetap aman. Salin menjadi draf visual jika ingin dipakai di Simulasi.</p></div><div className="overflow-x-auto"><Table><TableHeader><TableRow><TableHead>Pertanyaan</TableHead><TableHead>Tipe</TableHead><TableHead>Poin</TableHead><TableHead /></TableRow></TableHeader><TableBody>{legacyQuestions.slice(0, 25).map((row) => <TableRow key={row.id}><TableCell className="max-w-xl truncate">{row.pertanyaan}</TableCell><TableCell>{row.tipe === 'pg' ? 'Pilihan ganda lama' : 'Essay lama'}</TableCell><TableCell>{row.poin}</TableCell><TableCell className="text-right">{canWrite && <Button size="sm" className="min-h-11" variant="outline" onClick={() => void copyLegacyQuestion(row.id)}>Salin ke workspace</Button>}</TableCell></TableRow>)}</TableBody></Table></div></Card>}</div></TabsContent>
      <TabsContent value="hasil"><div className="space-y-4"><AssessmentAnalyticsPanel token={token} classes={classes} students={students} subjects={mapel} /><ResultsPanel packages={packages} results={results} onSelect={selectPackage} /></div></TabsContent>
    </Tabs>
  </div>
}

export function SimulasiPreviewView({ token }: { token: string }) {
  const [data, setData] = useState<Row | null>(null)
  const [error, setError] = useState('')
  const packageID = window.location.pathname.split('/').filter(Boolean).pop() || ''
  useEffect(() => { if (!packageID) return; void request(`/simulasi/paket/${encodeURIComponent(packageID)}/preview`, token).then(setData).catch((reason) => setError(reason.message || 'Pratinjau tidak tersedia.')) }, [packageID, token])
  if (error) return <main data-assessment-workspace="preview" className="grid min-h-screen place-items-center bg-slate-100 p-6"><Card className="max-w-md p-6 text-center"><h1 className="font-bold">Pratinjau tidak tersedia</h1><p className="mt-2 text-sm text-muted-foreground">{error}</p><Button className="mt-5" onClick={() => window.close()}>Tutup tab</Button></Card></main>
  if (!data) return <div className="grid min-h-screen place-items-center bg-slate-100 text-sm text-muted-foreground">Memuat pratinjau…</div>
  const packet = data.paket || {}
  return <main data-assessment-workspace="preview" className="min-h-screen bg-slate-100 p-4 text-slate-900 sm:p-8"><div className="mx-auto max-w-3xl space-y-4"><header className="sticky top-3 z-10 flex flex-wrap items-center justify-between gap-3 rounded-2xl border bg-white/95 p-4 shadow-sm backdrop-blur"><div><p className="text-xs font-bold uppercase tracking-wide text-primary">Pratinjau siswa</p><h1 className="text-xl font-bold">{packet.nama || 'Paket simulasi'}</h1><p className="mt-1 text-xs text-muted-foreground">{data.soal?.length || 0} soal · {packet.durasiMenit || 0} menit</p></div><Button variant="outline" onClick={() => window.close()}><X className="h-4 w-4" /> Tutup tab</Button></header><Card className="p-4 sm:p-6"><p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">{packet.instruksi || 'Ini hanya pratinjau. Jawaban tidak disimpan.'}</p></Card>{(data.soal || []).map((item: Row, index: number) => <div key={item.id || index}><div className="mb-2 px-1 text-xs font-semibold text-muted-foreground">Soal {index + 1} · {item.bobot || 1} poin</div><StudentQuestionPreview key={item.id || index} question={item.soal || item} token={token} /></div>)}</div></main>
}

function AssessmentAnalyticsPanel({ token, classes, students, subjects }: { token: string; classes: Row[]; students: Row[]; subjects: Row[] }) {
  const [filters, setFilters] = useState({ modul: '', mapelId: '', kelasId: '', pesertaDidikId: '', status: '', dari: '', sampai: '' })
  const [data, setData] = useState<Row | null>(null)
  const [questionData, setQuestionData] = useState<Row | null>(null)
  const [loading, setLoading] = useState(false)
  const query = () => {
    const params = new URLSearchParams()
    Object.entries(filters).forEach(([key, value]) => { if (value) params.set(key, value) })
    return params.toString()
  }
  async function refresh() {
    setLoading(true)
    try {
      const suffix = query() ? `?${query()}` : ''
      const [summary, questions] = await Promise.all([
        request(`/assessment/analytics${suffix}`, token),
        request(`/assessment/analytics/questions${suffix}`, token),
      ])
      setData(summary)
      setQuestionData(questions)
    }
    catch (error: any) { toast.error(error.message || 'Analitik asesmen tidak dapat dimuat.') }
    finally { setLoading(false) }
  }
  useEffect(() => {
    const timer = window.setTimeout(() => void refresh(), 250)
    return () => window.clearTimeout(timer)
  }, [token, filters]) // eslint-disable-line react-hooks/exhaustive-deps
  const download = (format: 'csv' | 'xlsx') => {
    const params = new URLSearchParams(query())
    if (format === 'xlsx') params.set('format', 'xlsx')
    void downloadFile(`/assessment/analytics/export?${params.toString()}`, token, `laporan-hasil-asesmen.${format}`).catch((error: any) => toast.error(error.message || 'Laporan tidak dapat diunduh.'))
  }
  const downloadPDF = () => {
    if (!filters.kelasId && !filters.pesertaDidikId) {
      toast.error('Pilih kelas atau siswa terlebih dahulu untuk laporan PDF.')
      return
    }
    const params = new URLSearchParams(query())
    void downloadFile(`/assessment/analytics/report.pdf?${params.toString()}`, token, 'laporan-hasil-asesmen.pdf').catch((error: any) => toast.error(error.message || 'Laporan PDF tidak dapat diunduh.'))
  }
  const downloadQuestionStats = (format: 'csv' | 'xlsx') => {
    const params = new URLSearchParams(query())
    if (format === 'xlsx') params.set('format', 'xlsx')
    void downloadFile(`/assessment/analytics/questions/export?${params.toString()}`, token, `analisis-per-soal.${format}`).catch((error: any) => toast.error(error.message || 'Analisis per soal tidak dapat diunduh.'))
  }
  const summary = data?.ringkasan || {}
  const records = Array.isArray(data?.pengerjaan) ? data.pengerjaan as Row[] : []
  const progress = Array.isArray(data?.perkembanganSiswa) ? data.perkembanganSiswa as Row[] : []
  const scoredAttemptsByStudent = useMemo(() => {
    const grouped = new Map<string, Row[]>()
    const sourceRecords = Array.isArray(data?.pengerjaan) ? data.pengerjaan as Row[] : []
    for (const record of sourceRecords) {
      if (record.nilai == null || record.pesertaDidikId == null) continue
      const studentId = String(record.pesertaDidikId)
      const attempts = grouped.get(studentId) || []
      attempts.push(record)
      grouped.set(studentId, attempts)
    }
    const latest = new Map<string, Row[]>()
    grouped.forEach((attempts, studentId) => {
      attempts.sort((a, b) => String(a.mulai || '').localeCompare(String(b.mulai || '')))
      latest.set(studentId, attempts.slice(-8))
    })
    return latest
  }, [data?.pengerjaan])
  const classProgress = Array.isArray(data?.statistikKelas) ? data.statistikKelas as Row[] : []
  const questionStats = useMemo(() => Array.isArray(questionData?.soal) ? questionData.soal as Row[] : [], [questionData?.soal])
  const domainProgress = useMemo(() => {
    const grouped = new Map<string, { domain: string; competency: string; items: number; attempts: number; answered: number; correct: number; pending: number; earned: number; weight: number }>()
    for (const question of questionStats) {
      const domain = String(question.domain || 'Tanpa domain')
      const competency = String(question.kompetensi || 'Tanpa kompetensi')
      const key = `${domain.toLocaleLowerCase('id')}\u0000${competency.toLocaleLowerCase('id')}`
      const current = grouped.get(key) || { domain, competency, items: 0, attempts: 0, answered: 0, correct: 0, pending: 0, earned: 0, weight: 0 }
      current.items += 1
      current.attempts += Number(question.percobaan || 0)
      current.answered += Number(question.terjawab || 0)
      current.correct += Number(question.benar || 0)
      current.pending += Number(question.menungguNilai || 0)
      current.earned += Number(question.nilaiTercapai || 0)
      current.weight += Number(question.bobotDinilai || 0)
      grouped.set(key, current)
    }
    return [...grouped.values()].sort((left, right) => left.domain.localeCompare(right.domain, 'id') || left.competency.localeCompare(right.competency, 'id'))
  }, [questionStats])
  const setFilter = (key: keyof typeof filters, value: string) => setFilters((current) => ({ ...current, [key]: value }))
  const fieldClass = 'min-h-11 w-full rounded-lg border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'
  return <Card className="overflow-hidden">
    <div className="border-b bg-muted/20 p-4 sm:p-5"><div className="flex flex-wrap items-start justify-between gap-3"><div><h2 className="font-bold">Analitik gabungan asesmen</h2><p className="mt-1 max-w-3xl text-sm text-muted-foreground">Pantau riwayat nilai Ujian Online dan Simulasi ANBK/TKA per siswa dan kelas. Histori kelas lama dipertahankan saat siswa berpindah rombel.</p></div><div className="flex flex-wrap gap-2"><Button className="min-h-11" variant="outline" disabled={loading} onClick={() => void refresh()}>{loading ? 'Memuat…' : 'Perbarui'}</Button><Button className="min-h-11" variant="outline" onClick={() => download('csv')}><Download className="h-4 w-4" /> CSV</Button><Button className="min-h-11" variant="outline" onClick={() => download('xlsx')}><Download className="h-4 w-4" /> XLSX</Button><Button className="min-h-11" variant="outline" disabled={!filters.kelasId && !filters.pesertaDidikId} onClick={downloadPDF} title="Pilih kelas atau siswa untuk laporan PDF"><Download className="h-4 w-4" /> PDF</Button></div></div>
      <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <label className="space-y-1 text-xs font-medium text-muted-foreground"><span>Modul</span><select className={fieldClass} value={filters.modul} onChange={(event) => setFilter('modul', event.target.value)}><option value="">Semua modul</option><option value="ujian_online">Ujian Online</option><option value="simulasi">Simulasi Asesmen</option></select></label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground"><span>Mata pelajaran</span><select className={fieldClass} value={filters.mapelId} onChange={(event) => setFilter('mapelId', event.target.value)}><option value="">Semua mata pelajaran</option>{subjects.map((row) => <option key={row.id} value={row.id}>{row.namaMapel}</option>)}</select></label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground"><span>Kelas</span><select className={fieldClass} value={filters.kelasId} onChange={(event) => setFilter('kelasId', event.target.value)}><option value="">Semua kelas</option>{classes.map((row) => <option key={row.id} value={row.id}>{`Kelas ${row.jenjang} ${row.namaRombel}`}</option>)}</select></label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground"><span>Siswa</span><select className={fieldClass} value={filters.pesertaDidikId} onChange={(event) => setFilter('pesertaDidikId', event.target.value)}><option value="">Semua siswa</option>{students.map((row) => <option key={row.id} value={row.id}>{row.nama}</option>)}</select></label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground"><span>Status</span><select className={fieldClass} value={filters.status} onChange={(event) => setFilter('status', event.target.value)}><option value="">Semua status</option><option value="selesai">Selesai</option><option value="berlangsung">Berlangsung</option><option value="menunggu_nilai">Menunggu penilaian</option><option value="dikunci">Dikunci</option><option value="kedaluwarsa">Kedaluwarsa</option></select></label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground"><span>Dari tanggal</span><Input className="min-h-11" type="date" value={filters.dari} onChange={(event) => setFilter('dari', event.target.value)} /></label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground"><span>Sampai tanggal</span><Input className="min-h-11" type="date" value={filters.sampai} onChange={(event) => setFilter('sampai', event.target.value)} /></label>
      </div>
      <div className="mt-3 flex flex-wrap gap-2">{[
        ['Total pengerjaan', summary.total ?? 0], ['Selesai', summary.selesai ?? 0], ['Berlangsung', summary.berlangsung ?? 0], ['Menunggu nilai', summary.menungguNilai ?? 0], ['Rata-rata nilai', summary.rataRataNilai == null ? '—' : `${Number(summary.rataRataNilai).toFixed(1)}`],
      ].map(([label, value]) => <div key={String(label)} className="min-w-28 flex-1 rounded-xl border bg-background p-3"><p className="text-xs text-muted-foreground">{label}</p><p className="mt-1 text-lg font-bold">{value}</p></div>)}</div>
    </div>
    {data?.terpotong && <p className="border-b bg-amber-50 px-4 py-2 text-sm text-amber-900">Daftar dibatasi 2.000 pengerjaan terbaru. Tambahkan filter tanggal/kelas untuk melihat bagian lain dan mengunduh laporan lengkap.</p>}
    <div className="grid gap-4 p-4 xl:grid-cols-[minmax(0,1.5fr)_minmax(260px,0.8fr)]">
      <section><h3 className="mb-2 font-semibold">Riwayat pengerjaan</h3><div className="overflow-x-auto rounded-xl border"><Table><TableHeader><TableRow><TableHead>Asesmen</TableHead><TableHead>Siswa & kelas</TableHead><TableHead>Status</TableHead><TableHead>Nilai</TableHead><TableHead>Waktu</TableHead></TableRow></TableHeader><TableBody>{records.slice(0, 100).map((row) => <TableRow key={`${row.modul}:${row.id}`}><TableCell className="min-w-40"><div className="font-medium">{row.asesmen}</div><div className="text-xs text-muted-foreground">{row.modul === 'simulasi' ? 'Simulasi Asesmen' : 'Ujian Online'}{row.mapel ? ` · ${row.mapel}` : ''}</div></TableCell><TableCell className="min-w-36"><div>{row.namaSiswa}</div><div className="text-xs text-muted-foreground">{row.kelas || 'Kelas tidak tercatat'}</div></TableCell><TableCell><Badge variant={row.status === 'selesai' ? 'default' : 'secondary'}>{row.status}</Badge></TableCell><TableCell>{row.nilai == null ? '—' : Number(row.nilai).toFixed(1)}</TableCell><TableCell className="min-w-36 text-xs">{row.mulai ? new Date(row.mulai).toLocaleString('id-ID') : '—'}</TableCell></TableRow>)}{!records.length && <EmptyState colSpan={5} label={loading ? 'Memuat data pengerjaan…' : 'Belum ada hasil untuk filter ini.'} />}</TableBody></Table></div>{records.length > 100 && <p className="mt-2 text-xs text-muted-foreground">Menampilkan 100 pengerjaan terbaru dari {records.length} data pada halaman ini.</p>}</section>
      <section><h3 className="mb-2 font-semibold">Perkembangan per siswa</h3><div className="max-h-[32rem] overflow-auto rounded-xl border"><Table><TableHeader><TableRow><TableHead>Siswa</TableHead><TableHead>Pengerjaan</TableHead><TableHead>Rata-rata</TableHead><TableHead>Tren 8 nilai terakhir</TableHead><TableHead>Riwayat terbaru</TableHead></TableRow></TableHeader><TableBody>{[...progress].sort((a, b) => String(a.namaSiswa).localeCompare(String(b.namaSiswa), 'id')).map((row) => { const attempts = scoredAttemptsByStudent.get(String(row.pesertaDidikId)) || []; return <TableRow key={row.pesertaDidikId}><TableCell className="min-w-32"><div className="font-medium">{row.namaSiswa}</div><div className="text-xs text-muted-foreground">{row.kelas}</div></TableCell><TableCell>{row.selesai}/{row.jumlahPengerjaan}</TableCell><TableCell>{row.rataRataNilai == null ? '—' : Number(row.rataRataNilai).toFixed(1)}</TableCell><TableCell><StudentScoreTrend attempts={attempts} /></TableCell><TableCell><details className="min-w-40"><summary className="min-h-10 cursor-pointer py-2 text-sm font-medium text-primary">Lihat {attempts.length} nilai</summary>{attempts.length ? <ol className="mt-2 space-y-2 border-l pl-3">{[...attempts].reverse().map((attempt) => <li key={`${attempt.modul}:${attempt.id}`} className="text-xs"><div className="font-medium">{attempt.asesmen}</div><div className="text-muted-foreground">{attempt.modul === 'simulasi' ? 'Simulasi asesmen' : 'Ujian Online'} · {attempt.mulai ? new Date(attempt.mulai).toLocaleDateString('id-ID') : 'Tanggal tidak tercatat'}</div><div className="font-semibold">Nilai {Number(attempt.nilai).toFixed(1)}</div></li>)}</ol> : <p className="mt-2 text-xs text-muted-foreground">Belum ada nilai yang dirilis.</p>}</details></TableCell></TableRow>})}{!progress.length && <EmptyState colSpan={5} label="Belum ada perkembangan siswa." />}</TableBody></Table></div><p className="mt-2 text-xs text-muted-foreground">Riwayat memuat hingga 8 asesmen bernilai terbaru per siswa. Gunakan filter dan ekspor CSV/XLSX untuk rekam lengkap.</p></section>
    </div>
    <section className="border-t px-4 pb-4"><h3 className="mb-2 font-semibold">Statistik per kelas</h3><div className="overflow-x-auto rounded-xl border"><Table><TableHeader><TableRow><TableHead>Kelas</TableHead><TableHead>Siswa</TableHead><TableHead>Pengerjaan</TableHead><TableHead>Selesai</TableHead><TableHead>Rata-rata nilai</TableHead></TableRow></TableHeader><TableBody>{classProgress.map((row) => <TableRow key={row.kelasId || 'unknown'}><TableCell className="font-medium">{row.kelas}</TableCell><TableCell>{row.jumlahSiswa}</TableCell><TableCell>{row.jumlahPengerjaan}</TableCell><TableCell>{row.selesai}</TableCell><TableCell>{row.rataRataNilai == null ? '—' : Number(row.rataRataNilai).toFixed(1)}</TableCell></TableRow>)}{!classProgress.length && <EmptyState colSpan={5} label="Belum ada statistik kelas." />}</TableBody></Table></div></section>
    <section className="border-t px-4 pb-4"><div className="mb-2"><h3 className="font-semibold">Capaian domain &amp; kompetensi</h3><p className="mt-1 text-xs text-muted-foreground">Ringkasan gabungan berbobot dari Ujian Online dan Simulasi sesuai filter di atas. Hanya butir yang telah dinilai masuk ke persentase capaian.</p></div><div className="overflow-x-auto rounded-xl border"><Table><TableHeader><TableRow><TableHead>Domain</TableHead><TableHead>Kompetensi</TableHead><TableHead>Butir</TableHead><TableHead>Respons</TableHead><TableHead>Benar</TableHead><TableHead>Menunggu</TableHead><TableHead>Capaian berbobot</TableHead></TableRow></TableHeader><TableBody>{domainProgress.map((row) => <TableRow key={`${row.domain}:${row.competency}`}><TableCell className="min-w-36 font-medium">{row.domain}</TableCell><TableCell className="min-w-48">{row.competency}</TableCell><TableCell>{row.items}</TableCell><TableCell>{row.answered}/{row.attempts}</TableCell><TableCell>{row.correct}</TableCell><TableCell>{row.pending}</TableCell><TableCell>{row.weight > 0 ? `${(row.earned / row.weight * 100).toFixed(1)}%` : '—'}</TableCell></TableRow>)}{!domainProgress.length && <EmptyState colSpan={7} label={loading ? 'Memuat capaian kompetensi…' : 'Belum ada data capaian untuk filter ini.'} />}</TableBody></Table></div></section>
    <section className="border-t px-4 pb-4"><div className="mb-2 flex flex-wrap items-start justify-between gap-2"><div><h3 className="font-semibold">Analisis per soal</h3><p className="mt-1 text-xs text-muted-foreground">Tingkat keberhasilan dihitung dari jawaban yang dinilai; uraian menunggu penilaian tidak dimasukkan sebagai salah.</p></div><div className="flex gap-2"><Button size="sm" variant="outline" onClick={() => downloadQuestionStats('csv')}>CSV soal</Button><Button size="sm" variant="outline" onClick={() => downloadQuestionStats('xlsx')}>XLSX soal</Button></div></div><div className="overflow-x-auto rounded-xl border"><Table><TableHeader><TableRow><TableHead>Asesmen / soal</TableHead><TableHead>Modul</TableHead><TableHead>Domain / kompetensi</TableHead><TableHead>Respons</TableHead><TableHead>Benar</TableHead><TableHead>Salah</TableHead><TableHead>Kosong</TableHead><TableHead>Menunggu</TableHead><TableHead>Berhasil</TableHead><TableHead>Skor rata-rata</TableHead></TableRow></TableHeader><TableBody>{questionStats.map((row) => <TableRow key={`${row.modul}:${row.asesmenId}:${row.soalId}`}><TableCell className="min-w-64"><div className="font-medium">{row.asesmen}</div><div className="line-clamp-2 text-xs text-muted-foreground">{row.pertanyaan}</div></TableCell><TableCell>{row.modul === 'simulasi' ? 'Simulasi' : 'Ujian Online'}</TableCell><TableCell className="min-w-40"><div>{row.domain || '—'}</div><div className="text-xs text-muted-foreground">{row.kompetensi || row.topik || 'Belum diberi label'}</div></TableCell><TableCell>{row.terjawab}/{row.percobaan}</TableCell><TableCell>{row.benar}</TableCell><TableCell>{row.salah}</TableCell><TableCell>{row.kosong}</TableCell><TableCell>{row.menungguNilai}</TableCell><TableCell>{row.tingkatKeberhasilan == null ? '—' : `${Number(row.tingkatKeberhasilan).toFixed(1)}%`}</TableCell><TableCell>{row.rataRataSkor == null ? '—' : `${Number(row.rataRataSkor).toFixed(1)}%`}</TableCell></TableRow>)}{!questionStats.length && <EmptyState colSpan={10} label={loading ? 'Memuat analisis soal…' : 'Belum ada data soal untuk filter ini.'} />}</TableBody></Table></div>{questionData?.terpotong && <p className="mt-2 text-xs text-amber-700">Analisis memakai hingga 2.000 pengerjaan terbaru. Persempit filter untuk hasil yang lebih terfokus.</p>}</section>
  </Card>
}

function StudentScoreTrend({ attempts }: { attempts: Row[] }) {
  if (!attempts.length) return <span className="text-xs text-muted-foreground">Belum ada nilai</span>
  const first = Number(attempts[0].nilai)
  const last = Number(attempts[attempts.length - 1].nilai)
  const change = last - first
  const direction = change > 0.05 ? 'naik' : change < -0.05 ? 'turun' : 'stabil'
  return <div className="flex min-w-32 items-center gap-2" role="img" aria-label={`Tren nilai ${direction}, dari ${first.toFixed(1)} menjadi ${last.toFixed(1)} pada ${attempts.length} asesmen terakhir`}>
    <div className="flex h-9 items-end gap-1" aria-hidden="true">{attempts.map((attempt) => { const score = Number(attempt.nilai); return <span key={`${attempt.modul}:${attempt.id}`} title={`${attempt.asesmen}: ${score.toFixed(1)} · ${attempt.mulai ? new Date(attempt.mulai).toLocaleDateString('id-ID') : 'tanggal tidak tercatat'}`} className={`w-2 rounded-t-sm ${score < 60 ? 'bg-amber-400' : 'bg-primary/70'}`} style={{ height: `${Math.max(3, Math.min(36, Math.round(score / 100 * 36)))}px` }} /> })}</div>
    <span className={`whitespace-nowrap text-xs font-medium ${change > 0.05 ? 'text-emerald-700' : change < -0.05 ? 'text-rose-700' : 'text-muted-foreground'}`}>{direction} {change === 0 ? '·' : `${change > 0 ? '+' : ''}${change.toFixed(1)}`}</span>
  </div>
}

function MaterialsPanel({ token, materials, canWrite, onReload }: { token: string; materials: Row[]; canWrite: boolean; onReload: () => void }) {
  const [form, setForm] = useState({ judul: '', jenis: 'text', konten: '', altText: '', mediaUrl: '', jenjang: 'SD/MI', kelasFase: '', topik: '', tags: '' })
  const [imageFile, setImageFile] = useState<File | null>(null)
  const [imagePreview, setImagePreview] = useState('')
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  useEffect(() => {
    if (!imageFile) { setImagePreview(''); return }
    const url = URL.createObjectURL(imageFile); setImagePreview(url)
    return () => URL.revokeObjectURL(url)
  }, [imageFile])
  const update = (key: string, value: string) => setForm((current) => ({ ...current, [key]: value }))
  const save = async (event: FormEvent) => {
    event.preventDefault(); setSaving(true)
    try {
      let payload = { ...form, status: 'draf' }
      if (form.jenis === 'image' && imageFile) {
        const upload = new FormData(); upload.append('file', imageFile); upload.append('altText', form.altText)
        const response = await fetch(`${apiBase}/simulasi/media`, { method: 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: upload })
        const body = await response.json().catch(() => ({})); if (!response.ok) throw new Error(body.error || 'Gambar bahan gagal diunggah.')
        payload = { ...payload, konten: body.path }
      }
      await request('/simulasi/bahan', token, 'POST', payload)
      toast.success('Bahan tersimpan sebagai draf reusable.'); setForm({ judul: '', jenis: 'text', konten: '', altText: '', mediaUrl: '', jenjang: 'SD/MI', kelasFase: '', topik: '', tags: '' }); setImageFile(null); setOpen(false); onReload()
    } catch (error: any) { toast.error(error.message || 'Bahan belum tersimpan.') } finally { setSaving(false) }
  }
  const archive = async (id: string) => {
    try { await request(`/simulasi/bahan/${id}`, token, 'DELETE'); toast.success('Bahan diarsipkan.'); onReload() } catch (error: any) { toast.error(error.message || 'Bahan belum dapat diarsipkan.') }
  }
  return <div className="space-y-4">
    <Card className="border-primary/20 bg-primary/[0.03] p-4"><div className="flex flex-wrap items-center justify-between gap-3"><div><p className="text-xs font-bold uppercase tracking-wide text-primary">Pustaka reusable</p><h2 className="mt-1 text-lg font-bold">Bahan soal</h2><p className="mt-1 max-w-2xl text-sm text-muted-foreground">Simpan bacaan, tabel, gambar, dan tautan HTTPS sekali lalu gunakan di banyak soal.</p></div>{canWrite && <Button onClick={() => setOpen(!open)}><Plus className="h-4 w-4" /> Tambah bahan</Button>}</div></Card>
    {open && canWrite && <FormCard title="Buat bahan reusable" description="Bahan baru selalu dimulai sebagai draf dan dapat dipakai dari kanvas paket."><form className="grid gap-3 md:grid-cols-2" onSubmit={save}><Field label="Judul"><Input value={form.judul} onChange={(event) => update('judul', event.target.value)} placeholder="Contoh: Bacaan tentang energi" required /></Field><Field label="Jenis"><Select value={form.jenis} onChange={(event) => { update('jenis', event.target.value); if (event.target.value !== 'image') setImageFile(null) }}><option value="text">Teks / bacaan</option><option value="table">Tabel</option><option value="image">Gambar</option><option value="media_link">Tautan media HTTPS</option></Select></Field>{form.jenis === 'image' ? <Field label="File gambar PNG/JPG/WEBP" wide><Input type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => setImageFile(event.target.files?.[0] || null)} /><p className="text-xs text-muted-foreground">Maksimal 5 MB. Teks alternatif wajib untuk aksesibilitas.</p>{imagePreview && <img className="mt-3 max-h-48 max-w-full rounded-xl border object-contain" src={imagePreview} alt={form.altText || 'Pratinjau gambar bahan'} />}</Field> : <Field label="Isi bahan" wide><textarea className="min-h-28 w-full rounded-xl border border-input bg-background p-3 text-sm" value={form.konten} onChange={(event) => update('konten', event.target.value)} placeholder={form.jenis === 'media_link' ? 'https://…' : 'Ketik isi bahan di sini.'} required /></Field>}<Field label="Teks alternatif"><Input value={form.altText} onChange={(event) => update('altText', event.target.value)} placeholder="Jelaskan gambar untuk siswa" required={form.jenis === 'image'} /></Field><Field label="Topik"><Input value={form.topik} onChange={(event) => update('topik', event.target.value)} /></Field><Field label="Kelas / fase"><Input value={form.kelasFase} onChange={(event) => update('kelasFase', event.target.value)} /></Field><Field label="Tag"><Input value={form.tags} onChange={(event) => update('tags', event.target.value)} placeholder="literasi, energi" /></Field><div className="flex gap-2 md:col-span-2"><Button disabled={saving || (form.jenis === 'image' && !imageFile)}>{saving ? 'Menyimpan…' : 'Simpan bahan'}</Button><Button type="button" variant="outline" onClick={() => setOpen(false)}>Batal</Button></div></form></FormCard>}
    <Card className="overflow-hidden"><div className="overflow-x-auto"><Table><TableHeader><TableRow><TableHead>Judul</TableHead><TableHead>Jenis</TableHead><TableHead>Topik</TableHead><TableHead>Status</TableHead><TableHead className="text-right">Aksi</TableHead></TableRow></TableHeader><TableBody>{materials.map((material) => <TableRow key={material.id}><TableCell><div className="font-medium">{material.judul}</div><div className="max-w-lg truncate text-xs text-muted-foreground">{material.konten}</div></TableCell><TableCell>{material.jenis === 'media_link' ? 'Media HTTPS' : material.jenis === 'table' ? 'Tabel' : material.jenis === 'image' ? 'Gambar' : 'Teks'}</TableCell><TableCell>{material.topik || '—'}</TableCell><TableCell><Badge variant={material.status === 'terbit' ? 'default' : 'secondary'}>{material.status || 'draf'}</Badge></TableCell><TableCell className="text-right">{canWrite && <Button size="sm" variant="ghost" onClick={() => void archive(material.id)} aria-label={`Arsipkan ${material.judul}`}><Trash2 className="h-4 w-4" /></Button>}</TableCell></TableRow>)}{!materials.length && <EmptyState colSpan={5} label="Belum ada bahan reusable." />}</TableBody></Table></div></Card>
  </div>
}

function ResultsPanel({ packages, results, onSelect }: { packages: Row[]; results: Row[]; onSelect: (row: Row) => Promise<void> }) {
  return <Card className="overflow-hidden"><div className="border-b bg-muted/20 p-4"><h2 className="font-bold">Hasil simulasi</h2><p className="mt-1 text-sm text-muted-foreground">Pilih paket untuk melihat rekap benar, salah, kosong, detail jawaban, dan penilaian uraian.</p></div><div className="overflow-x-auto"><Table><TableHeader><TableRow><TableHead>Paket</TableHead><TableHead>Status</TableHead><TableHead>Waktu</TableHead><TableHead>Ringkasan</TableHead><TableHead /></TableRow></TableHeader><TableBody>{packages.filter((item) => item.status === 'terbit').map((item) => <TableRow key={item.id}><TableCell className="font-medium">{item.nama || 'Paket tanpa judul'}</TableCell><TableCell><Badge>{item.status}</Badge></TableCell><TableCell>{item.waktuMulai ? new Date(item.waktuMulai).toLocaleString('id-ID') : 'Tanpa jadwal'}</TableCell><TableCell>{results.length ? `${results.length} upaya terbaca` : 'Muat hasil paket'}</TableCell><TableCell><Button size="sm" className="min-h-11" variant="outline" onClick={() => void onSelect(item)}>Lihat hasil</Button></TableCell></TableRow>)}{!packages.some((item) => item.status === 'terbit') && <EmptyState colSpan={5} label="Belum ada paket terbit." />}</TableBody></Table></div></Card>
}

/* Legacy JSON editor retired in favor of the visual builder below.
function LegacyQuestionForm({ form, setForm, mapel, image, setImage, saving, onCancel, onSubmit }: any) {
  const update = (key: string, value: any) => setForm((current: any) => ({ ...current, [key]: value }))
  return <FormCard title="Buat soal simulasi" description="Konfigurasi jawaban menggunakan JSON agar semua format asesmen dapat divalidasi server."><form className="grid gap-3 md:grid-cols-2" onSubmit={onSubmit}><Field label="Jenjang"><Input value={form.jenjang} onChange={(event) => update('jenjang', event.target.value)} required /></Field><Field label="Kelas / Fase"><Input value={form.kelasFase} onChange={(event) => update('kelasFase', event.target.value)} /></Field><Field label="Mode"><Select value={form.mode} onChange={(event) => update('mode', event.target.value)}><option value="anbk_akm">Simulasi ANBK/AKM</option><option value="tka_sd">Simulasi TKA SD</option></Select></Field><Field label="Mata pelajaran"><Select value={form.mapelId} onChange={(event) => update('mapelId', event.target.value)}><option value="">Pilih mapel (opsional)</option>{mapel.map((item: Row) => <option key={item.id} value={item.id}>{item.namaMapel}</option>)}</Select></Field><Field label="Domain"><Input value={form.domain} onChange={(event) => update('domain', event.target.value)} placeholder="Literasi membaca / Bilangan" /></Field><Field label="Topik"><Input value={form.topik} onChange={(event) => update('topik', event.target.value)} /></Field><Field label="Kompetensi"><Input value={form.kompetensi} onChange={(event) => update('kompetensi', event.target.value)} /></Field><Field label="Level kognitif"><Input value={form.levelKognitif} onChange={(event) => update('levelKognitif', event.target.value)} placeholder="Memahami / Menerapkan / Bernalar" /></Field><Field label="Tingkat kesulitan"><Select value={form.tingkatKesulitan} onChange={(event) => update('tingkatKesulitan', event.target.value)}><option value="mudah">Mudah</option><option value="sedang">Sedang</option><option value="sulit">Sulit</option></Select></Field><Field label="Tipe soal"><Select value={form.tipe} onChange={(event) => { const tipe = event.target.value as QuestionType; setForm((current: any) => ({ ...current, tipe, konfigurasi: JSON.stringify(configExample(tipe), null, 2) })) }}>{questionTypes.map(([id, label]) => <option key={id} value={id}>{label}</option>)}</Select></Field><Field label="Tag"><Input value={form.tags} onChange={(event) => update('tags', event.target.value)} placeholder="pecahan, informasi tersurat" /></Field><Field label="Bobot"><Input type="number" min="0.1" step="0.1" value={form.bobot} onChange={(event) => update('bobot', event.target.value)} /></Field><Field label="Pertanyaan" wide><textarea className="min-h-28 rounded-xl border border-input bg-background p-3 text-sm" value={form.pertanyaan} onChange={(event) => update('pertanyaan', event.target.value)} required /></Field><Field label="Konfigurasi jawaban (JSON)" wide><textarea className="min-h-52 rounded-xl border border-input bg-muted/30 p-3 font-mono text-xs" value={form.konfigurasi} onChange={(event) => update('konfigurasi', event.target.value)} required /></Field><Field label="Stimulus (JSON array, opsional)" wide><textarea className="min-h-24 rounded-xl border border-input bg-muted/30 p-3 font-mono text-xs" value={form.stimulus} onChange={(event) => update('stimulus', event.target.value)} placeholder={'[{"jenis":"text","konten":"Teks stimulus","urutan":1}]'} /></Field><Field label="Gambar stimulus (PNG/JPG, maks. 5 MB)" wide><input type="file" accept="image/png,image/jpeg" className="rounded-xl border border-input bg-background p-2 text-sm" onChange={(event) => setImage(event.target.files?.[0] || null)} /><p className="text-xs text-muted-foreground">{image ? image.name : 'Opsional; gambar ditautkan secara aman setelah soal tersimpan.'}</p></Field><Field label="Pembahasan internal" wide><textarea className="min-h-20 rounded-xl border border-input bg-background p-3 text-sm" value={form.pembahasan} onChange={(event) => update('pembahasan', event.target.value)} /></Field><Field label="Status"><Select value={form.status} onChange={(event) => update('status', event.target.value)}><option value="draf">Draf</option><option value="terbit">Terbit</option></Select></Field><div className="flex items-end gap-2"><Button disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan soal'}</Button><Button type="button" variant="outline" onClick={onCancel}>Batal</Button></div></form></FormCard>
}
*/
/* eslint-disable no-unused-vars -- kept as a compatibility helper for older integrations. */
export function QuestionForm({ form, setForm, mapel, image, setImage, saving, onCancel, onSubmit }: any) {
  const update = (key: string, value: any) => setForm((current: any) => ({ ...current, [key]: value }))
  const setConfig = (konfigurasi: any) => update('konfigurasi', konfigurasi)
  const chooseType = (id: QuestionType) => { update('tipe', id); setConfig(configExample(id)) }
  const handleTypeDrop = (event: DragEvent<HTMLFormElement>) => {
    const type = event.dataTransfer.getData('application/x-simulasi-question-type') as QuestionType
    if (!questionTypes.some(([id]) => id === type)) return
    event.preventDefault()
    chooseType(type)
  }
  const typeCards: Array<[QuestionType, string, string, any]> = [
    ['pg_tunggal', 'Pilihan ganda', 'Satu jawaban paling tepat', CircleDot], ['pg_kompleks', 'Pilihan kompleks', 'Pilih semua jawaban yang benar', ListChecks],
    ['benar_salah', 'Benar / Salah', 'Nilai setiap pernyataan', CheckCircle2], ['menjodohkan', 'Menjodohkan', 'Pasangkan dua kolom', Link2],
    ['isian_singkat', 'Jawaban singkat', 'Terima satu atau beberapa jawaban', TextCursorInput], ['uraian', 'Paragraf / uraian', 'Dinilai guru dengan rubrik', FileText],
    ['dropdown', 'Dropdown', 'Pilih satu jawaban dari daftar', ListChecks], ['skala_linear', 'Skala linear', 'Pilih nilai dalam sebuah rentang', Grid2X2], ['rating', 'Rating', 'Beri nilai dengan ikon bintang', Star],
    ['kisi_pg', 'Kisi pilihan tunggal', 'Satu pilihan untuk setiap baris', Grid2X2], ['kisi_checkbox', 'Kisi kotak centang', 'Beberapa pilihan per baris', Grid2X2],
    ['tanggal', 'Tanggal', 'Jawab menggunakan tanggal', CalendarDays], ['waktu', 'Waktu / durasi', 'Jawab menggunakan waktu', CalendarDays], ['susun_urutan', 'Susun urutan', 'Urutkan langkah dengan drag atau tombol', ListOrdered], ['unggah_berkas', 'Unggah berkas', 'Siswa mengirim file yang dinilai tutor', FileUp],
  ]
  const cfg = form.konfigurasi || configExample(form.tipe)
  return <FormCard title="Pembuat soal visual" description="Ketik pertanyaan, pilih bentuk jawaban, lalu klik pilihan yang benar. Tidak ada JSON atau syntax yang perlu diisi.">
    <form className="space-y-5" onSubmit={onSubmit} onDragOver={(event) => { if (event.dataTransfer.types.includes('application/x-simulasi-question-type')) event.preventDefault() }} onDrop={handleTypeDrop}>
      <section className="rounded-2xl border bg-primary/[0.03] p-4"><div className="mb-3 flex items-center gap-2"><span className="grid h-7 w-7 place-items-center rounded-full bg-primary text-xs font-bold text-primary-foreground">1</span><div><h3 className="font-semibold">Pilih bentuk pertanyaan</h3><p className="text-xs text-muted-foreground">Klik jenis soal, atau seret kartunya ke area pertanyaan di bawah. Semuanya juga bisa dipilih dengan keyboard.</p></div></div><div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">{typeCards.map(([id, title, description, Icon]) => <button type="button" key={id} draggable onDragStart={(event) => { event.dataTransfer.effectAllowed = 'copy'; event.dataTransfer.setData('application/x-simulasi-question-type', id) }} onClick={() => chooseType(id)} aria-label={`Pilih jenis soal ${title}; dapat diseret ke area pertanyaan`} className={`flex min-h-16 items-start gap-3 rounded-xl border p-3 text-left transition hover:border-primary/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary ${form.tipe === id ? 'border-primary bg-primary/10 ring-1 ring-primary' : 'bg-background'}`}><Icon className="mt-0.5 h-5 w-5 shrink-0 text-primary" /><span><strong className="block text-sm">{title}</strong><span className="text-xs text-muted-foreground">{description}</span></span></button>)}</div></section>
      <section data-question-canvas className="grid gap-4 rounded-2xl transition-colors lg:grid-cols-[minmax(0,1fr)_310px]"><div className="space-y-4 rounded-2xl border bg-card p-4"><div className="flex items-center gap-2"><span className="grid h-7 w-7 place-items-center rounded-full bg-primary text-xs font-bold text-primary-foreground">2</span><h3 className="font-semibold">Tulis pertanyaan dan jawaban</h3></div><Field label="Pertanyaan" wide><textarea className="min-h-28 rounded-xl border border-input bg-background p-3 text-base leading-relaxed" value={form.pertanyaan} onChange={(event) => update('pertanyaan', event.target.value)} placeholder="Contoh: Manakah informasi utama pada bacaan di atas?" required /></Field><AnswerBuilder tipe={form.tipe} config={cfg} setConfig={setConfig} /><StimulusBuilder items={form.stimulus || []} setItems={(stimulus: Row[]) => update('stimulus', stimulus)} image={image} setImage={setImage} /></div><QuestionPreview form={form} config={cfg} /></section>
      <section className="rounded-2xl border bg-card p-4"><div className="mb-3 flex items-center gap-2"><span className="grid h-7 w-7 place-items-center rounded-full bg-primary text-xs font-bold text-primary-foreground">3</span><h3 className="font-semibold">Atur nilai dan informasi guru</h3></div><div className="grid gap-3 md:grid-cols-4"><Field label="Skor soal"><Input type="number" min="0.1" step="0.1" value={form.bobot} onChange={(event) => update('bobot', event.target.value)} required /></Field><Field label="Status"><Select value={form.status} onChange={(event) => update('status', event.target.value)}><option value="draf">Simpan sebagai draf</option><option value="terbit">Terbitkan ke bank soal</option></Select></Field><Field label="Tingkat kesulitan"><Select value={form.tingkatKesulitan} onChange={(event) => update('tingkatKesulitan', event.target.value)}><option value="mudah">Mudah</option><option value="sedang">Sedang</option><option value="sulit">Sulit</option></Select></Field><label className="flex min-h-11 items-center gap-2 rounded-xl border px-3 text-sm"><Checkbox checked={form.wajibDijawab !== false} onChange={(event) => update('wajibDijawab', event.currentTarget.checked)} />Wajib dijawab</label></div><details className="mt-4 rounded-xl bg-muted/50 p-3"><summary className="cursor-pointer text-sm font-medium">Metadata dan pembahasan guru (opsional)</summary><div className="mt-3 grid gap-3 md:grid-cols-2"><Field label="Mode"><Select value={form.mode} onChange={(event) => update('mode', event.target.value)}><option value="anbk_akm">Simulasi ANBK/AKM</option><option value="tka_sd">Simulasi TKA SD</option></Select></Field><Field label="Mata pelajaran"><Select value={form.mapelId} onChange={(event) => update('mapelId', event.target.value)}><option value="">Tidak dipilih</option>{mapel.map((item: Row) => <option key={item.id} value={item.id}>{item.namaMapel}</option>)}</Select></Field><Field label="Jenjang"><Input value={form.jenjang} onChange={(event) => update('jenjang', event.target.value)} /></Field><Field label="Kelas / fase"><Input value={form.kelasFase} onChange={(event) => update('kelasFase', event.target.value)} /></Field><Field label="Domain / topik"><Input value={form.domain} onChange={(event) => update('domain', event.target.value)} placeholder="Contoh: Literasi membaca" /></Field><Field label="Kompetensi"><Input value={form.kompetensi} onChange={(event) => update('kompetensi', event.target.value)} /></Field><Field label="Level kognitif"><Input value={form.levelKognitif} onChange={(event) => update('levelKognitif', event.target.value)} placeholder="Memahami, menerapkan, atau bernalar" /></Field><Field label="Tag"><Input value={form.tags} onChange={(event) => update('tags', event.target.value)} placeholder="pecahan, informasi tersurat" /></Field><Field label="Pembahasan internal" wide><textarea className="min-h-20 rounded-xl border border-input bg-background p-3 text-sm" value={form.pembahasan} onChange={(event) => update('pembahasan', event.target.value)} placeholder="Catatan atau penjelasan untuk guru." /></Field></div></details></section>
      <div className="flex flex-wrap justify-between gap-2 border-t pt-4"><p className="text-sm text-muted-foreground">Kunci jawaban dan pembahasan internal tidak pernah dikirim ke siswa.</p><div className="flex gap-2"><Button type="button" variant="outline" onClick={onCancel}>Batal</Button><Button disabled={saving}>{saving ? 'Menyimpan…' : form.status === 'terbit' ? 'Simpan & terbitkan' : 'Simpan draf soal'}</Button></div></div>
    </form>
  </FormCard>
}
/* eslint-enable no-unused-vars */

const newID = (prefix: string) => `${prefix}-${globalThis.crypto?.randomUUID?.() || Math.random().toString(36).slice(2)}`
const reordered = <T,>(values: T[], from: number, to: number) => { const copy = [...values]; const [moved] = copy.splice(from, 1); copy.splice(to, 0, moved); return copy }
function reorderDragType(scope: string) { return `application/x-pkbm-reorder-${scope}` }
function startReorderDrag(event: DragEvent<HTMLElement>, index: number, setDragged: (index: number | null) => void, scope: string) {
  setDragged(index)
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData(reorderDragType(scope), String(index))
}
function sourceReorderIndex(event: DragEvent<HTMLElement>, fallback: number | null, scope: string) {
  const value = event.dataTransfer.getData(reorderDragType(scope))
  const index = Number(value)
  return value !== '' && Number.isInteger(index) ? index : fallback
}
function DragHandle() { return <GripVertical className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden /> }
function ReorderButtons({ index, length, onMove, label }: { index: number; length: number; onMove: (to: number) => void; label: string }) {
  return <span className="flex shrink-0 items-center gap-0.5" aria-label={`Urutkan ${label}`}>
    <Button type="button" size="icon" variant="ghost" className="h-11 w-11" disabled={index === 0} onClick={() => onMove(index - 1)} aria-label={`Pindahkan ${label} ke atas`} title="Pindahkan ke atas"><ArrowUp className="h-4 w-4" /></Button>
    <Button type="button" size="icon" variant="ghost" className="h-11 w-11" disabled={index === length - 1} onClick={() => onMove(index + 1)} aria-label={`Pindahkan ${label} ke bawah`} title="Pindahkan ke bawah"><ArrowDown className="h-4 w-4" /></Button>
  </span>
}
export function AnswerBuilder({ tipe, config, setConfig, groupName = 'visual-correct-choice', token, onUploadChoiceImage }: { tipe: QuestionType; config: any; setConfig: (config: any) => void; groupName?: string; token?: string; onUploadChoiceImage?: (choiceID: string, file: File, altText: string) => Promise<Row | null> }) {
  const partialScoringTypes: QuestionType[] = ['pg_kompleks', 'benar_salah', 'menjodohkan', 'kisi_pg', 'kisi_checkbox', 'susun_urutan']
  let editor: ReactNode
  if (tipe === 'pg_tunggal' || tipe === 'pg_kompleks' || tipe === 'dropdown') editor = <ChoiceBuilder config={config} multiple={tipe === 'pg_kompleks'} setConfig={setConfig} groupName={groupName} token={token} onUploadImage={onUploadChoiceImage} />
  else if (tipe === 'benar_salah') editor = <TrueFalseBuilder config={config} setConfig={setConfig} />
  else if (tipe === 'menjodohkan') editor = <MatchBuilder config={config} setConfig={setConfig} />
  else if (tipe === 'isian_singkat') editor = <ShortAnswerBuilder config={config} setConfig={setConfig} />
  else if (tipe === 'tanggal' || tipe === 'waktu') editor = <ShortAnswerBuilder config={config} setConfig={setConfig} inputType={tipe === 'tanggal' ? 'date' : 'time'} />
  else if (tipe === 'skala_linear' || tipe === 'rating') editor = <ScaleBuilder config={config} setConfig={setConfig} rating={tipe === 'rating'} groupName={groupName} />
  else if (tipe === 'kisi_pg' || tipe === 'kisi_checkbox') editor = <GridBuilder config={config} setConfig={setConfig} multiple={tipe === 'kisi_checkbox'} />
  else if (tipe === 'susun_urutan') editor = <OrderBuilder config={config} setConfig={setConfig} />
  else if (tipe === 'unggah_berkas') editor = <FileUploadBuilder config={config} setConfig={setConfig} />
  else editor = <RubricBuilder config={config} setConfig={setConfig} />
  const partialEnabled = config.partialScoring === 'proportional'
  return <div className="space-y-3">{editor}{partialScoringTypes.includes(tipe) && <div className="rounded-xl border bg-muted/20 p-3"><Label htmlFor={`scoring-${groupName}`}>Aturan skor</Label><Select id={`scoring-${groupName}`} value={partialEnabled ? 'proportional' : 'exact'} onChange={(event) => setConfig({ ...config, partialScoring: event.target.value })}><option value="exact">Nilai penuh hanya jika semua jawaban tepat</option><option value="proportional">Berikan skor proporsional untuk bagian yang benar</option></Select><p className="mt-1 text-xs text-muted-foreground">Skor proporsional mengurangi nilai jika pilihan salah ditambahkan. Siswa tidak melihat kunci jawaban.</p></div>}</div>
}
function ChoiceBuilder({ config, multiple, setConfig, groupName, token, onUploadImage }: { config: any; multiple: boolean; setConfig: (config: any) => void; groupName: string; token?: string; onUploadImage?: (choiceID: string, file: File, altText: string) => Promise<Row | null> }) {
  const choices: Row[] = config.choices || []
  const correct: string[] = config.correctIds || []
  const [dragged, setDragged] = useState<number | null>(null)
  const [altText, setAltText] = useState<Record<string, string>>({})
  const [uploading, setUploading] = useState<string | null>(null)
  const [pending, setPending] = useState<Record<string, string>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const update = (next: Row[], ids = correct) => setConfig({ ...config, choices: next, correctIds: ids })
  const toggle = (id: string) => update(choices, multiple ? (correct.includes(id) ? correct.filter((item: string) => item !== id) : [...correct, id]) : [id])
  const uploadImage = async (choice: Row, file?: File) => {
    if (!file) return
    const text = String(altText[choice.id] || '').trim()
    if (!text) { setErrors((value) => ({ ...value, [choice.id]: 'Tulis teks alternatif agar gambar dapat dipahami pembaca layar.' })); return }
    if (!onUploadImage) { setErrors((value) => ({ ...value, [choice.id]: 'Unggah gambar tersedia setelah soal memiliki draf tersimpan.' })); return }
    setErrors((value) => ({ ...value, [choice.id]: '' }))
    setUploading(choice.id)
    try {
      const uploaded = await onUploadImage(choice.id, file, text)
      if (uploaded?.id) {
        update(choices.map((item) => item.id === choice.id ? { ...item, imageId: uploaded.id, imageAltText: text } : item))
        setPending((value) => ({ ...value, [choice.id]: '' }))
      } else {
        setPending((value) => ({ ...value, [choice.id]: file.name }))
      }
    } catch (error) {
      setErrors((value) => ({ ...value, [choice.id]: error instanceof Error ? error.message : 'Gambar belum dapat diunggah. Coba lagi.' }))
    } finally { setUploading(null) }
  }
  const addChoice = () => update([...choices, { id: newID('opsi'), text: '' }])
  return <div className="space-y-2">
    <div className="flex items-center justify-between"><div><h4 className="font-medium">Pilihan jawaban</h4><p className="text-xs text-muted-foreground">{multiple ? 'Centang semua pilihan yang menjadi kunci.' : 'Pilih satu lingkaran sebagai kunci jawaban.'} Seret pegangan atau gunakan tombol panah untuk mengubah urutan.</p></div><Button type="button" size="sm" variant="outline" onClick={addChoice}><Plus className="h-3.5 w-3.5" /> Tambah</Button></div>
    {choices.map((choice, index) => <div key={choice.id} draggable onDragStart={(event) => startReorderDrag(event, index, setDragged, 'answer-choice')} onDragEnd={() => setDragged(null)} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); const source = sourceReorderIndex(event, dragged, 'answer-choice'); if (source !== null && source !== index) update(reordered(choices, source, index)); setDragged(null) }} className="rounded-xl border bg-background p-2">
      <div className="flex items-center gap-2"><span className="cursor-grab" title="Seret untuk mengurutkan"><DragHandle /></span><ReorderButtons index={index} length={choices.length} label={`pilihan ${index + 1}`} onMove={(to) => update(reordered(choices, index, to))} /><input aria-label={`Kunci pilihan ${index + 1}`} type={multiple ? 'checkbox' : 'radio'} name={groupName} checked={correct.includes(choice.id)} onChange={() => toggle(choice.id)} /><Input className="h-9 min-w-0 flex-1" value={choice.text} onChange={(event) => update(choices.map((item) => item.id === choice.id ? { ...item, text: event.target.value } : item))} placeholder={`Pilihan ${index + 1}`} /><Button type="button" size="icon" variant="ghost" disabled={choices.length <= 2} onClick={() => update(choices.filter((item) => item.id !== choice.id), correct.filter((id) => id !== choice.id))} aria-label={`Hapus pilihan ${index + 1}`}><Trash2 className="h-4 w-4" /></Button></div>
      {onUploadImage && <div className="mt-2 grid gap-2 rounded-lg bg-muted/30 p-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end"><Field label={`Teks alternatif gambar pilihan ${index + 1}`}><Input value={altText[choice.id] ?? choice.imageAltText ?? ''} onChange={(event) => setAltText((value) => ({ ...value, [choice.id]: event.target.value }))} placeholder="Jelaskan isi gambar" /></Field><label className={`inline-flex min-h-11 cursor-pointer items-center justify-center gap-2 rounded-lg border px-3 text-sm font-medium hover:bg-background ${uploading === choice.id ? 'pointer-events-none opacity-60' : ''}`}><ImageIcon className="h-4 w-4" />{uploading === choice.id ? 'Mengunggah…' : choice.imageId ? 'Ganti gambar' : 'Tambah gambar'}<input type="file" accept="image/png,image/jpeg" className="sr-only" disabled={uploading === choice.id} onChange={(event) => { void uploadImage(choice, event.currentTarget.files?.[0]); event.currentTarget.value = '' }} /></label>{pending[choice.id] && <p className="text-xs text-amber-700 sm:col-span-2">{pending[choice.id]} akan diunggah saat soal disimpan.</p>}{errors[choice.id] && <p role="alert" className="text-xs text-destructive sm:col-span-2">{errors[choice.id]}</p>}{choice.imageId && token && <div className="max-w-xs sm:col-span-2"><StimulusImage id={String(choice.imageId)} alt={String(choice.imageAltText || altText[choice.id] || choice.text || `Gambar pilihan ${index + 1}`)} token={token} /></div>}</div>}
    </div>)}
  </div>
}
function TrueFalseBuilder({ config, setConfig }: { config: any; setConfig: (config: any) => void }) { const rows = config.statements || []; const [dragged, setDragged] = useState<number | null>(null); const update = (statements: Row[]) => setConfig({ ...config, statements }); return <div className="space-y-2"><div className="flex items-center justify-between"><div><h4 className="font-medium">Pernyataan benar / salah</h4><p className="text-xs text-muted-foreground">Tulis setiap pernyataan lalu pilih jawaban yang benar.</p></div><Button type="button" size="sm" variant="outline" onClick={() => update([...rows, { id: newID('pernyataan'), text: '', correct: true }])}><Plus className="h-3.5 w-3.5" /> Tambah</Button></div>{rows.map((row: Row, index: number) => <div key={row.id} draggable onDragStart={(event) => startReorderDrag(event, index, setDragged, 'true-false')} onDragEnd={() => setDragged(null)} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); const source = sourceReorderIndex(event, dragged, 'true-false'); if (source !== null && source !== index) update(reordered(rows, source, index)); setDragged(null) }} className="grid gap-2 rounded-xl border bg-background p-2 sm:grid-cols-[auto_auto_1fr_130px_auto]"><span className="cursor-grab self-center"><DragHandle /></span><ReorderButtons index={index} length={rows.length} label={`pernyataan ${index + 1}`} onMove={(to) => update(reordered(rows, index, to))} /><Input value={row.text} onChange={(event) => update(rows.map((item: Row) => item.id === row.id ? { ...item, text: event.target.value } : item))} placeholder={`Pernyataan ${index + 1}`} /><Select aria-label={`Kunci pernyataan ${index + 1}`} value={String(row.correct)} onChange={(event) => update(rows.map((item: Row) => item.id === row.id ? { ...item, correct: event.target.value === 'true' } : item))}><option value="true">Kunci: Benar</option><option value="false">Kunci: Salah</option></Select><Button type="button" size="icon" variant="ghost" disabled={rows.length <= 1} onClick={() => update(rows.filter((item: Row) => item.id !== row.id))} aria-label={`Hapus pernyataan ${index + 1}`}><Trash2 className="h-4 w-4" /></Button></div>)}</div> }
function MatchBuilder({ config, setConfig }: { config: any; setConfig: (config: any) => void }) { const left = config.left || [], right = config.right || [], pairs = config.pairs || {}; const setLeft = (items: Row[]) => setConfig({ ...config, left: items, pairs: Object.fromEntries(items.map((item: Row) => [item.id, pairs[item.id] || right[0]?.id || ''])) }); const setRight = (items: Row[]) => setConfig({ ...config, right: items }); return <div className="space-y-3"><div><h4 className="font-medium">Pasangkan jawaban</h4><p className="text-xs text-muted-foreground">Isi dua kolom, lalu pilih pasangan benar pada setiap baris kiri.</p></div><div className="grid gap-3 md:grid-cols-2"><EditableList title="Kolom kiri" items={left} placeholder="Contoh: 1/2" onChange={setLeft} onAdd={() => setLeft([...left, { id: newID('kiri'), text: '' }])} /><EditableList title="Kolom kanan" items={right} placeholder="Contoh: 0,5" onChange={setRight} onAdd={() => setRight([...right, { id: newID('kanan'), text: '' }])} /></div><div className="space-y-2 rounded-xl bg-muted/50 p-3">{left.map((item: Row) => <label key={item.id} className="grid gap-2 text-sm sm:grid-cols-2 sm:items-center"><span className="font-medium">{item.text || 'Isian kolom kiri'}</span><Select value={pairs[item.id] || ''} onChange={(event) => setConfig({ ...config, pairs: { ...pairs, [item.id]: event.target.value } })}><option value="">Pilih pasangan benar</option>{right.map((choice: Row) => <option key={choice.id} value={choice.id}>{choice.text || 'Isian kolom kanan'}</option>)}</Select></label>)}</div></div> }
function EditableList({ title, items, placeholder, onChange, onAdd }: any) { const [dragged, setDragged] = useState<number | null>(null); const dragScope = `editable-${title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}`; return <div className="space-y-2 rounded-xl border p-3"><div className="flex items-center justify-between"><strong className="text-sm">{title}</strong><Button type="button" size="sm" variant="ghost" onClick={onAdd}><Plus className="h-3.5 w-3.5" /> Tambah</Button></div>{items.map((item: Row, index: number) => <div key={item.id} draggable onDragStart={(event) => startReorderDrag(event, index, setDragged, dragScope)} onDragEnd={() => setDragged(null)} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); const source = sourceReorderIndex(event, dragged, dragScope); if (source !== null && source !== index) onChange(reordered(items, source, index)); setDragged(null) }} className="flex items-center gap-1"><span className="cursor-grab"><DragHandle /></span><ReorderButtons index={index} length={items.length} label={`${title.toLowerCase()} ${index + 1}`} onMove={(to) => onChange(reordered(items, index, to))} /><Input className="h-9 flex-1" value={item.text} onChange={(event) => onChange(items.map((row: Row) => row.id === item.id ? { ...row, text: event.target.value } : row))} placeholder={placeholder} /><Button type="button" size="icon" variant="ghost" disabled={items.length <= 2} onClick={() => onChange(items.filter((row: Row) => row.id !== item.id))} aria-label={`Hapus ${title.toLowerCase()} ${index + 1}`}><Trash2 className="h-4 w-4" /></Button></div>)}</div> }
function ShortAnswerBuilder({ config, setConfig, inputType = 'text' }: { config: any; setConfig: (config: any) => void; inputType?: string }) {
  const answers = config.acceptedAnswers || []
  const update = (acceptedAnswers: string[]) => setConfig({ ...config, acceptedAnswers })
  return <div className="space-y-3">
    <div className="space-y-2">
      <div className="flex items-center justify-between"><div><h4 className="font-medium">Jawaban yang diterima</h4><p className="text-xs text-muted-foreground">{inputType === 'date' ? 'Masukkan tanggal jawaban dengan format kalender.' : inputType === 'time' ? 'Masukkan waktu jawaban dengan format jam dan menit.' : 'Masukkan variasi jawaban yang tetap dinilai benar, misalnya “empat” dan “4”.'}</p></div><Button type="button" size="sm" variant="outline" onClick={() => update([...answers, ''])}><Plus className="h-3.5 w-3.5" /> Tambah</Button></div>
      {answers.map((answer: string, index: number) => <div key={`${index}-${answer}`} className="flex gap-2"><Input type={inputType} value={answer} onChange={(event) => update(answers.map((item: string, itemIndex: number) => itemIndex === index ? event.target.value : item))} aria-label={`Jawaban diterima ${index + 1}`} /><Button type="button" size="icon" variant="ghost" disabled={answers.length <= 1} onClick={() => update(answers.filter((_: string, itemIndex: number) => itemIndex !== index))}><Trash2 className="h-4 w-4" /></Button></div>)}
    </div>
    {inputType === 'text' && <TextResponseRulesBuilder config={config} setConfig={setConfig} />}
  </div>
}
function ScaleBuilder({ config, setConfig, rating, groupName }: { config: any; setConfig: (config: any) => void; rating: boolean; groupName: string }) {
  const min = rating ? 1 : Number(config.scaleMin ?? 1); const max = rating ? Number(config.ratingMax ?? 5) : Number(config.scaleMax ?? 5); const correct = Number(config.correctNumber ?? min)
  const update = (key: string, value: any) => setConfig({ ...config, [key]: value })
  return <div className="space-y-3 rounded-xl border p-3"><div><h4 className="font-medium">{rating ? 'Pengaturan rating' : 'Rentang skala'}</h4><p className="text-xs text-muted-foreground">Tentukan rentang dan nilai yang dianggap benar. Siswa hanya menerima rentang, bukan kuncinya.</p></div><div className="grid gap-3 sm:grid-cols-2">{rating ? <Field label="Jumlah bintang"><Select value={String(max)} onChange={(event) => update('ratingMax', Number(event.target.value))}>{[3, 4, 5, 6, 7, 8, 9, 10].map((value) => <option key={value} value={value}>{value} bintang</option>)}</Select></Field> : <><Field label="Nilai minimum"><Input type="number" min="0" max={max - 1} value={min} onChange={(event) => update('scaleMin', Number(event.target.value))} /></Field><Field label="Nilai maksimum"><Input type="number" min={min + 1} max={min + 10} value={max} onChange={(event) => update('scaleMax', Number(event.target.value))} /></Field><Field label="Label minimum"><Input value={config.scaleMinLabel || ''} onChange={(event) => update('scaleMinLabel', event.target.value)} placeholder="Contoh: Belum paham" /></Field><Field label="Label maksimum"><Input value={config.scaleMaxLabel || ''} onChange={(event) => update('scaleMaxLabel', event.target.value)} placeholder="Contoh: Sangat paham" /></Field></>}</div><Field label="Pilih nilai kunci"><div className="flex flex-wrap gap-2">{Array.from({ length: Math.min(max - min + 1, 11) }, (_, index) => min + index).map((value) => <label key={value} className="flex min-h-11 cursor-pointer items-center gap-2 rounded-lg border px-3"><input type="radio" name={groupName} checked={correct === value} onChange={() => update('correctNumber', value)} />{rating ? '★'.repeat(value) : value}</label>)}</div></Field></div>
}
function GridBuilder({ config, setConfig, multiple }: { config: any; setConfig: (config: any) => void; multiple: boolean }) {
  const rows = config.rows || []; const columns = config.columns || []; const update = (patch: any) => setConfig({ ...config, ...patch })
  const addRow = () => { const id = newID('baris'); update({ rows: [...rows, { id, text: '' }], gridCorrect: { ...config.gridCorrect, [id]: columns[0]?.id || '' }, gridMultiCorrect: { ...config.gridMultiCorrect, [id]: columns[0] ? [columns[0].id] : [] } }) }
  const addColumn = () => { const id = newID('kolom'); update({ columns: [...columns, { id, text: '' }] }) }
  const setRows = (next: Row[]) => {
    const ids = new Set(next.map((row) => row.id)); const firstColumn = columns[0]?.id || ''
    update({ rows: next, gridCorrect: Object.fromEntries(next.map((row) => [row.id, ids.has(row.id) && columns.some((column: Row) => column.id === config.gridCorrect?.[row.id]) ? config.gridCorrect[row.id] : firstColumn])), gridMultiCorrect: Object.fromEntries(next.map((row) => [row.id, (config.gridMultiCorrect?.[row.id] || []).filter((id: string) => columns.some((column: Row) => column.id === id)).length ? config.gridMultiCorrect[row.id].filter((id: string) => columns.some((column: Row) => column.id === id)) : firstColumn ? [firstColumn] : []])) })
  }
  const setColumns = (next: Row[]) => {
    const ids = new Set(next.map((column) => column.id)); const firstColumn = next[0]?.id || ''
    update({ columns: next, gridCorrect: Object.fromEntries(rows.map((row: Row) => [row.id, ids.has(config.gridCorrect?.[row.id]) ? config.gridCorrect[row.id] : firstColumn])), gridMultiCorrect: Object.fromEntries(rows.map((row: Row) => { const kept = (config.gridMultiCorrect?.[row.id] || []).filter((id: string) => ids.has(id)); return [row.id, kept.length ? kept : firstColumn ? [firstColumn] : []] })) })
  }
  const toggle = (rowID: string, columnID: string) => {
    if (!multiple) update({ gridCorrect: { ...config.gridCorrect, [rowID]: columnID } })
    else { const values = config.gridMultiCorrect?.[rowID] || []; update({ gridMultiCorrect: { ...config.gridMultiCorrect, [rowID]: values.includes(columnID) ? values.filter((id: string) => id !== columnID) : [...values, columnID] } }) }
  }
  return <div className="space-y-3"><div><h4 className="font-medium">Kisi {multiple ? 'kotak centang' : 'pilihan tunggal'}</h4><p className="text-xs text-muted-foreground">Isi label baris dan kolom, lalu tentukan kunci pada setiap baris.</p></div><div className="grid gap-3 md:grid-cols-2"><EditableList title="Baris" items={rows} placeholder="Contoh: Pernyataan pertama" onChange={setRows} onAdd={addRow} /><EditableList title="Kolom jawaban" items={columns} placeholder="Contoh: Benar" onChange={setColumns} onAdd={addColumn} /></div><div className="overflow-x-auto rounded-xl border"><table className="min-w-full text-sm"><thead><tr className="bg-muted/50"><th className="p-2 text-left">Baris</th>{columns.map((column: Row) => <th key={column.id} className="min-w-24 p-2">{column.text || 'Kolom'}</th>)}</tr></thead><tbody>{rows.map((row: Row) => <tr key={row.id} className="border-t"><th className="p-2 text-left font-medium">{row.text || 'Baris'}</th>{columns.map((column: Row) => <td key={column.id} className="p-2 text-center"><input type={multiple ? 'checkbox' : 'radio'} name={`grid-${row.id}`} aria-label={`Kunci ${row.text} — ${column.text}`} checked={multiple ? Boolean(config.gridMultiCorrect?.[row.id]?.includes(column.id)) : config.gridCorrect?.[row.id] === column.id} onChange={() => toggle(row.id, column.id)} /></td>)}</tr>)}</tbody></table></div></div>
}
function OrderBuilder({ config, setConfig }: { config: any; setConfig: (config: any) => void }) {
  const choices = config.choices || []; const [dragged, setDragged] = useState<number | null>(null)
  const update = (next: Row[]) => setConfig({ ...config, choices: next, correctOrder: next.map((item) => item.id) })
  return <div className="space-y-2"><div className="flex items-center justify-between"><div><h4 className="font-medium">Urutan yang benar</h4><p className="text-xs text-muted-foreground">Tulis langkah, lalu susun dari urutan pertama ke terakhir. Gunakan seret atau tombol panah.</p></div><Button type="button" size="sm" variant="outline" onClick={() => update([...choices, { id: newID('langkah'), text: '' }])}><Plus className="h-3.5 w-3.5" /> Tambah langkah</Button></div>{choices.map((choice: Row, index: number) => <div key={choice.id} draggable onDragStart={(event) => startReorderDrag(event, index, setDragged, 'answer-order')} onDragEnd={() => setDragged(null)} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); const source = sourceReorderIndex(event, dragged, 'answer-order'); if (source !== null && source !== index) update(reordered(choices, source, index)); setDragged(null) }} className="flex items-center gap-2 rounded-xl border p-2"><span className="cursor-grab"><DragHandle /></span><ReorderButtons index={index} length={choices.length} label={`langkah ${index + 1}`} onMove={(to) => update(reordered(choices, index, to))} /><span className="w-6 text-center text-xs font-bold text-muted-foreground">{index + 1}</span><Input value={choice.text} onChange={(event) => update(choices.map((item: Row) => item.id === choice.id ? { ...item, text: event.target.value } : item))} placeholder={`Langkah ${index + 1}`} /><Button type="button" size="icon" variant="ghost" disabled={choices.length <= 2} onClick={() => update(choices.filter((item: Row) => item.id !== choice.id))} aria-label={`Hapus langkah ${index + 1}`}><Trash2 className="h-4 w-4" /></Button></div>)}</div>
}
function FileUploadBuilder({ config, setConfig }: { config: any; setConfig: (config: any) => void }) {
  const types: Array<[string, string, string[]]> = [['PDF', 'PDF', ['pdf']], ['Word', 'Word', ['docx']], ['Excel', 'Excel', ['xlsx']], ['PNG', 'PNG', ['png']], ['JPG', 'JPG / JPEG', ['jpg', 'jpeg']]]
  const allowed: string[] = config.allowedFileTypes || []
  const toggle = (extensions: string[], checked: boolean) => setConfig({ ...config, allowedFileTypes: checked ? [...new Set([...allowed, ...extensions])] : allowed.filter((item) => !extensions.includes(item)) })
  return <div className="space-y-3 rounded-xl border p-3"><div><h4 className="font-medium">Batas unggahan siswa</h4><p className="text-xs text-muted-foreground">Berkas disimpan privat dan hanya bisa dilihat siswa pemilik serta staf yang berwenang. Semua unggahan dinilai tutor.</p></div><div className="grid gap-3 sm:grid-cols-2"><Field label="Maks. jumlah file"><Input type="number" min="1" max="10" value={config.maxFiles ?? 3} onChange={(event) => setConfig({ ...config, maxFiles: Number(event.target.value) })} /></Field><Field label="Maks. ukuran per file (MB)"><Input type="number" min="1" max="25" value={config.maxFileSizeMB ?? 10} onChange={(event) => setConfig({ ...config, maxFileSizeMB: Number(event.target.value) })} /></Field></div><fieldset><legend className="mb-2 text-sm font-medium">Jenis file yang diterima</legend><div className="flex flex-wrap gap-2">{types.map(([id, label, extensions]) => <label key={id} className="flex min-h-11 items-center gap-2 rounded-lg border px-3 text-sm"><Checkbox checked={extensions.some((extension) => allowed.includes(extension))} onChange={(event) => toggle(extensions, event.currentTarget.checked)} />{label}</label>)}</div></fieldset></div>
}
function TextResponseRulesBuilder({ config, setConfig }: { config: any; setConfig: (config: any) => void }) {
  const update = (key: string, value: string) => setConfig({ ...config, [key]: value === '' ? undefined : key === 'validationMessage' ? value : Number(value) })
  return <fieldset className="space-y-3 rounded-xl border bg-muted/20 p-3">
    <legend className="px-1 text-sm font-medium">Validasi panjang jawaban (opsional)</legend>
    <p className="text-xs text-muted-foreground">Batas ini diperiksa saat siswa mengirim. Autosave tetap menerima tulisan sementara yang belum lengkap.</p>
    <div className="grid gap-3 sm:grid-cols-2">
      <Field label="Minimal karakter"><Input type="number" min="0" max="10000" value={config.textMinLength ?? ''} onChange={(event) => update('textMinLength', event.target.value)} placeholder="Tanpa batas minimum" /></Field>
      <Field label="Maksimal karakter"><Input type="number" min="0" max="10000" value={config.textMaxLength ?? ''} onChange={(event) => update('textMaxLength', event.target.value)} placeholder="Tanpa batas maksimum" /></Field>
    </div>
    <Field label="Pesan bila belum sesuai"><Input maxLength={200} value={config.validationMessage || ''} onChange={(event) => update('validationMessage', event.target.value)} placeholder="Contoh: Jelaskan jawaban dengan minimal 2 kalimat." /></Field>
  </fieldset>
}

function RubricBuilder({ config, setConfig }: { config: any; setConfig: (config: any) => void }) {
  const rubric = config.rubrik || []
  const update = (rubrik: Row[]) => setConfig({ ...config, rubrik })
  return <div className="space-y-3">
    <div className="space-y-2"><div className="flex items-center justify-between"><div><h4 className="font-medium">Rubrik penilaian uraian</h4><p className="text-xs text-muted-foreground">Siswa akan menunggu penilaian guru setelah mengirim jawaban.</p></div><Button type="button" size="sm" variant="outline" onClick={() => update([...rubric, { kriteria: '', maks: 1 }])}><Plus className="h-3.5 w-3.5" /> Tambah kriteria</Button></div>{rubric.map((row: Row, index: number) => <div key={index} className="grid gap-2 sm:grid-cols-[1fr_110px_auto]"><Input value={row.kriteria} onChange={(event) => update(rubric.map((item: Row, itemIndex: number) => itemIndex === index ? { ...item, kriteria: event.target.value } : item))} placeholder="Contoh: Ketepatan alasan" /><Input type="number" min="0.1" step="0.1" value={row.maks} onChange={(event) => update(rubric.map((item: Row, itemIndex: number) => itemIndex === index ? { ...item, maks: Number(event.target.value) } : item))} /><Button type="button" size="icon" variant="ghost" disabled={rubric.length <= 1} onClick={() => update(rubric.filter((_: Row, itemIndex: number) => itemIndex !== index))}><Trash2 className="h-4 w-4" /></Button></div>)}</div>
    <TextResponseRulesBuilder config={config} setConfig={setConfig} />
  </div>
}
function StimulusBuilder({ items, setItems, image, setImage }: any) { const [kind, setKind] = useState('text'); const [dragged, setDragged] = useState<number | null>(null); const add = () => setItems([...items, { id: newID('stimulus'), jenis: kind, konten: '', altText: '', urutan: items.length + 1 }]); const update = (next: Row[]) => setItems(next.map((item, index) => ({ ...item, urutan: index + 1 }))); return <div className="space-y-2 border-t pt-4"><div className="flex flex-wrap items-end justify-between gap-2"><div><h4 className="font-medium">Stimulus pendukung <span className="font-normal text-muted-foreground">(opsional)</span></h4><p className="text-xs text-muted-foreground">Tambahkan bacaan, tabel sederhana, tautan video HTTPS, atau gambar. Gunakan tombol panah bila tidak memakai mouse.</p></div><div className="flex gap-2"><Select value={kind} onChange={(event) => setKind(event.target.value)}><option value="text">Teks / bacaan</option><option value="table">Tabel</option><option value="media_link">Tautan media</option></Select><Button type="button" size="sm" variant="outline" onClick={add}><Plus className="h-3.5 w-3.5" /> Tambah</Button></div></div>{items.map((item: Row, index: number) => <div key={item.id} draggable onDragStart={(event) => startReorderDrag(event, index, setDragged, 'stimulus')} onDragEnd={() => setDragged(null)} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); const source = sourceReorderIndex(event, dragged, 'stimulus'); if (source !== null && source !== index) update(reordered(items, source, index)); setDragged(null) }} className="rounded-xl border bg-background p-2"><div className="mb-2 flex items-center justify-between"><span className="flex items-center gap-1 text-xs font-medium"><span className="cursor-grab"><DragHandle /></span>{item.jenis === 'table' ? <Table2 className="h-3.5 w-3.5" /> : item.jenis === 'media_link' ? <Link2 className="h-3.5 w-3.5" /> : <FileText className="h-3.5 w-3.5" />}{item.jenis === 'media_link' ? 'Tautan media' : item.jenis === 'table' ? 'Tabel' : 'Teks'}</span><span className="flex items-center gap-1"><ReorderButtons index={index} length={items.length} label={`stimulus ${index + 1}`} onMove={(to) => update(reordered(items, index, to))} /><Button type="button" size="icon" variant="ghost" onClick={() => update(items.filter((row: Row) => row.id !== item.id))} aria-label={`Hapus stimulus ${index + 1}`}><Trash2 className="h-4 w-4" /></Button></span></div><textarea className="min-h-20 w-full rounded-lg border border-input bg-background p-2 text-sm" value={item.konten} onChange={(event) => update(items.map((row: Row) => row.id === item.id ? { ...row, konten: event.target.value } : row))} placeholder={item.jenis === 'media_link' ? 'https://…' : item.jenis === 'table' ? 'Ketik tabel sederhana, satu baris per baris.' : 'Ketik teks stimulus di sini.'} /></div>)}<label className="flex cursor-pointer items-center gap-2 rounded-xl border border-dashed p-3 text-sm hover:bg-muted/50"><ImageIcon className="h-4 w-4 text-primary" /><span className="flex-1">{image ? image.name : 'Unggah gambar stimulus PNG/JPG (maks. 5 MB)'}</span><input className="sr-only" type="file" accept="image/png,image/jpeg" onChange={(event) => setImage(event.target.files?.[0] || null)} /></label></div> }
function QuestionPreview({ form, config }: { form: Row; config: any }) { return <aside className="h-fit rounded-2xl border bg-slate-50 p-4"><p className="text-xs font-bold uppercase tracking-wide text-primary">Pratinjau siswa</p><h3 className="mt-2 font-semibold">{form.pertanyaan || 'Pertanyaan akan muncul di sini'}</h3><p className="mt-1 text-xs text-muted-foreground">{form.bobot || 1} poin · {questionTypes.find(([id]) => id === form.tipe)?.[1]}</p><div className="mt-4 rounded-xl border bg-white p-3"><QuestionAnswerControl question={{ ...form, konfigurasi: config }} questionId="visual-question-preview" value={null} onChange={() => undefined} /></div><p className="mt-4 text-xs text-muted-foreground">Kunci jawaban tidak ditampilkan pada pratinjau atau kepada siswa.</p></aside> }

const asDateTimeInput = (value: string | null | undefined) => value ? new Date(value).toISOString().slice(0, 16) : ''
const canvasQuestion = (type: QuestionType = 'pg_tunggal') => ({ ...emptyQuestion(), tipe: type, konfigurasi: configExample(type), stimulus: [] as Row[] })
const canvasPackage = () => ({ ...emptyPaket(), nama: '', waktuMulai: '', waktuSelesai: '' })
function canvasPayload(draft: any) {
  const toTime = (value: string) => value ? new Date(value).toISOString() : null
  return {
    revision: Number(draft.paket.revision || draft.revision || 0),
    paket: { ...draft.paket, mapelId: draft.paket.mapelId || null, durasiMenit: Number(draft.paket.durasiMenit || 60), maksPercobaan: Number(draft.paket.maksPercobaan || 1), nilaiLulus: draft.paket.nilaiLulus === '' || draft.paket.nilaiLulus === null ? null : Number(draft.paket.nilaiLulus), waktuMulai: toTime(draft.paket.waktuMulai), waktuSelesai: toTime(draft.paket.waktuSelesai) },
    sections: (draft.sections || []).map((section: Row, index: number) => ({ id: section.id, nama: section.nama, deskripsi: section.deskripsi || '', urutan: index + 1 })),
    items: draft.items.map((item: Row) => ({ soalId: item.soalId || '', bagianId: item.bagianId || '', bobot: Number(item.bobot || item.soal?.bobot || 1), soal: item.sourceOnly ? undefined : { ...item.soal, mapelId: item.soal?.mapelId || null, bobot: Number(item.soal?.bobot || 1), status: 'draf', stimulus: (item.soal?.stimulus || []).filter((stimulus: Row) => stimulus.jenis !== 'image') } })),
    pesertaDidikIds: draft.pesertaDidikIds,
  }
}
function mergeBuilderResponse(response: Row, previous: any): any {
  const paket = { ...response.paket, waktuMulai: asDateTimeInput(response.paket?.waktuMulai), waktuSelesai: asDateTimeInput(response.paket?.waktuSelesai), nilaiLulus: response.paket?.nilaiLulus ?? '' }
  const sections = (response.sections || []).map((section: Row) => ({ id: section.id, nama: section.nama, deskripsi: section.deskripsi || '' }))
  const items = (response.items || []).map((server: Row, index: number) => {
    const before = previous.items[index] || {}
    return { ...before, id: server.id, soalId: server.soal?.id || server.soalId, bagianId: server.bagianId || '', bobot: server.bobot, soal: { ...(before.soal || {}), ...(server.soal || {}) } }
  })
  return { paket, sections, items, pesertaDidikIds: response.pesertaDidikIds || [] }
}
function BuilderCanvas({ token, students, classes, bank, materials, initialPaket, onClose }: any) {
  const builderRef = useRef<HTMLDivElement | null>(null)
  const [draft, setDraftState] = useState<any>(() => ({ paket: canvasPackage(), sections: [], items: initialPaket?.prefillQuestion ? [{ soalId: initialPaket.prefillQuestion.id, bagianId: '', sourceOnly: true, bobot: initialPaket.prefillQuestion.bobot, soal: initialPaket.prefillQuestion }] : [], pesertaDidikIds: [] }))
  const draftRef = useRef(draft)
  const historyRef = useRef<{ past: any[]; future: any[]; groupKey: string; groupAt: number }>({ past: [], future: [], groupKey: '', groupAt: 0 })
  const [historyFlags, setHistoryFlags] = useState({ canUndo: false, canRedo: false })
  const [historyAnnouncement, setHistoryAnnouncement] = useState('')
  const [saveState, setSaveState] = useState('Membuat draf…')
  const setDraft = (updater: any, groupKey = '') => {
    const current = draftRef.current
    const next = typeof updater === 'function' ? updater(current) : updater
    if (JSON.stringify(current) === JSON.stringify(next)) return
    const history = historyRef.current
    const now = Date.now()
    const grouped = Boolean(groupKey && history.groupKey === groupKey && now - history.groupAt < 900)
    if (!grouped) {
      history.past.push(JSON.parse(JSON.stringify(current)))
      if (history.past.length > 50) history.past.shift()
    }
    history.future = []
    history.groupKey = groupKey
    history.groupAt = now
    draftRef.current = next
    setDraftState(next)
    setHistoryFlags({ canUndo: history.past.length > 0, canRedo: false })
  }
  const setServerDraft = (next: any, resetHistory = false) => {
    draftRef.current = next
    setDraftState(next)
    if (resetHistory) {
      historyRef.current = { past: [], future: [], groupKey: '', groupAt: 0 }
      setHistoryFlags({ canUndo: false, canRedo: false })
    }
  }
  const undoDraft = useCallback(() => {
    if (saveState === 'Menyimpan…') return
    const history = historyRef.current
    const previous = history.past.pop()
    if (!previous) return
    history.future.push(draftRef.current)
    history.groupKey = ''
    draftRef.current = previous
    setDraftState(previous)
    setHistoryFlags({ canUndo: history.past.length > 0, canRedo: true })
    setHistoryAnnouncement('Perubahan terakhir diurungkan.')
  }, [saveState])
  const redoDraft = useCallback(() => {
    if (saveState === 'Menyimpan…') return
    const history = historyRef.current
    const next = history.future.pop()
    if (!next) return
    history.past.push(draftRef.current)
    if (history.past.length > 50) history.past.shift()
    history.groupKey = ''
    draftRef.current = next
    setDraftState(next)
    setHistoryFlags({ canUndo: history.past.length > 0, canRedo: history.future.length > 0 })
    setHistoryAnnouncement('Perubahan diulangi.')
  }, [saveState])
  const [packageID, setPackageID] = useState<string | null>(initialPaket?.id || null)
  const [ready, setReady] = useState(!initialPaket?.id)
  const [conflict, setConflict] = useState(false)
  const [active, setActive] = useState(0)
  const [pickerOpen, setPickerOpen] = useState(false)
  const [preview, setPreview] = useState(false)
  const [mobileSheet, setMobileSheet] = useState<'questions' | 'settings' | 'participants' | 'preview' | null>(null)
  const mobileSheetTrigger = useRef<HTMLElement | null>(null)
  const saved = useRef('')
  const sequence = useRef(Promise.resolve())
  const pendingImage = useRef<Record<string, File>>({})
  const localDraftKey = useRef(`builder-local-${newID('draft')}`)
  const offlineQueued = useRef(false)
  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = previousOverflow }
  }, [])
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target
      const editingText = target instanceof HTMLElement && (target.isContentEditable || target.matches('input, textarea, select, [contenteditable="true"]'))
      if ((event.ctrlKey || event.metaKey) && !editingText) {
        const key = event.key.toLowerCase()
        if (key === 'z' && event.shiftKey || key === 'y') {
          event.preventDefault()
          redoDraft()
          return
        }
        if (key === 'z') {
          event.preventDefault()
          undoDraft()
          return
        }
      }
      if (event.key === 'Escape') {
        if (mobileSheet) {
          // Handle the sheet before Radix's bubbling Escape handler can close
          // both the sheet and the full-screen builder in the same keypress.
          event.preventDefault()
          event.stopPropagation()
          setMobileSheet(null)
        } else onClose()
        return
      }
      // The full-screen builder is a modal workspace. Keep keyboard focus out
      // of the hidden staff app while a mobile sheet is not already trapping it.
      if (event.key !== 'Tab' || mobileSheet) return
      const root = builderRef.current
      if (!root) return
      const focusable = Array.from(root.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), summary, [tabindex]:not([tabindex="-1"])',
      )).filter((element) => element.getAttribute('aria-hidden') !== 'true' && element.getClientRects().length > 0)
      if (!focusable.length) {
        event.preventDefault()
        root.focus()
        return
      }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      const activeElement = document.activeElement
      if (event.shiftKey && (activeElement === first || !root.contains(activeElement))) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && (activeElement === last || !root.contains(activeElement))) {
        event.preventDefault()
        first.focus()
      }
    }
    window.addEventListener('keydown', handleKeyDown, true)
    return () => window.removeEventListener('keydown', handleKeyDown, true)
  }, [mobileSheet, onClose, redoDraft, undoDraft])
  useEffect(() => {
    // The legacy question cards use text/plain for their own reorder operation.
    // Reject other drag payloads at capture time so dropping a question type,
    // option, or stimulus over the sidebar cannot accidentally move question 1.
    const guardQuestionListDrop = (event: globalThis.DragEvent) => {
      const target = event.target
      if (!(target instanceof Element)) return
      const sidebar = target.closest('aside')
      if (!sidebar?.querySelector('strong')?.textContent?.startsWith('Soal (')) return
      if (Array.from(event.dataTransfer?.types || []).includes('text/plain')) return
      event.preventDefault()
      event.stopPropagation()
    }
    document.addEventListener('drop', guardQuestionListDrop, true)
    return () => document.removeEventListener('drop', guardQuestionListDrop, true)
  }, [])
  useEffect(() => {
    if (!initialPaket?.id) return
    void request(`/simulasi/paket/${initialPaket.id}/builder`, token).then((response) => {
      const next = mergeBuilderResponse(response, { paket: canvasPackage(), sections: [], items: [], pesertaDidikIds: [] })
      saved.current = JSON.stringify(canvasPayload(next)); setServerDraft(next, true); setPackageID(response.paket.id); setReady(true); setSaveState('Tersimpan')
    }).catch((error) => { toast.error(error.message || 'Draf paket tidak dapat dimuat.'); onClose() })
  }, [initialPaket?.id, onClose, token])
  const persist = async (current: any) => {
    const payload = canvasPayload(current)
    setSaveState('Menyimpan…')
    const path = packageID ? `/simulasi/paket/${packageID}/builder` : '/simulasi/paket/builder'
    const method = packageID ? 'PUT' as const : 'POST' as const
    try {
      const response = await request(path, token, method, payload)
      const next = mergeBuilderResponse(response, current)
      saved.current = JSON.stringify(canvasPayload(next)); setPackageID(response.paket.id); setServerDraft(next); setSaveState('Tersimpan'); setConflict(false)
      offlineQueued.current = false
      await removeDraft(packageID ? `builder-${packageID}` : localDraftKey.current).catch(() => undefined)
      return next
    } catch (error) {
      const apiError = error as ApiError
      if (!navigator.onLine || error instanceof TypeError) {
        await enqueueDraft({ key: packageID ? `builder-${packageID}` : localDraftKey.current, path, method, payload, queuedAt: Date.now() }).catch(() => undefined)
        saved.current = JSON.stringify(payload)
        offlineQueued.current = true
        setSaveState('Offline — tersimpan lokal')
        return current
      }
      if (apiError.status === 409) { setSaveState('Konflik versi — pilih versi yang ingin dipakai'); setConflict(true) }
      throw error
    }
  }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!ready) return
    const signature = JSON.stringify(canvasPayload(draft))
    if (signature === saved.current) return
    setSaveState('Perubahan belum tersimpan')
    const timer = setTimeout(() => { sequence.current = sequence.current.catch(() => undefined).then(() => persist(draft).then(() => undefined)).catch((error) => { if ((error as ApiError).status !== 409) setSaveState('Gagal menyimpan — perubahan tetap di layar'); toast.error(error.message || 'Draf tidak tersimpan.') }) }, 700)
    return () => clearTimeout(timer)
  }, [draft, ready]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const handleOnline = () => { if (ready && (offlineQueued.current || JSON.stringify(canvasPayload(draft)) !== saved.current)) { offlineQueued.current = false; void persist(draft) } }
    window.addEventListener('online', handleOnline)
    return () => window.removeEventListener('online', handleOnline)
  }, [draft, ready]) // eslint-disable-line react-hooks/exhaustive-deps
  const updatePaket = (key: string, value: any) => setDraft((valueNow: any) => ({ ...valueNow, paket: { ...valueNow.paket, [key]: value } }), `paket:${key}`)
  const updateItem = (index: number, next: Row) => {
    const before = draftRef.current.items[index]
    const changedQuestionFields = Object.keys(next.soal || {}).filter((key) => JSON.stringify(next.soal?.[key]) !== JSON.stringify(before?.soal?.[key]))
    const group = changedQuestionFields.length === 1 ? `item:${index}:${changedQuestionFields[0]}` : ''
    setDraft((valueNow: any) => ({ ...valueNow, items: valueNow.items.map((item: Row, itemIndex: number) => itemIndex === index ? next : item) }), group)
  }
  const addQuestion = (type: QuestionType, at = draft.items.length) => { setDraft((valueNow: any) => { const items = [...valueNow.items]; const sectionId = valueNow.items[active]?.bagianId || valueNow.sections[valueNow.sections.length - 1]?.id || ''; items.splice(Math.min(at, items.length), 0, { soal: canvasQuestion(type), bagianId: sectionId, bobot: 1 }); return { ...valueNow, items } }); setActive(at) }
  const handleTypeDrop = (event: DragEvent<HTMLElement>) => { const type = event.dataTransfer.getData('application/x-simulasi-question-type') as QuestionType; if (!questionTypes.some(([id]) => id === type)) return; event.preventDefault(); addQuestion(type, activeQuestion ? active + 1 : draft.items.length) }
  const removeQuestion = (index: number) => { setDraft((valueNow: any) => ({ ...valueNow, items: valueNow.items.filter((_: Row, itemIndex: number) => itemIndex !== index) })); setActive(Math.max(0, index - 1)) }
  const copyQuestion = (index: number) => setDraft((valueNow: any) => { const source = valueNow.items[index]; const copy = { ...source, id: undefined, soalId: undefined, sourceOnly: false, soal: { ...source.soal, id: undefined, status: 'draf', pertanyaan: `${source.soal?.pertanyaan || ''}`.trim(), stimulus: [...(source.soal?.stimulus || [])] } }; const items = [...valueNow.items]; items.splice(index + 1, 0, copy); return { ...valueNow, items } })
  const uploadImage = async (index: number, file: File) => {
    const item = draft.items[index]
    if (!item?.soalId || item.sourceOnly) { pendingImage.current[String(index)] = file; toast.message('Gambar akan dapat diunggah setelah salinan draf soal tersimpan.'); return }
    const form = new FormData(); form.append('file', file); form.append('altText', `Stimulus untuk ${item.soal?.pertanyaan || 'soal simulasi'}`)
    try {
      const response = await fetch(`${apiBase}/simulasi/soal/${item.soalId}/stimulus/gambar`, { method: 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: form })
      if (!response.ok) throw new Error('Gambar stimulus gagal diunggah.')
      const question = await request(`/simulasi/soal/${item.soalId}`, token)
      updateItem(index, { ...item, soal: question }); toast.success('Gambar stimulus ditambahkan.')
    } catch (error: any) { toast.error(error.message || 'Gambar stimulus gagal diunggah.') }
  }
  const uploadChoiceImage = async (index: number, choiceID: string, file: File, altText: string) => {
    const item = draft.items[index]
    if (!item?.soalId || item.sourceOnly) throw new Error('Draf soal sedang disiapkan. Tunggu status tersimpan, lalu unggah kembali gambar pilihan.')
    const form = new FormData(); form.append('file', file); form.append('altText', altText)
    const response = await fetch(`${apiBase}/simulasi/soal/${encodeURIComponent(item.soalId)}/opsi/${encodeURIComponent(choiceID)}/gambar`, { method: 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: form })
    const body = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(body.error || 'Gambar pilihan belum dapat diunggah.')
    return body as Row
  }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { draft.items.forEach((item: Row, index: number) => { const file = pendingImage.current[String(index)]; if (file && item.soalId && !item.sourceOnly) { delete pendingImage.current[String(index)]; void uploadImage(index, file) } }) }, [draft.items])
  const loadServerVersion = async () => {
    if (!packageID) return
    try {
      const response = await request(`/simulasi/paket/${packageID}/builder`, token)
      const next = mergeBuilderResponse(response, draft)
      setServerDraft(next, true); saved.current = JSON.stringify(canvasPayload(next)); setSaveState('Tersimpan'); setConflict(false)
      toast.success('Versi server dimuat.')
    } catch (error: any) { toast.error(error.message || 'Versi server belum dapat dimuat.') }
  }
  const saveAsCopy = async () => {
    try {
      const response = await request('/simulasi/paket/builder', token, 'POST', { ...canvasPayload(draft), revision: 0 })
      const next = mergeBuilderResponse(response, draft)
      setServerDraft(next, true); setPackageID(response.paket.id); saved.current = JSON.stringify(canvasPayload(next)); setSaveState('Tersimpan sebagai salinan'); setConflict(false)
      toast.success('Perubahan disimpan sebagai salinan draf baru.')
    } catch (error: any) { toast.error(error.message || 'Salinan draf belum dapat dibuat.') }
  }
  const publish = async () => {
    try { const savedDraft = await persist(draft); const id = packageID || savedDraft.paket.id; await request(`/simulasi/paket/${id}/publikasi`, token, 'POST'); toast.success('Paket diterbitkan dan soal dibekukan.'); onClose() } catch (error: any) { toast.error(error.message || 'Lengkapi soal dan peserta sebelum menerbitkan.') }
  }
  const openPreviewTab = async () => {
    // Open synchronously from the click gesture so browsers do not treat the
    // later async save as a popup. The tab is navigated after the latest draft
    // has been flushed to the server.
    const previewWindow = window.open('', '_blank')
    if (previewWindow) previewWindow.opener = null
    try {
      // Flush the current canvas even when this is an existing draft. The
      // preview tab must reflect the latest edits, not the last autosave.
      const savedDraft = await persist(draft)
      const id = savedDraft.paket.id
      if (!id) throw new Error('Draf belum memiliki ID paket.')
      const previewURL = `/simulasi/preview/${encodeURIComponent(id)}`
      if (previewWindow) previewWindow.location.href = previewURL
      else window.open(previewURL, '_blank', 'noopener,noreferrer')
    } catch (error: any) {
      previewWindow?.close()
      toast.error(error.message || 'Pratinjau belum dapat dibuka.')
    }
  }
  const activeItem = draft.items[active]
  const activeQuestion = activeItem?.soal || null
  const addSection = () => {
    const section = { id: newID('section'), nama: `Bagian ${draft.sections.length + 1}`, deskripsi: '' }
    setDraft((current: any) => ({ ...current, sections: [...current.sections, section], items: current.items.map((item: Row, itemIndex: number) => itemIndex === active ? { ...item, bagianId: section.id } : item) }))
  }
  const updateSection = (index: number, next: Row) => setDraft((current: any) => ({ ...current, sections: current.sections.map((section: Row, sectionIndex: number) => sectionIndex === index ? next : section) }))
  const removeSection = (index: number) => setDraft((current: any) => { const removed = current.sections[index]; return { ...current, sections: current.sections.filter((_: Row, sectionIndex: number) => sectionIndex !== index), items: current.items.map((item: Row) => { const config = item.soal?.konfigurasi || {}; const routes = Object.fromEntries(Object.entries(config.branchToByAnswer || {}).filter(([, target]) => target !== removed.id)); const soal = item.soal ? { ...item.soal, konfigurasi: { ...config, branchToByAnswer: Object.keys(routes).length ? routes : undefined } } : item.soal; return { ...item, ...(item.bagianId === removed.id ? { bagianId: '' } : {}), ...(soal ? { soal } : {}) } }) } })
  const showMobileSheet = (sheet: NonNullable<typeof mobileSheet>, trigger: HTMLElement) => { mobileSheetTrigger.current = trigger; setMobileSheet(sheet) }
  return <div ref={builderRef} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby="builder-title" className="fixed inset-0 z-[100000] isolate overflow-auto overscroll-contain bg-slate-100 text-slate-900"><div className="mx-auto min-h-full max-w-[1600px] p-3 sm:p-5"><header className="sticky top-0 z-20 mb-4 flex flex-wrap items-center gap-2 rounded-2xl border bg-white/95 p-3 shadow-sm backdrop-blur sm:gap-3"><Button type="button" size="icon" variant="ghost" onClick={onClose} aria-label="Tutup pembuat paket"><X className="h-5 w-5" /></Button><Input id="builder-title" autoFocus={!draft.paket.nama} className="h-10 min-w-0 flex-1 border-0 text-lg font-bold shadow-none focus-visible:ring-0 sm:min-w-60" value={draft.paket.nama} onChange={(event) => updatePaket('nama', event.target.value)} placeholder="Beri judul paket" aria-label="Judul paket simulasi" /><span className={`flex min-h-8 items-center gap-1 rounded-full px-2 text-xs ${saveState.startsWith('Gagal') || saveState.startsWith('Konflik') ? 'bg-destructive/10 text-destructive' : 'bg-muted text-muted-foreground'}`}><Save className="h-3.5 w-3.5" />{saveState}</span><Button className="min-h-11 min-w-11" size="icon" type="button" variant="outline" onClick={undoDraft} disabled={!historyFlags.canUndo || saveState === 'Menyimpan…'} aria-label="Urungkan perubahan" title="Urungkan perubahan (Ctrl/Cmd+Z)"><Undo2 className="h-4 w-4" /></Button><Button className="min-h-11 min-w-11" size="icon" type="button" variant="outline" onClick={redoDraft} disabled={!historyFlags.canRedo || saveState === 'Menyimpan…'} aria-label="Ulangi perubahan" title="Ulangi perubahan (Ctrl/Cmd+Y)"><Redo2 className="h-4 w-4" /></Button><Button className="min-h-11" type="button" variant="outline" onClick={() => void openPreviewTab()}><ExternalLink className="h-4 w-4" /> Tab baru</Button><Button className="min-h-11" type="button" variant="outline" onClick={(event) => { if (preview) { setPreview(false); setMobileSheet(null) } else { setPreview(true); if (window.matchMedia('(max-width: 1279px)').matches) showMobileSheet('preview', event.currentTarget) } }}>{preview ? 'Tutup pratinjau' : 'Pratinjau siswa'}</Button><Button className="min-h-11" type="button" onClick={() => void publish()} disabled={!ready || saveState === 'Menyimpan…' || !navigator.onLine}>Terbitkan</Button></header>
    <span className="sr-only" aria-live="polite">{saveState} {historyAnnouncement}</span>{conflict && <div role="alert" className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950"><div><strong>Versi draf berubah di perangkat lain.</strong><p className="mt-1 text-xs">Pilih versi server, atau simpan pekerjaan saat ini sebagai salinan baru.</p></div><div className="flex flex-wrap gap-2"><Button className="min-h-11" size="sm" variant="outline" onClick={() => void loadServerVersion()}>Muat versi server</Button><Button className="min-h-11" size="sm" onClick={() => void saveAsCopy()}>Simpan sebagai salinan</Button></div></div>}
    <div className="sticky top-[4.75rem] z-10 mb-4 grid grid-cols-2 gap-2 rounded-2xl border bg-white/95 p-2 shadow-sm backdrop-blur sm:grid-cols-4 xl:hidden"><Button className="min-h-11" variant="outline" onClick={(event) => showMobileSheet('questions', event.currentTarget)}>Soal ({draft.items.length})</Button><Button className="min-h-11" variant="outline" onClick={(event) => showMobileSheet('settings', event.currentTarget)}>Pengaturan</Button><Button className="min-h-11" variant="outline" onClick={(event) => showMobileSheet('participants', event.currentTarget)}>Peserta ({draft.pesertaDidikIds.length})</Button><Button className="min-h-11" variant="outline" onClick={(event) => { setPreview(true); showMobileSheet('preview', event.currentTarget) }}>Pratinjau</Button></div>
    <section className="mb-4 space-y-2 rounded-2xl border bg-white p-3 xl:hidden"><div className="flex items-center justify-between gap-2"><div><h2 className="text-sm font-bold">Bagian form ({draft.sections.length})</h2><p className="text-xs text-muted-foreground">Kelompokkan soal dan atur urutan bagian.</p></div><Button type="button" className="min-h-11" size="sm" variant="outline" onClick={addSection}><Plus className="h-4 w-4" /> Tambah bagian</Button></div>{draft.sections.map((section: Row, sectionIndex: number) => <div key={section.id} className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 rounded-xl border p-2"><Input aria-label={`Nama bagian ${sectionIndex + 1} di ponsel`} value={section.nama} onChange={(event) => updateSection(sectionIndex, { ...section, nama: event.target.value })} /><ReorderButtons index={sectionIndex} length={draft.sections.length} label={`bagian ${sectionIndex + 1}`} onMove={(to) => setDraft((current: any) => ({ ...current, sections: reordered(current.sections, sectionIndex, to) }))} /><Button type="button" size="icon" variant="ghost" className="h-11 w-11" aria-label={`Hapus bagian ${sectionIndex + 1}`} onClick={() => removeSection(sectionIndex)}><Trash2 className="h-4 w-4" /></Button><Input className="col-span-3" aria-label={`Petunjuk bagian ${sectionIndex + 1} di ponsel`} value={section.deskripsi} placeholder="Petunjuk (opsional)" onChange={(event) => updateSection(sectionIndex, { ...section, deskripsi: event.target.value })} /></div>)}{activeQuestion && draft.sections.length > 0 && <Field label="Bagian soal yang sedang dipilih"><Select aria-label="Bagian soal yang sedang dipilih" value={activeItem.bagianId || ''} onChange={(event) => updateItem(active, { ...activeItem, bagianId: event.target.value })}><option value="">Tanpa bagian</option>{draft.sections.map((section: Row) => <option key={section.id} value={section.id}>{section.nama}</option>)}</Select></Field>}</section>
    <section aria-label="Tema dan pengalaman siswa" className="mb-4 grid gap-3 rounded-2xl border bg-white p-4 md:grid-cols-2">
      <Field label="Tema visual paket"><Select aria-label="Tema visual paket" value={draft.paket.temaWarna || '#1c5d94'} onChange={(event) => updatePaket('temaWarna', event.target.value)}><option value="#1c5d94">Biru institusi</option><option value="#166534">Hijau</option><option value="#6b21a8">Ungu</option><option value="#9a3412">Jingga</option><option value="#334155">Arang</option></Select></Field>
      <Field label="Pesan setelah siswa mengirim"><Input aria-label="Pesan setelah siswa mengirim" maxLength={500} value={draft.paket.pesanKonfirmasi || ''} onChange={(event) => updatePaket('pesanKonfirmasi', event.target.value)} placeholder="Jawaban kamu sudah berhasil dikirim." /></Field>
    </section>
    <div className="grid gap-4 xl:grid-cols-[260px_minmax(0,1fr)_310px]">
      <aside className="h-fit self-start rounded-2xl border bg-white p-3 xl:sticky xl:top-24"><div className="mb-3 flex items-center justify-between"><strong className="text-sm">Soal ({draft.items.length})</strong><Button size="sm" variant="ghost" onClick={() => setPickerOpen(!pickerOpen)}>Bank soal</Button></div><div className="mb-3 rounded-xl border bg-slate-50 p-2"><div className="mb-2 flex items-center justify-between"><strong className="text-xs">Bagian form ({draft.sections.length})</strong><Button type="button" size="sm" variant="outline" className="min-h-9" onClick={addSection}><Plus className="h-3.5 w-3.5" /> Bagian</Button></div><div className="space-y-2">{draft.sections.map((section: Row, sectionIndex: number) => <div key={section.id} draggable onDragStart={(event) => event.dataTransfer.setData('application/x-simulasi-section', String(sectionIndex))} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { const from = Number(event.dataTransfer.getData('application/x-simulasi-section')); if (from === sectionIndex || !Number.isInteger(from)) return; setDraft((current: any) => ({ ...current, sections: reordered(current.sections, from, sectionIndex) })) }} className="space-y-1 rounded-lg border bg-white p-2"><div className="flex items-center gap-1"><span className="cursor-grab text-muted-foreground" aria-hidden="true"><DragHandle /></span><Input aria-label={`Nama bagian ${sectionIndex + 1}`} className="h-9" value={section.nama} onChange={(event) => updateSection(sectionIndex, { ...section, nama: event.target.value })} /><ReorderButtons index={sectionIndex} length={draft.sections.length} label={`bagian ${sectionIndex + 1}`} onMove={(to) => setDraft((current: any) => ({ ...current, sections: reordered(current.sections, sectionIndex, to) }))} /><Button type="button" size="icon" variant="ghost" aria-label={`Hapus bagian ${sectionIndex + 1}`} onClick={() => removeSection(sectionIndex)}><Trash2 className="h-4 w-4" /></Button></div><Input aria-label={`Deskripsi bagian ${sectionIndex + 1}`} className="h-9 text-xs" value={section.deskripsi} placeholder="Petunjuk bagian (opsional)" onChange={(event) => updateSection(sectionIndex, { ...section, deskripsi: event.target.value })} /><p className="px-1 text-[11px] text-muted-foreground">{draft.items.filter((item: Row) => item.bagianId === section.id).length} soal · seret untuk mengurutkan</p></div>)}{!draft.sections.length && <p className="px-1 text-[11px] leading-relaxed text-muted-foreground">Tambahkan bagian untuk mengelompokkan soal, seperti Google Forms. Paket lama tanpa bagian tetap didukung.</p>}</div></div><div className="grid gap-2"><Select aria-label="Tambah jenis pertanyaan" value="" onChange={(event) => { if (event.target.value) addQuestion(event.target.value as QuestionType); event.currentTarget.value = '' }}><option value="">+ Tambah pertanyaan</option>{questionTypes.map(([id, label]) => <option key={id} value={id}>{label}</option>)}</Select><details className="rounded-xl border border-dashed p-2"><summary className="min-h-10 cursor-pointer py-2 text-xs font-medium">Atau seret tipe soal ke kanvas</summary><div className="mt-2 grid gap-1">{questionTypes.map(([id, label]) => <button type="button" key={id} draggable onDragStart={(event) => { event.dataTransfer.effectAllowed = 'copy'; event.dataTransfer.setData('application/x-simulasi-question-type', id) }} onClick={() => addQuestion(id)} className="flex min-h-10 items-center gap-2 rounded-lg px-2 text-left text-xs hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"><DragHandle />{label}<span className="ml-auto text-muted-foreground">Klik atau seret</span></button>)}</div></details>{draft.items.map((item: Row, index: number) => <div key={item.id || `${item.soalId}-${index}`} className="flex items-stretch gap-1"><button type="button" draggable onDragStart={(event) => event.dataTransfer.setData('text/plain', String(index))} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { const source = Number(event.dataTransfer.getData('text/plain')); if (source === index) return; setDraft((valueNow: any) => ({ ...valueNow, items: reordered(valueNow.items, source, index) })); setActive(index) }} onClick={() => setActive(index)} className={`min-w-0 flex-1 rounded-xl border p-3 text-left text-sm ${active === index ? 'border-primary bg-primary/10' : 'hover:bg-muted/50'}`}><span className="flex items-center gap-2"><DragHandle />{index + 1}. {item.soal?.pertanyaan || 'Pertanyaan tanpa judul'}</span><span className="mt-1 block pl-6 text-xs text-muted-foreground">{questionTypes.find(([id]) => id === item.soal?.tipe)?.[1] || 'Soal'}{item.bagianId ? ` · ${draft.sections.find((section: Row) => section.id === item.bagianId)?.nama || 'Bagian'}` : ''}</span></button><ReorderButtons index={index} length={draft.items.length} label={`soal ${index + 1}`} onMove={(to) => { setDraft((valueNow: any) => ({ ...valueNow, items: reordered(valueNow.items, index, to) })); setActive(to) }} /></div>)}</div>
        {pickerOpen && <div className="mt-3 max-h-72 space-y-3 overflow-auto rounded-xl border bg-muted/30 p-2"><div><p className="mb-1 text-xs font-medium">Gunakan soal terbit dari bank</p>{bank.filter((question: Row) => question.status === 'terbit').map((question: Row) => <button type="button" key={question.id} onClick={() => { setDraft((valueNow: any) => ({ ...valueNow, items: [...valueNow.items, { soalId: question.id, sourceOnly: true, bobot: question.bobot, soal: question }] })); setActive(draft.items.length); setPickerOpen(false) }} className="w-full rounded-lg bg-white p-2 text-left text-xs hover:ring-1 hover:ring-primary">{question.pertanyaan}</button>)}{!bank.some((question: Row) => question.status === 'terbit') && <p className="p-2 text-xs text-muted-foreground">Belum ada soal terbit yang dapat digunakan.</p>}</div><div><p className="mb-1 text-xs font-medium">Bahan reusable</p>{materials.slice(0, 8).map((material: Row) => <button type="button" key={material.id} disabled={!activeQuestion} onClick={() => { if (!activeQuestion) return; const stimulus = [...(activeQuestion.stimulus || []), { id: material.id, jenis: material.jenis, konten: material.konten, altText: material.altText }]; updateItem(active, { ...activeItem, sourceOnly: false, soal: { ...activeQuestion, stimulus } }); setPickerOpen(false) }} className="w-full rounded-lg bg-white p-2 text-left text-xs hover:ring-1 hover:ring-primary disabled:opacity-50">{material.judul}</button>)}{!materials.length && <p className="p-2 text-xs text-muted-foreground">Belum ada bahan reusable.</p>}</div></div>}</aside>
      <main onDragOver={(event) => { if (event.dataTransfer.types.includes('application/x-simulasi-question-type')) event.preventDefault() }} onDrop={handleTypeDrop} className="min-w-0 rounded-2xl transition-colors [&:has([data-type-drop])]:bg-primary/5">{activeQuestion && draft.sections.length > 0 && <div className="mb-3 rounded-xl border bg-white p-3"><Field label="Bagian pertanyaan ini"><Select value={activeItem.bagianId || ''} onChange={(event) => updateItem(active, { ...activeItem, bagianId: event.target.value })}><option value="">Tanpa bagian</option>{draft.sections.map((section: Row) => <option key={section.id} value={section.id}>{section.nama}</option>)}</Select></Field></div>}{activeQuestion ? <CanvasQuestionCard item={activeItem} index={active} token={token} sections={draft.sections} sectionId={activeItem.bagianId || ''} onChange={(question: Row) => updateItem(active, { ...activeItem, sourceOnly: false, soal: question })} onDelete={() => { if (window.confirm('Hapus pertanyaan ini dari paket? Soal sumber tetap aman di Bank Soal.')) removeQuestion(active) }} onCopy={() => copyQuestion(active)} onUpload={(file: File) => void uploadImage(active, file)} onUploadChoiceImage={(choiceID: string, file: File, altText: string) => uploadChoiceImage(active, choiceID, file, altText)} /> : <Card data-type-drop className="grid min-h-96 place-items-center p-8 text-center"><div><ClipboardCheck className="mx-auto h-10 w-10 text-primary" /><h2 className="mt-3 font-bold">Mulai dengan sebuah pertanyaan</h2><p className="mt-1 text-sm text-muted-foreground">Pilih jenis soal pada panel kiri atau seret tipe soal ke area ini. Draf paket tersimpan otomatis.</p></div></Card>}{preview && activeQuestion && <div className="mt-4"><StudentQuestionPreview key={activeItem.id || active} question={activeQuestion} token={token} /></div>}</main>
      <aside className="h-fit self-start space-y-3 rounded-2xl border bg-white p-4 xl:sticky xl:top-24"><div><p className="text-xs font-bold uppercase tracking-wide text-primary">Siapkan & terbitkan</p><h2 className="mt-1 font-bold">Pengaturan paket</h2></div><Field label="Durasi (menit)"><Input type="number" min="1" max="360" value={draft.paket.durasiMenit} onChange={(event) => updatePaket('durasiMenit', event.target.value)} /></Field><Field label="Mulai (opsional)"><Input type="datetime-local" value={draft.paket.waktuMulai} onChange={(event) => updatePaket('waktuMulai', event.target.value)} /></Field><Field label="Selesai (opsional)"><Input type="datetime-local" value={draft.paket.waktuSelesai} onChange={(event) => updatePaket('waktuSelesai', event.target.value)} /></Field><Field label="Instruksi siswa"><textarea className="min-h-24 rounded-xl border border-input bg-background p-3 text-sm" value={draft.paket.instruksi} onChange={(event) => updatePaket('instruksi', event.target.value)} /></Field><details className="rounded-xl bg-muted/50 p-3"><summary className="cursor-pointer text-sm font-medium">Pengaturan hasil dan percobaan</summary><div className="mt-3 grid gap-3"><Field label="Maks. percobaan"><Input type="number" min="1" max="10" value={draft.paket.maksPercobaan} onChange={(event) => updatePaket('maksPercobaan', event.target.value)} /></Field><Field label="Nilai lulus"><Input type="number" min="0" max="100" value={draft.paket.nilaiLulus} onChange={(event) => updatePaket('nilaiLulus', event.target.value)} /></Field><label className="flex items-center gap-2 text-sm"><Checkbox checked={draft.paket.acakUrutan} onChange={(event) => updatePaket('acakUrutan', event.currentTarget.checked)} />Acak urutan soal</label><label className="flex items-center gap-2 text-sm"><Checkbox checked={draft.paket.tampilkanNilai} onChange={(event) => updatePaket('tampilkanNilai', event.currentTarget.checked)} />Tampilkan nilai</label><label className="flex items-center gap-2 text-sm"><Checkbox checked={draft.paket.tampilkanRingkasan} onChange={(event) => updatePaket('tampilkanRingkasan', event.currentTarget.checked)} />Tampilkan ringkasan</label><label className="flex items-start gap-2 text-sm"><Checkbox checked={draft.paket.izinkanEditRespons} onChange={(event) => updatePaket('izinkanEditRespons', event.currentTarget.checked)} /><span>Izinkan siswa mengubah respons setelah dikirim<span className="mt-0.5 block text-xs text-muted-foreground">Berlaku sampai jadwal paket berakhir. Perubahan tetap tercatat sebagai revisi.</span></span></label></div></details><ParticipantPicker students={students} classes={classes} selectedIds={draft.pesertaDidikIds} onChange={(ids) => setDraft((valueNow: any) => ({ ...valueNow, pesertaDidikIds: ids }))} /><div className="rounded-xl bg-primary/5 p-3 text-xs"><p>{draft.items.length ? '✓ Ada soal' : '○ Tambahkan minimal satu soal'}</p><p>{draft.pesertaDidikIds.length ? '✓ Peserta ditugaskan' : '○ Pilih minimal satu peserta'}</p><p>{draft.paket.nama.trim() ? '✓ Judul paket diisi' : '○ Beri judul sebelum terbit'}</p></div></aside>
    </div>{mobileSheet && <Dialog open onOpenChange={(open) => { if (!open) setMobileSheet(null) }}><DialogContent onCloseAutoFocus={(event) => { event.preventDefault(); requestAnimationFrame(() => mobileSheetTrigger.current?.focus()) }} className="inset-x-0 bottom-0 left-0 top-auto max-h-[85dvh] w-full max-w-none translate-x-0 translate-y-0 overflow-y-auto rounded-t-3xl rounded-b-none p-4 pb-[max(1rem,env(safe-area-inset-bottom))] shadow-2xl sm:w-full sm:p-4"><DialogHeader className="pr-10"><DialogTitle>{mobileSheet === 'questions' ? 'Daftar soal' : mobileSheet === 'settings' ? 'Pengaturan paket' : mobileSheet === 'participants' ? 'Peserta' : 'Pratinjau siswa'}</DialogTitle><DialogDescription className="sr-only">Panel pembuat simulasi. Tekan Escape untuk menutup dan kembali ke kanvas.</DialogDescription></DialogHeader>{mobileSheet === 'questions' && <div className="space-y-2"><Select aria-label="Tambah pertanyaan" value="" onChange={(event) => { if (event.target.value) addQuestion(event.target.value as QuestionType); event.currentTarget.value = '' }}><option value="">+ Tambah pertanyaan</option>{questionTypes.map(([id, label]) => <option key={id} value={id}>{label}</option>)}</Select>{draft.items.map((item: Row, index: number) => <div className="flex items-stretch gap-1" key={item.id || `${item.soalId}-${index}`}><button type="button" className={`min-h-11 min-w-0 flex-1 rounded-xl border p-3 text-left ${active === index ? 'border-primary bg-primary/10' : ''}`} onClick={() => { setActive(index); setMobileSheet(null) }}>{index + 1}. {item.soal?.pertanyaan || 'Pertanyaan tanpa judul'}</button><ReorderButtons index={index} length={draft.items.length} label={`soal ${index + 1}`} onMove={(to) => { setDraft((valueNow: any) => ({ ...valueNow, items: reordered(valueNow.items, index, to) })); setActive(to) }} /></div>)}</div>}{mobileSheet === 'settings' && <div className="space-y-3"><Field label="Durasi (menit)"><Input type="number" min="1" value={draft.paket.durasiMenit} onChange={(event) => updatePaket('durasiMenit', event.target.value)} /></Field><Field label="Instruksi siswa"><textarea className="min-h-24 w-full rounded-xl border border-input bg-background p-3 text-sm" value={draft.paket.instruksi} onChange={(event) => updatePaket('instruksi', event.target.value)} /></Field><Field label="Nilai lulus"><Input type="number" min="0" max="100" value={draft.paket.nilaiLulus} onChange={(event) => updatePaket('nilaiLulus', event.target.value)} /></Field></div>}{mobileSheet === 'participants' && <ParticipantPicker students={students} classes={classes} selectedIds={draft.pesertaDidikIds} onChange={(ids) => setDraft((valueNow: any) => ({ ...valueNow, pesertaDidikIds: ids }))} />}{mobileSheet === 'preview' && (activeQuestion ? <StudentQuestionPreview key={activeItem.id || active} question={activeQuestion} token={token} /> : <EmptyState title="Belum ada soal" description="Tambahkan soal untuk melihat pratinjau siswa." />)}</DialogContent></Dialog>}</div></div>
}
function ParticipantPicker({ students, classes, selectedIds, onChange }: { students: Row[]; classes: Row[]; selectedIds: string[]; onChange: (ids: string[]) => void }) {
  const activeStudents = students.filter((student) => student.status === 'aktif')
  const selected = new Set(selectedIds)
  const allSelected = activeStudents.length > 0 && activeStudents.every((student) => selected.has(student.id))
  const toggleStudents = (rows: Row[], checked: boolean) => {
    const next = new Set(selected)
    rows.forEach((student) => checked ? next.add(student.id) : next.delete(student.id))
    onChange(activeStudents.filter((student) => next.has(student.id)).map((student) => student.id))
  }
  const classRows = classes.map((kelas) => ({ kelas, students: activeStudents.filter((student) => String(student.kelasId || '') === String(kelas.id)) })).filter((row) => row.students.length > 0)
  const allClassesSelected = classRows.length > 0 && classRows.every(({ students: classStudents }) => classStudents.every((student) => selected.has(student.id)))
  const classStudents = classRows.flatMap((row) => row.students)
  return <section className="border-t pt-3"><div className="mb-2 flex items-center justify-between gap-2"><div><strong className="text-sm">Peserta ({selectedIds.length})</strong><p className="text-[11px] text-muted-foreground">Pilih individu, satu kelas, semua kelas, atau semua peserta aktif.</p></div><Button type="button" size="sm" variant="outline" onClick={() => toggleStudents(activeStudents, !allSelected)}><UsersRound className="h-3.5 w-3.5" />{allSelected ? 'Hapus semua' : 'Pilih semua'}</Button></div><div className="mb-2 space-y-1 rounded-xl border bg-muted/20 p-2"><label className="flex min-h-10 items-center gap-2 rounded-lg px-2 text-xs font-semibold hover:bg-white"><Checkbox checked={allSelected} onChange={(event) => toggleStudents(activeStudents, event.currentTarget.checked)} />Semua peserta aktif ({activeStudents.length})</label><label className="flex min-h-10 items-center gap-2 rounded-lg px-2 text-xs font-semibold hover:bg-white"><Checkbox checked={allClassesSelected} onChange={(event) => toggleStudents(classStudents, event.currentTarget.checked)} />Semua kelas ({classRows.length})</label>{classRows.map(({ kelas, students: rowsInClass }) => { const checked = rowsInClass.every((student) => selected.has(student.id)); return <label key={kelas.id} className="flex min-h-10 items-center gap-2 rounded-lg px-2 text-xs hover:bg-white"><Checkbox checked={checked} onChange={(event) => toggleStudents(rowsInClass, event.currentTarget.checked)} /><span>Kelas {kelas.namaRombel || kelas.nama || kelas.id}</span><span className="ml-auto text-muted-foreground">{rowsInClass.length}</span></label> })}{!classRows.length && <p className="p-2 text-xs text-muted-foreground">Belum ada kelas dengan peserta aktif.</p>}</div><div className="max-h-56 space-y-1 overflow-auto rounded-xl border p-2">{activeStudents.map((student) => <label key={student.id} className="flex min-h-10 items-center gap-2 rounded-lg px-2 text-xs hover:bg-muted/40"><Checkbox checked={selected.has(student.id)} onChange={(event) => toggleStudents([student], event.currentTarget.checked)} />{student.nama}</label>)}{!activeStudents.length && <p className="p-2 text-xs text-muted-foreground">Belum ada peserta aktif.</p>}</div></section>
}
function CanvasQuestionCard({ item, index, token, sections = [], sectionId = '', onChange, onDelete, onCopy, onUpload, onUploadChoiceImage }: any) {
  const question = item.soal; const config = question.konfigurasi || configExample(question.tipe); const update = (key: string, value: any) => onChange({ ...question, [key]: value }); const imageCount = (question.stimulus || []).filter((stimulus: Row) => stimulus.jenis === 'image').length
  const setType = (type: QuestionType) => onChange({ ...question, tipe: type, konfigurasi: configExample(type) })
  const handleTypeDrop = (event: DragEvent<HTMLDivElement>) => {
    const type = event.dataTransfer.getData('application/x-simulasi-question-type') as QuestionType
    if (!questionTypes.some(([id]) => id === type)) return
    event.preventDefault()
    event.stopPropagation()
    setType(type)
  }
  return <Card data-question-card className="overflow-hidden" onDragOver={(event) => { if (event.dataTransfer.types.includes('application/x-simulasi-question-type')) event.preventDefault() }} onDrop={handleTypeDrop}><div className="flex items-center justify-between bg-primary/5 p-3"><span className="font-semibold">Pertanyaan {index + 1}</span><div className="flex gap-1"><Button type="button" size="icon" variant="ghost" onClick={onCopy} aria-label="Duplikasi pertanyaan" title="Duplikasi pertanyaan"><Copy className="h-4 w-4" /></Button><Button type="button" size="icon" variant="ghost" onClick={onDelete} aria-label="Hapus pertanyaan" title="Hapus pertanyaan"><Trash2 className="h-4 w-4" /></Button></div></div><div className="space-y-4 p-4"><div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]"><Select aria-label="Jenis soal" value={question.tipe} onChange={(event) => setType(event.target.value as QuestionType)}>{questionTypes.map(([id, label]) => <option key={id} value={id}>{label}</option>)}</Select><details className="relative"><summary className="flex min-h-11 cursor-pointer list-none items-center justify-center rounded-xl border px-3 text-xs font-medium hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary">Pilih lewat kartu seret</summary><div className="absolute right-0 z-20 mt-1 grid max-h-72 min-w-64 gap-1 overflow-auto rounded-xl border bg-white p-2 shadow-xl">{questionTypes.map(([id, label]) => <button type="button" key={id} draggable aria-label={`Seret jenis soal ${label}`} onDragStart={(event) => { event.dataTransfer.effectAllowed = 'copy'; event.dataTransfer.setData('application/x-simulasi-question-type', id) }} onClick={() => setType(id)} className={`flex min-h-11 items-center gap-2 rounded-lg border px-3 text-left text-sm hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary ${question.tipe === id ? 'border-primary bg-primary/10' : 'border-transparent'}`}><DragHandle />{label}<span className="ml-auto text-xs text-muted-foreground">Klik / seret</span></button>)}</div></details></div><p className="-mt-2 text-xs text-muted-foreground">Klik jenis soal, atau seret kartu jenis soal ke kartu pertanyaan ini. Pilihan ini juga bisa digunakan dengan keyboard.</p><textarea className="min-h-28 w-full rounded-xl border border-input bg-background p-3 text-base leading-relaxed" value={question.pertanyaan} onChange={(event) => update('pertanyaan', event.target.value)} placeholder="Tulis pertanyaan untuk siswa" /><AnswerBuilder groupName={`visual-answer-${item.id || index}`} tipe={question.tipe} config={config} token={token} onUploadChoiceImage={onUploadChoiceImage} setConfig={(konfigurasi: any) => update('konfigurasi', konfigurasi)} /><BranchingBuilder question={question} config={config} sections={sections} sectionId={sectionId} onChange={(konfigurasi: any) => update('konfigurasi', konfigurasi)} /><CanvasStimulusEditor items={question.stimulus || []} onChange={(stimulus: Row[]) => update('stimulus', stimulus)} /><div className="grid gap-3 border-t pt-4 sm:grid-cols-2"><Field label="Skor"><Input type="number" min="0.1" step="0.1" value={question.bobot} onChange={(event) => update('bobot', event.target.value)} /></Field><label className="flex min-h-11 items-center gap-2 rounded-xl border px-3 text-sm"><Checkbox checked={question.wajibDijawab !== false} onChange={(event) => update('wajibDijawab', event.currentTarget.checked)} />Wajib dijawab</label><Field label="Gambar stimulus"><label className="flex h-10 cursor-pointer items-center gap-2 rounded-xl border border-dashed px-3 text-sm hover:bg-muted/50"><ImageIcon className="h-4 w-4 text-primary" />{imageCount ? `${imageCount} gambar terpasang` : 'Unggah PNG/JPG'}<input className="sr-only" type="file" accept="image/png,image/jpeg" onChange={(event) => { const file = event.target.files?.[0]; if (file) onUpload(file); event.currentTarget.value = '' }} /></label></Field></div><details className="rounded-xl bg-muted/50 p-3"><summary className="cursor-pointer text-sm font-medium">Metadata guru (opsional)</summary><div className="mt-3 grid gap-3 sm:grid-cols-2"><Field label="Topik"><Input value={question.topik} onChange={(event) => update('topik', event.target.value)} /></Field><Field label="Kompetensi"><Input value={question.kompetensi} onChange={(event) => update('kompetensi', event.target.value)} /></Field><Field label="Pembahasan internal" wide><textarea className="min-h-20 rounded-xl border border-input bg-background p-3 text-sm" value={question.pembahasan} onChange={(event) => update('pembahasan', event.target.value)} /></Field></div></details></div></Card>
}

function BranchingBuilder({ question, config, sections, sectionId, onChange }: { question: Row; config: Row; sections: Row[]; sectionId: string; onChange: (config: Row) => void }) {
  if (!['pg_tunggal', 'dropdown'].includes(question.tipe) || sections.length < 2 || !sectionId) return null
  const sourceIndex = sections.findIndex((section) => section.id === sectionId)
  const choices = config.choices || []
  const routes = config.branchToByAnswer || {}
  if (sourceIndex < 0 || choices.length === 0) return null
  const setRoute = (choiceId: string, target: string) => {
    const next = { ...routes }
    if (target) next[choiceId] = target
    else delete next[choiceId]
    onChange({ ...config, branchToByAnswer: Object.keys(next).length ? next : undefined })
  }
  return <details className="rounded-xl border border-blue-200 bg-blue-50/50 p-3">
    <summary className="min-h-10 cursor-pointer py-2 text-sm font-semibold text-blue-950">Alur berdasarkan jawaban <span className="font-normal text-blue-800">(opsional)</span></summary>
    <p className="mb-3 text-xs leading-relaxed text-blue-900">Arahkan siswa ke bagian tertentu berdasarkan pilihan. Jika dibiarkan, siswa lanjut ke bagian berikutnya. Bagian yang dilewati tidak ditampilkan atau dinilai.</p>
    <div className="space-y-2">{choices.map((choice: Row) => <div key={choice.id} className="grid gap-2 rounded-lg border border-blue-100 bg-white p-2 sm:grid-cols-[minmax(0,1fr)_minmax(180px,1fr)] sm:items-center"><span className="line-clamp-2 text-sm">{choice.text || 'Pilihan tanpa teks'}</span><Select aria-label={`Arah untuk jawaban ${choice.text || choice.id}`} value={routes[choice.id] || ''} onChange={(event) => setRoute(choice.id, event.target.value)}><option value="">Lanjut normal</option><option value="__selesai__">Akhiri simulasi</option>{sections.slice(sourceIndex + 1).map((section: Row) => <option key={section.id} value={section.id}>Lanjut ke: {section.nama}</option>)}</Select></div>)}</div>
  </details>
}
function CanvasStimulusEditor({ items, onChange }: { items: Row[]; onChange: (items: Row[]) => void }) { const content = items.filter((item) => item.jenis !== 'image'); const update = (next: Row[]) => onChange([...next, ...items.filter((item) => item.jenis === 'image')].map((item, index) => ({ ...item, urutan: index + 1 }))); const add = (jenis: string) => update([...content, { id: newID('stimulus'), jenis, konten: '', altText: '' }]); return <section className="space-y-2 border-t pt-4"><div className="flex flex-wrap items-center justify-between gap-2"><div><h4 className="font-medium">Stimulus pendukung</h4><p className="text-xs text-muted-foreground">Tambahkan bacaan, tabel visual, atau tautan media HTTPS.</p></div><div className="flex gap-1"><Button type="button" size="sm" variant="outline" onClick={() => add('text')}>Teks</Button><Button type="button" size="sm" variant="outline" onClick={() => add('table')}>Tabel</Button><Button type="button" size="sm" variant="outline" onClick={() => add('media_link')}>Media</Button></div></div>{content.map((item, index) => <div key={item.id || index} className="rounded-xl border p-3"><div className="mb-2 flex items-center justify-between text-xs font-medium"><span>{item.jenis === 'table' ? 'Tabel' : item.jenis === 'media_link' ? 'Tautan media' : 'Teks stimulus'}</span><Button type="button" size="icon" variant="ghost" onClick={() => update(content.filter((_: Row, itemIndex: number) => itemIndex !== index))}><Trash2 className="h-4 w-4" /></Button></div>{item.jenis === 'table' ? <TableGrid value={item.konten} onChange={(konten) => update(content.map((row: Row, itemIndex: number) => itemIndex === index ? { ...row, konten } : row))} /> : <textarea className="min-h-20 w-full rounded-lg border border-input bg-background p-2 text-sm" value={item.konten} onChange={(event) => update(content.map((row: Row, itemIndex: number) => itemIndex === index ? { ...row, konten: event.target.value } : row))} placeholder={item.jenis === 'media_link' ? 'https://…' : 'Ketik stimulus di sini.'} />}</div>)}</section> }
function TableGrid({ value, onChange }: { value: string; onChange: (value: string) => void }) { const rows = (value ? value.split('\n').map((row) => row.split('\t')) : [['', ''], ['', '']]); const columns = Math.max(2, ...rows.map((row) => row.length)); const grid = rows.map((row) => Array.from({ length: columns }, (_, index) => row[index] || '')); const setGrid = (next: string[][]) => onChange(next.map((row) => row.join('\t')).join('\n')); return <div className="space-y-2"><div className="overflow-auto"><table className="min-w-full border-collapse">{grid.map((row, rowIndex) => <tbody key={rowIndex}><tr>{row.map((cell, columnIndex) => <td key={columnIndex} className="border p-1"><Input className="h-8 min-w-28" value={cell} onChange={(event) => { const next = grid.map((copy) => [...copy]); next[rowIndex][columnIndex] = event.target.value; setGrid(next) }} /></td>)}</tr></tbody>)}</table></div><div className="flex gap-2"><Button type="button" size="sm" variant="outline" onClick={() => setGrid([...grid, Array(columns).fill('')])}>+ Baris</Button><Button type="button" size="sm" variant="outline" onClick={() => setGrid(grid.map((row) => [...row, '']))}>+ Kolom</Button></div></div> }
function StudentQuestionPreview({ question, token }: { question: Row; token: string }) {
  const initialValue = question.tipe === 'pg_kompleks' ? [] : question.tipe === 'benar_salah' || question.tipe === 'menjodohkan' ? {} : ''
  const [value, setValue] = useState<any>(initialValue)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => setValue(initialValue), [question.id, question.tipe])
  const questionId = `preview-${question.id || 'draft'}`
  return <Card className="border-primary/30 bg-primary/[0.02] p-5">
    <p className="text-xs font-bold uppercase tracking-wide text-primary">Pratinjau seperti siswa</p>
    <p className="mt-3 whitespace-pre-wrap text-lg font-semibold" id={questionId}>{question.pertanyaan || 'Pertanyaan akan tampil di sini'}</p>
    {(question.stimulus || []).length > 0 && <div className="mt-4 rounded-xl border bg-white p-3"><StimulusContent items={question.stimulus} token={token} compact /></div>}
    <div className="mt-4"><QuestionAnswerControl question={question} questionId={questionId} value={value} onChange={setValue} token={token} /></div>
    <p className="mt-4 text-xs text-muted-foreground">Ini pratinjau interaktif; kunci jawaban dan pembahasan tidak ditampilkan.</p>
  </Card>
}

function VisualQuestionForm({ token, mapel, onCancel, onSaved }: { token: string; mapel: Row[]; onCancel: () => void; onSaved: () => void }) {
  const [question, setQuestion] = useState<Row>({ ...canvasQuestion(), id: 'draft-question' })
  const [mapelId, setMapelId] = useState('')
  const [image, setImage] = useState<File | null>(null)
  const [saving, setSaving] = useState(false)
  const pendingChoiceImages = useRef<Record<string, { file: File; altText: string }>>({})
  const savedQuestionID = useRef('')

  const save = async (event: FormEvent) => {
    event.preventDefault()
    if (!question.pertanyaan.trim()) { toast.error('Tulis pertanyaan terlebih dahulu.'); return }
    setSaving(true)
    try {
      const payload = {
        ...question,
        mapelId: mapelId || null,
        status: 'draf',
        bobot: Number(question.bobot || 1),
        stimulus: (question.stimulus || []).filter((stimulus: Row) => stimulus.jenis !== 'image'),
      }
      const created = savedQuestionID.current
        ? await request(`/simulasi/soal/${savedQuestionID.current}`, token, 'PUT', payload)
        : await request('/simulasi/soal', token, 'POST', payload)
      savedQuestionID.current = created.id
      if (image) {
        const form = new FormData()
        form.append('file', image)
        form.append('altText', `Stimulus untuk ${question.pertanyaan}`)
        const response = await fetch(`${apiBase}/simulasi/soal/${created.id}/stimulus/gambar`, { method: 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: form })
        if (!response.ok) { const body = await response.json().catch(() => ({})); throw new Error(body.error || 'Gambar stimulus gagal diunggah.') }
        setImage(null)
      }
      const validChoiceIDs = new Set((question.konfigurasi?.choices || []).map((choice: Row) => String(choice.id)))
      for (const [choiceID, pendingImage] of Object.entries(pendingChoiceImages.current)) {
        if (!validChoiceIDs.has(choiceID)) { delete pendingChoiceImages.current[choiceID]; continue }
        const form = new FormData()
        form.append('file', pendingImage.file)
        form.append('altText', pendingImage.altText)
        const response = await fetch(`${apiBase}/simulasi/soal/${created.id}/opsi/${encodeURIComponent(choiceID)}/gambar`, { method: 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: form })
        const body = await response.json().catch(() => ({}))
        if (!response.ok) throw new Error(body.error || `Gambar pilihan ${choiceID} belum dapat diunggah.`)
        delete pendingChoiceImages.current[choiceID]
        setQuestion((current) => ({ ...current, konfigurasi: { ...current.konfigurasi, choices: (current.konfigurasi?.choices || []).map((choice: Row) => choice.id === choiceID ? { ...choice, imageId: body.id, imageAltText: pendingImage.altText } : choice) } }))
      }
      toast.success('Soal visual disimpan sebagai draf di Bank Soal.')
      onSaved()
    } catch (error: any) {
      toast.error(error.message || 'Soal belum dapat disimpan.')
    } finally {
      setSaving(false)
    }
  }

  const item = { soal: { ...question, mapelId } }
  return <FormCard title="Buat soal visual" description="Tulis soal, pilih tipe, susun jawaban, lalu simpan sebagai draf. Tidak perlu JSON atau syntax.">
    <form className="space-y-4" onSubmit={save}>
      <div className="grid gap-3 rounded-xl border bg-muted/20 p-3 sm:grid-cols-2">
        <Field label="Mode"><Select value={question.mode} onChange={(event) => setQuestion((value) => ({ ...value, mode: event.target.value }))}><option value="anbk_akm">ANBK / AKM</option><option value="tka_sd">TKA SD</option></Select></Field>
        <Field label="Mata pelajaran"><Select value={mapelId} onChange={(event) => setMapelId(event.target.value)}><option value="">Pilih mapel (opsional)</option>{mapel.map((item: Row) => <option key={item.id} value={item.id}>{item.namaMapel}</option>)}</Select></Field>
        <Field label="Jenjang"><Input value={question.jenjang} onChange={(event) => setQuestion((value) => ({ ...value, jenjang: event.target.value }))} /></Field>
        <Field label="Kelas / fase"><Input value={question.kelasFase} onChange={(event) => setQuestion((value) => ({ ...value, kelasFase: event.target.value }))} /></Field>
      </div>
      <CanvasQuestionCard item={item} index={0} token={token} onChange={(next: Row) => setQuestion((value) => ({ ...value, ...next, mapelId }))} onDelete={() => undefined} onCopy={() => undefined} onUpload={(file: File) => { setImage(file); toast.message('Gambar dipilih dan akan diunggah saat soal disimpan.') }} onUploadChoiceImage={async (choiceID: string, file: File, altText: string) => { pendingChoiceImages.current[choiceID] = { file, altText }; toast.message('Gambar pilihan akan diunggah saat soal disimpan.'); return null }} />
      {image && <p className="rounded-lg bg-primary/5 p-3 text-sm text-primary">Gambar siap diunggah: {image.name}</p>}
      <div className="flex flex-wrap justify-end gap-2 border-t pt-4"><Button type="button" variant="outline" onClick={onCancel}>Batal</Button><Button disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan draf soal'}</Button></div>
    </form>
  </FormCard>
}

function PackageForm({ form, setForm, mapel, saving, onCancel, onSubmit }: any) { const update = (key: string, value: any) => setForm((current: any) => ({ ...current, [key]: value })); const check = (key: string) => <label className="flex items-center gap-2 text-sm"><Checkbox checked={form[key]} onChange={(event) => update(key, event.currentTarget.checked)} />{key === 'acakUrutan' ? 'Acak urutan per siswa' : key === 'tampilkanNilai' ? 'Tampilkan nilai' : key === 'tampilkanRingkasan' ? 'Tampilkan ringkasan' : key === 'izinkanEditRespons' ? 'Izinkan ubah respons setelah dikirim' : 'Tampilkan pembahasan'}</label>; return <FormCard title="Buat paket simulasi" description="Paket tersimpan sebagai draf hingga berisi soal dan peserta."><form className="grid gap-3 md:grid-cols-2" onSubmit={onSubmit}><Field label="Nama paket"><Input value={form.nama} onChange={(event) => update('nama', event.target.value)} required /></Field><Field label="Mode"><Select value={form.mode} onChange={(event) => update('mode', event.target.value)}><option value="anbk_akm">Simulasi ANBK/AKM</option><option value="tka_sd">Simulasi TKA SD</option></Select></Field><Field label="Jenjang"><Input value={form.jenjang} onChange={(event) => update('jenjang', event.target.value)} required /></Field><Field label="Mata pelajaran"><Select value={form.mapelId} onChange={(event) => update('mapelId', event.target.value)}><option value="">Pilih mapel (opsional)</option>{mapel.map((item: Row) => <option key={item.id} value={item.id}>{item.namaMapel}</option>)}</Select></Field><Field label="Durasi (menit)"><Input type="number" min="1" max="360" value={form.durasiMenit} onChange={(event) => update('durasiMenit', event.target.value)} required /></Field><Field label="Maks. percobaan"><Input type="number" min="1" max="10" value={form.maksPercobaan} onChange={(event) => update('maksPercobaan', event.target.value)} /></Field><Field label="Mulai (opsional)"><Input type="datetime-local" value={form.waktuMulai} onChange={(event) => update('waktuMulai', event.target.value)} /></Field><Field label="Selesai (opsional)"><Input type="datetime-local" value={form.waktuSelesai} onChange={(event) => update('waktuSelesai', event.target.value)} /></Field><Field label="Nilai lulus (opsional)"><Input type="number" min="0" max="100" value={form.nilaiLulus} onChange={(event) => update('nilaiLulus', event.target.value)} /></Field><Field label="Pengaturan"><div className="grid gap-2 pt-2">{check('acakUrutan')}{check('tampilkanNilai')}{check('tampilkanRingkasan')}{check('tampilkanPembahasan')}{check('izinkanEditRespons')}</div></Field><Field label="Deskripsi" wide><textarea className="min-h-20 rounded-xl border border-input bg-background p-3 text-sm" value={form.deskripsi} onChange={(event) => update('deskripsi', event.target.value)} /></Field><Field label="Instruksi peserta" wide><textarea className="min-h-24 rounded-xl border border-input bg-background p-3 text-sm" value={form.instruksi} onChange={(event) => update('instruksi', event.target.value)} /></Field><div className="flex gap-2"><Button disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan draf'}</Button><Button type="button" variant="outline" onClick={onCancel}>Batal</Button></div></form></FormCard> }
function PackageDetail({ packet, token, currentUser, items, questions, classes, students, assigned, setAssigned, results, shareTokens, detail, writable, canGrade, onAttach, onAssign, onPublish, onDuplicate, onInspect, onGrade, onCreateShare, onRevokeShare, onPreview, onExport, onExportXlsx }: any) {
  const [collaboratorRole, setCollaboratorRole] = useState(currentUser?.role === 'admin' ? 'admin' : packet.dibuatOlehUserId === currentUser?.id ? 'owner' : '')
  useEffect(() => { setCollaboratorRole(currentUser?.role === 'admin' ? 'admin' : packet.dibuatOlehUserId === currentUser?.id ? 'owner' : '') }, [packet.id, packet.dibuatOlehUserId, currentUser?.id, currentUser?.role])
  useEffect(() => {
    let active = true
    request(`/assessment/simulasi/${encodeURIComponent(packet.id)}/collaborators`, token)
      .then((data) => { if (active) setCollaboratorRole(String(data?.currentRole || '')) })
      .catch(() => {})
    return () => { active = false }
  }, [packet.id, token])
  const canManagePackage = canGrade && ['admin', 'owner', 'editor'].includes(collaboratorRole)
  const canGradeAnswers = canGrade && ['admin', 'owner', 'editor', 'grader'].includes(collaboratorRole)
  return <div className="space-y-4">
    <div className="flex flex-wrap items-start justify-between gap-2"><div><h3 className="font-bold">{packet.nama}</h3><p className="text-xs text-muted-foreground">{items.length} soal · {assigned.length} peserta · status {packet.status}</p></div><div className="flex flex-wrap gap-2"><AssessmentCollaboratorManager token={token} module="simulasi" assessmentId={packet.id} onRoleChange={setCollaboratorRole} /><Button size="sm" variant="outline" onClick={onPreview}><ExternalLink className="h-3.5 w-3.5" /> Pratinjau</Button><Button size="sm" variant="outline" disabled={!canManagePackage} onClick={onDuplicate}><Copy className="h-3.5 w-3.5" /> Duplikasi</Button>{results.length > 0 && <><Button size="sm" variant="outline" onClick={onExport}><Download className="h-3.5 w-3.5" /> CSV</Button><Button size="sm" variant="outline" onClick={onExportXlsx}><Download className="h-3.5 w-3.5" /> XLSX</Button></>}{writable && <Button size="sm" onClick={onPublish}><Send className="h-3.5 w-3.5" /> Terbitkan</Button>}</div></div>
    {packet.status === 'terbit' && <ShareTokenPanel tokens={shareTokens || []} items={items} canManage={canManagePackage} onCreate={onCreateShare} onRevoke={onRevokeShare} />}
    {writable && <><section><h4 className="mb-2 text-sm font-semibold">Tambahkan soal terbit</h4><div className="max-h-48 space-y-2 overflow-auto rounded-xl border p-2">{questions.map((question: Row) => <div key={question.id} className="flex items-center justify-between gap-2 rounded-lg bg-muted/30 p-2 text-xs"><span className="line-clamp-2">{question.pertanyaan}</span><Button size="sm" variant="outline" onClick={() => void onAttach(question)}>Tambah</Button></div>)}{!questions.length && <p className="p-2 text-xs text-muted-foreground">Tidak ada soal terbit yang belum dipilih.</p>}</div></section><section><div className="mb-2 flex items-center justify-between"><h4 className="text-sm font-semibold">Tugaskan peserta</h4><Button size="sm" variant="outline" onClick={() => void onAssign()}>Simpan penugasan</Button></div><ParticipantPicker students={students} classes={classes || []} selectedIds={assigned} onChange={setAssigned} /></section></>}
    <section><h4 className="mb-2 text-sm font-semibold">Susunan soal</h4><ol className="space-y-1 text-xs">{items.map((item: Row) => <li key={item.id} className="rounded-lg bg-muted/30 p-2">{item.urutan}. {item.soal?.pertanyaan} <span className="text-muted-foreground">({item.bobot} poin)</span></li>)}{!items.length && <p className="text-xs text-muted-foreground">Belum ada soal.</p>}</ol></section>
    <section><h4 className="mb-2 text-sm font-semibold">Hasil peserta</h4>{results.length ? <div className="space-y-1">{results.map((result: Row) => <div key={result.id} className="flex items-center justify-between gap-2 rounded-lg bg-muted/30 p-2 text-xs"><span>{result.pesertaDidik?.nama}</span><span>{result.status} · {result.skorAkhir ?? result.skorOtomatis ?? '-'}</span><Button size="sm" variant="outline" onClick={() => void onInspect(result)}>Detail</Button></div>)}</div> : <p className="text-xs text-muted-foreground">Belum ada upaya peserta.</p>}</section>
    {detail && <section className="space-y-3 rounded-xl border p-3"><div><h4 className="text-sm font-semibold">Detail jawaban: {detail.pesertaDidik?.nama}</h4><p className="text-xs text-muted-foreground">Kunci dan rubrik hanya tampil untuk staf. Soal pada cabang yang tidak dipilih tetap tercatat untuk audit.</p></div>{(detail.items || []).map((item: Row) => <div key={item.upayaSoalId} className={`rounded-lg p-3 text-sm ${item.aktif === false ? 'border border-dashed bg-muted/10 opacity-70' : 'bg-muted/30'}`}><p className="flex flex-wrap items-center gap-2 font-medium">{item.urutan}. {item.soal?.pertanyaan}{item.aktif === false && <Badge variant="secondary">Dilewati</Badge>}</p>{item.aktif !== false && <><p className="mt-1 whitespace-pre-wrap text-xs text-muted-foreground">Jawaban: {typeof item.jawaban?.jawabanJson === 'string' ? item.jawaban.jawabanJson : item.files?.length ? `${item.files.length} berkas diunggah` : '-'}</p>{item.files?.length > 0 && <ul className="mt-2 space-y-1">{item.files.map((file: Row) => <li key={file.id}><Button size="sm" variant="outline" onClick={() => void downloadFile(`/simulasi/upaya/${detail.upaya.id}/file/${file.id}`, token, file.namaFile)}><Download className="h-3.5 w-3.5" />{file.namaFile} <span className="text-xs text-muted-foreground">({(Number(file.ukuran || 0) / 1024 / 1024).toFixed(2)} MB)</span></Button></li>)}</ul>}{item.revisions?.length > 0 && <details className="mt-3 rounded-lg border bg-white p-3"><summary className="cursor-pointer text-xs font-semibold">Riwayat perubahan respons ({item.revisions.length})</summary><ol className="mt-3 space-y-2">{item.revisions.map((revision: Row) => <li key={revision.nomor} className="rounded-md bg-muted/30 p-2 text-xs"><p className="font-semibold">Revisi {revision.nomor} · {new Date(revision.dibuatPada).toLocaleString('id-ID')}</p><p className="mt-1 text-muted-foreground">Sebelum: <code className="break-all">{JSON.stringify(revision.jawabanSebelum)}</code></p><p className="text-muted-foreground">Sesudah: <code className="break-all">{JSON.stringify(revision.jawabanSesudah)}</code></p></li>)}</ol></details>}{['uraian', 'unggah_berkas'].includes(item.soal?.tipe) && item.jawaban?.id && <ManualGrade answer={item.jawaban} max={item.bobot} canGrade={canGradeAnswers} onGrade={onGrade} />}</>}</div>)}</section>}
  </div>
}
function ShareTokenPanel({ tokens, items, canManage, onCreate, onRevoke }: { tokens: Row[]; items: Row[]; canManage: boolean; onCreate: (options?: { prefill?: Record<string, unknown>; embedOrigins?: string[] }) => Promise<Row | undefined>; onRevoke: (id: string) => void }) {
  const [prefillOpen, setPrefillOpen] = useState(false)
  const [prefillValues, setPrefillValues] = useState<Record<string, unknown>>({})
  const [enabledItems, setEnabledItems] = useState<Record<string, boolean>>({})
  const [embedOpen, setEmbedOpen] = useState(false)
  const [embedOrigin, setEmbedOrigin] = useState('')
  const [embedCode, setEmbedCode] = useState('')
  const copy = async (value: string) => { try { await navigator.clipboard.writeText(value); toast.success('Tautan disalin.') } catch { toast.error('Tautan tidak dapat disalin otomatis.') } }
  const availableItems = items.filter((item) => item.soal?.tipe !== 'unggah_berkas')
  const selectedPrefill = Object.fromEntries(Object.entries(prefillValues).filter(([id, value]) => enabledItems[id] && value !== undefined && value !== null && value !== '' && !(Array.isArray(value) && value.length === 0) && !(typeof value === 'object' && !Array.isArray(value) && Object.keys(value as object).length === 0)))
  const hasIncompletePrefill = Object.entries(enabledItems).some(([id, enabled]) => enabled && !Object.hasOwn(selectedPrefill, id))
  const closePrefill = (open: boolean) => { setPrefillOpen(open); if (!open) { setPrefillValues({}); setEnabledItems({}) } }
  return <>
    <section className="rounded-xl border border-primary/20 bg-primary/[0.03] p-3"><div className="flex flex-wrap items-start justify-between gap-2"><div><h4 className="flex items-center gap-2 text-sm font-semibold"><KeyRound className="h-4 w-4 text-primary" />Tautan pengerjaan siswa</h4><p className="mt-1 text-xs text-muted-foreground">Buat link aman. Siswa tetap harus masuk dengan akun siswa.</p></div>{canManage && <div className="flex flex-wrap gap-2"><Button size="sm" variant="outline" onClick={() => void onCreate()}><Link2 className="h-3.5 w-3.5" />Buat tautan</Button><Button size="sm" variant="outline" onClick={() => { setEmbedCode(''); setEmbedOpen(true) }}><ExternalLink className="h-3.5 w-3.5" />Kode semat</Button><Button size="sm" onClick={() => setPrefillOpen(true)}><TextCursorInput className="h-3.5 w-3.5" />Tautan isian awal</Button></div>}</div>{tokens.length > 0 && <div className="mt-3 space-y-2">{tokens.map((row) => <div key={row.id} className="flex flex-wrap items-center gap-2 rounded-lg border bg-white p-2 text-xs"><span className="font-mono text-muted-foreground">…{row.tokenPrefix}</span><Badge variant={row.status === 'aktif' ? 'default' : 'secondary'}>{row.status}</Badge>{row.expiresAt && <span className="text-muted-foreground">s.d. {new Date(row.expiresAt).toLocaleDateString('id-ID')}</span>}{row.url ? <Input className="min-w-0 w-full flex-1 font-mono text-[11px] sm:min-w-[220px]" readOnly value={row.url} onFocus={(event) => event.currentTarget.select()} aria-label="Tautan pengerjaan" /> : <span className="text-muted-foreground">Link hanya dapat disalin saat dibuat.</span>}<div className="ml-auto flex gap-1">{row.url && <Button size="sm" variant="outline" onClick={() => void copy(row.url)}><Copy className="h-3.5 w-3.5" />Salin</Button>}{canManage && row.status === 'aktif' && <Button size="sm" variant="outline" onClick={() => onRevoke(row.id)}>Cabut</Button>}</div></div>)}</div>}{!tokens.length && <p className="mt-3 text-xs text-muted-foreground">Belum ada tautan aktif.</p>}</section>
    <Dialog open={prefillOpen} onOpenChange={closePrefill}><DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto"><DialogHeader><DialogTitle>Buat tautan dengan isian awal</DialogTitle><DialogDescription>Pilih jawaban yang ingin disiapkan sebelumnya. Nilai isian disimpan aman di server dan tidak dimasukkan ke alamat tautan. Setiap siswa tetap mengerjakan dengan akunnya sendiri.</DialogDescription></DialogHeader><div className="space-y-3">{availableItems.map((item, index) => { const question = item.soal || {}; const enabled = Boolean(enabledItems[item.id]); return <section key={item.id} className="rounded-xl border p-3"><label className="flex min-h-11 cursor-pointer items-center gap-3 font-medium"><Checkbox checked={enabled} onChange={(event) => { const checked = event.currentTarget.checked; setEnabledItems((current) => ({ ...current, [item.id]: checked })) }} /><span>{index + 1}. {question.pertanyaan || 'Soal tanpa judul'}</span></label><p className="ml-8 text-xs text-muted-foreground">{questionTypes.find(([id]) => id === question.tipe)?.[1] || question.tipe}</p>{enabled && <div className="mt-3 border-t pt-3"><QuestionAnswerControl question={question} questionId={`prefill-${item.id}`} value={prefillValues[item.id] ?? null} onChange={(value) => setPrefillValues((current) => ({ ...current, [item.id]: value }))} /></div>}</section>})}{!availableItems.length && <p className="rounded-lg bg-muted/40 p-3 text-sm text-muted-foreground">Tidak ada tipe soal yang mendukung isian awal pada paket ini. Soal unggah berkas tidak dapat dipraisi.</p>}</div><div className="flex flex-wrap justify-end gap-2 border-t pt-4"><Button variant="outline" onClick={() => closePrefill(false)}>Batal</Button><Button disabled={!canManage || !Object.keys(selectedPrefill).length || hasIncompletePrefill} onClick={() => { void onCreate({ prefill: selectedPrefill }); closePrefill(false) }}>Buat tautan aman</Button></div>{hasIncompletePrefill && <p role="alert" className="text-sm text-amber-700">Lengkapi nilai pada setiap soal yang dipilih, atau matikan pilihan tersebut.</p>}</DialogContent></Dialog>
    <Dialog open={embedOpen} onOpenChange={(open) => { setEmbedOpen(open); if (!open) { setEmbedOrigin(''); setEmbedCode('') } }}><DialogContent className="max-w-2xl"><DialogHeader><DialogTitle>{embedCode ? 'Kode semat simulasi' : 'Sematkan simulasi'}</DialogTitle><DialogDescription>{embedCode ? 'Salin kode berikut ke halaman sekolah. Iframe hanya dapat dibuka dari domain yang Anda izinkan; siswa tetap harus login.' : 'Masukkan origin halaman tempat simulasi akan ditampilkan. Gunakan HTTPS dan domain saja, tanpa path (contoh: https://kelas.sekolah.sch.id). Tautan tetap meminta siswa login.'}</DialogDescription></DialogHeader>{embedCode ? <><textarea className="min-h-28 w-full rounded-xl border bg-muted/20 p-3 font-mono text-xs" value={embedCode} readOnly aria-label="Kode iframe" onFocus={(event) => event.currentTarget.select()} /><div className="flex justify-end gap-2"><Button variant="outline" onClick={() => setEmbedCode('')}>Kembali</Button><Button onClick={() => void copy(embedCode)}>Salin kode</Button></div></> : <><Field label="Origin domain sekolah"><Input type="url" placeholder="https://kelas.sekolah.sch.id" value={embedOrigin} onChange={(event) => setEmbedOrigin(event.target.value)} /></Field><div className="flex justify-end gap-2"><Button variant="outline" onClick={() => setEmbedOpen(false)}>Batal</Button><Button disabled={!embedOrigin.trim()} onClick={() => { void (async () => { const created = await onCreate({ embedOrigins: [embedOrigin.trim()] }); if (!created?.url) return; const safeSrc = new URL(created.url, window.location.origin).toString().replaceAll('&', '&amp;').replaceAll('"', '&quot;'); setEmbedCode(`<iframe src="${safeSrc}" title="Simulasi asesmen" width="100%" height="720" loading="lazy" referrerpolicy="strict-origin-when-cross-origin" style="border:0;max-width:100%;"></iframe>`) })() }}>Buat kode semat</Button></div></>}</DialogContent></Dialog>
  </>
}
function ManualGrade({ answer, max, canGrade, onGrade }: { answer: Row; max: number; canGrade: boolean; onGrade: (answer: Row, score: number, note: string) => void }) { const [score, setScore] = useState(answer.skorManual ?? ''); const [note, setNote] = useState(answer.komentarGuru ?? ''); return <div className="mt-3 flex flex-wrap items-end gap-2"><label className="grid gap-1 text-xs">Skor (maks. {max})<Input className="h-8 w-24" type="number" min="0" max={max} step="0.1" value={score} onChange={(event) => setScore(event.target.value)} disabled={!canGrade} /></label><label className="grid flex-1 gap-1 text-xs">Komentar<Input className="h-8" value={note} onChange={(event) => setNote(event.target.value)} disabled={!canGrade} /></label>{canGrade && <Button size="sm" onClick={() => void onGrade(answer, Number(score), note)}>Simpan nilai</Button>}</div> }
function Field({ label, children, wide = false }: { label: string; children: ReactNode; wide?: boolean }) {
  const id = useId()
  const labelID = `field-label-${id}`
  const childIsElement = isValidElement(children)
  const childProps = childIsElement ? (children.props as Record<string, unknown>) : {}
  const isGroup = childIsElement && typeof children.type === 'string' && ['div', 'fieldset'].includes(children.type)
  const control = childIsElement
    ? cloneElement(children as React.ReactElement<any>, isGroup
      ? { 'aria-labelledby': labelID }
      : { id: childProps.id || id, 'aria-labelledby': labelID })
    : children
  return <div className={`grid gap-1.5 ${wide ? 'md:col-span-2' : ''}`}><Label id={labelID} htmlFor={isGroup ? undefined : id}>{label}</Label>{control}</div>
}
