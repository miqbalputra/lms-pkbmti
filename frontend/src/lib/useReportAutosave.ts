import { useEffect, useEffectEvent } from 'react'
import { useUnsavedChangesDialogOpen } from '../context/useUnsavedChanges'

export function useReportAutosave(enabled: boolean, draft: unknown, save: () => Promise<void>) {
  const dialogOpen = useUnsavedChangesDialogOpen()
  const canSave = enabled && !dialogOpen
  const autosave = useEffectEvent(() => {
    if (canSave) void save().catch(() => {})
  })

  useEffect(() => {
    if (!canSave) return
    const timer = window.setTimeout(() => autosave(), 1200)
    return () => window.clearTimeout(timer)
  }, [canSave, draft])
}
