import {
  Archive as ArchiveIcon,
  Award,
  BookMarked,
  BookOpen,
  Building2,
  CalendarDays,
  ClipboardCheck,
  ClipboardList,
  Database,
  FileBarChart,
  FileText,
  FileUp,
  GraduationCap,
  IdCard,
  LayoutDashboard,
  Monitor,
  School,
  Settings,
  ShieldCheck,
  UserCog,
  Users,
  type LucideIcon,
} from 'lucide-react'
import { useSidebar } from '../../context/SidebarContext'
import { cn } from '../../lib/utils'

export interface NavItem {
  id: string
  label: string
  icon: LucideIcon
  roles: string[]
}

export interface NavGroup {
  groupLabel: string
  items: NavItem[]
}

export const NAV_GROUPS: NavGroup[] = [
  {
    groupLabel: 'MENU UTAMA',
    items: [
      { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'kalender', label: 'Kalender Akademik', icon: CalendarDays, roles: ['admin', 'kepala_sekolah', 'guru'] },
    ],
  },
  {
    groupLabel: 'DATA MASTER',
    items: [
      { id: 'tutor', label: 'Tutor', icon: Users, roles: ['admin', 'kepala_sekolah'] },
      { id: 'orang-tua', label: 'Orang Tua', icon: Users, roles: ['admin', 'kepala_sekolah'] },
      { id: 'pokjar', label: 'Pokjar', icon: Building2, roles: ['admin', 'kepala_sekolah'] },
      { id: 'tahun-ajaran', label: 'Tahun Ajaran', icon: CalendarDays, roles: ['admin', 'kepala_sekolah'] },
      { id: 'semester', label: 'Semester', icon: CalendarDays, roles: ['admin'] },
      { id: 'mapel', label: 'Mata Pelajaran', icon: BookOpen, roles: ['admin', 'kepala_sekolah'] },
      { id: 'program', label: 'Program (Paket)', icon: School, roles: ['admin', 'kepala_sekolah'] },
      { id: 'fase', label: 'Fase', icon: BookOpen, roles: ['admin', 'kepala_sekolah'] },
      { id: 'kelas-mapel', label: 'Mapel per Kelas', icon: BookOpen, roles: ['admin', 'kepala_sekolah'] },
      { id: 'penugasan', label: 'Penugasan Tutor', icon: UserCog, roles: ['admin', 'kepala_sekolah'] },
    ],
  },
  {
    groupLabel: 'KELAS & PESERTA DIDIK',
    items: [
      { id: 'kelas', label: 'Kelas Rombel', icon: School, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'peserta-didik', label: 'Peserta Didik', icon: GraduationCap, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'relasi-orang-tua', label: 'Relasi Orang Tua', icon: Users, roles: ['admin', 'kepala_sekolah'] },
      { id: 'kenaikan-kelas', label: 'Kenaikan Kelas', icon: GraduationCap, roles: ['admin', 'kepala_sekolah'] },
      { id: 'arsip', label: 'Arsip Data', icon: ArchiveIcon, roles: ['admin', 'kepala_sekolah'] },
    ],
  },
  {
    groupLabel: 'KBM & PEMBELAJARAN',
    items: [
      { id: 'presensi', label: 'Presensi Mingguan', icon: ClipboardCheck, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'jurnal-mengajar', label: 'Jurnal Mengajar', icon: ClipboardList, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'pengumuman', label: 'Pengumuman', icon: BookOpen, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'tugas', label: 'Tugas Siswa', icon: ClipboardList, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'materi', label: 'Materi Pembelajaran', icon: BookOpen, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'rpp', label: 'RPP', icon: FileText, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'kelas-virtual', label: 'Kelas Virtual', icon: School, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'modul-belajar', label: 'Modul Pembelajaran', icon: BookOpen, roles: ['admin', 'kepala_sekolah', 'guru'] },
    ],
  },
  {
    groupLabel: 'PENILAIAN & RAPOR',
    items: [
      { id: 'nilai', label: 'Modul Nilai', icon: ClipboardList, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'sumber-nilai', label: 'Sumber Nilai & Bobot', icon: ClipboardList, roles: ['admin', 'kepala_sekolah'] },
      { id: 'pengaturan-nilai', label: 'Pengaturan Nilai', icon: Award, roles: ['admin'] },
      { id: 'perilaku', label: 'Catatan Perilaku', icon: ClipboardList, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'kompetensi', label: 'Kompetensi', icon: ClipboardList, roles: ['guru'] },
      { id: 'nilai-kompetensi', label: 'Nilai Kompetensi', icon: ClipboardCheck, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'rapor', label: 'Rapor', icon: BookOpen, roles: ['admin', 'kepala_sekolah', 'guru'] },
    ],
  },
  {
    groupLabel: 'UJIAN & SERTIFIKAT',
    items: [
      { id: 'bank-soal', label: 'Bank Soal', icon: BookOpen, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'ujian', label: 'Ujian (Luring)', icon: ClipboardList, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'ujian-online', label: 'Ujian Online', icon: Monitor, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'ujian-monitor', label: 'Monitor Ujian', icon: Monitor, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'portal-ortu', label: 'Portal Orang Tua', icon: Users, roles: ['admin', 'kepala_sekolah'] },
      { id: 'sertifikat', label: 'Sertifikat', icon: Award, roles: ['admin', 'kepala_sekolah'] },
      { id: 'kartu-pelajar', label: 'Kartu Pelajar', icon: IdCard, roles: ['admin', 'kepala_sekolah', 'guru'] },
    ],
  },
  {
    groupLabel: 'LAPORAN & IMPORT',
    items: [
      { id: 'laporan', label: 'Pusat Laporan', icon: FileBarChart, roles: ['admin', 'kepala_sekolah', 'guru'] },
      { id: 'analytics', label: 'Analytics Dashboard', icon: FileBarChart, roles: ['admin', 'kepala_sekolah'] },
      { id: 'import', label: 'Import Terpusat', icon: FileUp, roles: ['admin', 'guru'] },
    ],
  },
  {
    groupLabel: 'PERPUSTAKAAN',
    items: [
      { id: 'buku', label: 'Buku', icon: BookOpen, roles: ['admin', 'kepala_sekolah'] },
      { id: 'buku-kelas', label: 'Penetapan Buku', icon: BookMarked, roles: ['admin', 'kepala_sekolah'] },
      { id: 'peminjaman-buku', label: 'Peminjaman Buku', icon: ClipboardCheck, roles: ['guru'] },
      { id: 'rekap-buku', label: 'Rekap Peminjaman', icon: ClipboardList, roles: ['admin', 'kepala_sekolah'] },
    ],
  },
  {
    groupLabel: 'SISTEM',
    items: [
      { id: 'dokumen-tutor', label: 'Dokumen Tutor', icon: FileText, roles: ['admin', 'guru'] },
      { id: 'surat-siswa', label: 'Upload Surat Siswa', icon: FileUp, roles: ['admin'] },
      { id: 'akun', label: 'Manajemen Akun', icon: UserCog, roles: ['admin', 'kepala_sekolah'] },
      { id: 'pengaturan-jadwal', label: 'Pengaturan Jadwal', icon: Settings, roles: ['admin', 'kepala_sekolah'] },
      { id: 'audit-log', label: 'Audit Log', icon: ShieldCheck, roles: ['admin', 'kepala_sekolah'] },
      { id: 'backup', label: 'Backup & Restore', icon: Database, roles: ['admin'] },
    ],
  },
]

