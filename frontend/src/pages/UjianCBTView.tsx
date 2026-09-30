import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { AlertTriangle, ArrowLeft, ArrowRight, CheckCircle2, Clock3, Flag, Grid2X2, Info, LockKeyhole, LogOut, RefreshCw, Send, Wifi, WifiOff } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { TurnstileWidget } from '../components/ui/turnstile'
import { QuestionAnswerControl, type FontScale } from '../components/simulasi/QuestionAnswerControl'
import { apiBase } from '../lib/api'
import { enqueueStudentChange, listStudentChanges, listStudentChangesForExam, removeStudentChange, type StudentAnswerChange } from '../lib/draftQueue'

type Row = Record<string, any> & { id: string }
type Exam = Row & { judul: string; namaPeserta: string; mapel?: Row }
type Stage = 'login' | 'exams' | 'instruction' | 'exam' | 'result' | 'recovery'
const json = (value: any) => { try { return typeof value === 'string' ? JSON.parse(value) : value } catch { return null } }
const makeKey = () => globalThis.crypto?.randomUUID?.() || 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (digit) => {
  const value = Math.floor(Math.random() * 16)
  return (digit === 'x' ? value : (value & 0x3) | 0x8).toString(16)
})
const formatDuration = (value: number) => { const safe = Math.max(0, Math.floor(value)); const hours = Math.floor(safe / 3600); const minutes = Math.floor((safe % 3600) / 60); const seconds = safe % 60; return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}` }

function controlFor(question: Row) {
  const options: string[] = Array.isArray(question.opsi) ? question.opsi : []
  const indices: number[] = Array.isArray(question.opsiIndex) ? question.opsiIndex : options.map((_, index) => index)
  if (question.tipe === 'pg' || question.tipe === 'true_false') return { question: { ...question, tipe: 'pg_tunggal', konfigurasi: { choices: options.map((text, index) => ({ id: String(indices[index] ?? index), text })) } }, legacy: true }
  if (question.tipe === 'checkbox') return { question: { ...question, tipe: 'pg_kompleks', konfigurasi: { choices: options.map((text, index) => ({ id: String(index), text })) } }, legacy: true }
  if (question.tipe === 'short_answer') return { question: { ...question, tipe: 'isian_singkat' }, legacy: true }
  if (question.tipe === 'essay') return { question: { ...question, tipe: 'uraian' }, legacy: true }
  return { question, legacy: false }
}

function decodeForControl(question: Row, raw: string | undefined) {
  if (raw === undefined || raw === null || raw === '') return null
  const type = question.tipe
  if (type === 'checkbox') {
    const values = json(raw)
    return Array.isArray(values) ? values.map(String) : []
  }
  if (type === 'unggah_berkas') {
    const values = json(raw)
    return Array.isArray(values) ? values.map((id) => typeof id === 'string' ? { id, namaFile: `Berkas ${id.slice(0, 6)}` } : id) : []
  }
  if (['pg', 'true_false', 'short_answer', 'essay'].includes(type)) return raw
  return json(raw) ?? raw
}

function encodeFromControl(question: Row, value: any) {
  if (question.tipe === 'checkbox') return JSON.stringify((Array.isArray(value) ? value : []).map((item) => Number(item)).filter(Number.isInteger))
  if (question.tipe === 'unggah_berkas') return JSON.stringify((Array.isArray(value) ? value : []).map((item) => typeof item === 'string' ? item : item?.id).filter(Boolean))
  if (['pg', 'true_false', 'short_answer', 'essay'].includes(question.tipe)) return String(value ?? '')
  return JSON.stringify(value ?? null)
}

function isAnswered(question: Row, raw: string | undefined) {
  const value = decodeForControl(question, raw)
  if (value === null || value === undefined || value === '') return false
  if (Array.isArray(value)) return value.length > 0
  if (typeof value === 'object') return Object.keys(value).length > 0
  return typeof value === 'string' ? value.trim().length > 0 : true
}

async function api(path: string, init: RequestInit = {}) {
  const response = await fetch(`${apiBase}${path}`, { credentials: 'include', ...init })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw Object.assign(new Error(data?.error || data?.message || `Permintaan gagal (${response.status})`), { status: response.status, data })
  return data
}

function StimulusPanel({ items, fontPx }: { items: Row[]; fontPx: number }) {
  if (!items.length) return <div className="grid min-h-64 place-items-center rounded-2xl border border-dashed border-slate-300 bg-slate-50 p-5 text-center text-sm text-slate-500">Tidak ada bahan pendukung untuk soal ini.</div>
  return <div className="space-y-5" style={{ fontSize: `${fontPx}px` }}>{items.map((item, index) => {
    if (item.jenis === 'table') {
      const rows = String(item.konten || '').split(/\r?\n/).filter((row: string) => row.trim()).map((row: string) => row.split('\t'))
      return <div key={item.id || index} className="overflow-auto rounded-xl border border-slate-200"><table className="min-w-full border-collapse"><tbody>{rows.map((row: string[], rowIndex: number) => <tr key={rowIndex} className={rowIndex === 0 ? 'bg-slate-50 font-semibold' : ''}>{row.map((cell, cellIndex) => rowIndex === 0 ? <th key={cellIndex} className="border-b border-r border-slate-200 px-3 py-2 text-left">{cell}</th> : <td key={cellIndex} className="border-b border-r border-slate-200 px-3 py-2 align-top">{cell}</td>)}</tr>)}</tbody></table></div>
    }
    if (item.jenis === 'media_link') return <a key={item.id || index} href={item.konten} target="_blank" rel="noopener noreferrer" className="flex min-h-12 items-center rounded-xl border border-sky-200 bg-sky-50 p-3 font-medium text-sky-800 underline">Buka media pendukung di tab baru</a>
    if (item.jenis === 'image' && /^(https:\/\/|\/)/i.test(String(item.konten || ''))) return <img key={item.id || index} src={item.konten} alt={item.altText || 'Gambar pendukung soal'} className="max-h-[34rem] max-w-full rounded-xl border border-slate-200 object-contain" />
    return <p key={item.id || index} className="whitespace-pre-wrap leading-relaxed">{item.konten || ''}</p>
  })}</div>
}

export default function UjianCBTView() {
  const [stage, setStage] = useState<Stage>('login')
  const [siteKey, setSiteKey] = useState('')
  const [captchaKey, setCaptchaKey] = useState(0)
  const [captchaToken, setCaptchaToken] = useState('')
  const [nisn, setNisn] = useState('')
  const [accessCode, setAccessCode] = useState('')
  const [exams, setExams] = useState<Exam[]>([])
  const [studentName, setStudentName] = useState('')
  const [selectedExam, setSelectedExam] = useState<Exam | null>(null)
  const [attemptId, setAttemptId] = useState('')
  const [questions, setQuestions] = useState<Row[]>([])
  const [answers, setAnswers] = useState<Record<string, string>>({})
  const [revisions, setRevisions] = useState<Record<string, number>>({})
  const [flags, setFlags] = useState<Record<string, boolean>>({})
  const [index, setIndex] = useState(0)
  const [serverOffset, setServerOffset] = useState(0)
  const [normalDeadline, setNormalDeadline] = useState(0)
  const [graceDeadline, setGraceDeadline] = useState(0)
  const [now, setNow] = useState(Date.now())
  const [online, setOnline] = useState(navigator.onLine)
  const [saveState, setSaveState] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [fontScale, setFontScale] = useState<FontScale>(() => { try { return (localStorage.getItem('ujian-font-scale') as FontScale) || 'medium' } catch { return 'medium' } })
  const [submitStep, setSubmitStep] = useState<0 | 1 | 2>(0)
  const [integrityConfirmed, setIntegrityConfirmed] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [tabWarning, setTabWarning] = useState(false)
  const [result, setResult] = useState<Row | null>(null)
  const [recoverable, setRecoverable] = useState<Record<string, StudentAnswerChange[]>>({})
  const [recoveryMessage, setRecoveryMessage] = useState('')
  const currentRef = useRef('')
  const timerRef = useRef<number | undefined>(undefined)
  const saveTimerRef = useRef<number | undefined>(undefined)
  const flushRef = useRef<Promise<boolean> | null>(null)
  const scrollByStimulusRef = useRef(new Map<string, number>())
  const stimulusRef = useRef<HTMLDivElement | null>(null)
  const currentQuestion = questions[index]
  const currentStimulus = Array.isArray(currentQuestion?.stimulus) ? currentQuestion.stimulus : []
  const stimulusKey = JSON.stringify(currentStimulus)
  const answeredCount = questions.filter((question) => isAnswered(question, answers[question.id])).length
  const emptyIndices = questions.map((question, questionIndex) => isAnswered(question, answers[question.id]) ? -1 : questionIndex).filter((item) => item >= 0)
  const flaggedIndices = questions.map((question, questionIndex) => flags[question.id] ? questionIndex : -1).filter((item) => item >= 0)
  const fontPx = fontScale === 'small' ? 15 : fontScale === 'large' ? 22 : 18
  const activeDeadline = now + serverOffset < normalDeadline ? normalDeadline : graceDeadline
  const remaining = activeDeadline ? Math.max(0, Math.floor((activeDeadline - (now + serverOffset)) / 1000)) : 0
  const inGrace = Boolean(normalDeadline && now + serverOffset >= normalDeadline && now + serverOffset < graceDeadline)
  const urgency = !normalDeadline ? 'bg-slate-600' : remaining <= 300 ? 'bg-rose-600' : remaining <= 900 ? 'bg-amber-500' : 'bg-emerald-600'
  const chooseIndex = useCallback((nextIndex: number) => {
    if (stimulusRef.current) scrollByStimulusRef.current.set(stimulusKey, stimulusRef.current.scrollTop)
    setIndex(Math.max(0, Math.min(questions.length - 1, nextIndex)))
    try { localStorage.setItem(`ujian-index:${attemptId}`, String(nextIndex)) } catch { /* optional */ }
  }, [attemptId, questions.length, stimulusKey])

  useEffect(() => { void api('/ujian-online/public-config').then((data) => setSiteKey(data.turnstileSiteKey || '')).catch(() => {}) }, [])
  useEffect(() => {
    if (stage !== 'exams' || !exams.length) return
    void Promise.all(exams.map(async (exam) => [exam.id, await listStudentChangesForExam(exam.id).catch(() => [])] as const)).then((entries) => setRecoverable(Object.fromEntries(entries)))
  }, [stage, exams])
  useEffect(() => {
    const onOnline = () => { setOnline(true); if (currentRef.current) void flushQueue(currentRef.current) }
    const onOffline = () => { setOnline(false); setSaveState('Offline — perubahan tersimpan lokal') }
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    return () => { window.removeEventListener('online', onOnline); window.removeEventListener('offline', onOffline) }
  })
  useEffect(() => {
    if (stage !== 'exam') return
    const timer = window.setInterval(() => setNow(Date.now()), 500)
    return () => window.clearInterval(timer)
  }, [stage])
  useEffect(() => {
    if (stage !== 'exam' || !graceDeadline || remaining > 0 || busy) return
    setSubmitStep(1)
    setError('Waktu telah habis. Jawaban yang sudah tersinkron akan dikirim; jawaban lokal yang belum tersinkron dapat diajukan untuk ditinjau guru.')
  }, [stage, remaining, graceDeadline, busy])
  useEffect(() => {
    try { localStorage.setItem('ujian-font-scale', fontScale) } catch { /* browser storage is optional */ }
  }, [fontScale])
  useEffect(() => {
    if (stage !== 'exam') return
    const onVisibility = () => {
      if (document.visibilityState !== 'visible') {
        setTabWarning(true)
        if (selectedExam) void api(`/ujian-online/${selectedExam.id}/tab-switch`, { method: 'POST' }).catch(() => {})
      } else if (selectedExam && attemptId && navigator.onLine) {
        void api(`/ujian-online/${selectedExam.id}/soal`).then((data) => {
          const receivedAt = Date.now()
          setServerOffset(new Date(data.serverTime || receivedAt).getTime() - receivedAt)
          setNormalDeadline(new Date(data.deadlineAt || 0).getTime())
          setGraceDeadline(new Date(data.graceDeadlineAt || 0).getTime())
        }).catch(() => {})
      }
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => document.removeEventListener('visibilitychange', onVisibility)
  }, [stage, selectedExam, attemptId])
  useEffect(() => {
    if (stage !== 'exam') return
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (target && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON'].includes(target.tagName))) return
      if (event.key === 'ArrowLeft') { event.preventDefault(); chooseIndex(index - 1) }
      if (event.key === 'ArrowRight') { event.preventDefault(); chooseIndex(index + 1) }
      if (event.key.toLowerCase() === 'm') setPaletteOpen(true)
      if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) setSubmitStep(1)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [stage, index, chooseIndex])
  useEffect(() => {
    if (stage === 'exam' && stimulusRef.current && scrollByStimulusRef.current.has(stimulusKey)) stimulusRef.current.scrollTop = scrollByStimulusRef.current.get(stimulusKey) || 0
  }, [stage, index, stimulusKey])
  useEffect(() => () => { window.clearTimeout(timerRef.current); window.clearTimeout(saveTimerRef.current) }, [])

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    if (!captchaToken) { setError('Selesaikan verifikasi keamanan sebelum melanjutkan.'); return }
    setBusy(true)
    try {
      const body = new FormData()
      body.append('nisn', nisn.trim())
      body.append('aksesKode', accessCode.trim())
      body.append('cf-turnstile-response', captchaToken)
      const found: Exam[] = await api('/ujian-online/cek', { method: 'POST', body })
      setExams(found)
      setStudentName(found[0]?.namaPeserta || '')
      setStage('exams')
      setCaptchaToken('')
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Ujian tidak dapat ditemukan.'); setCaptchaKey((value) => value + 1); setCaptchaToken('') }
    finally { setBusy(false) }
  }

  async function loadAttempt(exam: Exam, id: string) {
    const data = await api(`/ujian-online/${exam.id}/soal`)
    const receivedAt = Date.now()
    setServerOffset(new Date(data.serverTime || receivedAt).getTime() - receivedAt)
    setNormalDeadline(new Date(data.deadlineAt || 0).getTime())
    setGraceDeadline(new Date(data.graceDeadlineAt || 0).getTime())
    const list: Row[] = (data.soal || []).map((question: Row) => ({ ...question, wajibDijawab: true }))
    setQuestions(list)
    setAttemptId(data.ujianPesertaId || id)
    const nextAnswers: Record<string, string> = {}
    const nextRevisions: Record<string, number> = {}
    for (const item of data.jawaban || []) { nextAnswers[item.ujianSoalId] = item.jawaban; nextRevisions[item.ujianSoalId] = Number(item.revision) || 0 }
    const nextFlags: Record<string, boolean> = {}
    for (const item of list) nextFlags[item.id] = Boolean(item.ditandai)
    const pending = await listStudentChanges(data.ujianPesertaId || id).catch(() => [])
    for (const change of pending) {
      if (change.kind === 'answer') nextAnswers[change.questionId] = String(change.value)
      if (change.kind === 'flag') nextFlags[change.questionId] = Boolean(change.value)
    }
    setAnswers(nextAnswers)
    setRevisions(nextRevisions)
    setFlags(nextFlags)
    const savedIndex = Number(localStorage.getItem(`ujian-index:${id}`))
    setIndex(Number.isFinite(savedIndex) ? Math.min(Math.max(0, savedIndex), Math.max(0, list.length - 1)) : 0)
    currentRef.current = data.ujianPesertaId || id
    setStage('exam')
    setSaveState(pending.length ? 'Menyinkronkan jawaban lokal…' : 'Tersimpan')
    if (pending.length && navigator.onLine) void flushQueue(data.ujianPesertaId || id)
  }

  async function begin(exam = selectedExam) {
    if (!exam) return
    if (!navigator.onLine) { setError('Untuk memulai atau melanjutkan ujian, sambungkan perangkat ke internet.'); return }
    setBusy(true); setError('')
    try {
      const started = await api(`/ujian-online/${exam.id}/mulai`, { method: 'POST' })
      setSelectedExam(exam)
      setAttemptId(started.id); currentRef.current = started.id
      await loadAttempt(exam, started.id)
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Ujian gagal dimulai.'); setStage('exams') }
    finally { setBusy(false) }
  }

  async function updateAnswer(question: Row, value: any) {
    if (!attemptId || !selectedExam) return
    const raw = encodeFromControl(question, value)
    setAnswers((current) => ({ ...current, [question.id]: raw }))
    const change: StudentAnswerChange = { key: `ujian:${attemptId}:answer:${question.id}`, attemptId, examId: selectedExam.id, questionId: question.id, kind: 'answer', value: raw, revision: revisions[question.id] || 0, requestId: makeKey(), queuedAt: Date.now() }
    try { await enqueueStudentChange(change) }
    catch { setSaveState('Gagal menyimpan lokal — jangan tutup halaman; coba lagi.'); return }
    setSaveState(navigator.onLine ? 'Menyimpan…' : 'Offline — perubahan tersimpan lokal')
    window.clearTimeout(saveTimerRef.current)
    saveTimerRef.current = window.setTimeout(() => { if (navigator.onLine) void flushQueue(attemptId) }, 500)
  }

  async function toggleFlag(question: Row) {
    if (!attemptId || !selectedExam) return
    const value = !flags[question.id]
    setFlags((current) => ({ ...current, [question.id]: value }))
    const change: StudentAnswerChange = { key: `ujian:${attemptId}:flag:${question.id}`, attemptId, examId: selectedExam.id, questionId: question.id, kind: 'flag', value, revision: 0, requestId: makeKey(), queuedAt: Date.now() }
    try { await enqueueStudentChange(change) } catch { setSaveState('Gagal menyimpan penanda lokal.'); return }
    if (navigator.onLine) void flushQueue(attemptId)
    else setSaveState('Offline — penanda tersimpan lokal')
  }

  async function flushQueue(id: string): Promise<boolean> {
    if (!navigator.onLine) return false
    if (flushRef.current) return flushRef.current
    const operation = (async () => {
      try {
        // An offline learner may return with a full paper queued. Allow enough
        // passes for all answers and flags instead of silently stopping at 20.
        for (let pass = 0; pass < 600; pass += 1) {
          if (currentRef.current !== id) return false
          const queue = await listStudentChanges(id)
          const change = queue.find((item) => item.key.startsWith('ujian:'))
          if (!change) { setSaveState('Tersimpan'); return true }
          const path = `/ujian-online/${change.examId}/${change.kind === 'flag' ? 'tandai' : 'jawab'}`
          const body = change.kind === 'flag'
            ? { ujianSoalId: change.questionId, ditandai: Boolean(change.value) }
            : { ujianSoalId: change.questionId, jawaban: String(change.value), baseRevision: change.revision }
          const result = await api(path, { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': change.requestId }, body: JSON.stringify(body) })
          if (change.kind === 'answer' && Number.isFinite(Number(result.revision))) {
            const acknowledgedRevision = Number(result.revision)
            setRevisions((current) => ({ ...current, [change.questionId]: acknowledgedRevision }))
            // A newer keystroke may have replaced this question's queued row
            // while the request was in flight. Carry the acknowledged server
            // revision forward so that the newest local value can be saved
            // without an avoidable stale-revision conflict.
            const latest = (await listStudentChanges(id)).find((item) => item.key === change.key)
            if (latest && latest.requestId !== change.requestId && latest.kind === 'answer') {
              await enqueueStudentChange({ ...latest, revision: acknowledgedRevision })
            }
          }
          const latest = (await listStudentChanges(id)).find((item) => item.key === change.key)
          if (latest?.requestId === change.requestId) await removeStudentChange(change.key)
        }
        setSaveState('Jawaban masih menyinkronkan — coba lagi')
        return false
      } catch (reason) {
        if ((reason as any)?.status === 409) setSaveState('Konflik jawaban — muat versi server sebelum melanjutkan')
        else setSaveState(navigator.onLine ? 'Gagal menyimpan — Coba lagi' : 'Offline — perubahan tersimpan lokal')
        return false
      }
    })()
    flushRef.current = operation
    try { return await operation } finally { if (flushRef.current === operation) flushRef.current = null }
  }

  async function reloadServerAnswers() {
    if (!selectedExam || !attemptId) return
    try {
      const data = await api(`/ujian-online/${selectedExam.id}/soal`)
      const values: Record<string, string> = {}; const version: Record<string, number> = {}; const marks: Record<string, boolean> = {}
      for (const answer of data.jawaban || []) { values[answer.ujianSoalId] = answer.jawaban; version[answer.ujianSoalId] = Number(answer.revision) || 0 }
      for (const question of data.soal || []) marks[question.id] = Boolean(question.ditandai)
      // Choosing the server copy is an explicit conflict resolution: remove
      // the local writes that are known to be stale so they cannot conflict
      // again on the next autosave or submit.
      for (const change of await listStudentChanges(attemptId)) await removeStudentChange(change.key)
      setQuestions(data.soal || []); setAnswers(values); setRevisions(version); setFlags(marks); setSaveState('Versi server dimuat'); setError('')
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Versi server tidak dapat dimuat.') }
  }

  async function submitRecovery(examOverride?: Exam, attemptOverride?: string) {
    const exam = examOverride || selectedExam
    const targetAttempt = attemptOverride || attemptId
    if (!exam || !targetAttempt) return
    setBusy(true); setRecoveryMessage('')
    try {
      const queued = (await listStudentChanges(targetAttempt)).filter((entry) => entry.kind === 'answer' && entry.key.startsWith('ujian:'))
      if (!queued.length) { setRecoveryMessage('Tidak ada jawaban lokal yang menunggu sinkronisasi.'); return }
      const result = await api(`/ujian-online/${exam.id}/pemulihan`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ idempotencyKey: makeKey(), answers: queued.map((entry) => ({ ujianSoalId: entry.questionId, jawaban: String(entry.value), baseRevision: entry.revision })) }) })
      for (const entry of queued) await removeStudentChange(entry.key)
      setRecoverable((current) => ({ ...current, [exam.id]: [] }))
      setRecoveryMessage(result.pesan || 'Jawaban dikirim kepada guru untuk ditinjau. Nilai tidak berubah otomatis.')
      setStage('recovery')
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Jawaban lokal belum dapat diajukan untuk ditinjau.') }
    finally { setBusy(false) }
  }

  async function submitRecoveryFromList(exam: Exam) {
    const pending = recoverable[exam.id] || []
    const targetAttempt = pending.find((entry) => entry.kind === 'answer')?.attemptId
    if (!targetAttempt) return
    setSelectedExam(exam); setAttemptId(targetAttempt); currentRef.current = targetAttempt
    await submitRecovery(exam, targetAttempt)
  }

  async function submitExam() {
    if (!selectedExam || !attemptId) return
    setBusy(true); setError('')
    try {
      let synced = await flushQueue(attemptId)
      const pending = await listStudentChanges(attemptId)
      if (!synced && graceDeadline && now + serverOffset >= graceDeadline) {
        const lateAnswers = pending.filter((item) => item.kind === 'answer' && item.key.startsWith('ujian:'))
        // Flags do not affect the grade. Once the server deadline has passed,
        // discard unsynced local flag changes instead of blocking submission.
        for (const item of pending.filter((entry) => entry.kind === 'flag')) await removeStudentChange(item.key)
        if (lateAnswers.length) {
          await api(`/ujian-online/${selectedExam.id}/pemulihan`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ idempotencyKey: makeKey(), answers: lateAnswers.map((item) => ({ ujianSoalId: item.questionId, jawaban: String(item.value), baseRevision: item.revision })) }) })
          for (const item of lateAnswers) await removeStudentChange(item.key)
          setRecoveryMessage('Jawaban yang belum tersinkron diajukan untuk ditinjau guru dan tidak mengubah nilai otomatis.')
        }
        synced = true
      }
      const remainingAnswers = (await listStudentChanges(attemptId)).some((item) => item.kind === 'answer' && item.key.startsWith('ujian:'))
      if (!synced || remainingAnswers) throw new Error('Masih ada jawaban belum tersimpan. Sambungkan internet dan coba lagi.')
      const score = await api(`/ujian-online/${selectedExam.id}/selesai`, { method: 'POST' })
      setResult(score); setStage('result'); setSubmitStep(0)
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Ujian belum dapat dikirim.') }
    finally { setBusy(false) }
  }

  async function uploadFile(question: Row, file: File) {
    if (!selectedExam) throw new Error('Sesi ujian belum aktif.')
    const form = new FormData(); form.append('file', file)
    const response = await fetch(`${apiBase}/ujian-online/${selectedExam.id}/soal/${question.id}/file`, { method: 'POST', credentials: 'include', body: form })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || 'Berkas tidak dapat diunggah.')
    return data as Row
  }

  async function removeFile(question: Row, fileId: string) {
    if (!selectedExam) return
    const response = await fetch(`${apiBase}/ujian-online/${selectedExam.id}/soal/${question.id}/file/${fileId}`, { method: 'DELETE', credentials: 'include' })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || 'Berkas tidak dapat dihapus.')
  }

  function saveScroll() {
    if (stimulusRef.current) scrollByStimulusRef.current.set(stimulusKey, stimulusRef.current.scrollTop)
  }

  if (stage === 'exams' && Object.values(recoverable).some((entries) => entries.some((entry) => entry.kind === 'answer'))) return <main className="min-h-screen bg-[#f3f6fb] p-4 text-slate-900"><Card className="mx-auto mt-12 max-w-xl space-y-4 p-6"><AlertTriangle className="h-9 w-9 text-amber-600" /><h1 className="text-xl font-bold">Ada jawaban lokal yang belum terkirim</h1><p className="text-sm leading-relaxed text-slate-600">Jawaban ini belum diakui server. Kamu dapat mengirimkannya kepada guru untuk ditinjau; pengajuan ini tidak mengubah nilai otomatis.</p>{exams.filter((exam) => recoverable[exam.id]?.some((entry) => entry.kind === 'answer')).map((exam) => <Button key={exam.id} className="min-h-12 w-full" disabled={busy || !online} onClick={() => void submitRecoveryFromList(exam)}>{busy ? 'Mengirim…' : `Kirim pemulihan · ${exam.judul}`}</Button>)}{error && <p role="alert" className="text-sm text-rose-800">{error}</p>}</Card></main>

  if (stage === 'login') return <main className="min-h-screen bg-[#f3f6fb] text-slate-900"><header className="relative min-h-52 overflow-hidden bg-gradient-to-br from-[#1c68a3] via-[#287db7] to-[#31587e] text-white"><div className="mx-auto flex max-w-5xl items-center gap-4 px-5 py-8"><div className="grid h-14 w-14 place-items-center rounded-full border-2 border-white/70 bg-white/10 text-lg font-black">TI</div><div><p className="text-lg font-extrabold">PKBM Tunas Ilmu</p><p className="text-sm text-white/80">Ujian Online</p></div></div></header><div className="mx-auto -mt-12 max-w-xl px-4 pb-12"><Card className="relative rounded-2xl border-0 p-6 shadow-2xl sm:p-9"><h1 className="text-2xl font-bold">Selamat datang</h1><p className="mt-2 leading-relaxed text-slate-600">Masuk menggunakan NISN dan kode akses dari tutor. Nama peserta akan diverifikasi dari data sekolah.</p><form className="mt-6 space-y-4" onSubmit={(event) => void login(event)}><label className="block text-sm font-semibold" htmlFor="cbt-nisn">NISN<input id="cbt-nisn" className="mt-2 min-h-12 w-full rounded-xl border border-slate-300 px-4 text-base font-normal focus:border-sky-600 focus:outline-none focus:ring-2 focus:ring-sky-600/20" inputMode="numeric" autoComplete="username" value={nisn} onChange={(event) => setNisn(event.target.value)} required /></label><label className="block text-sm font-semibold" htmlFor="cbt-code">Kode akses<input id="cbt-code" className="mt-2 min-h-12 w-full rounded-xl border border-slate-300 px-4 text-base font-normal tracking-widest focus:border-sky-600 focus:outline-none focus:ring-2 focus:ring-sky-600/20" autoComplete="one-time-code" value={accessCode} onChange={(event) => setAccessCode(event.target.value)} required /></label><TurnstileWidget key={captchaKey} sitekey={siteKey} onSuccess={setCaptchaToken} onError={() => setCaptchaToken('')} onExpire={() => setCaptchaToken('')} />{error && <p role="alert" className="rounded-xl border border-rose-200 bg-rose-50 p-3 text-sm text-rose-800">{error}</p>}<Button type="submit" disabled={busy || !captchaToken} className="min-h-12 w-full text-base">{busy ? <RefreshCw className="h-4 w-4 animate-spin" /> : <LockKeyhole className="h-4 w-4" />}{busy ? 'Memeriksa…' : 'Cari ujian'}</Button></form><p className="mt-4 text-center text-xs text-slate-500">Data jawaban tersimpan secara berkala. Jangan bagikan kode akses kepada orang lain.</p></Card></div></main>

  if (stage === 'exams' || stage === 'instruction') return <main className="min-h-screen bg-[#f3f6fb] text-slate-900"><header className="bg-gradient-to-r from-[#1c5d94] to-[#31587e] text-white"><div className="mx-auto flex max-w-5xl items-center justify-between gap-3 px-4 py-6"><div><p className="text-xs font-bold uppercase tracking-[.15em] text-white/75">Peserta terverifikasi</p><h1 className="text-xl font-bold">{studentName || exams[0]?.namaPeserta}</h1><p className="text-sm text-white/75">NISN {nisn}</p></div><Button variant="outline" className="min-h-11 border-white/40 bg-white/10 text-white hover:bg-white/20" onClick={() => { void api('/ujian-online/logout', { method: 'POST' }); setStage('login'); setExams([]) }}><LogOut className="h-4 w-4" /> Keluar</Button></div></header><div className="mx-auto max-w-4xl space-y-4 px-4 py-6">{error && <p role="alert" className="rounded-xl border border-rose-200 bg-rose-50 p-3 text-sm text-rose-800">{error}</p>}{stage === 'instruction' && selectedExam ? <Card className="space-y-4 p-5 sm:p-7"><Button variant="ghost" className="min-h-11 px-0" onClick={() => { setStage('exams'); setError('') }}><ArrowLeft className="h-4 w-4" /> Kembali ke daftar</Button><p className="text-xs font-bold uppercase tracking-[.12em] text-sky-700">Panduan sebelum ujian</p><h2 className="text-2xl font-bold">{selectedExam.judul}</h2><div className="grid gap-3 rounded-xl bg-slate-50 p-4 text-sm sm:grid-cols-2"><p><strong>Peserta:</strong> {studentName || selectedExam.namaPeserta}</p><p><strong>Mata pelajaran:</strong> {selectedExam.mapel?.namaMapel || 'Ujian Online'}</p><p><strong>Waktu:</strong> {selectedExam.durasiMenit} menit</p><p><strong>Berakhir:</strong> {new Date(selectedExam.waktuSelesai).toLocaleString('id-ID')}</p></div><ul className="list-disc space-y-2 pl-5 text-sm leading-relaxed text-slate-600"><li>Pastikan nama di atas adalah identitasmu. Identitas diambil langsung dari data sekolah.</li><li>Jawaban tersimpan otomatis saat ada koneksi. Jika koneksi terputus, perubahan yang belum tersinkron disimpan di perangkat ini untuk dicoba kembali.</li><li>Gunakan tombol <strong>Ragu-ragu</strong> untuk menandai soal. Kamu dapat berpindah menggunakan daftar nomor atau tombol panah.</li><li>Jangan menutup halaman sebelum melihat status <strong>Tersimpan</strong>. Perpindahan tab hanya memberi pengingat kecuali tutor mengatur batas khusus.</li></ul><label className="flex min-h-12 items-center gap-3 rounded-xl border border-slate-200 p-3 text-sm"><input type="checkbox" checked={studentName === (selectedExam.namaPeserta || studentName)} onChange={() => setStudentName((current) => current === '' ? selectedExam.namaPeserta : '')} className="h-5 w-5 accent-sky-700" />Saya sudah memeriksa identitas peserta.</label><Button className="min-h-12 w-full" disabled={busy || !studentName} onClick={() => void begin()}><CheckCircle2 className="h-4 w-4" />{busy ? 'Menyiapkan ujian…' : selectedExam.status === 'mulai' ? 'Lanjutkan ujian' : 'Mulai ujian'}</Button></Card> : exams.map((exam) => <Card key={exam.id} className="space-y-3 p-5"><div className="flex flex-wrap items-center justify-between gap-2"><span className="rounded-full bg-sky-50 px-3 py-1 text-xs font-bold text-sky-800">{exam.mapel?.namaMapel || 'Ujian Online'}</span>{exam.status === 'mulai' && <span className="text-xs font-semibold text-amber-700">Sedang berlangsung</span>}</div><h2 className="text-lg font-bold">{exam.judul}</h2><p className="text-sm text-slate-600">{exam.durasiMenit} menit · {new Date(exam.waktuMulai).toLocaleString('id-ID')} – {new Date(exam.waktuSelesai).toLocaleString('id-ID')}</p><div className="flex flex-wrap items-center justify-between gap-3"><span className="text-sm text-slate-600">{exam.namaPeserta || studentName}</span><Button className="min-h-11" disabled={['selesai', 'dikunci', 'menunggu_nilai'].includes(exam.status) && !exam.bolehEditRespons} onClick={() => { setSelectedExam(exam); setStage('instruction'); setError('') }}>{exam.status === 'mulai' ? 'Lanjutkan ujian' : exam.bolehEditRespons ? 'Lihat / perbaiki jawaban' : 'Lihat panduan'}</Button></div></Card>)}</div></main>

  if (stage === 'recovery') return <main className="grid min-h-screen place-items-center bg-[#f3f6fb] p-4"><Card className="w-full max-w-lg space-y-4 p-6 text-center"><CheckCircle2 className="mx-auto h-12 w-12 text-emerald-600" /><h1 className="text-2xl font-bold">Jawaban diterima untuk ditinjau</h1><p className="text-slate-600">{recoveryMessage || 'Kiriman pemulihan tersimpan. Guru akan meninjau sebelum ada perubahan nilai.'}</p><Button className="min-h-11 w-full" onClick={() => setStage('exams')}>Kembali</Button></Card></main>
  if (stage === 'result') return <main className="grid min-h-screen place-items-center bg-[#f3f6fb] p-4"><Card className="w-full max-w-lg space-y-4 p-6 text-center"><CheckCircle2 className="mx-auto h-12 w-12 text-emerald-600" /><h1 className="text-2xl font-bold">Ujian selesai</h1><p className="text-slate-600">{selectedExam?.judul}</p>{result?.menungguPenilaian ? <p className="rounded-xl bg-sky-50 p-4 text-sky-900">Jawaban tersimpan. {result.uraianMenunggu || 0} jawaban uraian menunggu penilaian guru.</p> : result?.skor !== undefined && result?.skor !== null ? <p className="text-4xl font-black text-sky-800">{Number(result.skor).toFixed(1)}</p> : <p>Hasil sedang diproses.</p>}{recoveryMessage && <p className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900">{recoveryMessage}</p>}<Button className="min-h-11 w-full" onClick={() => setStage('exams')}>Selesai</Button></Card></main>

  if (stage === 'exam' && currentQuestion) {
    const mapped = controlFor(currentQuestion)
    const value = decodeForControl(currentQuestion, answers[currentQuestion.id])
    const statusUrgency = remaining <= 300 ? 'text-rose-700' : remaining <= 900 ? 'text-amber-700' : 'text-emerald-700'
    return (
      <main className="min-h-screen bg-[#eef3f9] pb-24 text-slate-900 lg:pb-6">
        <header className="sticky top-0 z-40 bg-gradient-to-r from-[#1c5d94] to-[#31587e] text-white shadow-lg">
          <div className="mx-auto flex max-w-[1600px] flex-wrap items-center justify-between gap-3 px-3 py-3 sm:px-5">
            <div className="min-w-0"><p className="text-[11px] font-bold uppercase tracking-[.14em] text-white/75">Ujian Online · {studentName}</p><h1 className="truncate font-bold sm:text-lg">{selectedExam?.judul}</h1></div>
            <div className="flex items-center gap-2">
              <span className={`rounded-lg px-3 py-2 font-mono font-bold text-white ${urgency}`} role="timer" aria-label={normalDeadline ? `${inGrace ? 'Waktu tambahan' : 'Sisa waktu'} ${formatDuration(remaining)}` : 'Ujian tanpa batas waktu'}><Clock3 className="mr-1 inline h-4 w-4" />{normalDeadline ? formatDuration(remaining) : 'Tanpa batas'}</span>
              <span className="hidden items-center gap-1 rounded-lg bg-white/10 px-3 py-2 text-xs sm:flex">{online ? <><Wifi className="h-4 w-4" />Terhubung</> : <><WifiOff className="h-4 w-4" />Offline</>}</span>
            </div>
          </div>
        </header>
        <div className="mx-auto max-w-[1600px] space-y-3 px-3 py-3 sm:px-5 sm:py-4">
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white p-3 shadow-sm">
            <div className="flex min-w-56 flex-1 items-center gap-3"><span className="rounded-lg bg-sky-50 px-3 py-2 text-sm font-bold text-sky-900">Soal {index + 1} / {questions.length}</span><div className="min-w-24 flex-1"><div className="h-2 overflow-hidden rounded-full bg-slate-100"><div className="h-full rounded-full bg-sky-600" style={{ width: `${questions.length ? answeredCount / questions.length * 100 : 0}%` }} /></div><p className="mt-1 text-xs text-slate-500">{answeredCount} terjawab · {questions.length - answeredCount} kosong</p></div></div>
            <div className="flex flex-wrap items-center gap-1.5"><span className="hidden text-xs text-slate-500 sm:inline">Ukuran teks</span>{(['small', 'medium', 'large'] as FontScale[]).map((size) => <Button key={size} variant={fontScale === size ? 'default' : 'outline'} className="h-11 w-11 min-h-11 min-w-11" onClick={() => setFontScale(size)} aria-label={`Ukuran teks ${size === 'small' ? '15' : size === 'medium' ? '18' : '22'} piksel`} aria-pressed={fontScale === size}><span className="font-serif">A</span></Button>)}<Button variant="outline" className="min-h-11 xl:hidden" onClick={() => setPaletteOpen(true)}><Grid2X2 className="h-4 w-4" /> Daftar soal</Button></div>
          </div>
          {inGrace && <p role="alert" className="rounded-xl border border-rose-200 bg-rose-50 p-3 text-sm font-semibold text-rose-800">Waktu pengerjaan utama telah habis. Segera kirim jawaban sebelum masa tambahan berakhir.</p>}
          {normalDeadline && remaining <= 900 && remaining > 300 && !inGrace && <p role="status" className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">Sisa waktu 15 menit atau kurang. Periksa jawaban yang masih kosong.</p>}
          {normalDeadline && remaining <= 300 && remaining > 0 && <p role="alert" className="rounded-xl border border-rose-200 bg-rose-50 p-3 text-sm text-rose-900">Waktu hampir habis. Simpan jawaban dan kirim ujian sekarang.</p>}
          {tabWarning && <div role="status" className="flex items-start gap-2 rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900"><Info className="mt-0.5 h-4 w-4 shrink-0" /><span>Kamu berpindah dari halaman ujian. Tetap fokus pada ujian; pengingat ini tidak menghentikan pengerjaan.</span><button type="button" className="ml-auto min-h-11 px-2 font-semibold" onClick={() => setTabWarning(false)}>Tutup</button></div>}
          {error && <div role="alert" className="flex flex-wrap items-center gap-3 rounded-xl border border-rose-200 bg-rose-50 p-3 text-sm text-rose-800"><AlertTriangle className="h-4 w-4" />{error}{saveState.startsWith('Konflik') && <Button variant="outline" className="min-h-11" onClick={() => void reloadServerAnswers()}>Muat versi server (buang lokal yang konflik)</Button>}</div>}
          <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-slate-200 bg-white p-3 text-xs"><span aria-live="polite" className="font-medium">{saveState || 'Autosave aktif'}</span><span className={statusUrgency}>{online ? 'Perubahan yang belum diakui tersimpan di perangkat' : 'Offline — perubahan tersimpan lokal dan akan dicoba kembali'}</span><Button size="sm" variant="outline" className="min-h-10" onClick={() => void flushQueue(attemptId)} disabled={!online}>Coba sinkronkan</Button></div>
          <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_230px]">
            <Card className="overflow-hidden border-slate-200 bg-white"><div className="flex items-center justify-between border-b bg-slate-50 px-4 py-3"><div><p className="text-[11px] font-bold uppercase tracking-[.12em] text-sky-800">Stimulus</p><h2 className="font-semibold">Bahan pendukung</h2></div><span className="text-xs text-slate-500">Panel gulir terpisah</span></div><div ref={stimulusRef} onScroll={saveScroll} className="min-h-72 max-h-[calc(100vh-270px)] overflow-y-auto p-4 sm:p-6"><StimulusPanel items={currentStimulus} fontPx={fontPx} /></div></Card>
            <Card className="flex min-h-[470px] flex-col overflow-hidden border-slate-200 bg-white"><div className="flex items-center justify-between border-b px-4 py-3"><div><p className="text-[11px] font-bold uppercase tracking-[.12em] text-sky-800">Pertanyaan {index + 1}</p><h2 className="text-sm font-semibold text-slate-600">{currentQuestion.namaBagian || 'Jawab dengan teliti'}</h2></div><Button variant={flags[currentQuestion.id] ? 'default' : 'outline'} className="min-h-11" onClick={() => void toggleFlag(currentQuestion)} aria-pressed={Boolean(flags[currentQuestion.id])}><Flag className={`h-4 w-4 ${flags[currentQuestion.id] ? 'fill-current' : ''}`} />{flags[currentQuestion.id] ? 'Ditandai' : 'Ragu-ragu'}</Button></div>
              <div className="flex flex-1 flex-col p-4 sm:p-6" style={{ fontSize: `${fontPx}px` }}><p className="mb-6 whitespace-pre-wrap text-[1.08em] font-semibold leading-relaxed">{currentQuestion.pertanyaan}</p><QuestionAnswerControl question={mapped.question} questionId={`ujian-question-${currentQuestion.id}`} value={value} onChange={(next) => { const legacyValue = currentQuestion.tipe === 'checkbox' && mapped.legacy ? next.map((id: string) => Number(id)) : next; void updateAnswer(currentQuestion, legacyValue) }} fontScale={fontScale} onFileUpload={(file) => uploadFile(currentQuestion, file)} onFileRemove={(fileId) => removeFile(currentQuestion, fileId)} onFileDownload={async (fileId, name) => { const response = await fetch(`${apiBase}/ujian-online/${selectedExam?.id}/soal/${currentQuestion.id}/file/${fileId}`, { credentials: 'include' }); if (!response.ok) return; const url = URL.createObjectURL(await response.blob()); const anchor = document.createElement('a'); anchor.href = url; anchor.download = name; anchor.click(); URL.revokeObjectURL(url) }} />
                <div className="mt-auto flex justify-between gap-3 border-t pt-5"><Button variant="outline" className="min-h-12" onClick={() => chooseIndex(index - 1)} disabled={index === 0}><ArrowLeft className="h-4 w-4" /> Sebelumnya</Button>{index === questions.length - 1 ? <Button className="min-h-12" onClick={() => setSubmitStep(1)}><Send className="h-4 w-4" /> Periksa & kirim</Button> : <Button className="min-h-12" onClick={() => chooseIndex(index + 1)}>Berikutnya <ArrowRight className="h-4 w-4" /></Button>}</div>
              </div>
            </Card>
            <Card className="hidden h-fit p-4 xl:block"><div className="flex items-center justify-between"><h2 className="font-bold">Daftar soal</h2><span className="text-xs text-slate-500">{emptyIndices.length} kosong</span></div><div className="mt-4 grid grid-cols-5 gap-2">{questions.map((question, questionIndex) => <button key={question.id} type="button" className={`relative min-h-11 min-w-11 rounded-lg border font-bold ${questionIndex === index ? 'border-sky-700 bg-sky-700 text-white' : isAnswered(question, answers[question.id]) ? 'border-emerald-200 bg-emerald-50 text-emerald-900' : 'border-slate-200 bg-slate-50 text-slate-700'}`} aria-label={`Soal ${questionIndex + 1}${isAnswered(question, answers[question.id]) ? ', sudah dijawab' : ', kosong'}${flags[question.id] ? ', ragu-ragu' : ''}`} onClick={() => chooseIndex(questionIndex)}>{questionIndex + 1}{flags[question.id] && <Flag className="absolute -right-1 -top-1 h-3 w-3 fill-amber-400 text-amber-600" />}</button>)}</div><div className="mt-4 space-y-1 text-xs text-slate-600"><p>● Sedang dibuka</p><p className="text-emerald-700">● Sudah dijawab</p><p className="text-amber-700">⚑ Ragu-ragu</p></div><Button className="mt-4 min-h-11 w-full" variant="outline" onClick={() => setSubmitStep(1)}>Periksa & kirim</Button></Card>
          </div>
        </div>
        {paletteOpen && <div className="fixed inset-0 z-50 flex items-end bg-slate-950/50 sm:items-center sm:justify-center" role="presentation" onClick={() => setPaletteOpen(false)}><section role="dialog" aria-modal="true" aria-labelledby="ujian-palette-title" className="w-full rounded-t-2xl bg-white p-5 pb-[max(1rem,env(safe-area-inset-bottom))] shadow-2xl sm:max-w-md sm:rounded-2xl" onClick={(event) => event.stopPropagation()}><div className="flex items-center justify-between"><h2 id="ujian-palette-title" className="text-lg font-bold">Daftar soal</h2><Button variant="outline" className="min-h-11" onClick={() => setPaletteOpen(false)}>Tutup</Button></div><div className="mt-4 grid grid-cols-5 gap-2">{questions.map((question, questionIndex) => <button key={question.id} type="button" className={`min-h-12 rounded-lg border font-bold ${questionIndex === index ? 'bg-sky-700 text-white' : isAnswered(question, answers[question.id]) ? 'bg-emerald-50 text-emerald-900' : 'bg-slate-50'}`} onClick={() => { chooseIndex(questionIndex); setPaletteOpen(false) }}>{questionIndex + 1}{flags[question.id] ? ' ⚑' : ''}</button>)}</div><Button className="mt-4 min-h-11 w-full" onClick={() => { setPaletteOpen(false); setSubmitStep(1) }}>Periksa & kirim</Button></section></div>}
        {submitStep > 0 && <div className="fixed inset-0 z-[60] grid place-items-center bg-slate-950/55 p-4" role="presentation"><section role="dialog" aria-modal="true" aria-labelledby="submit-title" className="max-h-[90dvh] w-full max-w-xl overflow-auto rounded-2xl bg-white p-5 shadow-2xl sm:p-7"><h2 id="submit-title" className="text-xl font-bold">{submitStep === 1 ? 'Periksa jawaban' : 'Konfirmasi integritas'}</h2>{submitStep === 1 ? <><p className="mt-2 text-sm text-slate-600">{answeredCount} dari {questions.length} soal sudah dijawab.</p><div className="mt-4 grid gap-3 sm:grid-cols-2"><div className="rounded-xl bg-rose-50 p-3"><strong className="text-rose-900">Belum dijawab ({emptyIndices.length})</strong><div className="mt-2 flex flex-wrap gap-2">{emptyIndices.map((item) => <button key={item} className="min-h-11 min-w-11 rounded-lg border border-rose-200 bg-white" onClick={() => { chooseIndex(item); setSubmitStep(0) }}>{item + 1}</button>)}</div></div><div className="rounded-xl bg-amber-50 p-3"><strong className="text-amber-900">Ragu-ragu ({flaggedIndices.length})</strong><div className="mt-2 flex flex-wrap gap-2">{flaggedIndices.map((item) => <button key={item} className="min-h-11 min-w-11 rounded-lg border border-amber-200 bg-white" onClick={() => { chooseIndex(item); setSubmitStep(0) }}>{item + 1}</button>)}{!flaggedIndices.length && <span className="text-sm text-slate-500">Tidak ada soal bertanda.</span>}</div></div></div><p className="mt-4 text-sm text-slate-600">Kamu masih dapat kembali untuk memperbaiki jawaban yang kosong atau bertanda.</p><div className="mt-5 flex flex-col-reverse justify-end gap-2 sm:flex-row"><Button variant="outline" className="min-h-11" onClick={() => setSubmitStep(0)}>Kembali ke soal</Button><Button className="min-h-11" onClick={() => setSubmitStep(2)}>Lanjutkan</Button></div></> : <><p className="mt-2 text-sm leading-relaxed text-slate-600">Pastikan jawaban yang akan dikirim adalah hasil pekerjaanmu sendiri. Setelah dikirim, jawaban tidak dapat diubah kecuali pengaturan ujian mengizinkan revisi.</p><label className="mt-4 flex min-h-12 items-start gap-3 rounded-xl border border-slate-200 p-3 text-sm"><input type="checkbox" className="mt-0.5 h-5 w-5 accent-sky-700" checked={integrityConfirmed} onChange={(event) => setIntegrityConfirmed(event.target.checked)} />Saya memastikan jawaban ini adalah pekerjaan saya dan siap mengirimnya.</label>{error && <p role="alert" className="mt-3 rounded-lg bg-rose-50 p-3 text-sm text-rose-800">{error}</p>}<div className="mt-5 flex flex-col-reverse justify-end gap-2 sm:flex-row"><Button variant="outline" className="min-h-11" onClick={() => { setSubmitStep(1); setError('') }}>Kembali</Button><Button className="min-h-11" disabled={!integrityConfirmed || busy || !online} onClick={() => void submitExam()}>{busy ? 'Mengirim…' : 'Ya, kirim ujian'}</Button></div>{!online && <p className="mt-3 text-sm text-amber-800">Kirim ujian membutuhkan internet. Perubahan lokal akan tetap menunggu sampai terkoneksi.</p>}</>}</section></div>}
      </main>
    )
  }
  return <main className="grid min-h-screen place-items-center bg-[#eef3f9] p-4"><Card className="w-full max-w-xl p-6"><h1 className="text-xl font-bold">Menyiapkan ujian</h1><p className="mt-2 text-slate-600">Belum ada soal yang tersedia. Coba muat ulang.</p><Button className="mt-4 min-h-11" onClick={() => { if (selectedExam && attemptId) void loadAttempt(selectedExam, attemptId) }}>Muat ulang</Button></Card></main>
}
