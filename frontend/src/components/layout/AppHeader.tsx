import { useEffect, useRef, useState, useCallback } from 'react'
import {
  Bell,
  ChevronDown,
  Command,
  FileText,
  LogOut,
  Search,
  ShieldAlert,
  User as UserIcon,
} from 'lucide-react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../ui/alert-dialog'
import { Button } from '../ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../ui/dropdown-menu'
import { ThemeToggleButton } from '../common/ThemeToggleButton'
import { NAV_ITEMS } from './AppSidebar'
import { useSidebar } from '../../context/SidebarContext'
import { toast } from 'sonner'
import { apiBase, request } from '../../lib/api'
import { useNavigate } from 'react-router-dom'
import { pathFor } from '../../lib/router'

interface User {
  id: string
  username: string
  role: string
  nama?: string
}

interface Notif {
  id: string
  judul: string
  isi: string
  tipe: string
  isRead: boolean
  createdAt: string
}

interface AppHeaderProps {
  token: string
  user: User
  onLogout: () => void
  onOpenTutorAccount: () => void
}

function fmtNotifTime(v: string): string {
  if (!v) return ''
  const d = new Date(v)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return 'Baru saja'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m lalu`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}j lalu`
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })
}

const tipeIcon: Record<string, string> = {
  ujian: '📝',
  tugas: '📋',
  presensi: '✅',
  rapor: '📊',
  umum: '📢',
}

