import { useEffect, useMemo, useState } from 'react'
import { BookOpen, RotateCcw } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Signature } from '../components/ui/Signature'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
type Row = Record<string, unknown> & { id: string }
type User = { id: string; username: string; role: string }

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

const KONDISI = ['Baik', 'Rusak Ringan', 'Rusak Berat', 'Hilang'] as const

export function PeminjamanBuku({ token, readOnly, user }: { token: string; readOnly: boolean; user: User }) {
  const [kelasList, setKelasList] = useState<Row[]>([])
  const [kelasId, setKelasId] = useState('')
  const [tab, setTab] = useState<'pinjam' | 'kembali'>('pinjam')

  useEffect(() => {
    void request('/kelas', token)
      .then((r: Row[]) => {
        setKelasList(r || [])
        if (r && r.length && !kelasId) setKelasId(r[0].id)
      })
      .catch(() => setKelasList([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  const kelasTerpilih = kelasList.find((k) => k.id === kelasId)
  const kelasStr = kelasTerpilih ? kelasLabel(kelasTerpilih) : ''

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Peminjaman & Pengembalian Buku"
        description="Catat peminjaman dan pengembalian buku modul untuk rombel yang Anda walikan."
        actions={
          <Badge variant="outline" className="text-xs">
            {user.role === 'admin' ? 'Semua kelas (Admin)' : user.role === 'kepala_sekolah' ? 'Akses baca' : 'Rombel wali'}
          </Badge>
        }
      />

      <Card className="rounded-2xl border border-border bg-card shadow-2xs">
        <CardHeader className="border-b border-border/60">
          <CardTitle>Pilih Rombongan Belajar</CardTitle>
          <CardDescription>{readOnly ? 'Anda hanya dapat melihat data.' : 'Pilih rombel untuk mencatat transaksi buku.'}</CardDescription>
        </CardHeader>
        <CardContent className="pt-6">
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="grid gap-2">
              <Label>Kelas / Rombel</Label>
              <Select value={kelasId} onChange={(e) => setKelasId(e.target.value)}>
                <option value="">Pilih kelas</option>
                {kelasList.map((k) => (
                  <option key={k.id} value={k.id}>
                    {kelasLabel(k)}
                  </option>
                ))}
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {kelasId && !readOnly && (
        <div className="flex gap-2">
          <Button variant={tab === 'pinjam' ? 'default' : 'outline'} onClick={() => setTab('pinjam')}>
            <BookOpen className="h-4 w-4 mr-1" /> Peminjaman
          </Button>
          <Button variant={tab === 'kembali' ? 'default' : 'outline'} onClick={() => setTab('kembali')}>
            <RotateCcw className="h-4 w-4 mr-1" /> Pengembalian
          </Button>
        </div>
      )}

      {kelasId && tab === 'pinjam' && (
        <PinjamForm token={token} kelasId={kelasId} kelasStr={kelasStr} userName={user.username} readOnly={readOnly} />
      )}
      {kelasId && tab === 'kembali' && (
        <KembaliForm token={token} kelasId={kelasId} kelasStr={kelasStr} userName={user.username} readOnly={readOnly} />
      )}
      {kelasId && readOnly && <KembaliList token={token} kelasId={kelasId} />}
    </div>
  )
}

function PinjamForm({
  token,
  kelasId,
  kelasStr,
  userName,
  readOnly,
}: {
  token: string
  kelasId: string
  kelasStr: string
  userName: string
  readOnly: boolean
}) {
  const [siswa, setSiswa] = useState<Row[]>([])
  const [bukuKelas, setBukuKelas] = useState<Row[]>([])
  const [tanggal, setTanggal] = useState(() => new Date().toISOString().slice(0, 10))
  const [ttd, setTtd] = useState('')
  const [saving, setSaving] = useState(false)
  // checked[pesertaDidikId][bukuId] = true
  const [checked, setChecked] = useState<Record<string, Record<string, boolean>>>({})

  useEffect(() => {
    if (!kelasId) return
    setSiswa([])
    setBukuKelas([])
    setChecked({})
    void request(`/peserta-didik?kelasId=${kelasId}`, token).then((r: Row[]) => setSiswa(r || [])).catch(() => setSiswa([]))
    void request(`/buku-kelas?kelasId=${kelasId}`, token).then((r: Row[]) => setBukuKelas(r || [])).catch(() => setBukuKelas([]))
  }, [token, kelasId])

  const buku = useMemo(() => bukuKelas.map((bk) => (bk.buku as Row)).filter(Boolean), [bukuKelas])

  function toggle(pdId: string, bukuId: string) {
    setChecked((prev) => {
      const next = { ...prev }
      const row = { ...(next[pdId] || {}) }
      row[bukuId] = !row[bukuId]
      if (!row[bukuId]) delete row[bukuId]
      next[pdId] = row
      return next
    })
  }

  const selectedCount = Object.values(checked).reduce((acc, row) => acc + Object.keys(row).length, 0)

  async function submit() {
    const items: { pesertaDidikId: string; bukuId: string; tanggalPinjam: string }[] = []
    for (const pdId of Object.keys(checked)) {
      for (const bukuId of Object.keys(checked[pdId])) {
        if (checked[pdId][bukuId]) items.push({ pesertaDidikId: pdId, bukuId, tanggalPinjam: tanggal })
      }
    }
    if (items.length === 0) {
      toast.error('Pilih minimal satu siswa dan satu buku.')
      return
    }
    if (!ttd) {
      toast.error('Tanda tangan wajib diisi.')
      return
    }
    setSaving(true)
    try {
      await request('/peminjaman-buku', token, 'POST', { kelasId, items, tandaTangan: ttd })
      toast.success(`${items.length} peminjaman berhasil dicatat untuk ${kelasStr}.`)
      setChecked({})
      setTtd('')
    } catch (err: any) {
      toast.error(err.message || 'Gagal mencatat peminjaman.')
    } finally {
      setSaving(false)
    }
  }

  if (readOnly) return null

  if (!buku.length) {
    return (
      <Card className="rounded-2xl border border-border bg-card shadow-2xs">
        <CardContent className="py-10 text-center text-sm text-muted-foreground">
          Belum ada buku yang ditetapkan untuk {kelasStr} pada semester berjalan. Hubungi admin untuk menetapkan buku
          melalui menu Penetapan Buku.
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className="rounded-2xl border border-border bg-card shadow-2xs">
      <CardHeader className="border-b border-border/60 flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div>
          <CardTitle>Catat Peminjaman — {kelasStr}</CardTitle>
          <CardDescription>
            Centang buku yang dipinjam tiap siswa. Tanggal pinjam &amp; tanda tangan berlaku untuk seluruh baris.
          </CardDescription>
        </div>
        <div className="text-sm text-muted-foreground">{selectedCount} buku dipilih</div>
      </CardHeader>
      <CardContent className="pt-0">
        <div className="grid gap-4 sm:grid-cols-3 border-b border-border/60 py-4">
          <div className="grid gap-2">
            <Label>Tanggal Pinjam</Label>
            <Input type="date" value={tanggal} onChange={(e) => setTanggal(e.target.value)} />
          </div>
        </div>
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="border-b border-border">
                <TableHead className=" sticky left-0 bg-secondary/70">Siswa</TableHead>
                {buku.map((b) => (
                  <TableHead key={b.id} className=" text-center">
                    {String(b.judul)}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {siswa.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-medium sticky left-0 bg-card">{String(s.nama)}</TableCell>
                  {buku.map((b) => (
                    <TableCell key={b.id} className="text-center">
                      <input
                        type="checkbox"
                        className="h-4 w-4 rounded border-border accent-primary cursor-pointer"
                        checked={!!(checked[s.id] || {})[b.id]}
                        onChange={() => toggle(s.id, b.id)}
                      />
                    </TableCell>
                  ))}
                </TableRow>
              ))}
              {!siswa.length && (
                <tr>
                  <td colSpan={buku.length + 1} className="h-24 text-center text-sm text-muted-foreground">
                    Belum ada peserta didik di rombel ini.
                  </td>
                </tr>
              )}
            </TableBody>
          </Table>
        </div>
        <div className="pt-4">
          <Signature value={ttd} onChange={setTtd} userName={userName} />
        </div>
        <div className="pt-4 flex gap-2">
          <Button disabled={saving || selectedCount === 0} onClick={submit}>
            {saving ? 'Menyimpan...' : `Simpan ${selectedCount} Peminjaman`}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function KembaliList({ token, kelasId }: { token: string; kelasId: string }) {
  const [rows, setRows] = useState<Row[]>([])
  useEffect(() => {
    if (!kelasId) return
    void request(`/peminjaman-buku/aktif?kelasId=${kelasId}`, token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }, [token, kelasId])
  return (
    <Card className="rounded-2xl border border-border bg-card shadow-2xs">
      <CardHeader className="border-b border-border/60">
        <CardTitle>Peminjaman Aktif</CardTitle>
        <CardDescription>Daftar buku yang sedang dipinjam di rombel ini.</CardDescription>
      </CardHeader>
      <CardContent className="pt-0">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Siswa</TableHead>
              <TableHead>Buku</TableHead>
              <TableHead>Tgl Pinjam</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => (
              <TableRow key={r.id}>
                <TableCell className="font-medium">{String((r.pesertaDidik as Row)?.nama || '-')}</TableCell>
                <TableCell>{String((r.buku as Row)?.judul || '-')}</TableCell>
                <TableCell className="text-sm">{String((r.tanggalPinjam || '').toString().slice(0, 10))}</TableCell>
              </TableRow>
            ))}
            {!rows.length && (
              <tr>
                <td colSpan={3} className="h-24 text-center text-sm text-muted-foreground">
                  Tidak ada peminjaman aktif.
                </td>
              </tr>
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

function KembaliForm({
  token,
  kelasId,
  kelasStr,
  userName,
  readOnly,
}: {
  token: string
  kelasId: string
  kelasStr: string
  userName: string
  readOnly: boolean
}) {
  const [rows, setRows] = useState<Row[]>([])
  const [tanggal, setTanggal] = useState(() => new Date().toISOString().slice(0, 10))
  const [ttd, setTtd] = useState('')
  const [saving, setSaving] = useState(false)
  // state[peminjamanId] = { checked, kondisi, catatan }
  const [state, setState] = useState<Record<string, { checked: boolean; kondisi: string; catatan: string }>>({})

  const load = () => {
    void request(`/peminjaman-buku/aktif?kelasId=${kelasId}`, token).then((r: Row[]) => {
      setRows(r || [])
      setState({})
    }).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
  }, [token, kelasId]) // eslint-disable-line react-hooks/exhaustive-deps

  function update(id: string, patch: Partial<{ checked: boolean; kondisi: string; catatan: string }>) {
    setState((prev) => {
      const cur = prev[id] || { checked: false, kondisi: 'Baik', catatan: '' }
      const next = { ...cur, ...patch }
      if (!next.checked) next.kondisi = 'Baik'
      return { ...prev, [id]: next }
    })
  }

  const selectedItems = rows.filter((r) => (state[r.id] || { checked: false }).checked)

  const blockedNote = useMemo(() => {
    for (const r of selectedItems) {
      const st = state[r.id]
      if (!st) continue
      if ((st.kondisi === 'Rusak Berat' || st.kondisi === 'Hilang') && !st.catatan.trim()) {
        return `Catatan wajib diisi untuk kondisi "${st.kondisi}".`
      }
    }
    return ''
  }, [selectedItems, state])

  async function submit() {
    const items = selectedItems.map((r) => {
      const st = state[r.id]
      return {
        peminjamanId: r.id,
        tanggalKembali: tanggal,
        kondisiBuku: st?.kondisi || 'Baik',
        catatan: st?.catatan || '',
      }
    })
    if (!items.length) {
      toast.error('Centang minimal satu buku untuk dikembalikan.')
      return
    }
    if (!ttd) {
      toast.error('Tanda tangan wajib diisi.')
      return
    }
    if (blockedNote) {
      toast.error(blockedNote)
      return
    }
    setSaving(true)
    try {
      await request('/peminjaman-buku/kembali', token, 'POST', { items, tandaTangan: ttd })
      toast.success(`${items.length} pengembalian berhasil dicatat untuk ${kelasStr}.`)
      setTtd('')
      load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal mencatat pengembalian.')
    } finally {
      setSaving(false)
    }
  }

  if (readOnly) return null

  return (
    <Card className="rounded-2xl border border-border bg-card shadow-2xs">
      <CardHeader className="border-b border-border/60 flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div>
          <CardTitle>Catat Pengembalian — {kelasStr}</CardTitle>
          <CardDescription>Centang buku yang dikembalikan, isi kondisi &amp; catatan bila perlu.</CardDescription>
        </div>
        <div className="text-sm text-muted-foreground">{selectedItems.length} buku dipilih</div>
      </CardHeader>
      <CardContent className="pt-0">
        <div className="grid gap-4 sm:grid-cols-3 border-b border-border/60 py-4">
          <div className="grid gap-2">
            <Label>Tanggal Kembali</Label>
            <Input type="date" value={tanggal} onChange={(e) => setTanggal(e.target.value)} />
          </div>
        </div>
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="border-b border-border">
                <TableHead>Kembali?</TableHead>
                <TableHead>Siswa</TableHead>
                <TableHead>Buku</TableHead>
                <TableHead>Tgl Pinjam</TableHead>
                <TableHead>Kondisi</TableHead>
                <TableHead>Catatan</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => {
                const st = state[r.id] || { checked: false, kondisi: 'Baik', catatan: '' }
                const needsCatatan = st.kondisi === 'Rusak Berat' || st.kondisi === 'Hilang'
                return (
                  <TableRow key={r.id}>
                    <TableCell className="text-center">
                      <input
                        type="checkbox"
                        className="h-4 w-4 rounded border-border accent-primary cursor-pointer"
                        checked={st.checked}
                        onChange={(e) => update(r.id, { checked: e.target.checked })}
                      />
                    </TableCell>
                    <TableCell className="font-medium">{String((r.pesertaDidik as Row)?.nama || '-')}</TableCell>
                    <TableCell>{String((r.buku as Row)?.judul || '-')}</TableCell>
                    <TableCell className="text-sm">{String((r.tanggalPinjam || '').toString().slice(0, 10))}</TableCell>
                    <TableCell>
                      <Select
                        value={st.kondisi}
                        disabled={!st.checked}
                        onChange={(e) => update(r.id, { kondisi: e.target.value })}
                        className="h-9 text-xs"
                      >
                        {KONDISI.map((k) => (
                          <option key={k}>{k}</option>
                        ))}
                      </Select>
                    </TableCell>
                    <TableCell>
                      <Input
                        value={st.catatan}
                        disabled={!st.checked || !needsCatatan}
                        onChange={(e) => update(r.id, { catatan: e.target.value })}
                        placeholder={needsCatatan ? 'Wajib diisi' : 'Opsional'}
                        className="h-9 text-xs"
                      />
                    </TableCell>
                  </TableRow>
                )
              })}
              {!rows.length && (
                <tr>
                  <td colSpan={6} className="h-24 text-center text-sm text-muted-foreground">
                    Tidak ada peminjaman aktif untuk rombel ini.
                  </td>
                </tr>
              )}
            </TableBody>
          </Table>
        </div>
        <div className="pt-4">
          <Signature value={ttd} onChange={setTtd} userName={userName} label="Tanda Tangan Penerima Pengembalian" />
        </div>
        <div className="pt-4 flex items-center gap-3">
          <Button disabled={saving || selectedItems.length === 0 || !!blockedNote} onClick={submit}>
            {saving ? 'Menyimpan...' : `Simpan ${selectedItems.length} Pengembalian`}
          </Button>
          {blockedNote && <span className="text-xs text-destructive">{blockedNote}</span>}
        </div>
      </CardContent>
    </Card>
  )
}