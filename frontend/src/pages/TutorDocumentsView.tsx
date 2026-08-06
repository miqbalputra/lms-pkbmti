import { useEffect, useState } from 'react'
import { Download, FileText, FileUp, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { PageToolbar } from '../components/ui/page'
import { apiBase } from '../lib/api'

type TutorDocumentRow = {
  id: string
  nama: string
  skPengangkatanTersedia: boolean
  skPengangkatanNama?: string
}

type AdminDocuments = {
  skPenugasanTersedia: boolean
  skPenugasanNama?: string
  tutors: TutorDocumentRow[]
}

type MyDocuments = {
  nama: string
  skPengangkatanTersedia: boolean
  skPenugasanTersedia: boolean
  skPengangkatanNama?: string
  skPenugasanNama?: string
}

async function readError(response: Response, fallback: string) {
  const body = await response.json().catch(() => ({})) as { error?: string }
  return body.error || fallback
}

async function downloadPdf(path: string, token: string, fallbackName: string) {
  const response = await fetch(apiBase + path, {
    credentials: 'include',
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!response.ok) throw new Error(await readError(response, 'Gagal mengunduh dokumen.'))
  const blob = await response.blob()
  const disposition = response.headers.get('Content-Disposition') || ''
  const match = /filename="?([^";]+)"?/.exec(disposition)
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = match?.[1] || fallbackName
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}

async function uploadPdf(path: string, file: File, token: string) {
  if (file.type !== 'application/pdf' && !file.name.toLowerCase().endsWith('.pdf')) {
    throw new Error('Dokumen harus berformat PDF.')
  }
  const data = new FormData()
  data.append('file', file)
  const response = await fetch(apiBase + path, {
    method: 'POST',
    credentials: 'include',
    headers: { Authorization: `Bearer ${token}` },
    body: data,
  })
  if (!response.ok) throw new Error(await readError(response, 'Gagal mengunggah dokumen.'))
}

