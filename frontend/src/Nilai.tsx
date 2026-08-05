import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { Download, Pencil, Plus, Save, Trash2 } from 'lucide-react'
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
} from './components/ui/alert-dialog'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './components/ui/dialog'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { PageToolbar } from './components/ui/page'
import { Select } from './components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from './components/ui/tabs'

type ScopeItem = {
  kelasId: string
  kelasLabel: string
  mapelId: string
  mapelNama: string
  tahunAjaranId: string
  tahunNama: string
}

type Tema = {
  id: string
  namaTema: string
  urutan: number
  jumlahCp: number
  bobotKeterampilan: number
  bobotPengetahuan: number
  capaian?: { urutanCp: number; labelDefault: string }[]
}

type CpCell = {
  urutanCp: number
  labelDefault: string
  deskripsiCp: string
  manual: boolean
  nilaiKeterampilan: number | null
  nilaiAkhir: number | null
}
type GridStudent = {
  pesertaDidik: { id: string; nama: string; nis: string }
  cp: CpCell[]
  nilaiUm: number | null
  nkTema: number | null
}
type Grid = {
  tema: Tema
  bobot: { keterampilan: number; pengetahuan: number }
  students: GridStudent[]
}

type Rekap = {
  id?: string
  pesertaDidik: { id: string; nama: string; nis: string }
  npAkhir: number | null
  predikatNP: string
  nkAkhir: number | null
  predikatNK: string
}

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
const SEMESTER = ['Ganjil', 'Genap']

