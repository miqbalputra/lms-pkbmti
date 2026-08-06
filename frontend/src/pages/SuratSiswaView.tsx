import { useEffect, useMemo, useState } from 'react'
import { FileArchive, FileText, RefreshCw, Trash2, Upload } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '../components/ui/alert'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select } from '../components/ui/select'
import { PageToolbar } from '../components/ui/page'
import { apiBase, request } from '../lib/api'

type Row = Record<string, unknown> & { id: string }
type SuratRow = {
  id: string
  judul: string
  cakupan: string
  kelasLabel?: string
  fileCount: number
  createdAt: string
}

function classLabel(row: Row) {
  return `Kelas ${String(row.jenjang || '')}${String(row.namaRombel || '')} - ${String((row.tahunAjaran as Row | undefined)?.namaTahunAjaran || '-')}`
}

function scopeLabel(row: SuratRow) {
  if (row.cakupan === 'semua_kelas') return 'Semua siswa aktif'
  if (row.cakupan === 'kelas') return row.kelasLabel || 'Kelas tertentu'
  return 'Anak tertentu'
}

async function readError(response: Response) {
  const body = await response.json().catch(() => ({})) as { error?: string }
  return body.error || 'Permintaan gagal'
}

export function SuratSiswaView({ token }: { token: string }) {
  const [classes, setClasses] = useState<Row[]>([])
  const [students, setStudents] = useState<Row[]>([])
  const [documents, setDocuments] = useState<SuratRow[]>([])
  const [judul, setJudul] = useState('')
  const [cakupan, setCakupan] = useState('kelas')
  const [kelasId, setKelasId] = useState('')
  const [selectedIDs, setSelectedIDs] = useState<string[]>([])
  const [search, setSearch] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  async function load() {
    setLoading(true)
    try {
      const [classRows, studentRows, documentRows] = await Promise.all([
        request('/kelas', token),
        request('/peserta-didik', token),
        request('/surat-siswa', token),
      ])
      setClasses((classRows as Row[]) || [])
      setStudents((studentRows as Row[]) || [])
      setDocuments((documentRows as SuratRow[]) || [])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Gagal memuat data surat.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  const filteredStudents = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return students
    return students.filter((row) => `${String(row.nama || '')} ${String(row.nisn || '')}`.toLowerCase().includes(q))
  }, [students, search])

  function toggleStudent(id: string) {
    setSelectedIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id])
  }

  async function upload() {
    if (!judul.trim()) {
      toast.error('Judul surat wajib diisi.')
      return
    }
    if (!file || !file.name.toLowerCase().endsWith('.zip')) {
      toast.error('Pilih file ZIP berisi PDF.')
      return
    }
    if (cakupan === 'kelas' && !kelasId) {
      toast.error('Pilih kelas tujuan.')
      return
    }
    if (cakupan === 'anak' && selectedIDs.length === 0) {
      toast.error('Pilih minimal satu anak.')
      return
    }
    setBusy(true)
    try {
      const data = new FormData()
      data.append('judul', judul.trim())
      data.append('cakupan', cakupan)
      data.append('kelasId', kelasId)
      data.append('pesertaDidikIds', JSON.stringify(selectedIDs))
      data.append('file', file)
      const response = await fetch(`${apiBase}/surat-siswa`, {
        method: 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      if (!response.ok) throw new Error(await readError(response))
      const result = await response.json() as { uploadedCount: number }
      toast.success(`${result.uploadedCount} surat berhasil dipublikasikan.`)
      setJudul('')
      setFile(null)
      setSelectedIDs([])
      const input = document.getElementById('surat-zip-input') as HTMLInputElement | null
      if (input) input.value = ''
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Upload surat gagal.')
    } finally {
      setBusy(false)
    }
  }

  async function removeDocument(row: SuratRow) {
    if (!window.confirm(`Hapus publikasi "${row.judul}" dan semua file di portal orang tua?`)) return
    try {
      await request(`/surat-siswa/${encodeURIComponent(row.id)}`, token, 'DELETE')
      toast.success('Publikasi surat dihapus.')
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Surat gagal dihapus.')
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Surat untuk Peserta Didik"
        description="Upload ZIP berisi PDF bernama NISN.pdf. Sistem akan menghubungkan setiap file ke anak yang tepat."
        actions={<Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}><RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />Muat ulang</Button>}
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><Upload className="h-5 w-5 text-brand-500" />Upload Surat Baru</CardTitle>
          <CardDescription>Contoh isi ZIP: <code>0012345678.pdf</code>, <code>0012345679.pdf</code>, dan seterusnya.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2"><Label>Judul surat</Label><Input value={judul} onChange={(event) => setJudul(event.target.value)} placeholder="Surat Keterangan Berkelakuan Baik" /></div>
            <div className="grid gap-2"><Label>Dikirim kepada</Label><Select value={cakupan} onChange={(event) => { setCakupan(event.target.value); setSelectedIDs([]) }}><option value="semua_kelas">Semua siswa aktif (semua kelas)</option><option value="kelas">Kelas tertentu</option><option value="anak">Anak tertentu</option></Select></div>
          </div>

          {cakupan === 'kelas' && <div className="grid gap-2 md:max-w-md"><Label>Kelas tujuan</Label><Select value={kelasId} onChange={(event) => setKelasId(event.target.value)}><option value="">Pilih kelas</option>{classes.map((row) => <option key={row.id} value={row.id}>{classLabel(row)}</option>)}</Select></div>}

          {cakupan === 'anak' && <div className="space-y-3 rounded-xl border border-border/70 p-3"><div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between"><Label>Pilih anak ({selectedIDs.length} dipilih)</Label><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Cari nama atau NISN" className="sm:max-w-xs" /></div><div className="grid max-h-56 gap-2 overflow-auto sm:grid-cols-2">{filteredStudents.map((row) => <label key={row.id} className="flex cursor-pointer items-center gap-2 rounded-lg border border-border/60 p-2 text-sm"><input type="checkbox" checked={selectedIDs.includes(row.id)} onChange={() => toggleStudent(row.id)} /><span><span className="block font-medium">{String(row.nama)}</span><span className="text-xs text-muted-foreground">NISN: {String(row.nisn || '-')}</span></span></label>)}{filteredStudents.length === 0 && <div className="py-4 text-sm text-muted-foreground">Anak tidak ditemukan.</div>}</div></div>}

          <div className="flex flex-col gap-3 sm:flex-row sm:items-center"><Input id="surat-zip-input" type="file" accept=".zip,application/zip" onChange={(event) => setFile(event.currentTarget.files?.[0] || null)} className="sm:max-w-md" /><Button onClick={() => void upload()} disabled={busy}>{busy ? 'Memvalidasi dan mengunggah...' : 'Upload ZIP'}</Button></div>
          <Alert><AlertDescription>Upload hanya diterbitkan jika semua target memiliki PDF dan semua nama file cocok dengan NISN target. Maksimal ZIP 256 MB dan PDF 20 MB per anak.</AlertDescription></Alert>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Riwayat Publikasi</CardTitle><CardDescription>Surat yang sudah tersedia di portal orang tua.</CardDescription></CardHeader>
        <CardContent className="space-y-2">
          {loading && <div className="py-8 text-center text-sm text-muted-foreground">Memuat surat...</div>}
          {!loading && documents.map((row) => <div key={row.id} className="flex flex-col gap-3 rounded-xl border border-border/70 p-3 sm:flex-row sm:items-center"><div className="flex min-w-0 flex-1 items-start gap-3"><FileText className="mt-0.5 h-5 w-5 shrink-0 text-brand-500" /><div className="min-w-0"><div className="truncate font-semibold">{row.judul}</div><div className="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground"><Badge variant="secondary">{scopeLabel(row)}</Badge><span>{row.fileCount} file</span><span>{new Date(row.createdAt).toLocaleDateString('id-ID')}</span></div></div></div><Button variant="outline" size="sm" onClick={() => void removeDocument(row)}><Trash2 className="h-4 w-4" />Hapus</Button></div>)}
          {!loading && documents.length === 0 && <div className="py-8 text-center text-sm text-muted-foreground">Belum ada surat yang dipublikasikan.</div>}
        </CardContent>
      </Card>

      <Card className="border-dashed"><CardContent className="flex gap-3 py-4 text-sm text-muted-foreground"><FileArchive className="mt-0.5 h-4 w-4 shrink-0" /><span>PDF tidak dibuka sebagai file publik. Orang tua hanya dapat mengunduh file milik anak yang terhubung ke akunnya.</span></CardContent></Card>
    </div>
  )
}
