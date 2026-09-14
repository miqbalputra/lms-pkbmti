import { createContext } from 'react'

export type SidebarContextType = {
  isExpanded: boolean
  isMobileOpen: boolean
  isHovered: boolean
  toggleSidebar: () => void
  toggleMobileSidebar: () => void
  setMobileOpen: (open: boolean) => void
  setIsHovered: (isHovered: boolean) => void
}

export const SidebarContext = createContext<SidebarContextType | undefined>(undefined)