export function AppHeader({ token, user, onLogout, onOpenTutorAccount }: AppHeaderProps) {
  const [logoutModalOpen, setLogoutModalOpen] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [isSearchFocused, setIsSearchFocused] = useState(false)
  const searchInputRef = useRef<HTMLInputElement>(null)
  const { toggleSidebar, toggleMobileSidebar, isExpanded } = useSidebar()
  const navigate = useNavigate()

  const go = (pageId: string) => navigate(pathFor(pageId))

  // Real notifications state
  const [notifs, setNotifs] = useState<Notif[]>([])
  const [unreadCount, setUnreadCount] = useState(0)

  const loadNotifs = useCallback(() => {
    if (!token) return
    Promise.all([
      request('/notifikasi', token).then((d) => setNotifs(Array.isArray(d) ? d.slice(0, 10) : [])),
      request('/notifikasi/unread-count', token).then((d) => setUnreadCount(Number(d.count || 0))),
    ]).catch(() => {})
  }, [token])

  useEffect(() => {
    loadNotifs()
    // Use SSE for real-time notifications, fall back to polling
    let evtSource: EventSource | null = null
    let pollingTimer: ReturnType<typeof setInterval> | null = null
    const startPolling = () => {
      if (pollingTimer === null) pollingTimer = setInterval(loadNotifs, 30000)
    }
    try {
      evtSource = new EventSource(`${apiBase}/notifikasi/stream?token=${encodeURIComponent(token)}`, { withCredentials: true } as any)
      evtSource.addEventListener('notifikasi', (e) => {
        try {
          const newNotifs = JSON.parse(e.data)
          setNotifs((prev) => {
            const combined = [...newNotifs, ...prev]
            const unique = Array.from(new Map(combined.map((n: any) => [n.id, n])).values())
            return unique.slice(0, 10)
          })
          toast.info('Notifikasi baru diterima', { description: newNotifs[0]?.judul || '' })
        } catch {}
      })
      evtSource.addEventListener('unread', (e) => {
        setUnreadCount(Number(e.data) || 0)
      })
      evtSource.onerror = () => {
        evtSource?.close()
        evtSource = null
        startPolling()
      }
    } catch {
      startPolling()
    }
    return () => {
      evtSource?.close()
      if (pollingTimer !== null) clearInterval(pollingTimer)
    }
  }, [loadNotifs, token])

  const markNotifRead = (id: string) => {
    request(`/notifikasi/${id}/baca`, token, 'PUT')
      .then(() => {
        setNotifs((prev) => prev.map((n) => (n.id === id ? { ...n, isRead: true } : n)))
        setUnreadCount((u) => Math.max(0, u - 1))
      })
      .catch(() => {})
  }

  const markAllRead = () => {
    request('/notifikasi/baca-all', token, 'PUT')
      .then(() => {
        setNotifs((prev) => prev.map((n) => ({ ...n, isRead: true })))
        setUnreadCount(0)
        toast.success('Semua notifikasi ditandai sudah dibaca.')
      })
      .catch(() => {})
  }

  // Ctrl+K / ⌘K focuses the search box
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault()
        searchInputRef.current?.focus()
        setIsSearchFocused(true)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  const filteredNavItems = NAV_ITEMS.filter(
    (item) =>
      item.roles.includes(user.role) &&
      (item.label.toLowerCase().includes(searchTerm.toLowerCase()) ||
        item.id.toLowerCase().includes(searchTerm.toLowerCase()))
  )

  const handleSelectSearchItem = (pageId: string, label: string) => {
    go(pageId)
    setSearchTerm('')
    setIsSearchFocused(false)
    toast.success(`Membuka halaman ${label}`)
  }

  const getInitials = (name: string) => {
    if (!name) return 'U'
    const parts = name.trim().split(/[\s_]+/)
    if (parts.length >= 2) {
      return (parts[0][0] + parts[1][0]).toUpperCase()
    }
    return name.substring(0, 2).toUpperCase()
  }

  const formatRoleLabel = (role: string) => {
    switch (role) {
      case 'admin':
        return 'Administrator'
      case 'kepala_sekolah':
        return 'Kepala Sekolah'
      case 'guru':
        return 'Tutor (Wali Kelas)'
      default:
        return role.toUpperCase()
    }
  }

  const handleConfirmLogout = () => {
    setLogoutModalOpen(false)
    toast.success('Sesi berhasil diakhiri.')
    onLogout()
  }

  const displayName = String(user.nama || user.username).trim() || user.username

  return (
    <>
      <header className="sticky top-0 z-99999 flex min-h-16 w-full border-b border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
        <div className="flex min-w-0 flex-1 items-center justify-between gap-2 px-3 sm:px-6 lg:px-8">
          {/* Left: hamburger + search */}
          <div className="flex items-center gap-3 lg:gap-5">
            {/* Hamburger (mobile = drawer, desktop = collapse) */}
            <button
              onClick={toggleMobileSidebar}
              className="lg:hidden text-gray-700 dark:text-gray-300"
              title="Buka menu navigasi"
            >
              <svg
                className="fill-current"
                width="24"
                height="24"
                viewBox="0 0 24 24"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path
                  fillRule="evenodd"
                  clipRule="evenodd"
                  d="M3 6.25C3 5.83579 3.33579 5.5 3.75 5.5H20.25C20.6642 5.5 21 5.83579 21 6.25C21 6.66421 20.6642 7 20.25 7H3.75C3.33579 7 3 6.66421 3 6.25ZM3 12C3 11.5858 3.33579 11.25 3.75 11.25H20.25C20.6642 11.25 21 11.5858 21 12C21 12.4142 20.6642 12.75 20.25 12.75H3.75C3.33579 12.75 3 12.4142 3 12ZM3 17.75C3 17.3358 3.33579 17 3.75 17H20.25C20.6642 17 21 17.3358 21 17.75C21 18.1642 20.6642 18.5 20.25 18.5H3.75C3.33579 18.5 3 18.1642 3 17.75Z"
                  fill="currentColor"
                />
              </svg>
            </button>
            <button
              onClick={toggleSidebar}
              className="hidden lg:block text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
              title={isExpanded ? 'Ciutkan sidebar' : 'Lebarkan sidebar'}
            >
              <svg
                className="fill-current"
                width="24"
                height="24"
                viewBox="0 0 24 24"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <rect x="3" y="4" width="18" height="16" rx="2" stroke="currentColor" strokeWidth="1.5" fill="none" />
                <line x1="9" y1="4" x2="9" y2="20" stroke="currentColor" strokeWidth="1.5" />
              </svg>
            </button>

            {/* Search */}
            <div className="relative hidden lg:block">
              <div className="relative flex h-11 w-full items-center gap-2.5 rounded-lg border border-gray-200 bg-white px-4 shadow-theme-xs outline-none transition focus-within:border-brand-300 focus-within:ring-3 focus-within:ring-brand-500/10 dark:border-gray-800 dark:bg-gray-900 xl:w-[430px]">
                <Search className="h-5 w-5 text-gray-400 dark:text-gray-500" />
                <input
                  ref={searchInputRef}
                  type="text"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  onFocus={() => setIsSearchFocused(true)}
                  onBlur={() => setTimeout(() => setIsSearchFocused(false), 200)}
                  placeholder="Cari menu atau ketik perintah..."
                  className="h-full w-full bg-transparent text-sm text-gray-700 placeholder:text-gray-400 focus:outline-none dark:text-gray-300"
                />
                <kbd className="hidden xl:inline-flex h-5 items-center gap-1 rounded border border-gray-200 bg-gray-50 px-1.5 font-mono text-[10px] font-medium text-gray-500 dark:border-gray-800 dark:bg-gray-800 dark:text-gray-400">
                  ⌘K
                </kbd>
              </div>

              {/* Search dropdown */}
              {isSearchFocused &&
                (searchTerm.trim() !== '' || filteredNavItems.length > 0) && (
                  <div className="absolute top-full left-0 mt-2 w-[420px] rounded-xl border border-gray-200 bg-white p-2 shadow-theme-lg z-9999 dark:border-gray-800 dark:bg-gray-900">
                    <div className="px-3 py-1.5 text-theme-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400 flex items-center justify-between">
                      <span>Hasil Pencarian Navigasi</span>
                      <Command className="h-3 w-3" />
                    </div>
                    <div className="max-h-60 overflow-y-auto custom-scrollbar space-y-1">
                      {filteredNavItems.length === 0 ? (
                        <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
                          Tidak ada menu yang cocok dengan "{searchTerm}"
                        </div>
                      ) : (
                        filteredNavItems.map((item) => {
                          const Icon = item.icon
                          return (
                            <button
                              key={item.id}
                              onMouseDown={() =>
                                handleSelectSearchItem(item.id, item.label)
                              }
                              className="flex h-auto w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/5"
                            >
                              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-50 text-brand-500 dark:bg-brand-500/[0.12] dark:text-brand-400">
                                <Icon className="h-4 w-4" />
                              </div>
                              <span className="flex-1 font-medium text-gray-800 dark:text-white/90">
                                {item.label}
                              </span>
                              <span className="text-[10px] text-gray-400 uppercase tracking-wider">
                                Buka
                              </span>
                            </button>
                          )
                        })
                      )}
                    </div>
                  </div>
                )}
            </div>
          </div>

          {/* Right: theme toggle, notifications, profile */}
          <div className="flex shrink-0 items-center gap-1.5 sm:gap-3">
            <ThemeToggleButton />

            {/* Notifications */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  className="relative flex h-10 w-10 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-500 hover:bg-gray-100 hover:text-gray-700 sm:h-11 sm:w-11 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-white"
                  title="Pusat Notifikasi"
                >
                  <Bell className="h-5 w-5" />
                  {unreadCount > 0 && (
                    <span className="absolute -top-1 -right-1 flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-[10px] font-bold text-white">
                      {unreadCount > 99 ? '99+' : unreadCount}
                    </span>
                  )}
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                align="end"
                className="w-[calc(100vw-1rem)] max-w-96 p-2 rounded-xl border-gray-200 dark:border-gray-800"
              >
                <div className="flex items-center justify-between px-3 py-2 border-b border-gray-100 dark:border-gray-800">
                  <div className="flex items-center gap-2">
                    <Bell className="h-4 w-4 text-brand-500" />
                    <span className="text-sm font-semibold text-gray-800 dark:text-white/90">
                      Notifikasi
                    </span>
                  </div>
                  {unreadCount > 0 && (
                    <span className="rounded-full bg-brand-50 px-2 py-0.5 text-[10px] font-medium text-brand-500 dark:bg-brand-500/[0.12] dark:text-brand-400">
                      {unreadCount} Baru
                    </span>
                  )}
                </div>

                <div className="max-h-72 overflow-y-auto custom-scrollbar divide-y divide-gray-100 dark:divide-gray-800 py-1">
                  {notifs.length === 0 ? (
                    <div className="p-4 text-center text-sm text-gray-500">Tidak ada notifikasi</div>
                  ) : (
                    notifs.map((n) => (
                      <div
                        key={n.id}
                        onClick={() => {
                          markNotifRead(n.id)
                          if (n.tipe === 'ujian') go('ujian')
                          else if (n.tipe === 'tugas') go('tugas')
                          else if (n.tipe === 'presensi') go('presensi')
                          else if (n.tipe === 'rapor') go('rapor')
                          else go('notifikasi')
                        }}
                        className={`flex items-start gap-3 p-3 hover:bg-gray-50 rounded-lg cursor-pointer transition-colors dark:hover:bg-white/[0.03] ${
                          !n.isRead ? 'bg-brand-50/50 dark:bg-brand-500/5' : ''
                        }`}
                      >
                        <div className="h-8 w-8 shrink-0 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center text-sm">
                          {tipeIcon[n.tipe] || '📢'}
                        </div>
                        <div className="space-y-0.5 flex-1 min-w-0">
                          <p className={`text-sm leading-snug ${!n.isRead ? 'font-semibold text-gray-800 dark:text-white/90' : 'text-gray-600 dark:text-gray-400'}`}>
                            {n.judul}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400 leading-snug line-clamp-2">
                            {n.isi}
                          </p>
                          <p className="text-[10px] text-gray-400 font-mono">{fmtNotifTime(n.createdAt)}</p>
                        </div>
                        {!n.isRead && (
                          <div className="h-2 w-2 shrink-0 rounded-full bg-brand-500 mt-1.5" />
                        )}
                      </div>
                    ))
                  )}
                </div>

                <div className="pt-2 border-t border-gray-100 dark:border-gray-800 flex gap-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => { go('notifikasi'); toast.success('Membuka pusat notifikasi') }}
                    className="flex-1 justify-center py-1 font-medium text-brand-500 hover:underline"
                  >
                    Lihat Semua
                  </Button>
                  {unreadCount > 0 && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={markAllRead}
                      className="flex-1 justify-center py-1 font-medium text-muted-foreground hover:underline"
                    >
                      Tandai Dibaca
                    </Button>
                  )}
                </div>
              </DropdownMenuContent>
            </DropdownMenu>

            {/* User profile */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex h-auto items-center gap-1.5 rounded-full px-1.5 py-1.5 hover:bg-gray-100 dark:hover:bg-white/5 focus-visible:ring-2 focus-visible:ring-brand-500/10 sm:gap-3 sm:px-2">
                  <div className="flex h-9 w-9 items-center justify-center rounded-full bg-brand-500 text-white font-bold text-xs">
                    {getInitials(displayName)}
                  </div>
                  <div className="hidden md:flex flex-col text-left">
                    <span className="text-sm font-semibold text-gray-800 dark:text-white/90 leading-tight">
                      {displayName}
                    </span>
                    <span className="text-[11px] font-medium text-gray-500 dark:text-gray-400">
                      {formatRoleLabel(user.role)}
                    </span>
                  </div>
                  <ChevronDown className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                align="end"
                className="w-56 rounded-xl border-gray-200 dark:border-gray-800"
              >
                <DropdownMenuLabel className="font-normal">
                  <div className="flex flex-col space-y-1">
                    <p className="text-sm font-semibold leading-none text-gray-800 dark:text-white/90">
                      {displayName}
                    </p>
                    <p className="text-xs leading-none text-gray-500 dark:text-gray-400">
                      Hak Akses: {formatRoleLabel(user.role)}
                    </p>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                {user.role === 'admin' && (
                  <DropdownMenuItem
                    onClick={() => go('akun')}
                    className="cursor-pointer"
                  >
                    <UserIcon className="mr-2 h-4 w-4 text-gray-500 dark:text-gray-400" />
                    <span>Pengaturan Akun</span>
                  </DropdownMenuItem>
                )}
                {user.role === 'guru' && (
                  <>
                    <DropdownMenuItem onClick={onOpenTutorAccount} className="cursor-pointer">
                      <UserIcon className="mr-2 h-4 w-4 text-gray-500 dark:text-gray-400" />
                      <span>Pengaturan Akun</span>
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onClick={() => go('dokumen-tutor')}
                      className="cursor-pointer"
                    >
                      <FileText className="mr-2 h-4 w-4 text-gray-500 dark:text-gray-400" />
                      <span>Dokumen Tutor</span>
                    </DropdownMenuItem>
                  </>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => setLogoutModalOpen(true)}
                  className="cursor-pointer text-error-500 focus:bg-error-50 focus:text-error-600 font-medium dark:focus:bg-error-500/10"
                >
                  <LogOut className="mr-2 h-4 w-4" />
                  <span>Keluar</span>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </header>

      {/* Logout confirmation */}
      <AlertDialog open={logoutModalOpen} onOpenChange={setLogoutModalOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <div className="flex items-center gap-2 text-error-500 mb-1">
              <ShieldAlert className="h-5 w-5" />
              <AlertDialogTitle>Konfirmasi Keluar</AlertDialogTitle>
            </div>
            <AlertDialogDescription>
              Apakah Anda yakin ingin keluar dari Sistem Informasi LMS PKBM Tunas Ilmu?
              Sesi Anda akan diakhiri dan Anda perlu masuk kembali.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirmLogout}
              className="bg-error-500 text-white hover:bg-error-600"
            >
              Ya, Keluar
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
