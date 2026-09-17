import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import { Download, Pencil, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { EmptyState, FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { request } from '../lib/api'
import { formatWibDate } from '../lib/wib'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
type Row = Record<string, unknown> & { id: string }

const portfolioBlank = { pesertaDidikId: '', mapelId: '', kompetensiId: '', judul: '', tipeBukti: '', komentarTutor: '', rubrik: '', statusPublikasi: 'draf' }
const followUpBlank = { pesertaDidikId: '', jurnalId: '', mapelId: '', kompetensiId: '', statusKetuntasan: 'penguatan', rencana: '', penanggungJawab: '', tenggat: '', hasil: '', selesaiAt: '', ringkasanOrangTua: '', statusPublikasi: 'draf' }

function classLabel(row: Row): string { return `Kelas ${String(row.jenjang || '')}${String(row.namaRombel || '')}` }
function publicationLabel(value: unknown): string { return value === 'dipublikasikan' ? 'Diterbitkan' : 'Draf internal' }
function masteryLabel(value: unknown): string { return value === 'tuntas' ? 'Tuntas' : value === 'remedial' ? 'Remedial' : 'Penguatan' }

export function PerkembanganBelajarView({ token, readOnly }: { token: string; readOnly: boolean }) {
  const [kelas, setKelas] = useState<Row[]>([])
  const [students, setStudents] = useState<Row[]>([])
  const [mapel, setMapel] = useState<Row[]>([])
  const [classID, setClassID] = useState('')
  const [studentID, setStudentID] = useState('')
  const [portfolio, setPortfolio] = useState<Row[]>([])
  const [followUps, setFollowUps] = useState<Row[]>([])
  const [dashboard, setDashboard] = useState<{ total: number; items: Row[]; tanpaPortofolio: Row[]; seringAbsen: Row[]; periodeKehadiran: string }>({ total: 0, items: [], tanpaPortofolio: [], seringAbsen: [], periodeKehadiran: '30 hari terakhir' })
  const [kompetensi, setKompetensi] = useState<Row[]>([])
  const [journals, setJournals] = useState<Row[]>([])
  const [portfolioOpen, setPortfolioOpen] = useState(false)
  const [portfolioEdit, setPortfolioEdit] = useState<Row | null>(null)
  const [portfolioForm, setPortfolioForm] = useState({ ...portfolioBlank })
  const [portfolioFile, setPortfolioFile] = useState<File | null>(null)
  const [followOpen, setFollowOpen] = useState(false)
  const [followEdit, setFollowEdit] = useState<Row | null>(null)
  const [followForm, setFollowForm] = useState({ ...followUpBlank })
  const [saving, setSaving] = useState(false)

  const visibleStudents = useMemo(() => students.filter((student) => !classID || String(student.kelasId || '') === classID), [students, classID])

  const load = useCallback(() => {
    const query = studentID ? `?pesertaDidikId=${encodeURIComponent(studentID)}` : classID ? `?kelasId=${encodeURIComponent(classID)}` : ''
    void request('/portofolio' + query, token).then((value: Row[]) => setPortfolio(Array.isArray(value) ? value : [])).catch(() => setPortfolio([]))
    void request('/tindak-lanjut-belajar' + (studentID ? `?pesertaDidikId=${encodeURIComponent(studentID)}` : ''), token).then((value: Row[]) => setFollowUps(Array.isArray(value) ? value : [])).catch(() => setFollowUps([]))
    void request('/tindak-lanjut-belajar/dashboard', token).then((value: { total?: number; items?: Row[]; tanpaPortofolio?: Row[]; seringAbsen?: Row[]; periodeKehadiran?: string }) => setDashboard({ total: Number(value?.total || 0), items: Array.isArray(value?.items) ? value.items : [], tanpaPortofolio: Array.isArray(value?.tanpaPortofolio) ? value.tanpaPortofolio : [], seringAbsen: Array.isArray(value?.seringAbsen) ? value.seringAbsen : [], periodeKehadiran: String(value?.periodeKehadiran || '30 hari terakhir') })).catch(() => setDashboard({ total: 0, items: [], tanpaPortofolio: [], seringAbsen: [], periodeKehadiran: '30 hari terakhir' }))
  }, [classID, studentID, token])

  useEffect(() => {
    void request('/kelas', token).then((value: Row[]) => setKelas(Array.isArray(value) ? value : [])).catch(() => setKelas([]))
    void request('/peserta-didik', token).then((value: Row[]) => setStudents(Array.isArray(value) ? value : [])).catch(() => setStudents([]))
    void request('/mapel', token).then((value: Row[]) => setMapel(Array.isArray(value) ? value : [])).catch(() => setMapel([]))
    void request('/kompetensi', token).then((value: Row[]) => setKompetensi(Array.isArray(value) ? value : [])).catch(() => setKompetensi([]))
  }, [token])
  useEffect(() => { load() }, [load])
  useEffect(() => {
    if (studentID && !visibleStudents.some((student) => student.id === studentID)) setStudentID('')
  }, [studentID, visibleStudents])
  useEffect(() => {
    if (!classID) { setJournals([]); return }
    void request(`/jurnal?kelasId=${encodeURIComponent(classID)}`, token).then((value: Row[]) => setJournals(Array.isArray(value) ? value : [])).catch(() => setJournals([]))
  }, [classID, token])

  function openPortfolio(row?: Row) {
    setPortfolioEdit(row || null)
    setPortfolioFile(null)
    setPortfolioForm(row ? {
      pesertaDidikId: String(row.pesertaDidikId || ''), mapelId: String(row.mapelId || ''), kompetensiId: String(row.kompetensiId || ''), judul: String(row.judul || ''), tipeBukti: String(row.tipeBukti || ''), komentarTutor: String(row.komentarTutor || ''), rubrik: String(row.rubrik || ''), statusPublikasi: String(row.statusPublikasi || 'draf'),
    } : { ...portfolioBlank, pesertaDidikId: studentID })
    setPortfolioOpen(true)
  }

  function openFollowUp(row?: Row) {
    setFollowEdit(row || null)
    setFollowForm(row ? {
      pesertaDidikId: String(row.pesertaDidikId || ''), jurnalId: String(row.jurnalId || ''), mapelId: String(row.mapelId || ''), kompetensiId: String(row.kompetensiId || ''), statusKetuntasan: String(row.statusKetuntasan || 'penguatan'), rencana: String(row.rencana || ''), penanggungJawab: String(row.penanggungJawab || ''), tenggat: String(row.tenggat || '').slice(0, 10), hasil: String(row.hasil || ''), selesaiAt: String(row.selesaiAt || '').slice(0, 10), ringkasanOrangTua: String(row.ringkasanOrangTua || ''), statusPublikasi: String(row.statusPublikasi || 'draf'),
    } : { ...followUpBlank, pesertaDidikId: studentID })
    setFollowOpen(true)
  }

  async function savePortfolio(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!portfolioForm.pesertaDidikId || !portfolioForm.mapelId || !portfolioForm.judul || (!portfolioEdit && !portfolioFile)) { toast.error('Anak, mapel, judul, dan berkas bukti wajib diisi.'); return }
    setSaving(true)
    try {
      const data = new FormData()
      Object.entries(portfolioForm).forEach(([key, value]) => { if (value) data.append(key, String(value)) })
      if (portfolioFile) data.append('file', portfolioFile)
      const response = await fetch(`${apiBase}/portofolio${portfolioEdit ? `/${portfolioEdit.id}` : ''}`, { method: portfolioEdit ? 'PUT' : 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}` }, body: data })
      const result = await response.json().catch(() => ({}))
      if (!response.ok) throw new Error((result as { error?: string }).error || 'Portofolio gagal disimpan.')
      toast.success(portfolioEdit ? 'Portofolio diperbarui.' : 'Portofolio disimpan.')
      setPortfolioOpen(false); setPortfolioEdit(null); load()
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Portofolio gagal disimpan.') } finally { setSaving(false) }
  }

  async function saveFollowUp(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!followForm.pesertaDidikId || !followForm.rencana) { toast.error('Anak dan rencana tindak lanjut wajib diisi.'); return }
    setSaving(true)
    try {
      const response = await fetch(`${apiBase}/tindak-lanjut-belajar${followEdit ? `/${followEdit.id}` : ''}`, { method: followEdit ? 'PUT' : 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ ...followForm, jurnalId: followForm.jurnalId || undefined, kompetensiId: followForm.kompetensiId || undefined }) })
      const result = await response.json().catch(() => ({}))
      if (!response.ok) throw new Error((result as { error?: string }).error || 'Tindak lanjut gagal disimpan.')
      toast.success(followEdit ? 'Tindak lanjut diperbarui.' : 'Tindak lanjut dibuat.')
      setFollowOpen(false); setFollowEdit(null); load()
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Tindak lanjut gagal disimpan.') } finally { setSaving(false) }
  }

  async function removePortfolio(row: Row) {
    if (!window.confirm(`Hapus portofolio "${String(row.judul || '')}"? Berkasnya juga akan dihapus.`)) return
    try { await request(`/portofolio/${row.id}`, token, 'DELETE'); toast.success('Portofolio dihapus.'); load() } catch (error) { toast.error(error instanceof Error ? error.message : 'Portofolio gagal dihapus.') }
  }

  async function downloadPortfolio(row: Row) {
    try {
      const response = await fetch(`${apiBase}/portofolio/${row.id}/download`, { credentials: 'include', headers: { Authorization: `Bearer ${token}` } })
      if (!response.ok) throw new Error('Berkas tidak tersedia.')
      const blob = await response.blob(); const url = URL.createObjectURL(blob); const anchor = document.createElement('a')
      anchor.href = url; anchor.download = String(row.fileName || row.judul || 'portofolio'); anchor.click(); URL.revokeObjectURL(url)
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Berkas gagal diunduh.') }
  }

  return <div className="space-y-5">
    <PageToolbar title="Perkembangan Belajar" description="Portofolio karya, ketuntasan, penguatan, dan remedial peserta didik." actions={!readOnly && <div className="flex flex-wrap gap-2"><Button variant="outline" onClick={() => openFollowUp()}><Plus className="h-4 w-4" /> Tindak lanjut</Button><Button onClick={() => openPortfolio()}><Plus className="h-4 w-4" /> Portofolio</Button></div>} />

    <Card className="grid gap-3 p-4 md:grid-cols-2">
      <div className="grid gap-2"><Label>Kelas / Rombel</Label><Select value={classID} onChange={(event) => { setClassID(event.target.value); setStudentID('') }}><option value="">Semua kelas yang dapat diakses</option>{kelas.map((row) => <option key={row.id} value={row.id}>{classLabel(row)}</option>)}</Select></div>
      <div className="grid gap-2"><Label>Peserta didik</Label><Select value={studentID} onChange={(event) => setStudentID(event.target.value)}><option value="">Semua peserta didik</option>{visibleStudents.map((row) => <option key={row.id} value={row.id}>{String(row.nama || '-')}</option>)}</Select></div>
    </Card>

    <Card className="grid gap-4 border-amber-200 bg-amber-50 p-4 text-amber-950 md:grid-cols-3 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-100"><div><p className="text-sm">Butuh perhatian</p><p className="text-2xl font-bold">{dashboard.total} anak</p><p className="mt-1 text-xs">Gabungan remedial/penguatan, belum ada bukti, atau sering absen.</p></div><div><p className="text-sm font-semibold">{dashboard.items.length} tindak lanjut terbuka</p><p className="mt-1 text-xs">Remedial dan penguatan yang perlu diperbarui sebelum rapor.</p></div><div><p className="text-sm font-semibold">{dashboard.tanpaPortofolio.length} tanpa portofolio · {dashboard.seringAbsen.length} sering absen</p><p className="mt-1 text-xs">Frekuensi absen dihitung dari {dashboard.periodeKehadiran}.</p></div></Card>

    {portfolioOpen && !readOnly && <FormCard title={portfolioEdit ? 'Edit portofolio' : 'Tambah portofolio'} description="Unggah foto karya, audio membaca, video praktik, atau berkas. Hanya yang diterbitkan terlihat oleh orang tua."><form className="grid gap-4 md:grid-cols-2" onSubmit={savePortfolio}>
      <div className="grid gap-2"><Label>Peserta didik</Label><Select value={portfolioForm.pesertaDidikId} onChange={(event) => setPortfolioForm({ ...portfolioForm, pesertaDidikId: event.target.value })} required><option value="">Pilih anak</option>{visibleStudents.map((row) => <option key={row.id} value={row.id}>{String(row.nama || '-')}</option>)}</Select></div>
      <div className="grid gap-2"><Label>Mata pelajaran</Label><Select value={portfolioForm.mapelId} onChange={(event) => setPortfolioForm({ ...portfolioForm, mapelId: event.target.value, kompetensiId: '' })} required><option value="">Pilih mapel</option>{mapel.map((row) => <option key={row.id} value={row.id}>{String(row.namaMapel || '-')}</option>)}</Select></div>
      <div className="grid gap-2 md:col-span-2"><Label>Judul bukti belajar</Label><Input value={portfolioForm.judul} onChange={(event) => setPortfolioForm({ ...portfolioForm, judul: event.target.value })} required /></div>
      <div className="grid gap-2"><Label>Kompetensi (opsional)</Label><Select value={portfolioForm.kompetensiId} onChange={(event) => setPortfolioForm({ ...portfolioForm, kompetensiId: event.target.value })}><option value="">— Tanpa kompetensi —</option>{kompetensi.map((row) => <option key={row.id} value={row.id}>{String(row.nama || '-')}</option>)}</Select></div>
      <div className="grid gap-2"><Label>Jenis bukti</Label><Select value={portfolioForm.tipeBukti} onChange={(event) => setPortfolioForm({ ...portfolioForm, tipeBukti: event.target.value })}><option value="">Deteksi dari berkas</option><option value="foto">Foto karya</option><option value="audio">Audio membaca</option><option value="video">Video praktik</option><option value="berkas">Berkas</option></Select></div>
      <div className="grid gap-2"><Label>Berkas {portfolioEdit ? '(opsional)' : ''}</Label><Input type="file" accept=".pdf,.doc,.docx,.png,.jpg,.jpeg,.mp3,.m4a,.wav,.mp4" onChange={(event) => setPortfolioFile(event.target.files?.[0] || null)} /></div>
      <div className="grid gap-2"><Label>Publikasi</Label><Select value={portfolioForm.statusPublikasi} onChange={(event) => setPortfolioForm({ ...portfolioForm, statusPublikasi: event.target.value })}><option value="draf">Draf internal</option><option value="dipublikasikan">Terbitkan ke orang tua</option></Select></div>
      <div className="grid gap-2"><Label>Komentar tutor</Label><textarea className="min-h-24 rounded-lg border border-input bg-background px-3 py-2 text-sm" value={portfolioForm.komentarTutor} onChange={(event) => setPortfolioForm({ ...portfolioForm, komentarTutor: event.target.value })} /></div>
      <div className="grid gap-2"><Label>Rubrik / penilaian singkat</Label><textarea className="min-h-24 rounded-lg border border-input bg-background px-3 py-2 text-sm" value={portfolioForm.rubrik} onChange={(event) => setPortfolioForm({ ...portfolioForm, rubrik: event.target.value })} /></div>
      <div className="flex gap-2 md:col-span-2"><Button type="submit" disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan portofolio'}</Button><Button type="button" variant="outline" onClick={() => setPortfolioOpen(false)}>Batal</Button></div>
    </form></FormCard>}

    {followOpen && !readOnly && <FormCard title={followEdit ? 'Edit tindak lanjut' : 'Tindak lanjut belajar'} description="Tandai ketuntasan, penguatan, atau remedial dan terbitkan ringkasannya bila perlu."><form className="grid gap-4 md:grid-cols-2" onSubmit={saveFollowUp}>
      <div className="grid gap-2"><Label>Peserta didik</Label><Select value={followForm.pesertaDidikId} onChange={(event) => setFollowForm({ ...followForm, pesertaDidikId: event.target.value })} required><option value="">Pilih anak</option>{visibleStudents.map((row) => <option key={row.id} value={row.id}>{String(row.nama || '-')}</option>)}</Select></div>
      <div className="grid gap-2"><Label>Status</Label><Select value={followForm.statusKetuntasan} onChange={(event) => setFollowForm({ ...followForm, statusKetuntasan: event.target.value })}><option value="tuntas">Tuntas</option><option value="penguatan">Perlu penguatan</option><option value="remedial">Perlu remedial</option></Select></div>
      <div className="grid gap-2"><Label>Mata pelajaran</Label><Select value={followForm.mapelId} onChange={(event) => setFollowForm({ ...followForm, mapelId: event.target.value, kompetensiId: '', jurnalId: '' })} required><option value="">Pilih mapel</option>{mapel.map((row) => <option key={row.id} value={row.id}>{String(row.namaMapel || '-')}</option>)}</Select></div>
      <div className="grid gap-2"><Label>Kompetensi (opsional)</Label><Select value={followForm.kompetensiId} onChange={(event) => setFollowForm({ ...followForm, kompetensiId: event.target.value })}><option value="">— Tanpa kompetensi —</option>{kompetensi.filter((row) => !followForm.mapelId || String(row.mapelId || '') === followForm.mapelId).map((row) => <option key={row.id} value={row.id}>{String(row.nama || '-')}</option>)}</Select></div>
      <div className="grid gap-2"><Label>Jurnal terkait (opsional)</Label><Select value={followForm.jurnalId} onChange={(event) => setFollowForm({ ...followForm, jurnalId: event.target.value })}><option value="">— Tanpa jurnal —</option>{journals.filter((row) => !followForm.mapelId || String(row.mapelId || '') === followForm.mapelId).map((row) => <option key={row.id} value={row.id}>{String((row.mapel as Row | undefined)?.namaMapel || 'Mapel')} · {String(row.materi || '-')}</option>)}</Select></div>
      <div className="grid gap-2"><Label>Penanggung jawab</Label><Input value={followForm.penanggungJawab} placeholder="Tutor / orang tua / bersama" onChange={(event) => setFollowForm({ ...followForm, penanggungJawab: event.target.value })} /></div>
      <div className="grid gap-2"><Label>Tenggat</Label><Input type="date" value={followForm.tenggat} onChange={(event) => setFollowForm({ ...followForm, tenggat: event.target.value })} /></div>
      <div className="grid gap-2"><Label>Selesai pada</Label><Input type="date" value={followForm.selesaiAt} onChange={(event) => setFollowForm({ ...followForm, selesaiAt: event.target.value })} /></div>
      <div className="grid gap-2 md:col-span-2"><Label>Rencana tindak lanjut</Label><textarea className="min-h-24 rounded-lg border border-input bg-background px-3 py-2 text-sm" value={followForm.rencana} onChange={(event) => setFollowForm({ ...followForm, rencana: event.target.value })} required /></div>
      <div className="grid gap-2"><Label>Hasil</Label><textarea className="min-h-20 rounded-lg border border-input bg-background px-3 py-2 text-sm" value={followForm.hasil} onChange={(event) => setFollowForm({ ...followForm, hasil: event.target.value })} /></div>
      <div className="grid gap-2"><Label>Ringkasan untuk orang tua</Label><textarea className="min-h-20 rounded-lg border border-input bg-background px-3 py-2 text-sm" value={followForm.ringkasanOrangTua} onChange={(event) => setFollowForm({ ...followForm, ringkasanOrangTua: event.target.value })} /></div>
      <div className="grid gap-2"><Label>Publikasi</Label><Select value={followForm.statusPublikasi} onChange={(event) => setFollowForm({ ...followForm, statusPublikasi: event.target.value })}><option value="draf">Draf internal</option><option value="dipublikasikan">Terbitkan ke orang tua</option></Select></div>
      <div className="flex gap-2 md:col-span-2"><Button type="submit" disabled={saving}>{saving ? 'Menyimpan...' : 'Simpan tindak lanjut'}</Button><Button type="button" variant="outline" onClick={() => setFollowOpen(false)}>Batal</Button></div>
    </form></FormCard>}

    <Card className="overflow-hidden"><div className="border-b p-4"><h2 className="font-bold">Portofolio belajar</h2></div><Table><TableHeader><TableRow><TableHead>Anak</TableHead><TableHead>Judul</TableHead><TableHead>Mapel</TableHead><TableHead>Publikasi</TableHead><TableHead className="text-right">Aksi</TableHead></TableRow></TableHeader><TableBody>{portfolio.map((row) => <TableRow key={row.id}><TableCell>{String((row.pesertaDidik as Row | undefined)?.nama || '-')}</TableCell><TableCell><p className="font-medium">{String(row.judul || '-')}</p><p className="text-xs text-muted-foreground">{String(row.fileName || '')}</p></TableCell><TableCell>{String((row.mapel as Row | undefined)?.namaMapel || '-')}</TableCell><TableCell>{publicationLabel(row.statusPublikasi)}</TableCell><TableCell className="text-right"><div className="flex justify-end gap-1"><Button size="icon" variant="ghost" onClick={() => void downloadPortfolio(row)} aria-label="Unduh"><Download className="h-4 w-4" /></Button>{!readOnly && <><Button size="icon" variant="ghost" onClick={() => openPortfolio(row)} aria-label="Edit"><Pencil className="h-4 w-4" /></Button><Button size="icon" variant="ghost" onClick={() => void removePortfolio(row)} aria-label="Hapus"><Trash2 className="h-4 w-4" /></Button></>}</div></TableCell></TableRow>)}{!portfolio.length && <EmptyState colSpan={5} title="Belum ada portofolio" description="Unggah bukti belajar anak dari kegiatan kelas." />}</TableBody></Table></Card>

    <Card className="overflow-hidden"><div className="border-b p-4"><h2 className="font-bold">Ketuntasan, penguatan, dan remedial</h2></div><Table><TableHeader><TableRow><TableHead>Anak</TableHead><TableHead>Mapel</TableHead><TableHead>Status</TableHead><TableHead>Rencana</TableHead><TableHead>Tenggat</TableHead><TableHead>Publikasi</TableHead><TableHead className="text-right">Aksi</TableHead></TableRow></TableHeader><TableBody>{followUps.map((row) => <TableRow key={row.id}><TableCell>{String((row.pesertaDidik as Row | undefined)?.nama || '-')}</TableCell><TableCell>{String((row.mapel as Row | undefined)?.namaMapel || '-')}</TableCell><TableCell>{masteryLabel(row.statusKetuntasan)}</TableCell><TableCell className="max-w-md whitespace-pre-wrap">{String(row.rencana || '-')}</TableCell><TableCell>{row.tenggat ? formatWibDate(row.tenggat) : '—'}</TableCell><TableCell>{publicationLabel(row.statusPublikasi)}</TableCell><TableCell className="text-right">{!readOnly && <Button size="sm" variant="ghost" onClick={() => openFollowUp(row)}><Pencil className="h-4 w-4" /> Edit</Button>}</TableCell></TableRow>)}{!followUps.length && <EmptyState colSpan={7} title="Belum ada tindak lanjut" description="Catat anak yang perlu penguatan atau remedial." />}</TableBody></Table></Card>
  </div>
}
