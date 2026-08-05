import { useEffect, useState } from 'react'
import { FileDown, FileSpreadsheet, FileText } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Label } from '../components/ui/label'
import { PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { request } from '../lib/api'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }

type Filter = {
  key: string
  label: string
  type: 'kelas' | 'mapel' | 'tahunAjaran' | 'pokjar' | 'select' | 'text'
  required: boolean
  options?: string[]
}
type Kind = {
  jenis: string
  nama: string
  formats: string[]
  filters: Filter[]
}

export function LaporanView({ token }: { token: string }) {
  const [kinds, setKinds] = useState<Kind[]>([])
  const [kelas, setKelas] = useState<Row[]>([])
  const [mapel, setMapel] = useState<Row[]>([])
  const [tahun, setTahun] = useState<Row[]>([])
  const [pokjar, setPokjar] = useState<Row[]>([])
  const [selectedJenis, setSelectedJenis] = useState('')
  const [values, setValues] = useState<Record<string, string>>({})
  const [format, setFormat] = useState('xlsx')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    void request('/laporan/jenis', token).then((r: Kind[]) => setKinds(r || [])).catch(() => setKinds([]))
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
    void request('/mapel', token).then((r: Row[]) => setMapel(r || [])).catch(() => setMapel([]))
    void request('/tahun-ajaran', token).then((r: Row[]) => setTahun(r || [])).catch(() => setTahun([]))
    void request('/pokjar', token).then((r: Row[]) => setPokjar(r || [])).catch(() => setPokjar([]))
  }, [token])

  const kind = kinds.find((k) => k.jenis === selectedJenis)

  function selectJenis(j: string) {
    setSelectedJenis(j)
    const k = kinds.find((x) => x.jenis === j)
    setFormat(k && k.formats.includes('xlsx') ? 'xlsx' : (k?.formats[0] || 'xlsx'))
    setValues({})
  }

  async function exportNow() {
    if (!kind) return
    const params = new URLSearchParams()
    params.set('jenis', kind.jenis)
    params.set('format', format)
    for (const f of kind.filters) {
      const v = values[f.key] || ''
      if (f.required && !v) {
        toast.error(`${f.label} wajib diisi.`)
        return
      }
      if (v) params.set(f.key, v)
    }
    setLoading(true)
    try {
      const res = await fetch(apiBase + '/laporan/export?' + params.toString(), {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const x = await res.json().catch(() => ({}))
        throw new Error((x as any)?.error || `Permintaan gagal (${res.status}).`)
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      const ext = format === 'pdf' ? 'pdf' : 'xlsx'
      a.download = `laporan-${kind.jenis}.${ext}`
      a.click()
      URL.revokeObjectURL(url)
      toast.success('Laporan diunduh.')
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengunduh laporan.')
    } finally {
      setLoading(false)
    }
  }

  function renderFilter(f: Filter) {
    const common = {
      value: values[f.key] || '',
      onChange: (e: React.ChangeEvent<HTMLSelectElement | HTMLInputElement>) =>
        setValues((v) => ({ ...v, [f.key]: e.target.value })),
    }
    if (f.type === 'select') {
      return (
        <Select {...(common as any)}>
          <option value="">{f.required ? `Pilih ${f.label}` : '— Semua —'}</option>
          {(f.options || []).map((o) => (
            <option key={o} value={o}>{o}</option>
          ))}
        </Select>
      )
    }
    if (f.type === 'kelas') {
      return (
        <Select {...(common as any)}>
          <option value="">{f.required ? 'Pilih kelas...' : '— Semua kelas —'}</option>
          {kelas.map((k) => (
            <option key={k.id} value={k.id}>Kelas {String(k.jenjang ?? '')}{String(k.namaRombel ?? '')}</option>
          ))}
        </Select>
      )
    }
    if (f.type === 'mapel') {
      return (
        <Select {...(common as any)}>
          <option value="">{f.required ? 'Pilih mapel...' : '— Semua mapel —'}</option>
          {mapel.map((m) => (
            <option key={m.id} value={m.id}>{String(m.namaMapel || '-')}</option>
          ))}
        </Select>
      )
    }
    if (f.type === 'tahunAjaran') {
      return (
        <Select {...(common as any)}>
          <option value="">{f.required ? 'Pilih tahun ajaran...' : '— Semua tahun —'}</option>
          {tahun.map((t) => (
            <option key={t.id} value={t.id}>{String(t.namaTahunAjaran || '-')}</option>
          ))}
        </Select>
      )
    }
    if (f.type === 'pokjar') {
      return (
        <Select {...(common as any)}>
          <option value="">{f.required ? 'Pilih pokjar...' : '— Semua pokjar —'}</option>
          {pokjar.map((p) => (
            <option key={p.id} value={p.id}>{String(p.namaPokjar || '-')}</option>
          ))}
        </Select>
      )
    }
    return <input {...(common as any)} className="flex h-9 w-full rounded-xl border border-border bg-background px-3 py-1 text-sm" placeholder={f.label} />
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Pusat Laporan"
        description="Agregator ekspor laporan (nilai, presensi, peminjaman buku, peserta didik per pokjar). Pilih jenis → isi filter → unduh."
      />

      <Card className="rounded-2xl border border-border bg-card p-5 shadow-2xs space-y-4">
        <div className="grid gap-2 sm:max-w-md">
          <Label>Jenis Laporan</Label>
          <Select value={selectedJenis} onChange={(e) => selectJenis(e.target.value)}>
            <option value="">Pilih jenis laporan...</option>
            {kinds.map((k) => (
              <option key={k.jenis} value={k.jenis}>{k.nama}</option>
            ))}
          </Select>
        </div>

        {kind && (
          <>
            <div className="grid gap-4 sm:grid-cols-2">
              {kind.filters.map((f) => (
                <div key={f.key} className="grid gap-2">
                  <Label>{f.label}{f.required ? ' *' : ''}</Label>
                  {renderFilter(f)}
                </div>
              ))}
            </div>

            <div className="grid gap-2 sm:max-w-xs">
              <Label>Format</Label>
              <div className="flex gap-2">
                {kind.formats.map((fm) => (
                  <Button
                    key={fm}
                    type="button"
                    variant={format === fm ? 'default' : 'outline'}
                    size="sm"
                    onClick={() => setFormat(fm)}
                  >
                    {fm === 'pdf' ? <FileText className="h-4 w-4" /> : <FileSpreadsheet className="h-4 w-4" />}
                    {fm.toUpperCase()}
                  </Button>
                ))}
              </div>
            </div>

            <div className="pt-2">
              <Button onClick={exportNow} disabled={loading}>
                <FileDown className="h-4 w-4" /> {loading ? 'Menyiapkan...' : 'Unduh Laporan'}
              </Button>
            </div>
          </>
        )}
      </Card>
    </div>
  )
}