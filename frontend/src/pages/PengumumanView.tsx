import { useEffect, useState, type FormEvent } from 'react'
import { Megaphone, Pencil, Plus, Trash2 } from 'lucide-react'
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
import { Checkbox } from '../components/ui/checkbox'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'
import { formatWibDate, wibDateTimeLocalToISO } from '../lib/wib'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

function fmtDate(v: unknown): string {
  return formatWibDate(v)
}

const emptyForm = {
  judul: '',
  isi: '',
  target: 'semua',
  kelasId: '',
  aktif: true,
  tanggalMulai: '',
  tanggalSelesai: '',
}

export function PengumumanView({
  token,
  user,
  readOnly,
}: {
  token: string
  user: User
  readOnly: boolean
}) {
  const [rows, setRows] = useState<Row[]>([])
  const [kelas, setKelas] = useState<Row[]>([])
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [submitting, setSubmitting] = useState(false)

  const isGuru = user.role === 'guru'
  // tutor wali hanya boleh target kelas miliknya; admin bebas semua kelas
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  const load = () => {
    void request('/pengumuman', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  function openAdd() {
    setForm({ ...emptyForm })
    setEditing(null)
    setAdding(true)
  }

  function openEdit(r: Row) {
    setEditing(r)
    setForm({
      judul: String(r.judul || ''),
      isi: String(r.isi || ''),
      target: String(r.target || 'semua'),
      kelasId: String(r.kelasId || ''),
      aktif: r.aktif !== false,
      tanggalMulai: fmtDate(r.tanggalMulai),
      tanggalSelesai: fmtDate(r.tanggalSelesai),
    })
    setAdding(true)
  }

  function canEdit(r: Row): boolean {
    if (user.role === 'admin') return true
    return String(r.dibuatOlehUserId || '') === user.id
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.judul.trim()) {
      toast.error('Judul wajib diisi.')
      return
    }
    if (form.target === 'kelas' && !form.kelasId) {
      toast.error('Pilih kelas target.')
      return
    }
    const payload: Record<string, unknown> = {
      judul: form.judul,
      isi: form.isi,
      target: form.target,
      kelasId: form.target === 'kelas' ? form.kelasId : null,
      aktif: form.aktif,
      tanggalMulai: form.tanggalMulai ? wibDateTimeLocalToISO(form.tanggalMulai) : null,
      tanggalSelesai: form.tanggalSelesai ? wibDateTimeLocalToISO(form.tanggalSelesai) : null,
    }
    setSubmitting(true)
    try {
      if (editing) {
        await request('/pengumuman/' + editing.id, token, 'PUT', payload)
        toast.success('Pengumuman diperbarui.')
      } else {
        await request('/pengumuman', token, 'POST', payload)
        toast.success('Pengumuman dibuat.')
      }
      setAdding(false)
      setEditing(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan pengumuman.')
    } finally {
      setSubmitting(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/pengumuman/' + deletingRow.id, token, 'DELETE')
      toast.success('Pengumuman dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus pengumuman.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Pengumuman"
        description={
          isGuru
            ? 'Buat pengumuman untuk rombel yang Anda walikan.'
            : 'Kelola pengumuman untuk seluruh rombel atau per kelas.'
        }
        actions={
          !readOnly && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Buat pengumuman
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard
          title={editing ? 'Edit Pengumuman' : 'Buat Pengumuman'}
          description="Pengumuman non-aktif atau di luar rentang tanggal tidak ditampilkan."
        >
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Judul</Label>
              <Input
                value={form.judul}
                onChange={(e) => setForm({ ...form, judul: e.target.value })}
                placeholder="Judul pengumuman"
                required
              />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Isi</Label>
              <textarea
                className="flex min-h-[120px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.isi}
                onChange={(e) => setForm({ ...form, isi: e.target.value })}
                placeholder="Tulis isi pengumuman..."
              />
            </div>
            <div className="grid gap-2">
              <Label>Target</Label>
              <Select
                value={form.target}
                onChange={(e) => setForm({ ...form, target: e.target.value, kelasId: '' })}
                disabled={isGuru}
              >
                <option value="semua">Semua rombel</option>
                <option value="kelas">Per kelas</option>
              </Select>
              {isGuru && (
                <p className="text-xs text-muted-foreground">Tutor hanya dapat menargetkan kelas walinya.</p>
              )}
            </div>
            <div className="grid gap-2">
              <Label>Kelas Target</Label>
              <Select
                value={form.kelasId}
                onChange={(e) => setForm({ ...form, kelasId: e.target.value })}
                disabled={form.target !== 'kelas'}
                required={form.target === 'kelas'}
              >
                <option value="">Pilih kelas</option>
                {kelasOptions.map((k) => (
                  <option key={k.id} value={k.id}>
                    {kelasLabel(k)}
                  </option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Tanggal Mulai (opsional)</Label>
              <Input
                type="date"
                value={form.tanggalMulai}
                onChange={(e) => setForm({ ...form, tanggalMulai: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label>Tanggal Selesai (opsional)</Label>
              <Input
                type="date"
                value={form.tanggalSelesai}
                onChange={(e) => setForm({ ...form, tanggalSelesai: e.target.value })}
              />
            </div>
            <div className="flex items-center gap-2 sm:col-span-2">
              <Checkbox
                id="aktif"
                checked={form.aktif}
                onChange={(e) => setForm({ ...form, aktif: e.target.checked })}
              />
              <Label htmlFor="aktif" className="cursor-pointer">
                Aktif (tampilkan)
              </Label>
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={submitting}>{submitting ? 'Menyimpan...' : editing ? 'Simpan perubahan' : 'Simpan pengumuman'}</Button>
              <Button
                type="button"
                variant="outline"
                disabled={submitting}
                onClick={() => {
                  setAdding(false)
                  setEditing(null)
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
              <TableHead>Judul</TableHead>
              <TableHead>Target</TableHead>
              <TableHead>Rentang</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const k = (r.kelas as Row) || {}
              return (
                <TableRow key={r.id}>
                  <TableCell>
                    <div className="font-medium flex items-center gap-2">
                      <Megaphone className="h-4 w-4 text-primary" />
                      {String(r.judul || '-')}
                    </div>
                    {r.isi ? (
                      <div className="text-xs text-muted-foreground line-clamp-2 max-w-md">{String(r.isi)}</div>
                    ) : null}
                  </TableCell>
                  <TableCell>
                    {r.target === 'kelas' ? (
                      <Badge variant="outline">{kelasLabel(k) || 'Per kelas'}</Badge>
                    ) : (
                      <Badge variant="secondary">Semua</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {fmtDate(r.tanggalMulai) || '—'} s/d {fmtDate(r.tanggalSelesai) || '—'}
                  </TableCell>
                  <TableCell>
                    {r.aktif === false ? (
                      <Badge variant="outline">Non-aktif</Badge>
                    ) : (
                      <Badge variant="default">Aktif</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      {!readOnly && canEdit(r) && (
                        <Button size="sm" variant="outline" aria-label="Ubah" onClick={() => openEdit(r)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                      )}
                      {!readOnly && user.role === 'admin' && (
                        <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => setDeletingRow(r)}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
            {!rows.length && <EmptyState colSpan={5} label="Belum ada pengumuman." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Pengumuman?</AlertDialogTitle>
            <AlertDialogDescription>
              Pengumuman <strong>{String(deletingRow?.judul || '')}</strong> akan dihapus permanen.
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
