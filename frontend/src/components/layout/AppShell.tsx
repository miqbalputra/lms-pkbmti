import type { ReactNode } from 'react'
import { AppHeader } from './AppHeader'
import { AppSidebar } from './AppSidebar'
import { Backdrop } from './Backdrop'
import { SidebarProvider, useSidebar } from '../../context/SidebarContext'

interface User {
  id: string
  username: string
  role: string
}

interface AppShellProps {
  user: User
  token: string
  page: string
  setPage: (p: string) => void
  onLogout: () => void
  children: ReactNode
}

function ShellContent({
  user,
  page,
  setPage,
  onLogout,
  children,
}: Omit<AppShellProps, 'token'>) {
  const { isExpanded, isHovered } = useSidebar()

  return (
    <div className="min-h-screen xl:flex bg-gray-50 dark:bg-gray-900">
      <div>
        <AppSidebar page={page} setPage={setPage} role={user.role} />
        <Backdrop />
      </div>
      <div
        className={`flex-1 flex flex-col min-w-0 transition-all duration-300 ease-in-out ${
          isExpanded || isHovered ? 'lg:ml-[290px]' : 'lg:ml-[90px]'
        }`}
      >
        <AppHeader setPage={setPage} user={user} onLogout={onLogout} />
        <div className="p-4 mx-auto max-w-(--breakpoint-2xl) md:p-6 w-full">
          {children}
        </div>
      </div>
    </div>
  )
}

export function AppShell({
  user,
  page,
  setPage,
  onLogout,
  children,
}: AppShellProps) {
  return (
    <SidebarProvider>
      <ShellContent
        user={user}
        page={page}
        setPage={setPage}
        onLogout={onLogout}
      >
        {children}
      </ShellContent>
    </SidebarProvider>
  )
}