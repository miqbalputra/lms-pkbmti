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
  onOpenTutorAccount: () => void
  children: ReactNode
}

function ShellContent({
  user,
  token,
  page,
  setPage,
  onLogout,
  onOpenTutorAccount,
  children,
}: AppShellProps) {
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
        <AppHeader
          token={token}
          setPage={setPage}
          user={user}
          onLogout={onLogout}
          onOpenTutorAccount={onOpenTutorAccount}
        />
        <div className="p-4 mx-auto max-w-(--breakpoint-2xl) md:p-6 w-full">
          {children}
        </div>
      </div>
    </div>
  )
}

export function AppShell({
  user,
  token,
  page,
  setPage,
  onLogout,
  onOpenTutorAccount,
  children,
}: AppShellProps) {
  return (
    <SidebarProvider>
      <ShellContent
        user={user}
        token={token}
        page={page}
        setPage={setPage}
        onLogout={onLogout}
        onOpenTutorAccount={onOpenTutorAccount}
      >
        {children}
      </ShellContent>
    </SidebarProvider>
  )
}
