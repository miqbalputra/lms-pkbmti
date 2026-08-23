import { useEffect, useState, type FormEvent } from 'react'
import { ExternalLink, Pencil, Plus, Trash2, Video } from 'lucide-react'
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
import { formatWibDateTime, wibDateTimeLocalToISO, wibDateTimeLocalValue } from '../lib/wib'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

function fmtDateTime(v: unknown): string {
  return formatWibDateTime(v)
}

const emptyForm = {
  mapelId: '',
  kelasId: '',
  judul: '',
  deskripsi: '',
  linkMeeting: '',
  waktuMulai: '',
  waktuSelesai: '',
}

export function KelasVirtualView({
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
  const [submitting, setSubmitting] = useState(false)

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  const load = () => {
    void request('/kelas-virtual', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
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
      mapelId: String(r.mapelId || ''),
      kelasId: String(r.kelasId || ''),
      judul: String(r.judul || ''),
      deskripsi: String(r.deskripsi || ''),
      linkMeeting: String(r.linkMeeting || ''),
      waktuMulai: wibDateTimeLocalValue(r.waktuMulai),
      waktuSelesai: wibDateTimeLocalValue(r.waktuSelesai),
    })
    setAdding(true)
  }

  function canEdit(r: Row): boolean {
    if (user.role === 'admin') return true
    return String(r.dibuatOlehUserId || '') === user.id
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.judul || !form.kelasId || !form.linkMeeting) {
      toast.error('Judul, kelas, dan link meeting wajib diisi.')
      return
    }
    const payload = {
      mapelId: form.mapelId || undefined,
      kelasId: form.kelasId,
      judul: form.judul,
      deskripsi: form.deskripsi,
      linkMeeting: form.linkMeeting,
      waktuMulai: form.waktuMulai ? wibDateTimeLocalToISO(form.waktuMulai) : undefined,
      waktuSelesai: form.waktuSelesai ? wibDateTimeLocalToISO(form.waktuSelesai) : undefined,
    }
    setSubmitting(true)
    try {
      if (editing) {
        await request('/kelas-virtual/' + editing.id, token, 'PUT', payload)
        toast.success('Kelas virtual diperbarui.')
      } else {
        await request('/kelas-virtual', token, 'POST', payload)
        toast.success('Kelas virtual dibuat.')
      }
      setAdding(false)
      setEditing(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan kelas virtual.')
    } finally {
      setSubmitting(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/kelas-virtual/' + deletingRow.id, token, 'DELETE')
      toast.success('Kelas virtual dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus kelas virtual.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Kelas Virtual"
        description="Jadwal kelas daring (link meeting) per mapel & rombel."
        actions={
          !readOnly && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Buat kelas virtual
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard title={editing ? 'Edit Kelas Virtual' : 'Buat Kelas Virtual'} description="Link meeting (Zoom/Meet) & jadwal pelaksanaan.">
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Judul</Label>
              <Input value={form.judul} onChange={(e) => setForm({ ...form, judul: e.target.value })} placeholder="Judul sesi kelas virtual" required />
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
            <div className="grid gap-2 sm:col-span-2">
              <Label>Link Meeting</Label>
              <Input value={form.linkMeeting} onChange={(e) => setForm({ ...form, linkMeeting: e.target.value })} placeholder="https://meet.example/abc-defg-hij" required />
            </div>
            <div className="grid gap-2">
              <Label>Waktu Mulai</Label>
              <Input type="datetime-local" value={form.waktuMulai} onChange={(e) => setForm({ ...form, waktuMulai: e.target.value })} />
            </div>
            <div className="grid gap-2">
              <Label>Waktu Selesai</Label>
              <Input type="datetime-local" value={form.waktuSelesai} onChange={(e) => setForm({ ...form, waktuSelesai: e.target.value })} />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Deskripsi (opsional)</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.deskripsi}
                onChange={(e) => setForm({ ...form, deskripsi: e.target.value })}
              />
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={submitting}>{submitting ? 'Menyimpan...' : editing ? 'Simpan perubahan' : 'Simpan'}</Button>
              <Button type="button" variant="outline" disabled={submitting} onClick={() => { setAdding(false); setEditing(null) }}>Batal</Button>
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
              <TableHead>Jadwal</TableHead>
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
                    <div className="font-medium flex items-center gap-2"><Video className="h-4 w-4 text-primary" />{String(r.judul || '-')}</div>
                    {r.deskripsi ? <div className="text-xs text-muted-foreground line-clamp-1 max-w-md">{String(r.deskripsi)}</div> : null}
                  </TableCell>
                  <TableCell>{String(m.namaMapel || '-')}</TableCell>
                  <TableCell>{kelasLabel(k)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    <div>{fmtDateTime(r.waktuMulai) || '—'}</div>
                    <div className="text-xs">s/d {fmtDateTime(r.waktuSelesai) || '—'}</div>
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      {r.linkMeeting ? (
                        <Button size="sm" variant="outline" onClick={() => window.open(String(r.linkMeeting), '_blank')}>
                          <ExternalLink className="h-3.5 w-3.5" /> Buka
                        </Button>
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
            {!rows.length && <EmptyState colSpan={5} label="Belum ada kelas virtual." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Kelas Virtual?</AlertDialogTitle>
            <AlertDialogDescription>Kelas virtual <strong>{String(deletingRow?.judul || '')}</strong> akan dihapus.</AlertDialogDescription>
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
