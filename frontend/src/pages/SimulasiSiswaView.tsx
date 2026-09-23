import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { AlertCircle, ArrowLeft, ArrowRight, CheckCircle2, Clock3, Flag, Grid2X2, Info, Link2, LogOut, Play, Save, Send, Wifi, WifiOff, X } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { QuestionAnswerControl, StimulusContent, type FontScale } from '../components/simulasi/QuestionAnswerControl'
import { apiBase, request } from '../lib/api'

type Row = Record<string, any> & { id: string }
function parse(value: any, fallback: any) { try { return typeof value === 'string' ? JSON.parse(value) : value ?? fallback } catch { return fallback } }
function clock(seconds: number) { const safe = Math.max(0, seconds); return `${String(Math.floor(safe / 60)).padStart(2, '0')}:${String(safe % 60).padStart(2, '0')}` }
function answered(value: any) { if (value === null || value === undefined || value === '') return false; if (Array.isArray(value)) return value.length > 0; return typeof value === 'object' ? Object.keys(value).length > 0 : true }
function scalePx(scale: FontScale) { return scale === 'small' ? 15 : scale === 'large' ? 20 : 17 }

export function SimulasiSiswaView({ token, onLogout }: { token: string; onLogout: () => void }) {
  const shareToken = useMemo(() => new URLSearchParams(window.location.search).get('share') || '', [])
  const sharedHeaders = shareToken ? { 'X-Simulasi-Share-Token': shareToken } : undefined
  const sharedRequest = (path: string, method = 'GET', body?: unknown) => request(path, token, method, body, undefined, sharedHeaders)
  const [rows, setRows] = useState<Row[]>([])
  const [instruction, setInstruction] = useState<Row | null>(null)
  const [workspace, setWorkspace] = useState<Row | null>(null)
  const [result, setResult] = useState<Row | null>(null)
  const [index, setIndex] = useState(0)
  const [answers, setAnswers] = useState<Record<string, any>>({})
  const [flagged, setFlagged] = useState<Record<string, boolean>>({})
  const [remaining, setRemaining] = useState(0)
  const [saveState, setSaveState] = useState('')
  const [loading, setLoading] = useState(true)
  const [online, setOnline] = useState(() => typeof navigator === 'undefined' ? true : navigator.onLine)
  const [fontScale, setFontScale] = useState<FontScale>(() => { try { return (localStorage.getItem('simulasi-font-scale') as FontScale) || 'medium' } catch { return 'medium' } })
  const [showInfo, setShowInfo] = useState(false)
  const [showPalette, setShowPalette] = useState(false)
  const [confirmSubmit, setConfirmSubmit] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const activeAttemptRef = useRef('')
  const remainingRef = useRef(remaining)
  const submitRef = useRef<(confirmed?: boolean, timedOut?: boolean) => Promise<void>>(async () => {})
  const submittingRef = useRef(false)
  const answerSequenceRef = useRef(0)
  const pendingAnswersRef = useRef(new Map<string, { value: any; sequence: number }>())
  const pendingFlagsRef = useRef(new Map<string, { value: boolean; sequence: number }>())
  const saveTimerRef = useRef<number | undefined>(undefined)
  const saveInFlightRef = useRef<{ attemptId: string; promise: Promise<boolean> } | null>(null)
  const flushAnswerQueueRef = useRef<(attemptId?: string) => Promise<boolean>>(async () => true)
  remainingRef.current = remaining

  const load = () => sharedRequest('/simulasi/saya').then((data) => setRows(Array.isArray(data) ? data : [])).catch((error) => toast.error(error.message || 'Daftar simulasi gagal dimuat.')).finally(() => setLoading(false))
  useEffect(() => { void load() }, [token, shareToken]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { try { localStorage.setItem('simulasi-font-scale', fontScale) } catch { /* storage optional */ } }, [fontScale])
  useEffect(() => {
    const onlineHandler = () => {
      setOnline(true)
      const attemptId = activeAttemptRef.current
      if (!attemptId) return
      if (remainingRef.current <= 0) {
        setSaveState('Memeriksa status waktu…')
        void submitRef.current(true, true)
        return
      }
      setSaveState('Menyinkronkan jawaban…')
      void flushAnswerQueueRef.current(attemptId)
    }
    const offlineHandler = () => setOnline(false)
    window.addEventListener('online', onlineHandler)
    window.addEventListener('offline', offlineHandler)
    return () => { window.removeEventListener('online', onlineHandler); window.removeEventListener('offline', offlineHandler) }
  }, [])
  useEffect(() => { if (!workspace || remaining <= 0) return; const timer = window.setInterval(() => setRemaining((value) => Math.max(0, value - 1)), 1000); return () => window.clearInterval(timer) }, [workspace, remaining])
  useEffect(() => { if (!workspace || remaining !== 0) return; void submit(true, true) }, [remaining, workspace]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!workspace) return
    const listener = (event: KeyboardEvent) => {
      if ((event.target as HTMLElement)?.tagName === 'TEXTAREA' || (event.target as HTMLElement)?.tagName === 'INPUT' || (event.target as HTMLElement)?.tagName === 'SELECT') return
      if (event.key === 'ArrowLeft') setIndex((value) => Math.max(0, value - 1))
      if (event.key === 'ArrowRight') setIndex((value) => Math.min((workspace.soal?.length || 1) - 1, value + 1))
      if (event.key.toLowerCase() === 'm') setShowPalette(true)
    }
    window.addEventListener('keydown', listener)
    return () => window.removeEventListener('keydown', listener)
  }, [workspace])

  async function openInstruction(packet: Row) { try { setInstruction(await sharedRequest(`/simulasi/saya/paket/${packet.id}/instruksi`)); setResult(null) } catch (error: any) { toast.error(error.message || 'Instruksi tidak tersedia.') } }
  async function start() { if (!instruction) return; try { const attempt = await sharedRequest(`/simulasi/saya/paket/${instruction.paket.id}/mulai`, 'POST'); await openWorkspace(attempt.id) } catch (error: any) { toast.error(error.message || 'Simulasi tidak dapat dimulai.') } }
  async function openWorkspace(id: string) {
    try {
      const data = await sharedRequest(`/simulasi/saya/upaya/${id}`)
      activeAttemptRef.current = id
      pendingAnswersRef.current.clear(); pendingFlagsRef.current.clear()
      let saved: Record<string, any> = {}
      try { saved = parse(sessionStorage.getItem(`simulasi-pending:${id}`), {}) } catch { /* storage optional */ }
      for (const item of Array.isArray(saved.answers) ? saved.answers : []) if (item?.id && item.pending && Object.prototype.hasOwnProperty.call(item.pending, 'value')) pendingAnswersRef.current.set(item.id, item.pending)
      for (const item of Array.isArray(saved.flags) ? saved.flags : []) if (item?.id && item.pending && typeof item.pending.value === 'boolean') pendingFlagsRef.current.set(item.id, item.pending)
      answerSequenceRef.current = Number(saved.sequence) || 0
      setWorkspace(data); setInstruction(null); setResult(null); setIndex(0); setRemaining(Number(data.sisaDetik || 0)); setSaveState('Autosave aktif')
      const initialAnswers: Record<string, any> = {}; const initialFlags: Record<string, boolean> = {}
      for (const question of data.soal || []) { initialAnswers[question.upayaSoalId] = parse(question.jawaban, null); initialFlags[question.upayaSoalId] = Boolean(question.ditandai) }
      pendingAnswersRef.current.forEach((pending, questionId) => { initialAnswers[questionId] = pending.value })
      pendingFlagsRef.current.forEach((pending, questionId) => { initialFlags[questionId] = pending.value })
      setAnswers(initialAnswers); setFlagged(initialFlags)
      if (pendingAnswersRef.current.size || pendingFlagsRef.current.size) void flushAnswerQueue(id)
    } catch (error: any) { toast.error(error.message || 'Upaya tidak dapat dilanjutkan.') }
  }
  function persistPending(attemptId = activeAttemptRef.current) {
    if (!attemptId) return
    try {
      sessionStorage.setItem(`simulasi-pending:${attemptId}`, JSON.stringify({
        sequence: answerSequenceRef.current,
        answers: [...pendingAnswersRef.current].map(([id, pending]) => ({ id, pending })),
        flags: [...pendingFlagsRef.current].map(([id, pending]) => ({ id, pending })),
      }))
    } catch { /* storage may be unavailable; server autosave remains active */ }
  }
  async function flushAnswerQueue(attemptId = activeAttemptRef.current): Promise<boolean> {
    if (!attemptId) return true
    if (saveInFlightRef.current) {
      if (saveInFlightRef.current.attemptId === attemptId) return saveInFlightRef.current.promise
      await saveInFlightRef.current.promise
      return flushAnswerQueue(attemptId)
    }
    const sync = (async () => {
      if (!navigator.onLine) { setOnline(false); setSaveState('Offline — jawaban tersimpan lokal'); return false }
      try {
        while (pendingAnswersRef.current.size || pendingFlagsRef.current.size) {
          if (activeAttemptRef.current !== attemptId) return false
          if (!navigator.onLine) throw new Error('Koneksi terputus.')
          const answer = pendingAnswersRef.current.entries().next().value as [string, { value: any; sequence: number }] | undefined
          if (answer) {
            const [questionId, pending] = answer
            const question = workspace?.soal?.find((item: Row) => item.upayaSoalId === questionId)
            const value = question?.soal?.tipe === 'unggah_berkas' && Array.isArray(pending.value)
              ? pending.value.map((file: Row | string) => typeof file === 'string' ? file : file.id).filter(Boolean)
              : pending.value
            await sharedRequest(`/simulasi/saya/upaya/${attemptId}/jawaban/${questionId}`, 'PUT', { jawaban: value })
            if (activeAttemptRef.current !== attemptId) return false
            if (pendingAnswersRef.current.get(questionId)?.sequence === pending.sequence) pendingAnswersRef.current.delete(questionId)
            persistPending(attemptId)
            continue
          }
          const flag = pendingFlagsRef.current.entries().next().value as [string, { value: boolean; sequence: number }] | undefined
          if (flag) {
            const [questionId, pending] = flag
            await sharedRequest(`/simulasi/saya/upaya/${attemptId}/soal/${questionId}/tandai`, 'PUT', { ditandai: pending.value })
            if (activeAttemptRef.current !== attemptId) return false
            if (pendingFlagsRef.current.get(questionId)?.sequence === pending.sequence) pendingFlagsRef.current.delete(questionId)
            persistPending(attemptId)
          }
        }
        if (activeAttemptRef.current !== attemptId) return false
        persistPending(attemptId)
        setSaveState('Tersimpan')
        return true
      } catch {
        if (activeAttemptRef.current !== attemptId) return false
        persistPending(attemptId)
        setSaveState(navigator.onLine ? 'Gagal tersimpan — Coba lagi' : 'Offline — jawaban tersimpan lokal')
        return false
      }
    })()
    const flight = { attemptId, promise: sync }
    saveInFlightRef.current = flight
    try { return await sync } finally { if (saveInFlightRef.current === flight) saveInFlightRef.current = null }
  }
  flushAnswerQueueRef.current = flushAnswerQueue
  function scheduleSave(attemptId: string) {
    window.clearTimeout(saveTimerRef.current)
    saveTimerRef.current = window.setTimeout(() => { void flushAnswerQueue(attemptId) }, 350)
  }
  async function saveAnswer(question: Row, value: any) {
    if (!workspace) return
    setAnswers((current) => ({ ...current, [question.upayaSoalId]: value }))
    answerSequenceRef.current += 1
    pendingAnswersRef.current.set(question.upayaSoalId, { value, sequence: answerSequenceRef.current })
    persistPending(workspace.upaya.id)
    if (!navigator.onLine) { setOnline(false); setSaveState('Offline — jawaban tersimpan lokal'); return }
    setSaveState('Menyimpan…'); scheduleSave(workspace.upaya.id)
  }
  async function uploadAnswerFile(question: Row, file: File): Promise<Row> {
    if (!workspace) throw new Error('Upaya pengerjaan tidak tersedia.')
    const form = new FormData()
    form.append('file', file)
    const response = await fetch(`${apiBase}/simulasi/saya/upaya/${workspace.upaya.id}/file/${question.upayaSoalId}`, {
      method: 'POST', credentials: 'include', headers: { Authorization: `Bearer ${token}`, ...(shareToken ? { 'X-Simulasi-Share-Token': shareToken } : {}) }, body: form,
    })
    const result = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(result?.error || result?.message || `Unggah gagal (${response.status}).`)
    return result as Row
  }
  async function removeAnswerFile(fileId: string) {
    if (!workspace) throw new Error('Upaya pengerjaan tidak tersedia.')
    await sharedRequest(`/simulasi/saya/upaya/${workspace.upaya.id}/file/${fileId}`, 'DELETE')
  }
  async function downloadAnswerFile(fileId: string, name: string) {
    if (!workspace) return
    try {
      const response = await fetch(`${apiBase}/simulasi/saya/upaya/${workspace.upaya.id}/file/${fileId}`, { credentials: 'include', headers: { Authorization: `Bearer ${token}`, ...(shareToken ? { 'X-Simulasi-Share-Token': shareToken } : {}) } })
      if (!response.ok) throw new Error('Berkas tidak dapat diunduh.')
      const url = URL.createObjectURL(await response.blob())
      const anchor = document.createElement('a'); anchor.href = url; anchor.download = name || 'jawaban'; anchor.click(); window.setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Berkas tidak dapat diunduh.') }
  }
  async function toggleFlag(question: Row) {
    if (!workspace) return
    const value = !flagged[question.upayaSoalId]; setFlagged((current) => ({ ...current, [question.upayaSoalId]: value }))
    answerSequenceRef.current += 1
    pendingFlagsRef.current.set(question.upayaSoalId, { value, sequence: answerSequenceRef.current })
    persistPending(workspace.upaya.id)
    if (!navigator.onLine) { setOnline(false); setSaveState('Offline — penanda tersimpan lokal'); return }
    setSaveState('Menyimpan…'); scheduleSave(workspace.upaya.id)
  }
  async function showAttemptResult(attemptId: string, warnAboutLocalAnswers = false) {
    const hadPendingAnswers = pendingAnswersRef.current.size > 0 || pendingFlagsRef.current.size > 0
    const data = await sharedRequest(`/simulasi/saya/upaya/${attemptId}/hasil`)
    if (activeAttemptRef.current !== attemptId) return
    try { sessionStorage.removeItem(`simulasi-pending:${attemptId}`) } catch { /* storage optional */ }
    activeAttemptRef.current = ''
    setResult(data); setWorkspace(null); setInstruction(null); setConfirmSubmit(false); void load()
    if (warnAboutLocalAnswers && hadPendingAnswers) toast.warning('Waktu berakhir saat jawaban masih menunggu sinkronisasi. Jawaban yang belum tersimpan tidak dapat dikirim.')
  }
  async function submit(confirmed = false, timedOut = false) {
    if (!workspace || submittingRef.current) return
    if (!confirmed) { setConfirmSubmit(true); return }
    const reachedDeadline = timedOut || remainingRef.current <= 0
    submittingRef.current = true
    setSubmitting(true)
    try {
      window.clearTimeout(saveTimerRef.current)
      const synced = await flushAnswerQueue(workspace.upaya.id)
      if (!synced || pendingAnswersRef.current.size || pendingFlagsRef.current.size) {
        if (!reachedDeadline) throw new Error('Masih ada perubahan yang belum tersimpan. Sambungkan internet dan coba kirim kembali.')
        // The server is authoritative about whether the deadline has elapsed.
        // Its workspace endpoint also finalizes expired attempts, so reconnecting
        // after an offline timeout can move the student to the correct result.
        const latest = await sharedRequest(`/simulasi/saya/upaya/${workspace.upaya.id}`)
        if (activeAttemptRef.current !== workspace.upaya.id) return
        if (latest.upaya?.status !== 'berlangsung') {
          await showAttemptResult(workspace.upaya.id, true)
          return
        }
        const serverRemaining = Math.max(0, Number(latest.sisaDetik) || 0)
        if (serverRemaining > 0) {
          setRemaining(serverRemaining)
          setSaveState('Jawaban belum tersinkron — mencoba kembali…')
          return
        }
        // At the deadline, avoid submitting unsynced answers as if they were
        // received in time. Recheck once; the server closes the attempt here.
        const expired = await sharedRequest(`/simulasi/saya/upaya/${workspace.upaya.id}`)
        if (activeAttemptRef.current !== workspace.upaya.id) return
        if (expired.upaya?.status !== 'berlangsung') {
          await showAttemptResult(workspace.upaya.id, true)
          return
        }
        setRemaining(Math.max(1, Number(expired.sisaDetik) || 1))
        setSaveState('Memeriksa waktu server…')
        return
      }
      if (reachedDeadline) {
        const latest = await sharedRequest(`/simulasi/saya/upaya/${workspace.upaya.id}`)
        if (activeAttemptRef.current !== workspace.upaya.id) return
        if (latest.upaya?.status !== 'berlangsung') {
          await showAttemptResult(workspace.upaya.id)
          return
        }
        const serverRemaining = Math.max(0, Number(latest.sisaDetik) || 0)
        if (serverRemaining > 0) {
          setRemaining(serverRemaining)
          return
        }
      }
      const attempt = await sharedRequest(`/simulasi/saya/upaya/${workspace.upaya.id}/kirim`, 'POST')
      await showAttemptResult(attempt.id)
    } catch (error: any) { toast.error(error.message || 'Pengiriman simulasi gagal.') } finally { submittingRef.current = false; setSubmitting(false) }
  }
  submitRef.current = submit
  const questions = workspace?.soal || []
  const current = questions[index]
  const answeredCount = questions.filter((question: Row) => answered(answers[question.upayaSoalId])).length
  const progress = questions.length ? answeredCount / questions.length * 100 : 0
  const currentFlagged = Boolean(current && flagged[current.upayaSoalId])

  if (loading) return <Centered label="Memuat simulasi kamu…" />
  if (result) return <ResultScreen result={result} onBack={() => { setResult(null); void load() }} />
  if (workspace && current) {
    const urgent = remaining < 300
    return <main className="min-h-screen bg-[#edf3fa] text-slate-900">
      <header className="sticky top-0 z-40 border-b border-white/10 bg-gradient-to-r from-[#1c5d94] via-[#287db7] to-[#24527d] text-white shadow-lg">
        <div className="mx-auto flex max-w-[1500px] items-center justify-between gap-3 px-3 py-3 sm:px-5">
          <div className="flex min-w-0 items-center gap-3"><div className="grid h-11 w-11 shrink-0 place-items-center rounded-2xl bg-white/15 text-lg font-black ring-1 ring-white/30">TI</div><div className="min-w-0"><p className="text-[11px] font-bold uppercase tracking-[.18em] text-white/75">Tunas Ilmu Learn</p><h1 className="truncate text-base font-bold sm:text-lg">{workspace.paket?.nama}</h1><p className="text-xs text-white/75">{workspace.paket?.mode === 'tka_sd' ? 'Simulasi TKA SD' : 'Simulasi ANBK / AKM'}</p></div></div>
          <div className="flex shrink-0 items-center gap-2 sm:gap-3"><div className={`flex items-center gap-2 rounded-xl px-3 py-2 font-mono text-base font-bold shadow-inner sm:text-xl ${urgent ? 'bg-red-500/90 text-white' : 'bg-white/15 text-white'}`} role="timer" aria-live={urgent ? 'assertive' : 'off'} aria-label={`Sisa waktu ${clock(remaining)}`}><Clock3 className="h-4 w-4 sm:h-5 sm:w-5" aria-hidden="true" />{clock(remaining)}</div><Button size="icon" variant="ghost" className="text-white hover:bg-white/15" onClick={onLogout} aria-label="Keluar dari portal"><LogOut className="h-4 w-4" /></Button></div>
        </div>
      </header>
      <div className="mx-auto w-full max-w-[1500px] space-y-3 px-3 py-3 sm:px-5 sm:py-4">
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white px-3 py-3 shadow-sm sm:px-4">
          <div className="flex min-w-0 items-center gap-3"><Badge className="shrink-0 bg-[#1d6fa8]">Soal {index + 1} dari {questions.length}</Badge><div className="min-w-28 flex-1"><div className="h-2 overflow-hidden rounded-full bg-slate-100" role="progressbar" aria-label="Kemajuan pengerjaan" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(progress)}><div className="h-full rounded-full bg-[#2f8cca] transition-all" style={{ width: `${progress}%` }} /></div><p className="mt-1 text-[11px] text-slate-500">{answeredCount} terjawab · {questions.length - answeredCount} belum dijawab</p></div></div>
          <div className="flex flex-wrap items-center justify-end gap-1.5"><span className="mr-1 hidden text-xs text-slate-500 sm:inline">Ukuran teks</span>{(['small', 'medium', 'large'] as FontScale[]).map((size, sizeIndex) => <Button key={size} size="icon" variant={fontScale === size ? 'default' : 'outline'} className="h-9 w-9 rounded-lg" onClick={() => setFontScale(size)} aria-label={`Ukuran teks ${size === 'small' ? 'kecil' : size === 'large' ? 'besar' : 'sedang'}`} aria-pressed={fontScale === size}><span className={sizeIndex === 0 ? 'text-xs' : sizeIndex === 1 ? 'text-sm' : 'text-base'}>A</span></Button>)}<Button size="sm" variant="outline" className="h-9" onClick={() => setShowInfo(true)}><Info className="h-4 w-4" /> <span className="hidden sm:inline">Informasi soal</span></Button><Button size="sm" variant="outline" className="h-9 xl:hidden" onClick={() => setShowPalette(true)}><Grid2X2 className="h-4 w-4" /> Daftar soal</Button></div>
        </div>
        {urgent && <div role="alert" className="flex items-center gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-medium text-red-800"><AlertCircle className="h-4 w-4 shrink-0" />Waktu hampir habis. Periksa jawaban dan kirimkan simulasi.</div>}
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-slate-200 bg-white/70 px-3 py-2 text-xs text-slate-600"><span className="flex items-center gap-1.5" aria-live="polite"><Save className="h-3.5 w-3.5" />{saveState || 'Autosave aktif'}</span><span className={online ? 'flex items-center gap-1 text-emerald-700' : 'flex items-center gap-1 text-amber-700'}>{online ? <><Wifi className="h-3.5 w-3.5" />Terhubung</> : <><WifiOff className="h-3.5 w-3.5" />Offline — jawaban akan dicoba lagi saat koneksi kembali</>}</span></div>
        <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(380px,1fr)_248px]">
          <Card className="min-h-[480px] overflow-hidden border-slate-200 bg-white shadow-sm"><div className="flex items-center justify-between border-b border-slate-200 bg-slate-50 px-4 py-3"><div><p className="text-[11px] font-bold uppercase tracking-[.14em] text-[#1d6fa8]">Stimulus</p><h2 className="font-semibold">Bacaan / informasi pendukung</h2></div><span className="text-xs text-slate-500">Panel dapat digulir</span></div><div className="max-h-[calc(100vh-285px)] min-h-[390px] overflow-y-auto p-4 sm:p-6" style={{ fontSize: `${scalePx(fontScale)}px` }}><StimulusContent items={current.soal?.stimulus || []} token={token} shareToken={shareToken} emptyLabel="Soal ini tidak menggunakan stimulus tambahan." /></div></Card>
          <Card className="min-h-[480px] overflow-hidden border-slate-200 bg-white shadow-sm"><div className="flex items-center justify-between border-b border-slate-200 bg-white px-4 py-3"><div><p className="text-[11px] font-bold uppercase tracking-[.14em] text-[#1d6fa8]">Pertanyaan {index + 1}</p><p className="text-xs text-slate-500">Pilih atau tulis jawabanmu dengan teliti.</p></div><Button size="sm" variant={currentFlagged ? 'default' : 'outline'} className="h-9" onClick={() => void toggleFlag(current)} aria-pressed={currentFlagged}><Flag className={`h-3.5 w-3.5 ${currentFlagged ? 'fill-current' : ''}`} />{currentFlagged ? 'Ditandai' : 'Ragu-ragu'}</Button></div><div className="flex min-h-[390px] flex-col p-4 sm:p-6" style={{ fontSize: `${scalePx(fontScale)}px` }}><p className="mb-6 whitespace-pre-wrap text-[1.08em] font-semibold leading-relaxed" id={`question-${current.upayaSoalId}`}>{current.soal?.pertanyaan}</p><QuestionAnswerControl question={current.soal || { tipe: 'isian_singkat' }} questionId={`question-${current.upayaSoalId}`} value={answers[current.upayaSoalId]} onChange={(value) => void saveAnswer(current, value)} fontScale={fontScale} onFileUpload={(file) => uploadAnswerFile(current, file)} onFileRemove={(fileId) => removeAnswerFile(fileId)} onFileDownload={(fileId, name) => void downloadAnswerFile(fileId, name)} /><div className="mt-auto flex flex-wrap justify-between gap-2 border-t border-slate-100 pt-5"><Button variant="outline" className="min-h-11" disabled={index === 0} onClick={() => setIndex((value) => value - 1)}><ArrowLeft className="h-4 w-4" /> Sebelumnya</Button>{index === questions.length - 1 ? <Button className="min-h-11" onClick={() => void submit()}><Send className="h-4 w-4" /> Kirim simulasi</Button> : <Button className="min-h-11" onClick={() => setIndex((value) => value + 1)}>Berikutnya <ArrowRight className="h-4 w-4" /></Button>}</div></div></Card>
          <QuestionPalette questions={questions} index={index} answers={answers} flagged={flagged} onSelect={setIndex} onSubmit={() => void submit()} />
        </div>
      </div>
      {showPalette && <Modal title="Daftar soal" onClose={() => setShowPalette(false)}><QuestionPalette questions={questions} index={index} answers={answers} flagged={flagged} onSelect={(next) => { setIndex(next); setShowPalette(false) }} onSubmit={() => { setShowPalette(false); void submit() }} mobile /></Modal>}
      {showInfo && <Modal title="Informasi pengerjaan" onClose={() => setShowInfo(false)}><div className="space-y-3 text-sm leading-relaxed text-slate-600"><p>Jawaban tersimpan otomatis saat kamu memilih atau mengetik. Pastikan status penyimpanan menunjukkan <strong className="text-slate-900">Tersimpan</strong> sebelum berpindah soal.</p><ul className="list-disc space-y-1 pl-5"><li>Gunakan tombol <strong className="text-slate-900">Ragu-ragu</strong> untuk menandai soal yang ingin diperiksa.</li><li>Gunakan tombol ukuran teks agar nyaman dibaca.</li><li>Panah kiri/kanan pada keyboard dapat digunakan untuk berpindah soal.</li><li>Pengiriman tidak dapat dibatalkan setelah dikonfirmasi.</li></ul></div></Modal>}
      {confirmSubmit && <Modal title="Periksa sebelum mengirim" onClose={() => setConfirmSubmit(false)}><SubmitSummary questions={questions} answers={answers} flagged={flagged} /><div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end"><Button variant="outline" onClick={() => setConfirmSubmit(false)}>Kembali memeriksa</Button><Button disabled={submitting} onClick={() => void submit(true)}>{submitting ? 'Mengirim…' : 'Ya, kirim simulasi'}</Button></div></Modal>}
    </main>
  }
  if (instruction) return <InstructionScreen instruction={instruction} onStart={() => void start()} onBack={() => setInstruction(null)} />
  return <main className="min-h-screen bg-[#edf3fa]"><header className="relative overflow-hidden bg-gradient-to-r from-[#1c5d94] via-[#287db7] to-[#24527d] text-white shadow-lg"><div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-4 py-7 sm:px-8 sm:py-9"><div className="flex items-center gap-3"><div className="grid h-12 w-12 place-items-center rounded-full bg-white/15 text-xl font-black ring-1 ring-white/35">TI</div><div><p className="text-xs font-bold uppercase tracking-[.18em] text-white/75">Portal Peserta Didik</p><h1 className="text-2xl font-black sm:text-3xl">Simulasi ANBK / TKA SD</h1><p className="mt-1 text-sm text-white/75">Latihan terarah dengan pengalaman ujian yang nyaman.</p></div></div><Button variant="outline" className="border-white/40 bg-white/10 text-white hover:bg-white/20" onClick={onLogout}><LogOut className="h-4 w-4" /> Keluar</Button></div></header><div className="mx-auto max-w-5xl px-4 pb-8 sm:px-8">{shareToken && <div className="relative z-10 -mt-4 mb-4 flex items-start gap-2 rounded-xl border border-[#2f8cca]/25 bg-white p-3 text-sm text-[#1d5f8e] shadow-sm"><Link2 className="mt-0.5 h-4 w-4 shrink-0" /><span>Kamu membuka tautan simulasi bersama. Kerjakan hanya jika tautan diberikan oleh guru.</span></div>}<div className="grid gap-4 sm:grid-cols-2">{rows.map((row) => { const packet = row.paket || {}; const ongoing = (row.attempts || []).find((attempt: Row) => attempt.status === 'berlangsung'); return <Card key={packet.id} className="group overflow-hidden border-slate-200 p-5 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md"><div className="flex items-center justify-between gap-2"><Badge variant={packet.mode === 'tka_sd' ? 'secondary' : 'default'}>{packet.mode === 'tka_sd' ? 'Simulasi TKA SD' : 'Simulasi ANBK/AKM'}</Badge>{ongoing && <span className="text-xs font-semibold text-amber-700">Sedang berlangsung</span>}</div><h2 className="mt-3 text-lg font-bold">{packet.nama}</h2><p className="mt-1 min-h-10 text-sm leading-relaxed text-muted-foreground">{packet.deskripsi || 'Latihan komputer untuk menguatkan kemampuanmu.'}</p><div className="mt-4 flex flex-wrap gap-2 text-xs text-muted-foreground"><span className="rounded-lg bg-slate-100 px-2.5 py-1">{packet.durasiMenit} menit</span><span className="rounded-lg bg-slate-100 px-2.5 py-1">Maks. {packet.maksPercobaan} percobaan</span></div><Button className="mt-5 min-h-11 w-full" disabled={!row.tersedia} onClick={() => ongoing ? void openWorkspace(ongoing.id) : void openInstruction(packet)}>{ongoing ? <><Play className="h-4 w-4" /> Lanjutkan pengerjaan</> : row.tersedia ? <><Play className="h-4 w-4" /> Lihat instruksi</> : 'Belum tersedia'}</Button></Card> })}{!rows.length && <Card className="p-12 text-center sm:col-span-2"><Play className="mx-auto h-10 w-10 text-[#2f8cca]/50" /><h2 className="mt-3 font-bold">Belum ada simulasi</h2><p className="text-sm text-muted-foreground">Guru akan menugaskan simulasi kepadamu di sini.</p></Card>}</div></div></main>
}

