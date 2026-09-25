import { useEffect, useState, type ChangeEvent } from 'react'
import { ArrowDown, ArrowUp, FileUp, Info, Link2, Trash2 } from 'lucide-react'
import { apiBase } from '../../lib/api'

type Question = Record<string, any> & { tipe?: string; pertanyaan?: string }
export type FontScale = 'small' | 'medium' | 'large'

function scalePx(scale: FontScale) { return scale === 'small' ? 15 : scale === 'large' ? 20 : 17 }
function parseTable(value: string) { return value.split('\n').filter((row) => row.trim() !== '').map((row) => row.split('\t')) }

export function QuestionAnswerControl({ question, questionId, value, onChange, fontScale = 'medium', onFileUpload, onFileRemove, onFileDownload, token, shareToken }: {
  question: Question
  questionId: string
  value: any
  onChange: (value: any) => void
  fontScale?: FontScale
  onFileUpload?: (file: File) => Promise<Question>
  onFileRemove?: (id: string) => Promise<void>
  onFileDownload?: (id: string, name: string) => void
  token?: string
  shareToken?: string
}) {
  const config = question.konfigurasi || {}
  const fontSize = `${scalePx(fontScale)}px`
  const [activeLeft, setActiveLeft] = useState<string | null>(null)
  const [draggedOrder, setDraggedOrder] = useState<number | null>(null)
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')

  useEffect(() => setActiveLeft(null), [questionId, question.tipe])

  if (question.tipe === 'unggah_berkas') {
    const files: Question[] = Array.isArray(value) ? value : []
    const allowed: string[] = config.allowedFileTypes || ['pdf', 'docx', 'xlsx', 'png', 'jpg', 'jpeg']
    const maxFiles = Number(config.maxFiles || 3)
    const maxBytes = Number(config.maxFileSizeMB || 10) * 1024 * 1024
    const accept = allowed.map((extension) => `.${extension}`).join(',')
    async function selectFile(event: ChangeEvent<HTMLInputElement>) {
      const file = event.currentTarget.files?.[0]
      event.currentTarget.value = ''
      if (!file) return
      const extension = file.name.split('.').pop()?.toLowerCase() || ''
      if (!allowed.includes(extension)) { setUploadError(`Jenis berkas .${extension || '?'} tidak diizinkan.`); return }
      if (file.size > maxBytes) { setUploadError(`Ukuran berkas melebihi ${config.maxFileSizeMB || 10} MB.`); return }
      if (files.length >= maxFiles) { setUploadError(`Maksimal ${maxFiles} berkas untuk soal ini.`); return }
      if (!onFileUpload) { setUploadError('Unggahan hanya aktif saat pengerjaan siswa.'); return }
      setUploadError('')
      setUploading(true)
      try { const uploaded = await onFileUpload(file); onChange([...files, uploaded]) }
      catch (error) { setUploadError(error instanceof Error ? error.message : 'Berkas gagal diunggah. Coba lagi.') }
      finally { setUploading(false) }
    }
    async function removeFile(file: Question) {
      if (!file.id || !onFileRemove) return
      setUploadError('')
      try { await onFileRemove(file.id); onChange(files.filter((item) => item.id !== file.id)) }
      catch (error) { setUploadError(error instanceof Error ? error.message : 'Berkas gagal dihapus. Coba lagi.') }
    }
    return <div className="space-y-3" style={{ fontSize }}>
      <label htmlFor={`${questionId}-files`} className={`flex min-h-14 cursor-pointer items-center justify-center gap-2 rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-4 text-center font-medium text-[#1d6fa8] transition hover:border-[#2f8cca] hover:bg-[#eff8ff] focus-within:ring-2 focus-within:ring-[#2f8cca] focus-within:ring-offset-2 ${!onFileUpload || uploading || files.length >= maxFiles ? 'cursor-not-allowed opacity-60' : ''}`}>
        <FileUp className="h-5 w-5 shrink-0" />{uploading ? 'Mengunggah…' : 'Pilih berkas jawaban'}
        <input id={`${questionId}-files`} type="file" accept={accept} className="sr-only" disabled={!onFileUpload || uploading || files.length >= maxFiles} onChange={selectFile} />
      </label>
      <p className="text-xs text-slate-500">Format: {allowed.map((item) => item.toUpperCase()).join(', ')} · Maks. {maxFiles} berkas · {config.maxFileSizeMB || 10} MB per berkas</p>
      {!onFileUpload && <p className="text-xs text-slate-500">Unggah aktif saat kamu mengerjakan simulasi.</p>}
      {uploadError && <p role="alert" className="rounded-lg bg-rose-50 p-3 text-sm text-rose-800">{uploadError}</p>}
      {files.length > 0 && <ul aria-label="Berkas yang dilampirkan" className="space-y-2">{files.map((file, index) => <li key={file.id || `${file.namaFile}-${index}`} className="flex min-h-12 items-center gap-2 rounded-lg border border-slate-200 bg-white p-2 text-sm"><button type="button" className="min-w-0 flex-1 truncate text-left font-medium text-[#1d6fa8] underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2f8cca]" onClick={() => file.id && onFileDownload?.(file.id, file.namaFile)} disabled={!onFileDownload || !file.id}>{file.namaFile || `Berkas ${index + 1}`}<span className="ml-2 text-xs font-normal text-slate-500">{file.ukuran ? `${(file.ukuran / 1024 / 1024).toFixed(2)} MB` : ''}</span></button>{onFileRemove && file.id && <button type="button" className="grid h-11 w-11 shrink-0 place-items-center rounded-lg text-slate-500 hover:bg-rose-50 hover:text-rose-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2f8cca]" aria-label={`Hapus berkas ${file.namaFile}`} onClick={() => void removeFile(file)}><Trash2 className="h-4 w-4" /></button>}</li>)}</ul>}
    </div>
  }

  if (question.tipe === 'pg_tunggal') return <div className="grid gap-3" role="radiogroup" aria-labelledby={questionId} style={{ fontSize }}>
    {(config.choices || []).map((choice: Question, index: number) => <label key={choice.id} className={`flex min-h-14 cursor-pointer items-start gap-3 rounded-xl border p-4 transition ${value === choice.id ? 'border-[#2f8cca] bg-[#eff8ff] shadow-sm' : 'border-slate-200 hover:border-[#8bc5e9] hover:bg-slate-50'}`}>
      <input aria-label={choice.text || `Pilihan ${String.fromCharCode(65 + index)}`} className="mt-1 h-5 w-5 shrink-0 accent-[#1d6fa8]" type="radio" name={questionId} checked={value === choice.id} onChange={() => onChange(choice.id)} />
      <span className="min-w-0 flex-1"><strong className="mr-1 text-[#1d6fa8]">{String.fromCharCode(65 + index)}.</strong>{choice.text}<ChoiceImage choice={choice} token={token} shareToken={shareToken} /></span>
    </label>)}
  </div>

  if (question.tipe === 'pg_kompleks') {
    const selected: string[] = Array.isArray(value) ? value : []
    return <div className="grid gap-3" role="group" aria-labelledby={questionId} style={{ fontSize }}>
      {(config.choices || []).map((choice: Question, index: number) => <label key={choice.id} className={`flex min-h-14 cursor-pointer items-start gap-3 rounded-xl border p-4 transition ${selected.includes(choice.id) ? 'border-[#2f8cca] bg-[#eff8ff] shadow-sm' : 'border-slate-200 hover:border-[#8bc5e9] hover:bg-slate-50'}`}>
        <input aria-label={choice.text || `Pernyataan ${String.fromCharCode(65 + index)}`} className="mt-1 h-5 w-5 shrink-0 accent-[#1d6fa8]" type="checkbox" checked={selected.includes(choice.id)} onChange={(event) => onChange(event.currentTarget.checked ? [...selected, choice.id] : selected.filter((id) => id !== choice.id))} />
        <span className="min-w-0 flex-1"><strong className="mr-1 text-[#1d6fa8]">{String.fromCharCode(65 + index)}.</strong>{choice.text}<ChoiceImage choice={choice} token={token} shareToken={shareToken} /></span>
      </label>)}
    </div>
  }

  if (question.tipe === 'dropdown') {
    const choices: Question[] = config.choices || []
    // Native <option> cannot render images. Keep the compact select for text-only
    // dropdowns, and switch to accessible radio cards when a teacher adds images.
    if (choices.some((choice) => choice.imageId)) return <fieldset className="grid gap-3" style={{ fontSize }}>
      <legend className="mb-2 font-medium">Pilih satu jawaban</legend>
      {choices.map((choice: Question, index: number) => <label key={choice.id} className={`flex min-h-14 cursor-pointer items-start gap-3 rounded-xl border p-4 transition ${value === choice.id ? 'border-[#2f8cca] bg-[#eff8ff] shadow-sm' : 'border-slate-200 hover:border-[#8bc5e9] hover:bg-slate-50'}`}>
        <input aria-label={choice.text || `Pilihan ${String.fromCharCode(65 + index)}`} className="mt-1 h-5 w-5 shrink-0 accent-[#1d6fa8]" type="radio" name={questionId} checked={value === choice.id} onChange={() => onChange(choice.id)} />
        <span className="min-w-0 flex-1"><strong className="mr-1 text-[#1d6fa8]">{String.fromCharCode(65 + index)}.</strong>{choice.text}<ChoiceImage choice={choice} token={token} shareToken={shareToken} /></span>
      </label>)}
    </fieldset>
    return <div style={{ fontSize }}><label className="mb-2 block font-medium" htmlFor={`${questionId}-dropdown`}>Pilih satu jawaban</label><select id={`${questionId}-dropdown`} className="min-h-12 w-full rounded-xl border border-slate-300 bg-white px-4 py-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2f8cca]" value={typeof value === 'string' ? value : ''} onChange={(event) => onChange(event.target.value)}><option value="">Pilih jawaban…</option>{choices.map((choice: Question) => <option key={choice.id} value={choice.id}>{choice.text}</option>)}</select></div>
  }

  if (question.tipe === 'skala_linear' || question.tipe === 'rating') {
    const min = question.tipe === 'rating' ? 1 : Number(config.scaleMin ?? 1)
    const max = question.tipe === 'rating' ? Number(config.ratingMax ?? 5) : Number(config.scaleMax ?? 5)
    const selected = value === null || value === undefined || value === '' ? null : Number(value)
    return <div className="space-y-2" style={{ fontSize }}><div className="flex flex-wrap items-center justify-between gap-2 text-sm text-slate-600"><span>{config.scaleMinLabel || ''}</span>{question.tipe === 'rating' ? <span>Ketuk jumlah bintang yang sesuai</span> : <span>Pilih satu nilai</span>}<span>{config.scaleMaxLabel || ''}</span></div><div className="flex flex-wrap gap-2" role="radiogroup" aria-label={question.tipe === 'rating' ? 'Pilih rating' : 'Pilih nilai skala'}>{Array.from({ length: Math.min(max - min + 1, 11) }, (_, index) => min + index).map((number) => <button type="button" key={number} role="radio" aria-checked={selected === number} aria-label={question.tipe === 'rating' ? `${number} dari ${max} bintang` : `Nilai ${number}`} onClick={() => onChange(number)} className={`min-h-12 min-w-12 rounded-xl border px-3 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2f8cca] ${selected === number ? 'border-[#1d6fa8] bg-[#eff8ff] text-[#164e72]' : 'border-slate-200 bg-white hover:border-[#8bc5e9]'}`}>{question.tipe === 'rating' ? '★'.repeat(number) : number}</button>)}</div></div>
  }

  if (question.tipe === 'kisi_pg' || question.tipe === 'kisi_checkbox') {
    const selected = value && typeof value === 'object' ? value : {}
    const multiple = question.tipe === 'kisi_checkbox'
    return <div className="overflow-x-auto rounded-xl border border-slate-200" style={{ fontSize }}><table className="w-full"><thead><tr className="bg-slate-50"><th scope="col" className="p-3 text-left">Pernyataan</th>{(config.columns || []).map((column: Question) => <th scope="col" key={column.id} className="min-w-20 p-3 text-center">{column.text}</th>)}</tr></thead><tbody>{(config.rows || []).map((row: Question) => <tr key={row.id} className="border-t border-slate-200"><th scope="row" className="p-3 text-left font-medium">{row.text}</th>{(config.columns || []).map((column: Question) => { const checked = multiple ? Boolean(selected[row.id]?.includes(column.id)) : selected[row.id] === column.id; return <td key={column.id} className="p-3 text-center"><input aria-label={`${row.text}: ${column.text}`} className="h-5 w-5 accent-[#1d6fa8]" type={multiple ? 'checkbox' : 'radio'} name={`${questionId}-${row.id}`} checked={checked} onChange={(event) => { if (!multiple) onChange({ ...selected, [row.id]: column.id }); else { const current: string[] = selected[row.id] || []; onChange({ ...selected, [row.id]: event.currentTarget.checked ? [...current, column.id] : current.filter((id) => id !== column.id) }) } }} /></td> })}</tr>)}</tbody></table></div>
  }

  if (question.tipe === 'susun_urutan') {
    const items: string[] = Array.isArray(value) ? value : []
    const choices: Question[] = config.choices || []
    const ordered = items.length ? items : choices.map((choice) => choice.id)
    const move = (index: number, target: number) => { const next = [...ordered]; const [item] = next.splice(index, 1); next.splice(target, 0, item); onChange(next) }
    return <div className="space-y-2" style={{ fontSize }}><p className="text-sm text-slate-600">Seret langkah untuk mengurutkan, atau gunakan tombol panah.</p>{ordered.map((id, index) => { const choice = choices.find((item) => item.id === id); if (!choice) return null; return <div key={id} draggable onDragStart={() => setDraggedOrder(index)} onDragOver={(event) => event.preventDefault()} onDrop={() => { if (draggedOrder !== null && draggedOrder !== index) move(draggedOrder, index); setDraggedOrder(null) }} className="flex min-h-12 items-center gap-2 rounded-xl border border-slate-200 bg-white p-2"><span className="cursor-grab px-1 text-slate-400" aria-hidden>⠿</span><span className="min-w-6 text-center text-sm font-bold text-[#1d6fa8]">{index + 1}.</span><span className="min-w-0 flex-1">{choice.text}</span><button type="button" className="grid h-11 w-11 place-items-center rounded-lg hover:bg-slate-100 disabled:opacity-40" disabled={index === 0} aria-label={`Pindahkan ${choice.text} ke atas`} onClick={() => move(index, index - 1)}><ArrowUp className="h-4 w-4" /></button><button type="button" className="grid h-11 w-11 place-items-center rounded-lg hover:bg-slate-100 disabled:opacity-40" disabled={index === ordered.length - 1} aria-label={`Pindahkan ${choice.text} ke bawah`} onClick={() => move(index, index + 1)}><ArrowDown className="h-4 w-4" /></button></div> })}</div>
  }

  if (question.tipe === 'benar_salah') {
    const selected = value && typeof value === 'object' ? value : {}
    return <div className="overflow-x-auto rounded-xl border border-slate-200" style={{ fontSize }}><table className="w-full">
      <thead><tr className="bg-slate-50"><th scope="col" className="p-3 text-left">Pernyataan</th><th scope="col" className="p-3 text-center">Benar</th><th scope="col" className="p-3 text-center">Salah</th></tr></thead>
      <tbody>{(config.statements || []).map((statement: Question) => <tr key={statement.id} className="border-t border-slate-200">
        <td className="p-3" id={`${questionId}-${statement.id}`}>{statement.text}</td>
        <td className="p-3 text-center"><input aria-label={`${statement.text || 'Pernyataan'}: Benar`} className="h-5 w-5 accent-[#1d6fa8]" type="radio" name={`${questionId}-${statement.id}`} checked={selected[statement.id] === true} onChange={() => onChange({ ...selected, [statement.id]: true })} /></td>
        <td className="p-3 text-center"><input aria-label={`${statement.text || 'Pernyataan'}: Salah`} className="h-5 w-5 accent-[#1d6fa8]" type="radio" name={`${questionId}-${statement.id}`} checked={selected[statement.id] === false} onChange={() => onChange({ ...selected, [statement.id]: false })} /></td>
      </tr>)}</tbody>
    </table></div>
  }

  if (question.tipe === 'menjodohkan') {
    const selected = value && typeof value === 'object' ? value : {}
    const rightIds = new Set(Object.values(selected).filter((id): id is string => typeof id === 'string' && Boolean(id)))
    return <div className="space-y-3" style={{ fontSize }}>
      <p className="text-sm text-slate-600">Pilih pernyataan di kiri, lalu pilih pasangan yang sesuai di kanan.</p>
      <div className="grid gap-3 md:grid-cols-2">
        <div className="rounded-xl border border-slate-200 bg-slate-50 p-3"><h3 className="mb-2 text-sm font-bold text-slate-700">Pernyataan</h3><div className="grid gap-2">
          {(config.left || []).map((left: Question, index: number) => <button type="button" key={left.id} className={`min-h-12 rounded-lg border px-3 py-2 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2f8cca] ${activeLeft === left.id ? 'border-[#1d6fa8] bg-[#dff1fc] text-[#164e72]' : selected[left.id] ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : 'border-slate-200 bg-white hover:border-[#8bc5e9]'}`} onClick={() => setActiveLeft(left.id)} aria-pressed={activeLeft === left.id}>
            <span className="mr-2 font-bold text-[#1d6fa8]">{index + 1}.</span>{left.text}{selected[left.id] && <span className="mt-1 block text-[11px] text-emerald-700">Sudah dipasangkan</span>}
          </button>)}
        </div></div>
        <div className="rounded-xl border border-slate-200 bg-slate-50 p-3"><h3 className="mb-2 text-sm font-bold text-slate-700">Pasangan</h3><div className="grid gap-2">
          {(config.right || []).map((right: Question) => <button type="button" key={right.id} className={`min-h-12 rounded-lg border px-3 py-2 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2f8cca] ${rightIds.has(right.id) ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : activeLeft ? 'border-slate-200 bg-white hover:border-[#8bc5e9] hover:bg-[#eff8ff]' : 'border-slate-200 bg-white text-slate-500'}`} onClick={() => { if (activeLeft) { onChange({ ...selected, [activeLeft]: right.id }); setActiveLeft(null) } }} disabled={!activeLeft} aria-label={`Pilih pasangan ${right.text || right.id}`}>
            <span className="mr-2 font-bold text-[#1d6fa8]">{right.id}</span>{right.text}{rightIds.has(right.id) && <span className="mt-1 block text-[11px] text-emerald-700">Terpilih</span>}
          </button>)}
        </div></div>
      </div>
      <p className="text-xs text-slate-500">{Object.keys(selected).length} dari {(config.left || []).length} pasangan dipilih.</p>
    </div>
  }

  const isEssay = question.tipe === 'uraian'
  const textMinLength = Number(config.textMinLength || 0)
  const textMaxLength = Number(config.textMaxLength || 0)
  const textValue = typeof value === 'string' ? value : ''
  const defaultValidationHint = textMinLength && textMaxLength
    ? `Jawaban ${textMinLength}–${textMaxLength} karakter.`
    : textMinLength ? `Jawaban minimal ${textMinLength} karakter.`
      : textMaxLength ? `Jawaban maksimal ${textMaxLength} karakter.` : ''
  const validationHint = [defaultValidationHint, String(config.validationMessage || '').trim()].filter(Boolean).join(' ')
  if (isEssay) return <div style={{ fontSize }}><textarea aria-label="Jawaban uraian" aria-describedby={`${questionId}-text-help`} maxLength={textMaxLength || undefined} className="min-h-36 w-full rounded-xl border border-slate-300 bg-white p-4 leading-relaxed shadow-inner outline-none transition focus:border-[#2f8cca] focus:ring-2 focus:ring-[#2f8cca]/20" value={textValue} onChange={(event) => onChange(event.target.value)} placeholder="Tuliskan jawabanmu dengan jelas." /><p id={`${questionId}-text-help`} className="mt-2 text-xs text-muted-foreground">{['Jawaban uraian akan dinilai guru menggunakan rubrik.', validationHint].filter(Boolean).join(' ')}</p>{textMaxLength > 0 && <p className="text-right text-xs text-muted-foreground">{Array.from(textValue).length}/{textMaxLength} karakter</p>}</div>

  const inputType = question.tipe === 'tanggal' ? 'date' : question.tipe === 'waktu' ? 'time' : 'text'
  const inputLabel = question.tipe === 'tanggal' ? 'Tanggal' : question.tipe === 'waktu' ? 'Waktu' : 'Jawaban singkat'
  const isShortText = question.tipe === 'isian_singkat'
  return <div style={{ fontSize }}><label htmlFor={`${questionId}-answer`} className="mb-2 block font-medium">{inputLabel}</label><input id={`${questionId}-answer`} aria-label={inputLabel} aria-describedby={`${questionId}-text-help`} maxLength={isShortText && textMaxLength > 0 ? textMaxLength : undefined} className="min-h-12 w-full rounded-xl border border-slate-300 bg-white px-4 py-3 shadow-inner outline-none transition focus:border-[#2f8cca] focus:ring-2 focus:ring-[#2f8cca]/20" type={inputType} value={textValue} onChange={(event) => onChange(event.target.value)} placeholder={inputType === 'text' ? 'Ketik jawaban singkatmu.' : undefined} /><p id={`${questionId}-text-help`} className="mt-2 text-xs text-muted-foreground">{inputType === 'text' && validationHint ? validationHint : inputType === 'text' ? 'Periksa kembali ejaan dan angka jawabanmu.' : 'Pilih jawaban menggunakan kontrol yang tersedia.'}</p>{isShortText && textMaxLength > 0 && <p className="text-right text-xs text-muted-foreground">{Array.from(textValue).length}/{textMaxLength} karakter</p>}</div>
}

