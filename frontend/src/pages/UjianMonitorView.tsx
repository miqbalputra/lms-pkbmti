import { useEffect, useState } from 'react'
import { Monitor, RefreshCw, Download, ClipboardCheck, X } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { EmptyState, PageToolbar } from '../components/ui/page'
import { Badge } from '../components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { downloadFile, request } from '../lib/api'
import { toast } from 'sonner'
import { formatWibDateTime } from '../lib/wib'

type Peserta = Record<string, unknown> & { id: string }
type ReviewQuestion = {
  ujianSoalId: string
  aktif: boolean
  jawabanId?: string
  tipe: string
  pertanyaan: string
  opsi?: string[]
  konfigurasi?: { rubrik?: Array<{ kriteria: string; maks: number }>; choices?: Array<{ id: string; text: string }>; correctIds?: string[]; acceptedAnswers?: string[]; statements?: Array<{ id: string; text: string; correct: boolean }>; pairs?: Record<string, string>; left?: Array<{ id: string; text: string }>; right?: Array<{ id: string; text: string }> }
  berkas?: Array<{ id: string; namaFile: string; ukuran: number }>
  kunci: string
  bobot: number
  jawaban: string
  nilai: number
  nilaiManual?: number | null
  komentarGuru: string
}
type AttemptReview = {
  attempt: { id: string; status: string; skor: number | null; pesertaDidik: { nama: string; nis: string } }
  soal: ReviewQuestion[]
}
type RecoveryReview = {
  id: string
  ujianPesertaId: string
  pesertaDidikId: string
  namaPeserta: string
  status: 'menunggu' | 'diterima' | 'ditolak'
  dibuatPada: string
  komentarPeninjau?: string
  jawaban: Array<{ ujianSoalId: string; urutan: number; pertanyaan: string; jawaban: string }>
}

function formatRecoveryAnswer(value: string) {
  try {
    const parsed = JSON.parse(value)
    if (Array.isArray(parsed)) return parsed.map((item) => typeof item === 'string' ? item : JSON.stringify(item)).join(' · ') || 'Tidak dijawab'
    if (parsed && typeof parsed === 'object') return Object.entries(parsed).map(([key, answer]) => `${key}: ${String(answer)}`).join(' · ') || 'Tidak dijawab'
    return parsed == null ? 'Tidak dijawab' : String(parsed)
  } catch { return value || 'Tidak dijawab' }
}

function formatQuestionKey(question: ReviewQuestion): string {
  if (question.kunci) return question.kunci
  const config = question.konfigurasi
  if (!config) return '—'
  if (config.correctIds?.length && config.choices?.length) return config.correctIds.map((id) => config.choices?.find((choice) => choice.id === id)?.text || id).join(' · ')
  if (config.statements?.length) return config.statements.map((row) => `${row.text}: ${row.correct ? 'Benar' : 'Salah'}`).join('\n')
  if (config.pairs && config.left?.length && config.right?.length) return config.left.map((row) => `${row.text} → ${config.right?.find((choice) => choice.id === config.pairs?.[row.id])?.text || '—'}`).join('\n')
  if (config.acceptedAnswers?.length) return config.acceptedAnswers.join(' · ')
  return 'Kunci tersedia di konfigurasi penilaian.'
}

