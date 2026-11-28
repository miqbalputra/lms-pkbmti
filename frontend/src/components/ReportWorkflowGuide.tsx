export function ReportWorkflowGuide({ current, ready, canReview, canScore, readOnly, busy, onStep }: {
  current: string
  ready: boolean
  canReview: boolean
  canScore: boolean
  readOnly: boolean
  busy: boolean
  onStep: (tab: string, target: string) => void
}) {
  const steps = [
    ['Pilih periode', 'Pilih tahun ajaran, kelas, dan semester.', '', 'report-scope'],
    ['Daftar CP', 'Pilih mata pelajaran untuk melihat CP. Guru penyusun dapat mengedit atau menambah CP di sini.', 'nilai', 'report-cp'],
    canScore
      ? ['Isi nilai', 'Nilai per CP dan deskripsi tersimpan otomatis setelah Anda berhenti mengisi.', 'nilai', 'report-scores']
      : ['Lihat nilai dan deskripsi', 'Periksa nilai yang tersimpan untuk mata pelajaran ini.', 'nilai', 'report-scores'],
    ...(canReview ? [
      ['Periksa rapor', 'Pilih siswa. Periksa nilai, presensi, dan daftar isian yang perlu dilengkapi.', 'rapor', 'report-student'],
      readOnly
        ? ['Lihat hasil rapor', 'Buka rapor atau riwayat versi yang tersedia.', 'rapor', 'report-status']
        : ['Finalkan dan terbitkan', 'Tandai siap, finalkan, lalu terbitkan untuk orang tua.', 'rapor', 'report-status'],
    ] : []),
  ]
  return <section aria-label="Langkah pengisian rapor" className="rounded-xl border border-primary/25 bg-primary/5 p-4">
    <h3 className="font-bold">Panduan langkah demi langkah</h3>
    {ready && !canReview && <p className="mt-1 text-sm text-muted-foreground">Setelah nilai disimpan, wali kelas dapat langsung memeriksanya di Rapor.</p>}
    <ol className={`mt-3 grid gap-2 sm:grid-cols-2 ${canReview ? 'xl:grid-cols-5' : 'lg:grid-cols-3'}`}>{steps.map(([title, hint, tab, target], i) => <li key={target}>
      <button type="button" aria-current={current === target ? 'step' : undefined} disabled={busy || i > 0 && !ready} onClick={() => onStep(tab, target)} className={`flex h-full w-full items-start gap-2 rounded-lg border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${current === target ? 'border-primary bg-primary text-primary-foreground shadow-sm' : 'border-primary/15 bg-background hover:border-primary/50'}`}>
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-current text-xs font-bold">{i + 1}</span><span><strong className="block text-sm">{title}</strong><span className={`mt-1 block text-xs ${current === target ? 'text-primary-foreground/90' : 'text-muted-foreground'}`}>{hint}</span></span>
      </button>
    </li>)}</ol>
  </section>
}
