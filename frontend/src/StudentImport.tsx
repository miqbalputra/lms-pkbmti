import { useState, type ChangeEvent } from 'react'
import { Download, Upload } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from './components/ui/alert'
import { Button } from './components/ui/button'
import { FormCard } from './components/ui/page'
import { Input } from './components/ui/input'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
const importType = 'siswa-lengkap'

type ImportIssue = { row: number; error: string }
type ImportResult = { berhasil?: number; gagal?: number; error?: string; issues?: ImportIssue[] }

export function StudentImport({ token, close, done }: { token: string; close: () => void; done: () => void }) {
  const [file, setFile] = useState<File | null>(null)
  const [message, setMessage] = useState('')
  const [issues, setIssues] = useState<ImportIssue[]>([])
  const [loading, setLoading] = useState(false)

  function select(e: ChangeEvent<HTMLInputElement>) {
    setFile(e.target.files?.[0] || null)
    setMessage('')
    setIssues([])
  }

  async function template() {
    try {
      const response = await fetch(`${apiBase}/import/template/${importType}`, {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!response.ok) {
        const result = await response.json().catch(() => ({})) as ImportResult
        setMessage(result.error || 'Template tidak dapat diunduh.')
        return
      }
      const url = URL.createObjectURL(await response.blob())
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = 'template-import-peserta-didik-lengkap.xlsx'
      anchor.click()
      URL.revokeObjectURL(url)
    } catch {
      setMessage('Template tidak dapat diunduh.')
    }
  }

  async function upload() {
    if (!file) {
      setMessage('Pilih file .xlsx terlebih dahulu.')
      return
    }
    setLoading(true)
    setMessage('')
    setIssues([])
    const data = new FormData()
    data.append('tipe', importType)
    data.append('file', file)
    try {
      const response = await fetch(`${apiBase}/import`, {
        method: 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: data,
      })
      const result = await response.json().catch(() => ({})) as ImportResult
      if (!response.ok) {
        setMessage(result.error || 'Import gagal.')
        setIssues(result.issues || [])
        return
      }
      const berhasil = result.berhasil || 0
      const gagal = result.gagal || 0
      setMessage(`${berhasil} peserta didik berhasil diimport${gagal ? `, ${gagal} baris gagal.` : '.'}`)
      setIssues(result.issues || [])
      done()
    } catch {
      setMessage('Import gagal. Periksa koneksi jaringan Anda.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <FormCard
      title="Import peserta didik"
      description="Gunakan template siswa lengkap. Kolom tanggal_lahir wajib diisi dengan format YYYY-MM-DD agar orang tua dapat login ke portal."
    >
      <div className="space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <Button variant="outline" onClick={() => void template()}>
            <Download className="h-4 w-4" />
            Unduh template
          </Button>
          <Input className="max-w-md" accept=".xlsx" type="file" onChange={select} />
          <Button disabled={loading} onClick={() => void upload()}>
            <Upload className="h-4 w-4" />
            {loading ? 'Mengimport...' : 'Import file'}
          </Button>
          <Button variant="ghost" onClick={close}>Tutup</Button>
        </div>
        {message && <Alert><AlertDescription>{message}</AlertDescription></Alert>}
        {issues.length > 0 && (
          <Alert className="border-destructive/40">
            <AlertTitle>Baris yang harus diperbaiki</AlertTitle>
            <AlertDescription>
              {issues.map((issue) => <div key={issue.row}>Baris {issue.row}: {issue.error}</div>)}
            </AlertDescription>
          </Alert>
        )}
      </div>
    </FormCard>
  )
}
