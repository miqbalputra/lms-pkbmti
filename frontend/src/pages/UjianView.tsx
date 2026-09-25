import { useEffect, useState, type FormEvent } from 'react'
import { ArrowDown, ArrowUp, BookOpen, Dices, Download, GripVertical, KeyRound, Pencil, Plus, Printer, Trash2 } from 'lucide-react'
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
import { downloadFile, request } from '../lib/api'
import { formatWibDateTime, wibDateTimeLocalToISO, wibDateTimeLocalValue } from '../lib/wib'
import { useNavigate } from 'react-router-dom'
import { pathFor } from '../lib/router'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

function visualChoices(question: Row | undefined): Row[] {
  if (!question) return []
  let config = question.konfigurasi
  if (typeof config === 'string') {
    try { config = JSON.parse(config) } catch { return [] }
  }
  const choices = (config as Record<string, unknown> | undefined)?.choices
  return Array.isArray(choices) ? choices.filter((choice): choice is Row => Boolean(choice && typeof choice === 'object' && 'id' in choice)) : []
}

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
  waktuMulai: '',
  waktuSelesai: '',
  durasiMenit: '60',
  gracePeriodMenit: '5',
  batasTabSwitch: '0',
  acakSoal: false,
  izinkanEditRespons: false,
  aksesKode: '',
}

