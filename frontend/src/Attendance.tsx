import { useCallback, useEffect, useRef, useState, type ChangeEvent, type ReactNode } from 'react'
import { AlertTriangle, CheckCircle2, Download, FileSpreadsheet, FileText, Image as ImageIcon, Info, Save, Trash2, UploadCloud } from 'lucide-react'
import { AttendanceRecap } from './AttendanceRecap'
import { apiBase, request } from './lib/api'
import { isSaturdayWibDate, isTodaySaturdayWib, nextSaturdayWib, wibToday } from './lib/wib'
import { Alert, AlertDescription } from './components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from './components/ui/alert-dialog'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from './components/ui/dialog'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { Select } from './components/ui/select'
import { Signature } from './components/ui/Signature'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './components/ui/table'
import { toast } from 'sonner'

type Row = Record<string, unknown> & { id: string }

const MAX_SOURCE_PHOTO_BYTES = 15 * 1024 * 1024
const TARGET_PHOTO_BYTES = 500 * 1024
const PHOTO_MAX_DIMENSION = 1600

function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(new Error('Foto tidak dapat dibaca.'))
    reader.readAsDataURL(blob)
  })
}

function canvasToJpeg(canvas: HTMLCanvasElement, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error('Foto tidak dapat dikompres.'))),
      'image/jpeg',
      quality,
    )
  })
}

function loadPhoto(file: File): Promise<{ image: HTMLImageElement; url: string }> {
  const url = URL.createObjectURL(file)
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve({ image, url })
    image.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error(`Foto ${file.name} tidak dapat dibuka. Gunakan JPG, PNG, atau WEBP.`))
    }
    image.src = url
  })
}

async function optimizeAttendancePhoto(file: File): Promise<string> {
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    throw new Error(`Format ${file.name} tidak didukung. Gunakan JPG, PNG, atau WEBP.`)
  }
  if (file.size > MAX_SOURCE_PHOTO_BYTES) {
    throw new Error(`Ukuran ${file.name} melebihi 15 MB.`)
  }

  const { image, url } = await loadPhoto(file)
  try {
    if (!image.naturalWidth || !image.naturalHeight) {
      throw new Error(`Dimensi foto ${file.name} tidak valid.`)
    }

    let maxDimension = PHOTO_MAX_DIMENSION
    let smallest: Blob | null = null
    for (let resizeAttempt = 0; resizeAttempt < 4; resizeAttempt += 1) {
      const scale = Math.min(1, maxDimension / Math.max(image.naturalWidth, image.naturalHeight))
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(1, Math.round(image.naturalWidth * scale))
      canvas.height = Math.max(1, Math.round(image.naturalHeight * scale))
      const context = canvas.getContext('2d')
      if (!context) throw new Error('Browser tidak mendukung pemrosesan foto.')
      context.fillStyle = '#ffffff'
      context.fillRect(0, 0, canvas.width, canvas.height)
      context.drawImage(image, 0, 0, canvas.width, canvas.height)

      for (const quality of [0.82, 0.72, 0.62, 0.52]) {
        const blob = await canvasToJpeg(canvas, quality)
        smallest = blob
        if (blob.size <= TARGET_PHOTO_BYTES) return blobToDataURL(blob)
      }
      maxDimension = Math.floor(maxDimension * 0.8)
    }

    if (!smallest) throw new Error(`Foto ${file.name} tidak dapat dikompres.`)
    return blobToDataURL(smallest)
  } finally {
    URL.revokeObjectURL(url)
  }
}

