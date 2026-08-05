import { Fragment, useEffect, useState, type FormEvent } from 'react'
import { ChevronDown, ChevronRight, Plus, Trash2 } from 'lucide-react'
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

type Row = Record<string, unknown> & { id: string }

const emptyForm = { mapelId: '', nama: '' }
const emptyOutcome = { kode: '', deskripsi: '' }

export function KompetensiView({ token, user, readOnly }: { token: string; user: User; readOnly: boolean }) {
  const [mapel, setMapel] = useState<Row[]>([])
  const [kelas, setKelas] = useState<Row[]>([])
  const [rows, setRows] = useState<Row[]>([])
  const [mapelId, setMapelId] = useState('')
  const [adding, setAdding] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [editId, setEditId] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [outcomes, setOutcomes] = useState<Record<string, Row[]>>({})
  const [ocForm, setOcForm] = useState<Record<string, { kode: string; deskripsi: string }>>({})
  const [ocEdit, setOcEdit] = useState<Record<string, string | null>>({})

  // Rombel-kompetensi assignment.
  const [kelasId, setKelasId] = useState('')
  const [rombel, setRombel] = useState<Row[]>([])

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  function load() {
    const q = mapelId ? '?mapelId=' + mapelId : ''
    void request('/kompetensi' + q, token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
  }, [token])

  useEffect(() => { load() }, [token, mapelId]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!kelasId) { setRombel([]); return }
    void request('/rombel-kompetensi?kelasId=' + kelasId, token).then((r: Row[]) => setRombel(r || [])).catch(() => setRombel([]))
  }, [kelasId, token]) // eslint-disable-line react-hooks/exhaustive-deps

  function loadOutcomes(kid: string) {
    void request('/kompetensi/' + kid + '/outcomes', token)
      .then((r: Row[]) => setOutcomes((o) => ({ ...o, [kid]: r || [] })))
      .catch(() => setOutcomes((o) => ({ ...o, [kid]: [] })))
  }

  function toggle(kid: string) {
    setExpanded((e) => {
      const next = !e[kid]
      if (next && !outcomes[kid]) loadOutcomes(kid)
      return { ...e, [kid]: next }
    })
  }

  async function saveKompetensi(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.nama.trim() || !form.mapelId) {
      toast.error('Mapel dan nama kompetensi wajib diisi.')
      return
    }
    try {
      if (editId) {
        await request('/kompetensi/' + editId, token, 'PUT', form)
        toast.success('Kompetensi diperbarui.')
      } else {
        await request('/kompetensi', token, 'POST', form)
        toast.success('Kompetensi ditambahkan.')
      }
      setAdding(false); setEditId(null); setForm({ ...emptyForm })
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan kompetensi.')
    }
  }

  async function deleteKompetensi(r: Row) {
    if (!confirm(`Hapus kompetensi "${String(r.nama || '')}" beserta capaian & penugasan rombel?`)) return
    try {
      await request('/kompetensi/' + r.id, token, 'DELETE')
      toast.success('Kompetensi dihapus.')
      void load()
      if (kelasId) void request('/rombel-kompetensi?kelasId=' + kelasId, token).then((r2: Row[]) => setRombel(r2 || [])).catch((e: unknown) => console.warn('gagal memuat ulang rombel-kompetensi:', e))
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus kompetensi.')
    }
  }

  async function saveOutcome(kid: string) {
    const f = ocForm[kid] || emptyOutcome
    if (!f.kode.trim()) { toast.error('Kode capaian wajib diisi.'); return }
    const edit = ocEdit[kid]
    try {
      if (edit) {
        await request(`/kompetensi/${kid}/outcomes/${edit}`, token, 'PUT', f)
        toast.success('Capaian diperbarui.')
      } else {
        await request('/kompetensi/' + kid + '/outcomes', token, 'POST', f)
        toast.success('Capaian ditambahkan.')
      }
      setOcForm((o) => ({ ...o, [kid]: { ...emptyOutcome } }))
      setOcEdit((o) => ({ ...o, [kid]: null }))
      void loadOutcomes(kid)
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan capaian.')
    }
  }

  async function deleteOutcome(kid: string, oc: Row) {
    if (!confirm(`Hapus capaian "${String(oc.kode || '')}"?`)) return
    try {
      await request(`/kompetensi/${kid}/outcomes/${oc.id}`, token, 'DELETE')
      toast.success('Capaian dihapus.')
      void loadOutcomes(kid)
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus capaian.')
    }
  }

  const assignedIds = new Set(rombel.map((r) => String(r.kompetensiId || '')))

  async function toggleAssign(k: Row) {
    if (!kelasId) { toast.error('Pilih kelas dahulu.'); return }
    const existing = rombel.find((r) => String(r.kompetensiId || '') === k.id)
    try {
      if (existing) {
        await request('/rombel-kompetensi/' + existing.id, token, 'DELETE')
        toast.success('Kompetensi dilepas dari kelas.')
      } else {
        await request('/rombel-kompetensi', token, 'POST', { kelasId, kompetensiId: k.id })
        toast.success('Kompetensi ditugaskan ke kelas.')
      }
      void request('/rombel-kompetensi?kelasId=' + kelasId, token).then((r: Row[]) => setRombel(r || [])).catch((e: unknown) => console.warn('gagal memuat ulang rombel-kompetensi:', e))
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengubah penugasan.')
    }
  }

  const mapelName = (id: string) => String(((mapel.find((m) => m.id === id) || {}) as Row).namaMapel || '-')

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Kompetensi & Penugasan Rombel"
        description="Master kompetensi per mapel beserta capaian, dan penugasan kompetensi ke kelas/rombel (untuk input nilai kompetensi)."
        actions={
          !readOnly && (
            <Button onClick={() => { setForm({ ...emptyForm }); setEditId(null); setAdding(true) }}>
              <Plus className="h-4 w-4" /> Tambah kompetensi
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard title={editId ? 'Edit Kompetensi' : 'Tambah Kompetensi'} description="Kompetensi inti per mapel.">
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={saveKompetensi}>
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
              <Label>Nama Kompetensi</Label>
              <Input value={form.nama} onChange={(e) => setForm({ ...form, nama: e.target.value })} required />
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit">{editId ? 'Simpan perubahan' : 'Simpan kompetensi'}</Button>
              <Button type="button" variant="outline" onClick={() => { setAdding(false); setEditId(null) }}>Batal</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs space-y-3">
        <div className="grid gap-2 sm:max-w-sm">
          <Label>Filter Mapel</Label>
          <Select value={mapelId} onChange={(e) => setMapelId(e.target.value)}>
            <option value="">Semua mapel</option>
            {mapel.map((m) => (
              <option key={m.id} value={m.id}>{String(m.namaMapel || '-')}</option>
            ))}
          </Select>
        </div>
        <div className="grid gap-2 sm:max-w-sm">
          <Label>Filter / Penugasan Kelas</Label>
          <Select value={kelasId} onChange={(e) => setKelasId(e.target.value)}>
            <option value="">— Tanpa filter kelas —</option>
            {kelasOptions.map((k) => (
              <option key={k.id} value={k.id}>Kelas {String(k.jenjang ?? '')}{String(k.namaRombel ?? '')}</option>
            ))}
          </Select>
          {isGuru && !kelasOptions.length && (
            <p className="text-xs text-muted-foreground">Anda belum ditugaskan sebagai wali kelas.</p>
          )}
        </div>
        {kelasId && (
          <p className="text-xs text-muted-foreground">
            Centang kolom "Ditugaskan" untuk menugaskan kompetensi ke kelas ini. Kompetensi yang ditugaskan muncul di matriks nilai kompetensi.
          </p>
        )}
      </Card>

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead className="w-10"></TableHead>
              <TableHead>Mapel</TableHead>
              <TableHead>Kompetensi</TableHead>
              {kelasId && <TableHead>Ditugaskan</TableHead>}
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const open = !!expanded[r.id]
              const oc = outcomes[r.id] || []
              const f = ocForm[r.id] || emptyOutcome
              const assigned = assignedIds.has(r.id)
              return (
                <Fragment key={r.id}>
                  <TableRow>
                    <TableCell>
                      <button aria-label={open ? 'Tutup' : 'Buka'} className="text-muted-foreground hover:text-foreground" onClick={() => toggle(r.id)}>
                        {open ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                      </button>
                    </TableCell>
                    <TableCell>{mapelName(String(r.mapelId || ''))}</TableCell>
                    <TableCell className="font-medium">{String(r.nama || '-')}</TableCell>
                    {kelasId && (
                      <TableCell>
                        {!readOnly ? (
                          <input
                            type="checkbox"
                            className="h-4 w-4 rounded border-border accent-primary"
                            checked={assigned}
                            onChange={() => toggleAssign(r)}
                          />
                        ) : (
                          <Badge variant={assigned ? 'default' : 'secondary'}>{assigned ? 'Ya' : 'Tidak'}</Badge>
                        )}
                      </TableCell>
                    )}
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        {!readOnly && (
                          <Button size="sm" variant="outline" onClick={() => {
                            setEditId(r.id)
                            setForm({ mapelId: String(r.mapelId || ''), nama: String(r.nama || '') })
                            setAdding(true)
                          }}>Ubah</Button>
                        )}
                        {!readOnly && (
                          <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => deleteKompetensi(r)}><Trash2 className="h-3.5 w-3.5" /></Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                  {open && (
                    <TableRow className="bg-secondary/30 hover:bg-secondary/30">
                      <TableCell colSpan={kelasId ? 5 : 4} className="p-4">
                        <div className="space-y-3">
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
            {!rows.length && <EmptyState colSpan={kelasId ? 5 : 4} label="Belum ada kompetensi." />}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}