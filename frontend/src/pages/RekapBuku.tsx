import { useEffect, useState } from 'react'
import { Download } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Label } from '../components/ui/label'
import { PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { formatWibDate } from '../lib/wib'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
type Row = Record<string, unknown> & { id: string }
type RekapRow = { peminjaman: Row; pengembalian: Row | null }

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

const STATUS: { value: string; label: string; cls: string }[] = [
  { value: 'Dipinjam', label: 'Dipinjam', cls: 'bg-amber-500/15 text-amber-600 border-amber-500/30' },
  { value: 'Dikembalikan', label: 'Dikembalikan', cls: 'bg-success/15 text-success border-success/30' },
]

export function RekapBuku({ token }: { token: string }) {
  const [kelas, setKelas] = useState<Row[]>([])
  const [years, setYears] = useState<Row[]>([])
  const [kelasId, setKelasId] = useState('')
  const [semester, setSemester] = useState('')
  const [tahunAjaranId, setTahunAjaranId] = useState('')
  const [status, setStatus] = useState('')
  const [rows, setRows] = useState<RekapRow[]>([])
  const [busy, setBusy] = useState(false)
  const [applied, setApplied] = useState(false)

  useEffect(() => {
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
    void request('/tahun-ajaran', token).then((r: Row[]) => setYears(r || [])).catch(() => setYears([]))
  }, [token])

  function queryStr() {
    const p = new URLSearchParams()
    if (kelasId) p.set('kelasId', kelasId)
    if (semester) p.set('semester', semester)
    if (tahunAjaranId) p.set('tahunAjaranId', tahunAjaranId)
    if (status) p.set('status', status)
    return p.toString()
  }

  function load() {
    const qs = queryStr()
    setApplied(true)
    void request('/buku/rekap?' + qs, token)
      .then((r: RekapRow[]) => setRows(r || []))
      .catch((e: any) => {
        setRows([])
        toast.error(`Gagal memuat rekap: ${String(e.message || e)}`)
      })
  }

  async function exportFmt(fmt: 'xlsx' | 'pdf') {
    setBusy(true)
    try {
      const r = await fetch(apiBase + '/buku/export?' + queryStr() + `&format=${fmt}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!r.ok) {
        const x = await r.json().catch(() => ({}))
        throw new Error((x as any)?.error || `Export gagal (${r.status}).`)
      }
      const url = URL.createObjectURL(await r.blob())
      const a = document.createElement('a')
      a.href = url
      a.download = `rekap-peminjaman-buku.${fmt === 'xlsx' ? 'xlsx' : 'pdf'}`
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
    <div className="space-y-4">
      <PageToolbar title="Rekap Peminjaman Buku" description="Tinjau seluruh transaksi peminjaman & pengembalian buku modul." />

      <Card className="rounded-2xl border border-border bg-card shadow-2xs">
        <CardHeader className="border-b border-border/60">
          <CardTitle>Filter Rekap</CardTitle>
          <CardDescription>Gabungkan filter sesuai kebutuhan. Pilih minimal satu lalu klik Tampilkan.</CardDescription>
        </CardHeader>
        <CardContent className="pt-6">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
            <div className="grid gap-2">
              <Label>Kelas</Label>
              <Select value={kelasId} onChange={(e) => setKelasId(e.target.value)}>
                <option value="">Semua kelas</option>
                {kelas.map((k) => (
                  <option key={k.id} value={k.id}>
                    {kelasLabel(k)}
                  </option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Semester</Label>
              <Select value={semester} onChange={(e) => setSemester(e.target.value)}>
                <option value="">Semua semester</option>
                <option>Ganjil</option>
                <option>Genap</option>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Tahun Ajaran</Label>
              <Select value={tahunAjaranId} onChange={(e) => setTahunAjaranId(e.target.value)}>
                <option value="">Semua tahun ajaran</option>
                {years.map((y) => (
                  <option key={y.id} value={y.id}>
                    {String(y.namaTahunAjaran)}
                  </option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Status</Label>
              <Select value={status} onChange={(e) => setStatus(e.target.value)}>
                <option value="">Semua status</option>
                {STATUS.map((s) => (
                  <option key={s.value} value={s.value}>
                    {s.label}
                  </option>
                ))}
              </Select>
            </div>
            <div className="flex items-end">
              <Button onClick={load}>Tampilkan</Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 sm:grid-cols-3">
        <Metric label="Total transaksi" value={rows.length} />
        <Metric label="Dipinjam" value={rows.filter((r) => String(r.peminjaman?.status) === 'Dipinjam').length} />
        <Metric label="Dikembalikan" value={rows.filter((r) => String(r.peminjaman?.status) === 'Dikembalikan').length} />
      </div>

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <CardHeader className="border-b border-border/60 flex flex-col md:flex-row md:items-center justify-between gap-3">
          <div>
            <CardTitle>Daftar Peminjaman</CardTitle>
            <CardDescription>Rincian peminjaman beserta pengembalian (kondisi & catatan).</CardDescription>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" disabled={busy || rows.length === 0 || !applied} onClick={() => exportFmt('xlsx')}>
              <Download className="h-4 w-4 mr-1" /> Export Excel
            </Button>
            <Button variant="outline" disabled={busy || rows.length === 0 || !applied} onClick={() => exportFmt('pdf')}>
              <Download className="h-4 w-4 mr-1" /> Export PDF
            </Button>
          </div>
        </CardHeader>
        <CardContent className="pt-0">
          <Table>
            <TableHeader>
              <TableRow className="border-b border-border">
                <TableHead>Siswa</TableHead>
                <TableHead>Kelas</TableHead>
                <TableHead>Buku</TableHead>
                <TableHead>Tgl Pinjam</TableHead>
                <TableHead>Semester</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Tgl Kembali</TableHead>
                <TableHead>Kondisi</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => {
                const p = r.peminjaman || ({} as Row)
                const k = (p.kelas as Row) || {}
                const kembali = r.pengembalian
                const st = STATUS.find((s) => s.value === String(p.status))
                return (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium">{String((p.pesertaDidik as Row)?.nama || '-')}</TableCell>
                    <TableCell>{kelasLabel(k)}</TableCell>
                    <TableCell>{String((p.buku as Row)?.judul || '-')}</TableCell>
                    <TableCell className="text-sm">{formatWibDate(p.tanggalPinjam)}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{String(p.semester || '-')}</Badge>
                    </TableCell>
                    <TableCell>
                      {st ? (
                        <Badge variant="outline" className={st.cls}>
                          {st.label}
                        </Badge>
                      ) : (
                        String(p.status || '-')
                      )}
                    </TableCell>
                    <TableCell className="text-sm">
                      {kembali ? formatWibDate(kembali.tanggalKembali) : '—'}
                    </TableCell>
                    <TableCell className="text-sm">{kembali ? String(kembali.kondisiBuku || '-') : '—'}</TableCell>
                  </TableRow>
                )
              })}
              {!rows.length && (
                <tr>
                  <td colSpan={8} className="h-24 text-center text-sm text-muted-foreground">
                    {applied ? 'Tidak ada data untuk filter ini.' : 'Pilih filter lalu klik Tampilkan.'}
                  </td>
                </tr>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
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
