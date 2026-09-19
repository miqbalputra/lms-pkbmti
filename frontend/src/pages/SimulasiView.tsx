import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { ClipboardCheck, Copy, Download, FileUp, Plus, Send } from 'lucide-react'
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
import type { User } from '../App'
import { apiBase, downloadFile, request } from '../lib/api'

type Row = Record<string, any> & { id: string }
type QuestionType = 'pg_tunggal' | 'pg_kompleks' | 'benar_salah' | 'menjodohkan' | 'isian_singkat' | 'uraian'

const questionTypes: Array<[QuestionType, string]> = [
  ['pg_tunggal', 'Pilihan ganda'], ['pg_kompleks', 'Pilihan ganda kompleks'], ['benar_salah', 'Benar / Salah'],
  ['menjodohkan', 'Menjodohkan'], ['isian_singkat', 'Isian singkat'], ['uraian', 'Uraian berubrik'],
]
const configExample = (type: QuestionType) => {
  if (type === 'pg_tunggal') return { choices: [{ id: 'a', text: 'Pilihan A' }, { id: 'b', text: 'Pilihan B' }], correctIds: ['a'] }
  if (type === 'pg_kompleks') return { choices: [{ id: 'a', text: 'Pernyataan A' }, { id: 'b', text: 'Pernyataan B' }, { id: 'c', text: 'Pernyataan C' }], correctIds: ['a', 'c'] }
  if (type === 'benar_salah') return { statements: [{ id: 'p1', text: 'Pernyataan pertama', correct: true }, { id: 'p2', text: 'Pernyataan kedua', correct: false }] }
  if (type === 'menjodohkan') return { left: [{ id: 'l1', text: 'Kolom kiri 1' }, { id: 'l2', text: 'Kolom kiri 2' }], right: [{ id: 'r1', text: 'Kolom kanan 1' }, { id: 'r2', text: 'Kolom kanan 2' }], pairs: { l1: 'r2', l2: 'r1' } }
  if (type === 'isian_singkat') return { acceptedAnswers: ['jawaban contoh', 'jawaban alternatif'] }
  return { rubrik: [{ kriteria: 'Ketepatan isi', maks: 3 }, { kriteria: 'Kejelasan penjelasan', maks: 2 }] }
}
const emptyQuestion = () => ({ jenjang: 'SD/MI', kelasFase: 'Kelas 5-6 / Fase C', mode: 'anbk_akm', domain: '', topik: '', kompetensi: '', levelKognitif: '', tingkatKesulitan: 'sedang', tags: '', tipe: 'pg_tunggal' as QuestionType, pertanyaan: '', konfigurasi: JSON.stringify(configExample('pg_tunggal'), null, 2), pembahasan: '', bobot: '1', status: 'draf', stimulus: '[]', mapelId: '' })
const emptyPaket = () => ({ nama: '', deskripsi: '', mode: 'anbk_akm', jenjang: 'SD/MI', mapelId: '', durasiMenit: '60', instruksi: 'Baca setiap soal dengan teliti. Kamu boleh menandai soal untuk diperiksa kembali.', nilaiLulus: '', waktuMulai: '', waktuSelesai: '', maksPercobaan: '1', acakUrutan: true, tampilkanNilai: true, tampilkanRingkasan: true, tampilkanPembahasan: false })

