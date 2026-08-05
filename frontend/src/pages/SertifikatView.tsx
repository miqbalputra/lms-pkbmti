import { useEffect, useState } from 'react'
import { Plus, Printer } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { request } from '../lib/api'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

function fmtDate(v: unknown): string {
  if (!v) return ''
  return String(v).slice(0, 10)
}

export function SertifikatView({
  token,
  readOnly,
}: {
  token: string
  readOnly: boolean
}) {
  const [rows, setRows] = useState<Row[]>([])
  const [program, setProgram] = useState<Row[]>([])
  const [siswa, setSiswa] = useState<Row[]>([])
  const [adding, setAdding] = useState(false)
  const [filterNama, setFilterNama] = useState('')
  const [form, setForm] = useState({ pesertaDidikId: '', programId: '' })
  const [saving, setSaving] = useState(false)

  const canCreate = !readOnly

  const load = () => {
    void request('/sertifikat', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/program', token).then((r: Row[]) => setProgram(r || [])).catch(() => setProgram([]))
    void request('/peserta-didik', token).then((r: Row[]) => setSiswa(r || [])).catch(() => setSiswa([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  const sudahTerbit = new Set(rows.map((r) => String(r.pesertaDidikId || '')))
  const lulusBelumSertifikat = siswa.filter(
    (s) => String(s.status || '') === 'lulus' && !sudahTerbit.has(s.id)
  )
  const filterNamaLower = filterNama.toLowerCase()
  const pilihanSiswa = lulusBelumSertifikat.filter((s) =>
    !filterNamaLower ||
    String(s.nama || '').toLowerCase().includes(filterNamaLower) ||
    String(s.nisn || '').toLowerCase().includes(filterNamaLower)
  )

  function openAdd() {
    setForm({ pesertaDidikId: '', programId: '' })
    setFilterNama('')
    setAdding(true)
  }

  async function submit() {
    if (!form.pesertaDidikId || !form.programId) {
      toast.error('Pilih peserta didik dan program.')
      return
    }
    setSaving(true)
    try {
      await request('/sertifikat', token, 'POST', form)
      toast.success('Sertifikat diterbitkan.')
      setAdding(false)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menerbitkan sertifikat.')
    } finally {
      setSaving(false)
    }
  }

  async function cetak(r: Row) {
    try {
      const res = await fetch(apiBase + '/sertifikat/' + r.id + '/print', {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('gagal mencetak')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'sertifikat-' + String(r.nomor || '') + '.pdf'
      a.click()
      URL.revokeObjectURL(url)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mencetak sertifikat.')
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Sertifikat"
        description="Terbitkan & cetak sertifikat kelulusan per peserta didik (Paket A/B/C) + QR verifikasi."
        actions={
          canCreate && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Terbitkan
            </Button>
          )
        }
      />

      {adding && canCreate && (
        <FormCard
          title="Terbitkan Sertifikat"
          description="Hanya peserta didik berstatus lulus yang belum memiliki sertifikat."
        >
          <div className="grid gap-4">
            <div className="grid gap-2">
              <Label>Program (Paket)</Label>
              <Select value={form.programId} onChange={(e) => setForm({ ...form, programId: e.target.value })}>
                <option value="">Pilih program</option>
                {program.map((p) => (
                  <option key={p.id} value={p.id}>
                    {String(p.kode || '')} — {String(p.nama || '')}
                  </option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Cari Peserta Didik</Label>
              <Input
                value={filterNama}
                onChange={(e) => setFilterNama(e.target.value)}
                placeholder="Nama atau NISN..."
              />
            </div>
            <div className="grid gap-2">
              <Label>Peserta Didik (Lulus)</Label>
              <Select
                value={form.pesertaDidikId}
                onChange={(e) => setForm({ ...form, pesertaDidikId: e.target.value })}
                size={Math.min(8, Math.max(3, pilihanSiswa.length))}
              >
                <option value="">Pilih peserta didik...</option>
                {pilihanSiswa.map((s) => (
                  <option key={s.id} value={s.id}>
                    {String(s.nama || '')} — NISN {String(s.nisn || '-')}
                  </option>
                ))}
              </Select>
              {!pilihanSiswa.length && (
                <p className="text-xs text-muted-foreground">
                  Tidak ada peserta didik lulus yang belum bersertifikat.
                </p>
              )}
            </div>
            <div className="flex gap-2">
              <Button onClick={submit} disabled={saving}>
                {saving ? 'Menerbitkan...' : 'Terbitkan sertifikat'}
              </Button>
              <Button type="button" variant="outline" onClick={() => setAdding(false)}>Batal</Button>
            </div>
          </div>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Nomor</TableHead>
              <TableHead>Peserta Didik</TableHead>
              <TableHead>Program</TableHead>
              <TableHead>Tanggal Terbit</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const pd = (r.pesertaDidik as Row) || {}
              const prog = (r.program as Row) || {}
              return (
                <TableRow key={r.id}>
                  <TableCell className="font-mono text-xs">{String(r.nomor || '-')}</TableCell>
                  <TableCell>
                    <div className="font-medium">{String(pd.nama || '-')}</div>
                    <div className="text-xs text-muted-foreground">NISN {String(pd.nisn || '-')}</div>
                  </TableCell>
                  <TableCell>{String(prog.nama || '-')}</TableCell>
                  <TableCell className="text-sm">{fmtDate(r.tanggalTerbit)}</TableCell>
                  <TableCell>
                    <Badge variant={r.status === 'terbit' ? 'default' : 'secondary'}>
                      {String(r.status || '-')}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button size="sm" variant="outline" onClick={() => cetak(r)}>
                        <Printer className="h-3.5 w-3.5" /> Cetak
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
            {!rows.length && <EmptyState colSpan={6} label="Belum ada sertifikat diterbitkan." />}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}