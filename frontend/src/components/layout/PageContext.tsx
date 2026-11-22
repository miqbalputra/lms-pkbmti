import { Link, useLocation } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'
import { NAV_ITEMS } from './nav'
import { MENU_GUIDES } from './menu-guide'
import { pageFromPath, pathFor } from '../../lib/router'

export function PageContext({ role }: { role: string }) {
  const { pathname } = useLocation()
  const id = pageFromPath(pathname)
  const item = NAV_ITEMS.find(v => v.id === id && v.roles.includes(role))
  const label = item?.label || (id === 'notifikasi' ? 'Notifikasi' : '')
  if (!label) return null
  return <div className="mb-4 rounded-xl border border-border bg-card px-4 py-3">
    <nav aria-label="Lokasi halaman" className="flex flex-wrap items-center gap-2 text-sm"><Link to={pathFor('dashboard')} className="font-medium text-primary hover:underline">Beranda</Link>{id !== 'dashboard' && <><ChevronRight aria-hidden="true" className="size-4 text-muted-foreground" /><span aria-current="page" className="font-semibold">{label}</span></>}</nav>
    <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{MENU_GUIDES[id]}</p>
  </div>
}

export function QuickActions({ role }: { role: string }) {
  const ids = role === 'admin' ? ['kelas', 'penugasan', 'presensi', 'rapor'] : ['jurnal-mengajar', 'presensi', 'nilai', 'rapor']
  return <section aria-label="Akses cepat" className="space-y-3"><h2 className="text-lg font-semibold">Mulai dari sini</h2><div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{ids.map(id => {
    const item = NAV_ITEMS.find(v => v.id === id && v.roles.includes(role))
    if (!item) return null
    const Icon = item.icon
    return <Link key={id} to={pathFor(id)} className="rounded-xl border border-border bg-card p-4 transition hover:border-primary hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"><Icon aria-hidden="true" className="mb-2 size-5 text-primary" /><h3 className="font-semibold">{item.label}</h3><p className="mt-1 text-xs leading-relaxed text-muted-foreground">{MENU_GUIDES[id]}</p></Link>
  })}</div></section>
}