export function SimulasiView({ token, user }: { token: string; user: User }) {
  const [questions, setQuestions] = useState<Row[]>([])
  const [packages, setPackages] = useState<Row[]>([])
  const [mapel, setMapel] = useState<Row[]>([])
  const [students, setStudents] = useState<Row[]>([])
  const [questionOpen, setQuestionOpen] = useState(false)
  const [packageOpen, setPackageOpen] = useState(false)
  const [questionForm, setQuestionForm] = useState(emptyQuestion)
  const [stimulusImage, setStimulusImage] = useState<File | null>(null)
  const [packageForm, setPackageForm] = useState(emptyPaket)
  const [selectedPackage, setSelectedPackage] = useState<Row | null>(null)
  const [items, setItems] = useState<Row[]>([])
  const [assigned, setAssigned] = useState<string[]>([])
  const [results, setResults] = useState<Row[]>([])
  const [attemptDetail, setAttemptDetail] = useState<Row | null>(null)
  const [saving, setSaving] = useState(false)

  const canWrite = user.role === 'admin' || user.role === 'guru'
  const load = () => {
    void Promise.all([
      request('/simulasi/soal', token), request('/simulasi/paket', token), request('/mapel', token), request('/peserta-didik', token).catch(() => []),
    ]).then(([q, p, m, s]) => { setQuestions(Array.isArray(q) ? q : []); setPackages(Array.isArray(p) ? p : []); setMapel(Array.isArray(m) ? m : []); setStudents(Array.isArray(s) ? s : []) }).catch((error) => toast.error(String(error.message || error)))
  }
  useEffect(load, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  async function saveQuestion(event: FormEvent) {
    event.preventDefault(); setSaving(true)
    try {
      const konfigurasi = JSON.parse(questionForm.konfigurasi)
      const stimulus = JSON.parse(questionForm.stimulus || '[]')
      const created = await request('/simulasi/soal', token, 'POST', { ...questionForm, mapelId: questionForm.mapelId || null, bobot: Number(questionForm.bobot), konfigurasi, stimulus })
      if (stimulusImage) {
        const form = new FormData(); form.append('file', stimulusImage); form.append('altText', `Stimulus untuk ${questionForm.pertanyaan}`)
        const response = await fetch(`${apiBase}/simulasi/soal/${created.id}/stimulus/gambar`, { method: 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: form })
        if (!response.ok) { const body = await response.json().catch(() => ({})); throw new Error(body.error || 'Gambar stimulus gagal diunggah.') }
      }
      toast.success('Soal simulasi disimpan.'); setQuestionOpen(false); setQuestionForm(emptyQuestion()); setStimulusImage(null); load()
    } catch (error: any) { toast.error(error.message || 'Konfigurasi soal belum valid.') } finally { setSaving(false) }
  }
  async function savePackage(event: FormEvent) {
    event.preventDefault(); setSaving(true)
    try {
      const toTime = (value: string) => value ? new Date(value).toISOString() : null
      await request('/simulasi/paket', token, 'POST', { ...packageForm, mapelId: packageForm.mapelId || null, durasiMenit: Number(packageForm.durasiMenit), maksPercobaan: Number(packageForm.maksPercobaan), nilaiLulus: packageForm.nilaiLulus ? Number(packageForm.nilaiLulus) : null, waktuMulai: toTime(packageForm.waktuMulai), waktuSelesai: toTime(packageForm.waktuSelesai) })
      toast.success('Paket draf dibuat. Tambahkan soal dan peserta sebelum menerbitkan.'); setPackageOpen(false); setPackageForm(emptyPaket()); load()
    } catch (error: any) { toast.error(error.message || 'Paket belum dapat disimpan.') } finally { setSaving(false) }
  }
  async function selectPackage(row: Row) {
    setSelectedPackage(row); setResults([])
    try {
      const [packetItems, assignments, packetResults] = await Promise.all([request(`/simulasi/paket/${row.id}/soal`, token), request(`/simulasi/paket/${row.id}/penugasan`, token), request(`/simulasi/paket/${row.id}/hasil`, token)])
      setItems(Array.isArray(packetItems) ? packetItems : []); setAssigned(Array.isArray(assignments) ? assignments.map((item: Row) => String(item.pesertaDidikId)) : []); setResults(Array.isArray(packetResults) ? packetResults : [])
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

  return <div className="space-y-4">
    <PageToolbar title="Simulasi ANBK/TKA SD" description="Buat latihan berbasis komputer dengan soal orisinal untuk literasi dan numerasi SD." actions={canWrite ? <div className="flex flex-wrap gap-2"><Button variant="outline" onClick={() => void downloadFile('/simulasi/soal/template', token, 'template-bank-soal-simulasi.xlsx')}><Download className="h-4 w-4" /> Template XLSX</Button><label className="inline-flex"><input className="sr-only" type="file" accept=".xlsx" onChange={importQuestions} /><Button type="button" variant="outline" asChild><span><FileUp className="h-4 w-4" /> Impor bank</span></Button></label><Button onClick={() => setQuestionOpen(true)}><Plus className="h-4 w-4" /> Buat soal</Button><Button variant="secondary" onClick={() => setPackageOpen(true)}><ClipboardCheck className="h-4 w-4" /> Buat paket</Button></div> : undefined} />
    {questionOpen && <QuestionForm form={questionForm} setForm={setQuestionForm} mapel={mapel} image={stimulusImage} setImage={setStimulusImage} saving={saving} onCancel={() => setQuestionOpen(false)} onSubmit={saveQuestion} />}
    {packageOpen && <PackageForm form={packageForm} setForm={setPackageForm} mapel={mapel} saving={saving} onCancel={() => setPackageOpen(false)} onSubmit={savePackage} />}
    <Tabs defaultValue="paket">
      <TabsList><TabsTrigger value="paket">Paket simulasi</TabsTrigger><TabsTrigger value="soal">Bank soal simulasi</TabsTrigger></TabsList>
      <TabsContent value="paket"><div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(380px,1.1fr)]"><Card className="overflow-hidden"><Table><TableHeader><TableRow><TableHead>Nama paket</TableHead><TableHead>Mode</TableHead><TableHead>Status</TableHead><TableHead /></TableRow></TableHeader><TableBody>{packages.map((row) => <TableRow key={row.id}><TableCell><div className="font-semibold">{row.nama}</div><div className="text-xs text-muted-foreground">{row.durasiMenit} menit · maks. {row.maksPercobaan} percobaan</div></TableCell><TableCell>{row.mode === 'tka_sd' ? 'TKA SD' : 'ANBK/AKM'}</TableCell><TableCell><Badge variant={row.status === 'terbit' ? 'default' : 'secondary'}>{row.status}</Badge></TableCell><TableCell><Button size="sm" variant="outline" onClick={() => void selectPackage(row)}>Kelola</Button></TableCell></TableRow>)}{!packages.length && <EmptyState colSpan={4} label="Belum ada paket simulasi." />}</TableBody></Table></Card>
      <Card className="p-4">{selectedPackage ? <PackageDetail packet={selectedPackage} items={items} questions={availableQuestions} students={students} assigned={assigned} setAssigned={setAssigned} results={results} detail={attemptDetail} writable={canWrite && selectedPackage.status === 'draf'} canGrade={canWrite} onAttach={attachQuestion} onAssign={assignStudents} onPublish={publish} onDuplicate={duplicate} onInspect={inspectAttempt} onGrade={gradeEssay} onExport={() => void downloadFile(`/simulasi/paket/${selectedPackage.id}/export`, token, 'hasil-simulasi.csv')} onExportXlsx={() => void downloadFile(`/simulasi/paket/${selectedPackage.id}/export?format=xlsx`, token, 'hasil-simulasi.xlsx')} /> : <EmptyState title="Pilih paket" description="Pilih paket di sebelah kiri untuk menyusun soal, menugaskan siswa, dan melihat hasil." />}</Card></div></TabsContent>
      <TabsContent value="soal"><Card className="overflow-hidden"><Table><TableHeader><TableRow><TableHead>Soal</TableHead><TableHead>Mode</TableHead><TableHead>Tipe</TableHead><TableHead>Status</TableHead><TableHead>Bobot</TableHead></TableRow></TableHeader><TableBody>{questions.map((row) => <TableRow key={row.id}><TableCell><div className="max-w-xl font-medium line-clamp-2">{row.pertanyaan}</div><div className="text-xs text-muted-foreground">{row.topik || row.domain || 'Tanpa topik'}</div></TableCell><TableCell>{row.mode === 'tka_sd' ? 'TKA SD' : 'ANBK/AKM'}</TableCell><TableCell>{questionTypes.find(([id]) => id === row.tipe)?.[1] || row.tipe}</TableCell><TableCell><Badge variant={row.status === 'terbit' ? 'default' : 'secondary'}>{row.status}</Badge></TableCell><TableCell>{row.bobot}</TableCell></TableRow>)}{!questions.length && <EmptyState colSpan={5} label="Belum ada soal simulasi." />}</TableBody></Table></Card></TabsContent>
    </Tabs>
  </div>
}

function QuestionForm({ form, setForm, mapel, image, setImage, saving, onCancel, onSubmit }: any) {
  const update = (key: string, value: any) => setForm((current: any) => ({ ...current, [key]: value }))
  return <FormCard title="Buat soal simulasi" description="Konfigurasi jawaban menggunakan JSON agar semua format asesmen dapat divalidasi server."><form className="grid gap-3 md:grid-cols-2" onSubmit={onSubmit}><Field label="Jenjang"><Input value={form.jenjang} onChange={(event) => update('jenjang', event.target.value)} required /></Field><Field label="Kelas / Fase"><Input value={form.kelasFase} onChange={(event) => update('kelasFase', event.target.value)} /></Field><Field label="Mode"><Select value={form.mode} onChange={(event) => update('mode', event.target.value)}><option value="anbk_akm">Simulasi ANBK/AKM</option><option value="tka_sd">Simulasi TKA SD</option></Select></Field><Field label="Mata pelajaran"><Select value={form.mapelId} onChange={(event) => update('mapelId', event.target.value)}><option value="">Pilih mapel (opsional)</option>{mapel.map((item: Row) => <option key={item.id} value={item.id}>{item.namaMapel}</option>)}</Select></Field><Field label="Domain"><Input value={form.domain} onChange={(event) => update('domain', event.target.value)} placeholder="Literasi membaca / Bilangan" /></Field><Field label="Topik"><Input value={form.topik} onChange={(event) => update('topik', event.target.value)} /></Field><Field label="Kompetensi"><Input value={form.kompetensi} onChange={(event) => update('kompetensi', event.target.value)} /></Field><Field label="Level kognitif"><Input value={form.levelKognitif} onChange={(event) => update('levelKognitif', event.target.value)} placeholder="Memahami / Menerapkan / Bernalar" /></Field><Field label="Tingkat kesulitan"><Select value={form.tingkatKesulitan} onChange={(event) => update('tingkatKesulitan', event.target.value)}><option value="mudah">Mudah</option><option value="sedang">Sedang</option><option value="sulit">Sulit</option></Select></Field><Field label="Tipe soal"><Select value={form.tipe} onChange={(event) => { const tipe = event.target.value as QuestionType; setForm((current: any) => ({ ...current, tipe, konfigurasi: JSON.stringify(configExample(tipe), null, 2) })) }}>{questionTypes.map(([id, label]) => <option key={id} value={id}>{label}</option>)}</Select></Field><Field label="Tag"><Input value={form.tags} onChange={(event) => update('tags', event.target.value)} placeholder="pecahan, informasi tersurat" /></Field><Field label="Bobot"><Input type="number" min="0.1" step="0.1" value={form.bobot} onChange={(event) => update('bobot', event.target.value)} /></Field><Field label="Pertanyaan" wide><textarea className="min-h-28 rounded-xl border border-input bg-background p-3 text-sm" value={form.pertanyaan} onChange={(event) => update('pertanyaan', event.target.value)} required /></Field><Field label="Konfigurasi jawaban (JSON)" wide><textarea className="min-h-52 rounded-xl border border-input bg-muted/30 p-3 font-mono text-xs" value={form.konfigurasi} onChange={(event) => update('konfigurasi', event.target.value)} required /></Field><Field label="Stimulus (JSON array, opsional)" wide><textarea className="min-h-24 rounded-xl border border-input bg-muted/30 p-3 font-mono text-xs" value={form.stimulus} onChange={(event) => update('stimulus', event.target.value)} placeholder={'[{"jenis":"text","konten":"Teks stimulus","urutan":1}]'} /></Field><Field label="Gambar stimulus (PNG/JPG, maks. 5 MB)" wide><input type="file" accept="image/png,image/jpeg" className="rounded-xl border border-input bg-background p-2 text-sm" onChange={(event) => setImage(event.target.files?.[0] || null)} /><p className="text-xs text-muted-foreground">{image ? image.name : 'Opsional; gambar ditautkan secara aman setelah soal tersimpan.'}</p></Field><Field label="Pembahasan internal" wide><textarea className="min-h-20 rounded-xl border border-input bg-background p-3 text-sm" value={form.pembahasan} onChange={(event) => update('pembahasan', event.target.value)} /></Field><Field label="Status"><Select value={form.status} onChange={(event) => update('status', event.target.value)}><option value="draf">Draf</option><option value="terbit">Terbit</option></Select></Field><div className="flex items-end gap-2"><Button disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan soal'}</Button><Button type="button" variant="outline" onClick={onCancel}>Batal</Button></div></form></FormCard>
}
function PackageForm({ form, setForm, mapel, saving, onCancel, onSubmit }: any) { const update = (key: string, value: any) => setForm((current: any) => ({ ...current, [key]: value })); const check = (key: string) => <label className="flex items-center gap-2 text-sm"><Checkbox checked={form[key]} onChange={(event) => update(key, event.currentTarget.checked)} />{key === 'acakUrutan' ? 'Acak urutan per siswa' : key === 'tampilkanNilai' ? 'Tampilkan nilai' : key === 'tampilkanRingkasan' ? 'Tampilkan ringkasan' : 'Tampilkan pembahasan'}</label>; return <FormCard title="Buat paket simulasi" description="Paket tersimpan sebagai draf hingga berisi soal dan peserta."><form className="grid gap-3 md:grid-cols-2" onSubmit={onSubmit}><Field label="Nama paket"><Input value={form.nama} onChange={(event) => update('nama', event.target.value)} required /></Field><Field label="Mode"><Select value={form.mode} onChange={(event) => update('mode', event.target.value)}><option value="anbk_akm">Simulasi ANBK/AKM</option><option value="tka_sd">Simulasi TKA SD</option></Select></Field><Field label="Jenjang"><Input value={form.jenjang} onChange={(event) => update('jenjang', event.target.value)} required /></Field><Field label="Mata pelajaran"><Select value={form.mapelId} onChange={(event) => update('mapelId', event.target.value)}><option value="">Pilih mapel (opsional)</option>{mapel.map((item: Row) => <option key={item.id} value={item.id}>{item.namaMapel}</option>)}</Select></Field><Field label="Durasi (menit)"><Input type="number" min="1" max="360" value={form.durasiMenit} onChange={(event) => update('durasiMenit', event.target.value)} required /></Field><Field label="Maks. percobaan"><Input type="number" min="1" max="10" value={form.maksPercobaan} onChange={(event) => update('maksPercobaan', event.target.value)} /></Field><Field label="Mulai (opsional)"><Input type="datetime-local" value={form.waktuMulai} onChange={(event) => update('waktuMulai', event.target.value)} /></Field><Field label="Selesai (opsional)"><Input type="datetime-local" value={form.waktuSelesai} onChange={(event) => update('waktuSelesai', event.target.value)} /></Field><Field label="Nilai lulus (opsional)"><Input type="number" min="0" max="100" value={form.nilaiLulus} onChange={(event) => update('nilaiLulus', event.target.value)} /></Field><Field label="Pengaturan"><div className="grid gap-2 pt-2">{check('acakUrutan')}{check('tampilkanNilai')}{check('tampilkanRingkasan')}{check('tampilkanPembahasan')}</div></Field><Field label="Deskripsi" wide><textarea className="min-h-20 rounded-xl border border-input bg-background p-3 text-sm" value={form.deskripsi} onChange={(event) => update('deskripsi', event.target.value)} /></Field><Field label="Instruksi peserta" wide><textarea className="min-h-24 rounded-xl border border-input bg-background p-3 text-sm" value={form.instruksi} onChange={(event) => update('instruksi', event.target.value)} /></Field><div className="flex gap-2"><Button disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan draf'}</Button><Button type="button" variant="outline" onClick={onCancel}>Batal</Button></div></form></FormCard> }
function PackageDetail({ packet, items, questions, students, assigned, setAssigned, results, detail, writable, canGrade, onAttach, onAssign, onPublish, onDuplicate, onInspect, onGrade, onExport, onExportXlsx }: any) {
  const toggle = (id: string, checked: boolean) => setAssigned((current: string[]) => checked ? [...new Set([...current, id])] : current.filter((value) => value !== id))
  return <div className="space-y-4">
    <div className="flex flex-wrap items-start justify-between gap-2"><div><h3 className="font-bold">{packet.nama}</h3><p className="text-xs text-muted-foreground">{items.length} soal · {assigned.length} peserta · status {packet.status}</p></div><div className="flex flex-wrap gap-2"><Button size="sm" variant="outline" onClick={onDuplicate}><Copy className="h-3.5 w-3.5" /> Duplikasi</Button>{results.length > 0 && <><Button size="sm" variant="outline" onClick={onExport}><Download className="h-3.5 w-3.5" /> CSV</Button><Button size="sm" variant="outline" onClick={onExportXlsx}><Download className="h-3.5 w-3.5" /> XLSX</Button></>}{writable && <Button size="sm" onClick={onPublish}><Send className="h-3.5 w-3.5" /> Terbitkan</Button>}</div></div>
    {writable && <><section><h4 className="mb-2 text-sm font-semibold">Tambahkan soal terbit</h4><div className="max-h-48 space-y-2 overflow-auto rounded-xl border p-2">{questions.map((question: Row) => <div key={question.id} className="flex items-center justify-between gap-2 rounded-lg bg-muted/30 p-2 text-xs"><span className="line-clamp-2">{question.pertanyaan}</span><Button size="sm" variant="outline" onClick={() => void onAttach(question)}>Tambah</Button></div>)}{!questions.length && <p className="p-2 text-xs text-muted-foreground">Tidak ada soal terbit yang belum dipilih.</p>}</div></section><section><div className="mb-2 flex items-center justify-between"><h4 className="text-sm font-semibold">Tugaskan peserta</h4><Button size="sm" variant="outline" onClick={() => void onAssign()}>Simpan penugasan</Button></div><div className="grid max-h-44 grid-cols-1 gap-2 overflow-auto rounded-xl border p-2 sm:grid-cols-2">{students.filter((student: Row) => student.status === 'aktif').map((student: Row) => <label key={student.id} className="flex items-center gap-2 text-xs"><Checkbox checked={assigned.includes(student.id)} onChange={(event) => toggle(student.id, event.currentTarget.checked)} />{student.nama}</label>)}</div></section></>}
    <section><h4 className="mb-2 text-sm font-semibold">Susunan soal</h4><ol className="space-y-1 text-xs">{items.map((item: Row) => <li key={item.id} className="rounded-lg bg-muted/30 p-2">{item.urutan}. {item.soal?.pertanyaan} <span className="text-muted-foreground">({item.bobot} poin)</span></li>)}{!items.length && <p className="text-xs text-muted-foreground">Belum ada soal.</p>}</ol></section>
    <section><h4 className="mb-2 text-sm font-semibold">Hasil peserta</h4>{results.length ? <div className="space-y-1">{results.map((result: Row) => <div key={result.id} className="flex items-center justify-between gap-2 rounded-lg bg-muted/30 p-2 text-xs"><span>{result.pesertaDidik?.nama}</span><span>{result.status} · {result.skorAkhir ?? result.skorOtomatis ?? '-'}</span><Button size="sm" variant="outline" onClick={() => void onInspect(result)}>Detail</Button></div>)}</div> : <p className="text-xs text-muted-foreground">Belum ada upaya peserta.</p>}</section>
    {detail && <section className="space-y-3 rounded-xl border p-3"><div><h4 className="text-sm font-semibold">Detail jawaban: {detail.pesertaDidik?.nama}</h4><p className="text-xs text-muted-foreground">Kunci dan rubrik hanya tampil untuk staf.</p></div>{(detail.items || []).map((item: Row) => <div key={item.upayaSoalId} className="rounded-lg bg-muted/30 p-3 text-sm"><p className="font-medium">{item.urutan}. {item.soal?.pertanyaan}</p><p className="mt-1 whitespace-pre-wrap text-xs text-muted-foreground">Jawaban: {typeof item.jawaban?.jawabanJson === 'string' ? item.jawaban.jawabanJson : '-'}</p>{item.soal?.tipe === 'uraian' && item.jawaban?.id && <ManualGrade answer={item.jawaban} max={item.bobot} canGrade={canGrade} onGrade={onGrade} />}</div>)}</section>}
  </div>
}
function ManualGrade({ answer, max, canGrade, onGrade }: { answer: Row; max: number; canGrade: boolean; onGrade: (answer: Row, score: number, note: string) => void }) { const [score, setScore] = useState(answer.skorManual ?? ''); const [note, setNote] = useState(answer.komentarGuru ?? ''); return <div className="mt-3 flex flex-wrap items-end gap-2"><label className="grid gap-1 text-xs">Skor (maks. {max})<Input className="h-8 w-24" type="number" min="0" max={max} step="0.1" value={score} onChange={(event) => setScore(event.target.value)} disabled={!canGrade} /></label><label className="grid flex-1 gap-1 text-xs">Komentar<Input className="h-8" value={note} onChange={(event) => setNote(event.target.value)} disabled={!canGrade} /></label>{canGrade && <Button size="sm" onClick={() => void onGrade(answer, Number(score), note)}>Simpan nilai</Button>}</div> }
function Field({ label, children, wide = false }: { label: string; children: ReactNode; wide?: boolean }) { return <div className={`grid gap-1.5 ${wide ? 'md:col-span-2' : ''}`}><Label>{label}</Label>{children}</div> }
