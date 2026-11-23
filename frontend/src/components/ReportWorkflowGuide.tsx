export function ReportWorkflowGuide({ current, ready, canReview, busy, onStep }: {
  current: number; ready: boolean; canReview: boolean; busy: boolean; onStep: (tab: string, target: string) => void
}) {
  const steps = [
    ['Pilih kelas', 'Tentukan tahun dan semester.', '', 'report-scope'],
    ['Susun CP', 'Pilih mapel dan tulis CP.', 'nilai', 'report-cp'],
    ['Isi nilai', 'Isi nilai dan tinjau deskripsi.', 'nilai', 'report-scores'],
    ['Periksa rapor', 'Lengkapi bagian yang ditandai.', 'rapor', 'report-student'],
    ['Final & cetak', 'Wali kelas memfinalkan rapor.', 'rapor', 'report-status'],
  ]
  return <section aria-label="Langkah pengisian rapor" className="rounded-xl border border-primary/25 bg-primary/5 p-4">
    <h3 className="font-bold">Panduan langkah demi langkah</h3>
    <p className="mt-1 text-sm text-muted-foreground">Kerjakan dari kiri ke kanan. Isian bertanda Wajib perlu lengkap sebelum rapor difinalkan; draf dapat disimpan bertahap.</p>
    <ol className="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-5">{steps.map(([title, hint, tab, target], i) => <li key={title}>
      <button type="button" aria-current={current === i + 1 ? 'step' : undefined} disabled={busy || i > 0 && !ready || i > 2 && !canReview} onClick={() => onStep(tab, target)} className={`flex h-full w-full items-start gap-2 rounded-lg border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${current === i + 1 ? 'border-primary bg-primary text-primary-foreground shadow-sm' : 'border-primary/15 bg-background hover:border-primary/50'}`}>
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-current text-xs font-bold">{i + 1}</span><span><strong className="block text-sm">{title}</strong><span className={`mt-1 block text-xs ${current === i + 1 ? 'text-primary-foreground/90' : 'text-muted-foreground'}`}>{i > 2 && !canReview ? 'Dilanjutkan oleh wali kelas atau admin.' : hint}</span></span>
      </button>
    </li>)}</ol>
  </section>
}
