import { useSidebar } from '../../context/useSidebar'
import { cn } from '../../lib/utils'
import { useLocation, useNavigate } from 'react-router-dom'
import { pageFromPath, pathFor } from '../../lib/router'
import { NAV_GROUPS } from './nav'

export function AppSidebar({ role }: { role: string }) {
  const { isExpanded, isMobileOpen, isHovered, setIsHovered, setMobileOpen } =
    useSidebar()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const currentPage = pageFromPath(pathname)

  const showLabels = isExpanded || isHovered || isMobileOpen

  const go = (pageId: string) => {
    navigate(pathFor(pageId))
    if (isMobileOpen) setMobileOpen(false)
  }

  return (
    <aside
      aria-label="Navigasi utama"
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
          onClick={() => go('dashboard')}
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
        <nav aria-label="Menu aplikasi" className="mb-6 px-4 lg:px-5">
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
                      const isActive = currentPage === item.id
                      return (
                        <li key={item.id}>
                          <button
                            onClick={() => go(item.id)}
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
