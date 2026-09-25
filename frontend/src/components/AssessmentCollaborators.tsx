import { useEffect, useState } from 'react'
import { UserPlus, UsersRound } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from './ui/dialog'
import { request } from '../lib/api'

type Collaborator = { id: string; userId: string; username: string; email?: string; peran: string; aktif: boolean }

export function AssessmentCollaboratorManager({ token, module, assessmentId, onRoleChange }: { token: string; module: 'ujian_online' | 'simulasi'; assessmentId: string; onRoleChange?: (role: string) => void }) {
  const [open, setOpen] = useState(false)
  const [rows, setRows] = useState<Collaborator[]>([])
  const [canManage, setCanManage] = useState(false)
  const [identity, setIdentity] = useState('')
  const [peran, setPeran] = useState('editor')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const path = `/assessment/${module}/${encodeURIComponent(assessmentId)}/collaborators`

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await request(path, token)
      setRows(Array.isArray(data?.items) ? data.items : [])
      setCanManage(Boolean(data?.canManage))
      onRoleChange?.(String(data?.currentRole || ''))
    } catch (cause: any) {
      setError(cause?.message || 'Daftar kolaborator tidak dapat dimuat.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { if (open) void load() }, [open, token, path]) // eslint-disable-line react-hooks/exhaustive-deps

  async function add() {
    const value = identity.trim()
    if (!value) { setError('Masukkan username atau email akun tutor.'); return }
    setSaving(true)
    setError('')
    try {
      await request(path, token, 'POST', value.includes('@') ? { email: value, peran } : { username: value, peran })
      setIdentity('')
      await load()
      toast.success('Kolaborator berhasil ditambahkan.')
    } catch (cause: any) {
      setError(cause?.message || 'Kolaborator gagal ditambahkan.')
    } finally {
      setSaving(false)
    }
  }

  async function remove(row: Collaborator) {
    setSaving(true)
    try {
      await request(`${path}/${encodeURIComponent(row.id)}`, token, 'DELETE')
      setRows((current) => current.filter((item) => item.id !== row.id))
      toast.success(`Akses ${row.username} dicabut.`)
    } catch (cause: any) {
      setError(cause?.message || 'Akses kolaborator gagal dicabut.')
    } finally {
      setSaving(false)
    }
  }

  return <>
    <Button type="button" size="sm" variant="outline" className="min-h-11" onClick={() => setOpen(true)}><UsersRound className="h-4 w-4" />Kolaborator</Button>
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Kolaborator asesmen</DialogTitle>
          <DialogDescription>Berikan akses terbatas kepada tutor aktif. Peran editor dapat menyunting, grader dapat menilai, dan viewer hanya dapat membaca.</DialogDescription>
        </DialogHeader>
        {loading ? <p className="py-5 text-sm text-muted-foreground" role="status">Memuat kolaborator…</p> : <div className="space-y-4">
          {error && <p className="rounded-lg bg-red-50 p-3 text-sm text-red-700" role="alert">{error}</p>}
          {canManage && <div className="grid gap-2 rounded-xl border bg-muted/20 p-3 sm:grid-cols-[minmax(0,1fr)_150px_auto] sm:items-end">
            <label className="grid gap-1 text-sm font-medium">Username atau email tutor<Input className="min-h-11" autoComplete="off" value={identity} onChange={(event) => setIdentity(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); void add() } }} placeholder="contoh: tutor1 atau tutor@sekolah.id" /></label>
            <label className="grid gap-1 text-sm font-medium">Hak akses<select className="min-h-11 rounded-lg border border-input bg-background px-3 text-sm" value={peran} onChange={(event) => setPeran(event.target.value)}><option value="editor">Editor</option><option value="grader">Penilai</option><option value="viewer">Pembaca</option></select></label>
            <Button type="button" className="min-h-11" disabled={saving} onClick={() => void add()}><UserPlus className="h-4 w-4" />Tambah</Button>
          </div>}
          {!canManage && <p className="rounded-lg bg-muted/40 p-3 text-sm text-muted-foreground">Kamu dapat melihat kolaborator, tetapi hanya pemilik asesmen yang dapat mengubah akses.</p>}
          <ul className="space-y-2" aria-label="Daftar kolaborator">
            {rows.map((row) => <li key={row.id} className="flex flex-wrap items-center justify-between gap-3 rounded-xl border p-3">
              <div className="min-w-0"><p className="truncate font-medium">{row.username}</p><p className="truncate text-xs text-muted-foreground">{row.email || 'Email tidak tersedia'} · {row.aktif ? 'Akun aktif' : 'Akun nonaktif'}</p></div>
              <div className="flex items-center gap-2"><span className="rounded-full bg-muted px-2.5 py-1 text-xs font-medium">{row.peran === 'grader' ? 'Penilai' : row.peran === 'viewer' ? 'Pembaca' : 'Editor'}</span>{canManage && <Button type="button" size="sm" variant="outline" className="min-h-11" disabled={saving} onClick={() => void remove(row)}>Cabut akses</Button>}</div>
            </li>)}
            {!rows.length && !loading && <li className="rounded-xl border border-dashed p-5 text-center text-sm text-muted-foreground">Belum ada kolaborator.</li>}
          </ul>
        </div>}
      </DialogContent>
    </Dialog>
  </>
}
