import { useEffect, useState, type FormEvent } from 'react'
import { Plus, Save, Trash2 } from 'lucide-react'
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

type Row = Record<string, unknown> & { id: string }

const emptyForm = { kode: '', nama: '', bolehDipakai: true }

export function SumberNilaiView({ token, readOnly }: { token: string; readOnly: boolean }) {
  const [sumber, setSumber] = useState<Row[]>([])
  const [mapel, setMapel] = useState<Row[]>([])
  const [bobot, setBobot] = useState<Row[]>([])
  const [adding, setAdding] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [editId, setEditId] = useState<string | null>(null)
  const [mapelId, setMapelId] = useState('')
  const [draft, setDraft] = useState<Record<string, string>>({})

  const load = () => {
    void request('/sumber-nilai', token).then((r: Row[]) => setSumber(r || [])).catch(() => setSumber([]))
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
    void request('/bobot-sumber-nilai', token).then((r: Row[]) => setBobot(r || [])).catch(() => setBobot([]))
  }

  useEffect(() => {
    load()
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  // Bobot for the selected mapel, keyed by sumberId.
  const bobotBySumber = new Map(
    bobot.filter((b) => String(b.mapelId || '') === mapelId).map((b) => [String(b.sumberId || ''), b])
  )

  function bobotVal(suId: string): string {
    if (draft[suId] !== undefined) return draft[suId]
    const b = bobotBySumber.get(suId)
    return b ? String(b.bobot) : ''
  }

  async function saveSumber(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.kode.trim() || !form.nama.trim()) {
      toast.error('Kode dan nama wajib diisi.')
      return
    }
    try {
      if (editId) {
        await request('/sumber-nilai/' + editId, token, 'PUT', form)
        toast.success('Sumber nilai diperbarui.')
      } else {
        await request('/sumber-nilai', token, 'POST', form)
        toast.success('Sumber nilai ditambahkan.')
      }
      setAdding(false)
      setEditId(null)
      setForm({ ...emptyForm })
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan sumber nilai.')
    }
  }

  async function deleteSumber(r: Row) {
    if (!confirm(`Hapus sumber "${String(r.nama || '')}"? Bobot terkait juga akan terhapus.`)) return
    try {
      await request('/sumber-nilai/' + r.id, token, 'DELETE')
      toast.success('Sumber nilai dihapus.')
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus sumber nilai.')
    }
  }

  async function saveBobot(su: Row) {
    if (!mapelId) {
      toast.error('Pilih mapel terlebih dahulu.')
      return
    }
    const v = Number(bobotVal(su.id))
    if (isNaN(v) || v < 0 || v > 100) {
      toast.error('Bobot harus angka 0–100.')
      return
    }
    try {
      await request('/bobot-sumber-nilai', token, 'POST', { mapelId, sumberId: su.id, bobot: v })
      toast.success('Bobot disimpan.')
      setDraft((d) => { const n = { ...d }; delete n[su.id]; return n })
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan bobot.')
    }
  }

  async function deleteBobot(su: Row) {
    const b = bobotBySumber.get(su.id)
    if (!b) return
    try {
      await request('/bobot-sumber-nilai/' + b.id, token, 'DELETE')
      toast.success('Bobot dihapus.')
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus bobot.')
    }
  }

  const totalBobot = sumber.reduce((acc, su) => {
    const v = Number(bobotVal(su.id))
    return acc + (isNaN(v) ? 0 : v)
  }, 0)

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Sumber Nilai & Bobot"
        description="Generalisasi sumber nilai (UM/TUGAS/UJIAN/PRAKTIK) & bobot per mapel — menghitung NA gabungan."
        actions={
          !readOnly && (
            <Button onClick={() => { setForm({ ...emptyForm }); setEditId(null); setAdding(true) }}>
              <Plus className="h-4 w-4" /> Tambah sumber
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard title={editId ? 'Edit Sumber Nilai' : 'Tambah Sumber Nilai'} description="Kode unik (mis. UM, TUGAS, UJIAN, PRAKTIK).">
          <form className="grid gap-4 sm:grid-cols-3" onSubmit={saveSumber}>
            <div className="grid gap-2">
              <Label>Kode</Label>
              <Input value={form.kode} onChange={(e) => setForm({ ...form, kode: e.target.value })} disabled={!!editId} required />
            </div>
            <div className="grid gap-2">
              <Label>Nama</Label>
              <Input value={form.nama} onChange={(e) => setForm({ ...form, nama: e.target.value })} required />
            </div>
            <div className="flex items-end gap-2 pb-1">
              <input
                id="boleh"
                type="checkbox"
                className="h-4 w-4 rounded border-border accent-primary"
                checked={form.bolehDipakai}
                onChange={(e) => setForm({ ...form, bolehDipakai: e.target.checked })}
              />
              <Label htmlFor="boleh" className="cursor-pointer">Boleh dipakai</Label>
            </div>
            <div className="flex gap-2 sm:col-span-3">
              <Button type="submit">{editId ? 'Simpan perubahan' : 'Simpan sumber'}</Button>
              <Button type="button" variant="outline" onClick={() => { setAdding(false); setEditId(null) }}>Batal</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <div className="px-4 py-3 border-b border-border">
          <h3 className="text-base font-bold">Daftar Sumber Nilai</h3>
        </div>
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Kode</TableHead>
              <TableHead>Nama</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sumber.map((r) => (
              <TableRow key={r.id}>
                <TableCell className="font-mono">{String(r.kode || '-')}</TableCell>
                <TableCell className="font-medium">{String(r.nama || '-')}</TableCell>
                <TableCell>
                  <Badge variant={r.bolehDipakai ? 'default' : 'secondary'}>
                    {r.bolehDipakai ? 'Aktif' : 'Nonaktif'}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    {!readOnly && (
                      <Button size="sm" variant="outline" onClick={() => {
                        setEditId(r.id)
                        setForm({ kode: String(r.kode || ''), nama: String(r.nama || ''), bolehDipakai: !!r.bolehDipakai })
                        setAdding(true)
                      }}>Ubah</Button>
                    )}
                    {!readOnly && (
                      <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => deleteSumber(r)}><Trash2 className="h-3.5 w-3.5" /></Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {!sumber.length && <EmptyState colSpan={4} label="Belum ada sumber nilai." />}
          </TableBody>
        </Table>
      </Card>

      <Card className="rounded-2xl border border-border bg-card p-5 shadow-2xs space-y-4">
        <div className="grid gap-2 sm:max-w-sm">
          <Label>Bobot per Mapel</Label>
          <Select value={mapelId} onChange={(e) => { setMapelId(e.target.value); setDraft({}) }}>
            <option value="">Pilih mapel untuk atur bobot...</option>
            {mapel.map((m) => (
              <option key={m.id} value={m.id}>{String(m.namaMapel || '-')}</option>
            ))}
          </Select>
        </div>

        {mapelId && (
          <>
            <Table>
              <TableHeader>
                <TableRow className="border-b border-border">
                  <TableHead>Sumber</TableHead>
                  <TableHead>Bobot (%)</TableHead>
                  <TableHead className="text-right">Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sumber.map((su) => {
                  const exists = bobotBySumber.has(su.id)
                  return (
                    <TableRow key={su.id}>
                      <TableCell className="font-medium">{String(su.kode || '')} — {String(su.nama || '')}</TableCell>
                      <TableCell>
                        <Input
                          type="number"
                          min={0}
                          max={100}
                          value={bobotVal(su.id)}
                          disabled={readOnly}
                          onChange={(e) => setDraft((d) => ({ ...d, [su.id]: e.target.value }))}
                          className="h-9 w-28"
                        />
                      </TableCell>
                      <TableCell>
                        <div className="flex justify-end gap-1">
                          {!readOnly && (
                            <Button size="sm" variant="outline" onClick={() => saveBobot(su)}><Save className="h-3.5 w-3.5" /> Simpan</Button>
                          )}
                          {!readOnly && exists && (
                            <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => deleteBobot(su)}><Trash2 className="h-3.5 w-3.5" /></Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
            <div className="text-sm text-muted-foreground">
              Total bobot: <strong className={totalBobot === 100 ? 'text-success' : 'text-destructive'}>{totalBobot}%</strong>
              {totalBobot !== 100 && ' (disarankan 100%; nilai dinormalisasi bila tidak).'}
            </div>
          </>
        )}
      </Card>
    </div>
  )
}