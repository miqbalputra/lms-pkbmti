import { useEffect, useState, type FormEvent } from 'react'
import { Pencil, Plus, Trash2 } from 'lucide-react'
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

const emptyForm = { mapelId: '', tipe: 'pg', pertanyaan: '', opsi: [''], kunci: '0', poin: '1' }

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
  const [submitting, setSubmitting] = useState(false)

  const load = () => {
    void request('/bank-soal', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  function openAdd() {
    setForm({ ...emptyForm, opsi: [''] })
    setEditing(null)
    setAdding(true)
  }

  function openEdit(r: Row) {
    setEditing(r)
    const opsi = parseOpsi(r.opsi)
    setForm({
      mapelId: String(r.mapelId || ''),
      tipe: String(r.tipe || 'pg'),
      pertanyaan: String(r.pertanyaan || ''),
      opsi: opsi.length ? opsi : [''],
      kunci: String(r.kunci || '0'),
      poin: String(r.poin ?? '1'),
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
      return { ...f, opsi: opsi.length ? opsi : [''], kunci: String(Math.min(Number(f.kunci), Math.max(0, opsi.length - 1))) }
    })
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.pertanyaan.trim()) {
      toast.error('Pertanyaan wajib diisi.')
      return
    }
    const isPg = form.tipe === 'pg'
    const opsi = isPg ? form.opsi.map((s) => s.trim()).filter(Boolean) : []
    if (isPg && opsi.length < 2) {
      toast.error('Soal PG minimal 2 opsi.')
      return
    }
    const payload = {
      mapelId: form.mapelId || undefined,
      tipe: form.tipe,
      pertanyaan: form.pertanyaan,
      opsi: isPg ? JSON.stringify(opsi) : '',
      kunci: isPg ? String(Math.min(Number(form.kunci), opsi.length - 1)) : form.kunci,
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
    <div className="space-y-4">
      <PageToolbar
        title="Bank Soal"
        description="Kumpulan soal pilihan ganda & essay per mapel. Dipakai menyusun ujian luring."
        actions={
          !readOnly && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Tambah soal
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard title={editing ? 'Edit Soal' : 'Tambah Soal'} description="Opsi (PG) disimpan sebagai daftar; kunci = indeks opsi yang benar.">
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
              <Label>Tipe</Label>
              <Select value={form.tipe} onChange={(e) => setForm({ ...form, tipe: e.target.value })}>
                <option value="pg">Pilihan Ganda</option>
                <option value="essay">Essay</option>
              </Select>
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
            {form.tipe === 'pg' ? (
              <div className="grid gap-2 sm:col-span-2">
                <Label>Opsi Jawaban</Label>
                <div className="space-y-2">
                  {form.opsi.map((op, i) => (
                    <div key={i} className="flex items-center gap-2">
                      <span className="text-sm font-medium w-5">{String.fromCharCode(65 + i)}.</span>
                      <Input value={op} onChange={(e) => setOpsi(i, e.target.value)} placeholder={`Opsi ${String.fromCharCode(65 + i)}`} />
                      {form.opsi.length > 1 && (
                        <Button type="button" size="sm" variant="outline" aria-label="Hapus opsi" onClick={() => removeOpsi(i)}><Trash2 className="h-3.5 w-3.5" /></Button>
                      )}
                    </div>
                  ))}
                </div>
                <Button type="button" variant="outline" size="sm" className="w-fit" onClick={addOpsi}><Plus className="h-3.5 w-3.5" /> Tambah opsi</Button>
              </div>
            ) : null}
            <div className="grid gap-2">
              <Label>{form.tipe === 'pg' ? 'Kunci (opsi benar)' : 'Kunci Jawaban'}</Label>
              {form.tipe === 'pg' ? (
                <Select value={String(Math.min(Number(form.kunci), Math.max(0, form.opsi.length - 1)))} onChange={(e) => setForm({ ...form, kunci: e.target.value })}>
                  {form.opsi.map((_, i) => (
                    <option key={i} value={i}>{String.fromCharCode(65 + i)}</option>
                  ))}
                </Select>
              ) : (
                <Input value={form.kunci} onChange={(e) => setForm({ ...form, kunci: e.target.value })} placeholder="Kunci jawaban essay" />
              )}
            </div>
            <div className="grid gap-2">
              <Label>Poin</Label>
              <Input type="number" value={form.poin} onChange={(e) => setForm({ ...form, poin: e.target.value })} />
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
              const kunciIdx = Number(r.kunci)
              return (
                <TableRow key={r.id}>
                  <TableCell>
                    <div className="font-medium line-clamp-2 max-w-md">{String(r.pertanyaan || '-')}</div>
                    {r.tipe === 'pg' && opsi.length ? (
                      <div className="text-xs text-muted-foreground mt-1 space-y-0.5">
                        {opsi.map((op, i) => (
                          <div key={i} className={i === kunciIdx ? 'text-success font-medium' : ''}>
                            {String.fromCharCode(65 + i)}. {op}{i === kunciIdx ? ' ✓' : ''}
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </TableCell>
                  <TableCell>{String(m.namaMapel || '-')}</TableCell>
                  <TableCell><Badge variant={r.tipe === 'pg' ? 'secondary' : 'outline'}>{r.tipe === 'pg' ? 'PG' : 'Essay'}</Badge></TableCell>
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