export function StimulusImage({ id, alt, token, shareToken }: { id: string; alt: string; token: string; shareToken?: string }) {
  const [url, setUrl] = useState('')
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    const controller = new AbortController()
    let objectUrl = ''
    setUrl('')
    setFailed(false)
    void fetch(`${apiBase}/simulasi/stimulus/${encodeURIComponent(id)}/file`, { credentials: 'include', headers: { Authorization: `Bearer ${token}`, ...(shareToken ? { 'X-Simulasi-Share-Token': shareToken } : {}) }, signal: controller.signal })
      .then((response) => response.ok ? response.blob() : Promise.reject(new Error('Gambar stimulus tidak dapat dimuat.')))
      .then((blob) => { objectUrl = URL.createObjectURL(blob); setUrl(objectUrl) })
      .catch(() => { if (!controller.signal.aborted) setFailed(true) })
    return () => { controller.abort(); if (objectUrl) URL.revokeObjectURL(objectUrl) }
  }, [id, token, shareToken])
  if (failed) return <p role="status" className="rounded-lg bg-amber-50 p-3 text-sm text-amber-800">Gambar stimulus tidak dapat dimuat. Teks alternatif: {alt || 'tidak tersedia'}.</p>
  return url ? <img src={url} alt={alt || 'Stimulus soal'} className="max-h-[32rem] max-w-full rounded-xl border border-slate-200 bg-white object-contain shadow-sm" /> : <div className="h-32 animate-pulse rounded-xl bg-slate-100" role="status" aria-label="Memuat gambar stimulus" />
}

