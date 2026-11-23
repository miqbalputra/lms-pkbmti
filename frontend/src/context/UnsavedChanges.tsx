import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useBlocker } from 'react-router-dom'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../components/ui/dialog'

import { UnsavedChangesContext as Context, type Guard } from './useUnsavedChanges'

export function UnsavedChangesProvider({ children }: { children: ReactNode }) {
  const guard = useRef<Guard | null>(null)
  const [pending, setPending] = useState<(() => void) | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [, update] = useState(0)
  const blocker = useBlocker(useCallback(() => {
    const state = guard.current?.read()
    return !!state && (state.dirty || state.busy)
  }, []))
  const register = useCallback((entry: Guard) => {
    guard.current = entry
    update(v => v + 1)
    return () => { if (guard.current === entry) guard.current = null }
  }, [])
  const leave = useCallback((action: () => void) => {
    const state = guard.current?.read()
    if (state && (state.dirty || state.busy)) { setError(''); setPending(() => action) }
    else action()
  }, [])
  useEffect(() => {
    const handle = (event: BeforeUnloadEvent) => {
      const state = guard.current?.read()
      if (state && (state.dirty || state.busy)) { event.preventDefault(); event.returnValue = '' }
    }
    window.addEventListener('beforeunload', handle)
    return () => window.removeEventListener('beforeunload', handle)
  }, [])

  const open = !!pending || blocker.state === 'blocked'
  const busy = saving || !!guard.current?.read().busy
  function stay() {
    if (saving) return
    setPending(null); setError('')
    if (blocker.state === 'blocked') blocker.reset()
  }
  async function resolve(save: boolean) {
    const state = guard.current?.read()
    if (!state || state.busy || saving) return
    setSaving(true); setError('')
    try {
      if (save) await state.save()
      else await state.discard()
      await new Promise<void>(resolve => requestAnimationFrame(() => resolve()))
      if (guard.current?.read().dirty || guard.current?.read().busy) {
        throw new Error('Perubahan belum selesai disimpan. Tetap di halaman ini dan periksa isian Anda.')
      }
      const action = pending
      setPending(null)
      if (blocker.state === 'blocked') blocker.proceed()
      else action?.()
    } catch (e) { setError(e instanceof Error ? e.message : 'Penyimpanan gagal. Isian Anda tetap dipertahankan.') }
    finally { setSaving(false) }
  }
  return <Context.Provider value={{ register, leave }}>{children}
    <Dialog open={open} onOpenChange={value => { if (!value) stay() }}>
      <DialogContent onInteractOutside={event => event.preventDefault()} onEscapeKeyDown={event => { if (saving) event.preventDefault() }}>
        <DialogHeader><DialogTitle>Ada isian yang belum disimpan</DialogTitle><DialogDescription>Simpan isian sebelum melanjutkan. Jika memilih buang perubahan, hanya isian yang belum disimpan yang dibatalkan; data yang sudah tersimpan tetap ada.</DialogDescription></DialogHeader>
        {busy && <p role="status" className="text-sm">Sedang memproses penyimpanan. Tunggu hingga selesai.</p>}
        {error && <p role="alert" className="rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-800">{error}</p>}
        <DialogFooter><Button variant="outline" disabled={saving} onClick={stay}>Tetap di halaman</Button><Button variant="outline" disabled={busy} onClick={() => void resolve(false)}>Buang perubahan</Button><Button disabled={busy} onClick={() => void resolve(true)}>Simpan &amp; lanjutkan</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </Context.Provider>
}
