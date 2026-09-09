import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, CheckCircle2, FileArchive, FileText, Image, RefreshCw, Upload } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '../components/ui/alert'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { apiBase, request } from '../lib/api'

type ClassRow = {
  id: string
  jenjang?: number | string
  namaRombel?: string
  tahunAjaran?: { namaTahunAjaran?: string }
}

type StudentRow = {
  id: string
  nama: string
  nisn: string
  status?: string
  identitasFileExt?: string | null
}

type UploadResult = {
  kelasId: string
  uploadedCount: number
  replacedCount: number
  unmatchedNISNs: string[]
  skippedFiles: Array<{ fileName: string; nisn?: string; reason: string }>
}

function classLabel(row: ClassRow) {
  const schoolYear = row.tahunAjaran?.namaTahunAjaran
  return `Kelas ${String(row.jenjang ?? '')}${String(row.namaRombel ?? '')}${schoolYear ? ` — ${schoolYear}` : ''}`
}

function extensionLabel(ext?: string | null) {
  return ext ? ext.toUpperCase() : 'Belum tersedia'
}

async function readError(response: Response) {
  const body = await response.json().catch(() => ({})) as { error?: string }
  return body.error || 'Unggahan identitas gagal.'
}

export function IdentitasSiswaView({ token }: { token: string }) {
  const [classes, setClasses] = useState<ClassRow[]>([])
  const [students, setStudents] = useState<StudentRow[]>([])
  const [kelasId, setKelasId] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadingStudents, setLoadingStudents] = useState(false)
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<UploadResult | null>(null)

  async function loadClasses() {
    setLoading(true)
    try {
      const rows = await request('/kelas', token) as ClassRow[]
      setClasses(rows || [])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Gagal memuat kelas.')
    } finally {
      setLoading(false)
    }
  }

  async function loadStudents(classID: string) {
    if (!classID) {
      setStudents([])
      return
    }
    setLoadingStudents(true)
    try {
      const rows = await request(`/peserta-didik?kelasId=${encodeURIComponent(classID)}`, token) as StudentRow[]
      setStudents(rows || [])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Gagal memuat siswa.')
      setStudents([])
    } finally {
      setLoadingStudents(false)
    }
  }

  useEffect(() => {
    void loadClasses()
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    void loadStudents(kelasId)
  }, [kelasId, token]) // eslint-disable-line react-hooks/exhaustive-deps

  const availableCount = useMemo(() => students.filter((student) => Boolean(student.identitasFileExt)).length, [students])

  async function upload() {
    if (!kelasId) {
      toast.error('Pilih kelas terlebih dahulu.')
      return
    }
    if (!file || !file.name.toLowerCase().endsWith('.zip')) {
      toast.error('Pilih file ZIP terlebih dahulu.')
      return
    }
    setBusy(true)
    setResult(null)
    try {
      const data = new FormData()
      data.append('kelasId', kelasId)
      data.append('file', file)
      const response = await fetch(`${apiBase}/identitas-siswa/zip`, {
        method: 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      if (!response.ok) throw new Error(await readError(response))
      const payload = await response.json() as UploadResult
      setResult(payload)
      toast.success(`${payload.uploadedCount} file identitas berhasil disimpan.`)
      setFile(null)
      const input = document.getElementById('identitas-siswa-zip') as HTMLInputElement | null
      if (input) input.value = ''
      await loadStudents(kelasId)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Unggahan identitas gagal.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Identitas Siswa"
        description="Unggah foto atau dokumen identitas privat untuk satu kelas melalui ZIP."
        actions={<Button variant="outline" size="sm" onClick={() => void loadClasses()} disabled={loading}><RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />Muat ulang</Button>}
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><Upload className="h-5 w-5 text-brand-500" />Unggah ZIP Identitas</CardTitle>
          <CardDescription>Setiap file harus bernama NISN 10 digit, misalnya <code>0012345678.jpg</code> atau <code>0012345678.pdf</code>.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2 md:max-w-xl">
            <Label>Kelas / Rombel</Label>
            <Select value={kelasId} onChange={(event) => { setKelasId(event.target.value); setResult(null) }} disabled={loading || busy}>
              <option value="">Pilih kelas</option>
              {classes.map((row) => <option key={row.id} value={row.id}>{classLabel(row)}</option>)}
            </Select>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <Input id="identitas-siswa-zip" type="file" accept=".zip,application/zip" disabled={busy} onChange={(event) => setFile(event.currentTarget.files?.[0] || null)} className="sm:max-w-md" />
            <Button onClick={() => void upload()} disabled={busy || loading}>{busy ? 'Memvalidasi dan menyimpan...' : 'Upload ZIP'}</Button>
          </div>
          <Alert>
            <FileArchive className="h-4 w-4" />
            <AlertDescription>Hanya JPG, JPEG, PNG, dan PDF yang diizinkan. ZIP maksimal 256 MB; setiap file maksimal 20 MB. NISN yang tidak ada di kelas ini akan dilaporkan tanpa membatalkan file valid lain.</AlertDescription>
          </Alert>
        </CardContent>
      </Card>

      {result && (
        <Card>
          <CardHeader><CardTitle className="flex items-center gap-2"><CheckCircle2 className="h-5 w-5 text-emerald-600" />Ringkasan unggahan</CardTitle></CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex flex-wrap gap-2"><Badge>{result.uploadedCount} disimpan</Badge><Badge variant="secondary">{result.replacedCount} mengganti file lama</Badge><Badge variant={result.unmatchedNISNs.length ? 'destructive' : 'secondary'}>{result.unmatchedNISNs.length} NISN tidak cocok</Badge></div>
            {result.unmatchedNISNs.length > 0 && <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-amber-950"><div className="mb-1 flex items-center gap-2 font-medium"><AlertTriangle className="h-4 w-4" />NISN tidak ditemukan di kelas terpilih</div><div className="break-words">{result.unmatchedNISNs.join(', ')}</div></div>}
            {result.skippedFiles.length > 0 && <div className="space-y-1 rounded-lg border border-border/70 p-3"><div className="font-medium">File yang dilewati</div>{result.skippedFiles.map((item) => <div key={`${item.fileName}-${item.reason}`} className="text-muted-foreground"><code>{item.fileName}</code> — {item.reason}</div>)}</div>}
          </CardContent>
        </Card>
      )}

      <Card className="overflow-hidden">
        <CardHeader>
          <CardTitle>Status Identitas Siswa</CardTitle>
          <CardDescription>{kelasId ? `${availableCount} dari ${students.length} siswa sudah memiliki dokumen identitas.` : 'Pilih kelas untuk melihat status dokumen.'}</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader><TableRow><TableHead>Nama</TableHead><TableHead>NISN</TableHead><TableHead>Status Siswa</TableHead><TableHead>Dokumen Identitas</TableHead></TableRow></TableHeader>
            <TableBody>
              {students.map((student) => <TableRow key={student.id}><TableCell className="font-medium">{student.nama || '-'}</TableCell><TableCell>{student.nisn || '-'}</TableCell><TableCell><Badge variant={student.status === 'aktif' ? 'default' : 'secondary'}>{student.status || '-'}</Badge></TableCell><TableCell><span className="inline-flex items-center gap-1 text-sm">{student.identitasFileExt === 'pdf' ? <FileText className="h-4 w-4 text-rose-600" /> : student.identitasFileExt ? <Image className="h-4 w-4 text-brand-500" /> : null}<Badge variant={student.identitasFileExt ? 'secondary' : 'outline'}>{extensionLabel(student.identitasFileExt)}</Badge></span></TableCell></TableRow>)}
              {!loadingStudents && kelasId && students.length === 0 && <TableRow><TableCell colSpan={4} className="h-24 text-center text-sm text-muted-foreground">Tidak ada siswa pada kelas ini.</TableCell></TableRow>}
              {!kelasId && <TableRow><TableCell colSpan={4} className="h-24 text-center text-sm text-muted-foreground">Pilih kelas untuk menampilkan siswa.</TableCell></TableRow>}
              {loadingStudents && <TableRow><TableCell colSpan={4} className="h-24 text-center text-sm text-muted-foreground">Memuat siswa...</TableCell></TableRow>}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
