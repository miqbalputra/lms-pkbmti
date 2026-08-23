import { useEffect, useState } from 'react'
import { Printer, Save } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'
import { formatWibDate } from '../lib/wib'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

function fmtNilai(v: unknown): string {
  if (v === null || v === undefined || v === '') return '—'
  return String(v)
}

function fmtDate(v: unknown): string {
  return formatWibDate(v)
}

export function RaporView({
  token,
  user,
  readOnly,
}: {
  token: string
  user: User
  readOnly: boolean
}) {
  const [kelas, setKelas] = useState<Row[]>([])
  const [tahun, setTahun] = useState<Row[]>([])
  const [kelasId, setKelasId] = useState('')
  const [siswa, setSiswa] = useState<Row[]>([])
  const [siswaId, setSiswaId] = useState('')
  const [semester, setSemester] = useState('Ganjil')
  const [tahunId, setTahunId] = useState('')
  const [rapor, setRapor] = useState<Record<string, any> | null>(null)
  const [catatan, setCatatan] = useState('')
  const [naikKelas, setNaikKelas] = useState<boolean | null>(null)
  const [kenaikanKe, setKenaikanKe] = useState('')
  const [saving, setSaving] = useState(false)

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas
  const canEditCatatan = !readOnly

  useEffect(() => {
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
    void request('/tahun-ajaran', token).then((r: Row[]) => {
      setTahun(r || [])
      const aktif = (r || []).find((t) => t.isAktif)
      if (aktif) setTahunId(aktif.id)
    }).catch(() => setTahun([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!kelasId) {
      setSiswa([])
      setSiswaId('')
      return
    }
    void request('/peserta-didik?kelasId=' + kelasId, token).then((r: Row[]) => setSiswa(r || [])).catch(() => setSiswa([]))
    setSiswaId('')
    setRapor(null)
  }, [kelasId, token]) // eslint-disable-line react-hooks/exhaustive-deps

  function loadRapor() {
    if (!siswaId) {
      toast.error('Pilih peserta didik terlebih dahulu.')
      return
    }
    const qs = `?semester=${semester}${tahunId ? `&tahunAjaranId=${tahunId}` : ''}`
    void request('/rapor/' + siswaId + qs, token)
      .then((r: any) => {
        setRapor(r)
        const cr = r.catatanRapor || {}
        setCatatan(String(cr.catatanWali || ''))
        setNaikKelas(cr.naikKelas === true || cr.naikKelas === false ? cr.naikKelas : null)
        setKenaikanKe(String(cr.kenaikanKe || ''))
      })
      .catch((err: any) => {
        toast.error(err.message || 'Gagal memuat rapor.')
        setRapor(null)
      })
  }

  async function cetak() {
    if (!siswaId) return
    try {
      const qs = `?semester=${semester}${tahunId ? `&tahunAjaranId=${tahunId}` : ''}`
      const res = await fetch(apiBase + '/rapor/' + siswaId + '/print' + qs, {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('gagal mencetak')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'rapor-' + (rapor?.pesertaDidik?.nama || 'siswa') + '-' + semester + '.pdf'
      a.click()
      URL.revokeObjectURL(url)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mencetak rapor.')
    }
  }

  async function saveCatatan() {
    if (!siswaId || !tahunId) {
      toast.error('Pilih tahun ajaran dan peserta didik.')
      return
    }
    setSaving(true)
    try {
      await request('/rapor/' + siswaId + '/catatan', token, 'PUT', {
        tahunAjaranId: tahunId,
        semester,
        catatanWali: catatan,
        naikKelas: naikKelas === null ? undefined : naikKelas,
        kenaikanKe: kenaikanKe || undefined,
      })
      toast.success('Catatan rapor disimpan.')
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan catatan.')
    } finally {
      setSaving(false)
    }
  }

  const pd = rapor?.pesertaDidik as Row | undefined
  const kls = rapor?.kelas as Row | undefined
  const ta = rapor?.tahunAjaran as Row | undefined
  const rekap = (rapor?.rekap as Row[]) || []
  const mapelByID = (rapor?.mapelByID as Record<string, Row>) || {}
  const perilaku = (rapor?.perilaku as Row[]) || []

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Rapor"
        description="Agregasi nilai akhir, kepribadian, dan catatan wali per peserta didik. Cetak PDF."
      />

      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs">
        <div className="grid gap-3 sm:grid-cols-4">
          <div className="grid gap-2">
            <Label>Kelas</Label>
            <Select value={kelasId} onChange={(e) => setKelasId(e.target.value)}>
              <option value="">Pilih kelas...</option>
              {kelasOptions.map((k) => (
                <option key={k.id} value={k.id}>{kelasLabel(k)}</option>
              ))}
            </Select>
          </div>
          <div className="grid gap-2">
            <Label>Peserta Didik</Label>
            <Select value={siswaId} onChange={(e) => setSiswaId(e.target.value)} disabled={!kelasId}>
              <option value="">Pilih peserta didik...</option>
              {siswa.map((s) => (
                <option key={s.id} value={s.id}>{String(s.nama || '')}</option>
              ))}
            </Select>
          </div>
          <div className="grid gap-2">
            <Label>Tahun Ajaran</Label>
            <Select value={tahunId} onChange={(e) => setTahunId(e.target.value)}>
              <option value="">Aktif</option>
              {tahun.map((t) => (
                <option key={t.id} value={t.id}>{String(t.namaTahunAjaran || '')}</option>
              ))}
            </Select>
          </div>
          <div className="grid gap-2">
            <Label>Semester</Label>
            <Select value={semester} onChange={(e) => setSemester(e.target.value)}>
              <option value="Ganjil">Ganjil</option>
              <option value="Genap">Genap</option>
            </Select>
          </div>
          <div className="sm:col-span-4 flex gap-2">
            <Button onClick={loadRapor}>Muat Rapor</Button>
            {rapor && (
              <Button variant="outline" onClick={cetak}><Printer className="h-4 w-4" /> Cetak PDF</Button>
            )}
          </div>
        </div>
      </Card>

      {rapor && (
        <>
          <Card className="rounded-2xl border border-border bg-card p-5 shadow-2xs space-y-1">
            <h3 className="text-base font-bold">Identitas Peserta Didik</h3>
            <div className="text-sm">Nama: <strong>{String(pd?.nama || '-')}</strong></div>
            <div className="text-sm">NISN: {String(pd?.nisn || '-')}</div>
            <div className="text-sm">Kelas: {kls ? kelasLabel(kls) : '-'}</div>
            <div className="text-sm">Tahun Ajaran: {String(ta?.namaTahunAjaran || '-')} — Semester {String(rapor.semester || '-')}</div>
          </Card>

          <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
            <div className="px-4 py-3 border-b border-border">
              <h3 className="text-base font-bold">Nilai Akademik</h3>
            </div>
            <Table>
              <TableHeader>
                <TableRow className="border-b border-border">
                  <TableHead>Mata Pelajaran</TableHead>
                  <TableHead>NP</TableHead>
                  <TableHead>NK</TableHead>
                  <TableHead>NA</TableHead>
                  <TableHead>Predikat</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rekap.map((r) => {
                  const mp = mapelByID[String(r.mapelId || '')] || {}
                  const pred = String(r.predikatNA || r.predikatNP || '')
                  return (
                    <TableRow key={r.id}>
                      <TableCell className="font-medium">{String(mp.namaMapel || '-')}</TableCell>
                      <TableCell>{fmtNilai(r.npAkhir)}</TableCell>
                      <TableCell>{fmtNilai(r.nkAkhir)}</TableCell>
                      <TableCell className="font-semibold">{fmtNilai(r.naAkhir)}</TableCell>
                      <TableCell>{pred ? <Badge variant="secondary">{pred}</Badge> : '—'}</TableCell>
                    </TableRow>
                  )
                })}
                {!rekap.length && <EmptyState colSpan={5} label="Belum ada nilai pada semester ini." />}
              </TableBody>
            </Table>
          </Card>

          <Card className="rounded-2xl border border-border bg-card p-5 shadow-2xs space-y-2">
            <h3 className="text-base font-bold">Catatan Kepribadian</h3>
            {perilaku.length ? (
              <ul className="space-y-1 text-sm">
                {perilaku.map((p) => (
                  <li key={p.id} className="flex gap-2">
                    <Badge variant={p.kategori === 'positif' ? 'default' : 'destructive'} className="shrink-0">
                      {fmtDate(p.tanggal)}
                    </Badge>
                    <span>{String(p.deskripsi || '-')}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">Tidak ada catatan perilaku.</p>
            )}
          </Card>

          <Card className="rounded-2xl border border-border bg-card p-5 shadow-2xs space-y-3">
            <h3 className="text-base font-bold">Catatan Wali & Kenaikan</h3>
            <div className="grid gap-2">
              <Label>Catatan Wali Kelas</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-60"
                value={catatan}
                disabled={!canEditCatatan}
                onChange={(e) => setCatatan(e.target.value)}
              />
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="grid gap-2">
                <Label>Kenaikan Kelas</Label>
                <Select
                  value={naikKelas === null ? '' : naikKelas ? 'naik' : 'tinggal'}
                  disabled={!canEditCatatan}
                  onChange={(e) => {
                    const v = e.target.value
                    setNaikKelas(v === 'naik' ? true : v === 'tinggal' ? false : null)
                  }}
                >
                  <option value="">Belum ditentukan</option>
                  <option value="naik">Naik kelas</option>
                  <option value="tinggal">Tinggal di kelas yang sama</option>
                </Select>
              </div>
              <div className="grid gap-2">
                <Label>Kenaikan Ke (opsional)</Label>
                <Input
                  value={kenaikanKe}
                  disabled={!canEditCatatan || naikKelas !== true}
                  onChange={(e) => setKenaikanKe(e.target.value)}
                  placeholder="Contoh: Kelas 7B"
                />
              </div>
            </div>
            {canEditCatatan && (
              <Button onClick={saveCatatan} disabled={saving}>
                <Save className="h-4 w-4" /> {saving ? 'Menyimpan...' : 'Simpan catatan rapor'}
              </Button>
            )}
          </Card>
        </>
      )}
    </div>
  )
}