export function AttendanceWorkspace({
  token,
  readOnly,
  userName = 'Guru / Wali Kelas',
}: {
  token: string
  readOnly: boolean
  userName?: string
}) {
  const [classes, setClasses] = useState<Row[]>([])
  const [meetings, setMeetings] = useState<Row[]>([])
  const [classID, setClassID] = useState('')
  const [selectedID, setSelectedID] = useState('new')
  const [wibDateToday, setWibDateToday] = useState(() => wibToday())
  const [date, setDate] = useState(() => nextSaturdayWib())
  const [students, setStudents] = useState<Row[]>([])
  const [marks, setMarks] = useState<Record<string, string>>({})
  const [signature, setSignature] = useState('')
  const [photos, setPhotos] = useState<string[]>([])
  const [status, setStatus] = useState('berlangsung')
  const [message, setMessage] = useState('')
  const [previewModalUrl, setPreviewModalUrl] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [processingPhotos, setProcessingPhotos] = useState(false)
  const [loadingMeeting, setLoadingMeeting] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const selectionVersion = useRef(0)
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const todayIsSaturday = isTodaySaturdayWib(new Date())
  const dateIsSaturday = isSaturdayWibDate(date)

  useEffect(() => {
    const timer = window.setInterval(() => setWibDateToday(wibToday()), 60_000)
    return () => window.clearInterval(timer)
  }, [])

  useEffect(() => {
    if (selectedID === 'new' && wibDateToday !== wibToday()) {
      setWibDateToday(wibToday())
      setDate(nextSaturdayWib())
    }
  }, [selectedID, wibDateToday])

  const loadMeetings = useCallback(() => request('/presensi', token).then(setMeetings), [token])

  useEffect(() => {
    void request('/kelas', token).then(setClasses)
    void loadMeetings()
  }, [loadMeetings, token])

  useEffect(() => {
    if (classID) {
      void request('/peserta-didik?kelasId=' + classID, token).then((rows: Row[]) => {
        setStudents(rows)
        if (selectedID === 'new') {
          setMarks(Object.fromEntries(rows.map((r) => [r.id, 'Hadir'])))
        }
      })
    }
  }, [classID, token, selectedID])

  async function choose(id: string) {
    const version = ++selectionVersion.current
    setSelectedID(id)
    setMessage('')
    if (id === 'new') {
      setLoadingMeeting(false)
      setClassID('')
      setDate(nextSaturdayWib())
      setSignature('')
      setPhotos([])
      setStatus('berlangsung')
      return
    }
    const summary = meetings.find((meeting) => meeting.id === id)
    if (!summary) return
    setClassID(String(summary.kelasId))
    setDate(String(summary.tanggal).slice(0, 10))
    setStatus(String(summary.statusPertemuan))
    setSignature('')
    setPhotos([])
    setMarks({})
    setLoadingMeeting(true)
    try {
      const row = await request('/presensi/' + id, token)
      if (selectionVersion.current !== version) return
      setClassID(String(row.kelasId))
      setDate(String(row.tanggal).slice(0, 10))
      setStatus(String(row.statusPertemuan))
      setSignature(String(row.tandaTangan || ''))

      let loadedPhotos: string[] = []
      if (row.buktiFoto) {
        try {
          loadedPhotos = JSON.parse(String(row.buktiFoto))
          if (!Array.isArray(loadedPhotos)) loadedPhotos = []
        } catch {
          loadedPhotos = [String(row.buktiFoto)]
        }
      }
      setPhotos(loadedPhotos)

      setMarks(
        Object.fromEntries(
          ((row.details as Row[]) || []).map((detail) => [
            String(detail.pesertaDidikId),
            String(detail.statusKehadiran),
          ])
        )
      )
    } catch (error) {
      if (selectionVersion.current !== version) return
      const detail = error instanceof Error ? error.message : String(error)
      setMessage(`Data pertemuan gagal dimuat: ${detail}`)
      toast.error(`Data pertemuan gagal dimuat: ${detail}`)
    } finally {
      if (selectionVersion.current === version) setLoadingMeeting(false)
    }
  }

  const handlePhotoUpload = async (e: ChangeEvent<HTMLInputElement>) => {
    const input = e.currentTarget
    const files = input.files
    if (!files || files.length === 0) return

    if (photos.length + files.length > 5) {
      toast.error('Maksimal 5 foto bukti pembelajaran kegiatan KBM.')
      input.value = ''
      return
    }

    const fileList = Array.from(files)
    input.value = ''
    setProcessingPhotos(true)
    try {
      const optimized: string[] = []
      for (const file of fileList) {
        try {
          optimized.push(await optimizeAttendancePhoto(file))
        } catch (error) {
          toast.error(error instanceof Error ? error.message : `Foto ${file.name} gagal diproses.`)
        }
      }
      if (optimized.length > 0) {
        setPhotos((previous) => [...previous, ...optimized].slice(0, 5))
        toast.success(`${optimized.length} foto siap diunggah.`)
      }
    } finally {
      setProcessingPhotos(false)
    }
  }

  const handleRemovePhoto = (index: number) => {
    setPhotos((prev) => prev.filter((_, i) => i !== index))
  }

  const isSignatureValid = signature.trim() !== ''
  const isPhotosValid = photos.length >= 1 && photos.length <= 5

  async function save() {
    if (!classID) {
      setMessage('Pilih rombongan belajar terlebih dahulu.')
      toast.error('Pilih rombongan belajar terlebih dahulu.')
      return
    }
    if (!isSaturdayWibDate(date)) {
      const warning = 'Hari ini bukan hari sabtu, pilih tanggal di hari Sabtu.'
      setMessage(warning)
      toast.error('Tanggal pertemuan hanya boleh dipilih pada hari Sabtu (WIB).')
      return
    }
    // Strict Validation 1: Teacher Signature Required
    if (!isSignatureValid) {
      setMessage('Langkah 2 Wajib: Tanda tangan pengajar di layar wajib diisi sebelum menyimpan.')
      toast.error('Langkah 2 Wajib: Tanda tangan pengajar di layar wajib diisi.')
      return
    }
    // Strict Validation 2: Photo Evidence Required (Min 1, Max 5)
    if (!isPhotosValid) {
      setMessage('Langkah 3 Wajib: Upload bukti foto KBM wajib dilakukan (Minimal 1 foto, Maksimal 5 foto).')
      toast.error('Langkah 3 Wajib: Upload foto bukti kegiatan KBM wajib diisi (minimal 1 foto).')
      return
    }

    try {
      setSubmitting(true)
      const payload = {
        kelasId: classID,
        tanggal: date,
        statusPertemuan: status,
        dibuatOtomatis: false,
        tandaTangan: signature,
      }
      let meeting: Row
      try {
        meeting =
          selectedID === 'new'
            ? await request('/presensi', token, 'POST', payload)
            : await request('/presensi/' + selectedID, token, 'PUT', payload)
      } catch (error) {
        const detail = error instanceof Error ? error.message : String(error)
        throw new Error(`Data pertemuan gagal disimpan: ${detail}`)
      }

      // Simpan ID segera setelah tahap pertama berhasil. Jika tahap foto gagal,
      // klik ulang akan memperbarui record yang sama, bukan membuat duplikat.
      setSelectedID(meeting.id)
      setMeetings((current) => {
        const previous = current.find((row) => row.id === meeting.id)
        const kelas = previous?.kelas || classes.find((row) => row.id === classID)
        const summary = { ...previous, ...meeting, kelas }
        return previous
          ? current.map((row) => (row.id === meeting.id ? summary : row))
          : [summary, ...current]
      })

      try {
        await request(
          '/presensi/' + meeting.id + '/details',
          token,
          'POST',
          students.map((student) => ({
            pesertaDidikId: student.id,
            statusKehadiran: marks[student.id] || 'Hadir',
          }))
        )
      } catch (error) {
        const detail = error instanceof Error ? error.message : String(error)
        throw new Error(`Pertemuan tersimpan, tetapi checklist siswa gagal disimpan: ${detail}`)
      }

      try {
        await request('/presensi/' + meeting.id + '/photos', token, 'PUT', {
          buktiFoto: JSON.stringify(photos),
        })
      } catch (error) {
        const detail = error instanceof Error ? error.message : String(error)
        throw new Error(`Presensi siswa tersimpan, tetapi foto KBM gagal disimpan: ${detail}. Klik Simpan kembali untuk mencoba ulang.`)
      }

      toast.success('Data presensi, tanda tangan & foto bukti KBM berhasil disimpan.')
      setMessage('Presensi, tanda tangan & bukti KBM berhasil disimpan.')
      try {
        await loadMeetings()
      } catch {
        toast.warning('Presensi sudah tersimpan, tetapi daftar pertemuan belum dapat dimuat ulang.')
      }
    } catch (e: any) {
      const err = String(e.message || e)
      setMessage(err)
      toast.error(`Gagal menyimpan: ${err}`)
    } finally {
      setSubmitting(false)
    }
  }

  async function deleteSelectedMeeting() {
    if (selectedID === 'new') return
    setDeleting(true)
    try {
      await request('/presensi/' + selectedID, token, 'DELETE')
      setMeetings((current) => current.filter((meeting) => meeting.id !== selectedID))
      await choose('new')
      setDeleteDialogOpen(false)
      setMessage('Data presensi berhasil dihapus.')
      toast.success('Data presensi dan seluruh checklist siswa berhasil dihapus.')
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error)
      setMessage(`Data presensi gagal dihapus: ${detail}`)
      toast.error(`Gagal menghapus presensi: ${detail}`)
    } finally {
      setDeleting(false)
    }
  }

  async function download(path: string, name: string) {
    const r = await fetch(apiBase + path, {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!r.ok) {
      setMessage('Export presensi gagal.')
      toast.error('Export presensi gagal.')
      return
    }
    const url = URL.createObjectURL(await r.blob())
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.click()
    URL.revokeObjectURL(url)
  }

  // presensiExportPath builds the /presensi/export query string for the chosen
  // kelas + optional date range. Empty from/to = full range.
  function presensiExportPath(format: 'csv' | 'xlsx' | 'pdf'): string {
    const params = new URLSearchParams()
    params.set('format', format)
    if (classID) params.set('kelasId', classID)
    if (from) params.set('from', from)
    if (to) params.set('to', to)
    return '/presensi/export?' + params.toString()
  }

  return (
    <div className="space-y-6">
      {/* Upper Card: Meeting Filter & Settings */}
      <Card className="rounded-2xl border border-border bg-card shadow-2xs">
        <CardHeader className="border-b border-border/60">
          <CardTitle>Presensi Pertemuan & Pembelajaran KBM</CardTitle>
          <CardDescription>Pilih pertemuan atau buat entri presensi KBM baru.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 pt-6">
          {!todayIsSaturday && (
            <Alert variant="destructive" className="border-warning/50 bg-warning/10 text-warning-foreground">
              <AlertTriangle className="h-4 w-4" />
              <AlertDescription>
                Hari ini bukan hari sabtu, pilih tanggal di hari Sabtu. Sistem menggunakan tanggal hari ini berdasarkan WIB (Asia/Jakarta).
              </AlertDescription>
            </Alert>
          )}
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Field label="Pertemuan">
              <Select value={selectedID} onChange={(e) => void choose(e.target.value)}>
                <option value="new">+ Pertemuan tambahan baru</option>
                {meetings.map((m) => (
                  <option key={m.id} value={m.id}>
                    Kelas {String((m.kelas as Row)?.jenjang)}
                    {String((m.kelas as Row)?.namaRombel)} -{' '}
                    {new Date(String(m.tanggal)).toLocaleDateString('id-ID')}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Rombel">
              <Select
                value={classID}
                disabled={selectedID !== 'new'}
                onChange={(e) => setClassID(e.target.value)}
              >
                <option value="">Pilih rombel</option>
                {classes.map((c) => (
                  <option key={c.id} value={c.id}>
                    Kelas {String(c.jenjang)}
                    {String(c.namaRombel)}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Tanggal Pertemuan">
              <Input
                type="date"
                value={date}
                onChange={(e) => {
                  const value = e.target.value
                  if (!isSaturdayWibDate(value)) {
                    setMessage('Hari ini bukan hari sabtu, pilih tanggal di hari Sabtu.')
                    toast.error('Tanggal pertemuan hanya boleh hari Sabtu (WIB).')
                    return
                  }
                  setMessage('')
                  setDate(value)
                }}
                aria-invalid={!dateIsSaturday}
              />
              <span className="text-xs text-muted-foreground">Hanya Sabtu; tanggal dihitung menurut WIB.</span>
            </Field>
            <Field label="Status Pertemuan">
              <Select value={status} onChange={(e) => setStatus(e.target.value)}>
                <option value="berlangsung">Berlangsung</option>
                <option value="libur">Libur</option>
                <option value="dipindah">Dipindah</option>
              </Select>
            </Field>
          </div>

          <div className="flex flex-wrap items-end gap-2">
            <div className="grid gap-1">
              <Label className="text-xs">Rentang tanggal (opsional)</Label>
              <div className="flex items-center gap-1">
                <Input type="date" value={from} onChange={(e) => setFrom(e.target.value)} className="w-auto" />
                <span className="text-xs text-muted-foreground">s/d</span>
                <Input type="date" value={to} onChange={(e) => setTo(e.target.value)} className="w-auto" />
              </div>
            </div>
            <Button
              variant="outline"
              onClick={() => void download(presensiExportPath('csv'), 'rekap-presensi.csv')}
            >
              <Download className="h-4 w-4 mr-1" /> CSV
            </Button>
            <Button
              variant="outline"
              onClick={() => void download(presensiExportPath('xlsx'), 'rekap-presensi.xlsx')}
            >
              <FileSpreadsheet className="h-4 w-4 mr-1" /> Excel
            </Button>
            <Button
              variant="outline"
              onClick={() => void download(presensiExportPath('pdf'), 'rekap-presensi.pdf')}
            >
              <FileText className="h-4 w-4 mr-1" /> PDF Rekap
            </Button>
            {selectedID !== 'new' && (
              <Button variant="outline" onClick={() => void download('/presensi/' + selectedID + '/pdf', 'presensi.pdf')}>
                <Download className="h-4 w-4 mr-1" /> PDF Pertemuan
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Main Card: 3-Step Guided Attendance Form */}
      {classID && (
        <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
          <CardHeader className="border-b border-border/60 flex flex-col md:flex-row md:items-center justify-between gap-4">
            <div>
              <CardTitle>Pengisian Presensi & Dokumentasi Kegiatan</CardTitle>
              <CardDescription>
                {selectedID === 'new'
                  ? 'Lengkapi 3 langkah wajib sebelum menyimpan data presensi.'
                  : 'Mode edit: ubah data yang salah, lalu simpan perubahan atau hapus presensi.'}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2 text-xs font-semibold">
              <Badge variant={isSignatureValid && isPhotosValid ? 'secondary' : 'outline'} className="gap-1 px-3 py-1">
                {isSignatureValid && isPhotosValid ? (
                  <>
                    <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                    <span>Siap Disimpan</span>
                  </>
                ) : (
                  <>
                    <AlertTriangle className="h-3.5 w-3.5 text-warning" />
                    <span>Menunggu Tanda Tangan & Foto</span>
                  </>
                )}
              </Badge>
            </div>
          </CardHeader>

          <CardContent className="space-y-6 pt-6">
            {/* Step 1: Checklist Attendance Table */}
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground font-extrabold text-xs">
                  1
                </span>
                <h4 className="text-sm font-bold text-foreground">
                  Checklist Kehadiran Peserta Didik ({students.length} Siswa)
                </h4>
              </div>

              <div className="rounded-xl border border-border overflow-hidden">
                <Table>
                  <TableHeader>
                    <TableRow className="border-b border-border">
                      <TableHead>
                        Nama Peserta Didik
                      </TableHead>
                      <TableHead className="w-56">
                        Status Kehadiran
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {students.map((s) => (
                      <TableRow key={s.id}>
                        <TableCell className="font-semibold text-foreground">{String(s.nama)}</TableCell>
                        <TableCell>
                          <Select
                            disabled={status === 'libur'}
                            value={marks[s.id] || 'Hadir'}
                            onChange={(e) => setMarks({ ...marks, [s.id]: e.target.value })}
                          >
                            {['Hadir', 'Sakit', 'Izin', 'Alpa'].map((v) => (
                              <option key={v}>{v}</option>
                            ))}
                          </Select>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>

            {/* Step 2: Digital Signature with Auto Teacher Name (MANDATORY) */}
            <div className="space-y-3 pt-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground font-extrabold text-xs">
                    2
                  </span>
                  <h4 className="text-sm font-bold text-foreground">
                    Tanda Tangan Pengajar di Layar <span className="text-destructive font-bold">*WAJIB</span>
                  </h4>
                </div>
                {isSignatureValid ? (
                  <Badge variant="secondary" className="gap-1 text-success bg-success/10 border-success/30">
                    <CheckCircle2 className="h-3 w-3" /> Tanda Tangan Terisi
                  </Badge>
                ) : (
                  <Badge variant="outline" className="text-warning border-warning/30">
                    Wajib Tanda Tangan
                  </Badge>
                )}
              </div>
              <Signature value={signature} onChange={setSignature} userName={userName} />
            </div>

            {/* Step 3: Upload Bukti Pembelajaran KBM (MANDATORY Min 1, Max 5 Photos) */}
            <div className="space-y-3 pt-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground font-extrabold text-xs">
                    3
                  </span>
                  <h4 className="text-sm font-bold text-foreground">
                    Upload Bukti Pembelajaran KBM <span className="text-destructive font-bold">*WAJIB (Min 1, Max 5 Foto)</span>
                  </h4>
                </div>
                {isPhotosValid ? (
                  <Badge variant="secondary" className="gap-1 text-success bg-success/10 border-success/30">
                    <CheckCircle2 className="h-3 w-3" /> {photos.length} / 5 Foto Terunggah
                  </Badge>
                ) : (
                  <Badge variant="outline" className="text-destructive border-destructive/30">
                    Wajib Upload Min. 1 Foto
                  </Badge>
                )}
              </div>

              <div className="rounded-2xl border border-border bg-secondary/30 p-5 space-y-4">
                {/* Upload Dropzone */}
                {photos.length < 5 && (
                  <div className="flex items-center gap-3">
                    <label className={`flex items-center gap-2 rounded-xl border border-dashed border-primary/50 bg-primary/5 px-4 py-3 text-xs font-semibold text-primary transition-all ${processingPhotos ? 'cursor-wait opacity-60' : 'cursor-pointer hover:bg-primary/10'}`}>
                      <UploadCloud className="h-4 w-4" />
                      <span>{processingPhotos ? 'Memproses foto...' : 'Pilih Foto KBM...'}</span>
                      <input
                        type="file"
                        accept="image/jpeg,image/png,image/webp"
                        multiple
                        onChange={handlePhotoUpload}
                        disabled={processingPhotos}
                        className="hidden"
                      />
                    </label>
                    <span className="text-[11px] text-muted-foreground">
                      JPG, PNG, WEBP; maks. 15 MB/foto. Foto otomatis dikompres sebelum disimpan.
                    </span>
                  </div>
                )}

                {/* Photo Thumbnails Preview Grid */}
                {photos.length > 0 ? (
                  <div className="grid grid-cols-2 sm:grid-cols-5 gap-3 pt-2">
                    {photos.map((imgUrl, index) => (
                      <div
                        key={index}
                        className="group relative aspect-[4/3] rounded-xl border border-border bg-card overflow-hidden shadow-2xs transition-all hover:shadow-md"
                      >
                        <img
                          src={imgUrl}
                          alt={`Bukti KBM ${index + 1}`}
                          className="h-full w-full object-cover cursor-pointer transition-transform group-hover:scale-105"
                          onClick={() => setPreviewModalUrl(imgUrl)}
                        />
                        {!readOnly && (
                          <button
                            type="button"
                            onClick={() => handleRemovePhoto(index)}
                            className="absolute top-1.5 right-1.5 h-6 w-6 rounded-full bg-destructive text-destructive-foreground flex items-center justify-center shadow-md hover:bg-destructive/90 transition-colors"
                            title="Hapus foto ini"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        )}
                        <div className="absolute bottom-1 left-1.5 px-1.5 py-0.5 rounded bg-black/60 text-white text-[10px] font-semibold">
                          Foto #{index + 1}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="rounded-xl border border-dashed p-6 text-center text-xs text-muted-foreground bg-card">
                    <ImageIcon className="h-8 w-8 mx-auto mb-1 text-muted-foreground/50" />
                    Belum ada foto bukti KBM. Presensi tidak dapat disimpan tanpa mengunggah minimal 1 foto kegiatan KBM.
                  </div>
                )}
              </div>
            </div>

            {/* Validation Checklist Banner & Save Button */}
            <div className="rounded-2xl border border-border bg-card p-5 space-y-4 shadow-2xs">
              <div className="flex flex-wrap items-center justify-between gap-3 text-xs border-b border-border pb-3">
                <span className="font-bold text-foreground flex items-center gap-1.5">
                  <Info className="h-4 w-4 text-primary" /> Kelengkapan Syarat Simpan Presensi:
                </span>
                <div className="flex flex-wrap items-center gap-3">
                  <span className={students.length > 0 ? 'text-success font-semibold' : 'text-muted-foreground'}>
                    {students.length > 0 ? '✓ Checklist Siswa OK' : '❌ Tidak ada siswa'}
                  </span>
                  <span className={isSignatureValid ? 'text-success font-semibold' : 'text-destructive font-semibold'}>
                    {isSignatureValid ? '✓ Tanda Tangan OK' : '❌ Tanda Tangan Kosong'}
                  </span>
                  <span className={isPhotosValid ? 'text-success font-semibold' : 'text-destructive font-semibold'}>
                    {isPhotosValid ? `✓ Foto KBM (${photos.length}/5) OK` : '❌ Foto KBM Kosong (Min. 1)'}
                  </span>
                </div>
              </div>

              <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
                <div className="flex flex-col sm:flex-row gap-2">
                  <Button
                    disabled={readOnly || !students.length || submitting || processingPhotos || loadingMeeting || deleting}
                    onClick={() => void save()}
                    className={`h-11 rounded-xl px-6 font-bold shadow-2xs transition-all ${
                      isSignatureValid && isPhotosValid
                        ? 'bg-primary hover:bg-primary/90 text-primary-foreground'
                        : 'bg-primary/80 hover:bg-primary text-primary-foreground'
                    }`}
                  >
                    <Save className="h-4 w-4 mr-2" />{' '}
                    {loadingMeeting
                      ? 'Memuat presensi...'
                      : processingPhotos
                        ? 'Memproses foto...'
                        : submitting
                          ? selectedID === 'new' ? 'Menyimpan...' : 'Menyimpan perubahan...'
                          : selectedID === 'new' ? 'Simpan Presensi & Bukti KBM' : 'Simpan Perubahan'}
                  </Button>
                  {selectedID !== 'new' && !readOnly && (
                    <Button
                      type="button"
                      variant="destructive"
                      className="h-11 rounded-xl px-5 font-bold"
                      disabled={submitting || loadingMeeting || deleting}
                      onClick={() => setDeleteDialogOpen(true)}
                    >
                      <Trash2 className="h-4 w-4 mr-2" /> Hapus Presensi
                    </Button>
                  )}
                </div>
                {message && (
                  <Alert className="py-2.5 px-4 text-xs font-semibold">
                    <AlertDescription>{message}</AlertDescription>
                  </Alert>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <AttendanceRecap token={token} />

      {/* Image Preview Modal */}
      <Dialog open={!!previewModalUrl} onOpenChange={(open) => !open && setPreviewModalUrl(null)}>
        <DialogContent className="max-w-3xl gap-2 p-2">
          <DialogTitle className="sr-only">Pratinjau bukti KBM</DialogTitle>
          <DialogDescription className="sr-only">
            Gambar bukti kegiatan pembelajaran yang diunggah.
          </DialogDescription>
          {previewModalUrl && (
            <img
              src={previewModalUrl}
              alt="Preview Bukti KBM"
              className="max-h-[80vh] w-full rounded-xl object-contain"
            />
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteDialogOpen} onOpenChange={(open) => !deleting && setDeleteDialogOpen(open)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Data Presensi?</AlertDialogTitle>
            <AlertDialogDescription>
              Presensi tanggal <strong>{date}</strong>, termasuk checklist kehadiran siswa, tanda tangan, dan seluruh foto KBM akan dihapus permanen.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Batal</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => void deleteSelectedMeeting()}
              disabled={deleting}
            >
              {deleting ? 'Menghapus...' : 'Ya, Hapus Presensi'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid gap-2">
      <Label className="font-bold text-xs text-foreground">{label}</Label>
      {children}
    </div>
  )
}