export function Nilai({ token, readOnly }: { token: string; readOnly: boolean }) {
  const [scope, setScope] = useState<ScopeItem[]>([])
  const [kelasId, setKelasId] = useState('')
  const [mapelId, setMapelId] = useState('')
  const [semester, setSemester] = useState('Ganjil')
  const [tab, setTab] = useState('kelola')

  useEffect(() => {
    void request('/nilai/scope', token)
      .then((rows: ScopeItem[]) => setScope(rows || []))
      .catch(() => setScope([]))
  }, [token])

  const kelasOptions = useMemo(() => {
    const seen = new Map<string, ScopeItem>()
    for (const s of scope) {
      if (!seen.has(s.kelasId)) seen.set(s.kelasId, s)
    }
    return Array.from(seen.values())
  }, [scope])

  const mapelOptions = useMemo(() => scope.filter((s) => s.kelasId === kelasId), [scope, kelasId])

  const active = useMemo(() => scope.find((s) => s.kelasId === kelasId && s.mapelId === mapelId), [scope, kelasId, mapelId])
  const tahunAjaranId = active?.tahunAjaranId ?? ''
  const tahunNama = active?.tahunNama ?? ''

  // Reset mapel when kelas changes and the current mapel isn't in the new kelas.
  useEffect(() => {
    if (kelasId && !mapelOptions.some((m) => m.mapelId === mapelId)) {
      setMapelId('')
    }
  }, [kelasId, mapelOptions, mapelId])

  const filterQuery = `kelasId=${kelasId}&mapelId=${mapelId}&semester=${semester}&tahunAjaranId=${tahunAjaranId}`

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Modul Nilai"
        description="Kelola tema & capaian pembelajaran, input nilai per peserta didik, dan lihat rekap nilai akhir dengan predikat."
      />

      <Card>
        <CardHeader className="border-b border-border/60">
          <CardTitle>Filter Penilaian</CardTitle>
          <CardDescription>Pilih rombongan belajar dan mata pelajaran yang Anda ampu.</CardDescription>
        </CardHeader>
        <CardContent className="pt-6">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="grid gap-2">
              <Label className="text-xs font-bold">Kelas / Rombel</Label>
              <Select value={kelasId} onChange={(e) => setKelasId(e.target.value)}>
                <option value="">Pilih kelas</option>
                {kelasOptions.map((k) => (
                  <option key={k.kelasId} value={k.kelasId}>{k.kelasLabel}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label className="text-xs font-bold">Mata Pelajaran</Label>
              <Select value={mapelId} disabled={!kelasId} onChange={(e) => setMapelId(e.target.value)}>
                <option value="">Pilih mapel</option>
                {mapelOptions.map((m) => (
                  <option key={m.mapelId} value={m.mapelId}>{m.mapelNama}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label className="text-xs font-bold">Semester</Label>
              <Select value={semester} onChange={(e) => setSemester(e.target.value)}>
                {SEMESTER.map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label className="text-xs font-bold">Tahun Ajaran</Label>
              <Input value={tahunNama} disabled placeholder="Tahun ajaran kelas" />
            </div>
          </div>
        </CardContent>
      </Card>

      {kelasId && mapelId && tahunAjaranId ? (
        <Tabs value={tab} onValueChange={setTab}>
          <TabsList>
            <TabsTrigger value="kelola">Kelola Tema</TabsTrigger>
            <TabsTrigger value="input">Input Nilai</TabsTrigger>
            <TabsTrigger value="rekap">Rekap</TabsTrigger>
          </TabsList>
          <TabsContent value="kelola">
            <KelolaTema token={token} readOnly={readOnly} filterQuery={filterQuery} kelasId={kelasId} mapelId={mapelId} semester={semester} tahunAjaranId={tahunAjaranId} />
          </TabsContent>
          <TabsContent value="input">
            <InputNilai token={token} readOnly={readOnly} filterQuery={filterQuery} />
          </TabsContent>
          <TabsContent value="rekap">
            <RekapTab token={token} filterQuery={filterQuery} kelasId={kelasId} mapelId={mapelId} semester={semester} tahunAjaranId={tahunAjaranId} mapelNama={active?.mapelNama ?? ''} kelasLabel={active?.kelasLabel ?? ''} />
          </TabsContent>
        </Tabs>
      ) : (
        <Card>
          <CardContent className="py-12 text-center text-sm text-muted-foreground">
            Pilih kelas dan mata pelajaran untuk mulai mengelola nilai.
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Tab 1: Kelola Tema
// ---------------------------------------------------------------------------

function KelolaTema({
  token,
  readOnly,
  filterQuery,
  kelasId,
  mapelId,
  semester,
  tahunAjaranId,
}: {
  token: string
  readOnly: boolean
  filterQuery: string
  kelasId: string
  mapelId: string
  semester: string
  tahunAjaranId: string
}) {
  const [temas, setTemas] = useState<Tema[]>([])
  const [loading, setLoading] = useState(true)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Tema | null>(null)
  const [deleting, setDeleting] = useState<Tema | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const load = () => {
    setLoading(true)
    void request('/tema?' + filterQuery, token)
      .then((rows: Tema[]) => setTemas(rows || []))
      .catch((e: any) => toast.error(`Gagal memuat tema: ${String(e.message || e)}`))
      .finally(() => setLoading(false))
  }

  useEffect(load, [filterQuery, token])

  const bobotTotal = (t: Tema) => t.bobotKeterampilan + t.bobotPengetahuan

  return (
    <Card>
      <CardHeader className="border-b border-border/60 flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div>
          <CardTitle>Kelola Tema & Capaian Pembelajaran</CardTitle>
          <CardDescription>Buat tema, atur jumlah capaian (CP), dan label deskripsi default.</CardDescription>
        </div>
        {!readOnly && (
          <Button
            onClick={() => { setEditing(null); setFormOpen(true) }}
            className="shadow-2xs"
          >
            <Plus className="h-4 w-4 mr-1" /> Tambah Tema
          </Button>
        )}
      </CardHeader>
      <CardContent className="pt-0">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Urutan</TableHead>
              <TableHead>Nama Tema</TableHead>
              <TableHead>Jumlah CP</TableHead>
              <TableHead>Bobot (K/P)</TableHead>
              {!readOnly && <TableHead className="text-right">Aksi</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow><TableCell colSpan={readOnly ? 4 : 5} className="h-24 text-center text-muted-foreground">Memuat...</TableCell></TableRow>
            ) : temas.length === 0 ? (
              <TableRow><TableCell colSpan={readOnly ? 4 : 5} className="h-32 text-center text-sm text-muted-foreground">Belum ada tema. Tambahkan tema pertama untuk mulai menilai.</TableCell></TableRow>
            ) : (
              temas.map((t) => (
                <TableRow key={t.id}>
                  <TableCell className="font-semibold">{t.urutan}</TableCell>
                  <TableCell className="font-medium text-foreground">{t.namaTema}</TableCell>
                  <TableCell>{t.jumlahCp}</TableCell>
                  <TableCell className="text-muted-foreground text-sm">{t.bobotKeterampilan}/{t.bobotPengetahuan}{bobotTotal(t) !== 100 ? ' ⚠' : ''}</TableCell>
                  {!readOnly && (
                    <TableCell>
                      <div className="flex justify-end gap-1.5">
                        <Button size="sm" variant="outline" className="h-8 px-2.5 text-xs" onClick={() => { setEditing(t); setFormOpen(true) }}>
                          <Pencil className="h-3.5 w-3.5 mr-1" /> Ubah
                        </Button>
                        <Button size="sm" variant="destructive" className="h-8 px-2.5 text-xs" onClick={() => setDeleting(t)}>
                          <Trash2 className="h-3.5 w-3.5 mr-1" /> Hapus
                        </Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>

      <TemaFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        editing={editing}
        submitting={submitting}
        readOnly={readOnly}
        kelasId={kelasId}
        mapelId={mapelId}
        semester={semester}
        tahunAjaranId={tahunAjaranId}
        token={token}
        onSaved={() => { setFormOpen(false); load() }}
        onSubmittingChange={setSubmitting}
      />

      <AlertDialog open={deleting !== null} onOpenChange={(o) => !o && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Tema?</AlertDialogTitle>
            <AlertDialogDescription>
              Tema &quot;{deleting?.namaTema}&quot; beserta seluruh nilai CP dan UM di dalamnya akan dihapus. Rekap nilai akhir akan dihitung ulang otomatis.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={submitting}>Batal</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              disabled={submitting}
              onClick={async () => {
                if (!deleting) return
                setSubmitting(true)
                try {
                  await request('/tema/' + deleting.id, token, 'DELETE')
                  toast.success('Tema berhasil dihapus.')
                  setDeleting(null)
                  load()
                } catch (e: any) {
                  toast.error(`Gagal menghapus: ${String(e.message || e)}`)
                } finally {
                  setSubmitting(false)
                }
              }}
            >
              {submitting ? 'Menghapus...' : 'Hapus'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}

function TemaFormDialog({
  open,
  onOpenChange,
  editing,
  submitting,
  kelasId,
  mapelId,
  semester,
  tahunAjaranId,
  token,
  onSaved,
  onSubmittingChange,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  editing: Tema | null
  submitting: boolean
  readOnly: boolean
  kelasId: string
  mapelId: string
  semester: string
  tahunAjaranId: string
  token: string
  onSaved: () => void
  onSubmittingChange: (b: boolean) => void
}) {
  const [namaTema, setNamaTema] = useState('')
  const [urutan, setUrutan] = useState(1)
  const [jumlahCp, setJumlahCp] = useState(1)
  const [labels, setLabels] = useState<string[]>([''])
  const [bobotK, setBobotK] = useState(60)
  const [bobotP, setBobotP] = useState(40)
  const [useOverride, setUseOverride] = useState(false)

  useEffect(() => {
    if (!open) return
    if (editing) {
      setNamaTema(editing.namaTema)
      setUrutan(editing.urutan)
      setJumlahCp(editing.jumlahCp)
      setLabels((editing.capaian || []).map((c) => c.labelDefault))
      setBobotK(editing.bobotKeterampilan)
      setBobotP(editing.bobotPengetahuan)
      setUseOverride(true)
    } else {
      setNamaTema('')
      setUrutan(1)
      setJumlahCp(1)
      setLabels([''])
      setBobotK(60)
      setBobotP(40)
      setUseOverride(false)
    }
  }, [open, editing])

  // Keep the labels array length in sync with jumlahCp (append/remove).
  useEffect(() => {
    setLabels((prev) => {
      const next = [...prev]
      while (next.length < jumlahCp) next.push('')
      while (next.length > jumlahCp) next.pop()
      return next
    })
  }, [jumlahCp])

  const bobotValid = !useOverride || bobotK + bobotP === 100

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (labels.length !== jumlahCp || labels.some((l) => l.trim() === '')) {
      toast.error('Isi deskripsi default untuk setiap CP.')
      return
    }
    if (!bobotValid) {
      toast.error('Total bobot keterampilan + pengetahuan harus 100.')
      return
    }
    onSubmittingChange(true)
    try {
      const body: Record<string, unknown> = {
        kelasId, mapelId, semester, tahunAjaranId,
        namaTema, urutan,
        jumlahCp,
        labelDefaults: labels,
      }
      if (useOverride) {
        body.bobotKeterampilan = bobotK
        body.bobotPengetahuan = bobotP
      }
      if (editing) {
        await request('/tema/' + editing.id, token, 'PUT', body)
        toast.success('Tema berhasil diperbarui.')
      } else {
        await request('/tema', token, 'POST', body)
        toast.success('Tema berhasil ditambahkan.')
      }
      onSaved()
    } catch (err: any) {
      toast.error(`Gagal menyimpan: ${String(err.message || err)}`)
    } finally {
      onSubmittingChange(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{editing ? 'Ubah Tema' : 'Tambah Tema'}</DialogTitle>
          <DialogDescription>
            {editing ? 'Perbarui tema dan deskripsi CP default.' : 'Buat tema baru dengan capaian pembelajaran.'} Bobot diambil dari pengaturan mapel kecuali Anda menggantinya.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4 mt-2">
          <div className="grid gap-2">
            <Label className="text-xs font-bold">Nama Tema</Label>
            <Input value={namaTema} onChange={(e) => setNamaTema(e.target.value)} required placeholder="Contoh: Tema 1 - Lingkunganku" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="grid gap-2">
              <Label className="text-xs font-bold">Urutan</Label>
              <Input type="number" min={1} value={urutan} onChange={(e) => setUrutan(Number(e.target.value))} required />
            </div>
            <div className="grid gap-2">
              <Label className="text-xs font-bold">Jumlah CP</Label>
              <Input type="number" min={1} max={10} value={jumlahCp} onChange={(e) => setJumlahCp(Math.max(1, Number(e.target.value)))} required />
            </div>
          </div>
          <div className="space-y-2">
            <Label className="text-xs font-bold">Deskripsi CP Default ({labels.length})</Label>
            {labels.map((lbl, i) => (
              <Input
                key={i}
                value={lbl}
                placeholder={`Deskripsi CP ${i + 1}`}
                onChange={(e) => setLabels(labels.map((x, j) => (j === i ? e.target.value : x)))}
                required
              />
            ))}
          </div>
          <div className="space-y-2 rounded-lg border border-border p-3">
            <label className="flex items-center gap-2 text-xs font-bold cursor-pointer">
              <input type="checkbox" checked={useOverride} onChange={(e) => setUseOverride(e.target.checked)} />
              Override bobot keterampilan/pengetahuan untuk tema ini
            </label>
            {useOverride && (
              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-1">
                  <Label className="text-[11px]">Bobot Keterampilan (%)</Label>
                  <Input type="number" min={0} max={100} value={bobotK} onChange={(e) => setBobotK(Number(e.target.value))} />
                </div>
                <div className="grid gap-1">
                  <Label className="text-[11px]">Bobot Pengetahuan (%)</Label>
                  <Input type="number" min={0} max={100} value={bobotP} onChange={(e) => setBobotP(Number(e.target.value))} />
                </div>
                {!bobotValid && <p className="col-span-2 text-xs text-destructive font-medium">Total bobot harus 100 (saat ini {bobotK + bobotP}).</p>}
              </div>
            )}
          </div>
          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Batal</Button>
            <Button type="submit" disabled={submitting || !bobotValid}>
              {submitting ? 'Menyimpan...' : 'Simpan'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------------------
// Tab 2: Input Nilai
// ---------------------------------------------------------------------------

type Draft = Record<string, { cp: Record<number, { deskripsiCp: string; nilaiKeterampilan: string }>; nilaiUm: string }>

function InputNilai({ token, readOnly, filterQuery }: { token: string; readOnly: boolean; filterQuery: string }) {
  const [temas, setTemas] = useState<Tema[]>([])
  const [temaId, setTemaId] = useState('')
  const [grid, setGrid] = useState<Grid | null>(null)
  const [draft, setDraft] = useState<Draft>({})
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    void request('/tema?' + filterQuery, token)
      .then((rows: Tema[]) => { setTemas(rows || []); setTemaId('') })
      .catch(() => setTemas([]))
  }, [filterQuery, token])

  useEffect(() => {
    if (!temaId) { setGrid(null); return }
    setLoading(true)
    void request('/tema/' + temaId + '/grid', token)
      .then((g: Grid) => {
        setGrid(g)
        const d: Draft = {}
        for (const st of g.students) {
          d[st.pesertaDidik.id] = {
            nilaiUm: st.nilaiUm == null ? '' : String(st.nilaiUm),
            cp: Object.fromEntries(st.cp.map((c) => [c.urutanCp, {
              deskripsiCp: c.deskripsiCp || c.labelDefault,
              nilaiKeterampilan: c.nilaiKeterampilan == null ? '' : String(c.nilaiKeterampilan),
            }])),
          }
        }
        setDraft(d)
      })
      .catch((e: any) => { toast.error(`Gagal memuat grid: ${String(e.message || e)}`); setGrid(null) })
      .finally(() => setLoading(false))
  }, [temaId, token])

  function setVal(studentId: string, uc: number, field: 'deskripsiCp' | 'nilaiKeterampilan', v: string) {
    setDraft((prev) => {
      const next = { ...prev }
      const row = { ...(next[studentId] || { cp: {}, nilaiUm: '' }) }
      row.cp = { ...row.cp, [uc]: { ...row.cp[uc], [field]: v } }
      next[studentId] = row
      return next
    })
  }
  function setUm(studentId: string, v: string) {
    setDraft((prev) => {
      const next = { ...prev }
      const row = { ...(next[studentId] || { cp: {}, nilaiUm: '' }) }
      row.nilaiUm = v
      next[studentId] = row
      return next
    })
  }

  function applyLabelDefault(uc: number, lbl: string) {
    setDraft((prev) => {
      const next = { ...prev }
      for (const st of grid?.students || []) {
        // Only overwrite rows that are not manually customized in the loaded grid.
        const cell = st.cp.find((c) => c.urutanCp === uc)
        if (cell && !cell.manual) {
          const row = { ...(next[st.pesertaDidik.id] || { cp: {}, nilaiUm: '' }) }
          row.cp = { ...row.cp, [uc]: { ...row.cp[uc], deskripsiCp: lbl } }
          next[st.pesertaDidik.id] = row
        }
      }
      return next
    })
    toast.success(`Deskripsi default CP ${uc} diterapkan ke seluruh siswa (yang belum diubah manual).`)
  }

  async function save() {
    if (!grid) return
    setSaving(true)
    try {
      const values = grid.students.map((st) => {
        const d = draft[st.pesertaDidik.id]
        return {
          pesertaDidikId: st.pesertaDidik.id,
          cp: st.cp.map((c) => ({
            urutanCp: c.urutanCp,
            deskripsiCp: d?.cp[c.urutanCp]?.deskripsiCp ?? c.labelDefault,
            nilaiKeterampilan: parseNum(d?.cp[c.urutanCp]?.nilaiKeterampilan),
          })),
          nilaiUm: parseNum(d?.nilaiUm),
        }
      })
      await request('/tema/' + temaId + '/nilai', token, 'PUT', { values })
      toast.success('Nilai berhasil disimpan. Rekap dihitung ulang otomatis.')
      // Reload grid to reflect computed columns + manual flags.
      const g: Grid = await request('/tema/' + temaId + '/grid', token)
      setGrid(g)
    } catch (e: any) {
      toast.error(`Gagal menyimpan: ${String(e.message || e)}`)
    } finally {
      setSaving(false)
    }
  }

  if (temas.length === 0) {
    return <Card><CardContent className="py-12 text-center text-sm text-muted-foreground">Belum ada tema untuk filter ini. Tambahkan tema di tab Kelola Tema.</CardContent></Card>
  }

  const cps = grid?.students[0]?.cp ?? []
  const bobot = grid?.bobot

  return (
    <Card>
      <CardHeader className="border-b border-border/60 flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div className="grid gap-2">
          <CardTitle>Input Nilai</CardTitle>
          <CardDescription>Pilih tema, isi nilai keterampilan per CP dan nilai UM. NK Tema & Nilai Akhir dihitung otomatis.</CardDescription>
          <Select value={temaId} onChange={(e) => setTemaId(e.target.value)} className="max-w-sm">
            <option value="">Pilih tema</option>
            {temas.map((t) => (
              <option key={t.id} value={t.id}>{t.urutan}. {t.namaTema}</option>
            ))}
          </Select>
        </div>
        {grid && !readOnly && (
          <Button onClick={save} disabled={saving} className="shadow-2xs">
            <Save className="h-4 w-4 mr-1" /> {saving ? 'Menyimpan...' : 'Simpan Nilai'}
          </Button>
        )}
      </CardHeader>
      <CardContent className="pt-6 space-y-4">
        {!temaId && <p className="text-sm text-muted-foreground">Pilih tema untuk menampilkan grid input nilai.</p>}
        {temaId && loading && <p className="text-sm text-muted-foreground">Memuat grid...</p>}
        {grid && (
          <>
            {bobot && (
              <p className="text-xs text-muted-foreground">
                Bobot tema: Keterampilan {bobot.keterampilan}% / Pengetahuan {bobot.pengetahuan}%.
                Nilai Akhir per CP = (NK × {bobot.keterampilan}%) + (UM × {bobot.pengetahuan}%).
              </p>
            )}
            {/* Deskripsi CP default editors */}
            <div className="rounded-lg border border-border p-3 space-y-2 bg-secondary/30">
              <p className="text-xs font-bold">Deskripsi CP Default (terapkan ke siswa yang belum diubah manual)</p>
              {cps.map((c) => (
                <div key={c.urutanCp} className="flex items-center gap-2">
                  <span className="text-xs font-semibold w-10 shrink-0">CP {c.urutanCp}</span>
                  <Input
                    defaultValue={c.labelDefault}
                    onBlur={(e) => { if (e.target.value !== c.labelDefault) applyLabelDefault(c.urutanCp, e.target.value) }}
                    disabled={readOnly}
                  />
                </div>
              ))}
            </div>

            <div className="overflow-x-auto rounded-xl border border-border">
              <Table>
                <TableHeader>
                  <TableRow className="border-b border-border">
                    <TableHead className=" whitespace-nowrap">Nama Siswa</TableHead>
                    {cps.map((c) => (
                      <TableHead key={c.urutanCp} className=" text-center min-w-[160px]">
                        CP {c.urutanCp}
                        <div className="font-normal normal-case text-[10px] text-muted-foreground">Nilai / Deskripsi</div>
                      </TableHead>
                    ))}
                    <TableHead className=" text-center">UM</TableHead>
                    <TableHead className=" text-center">NK Tema</TableHead>
                    {cps.map((c) => (
                      <TableHead key={'na' + c.urutanCp} className=" text-center">NA CP{c.urutanCp}</TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {grid.students.map((st) => (
                    <TableRow key={st.pesertaDidik.id} className="align-top">
                      <TableCell className="font-medium text-foreground whitespace-nowrap">{st.pesertaDidik.nama}</TableCell>
                      {st.cp.map((c) => {
                        const d = draft[st.pesertaDidik.id]?.cp[c.urutanCp]
                        return (
                          <TableCell key={c.urutanCp} className="space-y-1">
                            <Input
                              type="number"
                              min={0}
                              max={100}
                              step="0.01"
                              value={d?.nilaiKeterampilan ?? ''}
                              disabled={readOnly}
                              onChange={(e) => setVal(st.pesertaDidik.id, c.urutanCp, 'nilaiKeterampilan', e.target.value)}
                              className="h-8 text-xs text-center"
                              placeholder="–"
                            />
                            <Input
                              value={d?.deskripsiCp ?? c.labelDefault}
                              disabled={readOnly}
                              onChange={(e) => setVal(st.pesertaDidik.id, c.urutanCp, 'deskripsiCp', e.target.value)}
                              className="h-8 text-[11px]"
                            />
                            {c.manual && <span className="text-[10px] text-warning font-semibold">manual</span>}
                          </TableCell>
                        )
                      })}
                      <TableCell className="text-center">
                        <Input
                          type="number"
                          min={0}
                          max={100}
                          step="0.01"
                          value={draft[st.pesertaDidik.id]?.nilaiUm ?? ''}
                          disabled={readOnly}
                          onChange={(e) => setUm(st.pesertaDidik.id, e.target.value)}
                          className="h-8 text-xs text-center w-20"
                          placeholder="–"
                        />
                      </TableCell>
                      <TableCell className="text-center font-semibold">{fmt(st.nkTema)}</TableCell>
                      {st.cp.map((c) => (
                        <TableCell key={'na' + c.urutanCp} className="text-center font-semibold text-xs">{fmt(c.nilaiAkhir)}</TableCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}

// ---------------------------------------------------------------------------
// Tab 3: Rekap
// ---------------------------------------------------------------------------

function RekapTab({
  token,
  filterQuery,
  kelasId,
  mapelId,
  semester,
  tahunAjaranId,
  mapelNama,
  kelasLabel,
}: {
  token: string
  filterQuery: string
  kelasId: string
  mapelId: string
  semester: string
  tahunAjaranId: string
  mapelNama: string
  kelasLabel: string
}) {
  const [rows, setRows] = useState<Rekap[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setLoading(true)
    void request('/nilai/rekap?' + filterQuery, token)
      .then((r: Rekap[]) => setRows(r || []))
      .catch((e: any) => toast.error(`Gagal memuat rekap: ${String(e.message || e)}`))
      .finally(() => setLoading(false))
  }, [filterQuery, token])

  async function exportFmt(fmt: 'xlsx' | 'pdf') {
    setBusy(true)
    try {
      const base = `/nilai/export?kelasId=${kelasId}&mapelId=${mapelId}&semester=${semester}&tahunAjaranId=${tahunAjaranId}&format=${fmt}`
      const r = await fetch(apiBase + base, { headers: { Authorization: `Bearer ${token}` } })
      if (!r.ok) {
        const x = await r.json().catch(() => ({}))
        throw new Error((x as any)?.error || `Export gagal (${r.status}).`)
      }
      const url = URL.createObjectURL(await r.blob())
      const a = document.createElement('a')
      a.href = url
      a.download = `rekap-nilai-${kelasLabel}-${mapelNama}-${semester}.${fmt === 'xlsx' ? 'xlsx' : 'pdf'}`
      a.click()
      URL.revokeObjectURL(url)
      toast.success(`Export ${fmt.toUpperCase()} berhasil diunduh.`)
    } catch (e: any) {
      toast.error(String(e.message || e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader className="border-b border-border/60 flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div>
          <CardTitle>Rekap Nilai Akhir</CardTitle>
          <CardDescription>Nilai Pengetahuan (NP) dari rata-rata UM, Nilai Keterampilan (NK) dari rata-rata per tema, beserta predikat.</CardDescription>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" disabled={busy || rows.length === 0} onClick={() => exportFmt('xlsx')}>
            <Download className="h-4 w-4 mr-1" /> Export Excel
          </Button>
          <Button variant="outline" disabled={busy || rows.length === 0} onClick={() => exportFmt('pdf')}>
            <Download className="h-4 w-4 mr-1" /> Export PDF
          </Button>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Nama Siswa</TableHead>
              <TableHead className=" text-center">NP</TableHead>
              <TableHead className=" text-center">Predikat NP</TableHead>
              <TableHead className=" text-center">NK</TableHead>
              <TableHead className=" text-center">Predikat NK</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow><TableCell colSpan={5} className="h-24 text-center text-muted-foreground">Memuat...</TableCell></TableRow>
            ) : rows.length === 0 ? (
              <TableRow><TableCell colSpan={5} className="h-32 text-center text-sm text-muted-foreground">Belum ada rekap. Isi nilai pada tema lalu simpan untuk menghitung rekap otomatis.</TableCell></TableRow>
            ) : (
              rows.map((r, i) => (
                <TableRow key={r.pesertaDidik?.id || i}>
                  <TableCell className="font-medium text-foreground">{r.pesertaDidik?.nama ?? '-'}</TableCell>
                  <TableCell className="text-center font-semibold">{fmt(r.npAkhir)}</TableCell>
                  <TableCell className="text-center">{r.predikatNP || '-'}</TableCell>
                  <TableCell className="text-center font-semibold">{fmt(r.nkAkhir)}</TableCell>
                  <TableCell className="text-center">{r.predikatNK || '-'}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

function fmt(v: number | null | undefined): string {
  return v == null ? '-' : Number(v).toFixed(2)
}

function parseNum(s: string | undefined): number | null {
  if (s == null || s.trim() === '') return null
  const n = Number(s)
  return Number.isFinite(n) ? n : null
}

async function request(path: string, token: string, method = 'GET', body?: unknown) {
  const r = await fetch(apiBase + path, {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: body ? JSON.stringify(body) : undefined,
  })
  const result = r.status === 204 ? null : await r.json().catch(() => ({}))
  if (!r.ok) throw new Error((result as any)?.error || 'Permintaan gagal')
  return result
}