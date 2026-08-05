import { useEffect, useState, type FormEvent } from 'react'
import { Dices, KeyRound, Pencil, Plus, Printer, Trash2 } from 'lucide-react'
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

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

function fmtDateTime(v: unknown): string {
  if (!v) return ''
  return String(v).slice(0, 16).replace('T', ' ')
}

const emptyForm = {
  mapelId: '',
  kelasId: '',
  judul: '',
  waktuMulai: '',
  waktuSelesai: '',
  durasiMenit: '60',
  batasTabSwitch: '0',
  acakSoal: false,
  aksesKode: '',
}

export function UjianView({
  token,
  user,
  readOnly,
  setPage,
}: {
  token: string
  user: User
  readOnly: boolean
  setPage?: (p: string) => void
}) {
  const [rows, setRows] = useState<Row[]>([])
  const [mapel, setMapel] = useState<Row[]>([])
  const [kelas, setKelas] = useState<Row[]>([])
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [soalUjian, setSoalUjian] = useState<Row | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  function genToken(): string {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
    let t = ''
    for (let i = 0; i < 6; i++) t += chars[Math.floor(Math.random() * chars.length)]
    return t
  }

  const load = () => {
    void request('/ujian', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
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
      waktuMulai: String(r.waktuMulai || '').slice(0, 16),
      waktuSelesai: String(r.waktuSelesai || '').slice(0, 16),
      durasiMenit: String(r.durasiMenit ?? '60'),
      batasTabSwitch: String(r.batasTabSwitch ?? '0'),
      acakSoal: !!r.acakSoal,
      aksesKode: String(r.aksesKode || ''),
    })
    setAdding(true)
  }

  function canEdit(r: Row): boolean {
    if (user.role === 'admin') return true
    return String(r.dibuatOlehUserId || '') === user.id
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.judul || !form.kelasId) {
      toast.error('Judul dan kelas wajib diisi.')
      return
    }
    const payload = {
      mapelId: form.mapelId || undefined,
      kelasId: form.kelasId,
      judul: form.judul,
      waktuMulai: form.waktuMulai ? new Date(form.waktuMulai).toISOString() : undefined,
      waktuSelesai: form.waktuSelesai ? new Date(form.waktuSelesai).toISOString() : undefined,
      durasiMenit: Number(form.durasiMenit) || 0,
      batasTabSwitch: Number(form.batasTabSwitch) || 0,
      acakSoal: form.acakSoal,
      aksesKode: form.aksesKode || '',
    }
    setSubmitting(true)
    try {
      if (editing) {
        await request('/ujian/' + editing.id, token, 'PUT', payload)
        toast.success('Ujian diperbarui.')
      } else {
        await request('/ujian', token, 'POST', payload)
        toast.success('Ujian dibuat.')
      }
      setAdding(false)
      setEditing(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan ujian.')
    } finally {
      setSubmitting(false)
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/ujian/' + deletingRow.id, token, 'DELETE')
      toast.success('Ujian dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus ujian.')
    } finally {
      setIsDeleting(false)
    }
  }

  async function cetak(r: Row, kunci: boolean) {
    try {
      const res = await fetch(apiBase + '/ujian/' + r.id + '/print' + (kunci ? '?kunci=1' : ''), {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('gagal mencetak')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = (kunci ? 'kunci-' : 'naskah-') + String(r.judul || 'ujian') + '.pdf'
      a.click()
      URL.revokeObjectURL(url)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mencetak ujian.')
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Ujian (Luring)"
        description="Susun ujian dari bank soal & cetak naskah + kunci jawaban (PDF)."
        actions={
          !readOnly && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Buat ujian
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard title={editing ? 'Edit Ujian' : 'Buat Ujian'} description="Ujian luring — pengerjaan offline, cetak naskah.">
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
              <Label>Waktu Mulai</Label>
              <Input type="datetime-local" value={form.waktuMulai} onChange={(e) => setForm({ ...form, waktuMulai: e.target.value })} />
            </div>
            <div className="grid gap-2">
              <Label>Waktu Selesai</Label>
              <Input type="datetime-local" value={form.waktuSelesai} onChange={(e) => setForm({ ...form, waktuSelesai: e.target.value })} />
            </div>
            <div className="grid gap-2">
              <Label>Durasi (menit)</Label>
              <Input type="number" value={form.durasiMenit} onChange={(e) => setForm({ ...form, durasiMenit: e.target.value })} />
            </div>
            <div className="grid gap-2">
              <Label>Batas Tab Switch (0 = tanpa batas)</Label>
              <Input type="number" min="0" value={form.batasTabSwitch} onChange={(e) => setForm({ ...form, batasTabSwitch: e.target.value })} />
              <p className="text-xs text-muted-foreground">Jika terlampaui, ujian otomatis dikunci & dinilai.</p>
            </div>
            <div className="flex items-center gap-2">
              <Checkbox id="acak" checked={form.acakSoal} onChange={(e) => setForm({ ...form, acakSoal: e.target.checked })} />
              <Label htmlFor="acak" className="cursor-pointer">Acak soal & opsi (deterministik per ujian)</Label>
            </div>
            <div className="grid gap-2">
              <Label>Kode Akses Ujian Online (opsional)</Label>
              <div className="flex gap-2">
                <Input
                  value={form.aksesKode}
                  onChange={(e) => setForm({ ...form, aksesKode: e.target.value.toUpperCase() })}
                  placeholder="6 digit huruf kapital & angka"
                  maxLength={6}
                  className="flex-1 font-mono tracking-widest"
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setForm({ ...form, aksesKode: genToken() })}
                  title="Generate kode acak"
                >
                  <Dices className="h-4 w-4" />
                </Button>
              </div>
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={submitting}>{submitting ? 'Menyimpan...' : editing ? 'Simpan perubahan' : 'Simpan ujian'}</Button>
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
              <TableHead>Waktu</TableHead>
              <TableHead>Kode Akses</TableHead>
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
                    <div className="font-medium">{String(r.judul || '-')}</div>
                    {r.acakSoal ? <Badge variant="outline" className="mt-1">Acak</Badge> : null}
                  </TableCell>
                  <TableCell>{String(m.namaMapel || '-')}</TableCell>
                  <TableCell>{kelasLabel(k)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    <div>{fmtDateTime(r.waktuMulai) || '—'}</div>
                    <div className="text-xs">s/d {fmtDateTime(r.waktuSelesai) || '—'}{r.durasiMenit ? ` (${r.durasiMenit} mnt)` : ''}</div>
                    {r.batasTabSwitch ? <div className="text-xs text-orange-600">Max {r.batasTabSwitch}x pindah tab</div> : null}
                  </TableCell>
                  <TableCell>
                    {r.aksesKode ? (
                      <div className="flex items-center gap-1.5">
                        <code className="text-xs bg-muted px-1.5 py-0.5 rounded font-mono">{String(r.aksesKode)}</code>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-6 px-1.5"
                          onClick={() => {
                            navigator.clipboard.writeText(String(r.aksesKode))
                            toast.success('Kode akses disalin!')
                          }}
                          title="Salin kode"
                        >
                          <svg className="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" /></svg>
                        </Button>
                        {!readOnly && canEdit(r) && (
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 px-1.5"
                            onClick={async () => {
                              const newCode = genToken()
                              try {
                                await request('/ujian/' + r.id, token, 'PUT', { ...r, aksesKode: newCode })
                                toast.success('Kode akses baru: ' + newCode)
                                load()
                              } catch (e: any) {
                                toast.error(e.message || 'Gagal regenerate.')
                              }
                            }}
                            title="Generate kode baru"
                          >
                            <Dices className="h-3 w-3" />
                          </Button>
                        )}
                      </div>
                    ) : (
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-6 px-1.5 text-xs"
                        onClick={async () => {
                          const newCode = genToken()
                          try {
                            await request('/ujian/' + r.id, token, 'PUT', { ...r, aksesKode: newCode })
                            toast.success('Kode akses: ' + newCode)
                            load()
                          } catch (e: any) {
                            toast.error(e.message || 'Gagal generate kode.')
                          }
                        }}
                        title="Generate kode akses"
                      >
                        <Dices className="h-3 w-3 mr-1" /> Generate
                      </Button>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1 flex-wrap">
                      {Boolean(r.aksesKode) && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => window.open('/api/ujian-online/page', '_blank')}
                          title="Buka halaman ujian siswa"
                        >
                          🎓 Siswa
                        </Button>
                      )}
                      {Boolean(r.aksesKode) && setPage && (
                        <Button
                          size="sm"
                          variant="default"
                          onClick={() => setPage('ujian-monitor')}
                          title="Monitor ujian berlangsung"
                        >
                          📊 Monitor
                        </Button>
                      )}
                      {!readOnly && canEdit(r) && (
                        <Button size="sm" variant="default" onClick={() => setSoalUjian(r)}>
                          <Plus className="h-3.5 w-3.5" /> Soal
                        </Button>
                      )}
                      <Button size="sm" variant="outline" onClick={() => cetak(r, false)}><Printer className="h-3.5 w-3.5" /> Naskah</Button>
                      <Button size="sm" variant="outline" onClick={() => cetak(r, true)}><KeyRound className="h-3.5 w-3.5" /> Kunci</Button>
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
            {!rows.length && <EmptyState colSpan={5} label="Belum ada ujian." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Ujian?</AlertDialogTitle>
            <AlertDialogDescription>Ujian <strong>{String(deletingRow?.judul || '')}</strong> beserta kaitan soalnya akan dihapus.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Batal</AlertDialogCancel>
            <AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={confirmDelete} disabled={isDeleting}>
              {isDeleting ? 'Menghapus...' : 'Hapus'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {soalUjian && (
        <PilihSoalDialog token={token} ujian={soalUjian} readOnly={readOnly} onClose={() => setSoalUjian(null)} />
      )}
    </div>
  )
}

function PilihSoalDialog({
  token,
  ujian,
  readOnly,
  onClose,
}: {
  token: string
  ujian: Row
  readOnly: boolean
  onClose: () => void
}) {
  const [bank, setBank] = useState<Row[]>([])
  const [attached, setAttached] = useState<Row[]>([])
  const [bobotMap, setBobotMap] = useState<Record<string, string>>({})

  const load = () => {
    void request('/bank-soal', token).then((r: Row[]) => setBank(r || [])).catch(() => setBank([]))
    void request('/ujian/' + ujian.id + '/soal', token).then((r: Row[]) => setAttached(r || [])).catch(() => setAttached([]))
  }

  useEffect(() => {
    load()
  }, [ujian.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const attachedBySoal = new Map(attached.map((a) => [String(a.soalId), a]))

  function bobotOf(soalId: string, fallback: number): string {
    if (bobotMap[soalId] !== undefined) return bobotMap[soalId]
    const a = attachedBySoal.get(soalId)
    return a ? String(a.bobot) : String(fallback)
  }

  async function toggle(soal: Row) {
    const sid = soal.id
    const isAttached = attachedBySoal.has(sid)
    if (readOnly) return
    try {
      if (isAttached) {
        await request('/ujian/' + ujian.id + '/soal/' + sid, token, 'DELETE')
      } else {
        await request('/ujian/' + ujian.id + '/soal', token, 'POST', {
          soalId: sid,
          bobot: Number(bobotOf(sid, Number(soal.poin) || 1)) || 0,
        })
      }
      load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengubah kaitan soal.')
    }
  }

  async function saveBobot(soal: Row) {
    const sid = soal.id
    const bobot = Number(bobotMap[sid])
    if (isNaN(bobot)) {
      toast.error('Bobot harus angka.')
      return
    }
    try {
      await request('/ujian/' + ujian.id + '/soal', token, 'POST', { soalId: sid, bobot })
      toast.success('Bobot disimpan.')
      setBobotMap((m) => { const n = { ...m }; delete n[sid]; return n })
      load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan bobot.')
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>Pilih Soal — {String(ujian.judul || '')}</DialogTitle>
          <DialogDescription>Centang soal dari bank untuk dimasukkan ke ujian. Atur bobot per soal.</DialogDescription>
        </DialogHeader>
        <div className="max-h-[55vh] overflow-y-auto">
          <Table>
            <TableHeader>
              <TableRow className="border-b border-border">
                <TableHead className="w-10 text-xs uppercase">✓</TableHead>
                <TableHead className="text-xs uppercase">Soal</TableHead>
                <TableHead className="text-xs uppercase">Tipe</TableHead>
                <TableHead className="text-xs uppercase">Bobot</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {bank.map((s) => {
                const isAttached = attachedBySoal.has(s.id)
                return (
                  <TableRow key={s.id}>
                    <TableCell>
                      <input
                        type="checkbox"
                        className="h-4 w-4 rounded border-border accent-primary cursor-pointer"
                        checked={isAttached}
                        disabled={readOnly}
                        onChange={() => toggle(s)}
                      />
                    </TableCell>
                    <TableCell className="text-sm">
                      <div className="line-clamp-2 max-w-sm">{String(s.pertanyaan || '-')}</div>
                    </TableCell>
                    <TableCell><Badge variant={s.tipe === 'pg' ? 'secondary' : 'outline'}>{s.tipe === 'pg' ? 'PG' : 'Essay'}</Badge></TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Input
                          type="number"
                          value={bobotOf(s.id, Number(s.poin) || 1)}
                          disabled={readOnly || !isAttached}
                          onChange={(e) => setBobotMap((m) => ({ ...m, [s.id]: e.target.value }))}
                          className="h-8 w-20 text-xs"
                        />
                        {!readOnly && isAttached && (
                          <Button size="sm" variant="outline" onClick={() => saveBobot(s)}>Simpan</Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
              {!bank.length && <EmptyState colSpan={4} label="Belum ada soal di bank." />}
            </TableBody>
          </Table>
        </div>
        <DialogFooter>
          <div className="text-xs text-muted-foreground mr-auto">{attached.length} soal terpilih</div>
          <Button variant="outline" onClick={onClose}>Tutup</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}