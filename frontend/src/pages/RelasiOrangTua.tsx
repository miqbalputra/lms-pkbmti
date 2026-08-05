import { useEffect, useState } from 'react'
import { ArrowRight, GraduationCap, Search, Users } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { PageToolbar } from '../components/ui/page'
import { cn } from '../lib/utils'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

type Ortu = { id: string; namaBapak: string; namaIbu: string; nikAyah: string; nikIbu: string }
type Child = {
  id: string
  nama: string
  jenisKelamin: string
  nik: string
  nis: string
  nisn: string
  status: string
  kelasLabel: string
}
type RelasiGroup = { orangTua: Ortu; children: Child[]; anakCount: number }

async function request(path: string, token: string, method = 'GET', body?: unknown) {
  const r = await fetch(apiBase + path, {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: body ? JSON.stringify(body) : undefined,
  })
  const result = r.status === 204 ? null : await r.json().catch(() => ({}))
  if (!r.ok) throw new Error((result as { error?: string })?.error || 'Permintaan gagal')
  return result
}

const AVATAR_COLORS = [
  'bg-rose-100 text-rose-700',
  'bg-blue-100 text-blue-700',
  'bg-emerald-100 text-emerald-700',
  'bg-amber-100 text-amber-700',
  'bg-violet-100 text-violet-700',
  'bg-cyan-100 text-cyan-700',
  'bg-pink-100 text-pink-700',
  'bg-indigo-100 text-indigo-700',
  'bg-teal-100 text-teal-700',
  'bg-orange-100 text-orange-700',
]

function colorFor(s: string): string {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return AVATAR_COLORS[h % AVATAR_COLORS.length]
}

