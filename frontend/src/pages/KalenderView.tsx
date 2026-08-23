import { useEffect, useState, type FormEvent } from 'react'
import { ChevronLeft, ChevronRight, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { FormCard, PageToolbar } from '../components/ui/page'
import { Select } from '../components/ui/select'
import { request } from '../lib/api'
import { formatWibDate, wibDateInputValue, wibMonthIndex, wibToday, wibDateTimeLocalToISO, wibYear } from '../lib/wib'

type Event = Record<string, unknown> & { id: string }

const tipeWarna: Record<string, string> = {
  libur: '#dc2626',
  ujian: '#2563eb',
  kegiatan: '#16a34a',
  upacara: '#9333ea',
  rapat: '#f59e0b',
}

const tipeOptions = [
  { value: 'libur', label: 'Libur' },
  { value: 'ujian', label: 'Ujian' },
  { value: 'kegiatan', label: 'Kegiatan' },
  { value: 'upacara', label: 'Upacara' },
  { value: 'rapat', label: 'Rapat' },
]

function daysInMonth(y: number, m: number) { return new Date(Date.UTC(y, m + 1, 0)).getUTCDate() }
function firstDayOfMonth(y: number, m: number) { return new Date(Date.UTC(y, m, 1)).getUTCDay() }

export function KalenderView({
  token,
  readOnly,
}: {
  token: string
  readOnly: boolean
}) {
  const [events, setEvents] = useState<Event[]>([])
  const [year, setYear] = useState(wibYear())
  const [month, setMonth] = useState(wibMonthIndex())
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ judul: '', deskripsi: '', tanggalMulai: '', tanggalSelesai: '', tipe: 'kegiatan', warna: '#16a34a' })

  useEffect(() => {
    const bulan = `${year}-${String(month + 1).padStart(2, '0')}`
    request(`/kalender?bulan=${bulan}`, token)
      .then((d) => setEvents(Array.isArray(d) ? d : []))
      .catch(() => {})
  }, [token, year, month])

  const prevMonth = () => { if (month === 0) { setMonth(11); setYear(y => y - 1) } else setMonth(m => m - 1) }
  const nextMonth = () => { if (month === 11) { setMonth(0); setYear(y => y + 1) } else setMonth(m => m + 1) }

  const handleCreate = (e: FormEvent) => {
    e.preventDefault()
    if (!form.judul || !form.tanggalMulai) { toast.error('Judul dan tanggal wajib diisi'); return }
    request('/kalender', token, 'POST', {
      judul: form.judul,
      deskripsi: form.deskripsi,
      tanggalMulai: wibDateTimeLocalToISO(form.tanggalMulai),
      tanggalSelesai: form.tanggalSelesai ? wibDateTimeLocalToISO(form.tanggalSelesai) : null,
      tipe: form.tipe,
      warna: form.warna || tipeWarna[form.tipe] || '#16a34a',
    })
      .then(() => {
        toast.success('Event kalender berhasil dibuat')
        setShowForm(false)
        setForm({ judul: '', deskripsi: '', tanggalMulai: '', tanggalSelesai: '', tipe: 'kegiatan', warna: '#16a34a' })
        const bulan = `${year}-${String(month + 1).padStart(2, '0')}`
        return request(`/kalender?bulan=${bulan}`, token)
      })
      .then((d) => setEvents(Array.isArray(d) ? d : []))
      .catch((e) => toast.error(String(e)))
  }

  const handleDelete = (id: string) => {
    if (!confirm('Hapus event ini?')) return
    request(`/kalender/${id}`, token, 'DELETE')
      .then(() => {
        toast.success('Event dihapus')
        setEvents((prev) => prev.filter((e) => e.id !== id))
      })
      .catch((e) => toast.error(String(e)))
  }

  const days = daysInMonth(year, month)
  const startDay = firstDayOfMonth(year, month)
  const monthName = new Intl.DateTimeFormat('id-ID', { timeZone: 'Asia/Jakarta', month: 'long', year: 'numeric' }).format(new Date(Date.UTC(year, month, 1)))

  // Map events by date
  const eventsByDate: Record<string, Event[]> = {}
  events.forEach((ev) => {
    const d = wibDateInputValue(ev.tanggalMulai)
    if (!eventsByDate[d]) eventsByDate[d] = []
    eventsByDate[d].push(ev)
  })

  const cells = []
  for (let i = 0; i < startDay; i++) cells.push(<div key={`e${i}`} className="h-20 bg-muted/30 rounded-lg" />)
  for (let d = 1; d <= days; d++) {
    const dateStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    const dayEvents = eventsByDate[dateStr] || []
    const isToday = wibToday() === dateStr
    cells.push(
      <div key={d} className={`h-20 rounded-lg border p-1.5 text-xs overflow-hidden ${isToday ? 'border-primary bg-primary/5' : 'border-border bg-card'}`}>
        <div className={`font-bold text-[11px] mb-1 ${isToday ? 'text-primary' : 'text-foreground'}`}>{d}</div>
        {dayEvents.map((ev) => (
          <div
            key={ev.id}
            className="rounded px-1 py-0.5 mb-0.5 truncate font-medium cursor-pointer hover:opacity-80"
            style={{ backgroundColor: String(ev.warna || tipeWarna[String(ev.tipe)] || '#ccc') + '22', color: String(ev.warna || tipeWarna[String(ev.tipe)] || '#333') }}
            title={String(ev.judul)}
          >
            {String(ev.judul).slice(0, 15)}
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Kalender Akademik"
        description="Jadwal kegiatan, ujian, dan libur akademik."
        actions={
          !readOnly ? (
            <Button onClick={() => setShowForm(!showForm)}>
              <Plus className="h-4 w-4" /> Tambah Event
            </Button>
          ) : undefined
        }
      />

      {showForm && (
        <FormCard title="Tambah Event Kalender" description="Isi detail event akademik.">
          <form className="grid gap-4 sm:grid-cols-2" onSubmit={handleCreate}>
            <div><Label>Judul</Label><Input value={form.judul} onChange={e => setForm({ ...form, judul: e.target.value })} required /></div>
            <div><Label>Tipe</Label>
              <Select value={form.tipe} onChange={e => setForm({ ...form, tipe: e.target.value, warna: tipeWarna[e.target.value] || '#16a34a' })}>
                {tipeOptions.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
              </Select>
            </div>
            <div><Label>Tanggal Mulai</Label><Input type="date" value={form.tanggalMulai} onChange={e => setForm({ ...form, tanggalMulai: e.target.value })} required /></div>
            <div><Label>Tanggal Selesai (opsional)</Label><Input type="date" value={form.tanggalSelesai} onChange={e => setForm({ ...form, tanggalSelesai: e.target.value })} /></div>
            <div className="sm:col-span-2"><Label>Deskripsi</Label><Input value={form.deskripsi} onChange={e => setForm({ ...form, deskripsi: e.target.value })} /></div>
            <div className="flex gap-2 sm:col-span-2">
              <Button type="submit">Simpan</Button>
              <Button variant="outline" type="button" onClick={() => setShowForm(false)}>Batal</Button>
            </div>
          </form>
        </FormCard>
      )}

      <Card className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
        <div className="flex items-center justify-between mb-4">
          <Button variant="outline" size="sm" onClick={prevMonth}><ChevronLeft className="h-4 w-4" /></Button>
          <h2 className="text-lg font-bold text-foreground">{monthName}</h2>
          <Button variant="outline" size="sm" onClick={nextMonth}><ChevronRight className="h-4 w-4" /></Button>
        </div>

        <div className="grid grid-cols-7 gap-1 mb-1">
          {['Min', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab'].map(d => (
            <div key={d} className="text-center text-[11px] font-semibold text-muted-foreground py-1">{d}</div>
          ))}
        </div>

        <div className="grid grid-cols-7 gap-1">
          {cells}
        </div>

        {/* Legend */}
        <div className="flex flex-wrap gap-3 mt-4 pt-4 border-t border-border">
          {tipeOptions.map(o => (
            <div key={o.value} className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <div className="w-2.5 h-2.5 rounded" style={{ backgroundColor: tipeWarna[o.value] }} />
              {o.label}
            </div>
          ))}
        </div>

        {/* Events list below calendar */}
        {events.length > 0 && (
          <div className="mt-4 pt-4 border-t border-border">
            <h3 className="text-sm font-semibold mb-2">Event Bulan Ini</h3>
            <div className="space-y-2">
              {events.map(ev => (
                <div key={ev.id} className="flex items-center gap-3 p-2 rounded-lg bg-muted/30 text-sm">
                  <div className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: String(ev.warna || tipeWarna[String(ev.tipe)]) }} />
                  <div className="flex-1 min-w-0">
                    <span className="font-medium">{String(ev.judul)}</span>
                    <span className="text-muted-foreground ml-2 text-xs">{formatWibDate(ev.tanggalMulai)}</span>
                  </div>
                  {!readOnly && (
                    <Button variant="ghost" size="sm" onClick={() => handleDelete(ev.id)}>
                      <Trash2 className="h-3 w-3 text-destructive" />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
