import { useEffect, useState } from 'react'
import { Download, Upload } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { User } from '../App'
import { request } from '../lib/api'
import { formatWibDateTime } from '../lib/wib'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Row = Record<string, unknown> & { id: string }
type Issue = { row: number; error: string }
type ImportLogRow = {
  id: string
  tipe: string
  fileName: string
  totalBaris: number
  berhasil: number
  gagal: number
  status: string
  createdAt?: string
  errorJson?: string
}
type Result = { logId: string; totalBaris: number; berhasil: number; gagal: number; issues: Issue[] }

const TIPES = [
  { tipe: 'siswa-lengkap', label: 'Peserta Didik Lengkap (+ Data Orang Tua)', needsKelas: false, adminOnly: true },
  { tipe: 'siswa', label: 'Peserta Didik (Basic)', needsKelas: false, adminOnly: true },
  { tipe: 'tutor', label: 'Tutor / Pengajar', needsKelas: false, adminOnly: true },
  { tipe: 'nilai-kompetensi', label: 'Nilai Kompetensi', needsKelas: true, adminOnly: false },
]

export function ImportView({ token, user }: { token: string; user: User }) {
  const [tipe, setTipe] = useState('siswa')
  const [kelas, setKelas] = useState<Row[]>([])
  const [kelasId, setKelasId] = useState('')
  const [semester, setSemester] = useState('Ganjil')
  const [file, setFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)
  const [result, setResult] = useState<Result | null>(null)
  const [logs, setLogs] = useState<ImportLogRow[]>([])

  const isGuru = user.role === 'guru'
  const kelasOptions = isGuru
    ? kelas.filter((k) => String(k.waliKelasId || '') === (user.tutorId || ''))
    : kelas

  const activeTipe = TIPES.find((t) => t.tipe === tipe)!
  const canUseTipe = activeTipe.adminOnly ? user.role === 'admin' : true

  function loadLogs() {
    void request('/import/log', token).then((r: ImportLogRow[]) => setLogs(r || [])).catch(() => setLogs([]))
  }

  useEffect(() => {
    loadLogs()
    void request('/kelas', token).then((r: Row[]) => setKelas(r || [])).catch(() => setKelas([]))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  async function downloadTemplate() {
    try {
      const res = await fetch(apiBase + '/import/template/' + tipe, {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const detail = await res.json().catch(() => ({})) as { error?: string }
        throw new Error(detail.error || `Gagal mengunduh template (${res.status}).`)
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `template-import-${tipe}.xlsx`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengunduh template.')
    }
  }

  async function doUpload() {
    if (!file) { toast.error('Pilih file Excel terlebih dahulu.'); return }
    if (activeTipe.needsKelas && !kelasId) { toast.error('Pilih kelas untuk import nilai kompetensi.'); return }
    setUploading(true)
    setResult(null)
    try {
      const data = new FormData()
      data.append('tipe', tipe)
      data.append('file', file)
      if (activeTipe.needsKelas) {
        data.append('kelasId', kelasId)
        data.append('semester', semester)
      }
      const r = await fetch(apiBase + '/import', {
        method: 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      const res = await r.json().catch(() => ({}))
      if (!r.ok) throw new Error((res as any)?.error || `Permintaan gagal (${r.status}).`)
      setResult(res as Result)
      toast.success(`Import selesai: ${(res as Result).berhasil} berhasil, ${(res as Result).gagal} gagal.`)
      setFile(null)
      void loadLogs()
    } catch (err: any) {
      toast.error(err.message || 'Gagal mengimport.')
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Import Terpusat"
        description="Unduh template Excel, isi, lalu unggah. Baris gagal dilewati & dicatat (partial success); riwayat import tersimpan."
      />

      <Card className="rounded-2xl border border-border bg-card p-5 shadow-2xs space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="grid gap-2">
            <Label>Tipe Import</Label>
            <Select value={tipe} onChange={(e) => { setTipe(e.target.value); setResult(null); setFile(null) }}>
              {TIPES.map((t) => {
                const disabled = t.adminOnly && user.role !== 'admin'
                return (
                  <option key={t.tipe} value={t.tipe} disabled={disabled}>
                    {t.label}{disabled ? ' (admin only)' : ''}
                  </option>
                )
              })}
            </Select>
          </div>
          <div className="grid gap-2">
            <Label>Template</Label>
            <Button type="button" variant="outline" onClick={downloadTemplate} disabled={!canUseTipe}>
              <Download className="h-4 w-4" /> Unduh template {activeTipe.label}
            </Button>
          </div>
        </div>

        {activeTipe.needsKelas && (
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label>Kelas / Rombel</Label>
              <Select value={kelasId} onChange={(e) => setKelasId(e.target.value)}>
                <option value="">Pilih kelas...</option>
                {kelasOptions.map((k) => (
                  <option key={k.id} value={k.id}>Kelas {String(k.jenjang ?? '')}{String(k.namaRombel ?? '')}</option>
                ))}
              </Select>
              {isGuru && !kelasOptions.length && (
                <p className="text-xs text-muted-foreground">Anda belum ditugaskan sebagai wali kelas.</p>
              )}
            </div>
            <div className="grid gap-2">
              <Label>Semester</Label>
              <Select value={semester} onChange={(e) => setSemester(e.target.value)}>
                <option value="Ganjil">Ganjil</option>
                <option value="Genap">Genap</option>
              </Select>
            </div>
          </div>
        )}

        <div className="grid gap-2">
          <Label>File Excel (.xlsx, maks 5 MB)</Label>
          <Input type="file" accept=".xlsx" onChange={(e) => setFile(e.target.files?.[0] || null)} />
          {tipe === 'siswa-lengkap' && <p className="text-xs text-muted-foreground">Kolom <code>kelas</code> memakai kode gabungan tingkat + rombel, misalnya <strong>1A</strong>, <strong>2B</strong>, atau <strong>6A</strong>. Format <strong>KELAS 1A</strong> juga diterima.</p>}
          {tipe === 'siswa' && <p className="text-xs text-muted-foreground">Template Basic memakai <code>kelas_id</code>, yaitu ID teknis rombel. Untuk input yang lebih mudah dibaca, gunakan template <strong>Peserta Didik Lengkap</strong> dengan kolom <code>kelas</code> seperti 1A atau 6A.</p>}
        </div>

        <div className="pt-1">
          <Button onClick={doUpload} disabled={uploading || !canUseTipe}>
            <Upload className="h-4 w-4" /> {uploading ? 'Mengimpor...' : 'Import'}
          </Button>
        </div>
      </Card>

      {result && (
        <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center gap-3">
            <h3 className="text-base font-bold">Hasil Import</h3>
            <Badge variant="default">{result.berhasil} berhasil</Badge>
            <Badge variant={result.gagal ? 'destructive' : 'secondary'}>{result.gagal} gagal</Badge>
            <span className="text-xs text-muted-foreground">dari {result.totalBaris} baris</span>
          </div>
          {result.issues.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow className="border-b border-border">
                  <TableHead>Baris</TableHead>
                  <TableHead>Pesan Error</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {result.issues.map((is, i) => (
                  <TableRow key={i}>
                    <TableCell className="font-mono">{is.row}</TableCell>
                    <TableCell className="text-sm">{is.error}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          {result.issues.length === 0 && (
            <div className="p-4 text-sm text-muted-foreground">Semua baris berhasil diimport.</div>
          )}
        </Card>
      )}

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <div className="px-4 py-3 border-b border-border">
          <h3 className="text-base font-bold">Riwayat Import</h3>
        </div>
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Waktu</TableHead>
              <TableHead>Tipe</TableHead>
              <TableHead>File</TableHead>
              <TableHead>Baris</TableHead>
              <TableHead>Hasil</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.map((l) => (
              <TableRow key={l.id}>
                <TableCell className="text-sm">{formatWibDateTime(l.createdAt)}</TableCell>
                <TableCell className="font-medium">{String(l.tipe || '-')}</TableCell>
                <TableCell className="text-sm">{String(l.fileName || '-')}</TableCell>
                <TableCell className="text-sm">{l.totalBaris}</TableCell>
                <TableCell>
                  <span className="text-sm">
                    <span className="text-success font-medium">{l.berhasil}</span> / <span className="text-destructive">{l.gagal}</span>
                  </span>
                </TableCell>
              </TableRow>
            ))}
            {!logs.length && <EmptyState colSpan={5} label="Belum ada riwayat import." />}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
