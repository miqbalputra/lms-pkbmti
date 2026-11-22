import { useEffect, useRef, useState } from 'react'
import { Search, X } from 'lucide-react'
import { useSidebar } from '../../context/useSidebar'
import { cn } from '../../lib/utils'
import { Link, useLocation } from 'react-router-dom'
import { pageFromPath, pathFor } from '../../lib/router'
import { NAV_GROUPS } from './nav'
import { MENU_GUIDES, matchesMenu } from './menu-guide'

export function AppSidebar({ role }: { role: string }) {
  const { isExpanded, isMobile, isMobileOpen, isHovered, setIsHovered, setMobileOpen } =
    useSidebar()
  const [query, setQuery] = useState('')
  const asideRef = useRef<HTMLElement>(null)
  const { pathname } = useLocation()
  const currentPage = pageFromPath(pathname)

  const showLabels = isExpanded || isHovered || isMobileOpen
  useEffect(() => {
    if (!isMobile || !isMobileOpen) return
    const previous = document.activeElement as HTMLElement | null
    asideRef.current?.querySelector<HTMLInputElement>('input')?.focus()
    return () => previous?.focus()
  }, [isMobile, isMobileOpen])

  const closeAfterNavigation = () => {
    if (isMobileOpen) setMobileOpen(false)
  }

  return (
    <aside
      id="primary-navigation"
      ref={asideRef}
      aria-label="Navigasi utama"
      aria-hidden={isMobile && !isMobileOpen ? true : undefined}
      inert={isMobile && !isMobileOpen}
      onKeyDown={e => {
        if (!isMobileOpen) return
        if (e.key === 'Escape') { e.preventDefault(); setMobileOpen(false) }
        if (e.key === 'Tab') {
          const controls = Array.from(asideRef.current?.querySelectorAll<HTMLElement>('a,button,input') || []).filter(el => el.getClientRects().length)
          const first = controls[0], last = controls[controls.length - 1]
          if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last?.focus() }
          else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first?.focus() }
        }
      }}
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
        <Link
          to={pathFor('dashboard')}
          onClick={closeAfterNavigation}
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
        </Link>
      </div>

      {isMobileOpen && <button type="button" aria-label="Tutup menu navigasi" onClick={() => setMobileOpen(false)} className="absolute right-2 top-3 flex size-11 items-center justify-center rounded-lg lg:hidden"><X aria-hidden="true" className="size-5" /></button>}

      {showLabels && <div className="px-4 pb-4">
        <div className="relative"><Search aria-hidden="true" className="pointer-events-none absolute left-3 top-3.5 size-4 text-gray-400" /><input id="sidebar-menu-search" aria-label="Cari menu pada navigasi" placeholder="Cari menu, misalnya absen..." value={query} onChange={e => setQuery(e.target.value)} className="h-11 w-full rounded-lg border border-gray-200 bg-transparent pl-9 pr-10 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/20 dark:border-gray-700" />{query && <button type="button" aria-label="Hapus pencarian menu" className="absolute right-0 top-0 flex size-11 items-center justify-center" onClick={() => setQuery('')}><X className="size-4" aria-hidden="true" /></button>}</div>
        <p className="mt-2 px-1 text-xs text-gray-500 dark:text-gray-400">{role === 'admin' ? 'Menu administrator' : role === 'guru' ? 'Menu guru / tutor' : 'Menu akun Anda'}</p>
      </div>}

      {/* Navigation */}
      <div className="flex flex-col overflow-y-auto duration-300 ease-linear no-scrollbar flex-1">
        <nav aria-label="Menu aplikasi" className="mb-6 px-4 lg:px-5">
          <div className="flex flex-col gap-3 lg:gap-4">
            {NAV_GROUPS.map((group) => {
              const filtered = group.items.filter((item) =>
                item.roles.includes(role) && matchesMenu(item, query)
              )
              if (filtered.length === 0) return null
              return (
                <div key={group.groupLabel}>
                  <h2
                    className={cn(
                      'mb-3 px-2 text-theme-xs uppercase flex leading-[18px] font-medium text-gray-500 dark:text-gray-400 lg:mb-4',
                      !isExpanded && !isHovered ? 'lg:justify-center' : 'justify-start'
                    )}
                  >
                    {showLabels ? group.groupLabel : '•'}
                  </h2>
                  <ul className="flex flex-col gap-1.5">
                    {filtered.map((item) => {
                      const Icon = item.icon
                      const isActive = currentPage === item.id
                      return (
                        <li key={item.id}>
                          <Link
                            to={item.id === 'portal-ortu' ? '/orangtua' : pathFor(item.id)}
                            target={item.id === 'portal-ortu' ? '_blank' : undefined}
                            rel={item.id === 'portal-ortu' ? 'noopener noreferrer' : undefined}
                            data-menu-id={item.id}
                            onClick={closeAfterNavigation}
                            aria-label={item.label}
                            aria-current={isActive ? 'page' : undefined}
                            title={MENU_GUIDES[item.id] ? `${item.label}: ${MENU_GUIDES[item.id]}` : item.label}
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
                              <Icon className="size-6" aria-hidden="true" />
                            </span>
                            {showLabels && (
                              <span className="whitespace-normal break-words text-left leading-5">{item.label}</span>
                            )}
                          </Link>
                        </li>
                      )
                    })}
                  </ul>
                </div>
              )
            })}
            {!NAV_GROUPS.some(group => group.items.some(item => item.roles.includes(role) && matchesMenu(item, query))) && <p role="status" className="px-2 py-4 text-sm text-gray-500">Menu tidak ditemukan. Coba kata lain atau hapus pencarian.</p>}
          </div>
        </nav>
      </div>
    </aside>
  )
}
