import { useEffect, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'
import { formatWibDate, wibDateTimeLocalToISO } from '../lib/wib'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

function fmtDate(v: unknown): string {
  return formatWibDate(v)
}

const emptyForm = { pesertaDidikId: '', kelasId: '', tanggal: '', kategori: 'positif', deskripsi: '' }

export function PerilakuView({
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
  const [rows, setRows] = useState<Row[]>([])
  const [form, setForm] = useState({ ...emptyForm })
  const [saving, setSaving] = useState(false)

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
      setRows([])
      return
    }
    void request('/peserta-didik?kelasId=' + kelasId, token).then((r: Row[]) => setSiswa(r || [])).catch(() => setSiswa([]))
    void request('/perilaku?kelasId=' + kelasId, token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }, [kelasId, token]) // eslint-disable-line react-hooks/exhaustive-deps

  function loadRows() {
    if (!kelasId) return
    void request('/perilaku?kelasId=' + kelasId, token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.kelasId || !form.pesertaDidikId || !form.deskripsi.trim()) {
      toast.error('Kelas, peserta didik, dan deskripsi wajib diisi.')
      return
    }
    setSaving(true)
    try {
      await request('/perilaku', token, 'POST', {
        pesertaDidikId: form.pesertaDidikId,
        kelasId: form.kelasId,
        tanggal: form.tanggal ? wibDateTimeLocalToISO(form.tanggal) : undefined,
        kategori: form.kategori,
        deskripsi: form.deskripsi,
      })
      toast.success('Catatan perilaku disimpan.')
      setForm({ ...emptyForm, kelasId: form.kelasId })
      void loadRows()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan catatan.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Catatan Perilaku"
        description="Catat perilaku positif/negatif peserta didik — teragregasi ke rapor sebagai kepribadian."
      />

      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs">
        <div className="grid gap-2 sm:max-w-sm">
          <Label>Kelas / Rombel</Label>
          <Select value={kelasId} onChange={(e) => { setKelasId(e.target.value); setForm({ ...emptyForm, kelasId: e.target.value }) }}>
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

      {kelasId && !readOnly && (
        <FormCard title="Tambah Catatan" description="Pilih peserta didik di kelas ini, lalu isi catatan.">
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2">
              <Label>Peserta Didik</Label>
              <Select value={form.pesertaDidikId} onChange={(e) => setForm({ ...form, pesertaDidikId: e.target.value })} required>
                <option value="">Pilih peserta didik...</option>
                {siswa.map((s) => (
                  <option key={s.id} value={s.id}>{String(s.nama || '')} — {String(s.nisn || '-')}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Tanggal</Label>
              <Input type="date" value={form.tanggal} onChange={(e) => setForm({ ...form, tanggal: e.target.value })} />
            </div>
            <div className="grid gap-2">
              <Label>Kategori</Label>
              <Select value={form.kategori} onChange={(e) => setForm({ ...form, kategori: e.target.value })}>
                <option value="positif">Positif</option>
                <option value="negatif">Negatif</option>
              </Select>
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Deskripsi</Label>
              <textarea
                className="flex min-h-[70px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.deskripsi}
                onChange={(e) => setForm({ ...form, deskripsi: e.target.value })}
                required
              />
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan catatan'}</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Tanggal</TableHead>
              <TableHead>Peserta Didik</TableHead>
              <TableHead>Kategori</TableHead>
              <TableHead>Deskripsi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const pd = (r.pesertaDidik as Row) || {}
              return (
                <TableRow key={r.id}>
                  <TableCell className="text-sm">{fmtDate(r.tanggal)}</TableCell>
                  <TableCell className="font-medium">{String(pd.nama || '-')}</TableCell>
                  <TableCell>
                    <Badge variant={r.kategori === 'positif' ? 'default' : 'destructive'}>
                      {String(r.kategori || '-')}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm">{String(r.deskripsi || '-')}</TableCell>
                </TableRow>
              )
            })}
            {!rows.length && <EmptyState colSpan={4} label={kelasId ? 'Belum ada catatan perilaku di kelas ini.' : 'Pilih kelas untuk menampilkan catatan.'} />}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