export const NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items)

interface AppSidebarProps {
  page: string
  setPage: (p: string) => void
  role: string
}

export function AppSidebar({ page, setPage, role }: AppSidebarProps) {
  const { isExpanded, isMobileOpen, isHovered, setIsHovered, setMobileOpen } =
    useSidebar()

  const showLabels = isExpanded || isHovered || isMobileOpen

  return (
    <aside
      className={cn(
        'fixed top-0 left-0 mt-16 flex h-[calc(100dvh-4rem)] flex-col bg-white text-gray-900 transition-all duration-300 ease-in-out z-50 border-r border-gray-200 dark:bg-gray-900 dark:border-gray-800 lg:mt-0 lg:h-screen',
        isExpanded || isMobileOpen || isHovered ? 'w-[min(78vw,300px)] lg:w-[290px]' : 'w-[90px]',
        isMobileOpen ? 'translate-x-0' : '-translate-x-full',
        'lg:translate-x-0'
      )}
      onMouseEnter={() => !isExpanded && setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {/* Brand header — Tunas Ilmu */}
      <div
        className={cn(
          'flex items-center px-4 py-5 lg:px-0 lg:py-8',
          !isExpanded && !isHovered ? 'lg:justify-center' : 'justify-start'
        )}
      >
        <button
          onClick={() => {
            setPage('dashboard')
            if (isMobileOpen) setMobileOpen(false)
          }}
          className="flex items-center gap-3"
          title="Tunas Ilmu Learn"
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand-500 text-white font-bold text-sm shadow-theme-xs">
            TI
          </div>
          {showLabels && (
            <div className="flex flex-col text-left overflow-hidden">
              <span className="truncate font-bold tracking-tight text-gray-800 dark:text-white/90 text-sm leading-tight">
                Tunas Ilmu Learn
              </span>
              <span className="truncate text-[10px] uppercase font-semibold tracking-wider text-gray-500 dark:text-gray-400">
                PKBM Tunas Ilmu
              </span>
            </div>
          )}
        </button>
      </div>

      {/* Navigation */}
      <div className="flex flex-col overflow-y-auto duration-300 ease-linear no-scrollbar flex-1">
        <nav className="mb-6 px-4 lg:px-5">
          <div className="flex flex-col gap-3 lg:gap-4">
            {NAV_GROUPS.map((group) => {
              const filtered = group.items.filter((item) =>
                item.roles.includes(role)
              )
              if (filtered.length === 0) return null
              return (
                <div key={group.groupLabel}>
                  <h2
                    className={cn(
                      'mb-3 px-2 text-theme-xs uppercase flex leading-[18px] font-medium text-gray-400 lg:mb-4',
                      !isExpanded && !isHovered ? 'lg:justify-center' : 'justify-start'
                    )}
                  >
                    {showLabels ? group.groupLabel : '•'}
                  </h2>
                  <ul className="flex flex-col gap-1.5">
                    {filtered.map((item) => {
                      const Icon = item.icon
                      const isActive = page === item.id
                      return (
                        <li key={item.id}>
                          <button
                            onClick={() => {
                              setPage(item.id)
                              if (isMobileOpen) setMobileOpen(false)
                            }}
                            className={cn(
                              'menu-item group',
                              isActive ? 'menu-item-active' : 'menu-item-inactive',
                              !isExpanded && !isHovered
                                ? 'lg:justify-center'
                                : 'lg:justify-start'
                            )}
                          >
                            <span
                              className={cn(
                                'menu-item-icon-size',
                                isActive
                                  ? 'menu-item-icon-active'
                                  : 'menu-item-icon-inactive'
                              )}
                            >
                              <Icon className="size-6" />
                            </span>
                            {showLabels && (
                              <span className="truncate">{item.label}</span>
                            )}
                          </button>
                        </li>
                      )
                    })}
                  </ul>
                </div>
              )
            })}
          </div>
        </nav>
      </div>
    </aside>
  )
}