export function UjianMonitorView({
  token,
}: {
  token: string
}) {
  const [ujians, setUjians] = useState<Peserta[]>([])
  const [selected, setSelected] = useState<string>('')
  const [pesertas, setPesertas] = useState<Peserta[]>([])
  const [loading, setLoading] = useState(false)
  const [review, setReview] = useState<AttemptReview | null>(null)
  const [reviewLoading, setReviewLoading] = useState(false)
  const [savingAnswer, setSavingAnswer] = useState('')
  const [manualDrafts, setManualDrafts] = useState<Record<string, { nilai: string; komentar: string }>>({})
  const [recoveries, setRecoveries] = useState<RecoveryReview[]>([])
  const [recoveryComments, setRecoveryComments] = useState<Record<string, string>>({})
  const [recoveryLoading, setRecoveryLoading] = useState(false)
  const [savingRecovery, setSavingRecovery] = useState('')

  useEffect(() => {
    request('/ujian', token)
      .then((d) => setUjians(Array.isArray(d) ? d : []))
      .catch(() => {})
  }, [token])

  const loadMonitor = (ujianId: string) => {
    setSelected(ujianId)
    setReview(null)
    setLoading(true)
    request(`/ujian-online/monitor/${ujianId}`, token)
      .then((d) => setPesertas(Array.isArray(d) ? d : []))
      .catch(() => setPesertas([]))
      .finally(() => setLoading(false))
    setRecoveryLoading(true)
    request(`/ujian-online/monitor/${ujianId}/recoveries`, token)
      .then((d) => setRecoveries(Array.isArray(d) ? d : []))
      .catch(() => setRecoveries([]))
      .finally(() => setRecoveryLoading(false))
  }

  const reviewRecovery = async (recovery: RecoveryReview, status: 'diterima' | 'ditolak') => {
    setSavingRecovery(recovery.id)
    try {
      await request(`/ujian-online/recoveries/${recovery.id}/review`, token, 'PUT', {
        status,
        komentar: recoveryComments[recovery.id] || '',
      })
      toast.success(status === 'diterima' ? 'Jawaban pemulihan diterima untuk catatan guru.' : 'Jawaban pemulihan ditolak.')
      const latest = await request(`/ujian-online/monitor/${selected}/recoveries`, token)
      setRecoveries(Array.isArray(latest) ? latest : [])
    } catch (error) {
      toast.error(String((error as Error).message || 'Gagal menyimpan tinjauan jawaban pemulihan.'))
    } finally {
      setSavingRecovery('')
    }
  }

  const exportResults = async () => {
    if (!selected) return
    try {
      await downloadFile(`/ujian/${selected}/export`, token, 'hasil-ujian.csv')
      toast.success('Hasil ujian berhasil diunduh.')
    } catch (error) {
      toast.error(String((error as Error).message || 'Gagal mengekspor hasil ujian.'))
    }
  }

  const exportResultsXlsx = async () => {
    if (!selected) return
    try {
      await downloadFile(`/ujian/${selected}/export?format=xlsx`, token, 'hasil-ujian.xlsx')
      toast.success('Laporan Excel Ujian Online berhasil diunduh.')
    } catch (error) {
      toast.error(String((error as Error).message || 'Gagal mengunduh laporan Excel Ujian Online.'))
    }
  }

  const openReview = async (attemptId: string) => {
    if (!selected) return
    setReviewLoading(true)
    try {
      const detail = await request(`/ujian-online/monitor/${selected}/attempt/${attemptId}`, token) as AttemptReview
      setReview(detail)
      setManualDrafts(Object.fromEntries((detail.soal || []).map((question) => [question.jawabanId || question.ujianSoalId, {
        nilai: question.nilaiManual == null ? String(question.nilai ?? 0) : String(question.nilaiManual),
        komentar: question.komentarGuru || '',
      }])))
    } catch (error) {
      toast.error(String((error as Error).message || 'Gagal memuat jawaban peserta.'))
    } finally {
      setReviewLoading(false)
    }
  }

  const saveManualGrade = async (question: ReviewQuestion) => {
    if (!selected || !review || !question.jawabanId) return
    const draft = manualDrafts[question.jawabanId] || { nilai: '', komentar: '' }
    const nilai = Number(draft.nilai)
    if (!Number.isFinite(nilai) || nilai < 0 || nilai > question.bobot) {
      toast.error(`Nilai harus antara 0 dan ${question.bobot}.`)
      return
    }
    setSavingAnswer(question.jawabanId)
    try {
      await request(`/ujian-online/monitor/${selected}/attempt/${review.attempt.id}/answer/${question.jawabanId}/grade`, token, 'POST', {
        nilai,
        komentar: draft.komentar,
      })
      toast.success('Penilaian uraian tersimpan.')
      await loadMonitor(selected)
      await openReview(review.attempt.id)
    } catch (error) {
      toast.error(String((error as Error).message || 'Gagal menyimpan penilaian.'))
    } finally {
      setSavingAnswer('')
    }
  }

  const fmt = (v: unknown) => formatWibDateTime(v) || '-'

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Monitor Ujian Online"
        description="Pantau status pengerjaan ujian online siswa secara real-time."
        actions={
          selected ? (
            <div className="flex gap-2">
              <Button variant="outline" onClick={exportResults}>
                <Download className="h-4 w-4" /> Export CSV
              </Button>
              <Button variant="outline" onClick={exportResultsXlsx}>
                <Download className="h-4 w-4" /> Export XLSX
              </Button>
              <Button variant="outline" onClick={() => loadMonitor(selected)}>
                <RefreshCw className="h-4 w-4" /> Refresh
              </Button>
            </div>
          ) : undefined
        }
      />

      <Card className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
        <div className="mb-4">
          <label className="text-sm font-semibold text-foreground">Pilih Ujian</label>
          <select
            className="mt-1 block w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            value={selected}
            onChange={(e) => loadMonitor(e.target.value)}
          >
            <option value="">-- Pilih ujian --</option>
            {ujians.map((u) => (
              <option key={u.id} value={u.id}>
                {String(u.judul)} — {String((u.mapel as Record<string, unknown>)?.namaMapel || '')}
              </option>
            ))}
          </select>
        </div>

        {loading && (
          <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
            <div className="h-4 w-4 rounded-full border-2 border-primary border-t-transparent animate-spin" />
            Memuat data monitoring...
          </div>
        )}

        {!loading && selected && pesertas.length === 0 && (
          <EmptyState title="Belum ada peserta" description="Belum ada siswa yang mengerjakan ujian ini." />
        )}

        {!loading && pesertas.length > 0 && (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>No</TableHead>
                  <TableHead>Nama Siswa</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Skor</TableHead>
                  <TableHead>Tab Switch</TableHead>
                  <TableHead>Mulai</TableHead>
                  <TableHead>Selesai</TableHead>
                  <TableHead>Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pesertas.map((p, i) => {
                  const pd = p.pesertaDidik as Record<string, unknown> | undefined
                  return (
                    <TableRow key={p.id}>
                      <TableCell>{i + 1}</TableCell>
                      <TableCell className="font-medium">{String(pd?.nama || '-')}</TableCell>
                      <TableCell>
                        <Badge variant={p.status === 'selesai' ? 'default' : 'secondary'}>
                          {p.status === 'menunggu_nilai' ? `Menunggu nilai (${Number(p.uraianMenunggu || 0)})` : p.status === 'dikunci' && Number(p.uraianMenunggu || 0) > 0 ? `Dikunci · menunggu nilai (${Number(p.uraianMenunggu)})` : String(p.status)}
                        </Badge>
                      </TableCell>
                      <TableCell>{p.skor != null ? Number(p.skor).toFixed(1) : '-'}</TableCell>
                      <TableCell>{Number(p.tabSwitch || 0)}</TableCell>
                      <TableCell className="text-xs">{fmt(p.mulai)}</TableCell>
                      <TableCell className="text-xs">{fmt(p.selesai)}</TableCell>
                      <TableCell>
                        {p.status !== 'mulai' && (
                          <Button variant="outline" size="sm" onClick={() => void openReview(p.id)}>
                            <ClipboardCheck className="h-4 w-4" /> Jawaban
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}

        {selected && (
          <section className="mt-6 space-y-3 border-t border-border pt-5" aria-labelledby="recovery-review-title">
            <div>
              <h2 id="recovery-review-title" className="text-lg font-semibold">Jawaban lokal untuk ditinjau</h2>
              <p className="text-sm text-muted-foreground">Jawaban ini terlambat tersinkron dan tidak mengubah nilai otomatis. Keputusan guru hanya menjadi catatan tinjauan.</p>
            </div>
            {recoveryLoading ? <p role="status" className="py-3 text-sm text-muted-foreground">Memuat jawaban pemulihan…</p> : recoveries.length === 0 ? <p className="rounded-xl border border-dashed border-border p-4 text-sm text-muted-foreground">Tidak ada jawaban pemulihan untuk ditinjau.</p> : recoveries.map((recovery) => (
              <article key={recovery.id} className="space-y-3 rounded-xl border border-border bg-background p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <h3 className="font-semibold">{recovery.namaPeserta || `Siswa ${recovery.pesertaDidikId}`}</h3>
                    <p className="text-xs text-muted-foreground">Dikirim {fmt(recovery.dibuatPada)} · Sesi {recovery.ujianPesertaId.slice(0, 8)}</p>
                  </div>
                  <Badge variant={recovery.status === 'menunggu' ? 'secondary' : 'default'}>{recovery.status === 'menunggu' ? 'Menunggu tinjauan' : recovery.status === 'diterima' ? 'Diterima guru' : 'Ditolak guru'}</Badge>
                </div>
                <div className="space-y-2">
                  {recovery.jawaban.map((answer, index) => (
                    <div key={`${answer.ujianSoalId}-${index}`} className="rounded-lg bg-muted/40 p-3 text-sm">
                      <p className="font-medium">Soal {answer.urutan || index + 1}: {answer.pertanyaan || 'Isi soal tidak tersedia'}</p>
                      <p className="mt-1 whitespace-pre-wrap text-muted-foreground">Jawaban yang diajukan: {formatRecoveryAnswer(answer.jawaban)}</p>
                    </div>
                  ))}
                </div>
                {recovery.status === 'menunggu' ? (
                  <div className="space-y-2">
                    <label className="block space-y-1 text-sm font-medium">Catatan tinjauan (opsional)
                      <textarea className="min-h-20 w-full rounded-lg border border-border bg-background px-3 py-2 font-normal" maxLength={4000} value={recoveryComments[recovery.id] || ''} onChange={(event) => setRecoveryComments((current) => ({ ...current, [recovery.id]: event.target.value }))} />
                    </label>
                    <div className="flex flex-wrap gap-2">
                      <Button variant="outline" className="min-h-11" disabled={savingRecovery === recovery.id} onClick={() => void reviewRecovery(recovery, 'diterima')}>{savingRecovery === recovery.id ? 'Menyimpan…' : 'Terima sebagai catatan'}</Button>
                      <Button variant="outline" className="min-h-11" disabled={savingRecovery === recovery.id} onClick={() => void reviewRecovery(recovery, 'ditolak')}>Tolak kiriman</Button>
                    </div>
                  </div>
                ) : recovery.komentarPeninjau ? <p className="rounded-lg bg-muted/40 p-3 text-sm">Catatan guru: {recovery.komentarPeninjau}</p> : null}
              </article>
            ))}
          </section>
        )}

        {reviewLoading && <div className="py-6 text-sm text-muted-foreground" role="status">Memuat jawaban peserta…</div>}
        {review && !reviewLoading && (
          <section className="mt-6 space-y-4 border-t border-border pt-5" aria-labelledby="attempt-review-title">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 id="attempt-review-title" className="text-lg font-semibold">Jawaban {review.attempt.pesertaDidik.nama}</h2>
                <p className="text-sm text-muted-foreground">NIS {review.attempt.pesertaDidik.nis || '—'} · Status: {review.attempt.status}{review.attempt.skor == null ? ' · Nilai menunggu penilaian' : ` · Nilai ${Number(review.attempt.skor).toFixed(1)}`}</p>
              </div>
              <Button variant="ghost" size="sm" aria-label="Tutup jawaban" onClick={() => setReview(null)}><X className="h-4 w-4" /></Button>
            </div>
            {review.soal.map((question, index) => {
              const draftKey = question.jawabanId || question.ujianSoalId
              const draft = manualDrafts[draftKey] || { nilai: '0', komentar: '' }
              const isEssay = ['essay', 'uraian', 'paragraf'].includes(question.tipe.toLowerCase())
              return (
                <article key={question.ujianSoalId} className={`space-y-3 rounded-xl border border-border bg-background p-4 ${question.aktif === false ? 'border-dashed opacity-75' : ''}`}>
                  <div className="flex flex-wrap justify-between gap-2">
                    <h3 className="font-medium">Soal {index + 1} {question.aktif === false && <Badge variant="secondary" className="ml-2">Dilewati · tidak dinilai</Badge>} <span className="text-xs font-normal text-muted-foreground">· {question.tipe} · bobot {question.bobot}</span></h3>
                    <span className="text-sm text-muted-foreground">Nilai tersimpan: {Number(question.nilai || 0).toFixed(2)}</span>
                  </div>
                  <p className="whitespace-pre-wrap text-sm">{question.pertanyaan}</p>
                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="rounded-lg bg-muted/40 p-3 text-sm"><strong>Jawaban siswa</strong><p className="mt-1 whitespace-pre-wrap">{question.jawaban || 'Tidak dijawab'}</p></div>
                    <div className="rounded-lg bg-muted/40 p-3 text-sm"><strong>Kunci jawaban (staf)</strong><p className="mt-1 whitespace-pre-wrap">{formatQuestionKey(question)}</p></div>
                  </div>
                  {question.konfigurasi?.rubrik?.length ? <div className="rounded-lg border border-primary/20 bg-primary/5 p-3 text-sm"><strong>Rubrik penilaian</strong><ul className="mt-2 space-y-1">{question.konfigurasi.rubrik.map((rubric, rubricIndex) => <li key={`${rubric.kriteria}-${rubricIndex}`} className="flex justify-between gap-3"><span>{rubric.kriteria}</span><span className="shrink-0 font-medium">maks. {rubric.maks}</span></li>)}</ul><p className="mt-2 text-xs text-muted-foreground">Nilai akhir tetap dibatasi oleh bobot soal ({question.bobot} poin).</p></div> : null}
                  {question.berkas?.length ? <div className="rounded-lg border p-3"><strong className="text-sm">Berkas jawaban</strong><div className="mt-2 flex flex-wrap gap-2">{question.berkas.map((file) => <Button key={file.id} size="sm" variant="outline" onClick={() => void downloadFile(`/ujian-online/monitor/${selected}/attempt/${review.attempt.id}/file/${file.id}`, token, file.namaFile)}>{file.namaFile} · {(file.ukuran / 1048576).toFixed(2)} MB</Button>)}</div></div> : null}
                  {question.opsi?.length ? <p className="text-xs text-muted-foreground">Opsi: {question.opsi.join(' · ')}</p> : null}
                  {isEssay && question.aktif !== false && (
                    <div className="grid gap-3 sm:grid-cols-[140px_1fr_auto] sm:items-end">
                      <label className="space-y-1 text-sm font-medium">Nilai (maks. {question.bobot})
                        <input className="block min-h-11 w-full rounded-lg border border-border bg-background px-3" type="number" min="0" max={question.bobot} step="0.01" value={draft.nilai} onChange={(event) => setManualDrafts((current) => ({ ...current, [draftKey]: { ...draft, nilai: event.target.value } }))} />
                      </label>
                      <label className="space-y-1 text-sm font-medium">Komentar untuk siswa
                        <input className="block min-h-11 w-full rounded-lg border border-border bg-background px-3" maxLength={4000} value={draft.komentar} onChange={(event) => setManualDrafts((current) => ({ ...current, [draftKey]: { ...draft, komentar: event.target.value } }))} placeholder="Opsional" />
                      </label>
                      <Button disabled={!question.jawabanId || savingAnswer === question.jawabanId} onClick={() => void saveManualGrade(question)}>
                        {savingAnswer === question.jawabanId ? 'Menyimpan…' : 'Simpan nilai'}
                      </Button>
                    </div>
                  )}
                </article>
              )
            })}
          </section>
        )}

        {!selected && !loading && (
          <EmptyState
            icon={<Monitor className="h-12 w-12 text-muted-foreground/40" />}
            title="Pilih ujian untuk dimonitor"
            description="Pilih ujian dari daftar di atas untuk melihat status pengerjaan siswa."
          />
        )}
      </Card>
    </div>
  )
}
