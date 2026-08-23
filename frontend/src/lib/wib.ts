export const WIB_TIME_ZONE = 'Asia/Jakarta'

type DateValue = Date | string | number

function parts(now: Date) {
  const values = new Intl.DateTimeFormat('en-US', {
    timeZone: WIB_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    weekday: 'short',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(now)
  return Object.fromEntries(values.map((part) => [part.type, part.value])) as Record<string, string>
}

function parseDateValue(value: DateValue): Date | null {
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value
  const raw = String(value).trim()
  if (!raw) return null

  // Date and datetime-local inputs have no timezone. They are application
  // values in WIB, so never let the browser's local timezone reinterpret them.
  if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) {
    const date = new Date(`${raw}T00:00:00+07:00`)
    return Number.isNaN(date.getTime()) ? null : date
  }
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d{1,3})?)?$/.test(raw)) {
    const date = new Date(`${raw}+07:00`)
    return Number.isNaN(date.getTime()) ? null : date
  }
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? null : date
}

export function wibDateInputValue(value: unknown): string {
  if (value === null || value === undefined || value === '') return ''
  const date = parseDateValue(value as DateValue)
  if (!date) return String(value).slice(0, 10)
  const valueParts = parts(date)
  return `${valueParts.year}-${valueParts.month}-${valueParts.day}`
}

export function wibDateTimeLocalValue(value: unknown): string {
  if (value === null || value === undefined || value === '') return ''
  const date = parseDateValue(value as DateValue)
  if (!date) return String(value).slice(0, 16).replace(' ', 'T')
  const valueParts = parts(date)
  return `${valueParts.year}-${valueParts.month}-${valueParts.day}T${valueParts.hour}:${valueParts.minute}`
}

export function wibDateTimeLocalToISO(value: string): string {
  const date = parseDateValue(value)
  return date ? date.toISOString() : ''
}

export function formatWibDate(value: unknown, options: Intl.DateTimeFormatOptions = {}): string {
  if (value === null || value === undefined || value === '') return ''
  const date = parseDateValue(value as DateValue)
  if (!date) return String(value)
  return new Intl.DateTimeFormat('id-ID', {
    timeZone: WIB_TIME_ZONE,
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    ...options,
  }).format(date)
}

export function formatWibDateTime(value: unknown): string {
  if (value === null || value === undefined || value === '') return ''
  const date = parseDateValue(value as DateValue)
  if (!date) return String(value)
  return new Intl.DateTimeFormat('id-ID', {
    timeZone: WIB_TIME_ZONE,
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export function wibYear(now = new Date()): number {
  return Number(parts(now).year)
}

export function wibMonthIndex(now = new Date()): number {
  return Number(parts(now).month) - 1
}

/** Calendar date as seen in Indonesia western time, independent of browser timezone. */
export function wibToday(now = new Date()) {
  const value = parts(now)
  return `${value.year}-${value.month}-${value.day}`
}

export function isSaturdayWibDate(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const [year, month, day] = value.split('-').map(Number)
  return new Date(Date.UTC(year, month - 1, day)).getUTCDay() === 6
}

/** Returns today when it is Saturday, otherwise the next Saturday in WIB. */
export function nextSaturdayWib(now = new Date()) {
  const today = wibToday(now)
  if (isSaturdayWibDate(today)) return today
  const [year, month, day] = today.split('-').map(Number)
  const current = new Date(Date.UTC(year, month - 1, day))
  const delta = (6 - current.getUTCDay() + 7) % 7 || 7
  current.setUTCDate(current.getUTCDate() + delta)
  return current.toISOString().slice(0, 10)
}

export function isTodaySaturdayWib(now = new Date()) {
  return isSaturdayWibDate(wibToday(now))
}
