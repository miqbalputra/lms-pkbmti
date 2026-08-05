import { useCallback, useEffect, useState } from 'react'
import { Alert, AlertDescription } from './components/ui/alert'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Checkbox } from './components/ui/checkbox'
import { Label } from './components/ui/label'
import { PageToolbar } from './components/ui/page'
import { Select } from './components/ui/select'

type Row = Record<string, unknown> & { id: string }
const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

export function ClassSubjects({ token }: { token: string }) {
  const [classes, setClasses] = useState<Row[]>([])
  const [subjects, setSubjects] = useState<Row[]>([])
  const [links, setLinks] = useState<Row[]>([])
  const [selected, setSelected] = useState('')
  const [chosen, setChosen] = useState<string[]>([])
  const [message, setMessage] = useState('')

  const load = useCallback(() => {
    return Promise.all([request('/kelas', token), request('/mapel', token), request('/kelas-mapel', token)])
      .then(([c, s, l]) => {
        setClasses(c)
        setSubjects(s)
        setLinks(l)
      })
      .catch((e) => setMessage(String(e)))
  }, [token])

  useEffect(() => {
    void load()
  }, [load])

  function choose(id: string) {
    setSelected(id)
    setChosen(links.filter((l) => l.kelasId === id).map((l) => String(l.mapelId)))
    setMessage('')
  }

  function toggle(id: string) {
    setChosen((value) => (value.includes(id) ? value.filter((item) => item !== id) : [...value, id]))
  }

  async function save() {
    if (!selected) return
    try {
      await request('/kelas/' + selected + '/mapel', token, 'PUT', { mapelIds: chosen })
      setMessage('Mata pelajaran rombel berhasil disimpan.')
      void load()
    } catch (e) {
      setMessage(String(e))
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar title="Mata Pelajaran per Kelas" description="Tentukan kurikulum setiap rombel sebagai dasar penugasan guru." />
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        <Card>
          <CardHeader>
            <CardTitle>Pilih Rombel</CardTitle>
            <CardDescription>Pilih rombel untuk mengatur mata pelajaran yang berlaku.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-2">
              <Label>Rombel</Label>
              <Select value={selected} onChange={(e) => choose(e.target.value)}>
                <option value="">Pilih rombel</option>
                {classes.map((c) => (
                  <option key={c.id} value={c.id}>
                    Kelas {String(c.jenjang)}{String(c.namaRombel)} - {String((c.tahunAjaran as Row)?.namaTahunAjaran || '')}
                  </option>
                ))}
              </Select>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Pilih mata pelajaran</CardTitle>
            <CardDescription>{selected ? 'Centang semua mapel yang berlaku.' : 'Pilih rombel terlebih dahulu.'}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {selected && (
              <>
                <div className="grid gap-3 sm:grid-cols-2">
                  {subjects
                    .filter((s) => s.isActive !== false)
                    .map((s) => (
                      <Label key={s.id} className="flex items-center gap-3 rounded-lg border p-3 font-normal hover:bg-muted/50">
                        <Checkbox checked={chosen.includes(s.id)} onChange={() => toggle(s.id)} />
                        <span>
                          {String(s.namaMapel)}{' '}
                          {Boolean(s.kodeMapel) && <span className="text-muted-foreground">({String(s.kodeMapel)})</span>}
                        </span>
                      </Label>
                    ))}
                </div>
                <Button onClick={() => void save()}>Simpan mata pelajaran</Button>
              </>
            )}
            {message && (
              <Alert>
                <AlertDescription>{message}</AlertDescription>
              </Alert>
            )}
          </CardContent>
        </Card>
      </div>
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
