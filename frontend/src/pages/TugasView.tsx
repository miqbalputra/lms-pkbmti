import { useEffect, useState, type FormEvent } from 'react'
import { ClipboardList, Download, Pencil, Plus, Trash2, Upload } from 'lucide-react'
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../components/ui/dialog'
import type { User } from '../App'
import { request } from '../lib/api'
import { formatWibDate } from '../lib/wib'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

function fmtDate(v: unknown): string {
  return formatWibDate(v)
}

function statusBadge(s: string) {
  if (s === 'Dinilai') return <Badge variant="default" className="bg-success text-success-foreground">Dinilai</Badge>
  if (s === 'Terlambat') return <Badge variant="destructive">Terlambat</Badge>
  return <Badge variant="secondary">Terkumpul</Badge>
}

const emptyForm = { mapelId: '', kelasId: '', modulId: '', judul: '', deskripsi: '', deadline: '', bolehUpload: true }

export function TugasView({
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
  const [kelas, setKelas] = useState<Row[]>([])
  const [modul, setModul] = useState<Row[]>([])
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [file, setFile] = useState<File | null>(null)
  const [saving, setSaving] = useState(false)
  const [pengumpulanTugas, setPengumpulanTugas] = useState<Row | null>(null)

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  const load = () => {
    void request('/tugas', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
    void request('/modul-belajar', token).then((r: Row[]) => setModul(r || [])).catch(() => setModul([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  function openAdd() {
    setForm({ ...emptyForm })
    setEditing(null)
    setFile(null)
    setAdding(true)
  }

  function openEdit(r: Row) {
    setEditing(r)
    setForm({
      mapelId: String(r.mapelId || ''),
      kelasId: String(r.kelasId || ''),
      modulId: String(r.modulId || ''),
      judul: String(r.judul || ''),
      deskripsi: String(r.deskripsi || ''),
      deadline: fmtDate(r.deadline),
      bolehUpload: r.bolehUpload !== false,
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
    if (!form.judul || !form.kelasId || !form.deadline) {
      toast.error('Judul, kelas, dan deadline wajib diisi.')
      return
    }
    setSaving(true)
    try {
      const data = new FormData()
      data.append('mapelId', form.mapelId)
      data.append('kelasId', form.kelasId)
      if (form.modulId) data.append('modulId', form.modulId)
      data.append('judul', form.judul)
      data.append('deskripsi', form.deskripsi)
      data.append('deadline', form.deadline)
      data.append('bolehUpload', form.bolehUpload ? 'true' : 'false')
      if (file) data.append('file', file)

      const r = await fetch(apiBase + '/tugas' + (editing ? '/' + editing.id : ''), {
        method: editing ? 'PUT' : 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      const res = await r.json().catch(() => ({}))
      if (!r.ok) throw new Error((res as any)?.error || `Permintaan gagal (${r.status}).`)
      toast.success(editing ? 'Tugas diperbarui.' : 'Tugas dibuat.')
      setAdding(false)
      setEditing(null)
      setFile(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan tugas.')
    } finally {
      setSaving(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/tugas/' + deletingRow.id, token, 'DELETE')
      toast.success('Tugas dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus tugas.')
    } finally {
      setIsDeleting(false)
    }
  }

  async function downloadLampiran(r: Row) {
    try {
      const res = await fetch(apiBase + '/tugas/' + r.id + '/lampiran', {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('lampiran tidak tersedia')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = String(r.judul || 'lampiran')
      a.click()
      URL.revokeObjectURL(url)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengunduh lampiran.')
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Tugas Siswa"
        description="Buat tugas per mapel & rombel. Pengumpulan & nilai dicatat oleh tutor."
        actions={
          !readOnly && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Buat tugas
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard title={editing ? 'Edit Tugas' : 'Buat Tugas'} description="Lampiran opsional (pdf/docx/xlsx/gambar, maks 10 MB).">
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Judul</Label>
              <Input value={form.judul} onChange={(e) => setForm({ ...form, judul: e.target.value })} required />
            </div>
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
              <Label>Kelas / Rombel</Label>
              <Select value={form.kelasId} onChange={(e) => setForm({ ...form, kelasId: e.target.value })} required>
                <option value="">Pilih kelas</option>
                {kelasOptions.map((k) => (
                  <option key={k.id} value={k.id}>{kelasLabel(k)}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Deadline</Label>
              <Input type="date" value={form.deadline} onChange={(e) => setForm({ ...form, deadline: e.target.value })} required />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Modul Pembelajaran (opsional)</Label>
              <Select value={form.modulId} onChange={(e) => setForm({ ...form, modulId: e.target.value })}>
                <option value="">— Tanpa modul —</option>
                {modul
                  .filter((m) => !form.mapelId || String(m.mapelId || '') === form.mapelId)
                  .map((m) => (
                    <option key={m.id} value={m.id}>{String(m.judul || '-')}</option>
                  ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Lampiran (opsional)</Label>
              <Input type="file" onChange={(e) => setFile(e.target.files?.[0] || null)} />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Deskripsi</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.deskripsi}
                onChange={(e) => setForm({ ...form, deskripsi: e.target.value })}
              />
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={saving}>{saving ? 'Menyimpan...' : editing ? 'Simpan perubahan' : 'Simpan tugas'}</Button>
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
              <TableHead>Kelas</TableHead>
              <TableHead>Deadline</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const m = (r.mapel as Row) || {}
              const k = (r.kelas as Row) || {}
              return (
                <TableRow key={r.id}>
                  <TableCell>
                    <div className="font-medium flex items-center gap-2"><ClipboardList className="h-4 w-4 text-primary" />{String(r.judul || '-')}</div>
                    {r.deskripsi ? <div className="text-xs text-muted-foreground line-clamp-1 max-w-md">{String(r.deskripsi)}</div> : null}
                  </TableCell>
                  <TableCell>{String(m.namaMapel || '-')}</TableCell>
                  <TableCell>{kelasLabel(k)}</TableCell>
                  <TableCell className="text-sm">{fmtDate(r.deadline)}</TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      {!readOnly && (
                        <Button size="sm" variant="default" onClick={() => setPengumpulanTugas(r)}>
                          <Upload className="h-3.5 w-3.5" /> Pengumpulan
                        </Button>
                      )}
                      {r.filePath ? (
                        <Button size="sm" variant="outline" aria-label="Unduh" onClick={() => downloadLampiran(r)}><Download className="h-3.5 w-3.5" /></Button>
                      ) : null}
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
            {!rows.length && <EmptyState colSpan={5} label="Belum ada tugas." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Tugas?</AlertDialogTitle>
            <AlertDialogDescription>Tugas <strong>{String(deletingRow?.judul || '')}</strong> beserta seluruh pengumpulan akan dihapus.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Batal</AlertDialogCancel>
            <AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={confirmDelete} disabled={isDeleting}>
              {isDeleting ? 'Menghapus...' : 'Hapus'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {pengumpulanTugas && (
        <PengumpulanDialog
          token={token}
          tugas={pengumpulanTugas}
          readOnly={readOnly}
          onClose={() => setPengumpulanTugas(null)}
        />
      )}
    </div>
  )
}

function PengumpulanDialog({
  token,
  tugas,
  readOnly,
  onClose,
}: {
  token: string
  tugas: Row
  readOnly: boolean
  onClose: () => void
}) {
  const [siswa, setSiswa] = useState<Row[]>([])
  const [pengumpulan, setPengumpulan] = useState<Row[]>([])
  // per-siswa form state: jawaban, file, nilai, catatan
  const [forms, setForms] = useState<Record<string, { jawaban: string; file: File | null; nilai: string; catatan: string }>>({})

  const load = () => {
    void request('/peserta-didik?kelasId=' + tugas.kelasId, token).then((r: Row[]) => setSiswa(r || [])).catch(() => setSiswa([]))
    void request('/tugas/' + tugas.id + '/pengumpulan', token).then((r: Row[]) => setPengumpulan(r || [])).catch(() => setPengumpulan([]))
  }

  useEffect(() => {
    load()
  }, [tugas.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const pkBySiswa = new Map(pengumpulan.map((p) => [String(p.pesertaDidikId), p]))

  function formOf(pdId: string) {
    const pk = pkBySiswa.get(pdId)
    return forms[pdId] || {
      jawaban: pk ? String(pk.jawabanTeks || '') : '',
      file: null,
      nilai: pk && pk.nilai != null ? String(pk.nilai) : '',
      catatan: pk ? String(pk.catatanTutor || '') : '',
    }
  }

  function setField(pdId: string, patch: Partial<{ jawaban: string; file: File | null; nilai: string; catatan: string }>) {
    setForms((prev) => ({ ...prev, [pdId]: { ...formOf(pdId), ...patch } }))
  }

  async function catat(pdId: string) {
    const f = formOf(pdId)
    const data = new FormData()
    data.append('pesertaDidikId', pdId)
    data.append('jawabanTeks', f.jawaban)
    if (f.file) data.append('file', f.file)
    try {
      const r = await fetch(apiBase + '/tugas/' + tugas.id + '/pengumpulan', {
        method: 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      const res = await r.json().catch(() => ({}))
      if (!r.ok) throw new Error((res as any)?.error || `Gagal (${r.status}).`)
      toast.success('Pengumpulan dicatat.')
      setForms((prev) => { const n = { ...prev }; delete n[pdId]; return n })
      load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal mencatat pengumpulan.')
    }
  }

  async function nilai(pdId: string) {
    const f = formOf(pdId)
    if (f.nilai === '' || isNaN(Number(f.nilai))) {
      toast.error('Nilai harus berupa angka.')
      return
    }
    try {
      await request('/tugas/' + tugas.id + '/nilai', token, 'POST', {
        pesertaDidikId: pdId,
        nilai: Number(f.nilai),
        catatanTutor: f.catatan,
      })
      toast.success('Nilai disimpan.')
      setForms((prev) => { const n = { ...prev }; delete n[pdId]; return n })
      load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan nilai.')
    }
  }

  async function downloadFile(pk: Row) {
    try {
      const res = await fetch(apiBase + '/tugas/' + tugas.id + '/pengumpulan/' + pk.id + '/file', {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('file tidak tersedia')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'pengumpulan-' + String((pk.pesertaDidik as Row)?.nama || '')
      a.click()
      URL.revokeObjectURL(url)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengunduh file.')
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl">
        <DialogHeader>
          <DialogTitle>Pengumpulan — {String(tugas.judul || '')}</DialogTitle>
          <DialogDescription>
            Deadline {fmtDate(tugas.deadline)}. Catat pengumpulan & nilai per peserta didik.
          </DialogDescription>
        </DialogHeader>
        <div className="max-h-[60vh] overflow-y-auto">
          <Table>
            <TableHeader>
              <TableRow className="border-b border-border">
                <TableHead className="text-xs uppercase">Siswa</TableHead>
                <TableHead className="text-xs uppercase">Status</TableHead>
                <TableHead className="text-xs uppercase">Jawaban / File</TableHead>
                <TableHead className="text-xs uppercase">Nilai</TableHead>
                <TableHead className="text-xs uppercase text-right">Aksi</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {siswa.map((s) => {
                const pk = pkBySiswa.get(s.id)
                const f = formOf(s.id)
                const dinilai = pk && pk.status === 'Dinilai'
                return (
                  <TableRow key={s.id}>
                    <TableCell className="font-medium align-top">{String(s.nama)}</TableCell>
                    <TableCell className="align-top">{pk ? statusBadge(String(pk.status)) : <Badge variant="outline">Belum</Badge>}</TableCell>
                    <TableCell className="align-top space-y-1">
                      <Input
                        value={f.jawaban}
                        disabled={!!readOnly || !!dinilai}
                        onChange={(e) => setField(s.id, { jawaban: e.target.value })}
                        placeholder="Jawaban teks"
                        className="h-8 text-xs"
                      />
                      <Input
                        type="file"
                        disabled={!!readOnly || !!dinilai}
                        onChange={(e) => setField(s.id, { file: e.target.files?.[0] || null })}
                        className="h-8 text-xs"
                      />
                      {pk && pk.filePath ? (
                        <button onClick={() => downloadFile(pk)} className="text-xs text-primary hover:underline flex items-center gap-1">
                          <Download className="h-3 w-3" /> File pengumpulan
                        </button>
                      ) : null}
                      {pk && pk.catatanTutor ? (
                        <div className="text-xs text-muted-foreground">Catatan: {String(pk.catatanTutor)}</div>
                      ) : null}
                    </TableCell>
                    <TableCell className="align-top space-y-1">
                      <Input
                        type="number"
                        value={f.nilai}
                        disabled={!!readOnly || !pk}
                        onChange={(e) => setField(s.id, { nilai: e.target.value })}
                        placeholder="Nilai"
                        className="h-8 w-20 text-xs"
                      />
                      <Input
                        value={f.catatan}
                        disabled={!!readOnly || !pk}
                        onChange={(e) => setField(s.id, { catatan: e.target.value })}
                        placeholder="Catatan tutor"
                        className="h-8 text-xs"
                      />
                    </TableCell>
                    <TableCell className="align-top text-right">
                      <div className="flex flex-col gap-1 items-end">
                        {!readOnly && !dinilai && (
                          <Button size="sm" variant="outline" onClick={() => catat(s.id)}>Catat</Button>
                        )}
                        {!readOnly && pk && !dinilai && (
                          <Button size="sm" onClick={() => nilai(s.id)}>Nilai</Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
              {!siswa.length && <EmptyState colSpan={5} label="Belum ada peserta didik di rombel ini." />}
            </TableBody>
          </Table>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Tutup</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
