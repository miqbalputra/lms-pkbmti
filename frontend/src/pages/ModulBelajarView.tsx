import { Fragment, useEffect, useState, type FormEvent } from 'react'
import { ChevronDown, ChevronRight, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { request } from '../lib/api'

type Row = Record<string, unknown> & { id: string }

const emptyForm = { mapelId: '', judul: '', urutan: 0, deskripsi: '' }
const emptyOutcome = { kode: '', deskripsi: '' }

export function ModulBelajarView({ token, readOnly }: { token: string; readOnly: boolean }) {
  const [mapel, setMapel] = useState<Row[]>([])
  const [rows, setRows] = useState<Row[]>([])
  const [mapelId, setMapelId] = useState('')
  const [adding, setAdding] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [editId, setEditId] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [outcomes, setOutcomes] = useState<Record<string, Row[]>>({})
  const [ocForm, setOcForm] = useState<Record<string, { kode: string; deskripsi: string }>>({})
  const [ocEdit, setOcEdit] = useState<Record<string, string | null>>({})

  function load() {
    const q = mapelId ? '?mapelId=' + mapelId : ''
    void request('/modul-belajar' + q, token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
  }, [token])

  useEffect(() => {
    load()
  }, [token, mapelId]) // eslint-disable-line react-hooks/exhaustive-deps

  function loadOutcomes(modulId: string) {
    void request('/modul-belajar/' + modulId + '/outcomes', token)
      .then((r: Row[]) => setOutcomes((o) => ({ ...o, [modulId]: r || [] })))
      .catch(() => setOutcomes((o) => ({ ...o, [modulId]: [] })))
  }

  function toggle(modulId: string) {
    setExpanded((e) => {
      const next = !e[modulId]
      if (next && !outcomes[modulId]) loadOutcomes(modulId)
      return { ...e, [modulId]: next }
    })
  }

  async function saveModul(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.judul.trim() || !form.mapelId) {
      toast.error('Mapel dan judul wajib diisi.')
      return
    }
    try {
      if (editId) {
        await request('/modul-belajar/' + editId, token, 'PUT', form)
        toast.success('Modul diperbarui.')
      } else {
        await request('/modul-belajar', token, 'POST', form)
        toast.success('Modul ditambahkan.')
      }
      setAdding(false); setEditId(null); setForm({ ...emptyForm })
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan modul.')
    }
  }

  async function deleteModul(r: Row) {
    if (!confirm(`Hapus modul "${String(r.judul || '')}" beserta capaian?`)) return
    try {
      await request('/modul-belajar/' + r.id, token, 'DELETE')
      toast.success('Modul dihapus.')
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus modul.')
    }
  }

  async function saveOutcome(modulId: string) {
    const f = ocForm[modulId] || emptyOutcome
    if (!f.kode.trim()) { toast.error('Kode capaian wajib diisi.'); return }
    const edit = ocEdit[modulId]
    try {
      if (edit) {
        await request(`/modul-belajar/${modulId}/outcomes/${edit}`, token, 'PUT', f)
        toast.success('Capaian diperbarui.')
      } else {
        await request('/modul-belajar/' + modulId + '/outcomes', token, 'POST', f)
        toast.success('Capaian ditambahkan.')
      }
      setOcForm((o) => ({ ...o, [modulId]: { ...emptyOutcome } }))
      setOcEdit((o) => ({ ...o, [modulId]: null }))
      void loadOutcomes(modulId)
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan capaian.')
    }
  }

  async function deleteOutcome(modulId: string, oc: Row) {
    if (!confirm(`Hapus capaian "${String(oc.kode || '')}"?`)) return
    try {
      await request(`/modul-belajar/${modulId}/outcomes/${oc.id}`, token, 'DELETE')
      toast.success('Capaian dihapus.')
      void loadOutcomes(modulId)
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus capaian.')
    }
  }

  const mapelName = (id: string) => String(((mapel.find((m) => m.id === id) || {}) as Row).namaMapel || '-')

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Modul Pembelajaran"
        description="Struktur modul per mapel (urutan, deskripsi) beserta capaian (outcomes). Terkait opsional ke materi/tugas."
        actions={
          !readOnly && (
            <Button onClick={() => { setForm({ ...emptyForm }); setEditId(null); setAdding(true) }}>
              <Plus className="h-4 w-4" /> Tambah modul
            </Button>
          )
        }
      />

      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs">
        <div className="grid gap-2 sm:max-w-sm">
          <Label>Filter Mapel</Label>
          <Select value={mapelId} onChange={(e) => setMapelId(e.target.value)}>
            <option value="">Semua mapel</option>
            {mapel.map((m) => (
              <option key={m.id} value={m.id}>{String(m.namaMapel || '-')}</option>
            ))}
          </Select>
        </div>
      </Card>

      {adding && !readOnly && (
        <FormCard title={editId ? 'Edit Modul' : 'Tambah Modul'} description="Modul pembelajaran per mapel.">
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={saveModul}>
            <div className="grid gap-2">
              <Label>Mata Pelajaran</Label>
              <Select value={form.mapelId} onChange={(e) => setForm({ ...form, mapelId: e.target.value })} required>
                <option value="">Pilih mapel...</option>
                {mapel.map((m) => (
                  <option key={m.id} value={m.id}>{String(m.namaMapel || '-')}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Urutan</Label>
              <Input type="number" min={0} value={form.urutan} onChange={(e) => setForm({ ...form, urutan: Number(e.target.value) })} />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Judul Modul</Label>
              <Input value={form.judul} onChange={(e) => setForm({ ...form, judul: e.target.value })} required />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Deskripsi</Label>
              <textarea
                className="flex min-h-[70px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.deskripsi}
                onChange={(e) => setForm({ ...form, deskripsi: e.target.value })}
              />
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit">{editId ? 'Simpan perubahan' : 'Simpan modul'}</Button>
              <Button type="button" variant="outline" onClick={() => { setAdding(false); setEditId(null) }}>Batal</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead className="w-10"></TableHead>
              <TableHead>Urutan</TableHead>
              <TableHead>Mapel</TableHead>
              <TableHead>Judul</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const open = !!expanded[r.id]
              const oc = outcomes[r.id] || []
              const f = ocForm[r.id] || emptyOutcome
              return (
                <Fragment key={r.id}>
                  <TableRow>
                    <TableCell>
                      <button aria-label={open ? 'Tutup' : 'Buka'} className="text-muted-foreground hover:text-foreground" onClick={() => toggle(r.id)}>
                        {open ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                      </button>
                    </TableCell>
                    <TableCell className="font-mono">{String(r.urutan ?? 0)}</TableCell>
                    <TableCell>{mapelName(String(r.mapelId || ''))}</TableCell>
                    <TableCell className="font-medium">{String(r.judul || '-')}</TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        {!readOnly && (
                          <Button size="sm" variant="outline" onClick={() => {
                            setEditId(r.id)
                            setForm({
                              mapelId: String(r.mapelId || ''),
                              judul: String(r.judul || ''),
                              urutan: Number(r.urutan ?? 0),
                              deskripsi: String(r.deskripsi || ''),
                            })
                            setAdding(true)
                          }}>Ubah</Button>
                        )}
                        {!readOnly && (
                          <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => deleteModul(r)}><Trash2 className="h-3.5 w-3.5" /></Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                  {open && (
                    <TableRow className="bg-secondary/30 hover:bg-secondary/30">
                      <TableCell colSpan={5} className="p-4">
                        <div className="space-y-3">
                          <div className="text-xs text-muted-foreground">
                            {String(r.deskripsi || '—')}
                          </div>
                          <Table>
                            <TableHeader>
                              <TableRow className="border-b border-border">
                                <TableHead className="font-bold text-xs uppercase tracking-wider">Kode</TableHead>
                                <TableHead className="font-bold text-xs uppercase tracking-wider">Deskripsi Capaian</TableHead>
                                <TableHead className="text-right font-bold text-xs uppercase tracking-wider">Aksi</TableHead>
                              </TableRow>
                            </TableHeader>
                            <TableBody>
                              {oc.map((c) => (
                                <TableRow key={c.id}>
                                  <TableCell className="font-mono">{String(c.kode || '-')}</TableCell>
                                  <TableCell className="text-sm">{String(c.deskripsi || '-')}</TableCell>
                                  <TableCell>
                                    <div className="flex justify-end gap-1">
                                      {!readOnly && (
                                        <Button size="sm" variant="outline" onClick={() => {
                                          setOcEdit((o) => ({ ...o, [r.id]: c.id }))
                                          setOcForm((o) => ({ ...o, [r.id]: { kode: String(c.kode || ''), deskripsi: String(c.deskripsi || '') } }))
                                        }}>Ubah</Button>
                                      )}
                                      {!readOnly && (
                                        <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => deleteOutcome(r.id, c)}><Trash2 className="h-3.5 w-3.5" /></Button>
                                      )}
                                    </div>
                                  </TableCell>
                                </TableRow>
                              ))}
                              {!oc.length && <EmptyState colSpan={3} label="Belum ada capaian." />}
                            </TableBody>
                          </Table>
                          {!readOnly && (
                            <div className="grid gap-2 sm:grid-cols-[120px_1fr_auto] items-end">
                              <div className="grid gap-1">
                                <Label className="text-xs">Kode</Label>
                                <Input value={f.kode} onChange={(e) => setOcForm((o) => ({ ...o, [r.id]: { ...(o[r.id] || emptyOutcome), kode: e.target.value } }))} />
                              </div>
                              <div className="grid gap-1">
                                <Label className="text-xs">Deskripsi</Label>
                                <Input value={f.deskripsi} onChange={(e) => setOcForm((o) => ({ ...o, [r.id]: { ...(o[r.id] || emptyOutcome), deskripsi: e.target.value } }))} />
                              </div>
                              <div className="flex gap-1">
                                <Button size="sm" onClick={() => saveOutcome(r.id)}>{ocEdit[r.id] ? 'Simpan' : '+ Capaian'}</Button>
                                {ocEdit[r.id] && (
                                  <Button size="sm" variant="outline" onClick={() => { setOcEdit((o) => ({ ...o, [r.id]: null })); setOcForm((o) => ({ ...o, [r.id]: { ...emptyOutcome } })) }}>Batal</Button>
                                )}
                              </div>
                            </div>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  )}
                </Fragment>
              )
            })}
            {!rows.length && <EmptyState colSpan={5} label="Belum ada modul pembelajaran." />}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}