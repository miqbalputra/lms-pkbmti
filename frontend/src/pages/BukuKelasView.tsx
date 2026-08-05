import { useEffect, useState, type FormEvent } from 'react'
import { Info, Plus, Trash2 } from 'lucide-react'
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
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
type Row = Record<string, unknown> & { id: string }

async function request(path: string, token: string, method = 'GET', body?: unknown) {
  const r = await fetch(apiBase + path, {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!r.ok) {
    const x = await r.json().catch(() => ({}))
    throw new Error((x as any)?.error || `Permintaan gagal (${r.status}).`)
  }
  return r.status === 204 ? null : r.json()
}

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

export function BukuKelasView({ token, readOnly }: { token: string; readOnly: boolean }) {
  const [rows, setRows] = useState<Row[]>([])
  const [kelas, setKelas] = useState<Row[]>([])
  const [buku, setBuku] = useState<Row[]>([])
  const [adding, setAdding] = useState(false)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [kelasId, setKelasId] = useState('')
  const [bukuId, setBukuId] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const load = () => {
    void request('/buku-kelas', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
    void request('/buku', token).then((r: Row[]) => setBuku(r || [])).catch(() => setBuku([]))
  }, [token])

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!kelasId || !bukuId) {
      toast.error('Pilih kelas dan buku terlebih dahulu.')
      return
    }
    setSubmitting(true)
    try {
      await request('/buku-kelas', token, 'POST', { kelasId, bukuId })
      toast.success('Penetapan buku berhasil disimpan. Semester diisi otomatis.')
      setAdding(false)
      setKelasId('')
      setBukuId('')
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan penetapan.')
    } finally {
      setSubmitting(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/buku-kelas/' + deletingRow.id, token, 'DELETE')
      toast.success('Penetapan buku dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus penetapan.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Penetapan Buku per Kelas"
        description="Tetapkan buku modul yang dipinjamkan ke setiap rombel. Semester diisi otomatis dari tahun ajaran aktif."
        actions={
          !readOnly && (
            <Button onClick={() => setAdding((v) => !v)}>
              <Plus className="h-4 w-4" />
              Tambah penetapan
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard
          title="Tetapkan buku ke kelas"
          description="Semester ditentukan otomatis berdasarkan tanggal hari ini dan titik potong semester tahun ajaran aktif."
        >
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2">
              <Label>Kelas / Rombel</Label>
              <Select value={kelasId} onChange={(e) => setKelasId(e.target.value)} required>
                <option value="">Pilih kelas</option>
                {kelas.map((k) => (
                  <option key={k.id} value={k.id}>
                    {kelasLabel(k)}
                  </option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Buku</Label>
              <Select value={bukuId} onChange={(e) => setBukuId(e.target.value)} required>
                <option value="">Pilih buku</option>
                {buku.map((b) => (
                  <option key={b.id} value={b.id}>
                    {String(b.judul)}
                    {b.kodeBuku ? ` (${String(b.kodeBuku)})` : ''}
                  </option>
                ))}
              </Select>
            </div>
            <div className="flex items-start gap-2 rounded-xl border border-border bg-secondary/40 p-3 sm:col-span-2">
              <Info className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
              <p className="text-xs text-muted-foreground">
                Semester tidak perlu dipilih — sistem mengisinya otomatis (Ganjil/Genap) sesuai titik potong semester
                tahun ajaran aktif. Buku yang sudah ditetapkan untuk kelas & semester yang sama akan ditolak.
              </p>
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={submitting}>{submitting ? 'Menyimpan...' : 'Simpan penetapan'}</Button>
              <Button type="button" variant="outline" disabled={submitting} onClick={() => setAdding(false)}>
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
              <TableHead>Kelas</TableHead>
              <TableHead>Buku</TableHead>
              <TableHead>Tahun Ajaran</TableHead>
              <TableHead>Semester</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const k = (r.kelas as Row) || {}
              const b = (r.buku as Row) || {}
              const ta = (k.tahunAjaran as Row) || {}
              return (
                <TableRow key={r.id}>
                  <TableCell className="font-medium">{kelasLabel(k)}</TableCell>
                  <TableCell>
                    <div className="font-medium">{String(b.judul || '-')}</div>
                    {b.kodeBuku ? (
                      <div className="text-xs text-muted-foreground">Kode: {String(b.kodeBuku)}</div>
                    ) : null}
                  </TableCell>
                  <TableCell className="text-sm">{String(ta.namaTahunAjaran || '-')}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{String(r.semester || '-')}</Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end">
                      {!readOnly && (
                        <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => setDeletingRow(r)}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
            {!rows.length && <EmptyState colSpan={5} label="Belum ada buku yang ditetapkan." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Penetapan Buku?</AlertDialogTitle>
            <AlertDialogDescription>
              Penetapan buku{' '}
              <strong>{String((deletingRow?.buku as Row)?.judul || '')}</strong> untuk kelas{' '}
              <strong>{kelasLabel((deletingRow?.kelas as Row) || {})}</strong> akan dihapus. Peminjaman yang sudah
              tercatat tidak terpengaruh.
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