export function StimulusContent({ items, token, shareToken, emptyLabel, compact = false }: {
  items: Question[]
  token: string
  shareToken?: string
  emptyLabel?: string
  compact?: boolean
}) {
  if (!items.length) return <div className={`grid place-items-center rounded-2xl border border-dashed border-slate-300 bg-slate-50 p-6 text-center ${compact ? 'min-h-24' : 'min-h-[320px]'}`}><div><Info className="mx-auto h-8 w-8 text-slate-400" /><p className="mt-3 text-sm text-slate-500">{emptyLabel || 'Tidak ada stimulus tambahan.'}</p></div></div>
  return <div className="space-y-4" aria-label="Stimulus soal">{items.map((item, index) => <div key={item.id || index}>
    {item.jenis === 'image' ? <StimulusImage id={item.id} alt={item.altText} token={token} shareToken={shareToken} />
      : item.jenis === 'table' ? <div className="overflow-auto rounded-xl border border-slate-200 bg-white"><table className="min-w-full border-collapse text-[.95em]"><tbody>{parseTable(item.konten || '').map((row, rowIndex) => <tr key={rowIndex} className={rowIndex === 0 ? 'bg-slate-50 font-semibold' : ''}>{row.map((cell, cellIndex) => <td key={cellIndex} className="border-b border-r border-slate-200 px-3 py-2 align-top last:border-r-0">{cell}</td>)}</tr>)}</tbody></table></div>
        : item.jenis === 'media_link' ? <a className="flex min-h-11 items-center gap-2 rounded-xl border border-[#2f8cca]/25 bg-[#eff8ff] px-3 py-3 font-medium text-[#1d6fa8] underline-offset-2 hover:underline" href={item.konten} target="_blank" rel="noopener noreferrer" aria-label="Buka media pendukung di tab baru"><Link2 className="h-4 w-4 shrink-0" />Buka media pendukung di tab baru</a>
          : <p className="whitespace-pre-wrap leading-relaxed">{item.konten}</p>}
  </div>)}</div>
}

function ChoiceImage({ choice, token, shareToken }: { choice: Question; token?: string; shareToken?: string }) {
  const imageId = String(choice.imageId || '')
  if (!imageId) return null
  if (!token) return <p className="mt-2 text-xs text-slate-500">Gambar pilihan: {choice.imageAltText || choice.text || 'tersedia saat pengerjaan'}</p>
  return <div className="mt-3 max-w-sm"><StimulusImage id={imageId} alt={choice.imageAltText || choice.text || 'Gambar pilihan jawaban'} token={token} shareToken={shareToken} /></div>
}
