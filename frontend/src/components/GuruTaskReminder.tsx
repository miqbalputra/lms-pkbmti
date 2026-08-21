import { useCallback, useEffect, useRef, useState } from 'react'
import { CalendarCheck, ClipboardList, Clock3 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import type { User } from '../App'
import { pathFor } from '../lib/router'
import { request } from '../lib/api'
import { Button } from './ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from './ui/dialog'

const REMINDER_INTERVAL_MS = 60 * 60 * 1000

type AttendanceReminder = {
  classId: string
  classLabel: string
  date: string
  actualDate?: string
  reason: string
  meetingId?: string
}

type JournalReminder = {
  classId: string
  classLabel: string
  date: string
}

type ReminderResponse = {
  presensi?: AttendanceReminder[]
  jurnal?: JournalReminder[]
}

function storageKey(userID: string) {
  return `guru-task-reminder:last-dismissed:${userID}`
}

function readLastDismissed(userID: string) {
  try {
    const value = Number(window.localStorage.getItem(storageKey(userID)))
    return Number.isFinite(value) && value > 0 ? value : 0
  } catch {
    return 0
  }
}

function saveLastDismissed(userID: string, timestamp: number) {
  try {
    window.localStorage.setItem(storageKey(userID), String(timestamp))
  } catch {
    // Storage can be unavailable in private/restricted browser sessions. The
    // reminder still works for the open tab through its in-memory timer.
  }
}

function formatDate(date: string) {
  const parsed = new Date(`${date}T00:00:00+07:00`)
  if (Number.isNaN(parsed.getTime())) return date
  return new Intl.DateTimeFormat('id-ID', {
    timeZone: 'Asia/Jakarta', day: 'numeric', month: 'long', year: 'numeric',
  }).format(parsed)
}

export function GuruTaskReminder({ token, user }: { token: string; user: User }) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [attendance, setAttendance] = useState<AttendanceReminder[]>([])
  const [journals, setJournals] = useState<JournalReminder[]>([])
  const scheduleRef = useRef<(delay: number) => void>(() => undefined)

  const dismiss = useCallback(() => {
    const dismissedAt = Date.now()
    saveLastDismissed(user.id, dismissedAt)
    setOpen(false)
    scheduleRef.current(REMINDER_INTERVAL_MS)
  }, [user.id])

  useEffect(() => {
    if (user.role !== 'guru') {
      setOpen(false)
      return
    }

    let disposed = false
    let timer: ReturnType<typeof window.setTimeout> | undefined

    const schedule = (delay: number) => {
      if (timer) window.clearTimeout(timer)
      timer = window.setTimeout(() => {
        void checkReminders()
      }, Math.max(0, delay))
    }

    const checkReminders = async () => {
      const remaining = readLastDismissed(user.id) + REMINDER_INTERVAL_MS - Date.now()
      if (remaining > 0) {
        schedule(remaining)
        return
      }
      try {
        const data = await request('/dashboard/guru-reminders', token) as ReminderResponse
        if (disposed) return
        const nextAttendance = Array.isArray(data.presensi) ? data.presensi : []
        const nextJournals = Array.isArray(data.jurnal) ? data.jurnal : []
        setAttendance(nextAttendance)
        setJournals(nextJournals)
        setOpen(nextAttendance.length > 0 || nextJournals.length > 0)
      } catch {
        // A transient request failure must not block the dashboard. Retry on
        // the normal hourly cadence instead of showing an empty/error popup.
      } finally {
        if (!disposed) schedule(REMINDER_INTERVAL_MS)
      }
    }

    scheduleRef.current = schedule
    void checkReminders()
    return () => {
      disposed = true
      if (timer) window.clearTimeout(timer)
      scheduleRef.current = () => undefined
    }
  }, [token, user.id, user.role])

  function completeAttendance(reminder: AttendanceReminder) {
    dismiss()
    const params = new URLSearchParams()
    if (reminder.meetingId) {
      params.set('presensiId', reminder.meetingId)
    } else {
      params.set('kelasId', reminder.classId)
      params.set('tanggal', reminder.date)
    }
    navigate(`${pathFor('presensi')}?${params.toString()}`)
  }

  function completeJournal(reminder: JournalReminder) {
    dismiss()
    const params = new URLSearchParams({ kelasId: reminder.classId, tanggal: reminder.date })
    navigate(`${pathFor('jurnal-mengajar')}?${params.toString()}`)
  }

  if (user.role !== 'guru') return null

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => (nextOpen ? setOpen(true) : dismiss())}>
      <DialogContent className="max-w-xl gap-5">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 pr-6">
            <Clock3 className="h-5 w-5 text-primary" /> Pengingat tugas mengajar
          </DialogTitle>
          <DialogDescription>
            Lengkapi tugas kelas berikut agar data pembelajaran tetap mutakhir. Pengingat akan muncul lagi satu jam setelah ditutup bila masih ada tugas.
          </DialogDescription>
        </DialogHeader>

        {attendance.length > 0 && (
          <section className="space-y-3">
            <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <CalendarCheck className="h-4 w-4 text-primary" /> Presensi Kelas belum diisi
            </h3>
            <div className="space-y-2">
              {attendance.map((reminder) => (
                <div key={`${reminder.classId}-${reminder.date}`} className="rounded-xl border border-border bg-muted/30 p-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-foreground">{reminder.classLabel} · {formatDate(reminder.date)}</p>
                      <p className="mt-1 text-xs text-muted-foreground">Belum lengkap: {reminder.reason}{reminder.actualDate ? `; dilaksanakan ${formatDate(reminder.actualDate)}` : ''}</p>
                    </div>
                    <Button size="sm" onClick={() => completeAttendance(reminder)}>Lengkapi di sini</Button>
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}

        {journals.length > 0 && (
          <section className="space-y-3">
            <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <ClipboardList className="h-4 w-4 text-primary" /> Jurnal Kelas belum diisi
            </h3>
            <div className="space-y-2">
              {journals.map((reminder) => (
                <div key={`${reminder.classId}-${reminder.date}`} className="rounded-xl border border-border bg-muted/30 p-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <p className="text-sm font-medium text-foreground">{reminder.classLabel} · {formatDate(reminder.date)}</p>
                    <Button size="sm" onClick={() => completeJournal(reminder)}>Lengkapi di sini</Button>
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}

        <div className="flex justify-end">
          <Button variant="outline" onClick={dismiss}>Tutup</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
