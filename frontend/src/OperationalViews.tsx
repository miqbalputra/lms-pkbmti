import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Download, FileSpreadsheet, FileText, History, Pencil, Plus, Trash2, Users } from 'lucide-react'
import { toast } from 'sonner'
import { downloadFile } from './lib/api'
import { ClassEditor } from './ClassEditor'
import { ClassHistory } from './ClassHistory'
import { StudentEditor } from './StudentEditor'
import { StudentImport } from './StudentImport'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from './components/ui/alert-dialog'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { EmptyState, FormCard, PageToolbar } from './components/ui/page'
import { Select } from './components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './components/ui/table'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
type Row = Record<string, unknown> & { id: string }
type Options = { kelas: Row[]; tutor: Row[]; pokjar: Row[]; years: Row[]; parents: Row[]; mapel: Row[] }

export function ClassesView({ token, readOnly }: { token: string; readOnly: boolean }) {
  const [rows, setRows] = useState<Row[]>([])
  const [options, setOptions] = useState<Options>()
  const [adding, setAdding] = useState(false)
  const [history, setHistory] = useState<Row>()
  const [editing, setEditing] = useState<Row>()
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const load = useCallback(() => request('/kelas', token).then(setRows), [token])

  useEffect(() => {
    void load()
    if (!readOnly) void loadOptions(token).then(setOptions)
  }, [load, readOnly, token])

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setSubmitting(true)
    try {
      const body: Record<string, unknown> = Object.fromEntries(new FormData(e.currentTarget))
      body.jenjang = Number(body.jenjang)
      await request('/kelas', token, 'POST', body)
      toast.success('Rombongan belajar berhasil dibuat.')
      setAdding(false)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal membuat rombel.')
    } finally {
      setSubmitting(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/kelas/' + deletingRow.id, token, 'DELETE')
      toast.success(`Kelas ${String(deletingRow.jenjang)}${String(deletingRow.namaRombel)} berhasil dihapus.`)
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus kelas.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Kelas & Rombongan Belajar"
        description={`${rows.length} rombel aktif terdaftar dalam sistem.`}
        actions={
          !readOnly && (
            <Button onClick={() => setAdding(true)}>
              <Plus className="h-4 w-4" />
              Buat rombel
            </Button>
          )
        }
      />

      {editing && (
        <ClassEditor
          classRow={editing}
          token={token}
          close={() => setEditing(undefined)}
          saved={load}
        />
      )}

      {history && (
        <ClassHistory
          classID={history.id}
          title={`Kelas ${String(history.jenjang)}${String(history.namaRombel)}`}
          token={token}
          close={() => setHistory(undefined)}
        />
      )}

      {adding && options && (
        <FormCard title="Buat rombel" description="Tentukan jenjang, lokasi, periode, dan wali kelas.">
          <form className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5" onSubmit={submit}>
            <Field label="Jenjang">
              <Select name="jenjang" defaultValue="1">
                {[1, 2, 3, 4, 5, 6].map((n) => (
                  <option key={n}>{n}</option>
                ))}
              </Select>
            </Field>
            <Field label="Nama rombel">
              <Input name="namaRombel" placeholder="A" required />
            </Field>
            <Picker label="Pokjar" name="pokjarId" rows={options.pokjar} field="namaPokjar" />
            <Picker label="Tahun ajaran" name="tahunAjaranId" rows={options.years} field="namaTahunAjaran" />
            <Picker label="Wali kelas" name="waliKelasId" rows={options.tutor} field="nama" optional />
            <div className="flex gap-2 sm:col-span-2 lg:col-span-5">
              <Button disabled={submitting}>{submitting ? 'Menyimpan...' : 'Simpan'}</Button>
              <Button variant="outline" type="button" disabled={submitting} onClick={() => setAdding(false)}>
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
              <TableHead>Rombel</TableHead>
              <TableHead>Pokjar</TableHead>
              <TableHead>Wali kelas</TableHead>
              <TableHead>Tahun ajaran</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => (
              <TableRow key={r.id}>
                <TableCell className="font-medium">
                  Kelas {String(r.jenjang)}{String(r.namaRombel)}
                </TableCell>
                <TableCell>{String((r.pokjar as Row)?.namaPokjar || '-')}</TableCell>
                <TableCell>{String((r.waliKelas as Row)?.nama || 'Belum ditetapkan')}</TableCell>
                <TableCell>{String((r.tahunAjaran as Row)?.namaTahunAjaran || '-')}</TableCell>
                <TableCell>
                  <div className="flex justify-end gap-2">
                    <Button size="sm" variant="outline" onClick={() => setHistory(r)}>
                      <History className="h-3.5 w-3.5" />
                      Riwayat
                    </Button>
                    {!readOnly && (
                      <>
                        <Button size="sm" variant="outline" onClick={() => setEditing(r)}>
                          <Pencil className="h-3.5 w-3.5" />
                          Wali
                        </Button>
                        <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => setDeletingRow(r)}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {!rows.length && <EmptyState colSpan={5} />}
          </TableBody>
        </Table>
      </Card>

      {/* Delete Modal Confirmation */}
      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Rombongan Belajar?</AlertDialogTitle>
            <AlertDialogDescription>
              Apakah Anda yakin ingin menghapus{' '}
              <strong>
                Kelas {String(deletingRow?.jenjang)}{String(deletingRow?.namaRombel)}
              </strong>
              ? Tindakan ini tidak dapat dibatalkan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Batal</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={confirmDelete}
              disabled={isDeleting}
            >
              {isDeleting ? 'Menghapus...' : 'Hapus Rombel'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export function StudentsView({ token, readOnly }: { token: string; readOnly: boolean }) {
  const [rows, setRows] = useState<Row[]>([])
  const [o, setO] = useState<Options>()
  const [editing, setEditing] = useState<Row>()
  const [importing, setImporting] = useState(false)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const load = useCallback(() => request('/peserta-didik', token).then(setRows), [token])

  useEffect(() => {
    void load()
    if (!readOnly) void loadOptions(token).then(setO)
  }, [load, readOnly, token])

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/peserta-didik/' + deletingRow.id, token, 'DELETE')
      toast.success(`Peserta didik ${String(deletingRow.nama)} berhasil dihapus.`)
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus peserta didik.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Data Peserta Didik"
        description={`${rows.length} peserta didik aktif tercatat.`}
        actions={
          <>
            <Button
              variant="outline"
              onClick={() => downloadFile('/peserta-didik/export?format=xlsx', token, 'daftar-peserta-didik.xlsx').catch((e) => toast.error(e.message || 'Gagal mengunduh Excel.'))}
            >
              <FileSpreadsheet className="h-4 w-4" />
              Excel
            </Button>
            <Button
              variant="outline"
              onClick={() => downloadFile('/peserta-didik/export?format=pdf', token, 'daftar-peserta-didik.pdf').catch((e) => toast.error(e.message || 'Gagal mengunduh PDF.'))}
            >
              <FileText className="h-4 w-4" />
              PDF
            </Button>
            {!readOnly && (
              <>
                <Button variant="outline" onClick={() => setImporting(true)}>
                  <Download className="h-4 w-4" />
                  Import Excel
                </Button>
                <Button onClick={() => setEditing({} as Row)}>
                  <Plus className="h-4 w-4" />
                  Tambah Siswa
                </Button>
              </>
            )}
          </>
        }
      />
      {importing && <StudentImport token={token} close={() => setImporting(false)} done={load} />}
      {editing && o && (
        <StudentEditor
          row={editing.id ? editing : undefined}
          options={o}
          token={token}
          close={() => setEditing(undefined)}
          saved={load}
        />
      )}
      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Peserta didik</TableHead>
              <TableHead>NIS / NISN</TableHead>
              <TableHead>Kelas</TableHead>
              <TableHead>Status</TableHead>
              {!readOnly && <TableHead className="text-right">Aksi</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => (
              <TableRow key={r.id}>
                <TableCell>
                  <div className="font-medium">{String(r.nama)}</div>
                  <div className="text-xs text-muted-foreground">{String(r.jenisKelamin)}</div>
                </TableCell>
                <TableCell>
                  {String(r.nis || '-')} / {String(r.nisn || '-')}
                </TableCell>
                <TableCell>
                  Kelas {String((r.kelas as Row)?.jenjang || '')}
                  {String((r.kelas as Row)?.namaRombel || '')}
                </TableCell>
                <TableCell>
                  <Badge variant="secondary">{String(r.status)}</Badge>
                </TableCell>
                {!readOnly && (
                  <TableCell>
                    <div className="flex justify-end gap-2">
                      <Button size="sm" variant="outline" onClick={() => setEditing(r)}>
                        <Pencil className="h-3.5 w-3.5" />
                        Ubah
                      </Button>
                      <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => setDeletingRow(r)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {!rows.length && <EmptyState colSpan={readOnly ? 4 : 5} />}
          </TableBody>
        </Table>
      </Card>

      {/* Delete Student Modal Confirmation */}
      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Peserta Didik?</AlertDialogTitle>
            <AlertDialogDescription>
              Apakah Anda yakin ingin menghapus peserta didik <strong>{String(deletingRow?.nama)}</strong>?
              Data riwayat dan entri terkait akan dihapus secara permanen.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Batal</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={confirmDelete}
              disabled={isDeleting}
            >
              {isDeleting ? 'Menghapus...' : 'Hapus Siswa'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export function AssignmentsView({ token, readOnly }: { token: string; readOnly: boolean }) {
  const [rows, setRows] = useState<Row[]>([])
  const [o, setO] = useState<Options>()
  const [adding, setAdding] = useState(false)
  const [bulk, setBulk] = useState(false)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const load = useCallback(() => request('/penugasan', token).then(setRows), [token])

  useEffect(() => {
    void load()
    void loadOptions(token).then(setO)
  }, [load, token])

  async function submit(e: FormEvent<HTMLFormElement>, all = false) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await request(
        all ? '/penugasan/semua-kelas' : '/penugasan',
        token,
        'POST',
        Object.fromEntries(new FormData(e.currentTarget))
      )
      toast.success(all ? 'Guru berhasil ditugaskan ke semua kelas.' : 'Penugasan guru berhasil disimpan.')
      setAdding(false)
      setBulk(false)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan penugasan guru.')
    } finally {
      setSubmitting(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/penugasan/' + deletingRow.id, token, 'DELETE')
      toast.success('Penugasan tutor berhasil dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus penugasan.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Penugasan Tutor"
        description="Atur akses penugasan tutor untuk pengajaran kombinasi rombel dan mata pelajaran."
        actions={
          !readOnly && (
            <>
              <Button variant="outline" onClick={() => setBulk(true)}>
                <Users className="h-4 w-4" />
                Semua kelas mapel
              </Button>
              <Button onClick={() => setAdding(true)}>
                <Plus className="h-4 w-4" />
                Tugaskan tutor
              </Button>
            </>
          )
        }
      />
      {bulk && o && (
        <AssignmentForm
          title="Tugaskan tutor ke semua kelas mapel"
          options={o}
          close={() => setBulk(false)}
          submit={(e) => void submit(e, true)}
          submitting={submitting}
          bulk
        />
      )}
      {adding && o && (
        <AssignmentForm
          title="Tugaskan tutor"
          options={o}
          close={() => setAdding(false)}
          submit={(e) => void submit(e)}
          submitting={submitting}
        />
      )}
      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Tutor</TableHead>
              <TableHead>Kelas</TableHead>
              <TableHead>Mata Pelajaran</TableHead>
              {!readOnly && <TableHead className="text-right">Aksi</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const preTutor = r.tutor as Row | undefined
              const preKelas = r.kelas as Row | undefined
              const preMapel = r.mapel as Row | undefined
              const tutor = preTutor?.nama ? preTutor : o?.tutor.find((t) => t.id === r.tutorId)
              const kelas = preKelas?.namaRombel != null ? preKelas : o?.kelas.find((k) => k.id === r.kelasId)
              const mapel = preMapel?.namaMapel ? preMapel : o?.mapel.find((m) => m.id === r.mapelId)
              return (
                <TableRow key={r.id}>
                  <TableCell className="font-medium text-foreground">
                    {tutor ? String(tutor.nama) : <span className="font-mono text-xs text-muted-foreground">{String(r.tutorId)}</span>}
                  </TableCell>
                  <TableCell>
                    {kelas ? `Kelas ${String(kelas.jenjang)}${String(kelas.namaRombel)}` : <span className="font-mono text-xs text-muted-foreground">{String(r.kelasId)}</span>}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {mapel ? String(mapel.namaMapel) : <span className="font-mono text-xs text-muted-foreground">{String(r.mapelId)}</span>}
                  </TableCell>
                  {!readOnly && (
                    <TableCell className="text-right">
                      <Button size="sm" variant="destructive" onClick={() => setDeletingRow(r)}>
                        <Trash2 className="h-3.5 w-3.5" />
                        Hapus
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              )
            })}
            {!rows.length && <EmptyState colSpan={readOnly ? 3 : 4} />}
          </TableBody>
        </Table>
      </Card>

      {/* Delete Assignment Modal Confirmation */}
      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Penugasan Guru?</AlertDialogTitle>
            <AlertDialogDescription>
              Apakah Anda yakin ingin menghapus hak akses penugasan guru ini?
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Batal</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={confirmDelete}
              disabled={isDeleting}
            >
              {isDeleting ? 'Menghapus...' : 'Hapus Penugasan'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export function ArchiveView({ token }: { token: string }) {
  const [years, setYears] = useState<Row[]>([])
  const [selected, setSelected] = useState('')
  const [semester, setSemester] = useState('')
  const [data, setData] = useState<{ riwayatKelas: Row[]; presensi: Row[] }>({
    riwayatKelas: [],
    presensi: [],
  })

  useEffect(() => {
    void request('/tahun-ajaran', token).then(setYears)
  }, [token])

  function load() {
    void request(`/arsip?tahunAjaranId=${selected}&semester=${semester}`, token).then(setData).catch(() => setData({ riwayatKelas: [], presensi: [] }))
  }

  return (
    <div className="space-y-4">
      <PageToolbar title="Arsip Akademik" description="Pilih tahun ajaran dan semester untuk menampilkan data historis." />
      <Card>
        <CardHeader>
          <CardTitle>Filter Arsip Periode</CardTitle>
          <CardDescription>Tentukan tahun ajaran dan semester yang ingin ditinjau.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-3">
            <Field label="Tahun ajaran">
              <Select value={selected} onChange={(e) => setSelected(e.target.value)}>
                <option value="">Pilih tahun ajaran</option>
                {years.map((y) => (
                  <option key={y.id} value={y.id}>
                    {String(y.namaTahunAjaran)}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Semester">
              <Select value={semester} onChange={(e) => setSemester(e.target.value)}>
                <option value="">Pilih semester</option>
                <option>Ganjil</option>
                <option>Genap</option>
              </Select>
            </Field>
            <div className="flex items-end">
              <Button disabled={!selected || !semester} onClick={load}>
                Tampilkan
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
      <div className="grid gap-4 sm:grid-cols-3">
        <Metric label="Riwayat peserta didik" value={data?.riwayatKelas?.length ?? 0} />
        <Metric label="Pertemuan presensi" value={data?.presensi?.length ?? 0} />
        <Metric label="Semester" value={semester || '-'} />
      </div>
      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Peserta didik</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Kelas</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data?.riwayatKelas?.map((r) => (
              <TableRow key={r.id}>
                <TableCell className="font-mono text-xs">{String(r.pesertaDidikId)}</TableCell>
                <TableCell>
                  <Badge variant="outline">{String(r.status)}</Badge>
                </TableCell>
                <TableCell>
                  Kelas {String((r.kelas as Row)?.jenjang || '')}
                  {String((r.kelas as Row)?.namaRombel || '')}
                </TableCell>
              </TableRow>
            ))}
            {!data?.riwayatKelas?.length && (
              <EmptyState colSpan={3} label="Pilih periode untuk melihat arsip." />
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}

function AssignmentForm({
  title,
  options,
  close,
  submit,
  bulk = false,
  submitting = false,
}: {
  title: string
  options: Options
  close: () => void
  submit: (e: FormEvent<HTMLFormElement>) => void
  bulk?: boolean
  submitting?: boolean
}) {
  return (
    <FormCard title={title}>
      <form className="grid gap-4 sm:grid-cols-3" onSubmit={submit}>
        <Picker label="Tutor" name="tutorId" rows={options.tutor} field="nama" />
        {!bulk && <Picker label="Kelas" name="kelasId" rows={options.kelas} field="namaRombel" />}
        <Picker label="Mata pelajaran" name="mapelId" rows={options.mapel} field="namaMapel" />
        {bulk && (
          <Picker
            label="Tahun ajaran (opsional)"
            name="tahunAjaranId"
            rows={options.years}
            field="namaTahunAjaran"
            optional
          />
        )}
        <div className="flex gap-2 sm:col-span-3">
          <Button disabled={submitting}>{submitting ? 'Menyimpan...' : 'Simpan'}</Button>
          <Button type="button" variant="outline" disabled={submitting} onClick={close}>
            Batal
          </Button>
        </div>
      </form>
    </FormCard>
  )
}

function Picker({
  label,
  name,
  rows,
  field,
  optional = false,
}: {
  label: string
  name: string
  rows: Row[]
  field: string
  optional?: boolean
}) {
  return (
    <Field label={label}>
      <Select name={name} required={!optional}>
        <option value="">{optional ? 'Semua tahun ajaran' : `Pilih ${label.toLowerCase()}`}</option>
        {rows.map((r) => (
          <option key={r.id} value={r.id}>
            {field === 'namaRombel'
              ? `Kelas ${String(r.jenjang)}${String(r.namaRombel)}`
              : String(r[field])}
          </option>
        ))}
      </Select>
    </Field>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid gap-2">
      <Label>{label}</Label>
      {children}
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string | number }) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl">{value}</CardTitle>
      </CardHeader>
    </Card>
  )
}

async function loadOptions(token: string): Promise<Options> {
  const [kelas, tutor, pokjar, years, parents, mapel] = await Promise.all(
    ['/kelas', '/tutor', '/pokjar', '/tahun-ajaran', '/orang-tua', '/mapel'].map((p) =>
      request(p, token)
    )
  )
  return { kelas, tutor, pokjar, years, parents, mapel }
}

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
  const result = r.status === 204 ? null : await r.json().catch(() => ({}))
  if (!r.ok) throw new Error(result?.error || 'Permintaan gagal')
  return result
}