export function UjianView({
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
  const [soalUjian, setSoalUjian] = useState<Row | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const navigate = useNavigate()

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
      waktuMulai: wibDateTimeLocalValue(r.waktuMulai),
      waktuSelesai: wibDateTimeLocalValue(r.waktuSelesai),
      durasiMenit: String(r.durasiMenit ?? '60'),
      gracePeriodMenit: String(r.gracePeriodMenit ?? '5'),
      batasTabSwitch: String(r.batasTabSwitch ?? '0'),
      acakSoal: !!r.acakSoal,
      izinkanEditRespons: !!r.izinkanEditRespons,
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
      waktuMulai: form.waktuMulai ? wibDateTimeLocalToISO(form.waktuMulai) : undefined,
      waktuSelesai: form.waktuSelesai ? wibDateTimeLocalToISO(form.waktuSelesai) : undefined,
      durasiMenit: Number(form.durasiMenit) || 0,
      gracePeriodMenit: Number(form.gracePeriodMenit) || 0,
      batasTabSwitch: Number(form.batasTabSwitch) || 0,
      acakSoal: form.acakSoal,
      izinkanEditRespons: form.izinkanEditRespons,
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
        actions={<div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => navigate(pathFor('bank-soal-ujian'))}><BookOpen className="h-4 w-4" /> Bank soal Ujian Online</Button>
          {!readOnly && <Button onClick={openAdd}><Plus className="h-4 w-4" />Buat ujian</Button>}
        </div>}
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
              <Label>Grace Period (menit)</Label>
              <Input type="number" min="0" value={form.gracePeriodMenit} onChange={(e) => setForm({ ...form, gracePeriodMenit: e.target.value })} />
              <p className="text-xs text-muted-foreground">Toleransi setelah durasi habis. Siswa masih bisa mengerjakan & mengirim jawaban selama grace period (default 5 mnt).</p>
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
            <div className="grid gap-1 sm:col-span-2">
              <div className="flex items-center gap-2">
                <Checkbox id="edit-respons" checked={form.izinkanEditRespons} onChange={(e) => setForm({ ...form, izinkanEditRespons: e.target.checked })} />
                <Label htmlFor="edit-respons" className="cursor-pointer">Izinkan siswa memperbaiki jawaban setelah dikirim (Ujian Online)</Label>
              </div>
              <p className="pl-6 text-xs text-muted-foreground">Setiap perubahan disimpan sebagai riwayat. Opsi ini tidak membuka kembali ujian yang dikunci dan berakhir otomatis.</p>
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
                    {!!r.acakSoal && <Badge variant="outline" className="mt-1">Acak</Badge>}
                  </TableCell>
                  <TableCell>{String(m.namaMapel || '-')}</TableCell>
                  <TableCell>{kelasLabel(k)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    <div>{fmtDateTime(r.waktuMulai) || '—'}</div>
                    <div className="text-xs">s/d {fmtDateTime(r.waktuSelesai) || '—'}{r.durasiMenit ? ` (${r.durasiMenit} mnt)` : ''}</div>
                    {Number(r.gracePeriodMenit) > 0 && <div className="text-xs text-blue-600">Grace: {String(r.gracePeriodMenit)} mnt</div>}
                    {Number(r.batasTabSwitch) > 0 && <div className="text-xs text-orange-600">Max {String(r.batasTabSwitch)}x pindah tab</div>}
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
                          onClick={() => window.open('/ujian', '_blank')}
                          title="Buka halaman ujian siswa"
                        >
                          🎓 Siswa
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => void downloadFile(`/ujian/${r.id}/export`, token, 'hasil-ujian.csv').catch((error: any) => toast.error(error.message || 'Hasil ujian tidak dapat diunduh.'))}
                        title="Unduh seluruh hasil jawaban dalam CSV"
                      >
                        <Download className="h-3.5 w-3.5" /> CSV
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => void downloadFile(`/ujian/${r.id}/export?format=xlsx`, token, 'hasil-ujian.xlsx').catch((error: any) => toast.error(error.message || 'Hasil ujian tidak dapat diunduh.'))}
                        title="Unduh seluruh hasil jawaban dalam Excel"
                      >
                        <Download className="h-3.5 w-3.5" /> XLSX
                      </Button>
                      {Boolean(r.aksesKode) && (
                        <Button
                          size="sm"
                          variant="default"
                          onClick={() => navigate(pathFor('ujian-monitor'))}
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
  const [sections, setSections] = useState<Row[]>([])
  const [newSectionName, setNewSectionName] = useState('')
  const [newSectionDescription, setNewSectionDescription] = useState('')
  const [targetSectionID, setTargetSectionID] = useState('')
  const [bobotMap, setBobotMap] = useState<Record<string, string>>({})
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null)
  const [savingBranchID, setSavingBranchID] = useState('')

  const load = () => {
    void request('/bank-soal', token).then((r: Row[]) => setBank(r || [])).catch(() => setBank([]))
    void request('/ujian/' + ujian.id + '/soal', token).then((r: Row[]) => setAttached(r || [])).catch(() => setAttached([]))
    void request('/ujian/' + ujian.id + '/bagian', token).then((r: Row[]) => {
      const list = r || []
      setSections(list)
      setTargetSectionID((current) => list.some((section) => String(section.id) === current) ? current : String(list[0]?.id || ''))
    }).catch(() => setSections([]))
  }

  useEffect(() => {
    load()
  }, [ujian.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const attachedBySoal = new Map(attached.map((a) => [String(a.soalId), a]))
  const sectionOrder = new Map(sections.map((section, index) => [String(section.id), index]))
  const orderedAttached = [...attached].sort((left, right) => {
    const leftSection = String(left.bagianId || '')
    const rightSection = String(right.bagianId || '')
    const leftRank = sectionOrder.get(leftSection) ?? sections.length
    const rightRank = sectionOrder.get(rightSection) ?? sections.length
    if (leftRank !== rightRank) return leftRank - rightRank
    return Number(left.urutan || 0) - Number(right.urutan || 0)
  })

  async function persistSections(next: Row[]) {
    const previous = sections
    setSections(next)
    try {
      const saved = await request('/ujian/' + ujian.id + '/bagian', token, 'PUT', {
        bagian: next.map((item, index) => ({ id: String(item.id || ''), nama: String(item.nama || ''), deskripsi: String(item.deskripsi || ''), urutan: index + 1 })),
      }) as Row[]
      setSections(saved || [])
      setTargetSectionID((current) => current || String(saved?.[0]?.id || ''))
      const validIDs = new Set((saved || []).map((section) => String(section.id)))
      setAttached((current) => current.map((item) => validIDs.has(String(item.bagianId || '')) ? item : { ...item, bagianId: '' }))
    } catch (err: any) {
      setSections(previous)
      toast.error(err.message || 'Bagian belum dapat disimpan.')
      throw err
    }
  }

  async function addSection() {
    const name = newSectionName.trim()
    if (!name) {
      toast.error('Isi nama bagian terlebih dahulu.')
      return
    }
    const id = globalThis.crypto?.randomUUID?.() || `section-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const next = [...sections, { id, nama: name, deskripsi: newSectionDescription.trim(), urutan: sections.length + 1 }]
    await persistSections(next)
    setNewSectionName('')
    setNewSectionDescription('')
    setTargetSectionID(id)
    toast.success('Bagian ditambahkan.')
  }

  async function changeQuestionSection(item: Row, sectionID: string) {
    try {
      await request('/ujian/' + ujian.id + '/soal', token, 'POST', {
        soalId: String(item.soalId), bobot: Number(item.bobot) || 0, bagianId: sectionID,
      })
      setAttached((current) => current.map((row) => row.id === item.id ? { ...row, bagianId: sectionID } : row))
    } catch (err: any) {
      toast.error(err.message || 'Bagian soal belum dapat disimpan.')
      load()
    }
  }

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
          bagianId: targetSectionID,
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

  async function saveBranchRoute(item: Row, choiceID: string, target: string) {
    const previous = (item.branchToByAnswer && typeof item.branchToByAnswer === 'object' ? item.branchToByAnswer : {}) as Record<string, string>
    const next = { ...previous }
    if (target) next[choiceID] = target
    else delete next[choiceID]
    setSavingBranchID(item.id)
    try {
      await request(`/ujian/${ujian.id}/soal/${item.id}/branch`, token, 'PUT', { branchToByAnswer: next })
      setAttached((current) => current.map((row) => row.id === item.id ? { ...row, branchToByAnswer: next } : row))
      toast.success('Alur berdasarkan jawaban tersimpan.')
    } catch (err: any) {
      toast.error(err.message || 'Alur jawaban belum dapat disimpan.')
      load()
    } finally {
      setSavingBranchID('')
    }
  }

  async function saveOrder(next: Row[]) {
    const previous = attached
    setAttached(next)
    try {
      await request(`/ujian/${ujian.id}/soal/urutan`, token, 'PUT', { urutanIds: next.map((row) => row.id) })
      setAttached(next.map((row, index) => ({ ...row, urutan: index + 1 })))
    } catch (err: any) {
      setAttached(previous)
      toast.error(err.message || 'Urutan soal belum dapat disimpan.')
      load()
    }
  }

  function moveAttached(from: number, to: number) {
    if (to < 0 || to >= orderedAttached.length || from === to) return
    if (String(orderedAttached[from]?.bagianId || '') !== String(orderedAttached[to]?.bagianId || '')) return
    const next = [...orderedAttached]
    const [moved] = next.splice(from, 1)
    next.splice(to, 0, moved)
    void saveOrder(next)
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>Pilih Soal — {String(ujian.judul || '')}</DialogTitle>
          <DialogDescription>Centang soal dari bank untuk dimasukkan ke ujian. Atur bobot per soal.</DialogDescription>
        </DialogHeader>
        <section aria-label="Bagian ujian" className="space-y-3 rounded-xl border bg-muted/20 p-3">
          <div><h3 className="text-sm font-semibold">Bagian ujian</h3><p className="text-xs text-muted-foreground">Kelompokkan pertanyaan agar siswa lebih mudah memahami dan mengerjakan ujian. Soal acak tetap berada di bagiannya.</p></div>
          <div className="grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
            <Input aria-label="Nama bagian baru" placeholder="Contoh: Literasi Membaca" value={newSectionName} onChange={(event) => setNewSectionName(event.target.value)} disabled={readOnly} />
            <Input aria-label="Petunjuk bagian baru" placeholder="Petunjuk singkat (opsional)" value={newSectionDescription} onChange={(event) => setNewSectionDescription(event.target.value)} disabled={readOnly} />
            <Button type="button" variant="outline" onClick={() => void addSection()} disabled={readOnly || !newSectionName.trim()}><Plus className="mr-1 h-4 w-4" />Tambah bagian</Button>
          </div>
          {sections.map((section, index) => <div key={section.id} className="grid items-center gap-2 rounded-lg border bg-background p-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
            <Input aria-label={`Nama bagian ${index + 1}`} value={String(section.nama || '')} disabled={readOnly} onChange={(event) => setSections((current) => current.map((row) => row.id === section.id ? { ...row, nama: event.target.value } : row))} onBlur={() => { if (!readOnly) void persistSections(sections) }} />
            <Input aria-label={`Petunjuk bagian ${index + 1}`} value={String(section.deskripsi || '')} placeholder="Petunjuk siswa (opsional)" disabled={readOnly} onChange={(event) => setSections((current) => current.map((row) => row.id === section.id ? { ...row, deskripsi: event.target.value } : row))} onBlur={() => { if (!readOnly) void persistSections(sections) }} />
            <div className="flex gap-1">
              <Button type="button" size="icon" variant="outline" className="h-11 w-11" disabled={readOnly || index === 0} onClick={() => { const next = [...sections]; [next[index - 1], next[index]] = [next[index], next[index - 1]]; void persistSections(next) }} aria-label={`Naikkan bagian ${index + 1}`}><ArrowUp className="h-4 w-4" /></Button>
              <Button type="button" size="icon" variant="outline" className="h-11 w-11" disabled={readOnly || index === sections.length - 1} onClick={() => { const next = [...sections]; [next[index + 1], next[index]] = [next[index], next[index + 1]]; void persistSections(next) }} aria-label={`Turunkan bagian ${index + 1}`}><ArrowDown className="h-4 w-4" /></Button>
              <Button type="button" size="icon" variant="outline" className="h-11 w-11" disabled={readOnly} onClick={() => void persistSections(sections.filter((row) => row.id !== section.id))} aria-label={`Hapus bagian ${index + 1}`}><Trash2 className="h-4 w-4" /></Button>
            </div>
          </div>)}
          {!!sections.length && <div className="max-w-xs"><Label htmlFor="exam-default-section">Bagian untuk soal baru</Label><Select id="exam-default-section" value={targetSectionID} onChange={(event) => setTargetSectionID(event.target.value)} disabled={readOnly}><option value="">Tanpa bagian</option>{sections.map((section) => <option key={section.id} value={section.id}>{String(section.nama)}</option>)}</Select></div>}
        </section>
        <section aria-label="Urutan soal ujian" className="space-y-2 rounded-xl border bg-muted/20 p-3">
          <div><h3 className="text-sm font-semibold">Urutan soal ({attached.length})</h3><p className="text-xs text-muted-foreground">Seret kartu atau gunakan tombol panah untuk menyusun urutan. Pengaturan acak akan mengikuti susunan dasar ini.</p></div>
          {!attached.length && <p className="text-xs text-muted-foreground">Pilih soal dari bank di bawah untuk mulai menyusun ujian.</p>}
          {orderedAttached.map((item, index) => <div key={item.id} className="space-y-2"><div draggable={!readOnly} onDragStart={(event) => { if (readOnly) return; setDraggedIndex(index); event.dataTransfer.setData('application/x-pkbm-ujian-soal-index', String(index)); event.dataTransfer.effectAllowed = 'move' }} onDragOver={(event) => { if (!readOnly) event.preventDefault() }} onDrop={(event) => { if (readOnly || !event.dataTransfer.types.includes('application/x-pkbm-ujian-soal-index')) return; event.preventDefault(); const source = Number(event.dataTransfer.getData('application/x-pkbm-ujian-soal-index')); if (Number.isInteger(source)) moveAttached(source, index); setDraggedIndex(null) }} className={`flex min-h-11 items-center gap-2 rounded-lg border bg-background p-2 ${readOnly ? '' : 'cursor-grab'} ${draggedIndex === index ? 'opacity-50' : ''}`}>
            <span className="text-muted-foreground" aria-hidden="true"><GripVertical className="h-4 w-4" /></span><span className="w-6 text-center text-xs font-bold">{index + 1}</span><span className="min-w-0 flex-1 truncate text-sm">{String((item.soal as Record<string, unknown> | undefined)?.pertanyaan || 'Soal')}</span><Badge variant="outline" className="max-w-32 truncate">{String(sections.find((section) => String(section.id) === String(item.bagianId || ''))?.nama || 'Tanpa bagian')}</Badge><div className="flex gap-1"><Button type="button" size="icon" variant="outline" className="h-10 w-10" disabled={readOnly || index === 0 || String(orderedAttached[index - 1]?.bagianId || '') !== String(item.bagianId || '')} onClick={() => moveAttached(index, index - 1)} aria-label={`Naikkan soal ${index + 1}`}><ArrowUp className="h-4 w-4" /></Button><Button type="button" size="icon" variant="outline" className="h-10 w-10" disabled={readOnly || index === orderedAttached.length - 1 || String(orderedAttached[index + 1]?.bagianId || '') !== String(item.bagianId || '')} onClick={() => moveAttached(index, index + 1)} aria-label={`Turunkan soal ${index + 1}`}><ArrowDown className="h-4 w-4" /></Button></div>
            {sections.length > 0 && <Select aria-label={`Bagian soal ${index + 1}`} className="max-w-48" value={String(item.bagianId || '')} disabled={readOnly} onChange={(event) => void changeQuestionSection(item, event.target.value)}><option value="">Tanpa bagian</option>{sections.map((section) => <option key={section.id} value={section.id}>{String(section.nama)}</option>)}</Select>}
          </div><UjianBranchEditor item={item} sections={sections} attached={attached} readOnly={readOnly} saving={savingBranchID === item.id} onSave={(choiceID, target) => void saveBranchRoute(item, choiceID, target)} /></div>)}
        </section>
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

function UjianBranchEditor({ item, sections, attached, readOnly, saving, onSave }: {
  item: Row
  sections: Row[]
  attached: Row[]
  readOnly: boolean
  saving: boolean
  onSave: (choiceID: string, target: string) => void
}) {
  const question = item.soal as Row | undefined
  const type = String(question?.tipe || '')
  const choices = visualChoices(question)
  const sectionID = String(item.bagianId || '')
  const sourcePosition = sections.findIndex((section) => String(section.id) === sectionID)
  if (!['pg_tunggal', 'dropdown'].includes(type) || !choices.length || sourcePosition < 0) return null
  const routes = (item.branchToByAnswer && typeof item.branchToByAnswer === 'object' ? item.branchToByAnswer : {}) as Record<string, string>
  const targetSections = sections.filter((section, index) => index > sourcePosition && attached.some((row) => String(row.bagianId || '') === String(section.id)))
  return <details className="rounded-lg border bg-background px-3 py-2">
    <summary className="min-h-10 cursor-pointer py-2 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary">Alur berdasarkan jawaban <span className="font-normal text-muted-foreground">(opsional)</span></summary>
    <p className="pb-2 text-xs text-muted-foreground">Pilih bagian lanjutan untuk setiap jawaban. Jawaban tanpa aturan mengikuti urutan normal. Rute hanya dapat menuju bagian setelahnya agar siswa tidak terjebak dalam putaran.</p>
    <div className="space-y-2 pb-2">{choices.map((choice, index) => {
      const choiceID = String(choice.id)
      const label = String(choice.text || `Pilihan ${index + 1}`)
      return <label key={choiceID} className="grid gap-1 text-xs sm:grid-cols-[minmax(0,1fr)_minmax(220px,0.8fr)] sm:items-center">
        <span className="line-clamp-2">{label}</span>
        <Select aria-label={`Arah untuk jawaban ${label}`} value={routes[choiceID] || ''} disabled={readOnly || saving} onChange={(event) => onSave(choiceID, event.target.value)}>
          <option value="">Lanjut urutan berikutnya</option>
          {targetSections.map((section) => <option key={section.id} value={String(section.id)}>Lanjut ke: {String(section.nama || 'Bagian')}</option>)}
          <option value="__selesai__">Akhiri ujian</option>
        </Select>
      </label>
    })}</div>
    {saving && <p className="pb-2 text-xs text-primary" role="status">Menyimpan alur…</p>}
    {!readOnly && !targetSections.length && <p className="pb-2 text-xs text-amber-700">Tambahkan soal ke bagian berikutnya untuk membuat tujuan lompatan.</p>}
  </details>
}
