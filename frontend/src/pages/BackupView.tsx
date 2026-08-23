import { useEffect, useRef, useState } from 'react'
import { BookOpen, ChevronDown, ChevronRight, Copy, Database, Download, FileUp, HardDriveDownload, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '../components/ui/alert'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Label } from '../components/ui/label'
import { PageToolbar } from '../components/ui/page'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { request, apiBase } from '../lib/api'
import { formatWibDateTime } from '../lib/wib'

type Backup = {
  name: string
  size: number
  modTime: string
  format: 'db' | 'sql'
  automatic: boolean
}

function formatBytes(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB'
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

function formatTime(s: string): string {
  return formatWibDateTime(s) || s
}

// Download a binary backup file (the read endpoints accept the admin JWT).
async function downloadBinary(path: string, token: string, fallback: string) {
  const r = await fetch(apiBase + path, { credentials: 'include', headers: { Authorization: `Bearer ${token}` } })
  if (!r.ok) {
    const x = await r.json().catch(() => ({}))
    throw new Error((x as { error?: string })?.error || 'Gagal mengunduh file backup')
  }
  const blob = await r.blob()
  const cd = r.headers.get('Content-Disposition') || ''
  const m = /filename="?([^";]+)"?/.exec(cd)
  const name = m?.[1] || fallback
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export function BackupView({ token }: { token: string }) {
  const [backups, setBackups] = useState<Backup[]>([])
  const [dir, setDir] = useState('backups')
  const [dialect, setDialect] = useState('sqlite')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState<string | null>(null) // 'db' | 'sql' | 'restore' | null
  const [restoreMsg, setRestoreMsg] = useState('')
  const [showGuide, setShowGuide] = useState(true)
  const fileRef = useRef<HTMLInputElement | null>(null)

  const isPG = dialect === 'postgresql'

  function copyCode(code: string) {
    void navigator.clipboard.writeText(code).then(
      () => toast.success('Disalin ke clipboard.'),
      () => toast.error('Gagal menyalin — salin manual.'),
    )
  }

  async function load() {
    setLoading(true)
    try {
      const r = (await request('/backup', token)) as { dir: string; backups: Backup[]; dialect: string }
      setDir(r.dir || 'backups')
      setBackups(r.backups || [])
      setDialect(r.dialect || 'sqlite')
    } catch (e: unknown) {
      toast.error(String((e as Error).message || 'Gagal memuat daftar backup'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  async function createBackup(format: 'db' | 'sql') {
    const actualFormat = isPG ? 'sql' : format
    setBusy(format)
    try {
      const r = (await request('/backup?format=' + actualFormat, token, 'POST')) as { name: string }
      toast.success(`Backup ${isPG ? 'pg_dump' : actualFormat.toUpperCase()} dibuat: ${r.name}`)
      void load()
    } catch (e: unknown) {
      toast.error(String((e as Error).message || 'Gagal membuat backup'))
    } finally {
      setBusy(null)
    }
  }

  async function downloadFresh(format: 'db' | 'sql') {
    setBusy('dl-' + format)
    try {
      await downloadBinary('/backup/download?format=' + format, token, `pkbm-lms.${format}`)
      toast.success('Backup diunduh.')
    } catch (e: unknown) {
      toast.error(String((e as Error).message || 'Gagal mengunduh'))
    } finally {
      setBusy(null)
    }
  }

  async function downloadFullBackup() {
    setBusy('full')
    try {
      await downloadBinary('/backup/download?format=full', token, isPG ? 'pkbm-lms-full.sql' : 'pkbm-lms-full.db')
      toast.success('Database full berhasil diunduh.')
    } catch (e: unknown) {
      toast.error(String((e as Error).message || 'Gagal mengunduh database full'))
    } finally {
      setBusy(null)
    }
  }

  async function downloadExisting(name: string) {
    setBusy('dl-' + name)
    try {
      await downloadBinary('/backup/file/' + encodeURIComponent(name), token, name)
    } catch (e: unknown) {
      toast.error(String((e as Error).message || 'Gagal mengunduh'))
    } finally {
      setBusy(null)
    }
  }

  async function removeBackup(name: string) {
    if (!confirm(`Hapus file backup "${name}"? Tindakan ini tidak dapat dibatalkan.`)) return
    setBusy('del-' + name)
    try {
      await request('/backup/' + encodeURIComponent(name), token, 'DELETE')
      toast.success('File backup dihapus.')
      void load()
    } catch (e: unknown) {
      toast.error(String((e as Error).message || 'Gagal menghapus'))
    } finally {
      setBusy(null)
    }
  }

  async function uploadRestore(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const f = fileRef.current?.files?.[0]
    if (!f) {
      toast.error('Pilih file backup (.sql) terlebih dahulu.')
      return
    }
    const ext = f.name.slice(f.name.lastIndexOf('.')).toLowerCase()
    const innerExt = ext === '.enc' ? f.name.slice(0, -4).slice(f.name.slice(0, -4).lastIndexOf('.')).toLowerCase() : ext
    if (isPG && innerExt !== '.sql') {
      toast.error('PostgreSQL hanya menerima file .sql atau .sql.enc')
      return
    }
    if (!isPG && innerExt !== '.db' && innerExt !== '.sql') {
      toast.error('File harus berekstensi .db, .sql, .db.enc, atau .sql.enc')
      return
    }
    if (!confirm('Restore akan mengganti seluruh isi database dengan file ini. Database saat ini akan diamankan terlebih dahulu. Lanjutkan?')) return
    setBusy('restore')
    setRestoreMsg('')
    try {
      const fd = new FormData()
      fd.append('file', f)
      const r = await fetch(apiBase + '/backup/restore', {
        method: 'POST',
        credentials: 'include',
        headers: { Authorization: `Bearer ${token}` },
        body: fd,
      })
      const x = (await r.json().catch(() => ({}))) as { error?: string; message?: string; restartScheduled?: boolean }
      if (!r.ok) throw new Error(x.error || 'Gagal menyiapkan restore')
      setRestoreMsg(x.message || 'Restore berhasil.')
      toast.success(isPG ? 'Restore PostgreSQL berhasil diterapkan.' : 'Restore disiapkan — restart server untuk menerapkan.')
      if (fileRef.current) fileRef.current.value = ''
    } catch (err: unknown) {
      toast.error(String((err as Error).message || 'Gagal restore'))
    } finally {
      setBusy(null)
    }
  }

  const n8nUrl = `${apiBase}/backup/download?format=full&key=YOUR_BACKUP_API_KEY`

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Backup & Restore Database"
        description={isPG
          ? "Unduh database PostgreSQL penuh via pg_dump (.sql) atau restore cukup dengan upload file."
          : "Unduh database SQLite penuh (.db) sekali klik, lalu restore cukup dengan upload file backup."
        }
      />

      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs space-y-3">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold">Buat Backup Baru</h2>
            <p className="text-xs text-muted-foreground">
              {isPG ? (
                <>Format <strong>.sql</strong> = dump PostgreSQL via <code>pg_dump</code> (portable, bisa direstore ke instance PG lain).</>
              ) : (
                <>Format <strong>.db</strong> = snapshot biner (paling cepat &amp; restore paling mudah). Format <strong>.sql</strong> = text portable (bisa dibaca/diff, untuk arsip n8n).</>
              )}
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={load} disabled={loading}>
            <RefreshCw className="h-4 w-4" /> Segarkan
          </Button>
        </div>
        <div className="rounded-xl border border-primary/20 bg-primary/5 p-3 flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-sm font-semibold">Download Database Full</p>
            <p className="text-xs text-muted-foreground">Satu file lengkap untuk dipindahkan atau dipakai restore.</p>
          </div>
          <Button onClick={downloadFullBackup} disabled={busy === 'full'}>
            <HardDriveDownload className="h-4 w-4" /> {busy === 'full' ? 'Mengunduh...' : 'Download Database Full'}
          </Button>
        </div>
        <div className="flex flex-wrap gap-2">
          {!isPG && (
            <Button onClick={() => createBackup('db')} disabled={busy === 'db'}>
              <Database className="h-4 w-4" /> {busy === 'db' ? 'Memproses...' : 'Backup .db (biner)'}
            </Button>
          )}
          <Button onClick={() => createBackup('sql')} disabled={busy === 'sql'} variant={isPG ? 'default' : 'outline'}>
            <Plus className="h-4 w-4" /> {busy === 'sql' ? 'Memproses...' : isPG ? 'Backup pg_dump (.sql)' : 'Backup .sql (text)'}
          </Button>
          {!isPG && (
            <Button onClick={() => downloadFresh('db')} disabled={busy === 'dl-db'} variant="ghost">
              <HardDriveDownload className="h-4 w-4" /> Unduh .db langsung
            </Button>
          )}
          <Button onClick={() => downloadFresh('sql')} disabled={busy === 'dl-sql'} variant="ghost">
            <Download className="h-4 w-4" /> Unduh .sql langsung
          </Button>
        </div>
      </Card>

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <div className="p-4 border-b border-border">
          <h2 className="text-sm font-semibold">Daftar File Backup</h2>
          <p className="text-xs text-muted-foreground">Disimpan di folder: <code className="text-foreground">{dir}</code></p>
        </div>
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              <TableHead>Nama File</TableHead>
              <TableHead>Format</TableHead>
              <TableHead>Ukuran</TableHead>
              <TableHead>Waktu</TableHead>
              <TableHead>Jenis</TableHead>
              <TableHead className="text-right">Aksi</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {backups.map((b) => (
              <TableRow key={b.name}>
                <TableCell className="font-mono text-xs">{b.name}</TableCell>
                <TableCell><Badge variant={b.format === 'db' ? 'default' : 'secondary'}>{b.format}</Badge></TableCell>
                <TableCell>{formatBytes(b.size)}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{formatTime(b.modTime)}</TableCell>
                <TableCell>
                  <Badge variant={b.automatic ? 'default' : 'outline'}>{b.automatic ? 'otomatis' : 'manual'}</Badge>
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    <Button size="sm" variant="outline" onClick={() => downloadExisting(b.name)} disabled={busy === 'dl-' + b.name}>
                      <Download className="h-3.5 w-3.5" /> Unduh
                    </Button>
                    <Button size="sm" variant="destructive" aria-label="Hapus" onClick={() => removeBackup(b.name)} disabled={busy === 'del-' + b.name}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {!backups.length && (
              <TableRow>
                <TableCell colSpan={6} className="text-sm text-muted-foreground text-center py-6">
                  Belum ada file backup. Buat satu di atas, atau aktifkan penjadwalan otomatis via <code>BACKUP_CRON</code>.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card className="rounded-2xl border border-border bg-card p-4 shadow-2xs space-y-3">
        <div>
          <h2 className="text-sm font-semibold flex items-center gap-2"><FileUp className="h-4 w-4" /> Restore (Pemulihan)</h2>
          <p className="text-xs text-muted-foreground">
            {isPG ? (
              <>Unggah file backup <code>.sql</code> atau backup terenkripsi <code>.sql.enc</code>. Restore PostgreSQL <strong>diterapkan langsung</strong> ke database tanpa restart.</>
            ) : (
              <>Unggah file backup <code>.db</code>, <code>.sql</code>, atau versi terenkripsinya (<code>.db.enc</code>/<code>.sql.enc</code>). Restore diterapkan pada <strong>restart server berikutnya</strong> — DB saat ini otomatis disalin ke <code>backups/pre-restore-*</code> sebagai pengaman.</>
            )}
          </p>
          {!isPG && <p className="text-xs font-medium text-primary">Di production, setelah upload server akan restart otomatis dan restore diterapkan.</p>}
        </div>
        <form className="flex flex-wrap items-end gap-3" onSubmit={uploadRestore}>
          <div className="grid gap-1.5 flex-1 min-w-[240px]">
            <Label className="text-xs">File backup {isPG ? '(.sql / .sql.enc)' : '(.db / .sql / .enc)'}</Label>
            <input ref={fileRef} type="file" accept={isPG ? '.sql,.sql.enc' : '.db,.sql,.db.enc,.sql.enc'} className="text-sm file:mr-3 file:rounded-lg file:border file:border-border file:bg-secondary file:px-3 file:py-1.5 file:text-sm" />
          </div>
          <Button type="submit" disabled={busy === 'restore'}>
            <FileUp className="h-4 w-4" /> {busy === 'restore' ? 'Memproses...' : 'Restore Sekarang'}
          </Button>
        </form>
        {restoreMsg && (
          <Alert>
            <AlertDescription>{restoreMsg}</AlertDescription>
          </Alert>
        )}
      </Card>

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <button
          onClick={() => setShowGuide((v) => !v)}
          className="flex w-full items-center justify-between gap-3 p-4 text-left hover:bg-secondary/40"
        >
          <span className="flex items-center gap-2 text-sm font-semibold">
            <BookOpen className="h-4 w-4" /> Panduan Lengkap Backup &amp; Restore
          </span>
          <span className="flex items-center gap-2 text-xs text-muted-foreground">
            {showGuide ? 'Sembunyikan' : 'Tampilkan'}
            {showGuide ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          </span>
        </button>

        {showGuide && (
          <div className="space-y-6 border-t border-border p-4 text-sm">
            {/* A. Ikhtisar */}
            <section className="space-y-2">
              <h3 className="text-sm font-semibold">A. Ikhtisar</h3>
              <p className="text-xs text-muted-foreground leading-relaxed">
                Sistem ini mencadangkan <strong>seluruh database</strong> sesuai engine: SQLite memakai snapshot
                <code>.db</code>/<code>.sql</code>, sedangkan PostgreSQL memakai dump penuh <code>.sql</code> via
                <code>pg_dump</code>. Backup bisa dibuat manual, terjadwal otomatis (env <code>BACKUP_CRON</code>),
                atau ditarik dari luar oleh <strong>n8n</strong> via HTTP.
              </p>
              <div className="grid gap-2 sm:grid-cols-2">
                <div className="rounded-lg border border-border bg-secondary/30 p-3 text-xs">
                  <p className="font-semibold">Format <code>.db</code> (biner)</p>
                  <ul className="mt-1 list-disc pl-4 space-y-0.5 text-muted-foreground">
                    <li>Snapshot atomik via <code>VACUUM INTO</code> — konsisten &amp; cepat.</li>
                    <li>Restore paling mudah: tukar file → restart.</li>
                    <li>Tidak bisa dibaca/diff sebagai teks.</li>
                  </ul>
                </div>
                {!isPG && <div className="rounded-lg border border-border bg-secondary/30 p-3 text-xs">
                  <p className="font-semibold">Format <code>.sql</code> (text)</p>
                  <ul className="mt-1 list-disc pl-4 space-y-0.5 text-muted-foreground">
                    <li>Dump portable mengikuti format <code>sqlite3 .dump</code>.</li>
                    <li>Bisa dibaca, diff, dan dipindahkan antar mesin.</li>
                    <li>Cocok untuk arsip n8n ke Google Drive/S3.</li>
                  </ul>
                </div>}
                {isPG && <div className="rounded-lg border border-border bg-secondary/30 p-3 text-xs sm:col-span-2">
                  <p className="font-semibold">Format <code>.sql</code> (PostgreSQL)</p>
                  <ul className="mt-1 list-disc pl-4 space-y-0.5 text-muted-foreground">
                    <li>Dump penuh portable dibuat melalui <code>pg_dump</code>.</li>
                    <li>Restore memakai <code>psql</code> dalam satu transaksi dengan backup pengaman.</li>
                    <li>Bisa disimpan oleh n8n ke Google Drive/S3.</li>
                  </ul>
                </div>}
              </div>
            </section>

            {/* B. Backup manual */}
            <section className="space-y-2">
              <h3 className="text-sm font-semibold">B. Backup Manual</h3>
              <ol className="list-decimal pl-5 space-y-1 text-xs text-muted-foreground">
                <li>Buka kartu <strong>"Buat Backup Baru"</strong> di atas.</li>
                <li>Klik tombol backup sesuai engine. PostgreSQL memakai <strong>"Backup pg_dump (.sql)"</strong>; SQLite menyediakan <strong>"Backup .db (biner)"</strong> dan <strong>"Backup .sql (text)"</strong>.</li>
                <li>Untuk satu file database lengkap, klik <strong>"Download Database Full"</strong>. File langsung masuk ke folder Download browser.</li>
                <li>Unduh file lama via tombol <em>Unduh</em> pada baris tabel; hapus via tombol tong sampah.</li>
              </ol>
            </section>

            {/* C. Backup otomatis */}
            <section className="space-y-2">
              <h3 className="text-sm font-semibold">C. Backup Otomatis Terjadwal</h3>
              <p className="text-xs text-muted-foreground">
                Set <code>BACKUP_CRON</code> (cron WIB) di environment server lalu restart. Saat kosong, backup
                sepenuhnya andalkan n8n (tidak ada penjadwalan internal).
              </p>
              <CodeBlock copy={copyCode} code={'# Contoh .env / variabel server\nBACKUP_CRON="0 2 * * *"      # tiap 02:00 WIB\nBACKUP_FORMAT="full"      # full | db | sql\nBACKUP_RETENTION="14"     # simpan 14 backup otomatis terbaru\nBACKUP_DIR="backups"'} />
              <p className="text-xs text-muted-foreground">
                Contoh cron lain: <code>0 2 * * 0</code> (Minggu 02:00), <code>0 */6 * * *</code> (tiap 6 jam). File
                otomatis diberi label <Badge variant="default">otomatis</Badge> dan di-prune sesuai retensi; file
                manual &amp; pre-restore pengaman tidak pernah dihapus otomatis.
              </p>
              <p className="text-xs text-muted-foreground">
                Untuk arsip offsite, isi <code>BACKUP_OFFSITE_URL</code> dan <code>BACKUP_ENCRYPTION_KEY</code>.
                Sistem mengenkripsi backup sebelum mengirimnya ke presigned S3, gateway n8n, atau endpoint internal.
                Simpan kunci enkripsi di secret manager karena tanpa kunci file <code>.enc</code> tidak dapat direstore.
              </p>
              <p className="text-xs text-muted-foreground">
                Untuk restore drill PostgreSQL, isi <code>BACKUP_DRILL_DATABASE_URL</code> dengan database disposable.
                Backup terjadwal akan diuji dengan <code>psql</code> di database tersebut sebelum dicatat sebagai berhasil.
              </p>
            </section>

            {/* D. Integrasi n8n */}
            <section className="space-y-2">
              <h3 className="text-sm font-semibold">D. Integrasi n8n (otomatisasi)</h3>
              <ol className="list-decimal pl-5 space-y-1 text-xs text-muted-foreground">
                <li>Set <code>BACKUP_API_KEY</code> di env server (gunakan string panjang &amp; acak), lalu restart.</li>
                <li>Di n8n, buat workflow: <strong>Schedule Trigger</strong> → <strong>HTTP Request</strong> → node penyimpanan.</li>
                <li>Konfigurasi node <strong>HTTP Request</strong>:
                  <ul className="list-disc pl-5 mt-1 space-y-0.5">
                    <li><strong>Method:</strong> <code>GET</code></li>
                    <li><strong>URL:</strong> <code>http://&lt;host&gt;:8080/api/backup/download?format=full&amp;key=YOUR_BACKUP_API_KEY</code> (ganti host &amp; key)</li>
                    <li><strong>Response:</strong> <code>File</code> (binary) — agar n8n menerima file</li>
                    <li>Atau kirim header <code>X-Backup-Key: YOUR_BACKUP_API_KEY</code> sebagai ganti <code>?key=</code></li>
                  </ul>
                </li>
                <li>Hubungkan ke node penyimpanan: <strong>Write Binary File</strong>, <strong>Google Drive</strong>, atau <strong>AWS S3</strong>.</li>
                <li>Format text: ganti <code>format=db</code> → <code>format=sql</code>.</li>
              </ol>
              <CodeBlock copy={copyCode} code={n8nUrl} />
              <p className="text-xs text-muted-foreground">
                Endpoint baca lain (juga menerima key): <code>GET /api/backup</code> (daftar file) dan
                <code> GET /api/backup/file/&lt;nama&gt;</code> (unduh file tertentu). Endpoint tulis (buat/hapus/restore)
                <strong> hanya menerima JWT admin</strong> — bukan key — demi keamanan.
              </p>
            </section>

            {/* E. Restore */}
            <section className="space-y-2">
              <h3 className="text-sm font-semibold">E. Restore (Pemulihan)</h3>
              {isPG && (
                <Alert>
                  <AlertDescription>
                    Restore PostgreSQL membuat backup pengaman <code>pre-restore-&lt;waktu&gt;.sql</code> terlebih dahulu,
                    lalu menerapkan dump upload dalam satu transaksi ke database production.
                  </AlertDescription>
                </Alert>
              )}
              {!isPG && <Alert>
                <AlertDescription>
                  Restore <strong>tidak</strong> langsung menimpa DB yang sedang jalan (file terkunci). Ia disiapkan
                  lalu diterapkan saat <strong>restart</strong> berikutnya. DB lama otomatis disalin ke
                  <code> backups/pre-restore-&lt;waktu&gt;.db</code> sebagai pengaman.
                </AlertDescription>
              </Alert>}
              {isPG && <ol className="list-decimal pl-5 space-y-1 text-xs text-muted-foreground">
                <li>Pilih file <code>.sql</code>, lalu klik <strong>"Restore Sekarang"</strong>.</li>
                <li>Server memvalidasi file dan membuat backup pengaman sebelum restore.</li>
                <li><code>psql</code> menerapkan dump dalam satu transaksi tanpa restart aplikasi.</li>
                <li>Selesai. Verifikasi data di dashboard.</li>
              </ol>}
              {!isPG && <ol className="list-decimal pl-5 space-y-1 text-xs text-muted-foreground">
                <li>Pilih file <code>.db</code> atau <code>.sql</code>, lalu klik <strong>"Restore Sekarang"</strong>.</li>
                <li>Server memvalidasi file terlebih dahulu dan menyiapkan restore secara aman.</li>
                <li>Di production, server restart otomatis dan restore diterapkan sebelum database dibuka.</li>
                <li>Saat startup, <code>applyPendingRestore</code> membackup DB lama → lalu menukar file (.db) atau rebuild dari dump (.sql).</li>
                <li>Selesai. Verifikasi data di dashboard.</li>
              </ol>}
              {!isPG && <p className="text-xs text-muted-foreground">
                <strong>Membatalkan restore</strong> bila salah: hentikan server → rename
                <code> backups/pre-restore-*.db</code> menjadi <code>pkbm-lms.db</code> (hapus dulu file live) → start
                ulang. Atau hapus file <code>.restore-pending*</code> sebelum restart untuk membatalkan sebelum diterapkan.
              </p>}
            </section>

            {/* F. Env vars */}
            <section className="space-y-2">
              <h3 className="text-sm font-semibold">F. Variabel Environment</h3>
              <div className="overflow-x-auto rounded-lg border border-border">
                <table className="w-full text-xs">
                  <thead className="bg-secondary/40">
                    <tr className="text-left">
                      <th className="p-2 font-semibold">Variabel</th>
                      <th className="p-2 font-semibold">Default</th>
                      <th className="p-2 font-semibold">Keterangan</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    <tr><td className="p-2 font-mono">BACKUP_API_KEY</td><td className="p-2 text-muted-foreground">—</td><td className="p-2 text-muted-foreground">Kunci statis untuk n8n (tanpa JWT). Wajib di-set agar key aktif.</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_CRON</td><td className="p-2 text-muted-foreground">— (off)</td><td className="p-2 text-muted-foreground">Cron WIB. Kosong = andalkan n8n/manual.</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_FORMAT</td><td className="p-2 text-muted-foreground">full</td><td className="p-2 text-muted-foreground">full | db | sql</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_AUTO_RESTART</td><td className="p-2 text-muted-foreground">true (production)</td><td className="p-2 text-muted-foreground">Restart otomatis setelah upload restore.</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_MAX_UPLOAD_MB</td><td className="p-2 text-muted-foreground">512</td><td className="p-2 text-muted-foreground">Batas ukuran file restore.</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_RETENTION</td><td className="p-2 text-muted-foreground">14</td><td className="p-2 text-muted-foreground">Jumlah backup otomatis (-auto-) yang disimpan.</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_DIR</td><td className="p-2 text-muted-foreground">backups</td><td className="p-2 text-muted-foreground">Folder penyimpanan backup.</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_OFFSITE_URL</td><td className="p-2 text-muted-foreground">—</td><td className="p-2 text-muted-foreground">Endpoint S3 presigned, n8n, atau storage gateway untuk upload terenkripsi.</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_ENCRYPTION_KEY</td><td className="p-2 text-muted-foreground">—</td><td className="p-2 text-muted-foreground">Kunci enkripsi backup offsite; wajib disimpan di secret manager.</td></tr>
                    <tr><td className="p-2 font-mono">BACKUP_DRILL_DATABASE_URL</td><td className="p-2 text-muted-foreground">—</td><td className="p-2 text-muted-foreground">Database disposable untuk menguji restore PostgreSQL terjadwal.</td></tr>
                  </tbody>
                </table>
              </div>
            </section>

            {/* G. Keamanan & troubleshooting */}
            <section className="space-y-2">
              <h3 className="text-sm font-semibold">G. Keamanan &amp; Troubleshooting</h3>
              <ul className="list-disc pl-5 space-y-1 text-xs text-muted-foreground">
                <li><strong>Jangan commit</strong> <code>BACKUP_API_KEY</code>; simpan di env/secret manager. Gunakan string panjang &amp; acak.</li>
                <li>Endpoint <strong>restore/create/delete</strong> hanya menerima <strong>JWT admin</strong> — bukan key statis — agar otomatisasi tak bisa merusak.</li>
                <li>File backup <code>.db</code> berisi <strong>data sensitif</strong> (kredensial hash, NIK). Simpan di tempat terbatas; enkripsi saat di cloud.</li>
                <li><strong>"database is locked"</strong> sudah ditangani <code>WAL + busy_timeout</code> — writer menunggu, bukan gagal. Jika muncul, periksa apakah ada proses lain (mis. sqlite3 CLI) memegang lock lama.</li>
                <li>Restore gagal? Lihat log startup (<code>RESTORE applied ...</code> atau error). DB lama tetap utuh di <code>backups/pre-restore-*</code>.</li>
                <li>File sementara unduh (<code>pkbm-dl-*</code>) ditulis di <em>OS temp dir</em>, bukan folder backup.</li>
                <li>Backup/restore mendukung <strong>SQLite</strong> dan <strong>PostgreSQL</strong>. PostgreSQL menggunakan <code>pg_dump</code>/<code>psql</code> — restore diterapkan langsung tanpa restart.</li>
              </ul>
            </section>
          </div>
        )}
      </Card>
    </div>
  )
}

// CodeBlock renders a code snippet with a copy button for the in-app guide.
function CodeBlock({ code, copy }: { code: string; copy: (s: string) => void }) {
  return (
    <div className="relative">
      <pre className="block rounded-lg border border-border bg-secondary/40 p-2 pr-10 text-xs overflow-x-auto whitespace-pre-wrap break-all">
        <code>{code}</code>
      </pre>
      <button
        type="button"
        onClick={() => copy(code)}
        className="absolute right-1.5 top-1.5 inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary"
        aria-label="Salin"
        title="Salin"
      >
        <Copy className="h-3.5 w-3.5" />
      </button>
    </div>
  )
}
