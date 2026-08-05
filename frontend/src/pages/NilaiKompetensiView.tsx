import { useEffect, useMemo, useState } from 'react'
import { Save } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'

type Row = Record<string, unknown> & { id: string }

function kelasLabel(k: Row): string {
  return `Kelas ${String(k.jenjang ?? '')}${String(k.namaRombel ?? '')}`
}

export function NilaiKompetensiView({ token, user, readOnly }: { token: string; user: User; readOnly: boolean }) {
  const [kelas, setKelas] = useState<Row[]>([])
  const [kelasId, setKelasId] = useState('')
  const [semester, setSemester] = useState('Ganjil')
  const [siswa, setSiswa] = useState<Row[]>([])
  const [kompetensiAll, setKompetensiAll] = useState<Row[]>([])
  const [rombel, setRombel] = useState<Row[]>([])
  const [nilai, setNilai] = useState<Row[]>([])
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  useEffect(() => {
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
    void request('/kompetensi', token).then((r: Row[]) => setKompetensiAll(r || [])).catch(() => setKompetensiAll([]))
  }, [token])

  useEffect(() => {
    if (!kelasId) {
      setSiswa([]); setRombel([]); setNilai([]); setDraft({})
      return
    }
    void request('/peserta-didik?kelasId=' + kelasId, token).then((r: Row[]) => setSiswa(r || [])).catch(() => setSiswa([]))
    void request('/rombel-kompetensi?kelasId=' + kelasId, token).then((r: Row[]) => setRombel(r || [])).catch(() => setRombel([]))
  }, [kelasId, token]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!kelasId || (semester !== 'Ganjil' && semester !== 'Genap')) { setNilai([]); setDraft({}); return }
    void request('/nilai-kompetensi?kelasId=' + kelasId + '&semester=' + semester, token)
      .then((r: Row[]) => setNilai(r || []))
      .catch((e) => { setNilai([]); toast.error(String((e as Error).message || '')) })
  }, [kelasId, semester, token]) // eslint-disable-line react-hooks/exhaustive-deps

  // Kompetensi yang ditugaskan ke kelas ini.
  const assignedKompetensi = useMemo(() => {
    const ids = new Set(rombel.map((r) => String(r.kompetensiId || '')))
    return kompetensiAll.filter((k) => ids.has(k.id))
  }, [rombel, kompetensiAll])

  // Nilai keyed by "pesertaDidikId|kompetensiId".
  const nilaiMap = useMemo(() => {
    const m = new Map<string, number>()
    for (const n of nilai) {
      m.set(String(n.pesertaDidikId || '') + '|' + String(n.kompetensiId || ''), Number(n.nilai ?? 0))
    }
    return m
  }, [nilai])

  function valOf(pdId: string, kid: string): string {
    const key = pdId + '|' + kid
    if (draft[key] !== undefined) return draft[key]
    if (nilaiMap.has(key)) return String(nilaiMap.get(key))
    return ''
  }

  function setCell(pdId: string, kid: string, v: string) {
    setDraft((d) => ({ ...d, [pdId + '|' + kid]: v }))
  }

  const dirtyCount = Object.keys(draft).length

  async function saveAll() {
    if (!kelasId || (semester !== 'Ganjil' && semester !== 'Genap')) return
    const payload: { pesertaDidikId: string; kompetensiId: string; nilai: number }[] = []
    for (const [key, v] of Object.entries(draft)) {
      const [pdId, kid] = key.split('|')
      const num = Number(v)
      if (!pdId || !kid || v === '' || isNaN(num)) continue
      payload.push({ pesertaDidikId: pdId, kompetensiId: kid, nilai: num })
    }
    if (!payload.length) { toast.error('Tidak ada nilai yang diubah.'); return }
    setSaving(true)
    try {
      await request('/nilai-kompetensi', token, 'POST', { kelasId, semester, nilai: payload })
      toast.success(`${payload.length} nilai kompetensi disimpan.`)
      setDraft({})
      void request('/nilai-kompetensi?kelasId=' + kelasId + '&semester=' + semester, token)
        .then((r: Row[]) => setNilai(r || [])).catch((e: unknown) => console.warn('gagal memuat nilai kompetensi:', e))
    } catch (err: any) {
      toast.error(err.message || 'Gagal menyimpan nilai kompetensi.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Nilai Kompetensi"
        description="Matriks nilai kompetensi per peserta didik & semester. Kompetensi ditentukan dari penugasan rombel (menu Kompetensi)."
      />

      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs">
        <div className="grid gap-3 sm:grid-cols-2 sm:max-w-xl">
          <div className="grid gap-2">
            <Label>Kelas / Rombel</Label>
            <Select value={kelasId} onChange={(e) => { setKelasId(e.target.value); setDraft({}) }}>
              <option value="">Pilih kelas...</option>
              {kelasOptions.map((k) => (
                <option key={k.id} value={k.id}>{kelasLabel(k)}</option>
              ))}
            </Select>
            {isGuru && !kelasOptions.length && (
              <p className="text-xs text-muted-foreground">Anda belum ditugaskan sebagai wali kelas.</p>
            )}
          </div>
          <div className="grid gap-2">
            <Label>Semester</Label>
            <Select value={semester} onChange={(e) => { setSemester(e.target.value); setDraft({}) }}>
              <option value="Ganjil">Ganjil</option>
              <option value="Genap">Genap</option>
            </Select>
          </div>
        </div>
      </Card>

      {kelasId && (
        <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
          {!assignedKompetensi.length ? (
            <div className="p-6 text-sm text-muted-foreground">
              Belum ada kompetensi yang ditugaskan ke kelas ini. Tugaskan via menu <strong>Kompetensi</strong>.
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow className="border-b border-border">
                    <TableHead className=" sticky left-0">Peserta Didik</TableHead>
                    {assignedKompetensi.map((k) => (
                      <TableHead key={k.id} className=" text-center min-w-[90px]">
                        {String(k.nama || '')}
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {siswa.map((s) => (
                    <TableRow key={s.id}>
                      <TableCell className="font-medium sticky left-0 bg-card">
                        {String(s.nama || '-')}
                        <div className="text-xs text-muted-foreground font-normal">{String(s.nisn || '-')}</div>
                      </TableCell>
                      {assignedKompetensi.map((k) => (
                        <TableCell key={k.id} className="text-center">
                          <Input
                            type="number"
                            min={0}
                            max={100}
                            value={valOf(s.id, k.id)}
                            disabled={readOnly}
                            onChange={(e) => setCell(s.id, k.id, e.target.value)}
                            className="h-9 w-20 mx-auto text-center"
                          />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))}
                  {!siswa.length && (
                    <TableRow>
                      <TableCell colSpan={assignedKompetensi.length + 1} className="text-sm text-muted-foreground text-center py-6">
                        Belum ada peserta didik di kelas ini.
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
              {!readOnly && (
                <div className="flex items-center justify-between gap-3 border-t border-border px-4 py-3">
                  <span className="text-xs text-muted-foreground">
                    {dirtyCount > 0 ? `${dirtyCount} sel berubah — belum disimpan.` : 'Tidak ada perubahan.'}
                  </span>
                  <Button onClick={saveAll} disabled={saving || !dirtyCount}>
                    <Save className="h-4 w-4" /> {saving ? 'Menyimpan...' : 'Simpan nilai'}
                  </Button>
                </div>
              )}
            </>
          )}
        </Card>
      )}
    </div>
  )
}