function QuestionPalette({ questions, index, answers, flagged, onSelect, onSubmit, mobile = false }: { questions: Row[]; index: number; answers: Record<string, any>; flagged: Record<string, boolean>; onSelect: (index: number) => void; onSubmit: () => void; mobile?: boolean }) { const unanswered = questions.length - questions.filter((question) => answered(answers[question.upayaSoalId])).length; return <Card className={`${mobile ? '' : 'hidden xl:block'} h-fit border-slate-200 p-4 shadow-sm`}><div className="flex items-center justify-between"><div><h2 className="text-sm font-bold">Daftar soal</h2><p className="text-[11px] text-slate-500">Pilih nomor untuk berpindah</p></div><span className="rounded-full bg-slate-100 px-2 py-1 text-[11px] font-semibold text-slate-600">{unanswered} kosong</span></div><div className="mt-4 grid grid-cols-5 gap-2">{questions.map((question: Row, questionIndex: number) => <button type="button" key={question.upayaSoalId} onClick={() => onSelect(questionIndex)} className={`relative min-h-10 rounded-lg text-sm font-bold outline-none ring-offset-2 transition focus-visible:ring-2 focus-visible:ring-[#1d6fa8] ${questionIndex === index ? 'bg-[#1d6fa8] text-white shadow-sm' : answered(answers[question.upayaSoalId]) ? 'bg-emerald-100 text-emerald-800 hover:bg-emerald-200' : 'bg-slate-100 text-slate-700 hover:bg-slate-200'}`} aria-label={`Buka soal ${questionIndex + 1}${answered(answers[question.upayaSoalId]) ? ', sudah dijawab' : ', belum dijawab'}${flagged[question.upayaSoalId] ? ', ditandai' : ''}`}>{questionIndex + 1}{flagged[question.upayaSoalId] && <Flag className="absolute -right-1 -top-1 h-3 w-3 fill-amber-400 text-amber-600" aria-hidden="true" />}</button>)}</div><div className="mt-4 space-y-1 text-[11px] text-slate-500"><p><span className="mr-1 inline-block h-2 w-2 rounded-full bg-[#1d6fa8]" />Sedang dibuka</p><p><span className="mr-1 inline-block h-2 w-2 rounded-full bg-emerald-400" />Sudah dijawab</p><p><Flag className="mr-1 inline h-3 w-3 text-amber-600" />Ragu-ragu</p></div><Button className="mt-5 min-h-11 w-full" variant="outline" onClick={onSubmit}><Send className="h-4 w-4" /> Selesai & kirim</Button></Card> }
function SubmitSummary({ questions, answers, flagged }: { questions: Row[]; answers: Record<string, any>; flagged: Record<string, boolean> }) { const unanswered = questions.filter((question) => !answered(answers[question.upayaSoalId])).length; const review = questions.filter((question) => flagged[question.upayaSoalId]).length; return <div className="grid grid-cols-2 gap-3 text-center sm:grid-cols-3"><div className="rounded-xl bg-emerald-50 p-3"><strong className="block text-2xl text-emerald-700">{questions.length - unanswered}</strong><span className="text-xs text-emerald-800">Terjawab</span></div><div className="rounded-xl bg-amber-50 p-3"><strong className="block text-2xl text-amber-700">{review}</strong><span className="text-xs text-amber-800">Ragu-ragu</span></div><div className="rounded-xl bg-red-50 p-3"><strong className="block text-2xl text-red-700">{unanswered}</strong><span className="text-xs text-red-800">Kosong</span></div></div> }
function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) { return <div className="fixed inset-0 z-[100] grid place-items-center bg-slate-950/45 p-4 backdrop-blur-sm" role="presentation" onClick={onClose}><section role="dialog" aria-modal="true" aria-labelledby="modal-title" className="max-h-[90vh] w-full max-w-lg overflow-auto rounded-2xl bg-white p-5 shadow-2xl" onClick={(event) => event.stopPropagation()}><div className="mb-5 flex items-center justify-between gap-3"><h2 id="modal-title" className="text-lg font-bold">{title}</h2><Button size="icon" variant="ghost" onClick={onClose} aria-label="Tutup"><X className="h-4 w-4" /></Button></div>{children}</section></div> }
function InstructionScreen({ instruction, onStart, onBack }: { instruction: Row; onStart: () => void; onBack: () => void }) { const packet = instruction.paket || {}; return <main className="min-h-screen bg-[#edf3fa]"><header className="bg-gradient-to-r from-[#1c5d94] via-[#287db7] to-[#24527d] text-white"><div className="mx-auto max-w-2xl px-5 py-8 sm:px-8"><div className="flex items-center gap-3"><div className="grid h-12 w-12 place-items-center rounded-full bg-white/15 text-xl font-black ring-1 ring-white/35">TI</div><div><p className="text-xs font-bold uppercase tracking-[.18em] text-white/75">Portal Peserta Didik</p><p className="text-sm text-white/80">Simulasi ANBK / TKA SD</p></div></div></div></header><div className="relative z-10 mx-auto -mt-10 max-w-2xl px-4 pb-8"><Card className="overflow-hidden border-slate-200 shadow-xl"><div className="border-b border-slate-200 p-6 sm:p-8"><p className="text-xs font-bold uppercase tracking-[.16em] text-[#1d6fa8]">Konfirmasi simulasi</p><h1 className="mt-2 text-2xl font-black text-slate-900 sm:text-3xl">{packet.nama}</h1><p className="mt-2 text-sm text-muted-foreground">Pastikan kamu siap sebelum memulai. Waktu dihitung setelah tombol mulai ditekan.</p></div><div className="p-6 sm:p-8"><div className="grid grid-cols-3 gap-2 rounded-2xl bg-slate-50 p-4 text-center text-sm"><div><strong className="block text-xl text-[#1d6fa8]">{instruction.jumlahSoal}</strong>soal</div><div><strong className="block text-xl text-[#1d6fa8]">{packet.durasiMenit}</strong>menit</div><div><strong className="block text-xl text-[#1d6fa8]">{packet.maksPercobaan}</strong>percobaan</div></div><h2 className="mt-6 font-bold">Petunjuk pengerjaan</h2><p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">{packet.instruksi || 'Baca soal dengan teliti. Jawabanmu tersimpan otomatis.'}</p><ul className="mt-4 list-disc space-y-1 pl-5 text-sm leading-relaxed text-muted-foreground"><li>Gunakan daftar soal untuk berpindah dengan cepat.</li><li>Tandai soal ragu-ragu agar mudah diperiksa kembali.</li><li>Gunakan kontrol ukuran teks sesuai kenyamanan membaca.</li><li>Pengiriman tidak dapat dibatalkan setelah dikonfirmasi.</li></ul><div className="mt-8 flex flex-col-reverse gap-2 sm:flex-row sm:justify-between"><Button variant="outline" className="min-h-11" onClick={onBack}>Kembali</Button><Button className="min-h-11 sm:min-w-48" onClick={onStart}><Play className="h-4 w-4" /> Mulai simulasi</Button></div></div></Card></div></main> }
function ResultScreen({ result, onBack }: { result: Row; onBack: () => void }) { const summary = result.ringkasan; return <main className="min-h-screen bg-[#edf3fa]"><header className="bg-gradient-to-r from-[#1c5d94] via-[#287db7] to-[#24527d] text-white"><div className="mx-auto max-w-lg px-5 py-8 sm:px-8"><div className="flex items-center gap-3"><div className="grid h-12 w-12 place-items-center rounded-full bg-white/15 text-xl font-black ring-1 ring-white/35">TI</div><div><p className="text-xs font-bold uppercase tracking-[.18em] text-white/75">Portal Peserta Didik</p><p className="text-sm text-white/80">Hasil simulasi</p></div></div></div></header><div className="relative z-10 mx-auto -mt-10 max-w-lg px-4 pb-8"><Card className="border-slate-200 p-8 text-center shadow-xl"><div className="mx-auto grid h-16 w-16 place-items-center rounded-full bg-emerald-100"><CheckCircle2 className="h-9 w-9 text-emerald-600" /></div><h1 className="mt-4 text-2xl font-bold">Simulasi selesai</h1><p className="mt-2 text-sm text-muted-foreground">{result.status === 'menunggu_nilai' ? 'Jawaban uraianmu menunggu penilaian guru.' : 'Jawabanmu sudah berhasil dikirim.'}</p>{result.skor !== undefined && <div className="mt-6 rounded-2xl bg-[#eaf5fd] p-5"><p className="text-sm text-[#1d5f8e]">Nilai kamu</p><strong className="text-4xl text-[#1d6fa8]">{Number(result.skor).toFixed(1)}</strong></div>}{summary && <div className="mt-4 grid grid-cols-3 gap-2 text-sm"><div className="rounded-xl bg-emerald-50 p-3"><strong className="block text-lg text-emerald-700">{summary.benar}</strong>Benar</div><div className="rounded-xl bg-red-50 p-3"><strong className="block text-lg text-red-700">{summary.salah}</strong>Salah</div><div className="rounded-xl bg-slate-100 p-3"><strong className="block text-lg">{summary.kosong}</strong>Kosong</div></div>}{(result.pembahasan || []).length > 0 && <div className="mt-5 space-y-3 text-left"><h2 className="font-bold">Pembahasan</h2>{result.pembahasan.map((item: Row) => <div key={item.urutan} className="rounded-xl bg-muted/50 p-3 text-sm"><p className="font-medium">{item.urutan}. {item.pertanyaan}</p><p className="mt-1 whitespace-pre-wrap text-muted-foreground">{item.pembahasan || 'Belum ada pembahasan.'}</p></div>)}</div>}<Button className="mt-7 min-h-11" onClick={onBack}>Kembali ke daftar</Button></Card></div></main> }
function Centered({ label }: { label: string }) { return <div className="grid min-h-screen place-items-center bg-[#edf3fa] text-sm text-muted-foreground">{label}</div> }