function initialsOf(s: string): string {
  const parts = s.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

function ortuLabel(o: Ortu): string {
  if (o.id === '') return 'Siswa tanpa orang tua'
  const b = o.namaBapak || '-'
  const i = o.namaIbu || '-'
  return `Bpk. ${b} & Ibu ${i}`
}

function ortuInitials(o: Ortu): string {
  if (o.id === '') return '?'
  const a = initialsOf(o.namaBapak || '')
  const b = initialsOf(o.namaIbu || '')
  if (!o.namaBapak && !o.namaIbu) return '?'
  if (!o.namaBapak) return b
  if (!o.namaIbu) return a
  return (a[0] + b[0]).toUpperCase()
}

function shortNik(nik: string): string {
  if (!nik) return ''
  return nik.length > 8 ? `${nik.slice(0, 4)}…${nik.slice(-4)}` : nik
}

export function RelasiOrangTua({ token }: { token: string; readOnly?: boolean }) {
  const [data, setData] = useState<RelasiGroup[]>([])
  const [q, setQ] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    const handle = setTimeout(() => {
      const path = '/orang-tua/relasi' + (q ? '?q=' + encodeURIComponent(q) : '')
      request(path, token)
        .then((r) => {
          setData((r as RelasiGroup[]) || [])
          setSelectedIndex((prev) => {
            const len = ((r as RelasiGroup[]) || []).length
            if (len === 0) return 0
            return prev >= len ? 0 : prev
          })
        })
        .catch(() => setData([]))
        .finally(() => setLoading(false))
    }, 250)
    return () => clearTimeout(handle)
  }, [token, q])

  const families = data.filter((g) => g.orangTua.id !== '')
  const orphan = data.find((g) => g.orangTua.id === '')
  const totalChildren = data.reduce((n, g) => n + g.children.length, 0)

  const selected = data[selectedIndex]
  const isOrphan = selected?.orangTua.id === ''

  return (
    <div>
      <PageToolbar
        title="Relasi Orang Tua → Anak"
        description="Pilih sebuah keluarga untuk melihat anak-anaknya (saudara sekandung) di sekolah. Berguna untuk mengetahui jumlah anak per orang tua."
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="Cari nama / NIK / NIS…"
                className="w-64 rounded-xl pl-9"
              />
            </div>
            <Badge variant="secondary" className="gap-1 rounded-lg px-2.5 py-1.5 text-xs">
              <Users className="h-3.5 w-3.5" /> {families.length} keluarga
            </Badge>
            <Badge variant="secondary" className="gap-1 rounded-lg px-2.5 py-1.5 text-xs">
              <GraduationCap className="h-3.5 w-3.5" /> {totalChildren} anak
            </Badge>
            {orphan ? (
              <Badge variant="outline" className="gap-1 rounded-lg px-2.5 py-1.5 text-xs">
                {orphan.anakCount} tanpa ortu
              </Badge>
            ) : null}
          </div>
        }
      />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-stretch">
        {/* Left panel: family cards */}
        <Card className="flex-1 lg:max-w-[40%] rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
          <div className="border-b border-border/60 px-4 py-3">
            <p className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
              Daftar Keluarga
            </p>
          </div>
          <div className="max-h-[72vh] space-y-2 overflow-auto p-3">
            {data.length === 0 && (
              <div className="flex flex-col items-center justify-center gap-2 py-16 text-center">
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-secondary">
                  <Users className="h-6 w-6 text-muted-foreground" />
                </div>
                <p className="text-sm text-muted-foreground">
                  {loading ? 'Memuat…' : q ? 'Tidak ada keluarga yang cocok.' : 'Belum ada data orang tua.'}
                </p>
              </div>
            )}
            {data.map((g, i) => {
              const sel = i === selectedIndex
              const orphanFlag = g.orangTua.id === ''
              const key = g.orangTua.id || '__orphan__'
              return (
                <button
                  key={key}
                  type="button"
                  onClick={() => setSelectedIndex(i)}
                  className={cn(
                    'group flex w-full items-center gap-3 rounded-2xl border p-3 text-left transition-all',
                    sel
                      ? 'border-primary bg-primary/5 shadow-md ring-1 ring-primary/30'
                      : 'border-border bg-card hover:border-primary/40 hover:bg-secondary/40'
                  )}
                >
                  <div
                    className={cn(
                      'flex h-11 w-11 shrink-0 items-center justify-center rounded-full text-sm font-bold',
                      orphanFlag ? 'bg-muted text-muted-foreground' : colorFor(ortuLabel(g.orangTua))
                    )}
                  >
                    {orphanFlag ? <Users className="h-5 w-5" /> : ortuInitials(g.orangTua)}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-foreground">
                      {ortuLabel(g.orangTua)}
                    </p>
                    <p className="truncate text-[11px] text-muted-foreground">
                      {orphanFlag
                        ? 'Belum ditautkan ke orang tua'
                        : [
                            g.orangTua.nikAyah && `NIK A: ${shortNik(g.orangTua.nikAyah)}`,
                            g.orangTua.nikIbu && `NIK I: ${shortNik(g.orangTua.nikIbu)}`,
                          ]
                            .filter(Boolean)
                            .join(' · ') || 'NIK belum diisi'}
                    </p>
                  </div>
                  <div className="flex flex-col items-center">
                    <span
                      className={cn(
                        'flex h-7 min-w-7 items-center justify-center rounded-full px-2 text-xs font-bold',
                        sel ? 'bg-primary text-primary-foreground' : 'bg-secondary text-foreground'
                      )}
                    >
                      {g.anakCount}
                    </span>
                    <span className="mt-0.5 text-[10px] text-muted-foreground">anak</span>
                  </div>
                </button>
              )
            })}
          </div>
        </Card>

        {/* Connector between panels */}
        <div className="hidden lg:flex flex-col items-center justify-center w-10 shrink-0">
          <div className="h-px w-full bg-border" />
          <div className="-mt-3 flex h-7 w-7 items-center justify-center rounded-full border border-primary/30 bg-card shadow-sm">
            <ArrowRight className="h-4 w-4 text-primary" />
          </div>
          <div className="h-px w-full flex-1 bg-border" />
        </div>

        {/* Right panel: children of selected family */}
        <Card className="flex-1 rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
          {selected ? (
            <>
              <div className="border-b border-border/60 bg-gradient-to-r from-primary/10 to-transparent px-5 py-4">
                <div className="flex items-center gap-3">
                  <div
                    className={cn(
                      'flex h-12 w-12 shrink-0 items-center justify-center rounded-full text-base font-bold',
                      isOrphan ? 'bg-muted text-muted-foreground' : colorFor(ortuLabel(selected.orangTua))
                    )}
                  >
                    {isOrphan ? <Users className="h-6 w-6" /> : ortuInitials(selected.orangTua)}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                      {isOrphan ? 'Grup khusus' : 'Keluarga'}
                    </p>
                    <p className="truncate text-lg font-extrabold text-foreground">
                      {ortuLabel(selected.orangTua)}
                    </p>
                    {!isOrphan && (selected.orangTua.nikAyah || selected.orangTua.nikIbu) ? (
                      <p className="truncate text-[11px] text-muted-foreground">
                        NIK Bapak: {selected.orangTua.nikAyah || '-'} · NIK Ibu: {selected.orangTua.nikIbu || '-'}
                      </p>
                    ) : null}
                  </div>
                  <Badge variant={isOrphan ? 'outline' : 'secondary'} className="shrink-0">
                    {isOrphan ? 'Tanpa orang tua' : `${selected.children.length} saudara`}
                  </Badge>
                </div>
                {isOrphan ? (
                  <p className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-[11px] text-amber-800">
                    Siswa-siswa ini belum ditautkan ke record orang tua. Tautkan/diubah dari halaman Peserta Didik.
                  </p>
                ) : (
                  <p className="mt-2 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                    <Users className="h-3.5 w-3.5" /> Anak-anak di bawah ini adalah saudara sekandung.
                  </p>
                )}
              </div>

              <div className="max-h-[64vh] overflow-auto p-5">
                {selected.children.length === 0 ? (
                  <div className="flex flex-col items-center justify-center gap-2 py-16 text-center">
                    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-secondary">
                      <GraduationCap className="h-6 w-6 text-muted-foreground" />
                    </div>
                    <p className="text-sm text-muted-foreground">Belum ada anak terhubung dengan keluarga ini.</p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {selected.children.map((c) => (
                      <div key={c.id} className="flex items-stretch gap-0">
                        {/* Tree connector trunk */}
                        <div className="relative flex w-6 shrink-0 justify-center">
                          <span className="absolute top-0 bottom-0 w-0.5 bg-primary/25" />
                          <span className="relative z-10 mt-3 flex h-6 w-6 items-center justify-center rounded-full bg-primary text-[10px] font-bold text-primary-foreground ring-4 ring-primary/10">
                            <GraduationCap className="h-3 w-3" />
                          </span>
                        </div>
                        {/* Child card */}
                        <div className="ml-2 flex-1 rounded-2xl border border-border bg-card p-3 shadow-2xs transition-colors hover:border-primary/30">
                          <div className="flex items-center gap-3">
                            <div
                              className={cn(
                                'flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-xs font-bold',
                                colorFor(c.nama)
                              )}
                            >
                              {initialsOf(c.nama)}
                            </div>
                            <div className="min-w-0 flex-1">
                              <p className="truncate text-sm font-semibold text-foreground">
                                {c.nama || '-'}
                              </p>
                              <p className="text-[11px] text-muted-foreground">
                                {c.jenisKelamin === 'P' ? 'Perempuan' : 'Laki-laki'} · NIK {c.nik || '-'}
                              </p>
                              <p className="text-[11px] text-muted-foreground">
                                NIS {c.nis || '-'} / NISN {c.nisn || '-'}
                              </p>
                            </div>
                            <div className="flex flex-col items-end gap-1">
                              <Badge variant="secondary" className="whitespace-nowrap">
                                {c.kelasLabel}
                              </Badge>
                              <Badge variant="outline" className="whitespace-nowrap text-[10px]">
                                {c.status}
                              </Badge>
                            </div>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </>
          ) : (
            <div className="flex h-full flex-col items-center justify-center gap-3 px-6 py-24 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary/10">
                <Users className="h-8 w-8 text-primary" />
              </div>
              <p className="text-sm font-semibold text-foreground">Pilih keluarga di sebelah kiri</p>
              <p className="max-w-xs text-xs text-muted-foreground">
                Klik salah satu kartu keluarga untuk melihat anak-anaknya dan hubungan saudara sekandungnya.
              </p>
            </div>
          )}
        </Card>
      </div>
    </div>
  )
}