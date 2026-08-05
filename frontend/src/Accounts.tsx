import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Pencil, Plus, Power } from 'lucide-react'
import { Alert, AlertDescription } from './components/ui/alert'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { Card } from './components/ui/card'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { EmptyState, FormCard, PageToolbar } from './components/ui/page'
import { Select } from './components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './components/ui/table'

type Row = Record<string, unknown> & { id: string }
const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

const roleLabels: Record<string, string> = {
  admin: 'Admin',
  guru: 'Tutor',
  kepala_sekolah: 'Kepala Sekolah',
  orang_tua: 'Orang Tua',
}

export function Accounts({ token }: { token: string }) {
  const [users, setUsers] = useState<Row[]>([])
  const [tutors, setTutors] = useState<Row[]>([])
  const [orangTuaList, setOrangTuaList] = useState<Row[]>([])
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const load = useCallback(() =>
    Promise.all([
      request('/users', token),
      request('/tutor', token),
      request('/orang-tua', token).catch(() => []),
    ])
      .then(([u, t, o]) => {
        setUsers(u)
        setTutors(t)
        if (Array.isArray(o)) setOrangTuaList(o)
      })
      .catch((e) => setError(String(e))),
    [token]
  )

  useEffect(() => {
    void load()
  }, [load])

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setSubmitting(true)
    const form = Object.fromEntries(new FormData(e.currentTarget))
    const body: Record<string, unknown> = { ...form, isActive: form.isActive === 'true' }
    if (body.role !== 'guru') body.tutorId = null
    if (body.role !== 'orang_tua') body.orangTuaId = null
    if (!body.password) delete body.password
    try {
      await request('/users' + (editing ? '/' + editing.id : ''), token, editing ? 'PUT' : 'POST', body)
      setShowForm(false)
      setEditing(null)
      void load()
    } catch (err) {
      setError(String(err))
    } finally {
      setSubmitting(false)
    }
  }

  async function toggle(user: Row) {
    try {
      await request('/users/' + user.id, token, 'PUT', {
        username: user.username,
        email: user.email,
        role: user.role,
        tutorId: user.tutorId || null,
        orangTuaId: user.orangTuaId || null,
        isActive: !user.isActive,
      })
      void load()
    } catch (e) {
      setError(String(e))
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Manajemen Akun Pengguna"
        description={`${users.length} akun pengguna terdaftar dalam sistem.`}
        actions={
          <Button onClick={() => { setEditing(null); setShowForm(true) }}>
            <Plus className="h-4 w-4" /> Buat akun baru
          </Button>
        }
      />

      {error && (
        <Alert className="border-destructive/40 text-destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {showForm && (
        <FormCard
          title={editing ? 'Ubah akun' : 'Buat akun'}
          description={editing ? 'Kosongkan kata sandi bila tidak ingin menggantinya.' : 'Buat akun pengguna baru dengan peran yang sesuai.'}
        >
          <form className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" onSubmit={save}>
            <Field label="Username">
              <Input name="username" defaultValue={String(editing?.username || '')} required />
            </Field>
            <Field label="Email">
              <Input name="email" type="email" defaultValue={String(editing?.email || '')} required />
            </Field>
            <Field label={editing ? 'Kata sandi baru (opsional)' : 'Kata sandi'}>
              <Input name="password" type="password" minLength={8} required={!editing} />
            </Field>
            <Field label="Peran">
              <Select name="role" defaultValue={String(editing?.role || 'guru')}>
                <option value="guru">Tutor</option>
                <option value="admin">Admin</option>
                <option value="kepala_sekolah">Kepala Sekolah</option>
                <option value="orang_tua">Orang Tua</option>
              </Select>
            </Field>
            <Field label="Tutor">
              <Select name="tutorId" defaultValue={String(editing?.tutorId || '')}>
                <option value="">Pilih tutor</option>
                {tutors.map((t) => (
                  <option key={t.id} value={t.id}>{String(t.nama)}</option>
                ))}
              </Select>
            </Field>
            <Field label="Orang Tua">
              <Select name="orangTuaId" defaultValue={String(editing?.orangTuaId || '')}>
                <option value="">Pilih orang tua</option>
                {orangTuaList.map((o: Row) => (
                  <option key={o.id} value={o.id}>{String(o.namaIbu || o.namaBapak || o.id)}</option>
                ))}
              </Select>
            </Field>
            <Field label="Status">
              <Select name="isActive" defaultValue={String(editing?.isActive ?? true)}>
                <option value="true">Aktif</option>
                <option value="false">Nonaktif</option>
              </Select>
            </Field>
            <div className="flex gap-2 sm:col-span-2 lg:col-span-3">
              <Button disabled={submitting}>{submitting ? 'Menyimpan...' : 'Simpan akun'}</Button>
              <Button variant="outline" type="button" onClick={() => { setShowForm(false); setEditing(null) }}>Batal</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Username</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Peran</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Aksi</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5}>
                    <EmptyState title="Belum ada akun" description="Buat akun pengguna baru untuk memulai." />
                  </TableCell>
                </TableRow>
              ) : (
                users.map((u) => (
                  <TableRow key={u.id}>
                    <TableCell className="font-medium">{String(u.username)}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{String(u.email)}</TableCell>
                    <TableCell>
                      <Badge variant={u.role === 'admin' ? 'default' : 'secondary'}>
                        {roleLabels[String(u.role)] || String(u.role)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={u.isActive ? 'default' : 'destructive'}>
                        {u.isActive ? 'Aktif' : 'Nonaktif'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="ghost" size="sm" onClick={() => { setEditing(u); setShowForm(true) }}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => toggle(u)}>
                          <Power className={`h-3.5 w-3.5 ${u.isActive ? 'text-destructive' : 'text-green-600'}`} />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </Card>
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid gap-2">
      <Label>{label}</Label>
      {children}
    </div>
  )
}

async function request(path: string, token: string, method = 'GET', body?: unknown) {
  const r = await fetch(apiBase + path, {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: body ? JSON.stringify(body) : undefined,
  })
  const result = r.status === 204 ? null : await r.json().catch(() => ({}))
  if (!r.ok) throw new Error(result?.error || 'Permintaan gagal')
  return result
}
