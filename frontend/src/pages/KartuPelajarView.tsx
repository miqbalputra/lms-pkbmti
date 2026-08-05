import { useEffect, useState } from 'react'
import { IdCard, Printer, Upload } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Label } from '../components/ui/label'
import { EmptyState, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

export function KartuPelajarView({
  token,
  user,
  readOnly,
}: {
  token: string
  user: User
  readOnly: boolean
}) {
  const [kelas, setKelas] = useState<Row[]>([])
  const [kelasId, setKelasId] = useState('')
  const [siswa, setSiswa] = useState<Row[]>([])

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  useEffect(() => {
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!kelasId) {
      setSiswa([])
      return
    }
    void request('/peserta-didik?kelasId=' + kelasId, token)
      .then((r: Row[]) => setSiswa(r || []))
      .catch(() => setSiswa([]))
  }, [kelasId, token]) // eslint-disable-line react-hooks/exhaustive-deps

  async function download(url: string, fname: string) {
    try {
      const res = await fetch(apiBase + url, {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('gagal mencetak')
      const blob = await res.blob()
      const obj = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = obj
      a.download = fname
      a.click()
      URL.revokeObjectURL(obj)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mencetak kartu.')
    }
  }

  async function cetakGroup() {
    if (!kelasId) {
      toast.error('Pilih kelas terlebih dahulu.')
      return
    }
    await download('/kartu-pelajar/group/' + kelasId + '/print', 'kartu-pelajar-' + kelasId + '.pdf')
  }

  async function cetakSatu(r: Row) {
    await download('/kartu-pelajar/' + r.id + '/print', 'kartu-' + String(r.nama || '') + '.pdf')
  }

  async function uploadFoto(r: Row, file: File) {
    const fd = new FormData()
    fd.append('foto', file)
    try {
      const res = await fetch(apiBase + '/peserta-didik/' + r.id + '/foto', {
        method: 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: fd,
      })
      if (!res.ok) {
        const x = await res.json().catch(() => ({}))
        throw new Error(x.error || 'Gagal mengunggah foto.')
      }
      toast.success('Foto diperbarui.')
      setSiswa((prev) => prev.map((s) => (s.id === r.id ? { ...s, fotoPath: '1' } : s)))
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengunggah foto.')
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Kartu Pelajar"
        description="Cetak kartu pelajar (ID card + QR verifikasi) per siswa atau massal per rombel."
        actions={
          kelasId && (
            <Button onClick={cetakGroup}>
              <Printer className="h-4 w-4" />
              Cetak massal
            </Button>
          )
        }
      />

      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs">
        <div className="grid gap-2 sm:max-w-sm">
          <Label>Kelas / Rombel</Label>
          <Select value={kelasId} onChange={(e) => setKelasId(e.target.value)}>
            <option value="">Pilih kelas...</option>
            {kelasOptions.map((k) => (
              <option key={k.id} value={k.id}>{kelasLabel(k)}</option>
            ))}
          </Select>
          {isGuru && !kelasOptions.length && (
            <p className="text-xs text-muted-foreground">Anda belum ditugaskan sebagai wali kelas.</p>
          )}
        </div>
      </Card>

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Nama</TableHead>
              <TableHead>NISN</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Foto</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {siswa.map((r) => (
              <TableRow key={r.id}>
                <TableCell className="font-medium">{String(r.nama || '-')}</TableCell>
                <TableCell className="text-sm">{String(r.nisn || '-')}</TableCell>
                <TableCell>
                  <Badge variant={String(r.status || '') === 'aktif' ? 'default' : 'secondary'}>
                    {String(r.status || '-')}
                  </Badge>
                </TableCell>
                <TableCell>
                  <label className="inline-flex cursor-pointer items-center gap-1 text-xs text-primary">
                    <Upload className="h-3.5 w-3.5" />
                    {r.fotoPath ? 'Ganti foto' : 'Unggah'}
                    <input
                      type="file"
                      accept="image/png,image/jpeg"
                      className="hidden"
                      disabled={readOnly}
                      onChange={(e) => {
                        const f = e.target.files?.[0]
                        if (f) void uploadFoto(r, f)
                        e.target.value = ''
                      }}
                    />
                  </label>
                </TableCell>
                <TableCell>
                  <div className="flex justify-end">
                    <Button size="sm" variant="outline" onClick={() => cetakSatu(r)}>
                      <IdCard className="h-3.5 w-3.5" /> Cetak
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {!siswa.length && <EmptyState colSpan={5} label={kelasId ? 'Tidak ada peserta didik di kelas ini.' : 'Pilih kelas untuk menampilkan daftar.'} />}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}