import { useEffect, useState, type DragEvent, type FormEvent } from 'react'
import { ArrowDown, ArrowUp, Link2, Pencil, Plus, Table2, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../components/ui/alert-dialog'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'
import { AnswerBuilder } from './SimulasiView'
import { configExample, type QuestionType } from '../components/simulasi/questionTypes'
import { QuestionAnswerControl, StimulusContent } from '../components/simulasi/QuestionAnswerControl'

type Row = Record<string, unknown> & { id: string }

function parseOpsi(v: unknown): string[] {
  if (!v) return []
  try {
    const a = JSON.parse(String(v))
    return Array.isArray(a) ? a.map(String) : []
  } catch {
    return []
  }
}

function parseStimulus(value: unknown): Row[] {
  if (Array.isArray(value)) return value as Row[]
  if (!value) return []
  try {
    const parsed: unknown = JSON.parse(String(value))
    return Array.isArray(parsed) ? parsed as Row[] : []
  } catch {
    return []
  }
}

const typeLabels: Record<string, string> = {
  pg: 'Pilihan ganda', checkbox: 'Kotak centang', dropdown: 'Dropdown',
  true_false: 'Benar / salah', short_answer: 'Jawaban singkat', essay: 'Uraian',
  pg_tunggal: 'Pilihan ganda', pg_kompleks: 'Pilihan ganda kompleks', benar_salah: 'Tabel benar / salah',
  menjodohkan: 'Menjodohkan', isian_singkat: 'Isian singkat', uraian: 'Uraian berubrik',
  skala_linear: 'Skala linear', rating: 'Rating', kisi_pg: 'Kisi pilihan tunggal',
  kisi_checkbox: 'Kisi kotak centang', tanggal: 'Tanggal', waktu: 'Waktu / durasi', susun_urutan: 'Susun urutan',
  unggah_berkas: 'Unggah berkas untuk dinilai guru',
}

const visualTypeOptions: Array<[QuestionType, string]> = [
  ['pg_tunggal', 'Pilihan ganda'], ['pg_kompleks', 'Kotak centang'], ['dropdown', 'Dropdown'],
  ['benar_salah', 'Tabel benar / salah'], ['menjodohkan', 'Menjodohkan'], ['isian_singkat', 'Jawaban singkat'],
  ['uraian', 'Paragraf / uraian berubrik'], ['skala_linear', 'Skala linear'], ['rating', 'Rating'],
  ['kisi_pg', 'Kisi pilihan tunggal'], ['kisi_checkbox', 'Kisi kotak centang'], ['tanggal', 'Tanggal'],
  ['waktu', 'Waktu / durasi'], ['susun_urutan', 'Susun urutan'], ['unggah_berkas', 'Unggah berkas untuk dinilai guru'],
]

function parseIndexes(raw: string): number[] {
  try {
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.map(Number).filter((index) => Number.isInteger(index) && index >= 0) : []
  } catch {
    return []
  }
}

function parseAcceptedAnswers(raw: unknown): string[] {
  try {
    const parsed: unknown = JSON.parse(String(raw || ''))
    return Array.isArray(parsed) ? parsed.map(String) : raw ? [String(raw)] : []
  } catch {
    return raw ? [String(raw)] : []
  }
}

const emptyForm = { mapelId: '', domain: '', topik: '', kompetensi: '', levelKognitif: '', tipe: 'pg_tunggal', pertanyaan: '', opsi: ['', ''], kunci: '0', poin: '1', konfigurasi: configExample('pg_tunggal'), stimulus: [] as Row[] }

export function BankSoalView({
  token,
  user,
  readOnly,
}: {
  token: string
  user: User
  readOnly: boolean
}) {
  const [rows, setRows] = useState<Row[]>([])
  const [mapel, setMapel] = useState<Row[]>([])
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [form, setForm] = useState({ ...emptyForm, opsi: [''] })
  const [visualMode, setVisualMode] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  const load = () => {
    void request('/bank-soal', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  function openAdd() {
    setForm({ ...emptyForm, opsi: ['', ''], konfigurasi: configExample('pg_tunggal') })
    setVisualMode(true)
    setEditing(null)
    setAdding(true)
  }

  function openEdit(r: Row) {
    setEditing(r)
    const opsi = parseOpsi(r.opsi)
    const type = String(r.tipe || 'pg')
    const key = String(r.kunci || '0')
    let configuration: any = null
    try {
      const raw = String(r.konfigurasi || '')
      if (raw) configuration = JSON.parse(raw)
    } catch { configuration = null }
    setVisualMode(Boolean(configuration))
    setForm({
      mapelId: String(r.mapelId || ''),
      domain: String(r.domain || ''),
      topik: String(r.topik || ''),
      kompetensi: String(r.kompetensi || ''),
      levelKognitif: String(r.levelKognitif || ''),
      tipe: type,
      pertanyaan: String(r.pertanyaan || ''),
      opsi: opsi.length ? opsi : type === 'true_false' ? ['Benar', 'Salah'] : ['', ''],
      kunci: type === 'short_answer' ? parseAcceptedAnswers(key).join('\n') : key,
      poin: String(r.poin ?? '1'),
      konfigurasi: configuration || configExample(type === 'dropdown' ? 'dropdown' : 'pg_tunggal'),
      stimulus: parseStimulus(r.stimulus),
    })
    setAdding(true)
  }

  function canEdit(r: Row): boolean {
    if (user.role === 'admin') return true
    return String(r.dibuatOlehUserId || '') === user.id
  }

  function setOpsi(i: number, v: string) {
    setForm((f) => {
      const opsi = [...f.opsi]
      opsi[i] = v
      return { ...f, opsi }
    })
  }
  function addOpsi() {
    setForm((f) => ({ ...f, opsi: [...f.opsi, ''] }))
  }
  function removeOpsi(i: number) {
    setForm((f) => {
      const opsi = f.opsi.filter((_, idx) => idx !== i)
      const nextOptions = opsi.length ? opsi : ['', '']
      const mapIndex = (index: number) => index === i ? -1 : index > i ? index - 1 : index
      const kunci = f.tipe === 'checkbox'
        ? JSON.stringify(parseIndexes(f.kunci).map(mapIndex).filter((index) => index >= 0))
        : String(Math.min(mapIndex(Number(f.kunci)) < 0 ? 0 : mapIndex(Number(f.kunci)), Math.max(0, nextOptions.length - 1)))
      return { ...f, opsi: nextOptions, kunci }
    })
  }
  function moveOpsi(index: number, offset: -1 | 1) {
    setForm((f) => {
      const target = index + offset
      if (target < 0 || target >= f.opsi.length) return f
      const opsi = [...f.opsi]
      ;[opsi[index], opsi[target]] = [opsi[target], opsi[index]]
      const remap = (oldIndex: number) => oldIndex === index ? target : oldIndex === target ? index : oldIndex
      const kunci = f.tipe === 'checkbox'
        ? JSON.stringify(parseIndexes(f.kunci).map(remap).sort((a, b) => a - b))
        : String(remap(Number(f.kunci)))
      return { ...f, opsi, kunci }
    })
  }

  function changeType(tipe: string) {
    const useVisual = visualTypeOptions.some(([id]) => id === tipe)
    setVisualMode(useVisual)
    setForm((current) => ({
      ...current,
      tipe,
      konfigurasi: useVisual ? configExample(tipe as QuestionType) : current.konfigurasi,
      kunci: tipe === 'checkbox' ? '[]' : tipe === 'short_answer' || tipe === 'essay' ? '' : '0',
      ...(tipe === 'true_false' ? { opsi: ['Benar', 'Salah'] } : {}),
    }))
  }

  function acceptQuestionTypeDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault()
    const type = event.dataTransfer.getData('application/x-simulasi-question-type')
    if (visualTypeOptions.some(([id]) => id === type)) changeType(type)
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.pertanyaan.trim()) {
      toast.error('Pertanyaan wajib diisi.')
      return
    }
    const choiceType = !visualMode && ['pg', 'checkbox', 'dropdown', 'true_false'].includes(form.tipe)
    const opsi = form.tipe === 'true_false' ? ['Benar', 'Salah'] : choiceType ? form.opsi.map((s) => s.trim()).filter(Boolean) : []
    if (choiceType && opsi.length < 2) {
      toast.error('Soal pilihan minimal memiliki 2 opsi.')
      return
    }
    let kunci = form.kunci
    if (form.tipe === 'checkbox') {
      const indexes = parseIndexes(form.kunci).filter((index) => index < opsi.length)
      if (!indexes.length) { toast.error('Pilih minimal satu jawaban benar.'); return }
      kunci = JSON.stringify(indexes)
    } else if (form.tipe === 'short_answer') {
      const accepted = form.kunci.split('\n').map((answer) => answer.trim()).filter(Boolean)
      if (!accepted.length) { toast.error('Masukkan minimal satu jawaban yang diterima.'); return }
      kunci = JSON.stringify(accepted)
    } else if (choiceType) {
      kunci = String(Math.min(Number(form.kunci), opsi.length - 1))
    }
    const payload = {
      mapelId: form.mapelId || undefined,
      domain: form.domain.trim(),
      topik: form.topik.trim(),
      kompetensi: form.kompetensi.trim(),
      levelKognitif: form.levelKognitif.trim(),
      tipe: form.tipe,
      pertanyaan: form.pertanyaan,
      opsi: choiceType ? JSON.stringify(opsi) : '',
      konfigurasi: visualMode ? form.konfigurasi : undefined,
      stimulus: form.stimulus,
      kunci,
      poin: Number(form.poin) || 0,
    }
    setSubmitting(true)
    try {
      if (editing) {
        await request('/bank-soal/' + editing.id, token, 'PUT', payload)
        toast.success('Soal diperbarui.')
      } else {
        await request('/bank-soal', token, 'POST', payload)
        toast.success('Soal dibuat.')
      }
      setAdding(false)
      setEditing(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan soal.')
    } finally {
      setSubmitting(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/bank-soal/' + deletingRow.id, token, 'DELETE')
      toast.success('Soal dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus soal.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div data-assessment-workspace="bank-soal" className="space-y-4">
      <PageToolbar
        title="Bank Soal"
        description="Buat soal Ujian Online secara visual: pilih bentuk jawaban, atur kunci dan skor, lalu tambahkan stimulus bila diperlukan."
        actions={
          !readOnly && (
            <Button className="min-h-11" onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Tambah soal
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard title={editing ? 'Edit Soal' : 'Tambah Soal'} description="Pilih format jawaban. Kunci dan penilaian hanya terlihat oleh staf.">
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2">
              <Label>Mata Pelajaran</Label>
              <Select value={form.mapelId} onChange={(e) => setForm({ ...form, mapelId: e.target.value })}>
                <option value="">Pilih mapel</option>
                {mapel.map((m) => (
                  <option key={m.id} value={m.id}>{String(m.namaMapel || '-')}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Jenis soal</Label>
              <div className="space-y-2" onDragOver={(event) => event.preventDefault()} onDrop={acceptQuestionTypeDrop}>
                <Select aria-label="Jenis soal" value={form.tipe} onChange={(event) => changeType(event.target.value)}>
                  <optgroup label="Pembuat soal visual">
                    {visualTypeOptions.map(([id, label]) => <option key={id} value={id}>{label}</option>)}
                  </optgroup>
                  <optgroup label="Format lama (tetap didukung)">
                    <option value="pg">Pilihan ganda lama</option>
                    <option value="checkbox">Kotak centang lama</option>
                    <option value="true_false">Benar / salah lama</option>
                    <option value="short_answer">Isian singkat lama</option>
                    <option value="essay">Uraian lama</option>
                  </optgroup>
                </Select>
                <details className="rounded-xl border bg-muted/20 p-2">
                  <summary className="min-h-10 cursor-pointer list-none py-2 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">Pilih atau seret jenis soal</summary>
                  <div data-testid="bank-question-type-dropzone" role="group" aria-label="Area lepas kartu jenis soal" onDragOver={(event) => event.preventDefault()} onDrop={(event) => { acceptQuestionTypeDrop(event); event.stopPropagation() }} className="mb-2 grid min-h-12 place-items-center rounded-lg border-2 border-dashed border-primary/40 bg-primary/[0.03] px-3 text-center text-xs text-muted-foreground">Lepas kartu jenis soal di sini
                  </div>
                  <div className="grid gap-2 pt-2 sm:grid-cols-2">
                    {visualTypeOptions.map(([id, label]) => <button
                      type="button"
                      key={id}
                      draggable
                      aria-label={`Seret jenis soal ${label}`}
                      onClick={() => changeType(id)}
                      onDragStart={(event) => { event.dataTransfer.effectAllowed = 'copy'; event.dataTransfer.setData('application/x-simulasi-question-type', id) }}
                      className={`flex min-h-11 items-center rounded-lg border px-3 text-left text-sm hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${form.tipe === id ? 'border-primary bg-primary/10 font-medium' : 'bg-background'}`}
                    >{label}<span className="ml-auto pl-2 text-xs text-muted-foreground">Klik / seret</span></button>)}
                  </div>
                  <p className="px-1 pt-2 text-xs text-muted-foreground">Seret kartu ke pilihan jenis soal di atas, atau pilih dengan klik/keyboard.</p>
                </details>
              </div>
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Pertanyaan</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.pertanyaan}
                onChange={(e) => setForm({ ...form, pertanyaan: e.target.value })}
                required
              />
            </div>
            <details className="rounded-xl border bg-muted/20 p-3 sm:col-span-2">
              <summary className="min-h-11 cursor-pointer py-2 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">Capaian belajar (opsional, membantu laporan perkembangan)</summary>
              <div className="mt-3 grid gap-3 sm:grid-cols-2">
                <div className="grid gap-2"><Label htmlFor="bank-question-domain">Domain</Label><Input id="bank-question-domain" value={form.domain} onChange={(event) => setForm({ ...form, domain: event.target.value })} placeholder="Contoh: Literasi membaca, Bilangan" /></div>
                <div className="grid gap-2"><Label htmlFor="bank-question-topic">Topik</Label><Input id="bank-question-topic" value={form.topik} onChange={(event) => setForm({ ...form, topik: event.target.value })} placeholder="Contoh: Informasi tersurat, Pecahan" /></div>
                <div className="grid gap-2"><Label htmlFor="bank-question-competency">Kompetensi</Label><Input id="bank-question-competency" value={form.kompetensi} onChange={(event) => setForm({ ...form, kompetensi: event.target.value })} placeholder="Kemampuan yang diukur" /></div>
                <div className="grid gap-2"><Label htmlFor="bank-question-cognitive-level">Level kognitif</Label><Input id="bank-question-cognitive-level" value={form.levelKognitif} onChange={(event) => setForm({ ...form, levelKognitif: event.target.value })} placeholder="Memahami, menerapkan, atau menalar" /></div>
              </div>
              <p className="mt-3 text-xs text-muted-foreground">Label ini akan dibekukan pada hasil pengerjaan dan digunakan untuk melihat capaian siswa per kompetensi.</p>
            </details>
            {!visualMode && ['pg', 'checkbox', 'dropdown'].includes(form.tipe) ? (
              <div className="grid gap-2 sm:col-span-2">
                <Label>Opsi Jawaban</Label>
                <div className="space-y-2">
                  {form.opsi.map((op, i) => (
                    <div key={i} className="flex items-center gap-2">
                      <span className="text-sm font-medium w-5">{String.fromCharCode(65 + i)}.</span>
                      <Input value={op} onChange={(e) => setOpsi(i, e.target.value)} placeholder={`Opsi ${String.fromCharCode(65 + i)}`} />
                      <div className="flex shrink-0 gap-1">
                        <Button type="button" size="sm" variant="outline" aria-label={`Naikkan opsi ${i + 1}`} disabled={i === 0} onClick={() => moveOpsi(i, -1)}><ArrowUp className="h-3.5 w-3.5" /></Button>
                        <Button type="button" size="sm" variant="outline" aria-label={`Turunkan opsi ${i + 1}`} disabled={i === form.opsi.length - 1} onClick={() => moveOpsi(i, 1)}><ArrowDown className="h-3.5 w-3.5" /></Button>
                      {form.opsi.length > 2 && (
                        <Button type="button" size="sm" variant="outline" aria-label="Hapus opsi" onClick={() => removeOpsi(i)}><Trash2 className="h-3.5 w-3.5" /></Button>
                      )}
                      </div>
                    </div>
                  ))}
                </div>
                <Button type="button" variant="outline" size="sm" className="w-fit" onClick={addOpsi}><Plus className="h-3.5 w-3.5" /> Tambah opsi</Button>
              </div>
            ) : null}
            {!visualMode && ['pg', 'dropdown', 'true_false'].includes(form.tipe) ? <div className="grid gap-2">
              <Label>Jawaban benar</Label>
                <Select value={String(Math.min(Number(form.kunci), Math.max(0, form.opsi.length - 1)))} onChange={(e) => setForm({ ...form, kunci: e.target.value })}>
                  {(form.tipe === 'true_false' ? ['Benar', 'Salah'] : form.opsi).map((option, i) => (
                    <option key={i} value={i}>{String.fromCharCode(65 + i)} · {option || `Opsi ${i + 1}`}</option>
                  ))}
                </Select>
            </div> : null}
            {!visualMode && form.tipe === 'checkbox' ? <div className="grid gap-2 sm:col-span-2">
              <Label>Jawaban benar (pilih semua yang tepat)</Label>
              {form.opsi.map((option, i) => <label key={i} className="flex min-h-11 items-center gap-3 rounded-lg border px-3 text-sm"><input type="checkbox" checked={parseIndexes(form.kunci).includes(i)} onChange={(event) => { const selected = new Set(parseIndexes(form.kunci)); if (event.target.checked) selected.add(i); else selected.delete(i); setForm({ ...form, kunci: JSON.stringify([...selected].sort((a, b) => a - b)) }) }} />{String.fromCharCode(65 + i)} · {option || `Opsi ${i + 1}`}</label>)}
            </div> : null}
            {!visualMode && (form.tipe === 'short_answer' || form.tipe === 'essay') ? <div className="grid gap-2 sm:col-span-2">
              <Label>{form.tipe === 'short_answer' ? 'Jawaban yang diterima (satu per baris)' : 'Rubrik / panduan penilaian untuk guru'}</Label>
              <textarea className="flex min-h-[96px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" value={form.kunci} onChange={(e) => setForm({ ...form, kunci: e.target.value })} placeholder={form.tipe === 'short_answer' ? 'contoh: 2,5\n2.5' : 'Tuliskan aspek jawaban yang perlu diperhatikan saat menilai'} />
              <p className="text-xs text-muted-foreground">{form.tipe === 'short_answer' ? 'Jawaban dinilai otomatis setelah dinormalisasi.' : 'Jawaban uraian akan masuk antrean penilaian guru.'}</p>
            </div> : null}
            {visualMode && <div className="grid gap-4 sm:col-span-2 lg:grid-cols-2">
              <section className="space-y-4 rounded-2xl border bg-background p-4">
                <div className="rounded-xl border border-primary/20 bg-primary/5 p-3 text-sm text-muted-foreground">Atur pilihan, kunci, skor, dan stimulus secara visual. Format teknis tidak perlu ditulis.</div>
                <AnswerBuilder tipe={form.tipe as QuestionType} config={form.konfigurasi} setConfig={(konfigurasi) => setForm((current) => ({ ...current, konfigurasi }))} groupName="bank-soal-answer" />
              </section>
              <aside className="h-fit space-y-4 rounded-2xl border bg-slate-50 p-4">
                <div><p className="text-xs font-bold uppercase tracking-wide text-primary">Pratinjau siswa</p><h3 id="bank-soal-preview-prompt" className="mt-2 font-semibold">{form.pertanyaan || 'Pertanyaan akan muncul di sini'}</h3><p className="mt-1 text-xs text-muted-foreground">{form.poin || 1} poin · {typeLabels[form.tipe] || form.tipe}</p></div>
                <div className="rounded-xl border bg-white p-3"><QuestionAnswerControl question={{ tipe: form.tipe, pertanyaan: form.pertanyaan, konfigurasi: form.konfigurasi }} questionId="bank-soal-preview-prompt" value={null} onChange={() => undefined} /></div>
                <div><h4 className="mb-2 text-sm font-semibold">Stimulus pendukung</h4><StimulusContent items={form.stimulus} token={token} compact emptyLabel="Belum ada stimulus. Tambahkan teks, tabel, atau tautan media." /></div>
                <p className="text-xs text-muted-foreground">Kunci dan pembahasan tidak ditampilkan dalam pratinjau siswa.</p>
              </aside>
            </div>}
            <div className="sm:col-span-2"><BankStimulusEditor items={form.stimulus} onChange={(stimulus) => setForm((current) => ({ ...current, stimulus }))} /></div>
            <div className="grid gap-2">
              <Label>Poin</Label>
              <Input type="number" min="0" step="0.25" value={form.poin} onChange={(e) => setForm({ ...form, poin: e.target.value })} />
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={submitting}>{submitting ? 'Menyimpan...' : editing ? 'Simpan perubahan' : 'Simpan soal'}</Button>
              <Button type="button" variant="outline" disabled={submitting} onClick={() => { setAdding(false); setEditing(null) }}>Batal</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Pertanyaan</TableHead>
              <TableHead>Mapel</TableHead>
              <TableHead>Tipe</TableHead>
              <TableHead>Poin</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const m = (r.mapel as Row) || {}
              const opsi = parseOpsi(r.opsi)
              const capaianLabels = [r.domain, r.kompetensi].filter(Boolean).map(String)
              const kunciIdx = Number(r.kunci)
              const correctIndexes = r.tipe === 'checkbox' ? parseIndexes(String(r.kunci || '')) : [kunciIdx]
              return (
                <TableRow key={r.id}>
                  <TableCell>
                    <div className="font-medium line-clamp-2 max-w-md">{String(r.pertanyaan || '-')}</div>
                    {capaianLabels.length > 0 && <div className="mt-1 text-xs text-muted-foreground">{capaianLabels.join(' · ')}</div>}
                    {['pg', 'checkbox', 'dropdown', 'true_false'].includes(String(r.tipe)) && opsi.length ? (
                      <div className="text-xs text-muted-foreground mt-1 space-y-0.5">
                        {opsi.map((op, i) => (
                          <div key={i} className={correctIndexes.includes(i) ? 'text-success font-medium' : ''}>
                            {String.fromCharCode(65 + i)}. {op}{correctIndexes.includes(i) ? ' ✓' : ''}
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </TableCell>
                  <TableCell>{String(m.namaMapel || '-')}</TableCell>
                  <TableCell><Badge variant={r.tipe === 'pg' || r.tipe === 'checkbox' ? 'secondary' : 'outline'}>{typeLabels[String(r.tipe)] || String(r.tipe || '—')}</Badge></TableCell>
                  <TableCell className="text-sm">{String(r.poin ?? '-')}</TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      {!readOnly && canEdit(r) && (
                        <Button size="sm" variant="outline" aria-label="Ubah" onClick={() => openEdit(r)}><Pencil className="h-3.5 w-3.5" /></Button>
                      )}
                      {!readOnly && canEdit(r) && (
                        <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => setDeletingRow(r)}><Trash2 className="h-3.5 w-3.5" /></Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
            {!rows.length && <EmptyState colSpan={5} label="Belum ada soal." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Soal?</AlertDialogTitle>
            <AlertDialogDescription>Soal akan dihapus permanen. Soal yang sedang dipakai ujian tidak dapat dihapus.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Batal</AlertDialogCancel>
            <AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={confirmDelete} disabled={isDeleting}>
              {isDeleting ? 'Menghapus...' : 'Hapus'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function stimulusTableGrid(content: unknown): string[][] {
  const rows = String(content || '').split(/\r?\n/).map((line) => line.split('\t'))
  const width = Math.max(2, ...rows.map((row) => row.length))
  while (rows.length < 2) rows.push([])
  return rows.map((row) => [...row, ...Array(Math.max(0, width - row.length)).fill('')])
}

function BankStimulusEditor({ items, onChange }: { items: Row[]; onChange: (items: Row[]) => void }) {
  const [kind, setKind] = useState('text')
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null)

  const updateItem = (index: number, patch: Record<string, unknown>) => onChange(items.map((item, itemIndex) => itemIndex === index ? { ...item, ...patch } : item))
  const updateTableCell = (index: number, rowIndex: number, columnIndex: number, value: string) => {
    const rows = stimulusTableGrid(items[index].konten).map((row) => [...row])
    rows[rowIndex][columnIndex] = value
    updateItem(index, { konten: rows.map((row) => row.join('\t')).join('\n') })
  }
  const moveItem = (from: number, to: number) => {
    if (to < 0 || to >= items.length || from === to) return
    const next = [...items]
    const [item] = next.splice(from, 1)
    next.splice(to, 0, item)
    onChange(next.map((row, index) => ({ ...row, urutan: index + 1 })))
  }
  const addItem = () => {
    const content = kind === 'table' ? '\t\n\t' : ''
    const id = globalThis.crypto?.randomUUID?.() || `stimulus-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    onChange([...items, { id, jenis: kind, konten: content, altText: '', urutan: items.length + 1 }])
  }

  return <section aria-label="Stimulus pendukung" className="space-y-3 border-t pt-4">
    <div className="flex flex-wrap items-end justify-between gap-2">
      <div><h4 className="font-semibold">Stimulus pendukung <span className="font-normal text-muted-foreground">(opsional)</span></h4><p className="mt-1 text-xs text-muted-foreground">Tambahkan bacaan, tabel melalui grid, atau tautan media HTTPS.</p></div>
      <div className="flex gap-2"><Select aria-label="Jenis stimulus baru" value={kind} onChange={(event) => setKind(event.target.value)}><option value="text">Teks / bacaan</option><option value="table">Tabel</option><option value="media_link">Tautan media</option></Select><Button type="button" size="sm" variant="outline" className="min-h-11" onClick={addItem}><Plus className="h-4 w-4" />Tambah</Button></div>
    </div>
    {items.map((item, index) => <div key={`${String(item.urutan || index)}-${String(item.jenis)}`} draggable onDragStart={(event) => { setDraggedIndex(index); event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('application/x-pkbm-bank-stimulus-index', String(index)) }} onDragEnd={() => setDraggedIndex(null)} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); if (!event.dataTransfer.types.includes('application/x-pkbm-bank-stimulus-index')) return; const from = Number(event.dataTransfer.getData('application/x-pkbm-bank-stimulus-index')); moveItem(Number.isInteger(from) ? from : draggedIndex ?? -1, index); setDraggedIndex(null) }} className={`space-y-2 rounded-xl border bg-white p-3 ${draggedIndex === index ? 'opacity-60' : ''}`}>
      <div className="flex flex-wrap items-center justify-between gap-2"><div className="flex items-center gap-2 text-sm font-medium"><span aria-hidden="true" className="cursor-grab text-muted-foreground">⠿</span>{item.jenis === 'table' ? <Table2 className="h-4 w-4 text-primary" /> : item.jenis === 'media_link' ? <Link2 className="h-4 w-4 text-primary" /> : null}{item.jenis === 'media_link' ? 'Tautan media' : item.jenis === 'table' ? 'Tabel stimulus' : 'Teks bacaan'}</div><div className="flex gap-1"><Button type="button" size="icon" variant="outline" className="h-10 w-10" aria-label={`Naikkan stimulus ${index + 1}`} disabled={index === 0} onClick={() => moveItem(index, index - 1)}><ArrowUp className="h-4 w-4" /></Button><Button type="button" size="icon" variant="outline" className="h-10 w-10" aria-label={`Turunkan stimulus ${index + 1}`} disabled={index === items.length - 1} onClick={() => moveItem(index, index + 1)}><ArrowDown className="h-4 w-4" /></Button><Button type="button" size="icon" variant="ghost" className="h-10 w-10" aria-label={`Hapus stimulus ${index + 1}`} onClick={() => onChange(items.filter((_, itemIndex) => itemIndex !== index).map((row, itemIndex) => ({ ...row, urutan: itemIndex + 1 })))}><Trash2 className="h-4 w-4" /></Button></div></div>
      {item.jenis === 'table' ? <div className="space-y-2"><div className="overflow-x-auto rounded-lg border"><table className="min-w-full border-collapse"><tbody>{stimulusTableGrid(item.konten).map((row, rowIndex) => <tr key={rowIndex}>{row.map((cell, columnIndex) => <td key={columnIndex} className="min-w-32 border-b border-r p-1"><Input aria-label={`Stimulus ${index + 1}, baris ${rowIndex + 1}, kolom ${columnIndex + 1}`} className="min-h-10" value={cell} onChange={(event) => updateTableCell(index, rowIndex, columnIndex, event.target.value)} placeholder={rowIndex === 0 ? `Kolom ${columnIndex + 1}` : 'Isi sel'} /></td>)}</tr>)}</tbody></table></div><div className="flex flex-wrap gap-2"><Button type="button" size="sm" variant="outline" className="min-h-10" onClick={() => { const rows = stimulusTableGrid(item.konten); updateItem(index, { konten: [...rows, Array(rows[0].length).fill('')].map((row) => row.join('\t')).join('\n') }) }}><Plus className="h-3.5 w-3.5" />Tambah baris</Button><Button type="button" size="sm" variant="outline" className="min-h-10" onClick={() => { const rows = stimulusTableGrid(item.konten).map((row) => [...row, '']); updateItem(index, { konten: rows.map((row) => row.join('\t')).join('\n') }) }}><Plus className="h-3.5 w-3.5" />Tambah kolom</Button></div></div>
        : item.jenis === 'media_link' ? <Input aria-label={`Tautan stimulus ${index + 1}`} type="url" value={String(item.konten || '')} onChange={(event) => updateItem(index, { konten: event.target.value })} placeholder="https://contoh.id/video-pembelajaran" />
          : <textarea aria-label={`Teks stimulus ${index + 1}`} className="min-h-24 w-full rounded-xl border border-input bg-background p-3 text-sm leading-relaxed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" value={String(item.konten || '')} onChange={(event) => updateItem(index, { konten: event.target.value })} placeholder="Ketik bacaan atau informasi pendukung di sini." />}
    </div>)}
    {!items.length && <p className="rounded-xl border border-dashed p-3 text-sm text-muted-foreground">Belum ada stimulus. Soal tetap bisa dibuat tanpa bahan pendukung.</p>}
  </section>
}
