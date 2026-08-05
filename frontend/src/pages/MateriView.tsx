import { useEffect, useState, type FormEvent } from 'react'
import { Copy, Download, FileText, Link as LinkIcon, Lock, MessageSquare, Pencil, Plus, Share2, Trash2 } from 'lucide-react'
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
} from '../components/ui/alert-dialog'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../components/ui/dialog'
import type { User } from '../App'
import { request } from '../lib/api'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

function fmtSize(bytes: number): string {
  if (!bytes) return ''
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

const emptyForm = { mapelId: '', kelasId: '', modulId: '', judul: '', deskripsi: '', urutan: '', tanggal: '', linkUrl: '' }

export function MateriView({
  token,
  user,
  readOnly,
}: {
  token: string
  user: User
  readOnly: boolean
}) {
  const [rows, setRows] = useState<Row[]>([])
  const [mapel, setMapel] = useState<Row[]>([])
  const [kelas, setKelas] = useState<Row[]>([])
  const [modul, setModul] = useState<Row[]>([])
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [file, setFile] = useState<File | null>(null)
  const [saving, setSaving] = useState(false)
  const [detail, setDetail] = useState<Row | null>(null)
  const [komentar, setKomentar] = useState<Row[]>([])
  const [newKomentar, setNewKomentar] = useState('')
  const [share, setShare] = useState<{ enabled: boolean; protectedShare: boolean; url: string } | null>(null)
  const [sharePwd, setSharePwd] = useState('')
  const [shareSaving, setShareSaving] = useState(false)
  const [shareCopied, setShareCopied] = useState(false)

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  const load = () => {
    void request('/materi', token).then((r: Row[]) => setRows(r || [])).catch(() => setRows([]))
  }

  useEffect(() => {
    load()
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
    void request('/modul-belajar', token).then((r: Row[]) => setModul(r || [])).catch(() => setModul([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  function openAdd() {
    setForm({ ...emptyForm })
    setEditing(null)
    setFile(null)
    setAdding(true)
  }

  function openEdit(r: Row) {
    setEditing(r)
    const tgl = r.tanggal ? String(r.tanggal).slice(0, 10) : ''
    setForm({
      mapelId: String(r.mapelId || ''),
      kelasId: String(r.kelasId || ''),
      modulId: String(r.modulId || ''),
      judul: String(r.judul || ''),
      deskripsi: String(r.deskripsi || ''),
      urutan: String(r.urutan ?? ''),
      tanggal: tgl,
      linkUrl: String(r.linkUrl || ''),
    })
    setFile(null)
    setAdding(true)
  }

  function canEdit(r: Row): boolean {
    if (user.role === 'admin') return true
    return String(r.dibuatOlehUserId || '') === user.id
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!form.judul || !form.kelasId) {
      toast.error('Judul dan kelas wajib diisi.')
      return
    }
    if (!editing && !file && !form.linkUrl.trim()) {
      toast.error('File atau link materi wajib diisi.')
      return
    }
    setSaving(true)
    try {
      const data = new FormData()
      data.append('mapelId', form.mapelId)
      data.append('kelasId', form.kelasId)
      if (form.modulId) data.append('modulId', form.modulId)
      data.append('judul', form.judul)
      data.append('deskripsi', form.deskripsi)
      data.append('urutan', form.urutan)
      data.append('tanggal', form.tanggal)
      data.append('linkUrl', form.linkUrl.trim())
      if (file) data.append('file', file)

      const r = await fetch(apiBase + '/materi' + (editing ? '/' + editing.id : ''), {
        method: editing ? 'PUT' : 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      const res = await r.json().catch(() => ({}))
      if (!r.ok) throw new Error((res as any)?.error || `Permintaan gagal (${r.status}).`)
      toast.success(editing ? 'Materi diperbarui.' : 'Materi diunggah.')
      setAdding(false)
      setEditing(null)
      setFile(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan materi.')
    } finally {
      setSaving(false)
    }
  }

  async function loadShare(r: Row) {
    try {
      const s = await request('/materi/' + r.id + '/share', token)
      setShare({ enabled: Boolean((s as Row).enabled), protectedShare: Boolean((s as Row).protected), url: String((s as Row).shareUrl || '') })
    } catch {
      setShare(null)
    }
  }

  async function saveShare() {
    if (!detail) return
    setShareSaving(true)
    try {
      const s = await request('/materi/' + detail.id + '/share', token, 'POST', { enabled: true, password: sharePwd.trim() })
      setShare({ enabled: Boolean((s as Row).enabled), protectedShare: Boolean((s as Row).protected), url: String((s as Row).shareUrl || '') })
      setSharePwd('')
      toast.success('Link share diperbarui.')
    } catch (err: any) {
      toast.error(err.message || 'Gagal membagikan materi.')
    } finally {
      setShareSaving(false)
    }
  }

  async function disableShare() {
    if (!detail) return
    setShareSaving(true)
    try {
      await request('/materi/' + detail.id + '/share', token, 'POST', { enabled: false, password: '' })
      setShare({ enabled: false, protectedShare: false, url: '' })
      setSharePwd('')
      toast.success('Link share dinonaktifkan.')
    } catch (err: any) {
      toast.error(err.message || 'Gagal menonaktifkan share.')
    } finally {
      setShareSaving(false)
    }
  }

  async function copyShareUrl() {
    if (!share?.url) return
    try {
      await navigator.clipboard.writeText(share.url)
      setShareCopied(true)
      setTimeout(() => setShareCopied(false), 1500)
    } catch {
      toast.error('Gagal menyalin link.')
    }
  }

  async function confirmDelete() {
    if (!deletingRow) return
    setIsDeleting(true)
    try {
      await request('/materi/' + deletingRow.id, token, 'DELETE')
      toast.success('Materi dihapus.')
      setDeletingRow(null)
      void load()
    } catch (err: any) {
      toast.error(err.message || 'Gagal menghapus materi.')
    } finally {
      setIsDeleting(false)
    }
  }

  async function download(r: Row) {
    try {
      const res = await fetch(apiBase + '/materi/' + r.id + '/download', {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('file tidak tersedia')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = String(r.judul || 'materi') + (String(r.tipe || ''))
      a.click()
      URL.revokeObjectURL(url)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengunduh materi.')
    }
  }

  async function openDetail(r: Row) {
    try {
      const d = await request('/materi/' + r.id, token)
      setDetail(d as Row)
      setKomentar(((d as Row).komentar as Row[]) || [])
      setNewKomentar('')
      setSharePwd('')
      void loadShare(d as Row)
    } catch (err: any) {
      toast.error(err.message || 'Gagal memuat detail materi.')
    }
  }

  async function submitKomentar() {
    if (!detail) return
    if (!newKomentar.trim()) return
    try {
      await request('/materi/' + detail.id + '/komentar', token, 'POST', { isi: newKomentar })
      setNewKomentar('')
      const d = await request('/materi/' + detail.id, token)
      setKomentar(((d as Row).komentar as Row[]) || [])
    } catch (err: any) {
      toast.error(err.message || 'Gagal menambah komentar.')
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Materi Pembelajaran"
        description="Unggah materi per mapel & rombel. Komentar internal staf tersedia di detail."
        actions={
          !readOnly && (
            <Button onClick={openAdd}>
              <Plus className="h-4 w-4" />
              Unggah materi
            </Button>
          )
        }
      />

      {adding && !readOnly && (
        <FormCard title={editing ? 'Edit Materi' : 'Unggah Materi'} description="Bisa upload file dan/atau link. File maks 10 MB (pdf, docx, xlsx, pptx, gambar, mp4, zip).">
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Judul</Label>
              <Input value={form.judul} onChange={(e) => setForm({ ...form, judul: e.target.value })} required />
            </div>
            <div className="grid gap-2">
              <Label>Nomor Urut</Label>
              <Input type="number" min={0} value={form.urutan} onChange={(e) => setForm({ ...form, urutan: e.target.value })} placeholder="0" />
            </div>
            <div className="grid gap-2">
              <Label>Tanggal</Label>
              <Input type="date" value={form.tanggal} onChange={(e) => setForm({ ...form, tanggal: e.target.value })} />
            </div>
            <div className="grid gap-2">
              <Label>Mata Pelajaran</Label>
              <Select value={form.mapelId} onChange={(e) => setForm({ ...form, mapelId: e.target.value })}>
                <option value="">Pilih mapel</option>
                {mapel.map((m) => (
                  <option key={m.id} value={m.id}>{String(m.namaMapel || '-')}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Kelas / Rombel</Label>
              <Select value={form.kelasId} onChange={(e) => setForm({ ...form, kelasId: e.target.value })} required>
                <option value="">Pilih kelas</option>
                {kelasOptions.map((k) => (
                  <option key={k.id} value={k.id}>{kelasLabel(k)}</option>
                ))}
              </Select>
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Modul Pembelajaran (opsional)</Label>
              <Select value={form.modulId} onChange={(e) => setForm({ ...form, modulId: e.target.value })}>
                <option value="">— Tanpa modul —</option>
                {modul
                  .filter((m) => !form.mapelId || String(m.mapelId || '') === form.mapelId)
                  .map((m) => (
                    <option key={m.id} value={m.id}>{String(m.judul || '-')}</option>
                  ))}
              </Select>
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Deskripsi (opsional)</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-xl border border-border bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={form.deskripsi}
                onChange={(e) => setForm({ ...form, deskripsi: e.target.value })}
              />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>Link URL (opsional)</Label>
              <Input type="url" value={form.linkUrl} onChange={(e) => setForm({ ...form, linkUrl: e.target.value })} placeholder="https://... (Google Drive, YouTube, dll)" />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>File {editing ? '(kosongkan untuk pakai file lama)' : ''}</Label>
              <Input type="file" onChange={(e) => setFile(e.target.files?.[0] || null)} required={!editing && !form.linkUrl.trim()} />
            </div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit" disabled={saving}>{saving ? 'Menyimpan...' : editing ? 'Simpan perubahan' : 'Unggah'}</Button>
              <Button type="button" variant="outline" onClick={() => { setAdding(false); setEditing(null); setFile(null) }}>Batal</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead className="w-12">No</TableHead>
              <TableHead>Judul</TableHead>
              <TableHead>Mapel</TableHead>
              <TableHead>Kelas</TableHead>
              <TableHead>File / Link</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const m = (r.mapel as Row) || {}
              const k = (r.kelas as Row) || {}
              const hasFile = !!r.filePath && String(r.tipe || '') !== ''
              const hasLink = !!String(r.linkUrl || '').trim()
              return (
                <TableRow key={r.id}>
                  <TableCell className="text-muted-foreground">{String(r.urutan ?? '') || '-'}</TableCell>
                  <TableCell>
                    <button className="font-medium flex items-center gap-2 text-left hover:underline" onClick={() => openDetail(r)}>
                      <FileText className="h-4 w-4 text-primary" />{String(r.judul || '-')}
                    </button>
                    {r.deskripsi ? <div className="text-xs text-muted-foreground line-clamp-1 max-w-md">{String(r.deskripsi)}</div> : null}
                  </TableCell>
                  <TableCell>{String(m.namaMapel || '-')}</TableCell>
                  <TableCell>{kelasLabel(k)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground space-y-1">
                    {hasFile && <div className="flex items-center gap-1"><FileText className="h-3 w-3" />{String(r.tipe || '')} {fmtSize(Number(r.ukuran) || 0)}</div>}
                    {hasLink && <div className="flex items-center gap-1"><LinkIcon className="h-3 w-3" /><a href={String(r.linkUrl)} target="_blank" rel="noreferrer" className="text-primary hover:underline truncate max-w-[160px]">Link</a></div>}
                    {!hasFile && !hasLink && <span>-</span>}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      {hasFile && <Button size="sm" variant="outline" aria-label="Unduh" onClick={() => download(r)}><Download className="h-3.5 w-3.5" /></Button>}
                      {!readOnly && canEdit(r) && (
                        <Button size="sm" variant="outline" aria-label="Ubah" onClick={() => openEdit(r)}><Pencil className="h-3.5 w-3.5" /></Button>
                      )}
                      {!readOnly && canEdit(r) && (
                        <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => setDeletingRow(r)}><Trash2 className="h-3.5 w-3.5" /></Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
            {!rows.length && <EmptyState colSpan={6} label="Belum ada materi." />}
          </TableBody>
        </Table>
      </Card>

      <AlertDialog open={!!deletingRow} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Materi?</AlertDialogTitle>
            <AlertDialogDescription>Materi <strong>{String(deletingRow?.judul || '')}</strong> beserta komentarnya akan dihapus.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Batal</AlertDialogCancel>
            <AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={confirmDelete} disabled={isDeleting}>
              {isDeleting ? 'Menghapus...' : 'Hapus'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={!!detail} onOpenChange={(open) => !open && setDetail(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2"><FileText className="h-5 w-5 text-primary" />{String(detail?.judul || '')}</DialogTitle>
            <DialogDescription>{String(detail?.deskripsi || '—')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              {detail?.filePath ? (
                <Button size="sm" variant="outline" onClick={() => detail && download(detail)}>
                  <Download className="h-3.5 w-3.5" /> Unduh {String(detail?.tipe || '')}
                </Button>
              ) : null}
              {detail?.linkUrl ? (
                <Button size="sm" variant="outline" onClick={() => window.open(String(detail?.linkUrl), '_blank', 'noopener,noreferrer')}>
                  <LinkIcon className="h-3.5 w-3.5" /> Buka Link
                </Button>
              ) : null}
              {detail?.tanggal ? (
                <span className="text-xs text-muted-foreground">Tanggal: {String(detail.tanggal).slice(0, 10)}</span>
              ) : null}
            </div>

            {!readOnly && (user.role === 'admin' || (detail && String(detail.dibuatOlehUserId || '') === user.id)) && (
              <div className="border-t border-border pt-3">
                <div className="flex items-center gap-2 text-sm font-semibold mb-2"><Share2 className="h-4 w-4" /> Bagikan ke Peserta Didik</div>
                <p className="text-xs text-muted-foreground mb-2">Buat link share (publik atau berpassword) yang bisa dibuka peserta didik tanpa login, seperti link Google Drive.</p>
                {share?.enabled && share.url ? (
                  <div className="space-y-2">
                    <div className="flex items-center gap-2">
                      <Input readOnly value={share.url} className="text-xs" />
                      <Button size="sm" variant="outline" onClick={copyShareUrl}>{shareCopied ? 'Tersalin' : <><Copy className="h-3.5 w-3.5" /> Salin</>}</Button>
                    </div>
                    <div className="text-xs text-muted-foreground flex items-center gap-1">
                      {share.protectedShare ? <><Lock className="h-3 w-3" /> Diproteksi password</> : 'Publik (siapa pun dengan link bisa akses)'}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Input type="password" value={sharePwd} onChange={(e) => setSharePwd(e.target.value)} placeholder="Password baru (kosongkan = publik)" className="text-xs" />
                      <Button size="sm" variant="outline" onClick={saveShare} disabled={shareSaving}>{shareSaving ? '...' : 'Perbarui'}</Button>
                      <Button size="sm" variant="destructive" onClick={disableShare} disabled={shareSaving}>Nonaktifkan</Button>
                    </div>
                  </div>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    <Input type="password" value={sharePwd} onChange={(e) => setSharePwd(e.target.value)} placeholder="Password (opsional — kosongkan untuk publik)" className="text-xs" />
                    <Button size="sm" onClick={saveShare} disabled={shareSaving}><Share2 className="h-3.5 w-3.5" /> {shareSaving ? '...' : 'Buat Link Share'}</Button>
                  </div>
                )}
              </div>
            )}

            <div className="border-t border-border pt-3">
              <div className="flex items-center gap-2 text-sm font-semibold mb-2"><MessageSquare className="h-4 w-4" /> Komentar Internal</div>
              <div className="space-y-2 max-h-48 overflow-y-auto">
                {komentar.map((k) => (
                  <div key={k.id} className="rounded-lg bg-secondary/50 px-3 py-2 text-sm">
                    <div className="text-xs text-muted-foreground">{String(k.userId || 'staf')} · {String(k.createdAt || '').slice(0, 16).replace('T', ' ')}</div>
                    <div>{String(k.isi || '')}</div>
                  </div>
                ))}
                {!komentar.length && <div className="text-sm text-muted-foreground">Belum ada komentar.</div>}
              </div>
              <div className="flex gap-2 mt-2">
                <Input value={newKomentar} onChange={(e) => setNewKomentar(e.target.value)} placeholder="Tulis komentar..." />
                <Button onClick={submitKomentar} disabled={!newKomentar.trim()}>Kirim</Button>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDetail(null)}>Tutup</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}