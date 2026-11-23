import { createContext, useContext, useEffect, useRef } from 'react'

export type FormState = { dirty: boolean; busy: boolean; save: () => Promise<void>; discard: () => Promise<void> }
export type Guard = { read: () => FormState }
export type Navigation = { register: (guard: Guard) => () => void; leave: (action: () => void) => void }
export const UnsavedChangesContext = createContext<Navigation | null>(null)

export function useGuardedNavigation() {
  const context = useContext(UnsavedChangesContext)
  if (!context) throw new Error('Navigation guard is unavailable')
  return context.leave
}

export function useUnsavedChanges(state: FormState) {
  const context = useContext(UnsavedChangesContext)
  if (!context) throw new Error('Navigation guard is unavailable')
  const current = useRef(state)
  current.current = state
  const { register } = context
  useEffect(() => register({ read: () => current.current }), [register, state.dirty, state.busy])
  return context.leave
}
