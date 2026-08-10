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
  onLogout: () => void
  onOpenTutorAccount: () => void
  children: ReactNode
}

function ShellContent({
  user,
  token,
  onLogout,
  onOpenTutorAccount,
  children,
}: AppShellProps) {
  const { isExpanded, isHovered } = useSidebar()

  return (
    <div className="min-h-screen xl:flex bg-gray-50 dark:bg-gray-900">
      <div>
        <AppSidebar role={user.role} />
        <Backdrop />
      </div>
      <div
        className={`flex-1 flex flex-col min-w-0 transition-all duration-300 ease-in-out ${
          isExpanded || isHovered ? 'lg:ml-[290px]' : 'lg:ml-[90px]'
        }`}
      >
        <AppHeader
          token={token}
          user={user}
          onLogout={onLogout}
          onOpenTutorAccount={onOpenTutorAccount}
        />
        <div className="w-full min-w-0 p-3 mx-auto max-w-(--breakpoint-2xl) sm:p-4 md:p-6">
          {children}
        </div>
      </div>
    </div>
  )
}

export function AppShell({
  user,
  token,
  onLogout,
  onOpenTutorAccount,
  children,
}: AppShellProps) {
  return (
    <SidebarProvider>
      <ShellContent
        user={user}
        token={token}
        onLogout={onLogout}
        onOpenTutorAccount={onOpenTutorAccount}
      >
        {children}
      </ShellContent>
    </SidebarProvider>
  )
}
