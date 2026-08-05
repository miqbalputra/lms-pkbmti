import { useEffect, useState, type FormEvent } from 'react'
import { ImageIcon, Pencil, Plus, Trash2 } from 'lucide-react'
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

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

function fmtDate(v: unknown): string {
  if (!v) return ''
  return String(v).slice(0, 10)
}

const emptyForm = {
  mapelId: '',
  kelasId: '',
  tanggal: new Date().toISOString().slice(0, 10),
  materi: '',
  kegiatan: '',
}

export function JurnalMengajarView({
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
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [foto, setFoto] = useState<File | null>(null)
  const [saving, setSaving] = useState(false)

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  const load = () => {
    void request('/jurnal', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  function openAdd() {
    setForm({ ...emptyForm })
    setEditing(null)
    setFoto(null)
    setAdding(true)
  }

  function openEdit(r: Row) {
    setEditing(r)
    setForm({
      mapelId: String(r.mapelId || ''),
      kelasId: String(r.kelasId || ''),
      tanggal: fmtDate(r.tanggal),
      materi: String(r.materi || ''),
      kegiatan: String(r.kegiatan || ''),
    })
    setFoto(null)
    setAdding(true)
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.mapelId || !form.kelasId || !form.tanggal) {
      toast.error('Mapel, kelas, dan tanggal wajib diisi.')
      return
    }
    setSaving(true)
    try {
      const data = new FormData()
      data.append('mapelId', form.mapelId)
      data.append('kelasId', form.kelasId)
      data.append('tanggal', form.tanggal)
      data.append('materi', form.materi)
      data.append('kegiatan', form.kegiatan)
      if (foto) data.append('foto', foto)

      const r = await fetch(apiBase + '/jurnal' + (editing ? '/' + editing.id : ''), {
        method: editing ? 'PUT' : 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      const res = await r.json().catch(() => ({}))
      if (!r.ok) throw new Error((res as any)?.error || `Permintaan gagal (${r.status}).`)
      toast.success(editing ? 'Jurnal diperbarui.' : 'Jurnal dicatat.')
      setAdding(false)
      setEditing(null)
      setFoto(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan jurnal.')
    } finally {
      setSaving(false)
    }
  }

  async function openFoto(r: Row) {
    try {
      const res = await fetch(apiBase + '/jurnal/' + r.id + '/foto', {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('foto tidak tersedia')
      const url = URL.createObjectURL(await res.blob())
      window.open(url, '_blank')
      setTimeout(() => URL.revokeObjectURL(url), 60000)
    } catch (err: any) {
      toast.error(err.message || 'Gagal memuat foto.')
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/jurnal/' + deletingRow.id, token, 'DELETE')
      toast.success('Jurnal dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus jurnal.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Jurnal Mengajar"
        description="Catat kegiatan mengajar harian per mapel & rombel. Jurnal langsung tersimpan & berlaku (tanpa persetujuan)."
        actions={
          !readOnly && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Catat jurnal
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard
          title={editing ? 'Edit Jurnal' : 'Catat Jurnal Mengajar'}
          description="Foto dokumentasi opsional (jpg/png, maks 5 MB). Jurnal langsung berlaku begitu disimpan."
        >
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2">
              <Label>Mata Pelajaran</Label>
              <Select value={form.mapelId} onChange={(e) => setForm({ ...form, mapelId: e.target.value })} required>
                <option value="">Pilih mapel</option>
                {mapel.map((m) => (
                  <option key={m.id} value={m.id}>
                    {String(m.namaMapel || '-')}
                  </option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Kelas / Rombel</Label>
              <Select value={form.kelasId} onChange={(e) => setForm({ ...form, kelasId: e.target.value })} required>
                <option value="">Pilih kelas</option>
                {kelasOptions.map((k) => (
                  <option key={k.id} value={k.id}>
                    {kelasLabel(k)}
                  </option>
                ))}
              </Select>
              {isGuru && !kelasOptions.length && (
                <p className="text-xs text-muted-foreground">Anda belum ditetapkan sebagai wali kelas mana pun.</p>
              )}
            </div>
            <div className="grid gap-2">
              <Label>Tanggal</Label>
              <Input type="date" value={form.tanggal} onChange={(e) => setForm({ ...form, tanggal: e.target.value })} required />
            </div>
            <div className="grid gap-2">
              <Label>Foto Dokumentasi (opsional)</Label>
              <Input
                type="file"
                accept="image/png,image/jpeg"
                onChange={(e) => setFoto(e.target.files?.[0] || null)}
              />
              {editing && (editing.fotoPath as string) && !foto && (
                <p className="text-xs text-muted-foreground">Foto lama tetap dipakai bila tidak diganti.</p>
              )}
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Materi</Label>
              <Input
                value={form.materi}
                onChange={(e) => setForm({ ...form, materi: e.target.value })}
                placeholder="Materi yang diajarkan"
              />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Kegiatan</Label>
              <textarea
                className="flex min-h-[120px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.kegiatan}
                onChange={(e) => setForm({ ...form, kegiatan: e.target.value })}
                placeholder="Deskripsi kegiatan pembelajaran..."
              />
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={saving}>
                {saving ? 'Menyimpan...' : editing ? 'Simpan perubahan' : 'Simpan jurnal'}
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setAdding(false)
                  setEditing(null)
                  setFoto(null)
                }}
              >
                Batal
              </Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Tanggal</TableHead>
              <TableHead>Mapel</TableHead>
              <TableHead>Kelas</TableHead>
              <TableHead>Materi</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const m = (r.mapel as Row) || {}
              const k = (r.kelas as Row) || {}
              const canModify = !readOnly
              return (
                <TableRow key={r.id}>
                  <TableCell className="text-sm">{fmtDate(r.tanggal)}</TableCell>
                  <TableCell className="font-medium">{String(m.namaMapel || '-')}</TableCell>
                  <TableCell>{kelasLabel(k)}</TableCell>
                  <TableCell>
                    <div className="text-sm">{String(r.materi || '-')}</div>
                    {r.kegiatan ? (
                      <div className="text-xs text-muted-foreground line-clamp-1 max-w-xs">{String(r.kegiatan)}</div>
                    ) : null}
                    {r.fotoPath ? (
                      <button
                        type="button"
                        onClick={() => openFoto(r)}
                        className="inline-flex items-center gap-1 text-xs text-primary hover:underline mt-1"
                      >
                        <ImageIcon className="h-3 w-3" /> Lihat foto
                      </button>
                    ) : null}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      {canModify && (
                        <Button size="sm" variant="outline" aria-label="Ubah" onClick={() => openEdit(r)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                      )}
                      {canModify && (
                        <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => setDeletingRow(r)}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
            {!rows.length && <EmptyState colSpan={5} label="Belum ada jurnal mengajar." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Jurnal?</AlertDialogTitle>
            <AlertDialogDescription>
              Jurnal mengajar <strong>{String((deletingRow?.mapel as Row)?.namaMapel || '')}</strong> tanggal{' '}
              {fmtDate(deletingRow?.tanggal)} akan dihapus.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Batal</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={confirmDelete}
              disabled={isDeleting}
            >
              {isDeleting ? 'Menghapus...' : 'Hapus'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}