import { useEffect, useState, type FormEvent } from 'react'
import { Download, FileText, Info, Pencil, Plus, Trash2 } from 'lucide-react'
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
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

type Options = {
  mapel: { id: string; nama: string }[]
  jenjang: number[]
  tahunAjaran: { id: string; nama: string; isAktif: boolean }[]
  fase: { id: string; kode: string; nama: string }[]
  activeTahunAjaranId: string
}

function fmtSize(bytes: number): string {
  if (!bytes) return ''
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

const emptyForm = {
  mapelId: '',
  jenjang: '',
  tahunAjaranId: '',
  faseId: '',
  judul: '',
  pertemuanKe: '',
  alokasiWaktu: '',
  tanggal: '',
  deskripsi: '',
}

export function RppView({
  token,
  user,
  readOnly,
}: {
  token: string
  user: User
  readOnly: boolean
}) {
  const [rows, setRows] = useState<Row[]>([])
  const [options, setOptions] = useState<Options>({ mapel: [], jenjang: [], tahunAjaran: [], fase: [], activeTahunAjaranId: '' })
  const [makerStatus, setMakerStatus] = useState(false)
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [file, setFile] = useState<File | null>(null)
  const [saving, setSaving] = useState(false)
  const [filter, setFilter] = useState({ jenjang: '', mapelId: '', tahunAjaranId: '' })

  const canCreate = user.role === 'admin' || makerStatus

  function load() {
    const qs = new URLSearchParams()
    if (filter.jenjang) qs.set('jenjang', filter.jenjang)
    if (filter.mapelId) qs.set('mapelId', filter.mapelId)
    if (filter.tahunAjaranId) qs.set('tahunAjaranId', filter.tahunAjaranId)
    const q = qs.toString() ? '?' + qs.toString() : ''
    void request('/rpp' + q, token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/rpp/options', token).then((r) => setOptions((r as Options) || { mapel: [], jenjang: [], tahunAjaran: [], fase: [], activeTahunAjaranId: '' })).catch(() => undefined)
    void request('/rpp/maker-status', token).then((r: Row) => setMakerStatus(Boolean((r as Row).isRppMaker))).catch(() => setMakerStatus(false))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    load()
  }, [filter]) // eslint-disable-line react-hooks/exhaustive-deps

  function activeTAId(): string {
    return options.activeTahunAjaranId || options.tahunAjaran[0]?.id || ''
  }

  function openAdd() {
    setForm({ ...emptyForm, tahunAjaranId: activeTAId() })
    setEditing(null)
    setFile(null)
    setAdding(true)
  }

  function openEdit(r: Row) {
    setEditing(r)
    const tgl = r.tanggal ? String(r.tanggal).slice(0, 10) : ''
    setForm({
      mapelId: String(r.mapelId || ''),
      jenjang: String(r.jenjang ?? ''),
      tahunAjaranId: String(r.tahunAjaranId || ''),
      faseId: String(r.faseId || ''),
      judul: String(r.judul || ''),
      pertemuanKe: r.pertemuanKe ? String(r.pertemuanKe) : '',
      alokasiWaktu: String(r.alokasiWaktu || ''),
      tanggal: tgl,
      deskripsi: String(r.deskripsi || ''),
    })
    setFile(null)
    setAdding(true)
  }

  function canEdit(r: Row): boolean {
    if (user.role === 'admin') return true
    return String(r.dibuatOlehUserId || '') === user.id
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.judul || !form.mapelId || !form.jenjang || !form.tahunAjaranId) {
      toast.error('Judul, mapel, jenjang, dan tahun ajaran wajib diisi.')
      return
    }
    if (!editing && !file) {
      toast.error('File RPP wajib diunggah (PDF/Word).')
      return
    }
    setSaving(true)
    try {
      const data = new FormData()
      data.append('mapelId', form.mapelId)
      data.append('jenjang', form.jenjang)
      data.append('tahunAjaranId', form.tahunAjaranId)
      if (form.faseId) data.append('faseId', form.faseId)
      data.append('judul', form.judul)
      data.append('pertemuanKe', form.pertemuanKe)
      data.append('alokasiWaktu', form.alokasiWaktu)
      data.append('tanggal', form.tanggal)
      data.append('deskripsi', form.deskripsi)
      if (file) data.append('file', file)

      const r = await fetch(apiBase + '/rpp' + (editing ? '/' + editing.id : ''), {
        method: editing ? 'PUT' : 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      const res = await r.json().catch(() => ({}))
      if (!r.ok) throw new Error((res as any)?.error || `Permintaan gagal (${r.status}).`)
      toast.success(editing ? 'RPP diperbarui.' : 'RPP diunggah.')
      setAdding(false)
      setEditing(null)
      setFile(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan RPP.')
    } finally {
      setSaving(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/rpp/' + deletingRow.id, token, 'DELETE')
      toast.success('RPP dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus RPP.')
    } finally {
      setIsDeleting(false)
    }
  }

  async function download(r: Row) {
    try {
      const res = await fetch(apiBase + '/rpp/' + r.id + '/download', {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('file tidak tersedia')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = String(r.judul || 'rpp') + (String(r.tipe || ''))
      a.click()
      URL.revokeObjectURL(url)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengunduh RPP.')
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="RPP / Rencana Pelaksanaan Pembelajaran"
        description="Unggah RPP (PDF/Word) per mapel & jenjang. 1 RPP dipakai bersama seluruh rombel di jenjang itu."
        actions={
          !readOnly && canCreate && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Unggah RPP
            </Button>
          )
        }
      />

      {!readOnly && user.role === 'guru' && !canCreate && (
        <div className="flex items-start gap-2 rounded-xl border border-yellow-200 bg-yellow-50 px-4 py-3 text-sm text-yellow-800">
          <Info className="h-4 w-4 mt-0.5 shrink-0" />
          <span>Anda bukan penyusun RPP yang ditugaskan. Hubungi admin untuk dicentang sebagai penyusun RPP. Anda tetap dapat melihat & mengunduh RPP untuk jenjang yang Anda ajar.</span>
        </div>
      )}

      {/* Filter bar */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="grid gap-1">
          <Label className="text-xs">Jenjang</Label>
          <Select value={filter.jenjang} onChange={(e) => setFilter({ ...filter, jenjang: e.target.value })} className="w-40">
            <option value="">Semua jenjang</option>
            {options.jenjang.map((n) => (
              <option key={n} value={String(n)}>Kelas {n}</option>
            ))}
          </Select>
        </div>
        <div className="grid gap-1">
          <Label className="text-xs">Mapel</Label>
          <Select value={filter.mapelId} onChange={(e) => setFilter({ ...filter, mapelId: e.target.value })} className="w-48">
            <option value="">Semua mapel</option>
            {options.mapel.map((m) => (
              <option key={m.id} value={m.id}>{m.nama || '-'}</option>
            ))}
          </Select>
        </div>
        <div className="grid gap-1">
          <Label className="text-xs">Tahun Ajaran</Label>
          <Select value={filter.tahunAjaranId} onChange={(e) => setFilter({ ...filter, tahunAjaranId: e.target.value })} className="w-44">
            <option value="">Semua TA</option>
            {options.tahunAjaran.map((t) => (
              <option key={t.id} value={t.id}>{t.nama || '-'}</option>
            ))}
          </Select>
        </div>
      </div>

      {adding && !readOnly && canCreate && (
        <FormCard
          title={editing ? 'Edit RPP' : 'Unggah RPP'}
          description="Identitas RPP diisi lengkap. File maks 10 MB (pdf, docx, doc). Semester diisi otomatis."
        >
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Judul / Topik RPP *</Label>
              <Input value={form.judul} onChange={(e) => setForm({ ...form, judul: e.target.value })} required placeholder="Mis. RPP Tema 1 — Diriku" />
            </div>
            <div className="grid gap-2">
              <Label>Mata Pelajaran *</Label>
              <Select value={form.mapelId} onChange={(e) => setForm({ ...form, mapelId: e.target.value })} required>
                <option value="">Pilih mapel</option>
                {options.mapel.map((m) => (
                  <option key={m.id} value={m.id}>{m.nama || '-'}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Jenjang / Tingkat *</Label>
              <Select value={form.jenjang} onChange={(e) => setForm({ ...form, jenjang: e.target.value })} required>
                <option value="">Pilih jenjang</option>
                {options.jenjang.map((n) => (
                  <option key={n} value={String(n)}>Kelas {n} (semua rombel)</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Tahun Ajaran *</Label>
              <Select value={form.tahunAjaranId} onChange={(e) => setForm({ ...form, tahunAjaranId: e.target.value })} required>
                <option value="">Pilih tahun ajaran</option>
                {options.tahunAjaran.map((t) => (
                  <option key={t.id} value={t.id}>{t.nama || '-'}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Fase (opsional)</Label>
              <Select value={form.faseId} onChange={(e) => setForm({ ...form, faseId: e.target.value })}>
                <option value="">— Tanpa fase —</option>
                {options.fase.map((f) => (
                  <option key={f.id} value={f.id}>{f.kode || ''} — {f.nama || '-'}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Pertemuan ke- (opsional)</Label>
              <Input type="number" min={1} value={form.pertemuanKe} onChange={(e) => setForm({ ...form, pertemuanKe: e.target.value })} placeholder="1" />
            </div>
            <div className="grid gap-2">
              <Label>Tanggal (opsional)</Label>
              <Input type="date" value={form.tanggal} onChange={(e) => setForm({ ...form, tanggal: e.target.value })} />
            </div>
            <div className="grid gap-2">
              <Label>Alokasi Waktu (opsional)</Label>
              <Input value={form.alokasiWaktu} onChange={(e) => setForm({ ...form, alokasiWaktu: e.target.value })} placeholder="2 x 35 menit" />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Deskripsi / Catatan (opsional)</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.deskripsi}
                onChange={(e) => setForm({ ...form, deskripsi: e.target.value })}
              />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>File RPP {editing ? '(kosongkan untuk pakai file lama)' : '*'}</Label>
              <Input type="file" accept=".pdf,.doc,.docx" onChange={(e) => setFile(e.target.files?.[0] || null)} required={!editing} />
              <p className="text-xs text-muted-foreground">Format: PDF, DOCX, atau DOC. Maks 10 MB.</p>
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={saving}>{saving ? 'Menyimpan...' : editing ? 'Simpan perubahan' : 'Unggah'}</Button>
              <Button type="button" variant="outline" onClick={() => { setAdding(false); setEditing(null); setFile(null) }}>Batal</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Judul</TableHead>
              <TableHead>Mapel</TableHead>
              <TableHead>Jenjang</TableHead>
              <TableHead>TA</TableHead>
              <TableHead>Pertemuan</TableHead>
              <TableHead>Tanggal</TableHead>
              <TableHead>File</TableHead>
              <TableHead>Pengunggah</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const m = (r.mapel as Row) || {}
              const ta = (r.tahunAjaran as Row) || {}
              const tutor = (r.tutor as Row) || {}
              return (
                <TableRow key={r.id}>
                  <TableCell>
                    <div className="font-medium flex items-center gap-2"><FileText className="h-4 w-4 text-primary" />{String(r.judul || '-')}</div>
                    {r.deskripsi ? <div className="text-xs text-muted-foreground line-clamp-1 max-w-md">{String(r.deskripsi)}</div> : null}
                  </TableCell>
                  <TableCell>{String(m.namaMapel || '-')}</TableCell>
                  <TableCell>Kelas {String(r.jenjang ?? '-')}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{String(ta.namaTahunAjaran || '-')}</TableCell>
                  <TableCell className="text-muted-foreground">{r.pertemuanKe ? String(r.pertemuanKe) : '-'}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{r.tanggal ? String(r.tanggal).slice(0, 10) : '-'}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    <div className="flex items-center gap-1"><FileText className="h-3 w-3" />{String(r.tipe || '')} {fmtSize(Number(r.ukuran) || 0)}</div>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{String(tutor.nama || '-')}</TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button size="sm" variant="outline" aria-label="Unduh" onClick={() => download(r)}><Download className="h-3.5 w-3.5" /></Button>
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
            {!rows.length && <EmptyState colSpan={9} label="Belum ada RPP." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus RPP?</AlertDialogTitle>
            <AlertDialogDescription>RPP <strong>{String(deletingRow?.judul || '')}</strong> beserta filenya akan dihapus.</AlertDialogDescription>
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