export function TutorDocumentsView({ token, role }: { token: string; role: string }) {
  const [adminData, setAdminData] = useState<AdminDocuments | null>(null)
  const [myData, setMyData] = useState<MyDocuments | null>(null)
  const [selectedFiles, setSelectedFiles] = useState<Record<string, File | null>>({})
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    try {
      const response = await fetch(apiBase + (role === 'admin' ? '/tutor/dokumen' : '/tutor/me/dokumen'), {
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!response.ok) throw new Error(await readError(response, 'Gagal memuat dokumen tutor.'))
      const data = await response.json()
      if (role === 'admin') setAdminData(data as AdminDocuments)
      else setMyData(data as MyDocuments)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Gagal memuat dokumen tutor.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [token, role]) // eslint-disable-line react-hooks/exhaustive-deps

  async function handleUpload(key: string, path: string) {
    const file = selectedFiles[key]
    if (!file) {
      toast.error('Pilih file PDF terlebih dahulu.')
      return
    }
    setBusy(key)
    try {
      await uploadPdf(path, file, token)
      toast.success('Dokumen berhasil disimpan.')
      setSelectedFiles((previous) => ({ ...previous, [key]: null }))
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Gagal mengunggah dokumen.')
    } finally {
      setBusy(null)
    }
  }

  async function handleDownload(key: string, path: string, fallbackName: string) {
    setBusy(key)
    try {
      await downloadPdf(path, token, fallbackName)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Gagal mengunduh dokumen.')
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Dokumen Tutor"
        description={role === 'admin' ? 'Kelola SK Pengangkatan per tutor dan SK Penugasan bersama.' : 'Unduh dokumen resmi yang telah disediakan admin.'}
        actions={
          <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
            <RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
            Muat ulang
          </Button>
        }
      />

      {loading && <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">Memuat dokumen...</CardContent></Card>}

      {!loading && role === 'guru' && myData && (
        <div className="grid gap-4 md:grid-cols-2">
          <DocumentCard
            title="SK Pengangkatan"
            description={myData.skPengangkatanNama || 'File khusus untuk tutor ini.'}
            available={myData.skPengangkatanTersedia}
            busy={busy === 'my-angkat'}
            onDownload={() => void handleDownload('my-angkat', '/tutor/me/dokumen/sk-pengangkatan', 'sk-pengangkatan.pdf')}
          />
          <DocumentCard
            title="SK Penugasan"
            description={myData.skPenugasanNama || 'File bersama untuk seluruh tutor.'}
            available={myData.skPenugasanTersedia}
            busy={busy === 'my-tugas'}
            onDownload={() => void handleDownload('my-tugas', '/tutor/me/dokumen/sk-penugasan', 'sk-penugasan.pdf')}
          />
        </div>
      )}

      {!loading && role === 'admin' && adminData && (
        <>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2"><FileText className="h-5 w-5 text-brand-500" />SK Penugasan Bersama</CardTitle>
              <CardDescription>Satu PDF ini dapat diunduh oleh semua akun tutor.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-3 sm:flex-row sm:items-center">
              <Input
                type="file"
                accept="application/pdf,.pdf"
                onChange={(event) => setSelectedFiles((previous) => ({ ...previous, shared: event.currentTarget.files?.[0] || null }))}
                className="sm:max-w-md"
              />
              <Button onClick={() => void handleUpload('shared', '/tutor/dokumen/sk-penugasan')} disabled={busy === 'shared'}>
                <FileUp className="h-4 w-4" />
                {busy === 'shared' ? 'Mengunggah...' : 'Simpan PDF'}
              </Button>
              {adminData.skPenugasanTersedia && (
                <Button variant="outline" onClick={() => void handleDownload('shared-download', '/tutor/dokumen/sk-penugasan/download', adminData.skPenugasanNama || 'sk-penugasan.pdf')} disabled={busy === 'shared-download'}>
                  <Download className="h-4 w-4" /> Unduh
                </Button>
              )}
              {adminData.skPenugasanTersedia && <Badge variant="success">Tersedia: {adminData.skPenugasanNama}</Badge>}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>SK Pengangkatan per Tutor</CardTitle>
              <CardDescription>Upload bersifat opsional. File hanya terlihat oleh tutor terkait dan admin.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {adminData.tutors.map((tutor) => {
                const key = `angkat-${tutor.id}`
                return (
                  <div key={tutor.id} className="flex flex-col gap-3 rounded-xl border border-border/70 p-3 lg:flex-row lg:items-center">
                    <div className="min-w-0 flex-1">
                      <div className="font-semibold text-foreground">{tutor.nama}</div>
                      <div className="text-xs text-muted-foreground">{tutor.skPengangkatanNama || 'Belum ada SK Pengangkatan'}</div>
                    </div>
                    <Input type="file" accept="application/pdf,.pdf" onChange={(event) => setSelectedFiles((previous) => ({ ...previous, [key]: event.currentTarget.files?.[0] || null }))} className="lg:max-w-sm" />
                    <Button size="sm" onClick={() => void handleUpload(key, `/tutor/${encodeURIComponent(tutor.id)}/dokumen/sk-pengangkatan`)} disabled={busy === key}>
                      <FileUp className="h-4 w-4" /> {busy === key ? 'Mengunggah...' : 'Simpan'}
                    </Button>
                    {tutor.skPengangkatanTersedia && <Button size="sm" variant="outline" onClick={() => void handleDownload(`download-${tutor.id}`, `/tutor/${encodeURIComponent(tutor.id)}/dokumen/sk-pengangkatan`, tutor.skPengangkatanNama || 'sk-pengangkatan.pdf')} disabled={busy === `download-${tutor.id}`}><Download className="h-4 w-4" /> Unduh</Button>}
                  </div>
                )
              })}
              {!adminData.tutors.length && <div className="py-8 text-center text-sm text-muted-foreground">Belum ada data tutor.</div>}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}

function DocumentCard({ title, description, available, busy, onDownload }: { title: string; description: string; available: boolean; busy: boolean; onDownload: () => void }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><FileText className="h-5 w-5 text-brand-500" />{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between gap-3">
          <Badge variant={available ? 'success' : 'secondary'}>{available ? 'Tersedia' : 'Belum tersedia'}</Badge>
          <Button variant="outline" onClick={onDownload} disabled={!available || busy}>
            <Download className="h-4 w-4" /> {busy ? 'Mengunduh...' : 'Unduh PDF'}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
