export type QuestionType = 'pg_tunggal' | 'pg_kompleks' | 'benar_salah' | 'menjodohkan' | 'isian_singkat' | 'uraian' | 'dropdown' | 'skala_linear' | 'rating' | 'kisi_pg' | 'kisi_checkbox' | 'tanggal' | 'waktu' | 'susun_urutan' | 'unggah_berkas'

export const questionTypes: Array<[QuestionType, string]> = [
  ['pg_tunggal', 'Pilihan ganda'], ['pg_kompleks', 'Pilihan ganda kompleks'], ['benar_salah', 'Benar / Salah'],
  ['menjodohkan', 'Menjodohkan'], ['isian_singkat', 'Jawaban singkat'], ['uraian', 'Paragraf / uraian berubrik'],
  ['dropdown', 'Dropdown'], ['skala_linear', 'Skala linear'], ['rating', 'Rating'], ['kisi_pg', 'Kisi pilihan tunggal'], ['kisi_checkbox', 'Kisi kotak centang'], ['tanggal', 'Tanggal'], ['waktu', 'Waktu / durasi'], ['susun_urutan', 'Susun urutan'],
  ['unggah_berkas', 'Unggah berkas'],
]

export const configExample = (type: QuestionType) => {
  if (type === 'pg_tunggal') return { choices: [{ id: 'a', text: 'Pilihan A' }, { id: 'b', text: 'Pilihan B' }], correctIds: ['a'] }
  if (type === 'pg_kompleks') return { choices: [{ id: 'a', text: 'Pernyataan A' }, { id: 'b', text: 'Pernyataan B' }, { id: 'c', text: 'Pernyataan C' }], correctIds: ['a', 'c'] }
  if (type === 'dropdown') return { choices: [{ id: 'a', text: 'Pilihan A' }, { id: 'b', text: 'Pilihan B' }], correctIds: ['a'] }
  if (type === 'benar_salah') return { statements: [{ id: 'p1', text: 'Pernyataan pertama', correct: true }, { id: 'p2', text: 'Pernyataan kedua', correct: false }] }
  if (type === 'menjodohkan') return { left: [{ id: 'l1', text: 'Kolom kiri 1' }, { id: 'l2', text: 'Kolom kiri 2' }], right: [{ id: 'r1', text: 'Kolom kanan 1' }, { id: 'r2', text: 'Kolom kanan 2' }], pairs: { l1: 'r2', l2: 'r1' } }
  if (type === 'isian_singkat') return { acceptedAnswers: ['jawaban contoh', 'jawaban alternatif'] }
  if (type === 'tanggal') return { acceptedAnswers: ['2026-09-24'] }
  if (type === 'waktu') return { acceptedAnswers: ['08:30'] }
  if (type === 'skala_linear') return { scaleMin: 1, scaleMax: 5, scaleMinLabel: 'Belum paham', scaleMaxLabel: 'Sangat paham', correctNumber: 5 }
  if (type === 'rating') return { ratingMax: 5, correctNumber: 5 }
  if (type === 'kisi_pg') return { rows: [{ id: 'r1', text: 'Baris 1' }, { id: 'r2', text: 'Baris 2' }], columns: [{ id: 'c1', text: 'Kolom 1' }, { id: 'c2', text: 'Kolom 2' }], gridCorrect: { r1: 'c1', r2: 'c2' } }
  if (type === 'kisi_checkbox') return { rows: [{ id: 'r1', text: 'Baris 1' }, { id: 'r2', text: 'Baris 2' }], columns: [{ id: 'c1', text: 'Kolom 1' }, { id: 'c2', text: 'Kolom 2' }], gridMultiCorrect: { r1: ['c1'], r2: ['c2'] } }
  if (type === 'susun_urutan') return { choices: [{ id: 'a', text: 'Langkah pertama' }, { id: 'b', text: 'Langkah kedua' }, { id: 'c', text: 'Langkah ketiga' }], correctOrder: ['a', 'b', 'c'] }
  if (type === 'unggah_berkas') return { allowedFileTypes: ['pdf', 'docx', 'xlsx', 'png', 'jpg', 'jpeg'], maxFiles: 3, maxFileSizeMB: 10 }
  return { rubrik: [{ kriteria: 'Ketepatan isi', maks: 3 }, { kriteria: 'Kejelasan penjelasan', maks: 2 }] }
}
