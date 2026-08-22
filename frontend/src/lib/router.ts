// Mapping halaman (page id) <-> URL path untuk navigasi berbasis router.
//
// Sebagian besar halaman memakai path langsung sesuai id (mis. 'presensi' ->
// '/presensi'). Hanya id dengan path khusus yang didaftarkan di sini:
//   - 'tutor' -> '/guru'   (label menu "Tutor"; pengguna biasa menyebut guru)
//   - 'ujian' -> '/ujian-luring'  (/ujian dipakai halaman publik ujian online)

const PATH_OVERRIDES: Record<string, string> = {
  tutor: '/guru',
  ujian: '/ujian-luring',
}

const REVERSE_OVERRIDES: Record<string, string> = Object.fromEntries(
  Object.entries(PATH_OVERRIDES).map(([id, path]) => [path, id])
)

// Path canonical untuk sebuah halaman (dipakai navigate).
export function pathFor(pageId: string): string {
  return PATH_OVERRIDES[pageId] ?? '/' + pageId
}

// Semua path yang sah untuk sebuah halaman, termasuk alias path default.
export function pathsFor(pageId: string): string[] {
  const custom = PATH_OVERRIDES[pageId]
  const defaults = ['/' + pageId]
  if (custom) return [custom, ...defaults]
  return defaults
}

// Halaman (page id) dari pathname aktif; fallback 'dashboard'.
export function pageFromPath(pathname: string): string {
  const clean = pathname.replace(/\/+$/, '') || '/'
  const mapped = REVERSE_OVERRIDES[clean]
  if (mapped) return mapped
  return clean.slice(1) || 'dashboard'
}

// Daftar page id yang dikenal aplikasi (sumber untuk rute & pencarian).
export const PAGE_IDS = [
  'dashboard',
  'kalender',
  'tutor',
  'orang-tua',
  'pokjar',
  'tahun-ajaran',
  'semester',
  'mapel',
  'program',
  'fase',
  'kelas-mapel',
  'penugasan',
  'kelas',
  'peserta-didik',
  'relasi-orang-tua',
  'kenaikan-kelas',
  'arsip',
  'presensi',
  'jurnal-mengajar',
  'pengumuman',
  'tugas',
  'materi',
  'rpp',
  'kelas-virtual',
  'modul-belajar',
  'nilai',
  'sumber-nilai',
  'pengaturan-nilai',
  'perilaku',
  'kompetensi',
  'nilai-kompetensi',
  'rapor',
  'bank-soal',
  'ujian',
  'ujian-online',
  'ujian-monitor',
  'portal-ortu',
  'sertifikat',
  'kartu-pelajar',
  'laporan',
  'analytics',
  'kepatuhan-pembelajaran',
  'import',
  'buku',
  'buku-kelas',
  'peminjaman-buku',
  'rekap-buku',
  'dokumen-tutor',
  'surat-siswa',
  'akun',
  'pengaturan-jadwal',
  'audit-log',
  'backup',
  'notifikasi',
] as const
