import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from './components/ui/alert'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { PageToolbar } from './components/ui/page'
import { Select } from './components/ui/select'

type Mapel = { id: string; namaMapel: string; kodeMapel: string }
type Ambang = { predikat: string; nilaiMinimum: number }
type Settings = {
  mapelId: string
  bobotKeterampilan: number
  bobotPengetahuan: number
  ambang: Ambang[]
}

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
const PREDIKAT: string[] = ['A', 'B', 'C']

export function PengaturanNilai({ token }: { token: string }) {
  const [mapels, setMapels] = useState<Mapel[]>([])
  const [mapelId, setMapelId] = useState('')
  const [bobotK, setBobotK] = useState(60)
  const [bobotP, setBobotP] = useState(40)
  const [ambangs, setAmbangs] = useState<Record<string, number>>({ A: 90, B: 78, C: 70 })
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')

  useEffect(() => {
    void request('/mapel', token).then((rows: Mapel[]) => setMapels(rows || [])).catch((e: unknown) => console.warn('gagal memuat mapel:', e))
  }, [token])

  const loadSettings = useCallback(async (id: string) => {
    if (!id) return
    setLoading(true)
    setMessage('')
    try {
      const data: Settings = await request('/settings/nilai?mapelId=' + id, token)
      setBobotK(data.bobotKeterampilan)
      setBobotP(data.bobotPengetahuan)
      const next: Record<string, number> = { A: 90, B: 78, C: 70 }
      for (const a of data.ambang || []) {
        if (a.predikat === 'A' || a.predikat === 'B' || a.predikat === 'C') next[a.predikat] = a.nilaiMinimum
      }
      setAmbangs(next)
    } catch (e: any) {
      setMessage(String(e.message || e))
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    if (mapelId) void loadSettings(mapelId)
  }, [mapelId, loadSettings])

  const bobotValid = Number(bobotK) + Number(bobotP) === 100

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!mapelId) {
      toast.error('Pilih mata pelajaran terlebih dahulu.')
      return
    }
    if (!bobotValid) {
      toast.error('Total bobot keterampilan + pengetahuan harus 100.')
      return
    }
    setSaving(true)
    setMessage('')
    try {
      await request('/settings/nilai', token, 'PUT', {
        mapelId,
        bobotKeterampilan: Number(bobotK),
        bobotPengetahuan: Number(bobotP),
        ambang: PREDIKAT.map((p) => ({ predikat: p, nilaiMinimum: Number(ambangs[p]) })),
      })
      toast.success('Pengaturan bobot & ambang predikat berhasil disimpan.')
      setMessage('Pengaturan berhasil disimpan.')
    } catch (e: any) {
      const err = String(e.message || e)
      setMessage(err)
      toast.error(`Gagal menyimpan: ${err}`)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Pengaturan Nilai"
        description="Atur bobot keterampilan/pengetahuan dan ambang predikat (A/B/C) per mata pelajaran. Perubahan tidak mengubah snapshot tema yang sudah ada."
      />

      <Card className="max-w-3xl">
        <CardHeader>
          <CardTitle>Konfigurasi Bobot & Ambang Predikat</CardTitle>
          <CardDescription>Pilih mata pelajaran, lalu sesuaikan bobot dan batas minimum predikat.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid gap-2">
            <Label className="text-xs font-bold">Mata Pelajaran</Label>
            <Select value={mapelId} onChange={(e) => setMapelId(e.target.value)}>
              <option value="">Pilih mata pelajaran</option>
              {mapels.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.namaMapel}{m.kodeMapel ? ` (${m.kodeMapel})` : ''}
                </option>
              ))}
            </Select>
          </div>

          {mapelId && (
            <form className="space-y-5" onSubmit={save}>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="grid gap-2">
                  <Label>Bobot Keterampilan (%)</Label>
                  <Input
                    type="number"
                    min={0}
                    max={100}
                    value={bobotK}
                    disabled={loading}
                    onChange={(e) => setBobotK(Number(e.target.value))}
                    required
                  />
                </div>
                <div className="grid gap-2">
                  <Label>Bobot Pengetahuan (%)</Label>
                  <Input
                    type="number"
                    min={0}
                    max={100}
                    value={bobotP}
                    disabled={loading}
                    onChange={(e) => setBobotP(Number(e.target.value))}
                    required
                  />
                </div>
              </div>

              {!bobotValid && (
                <Alert variant="destructive">
                  <AlertDescription>
                    Total bobot ({Number(bobotK) + Number(bobotP)}) harus sama dengan 100.
                  </AlertDescription>
                </Alert>
              )}

              <div className="grid gap-4 sm:grid-cols-3">
                {PREDIKAT.map((p) => (
                  <div key={p} className="grid gap-2">
                    <Label>Nilai Minimum Predikat {p}</Label>
                    <Input
                      type="number"
                      min={0}
                      max={100}
                      step="0.01"
                      value={ambangs[p]}
                      disabled={loading}
                      onChange={(e) => setAmbangs({ ...ambangs, [p]: Number(e.target.value) })}
                      required
                    />
                  </div>
                ))}
              </div>

              <div className="flex items-center gap-3">
                <Button type="submit" disabled={saving || loading || !bobotValid}>
                  {saving ? 'Menyimpan...' : 'Simpan pengaturan'}
                </Button>
                {message && <span className="text-xs text-muted-foreground font-medium">{message}</span>}
              </div>
            </form>
          )}

          {!mapelId && (
            <p className="text-xs text-muted-foreground">Pilih mata pelajaran untuk memuat konfigurasi saat ini.</p>
          )}
        </CardContent>
      </Card>
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
  if (!r.ok) throw new Error((result as any)?.error || 'Permintaan gagal')
  return result
}
