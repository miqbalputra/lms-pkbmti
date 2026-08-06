const WIB_TIME_ZONE = 'Asia/Jakarta'

function parts(now: Date) {
  const values = new Intl.DateTimeFormat('en-US', {
    timeZone: WIB_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    weekday: 'short',
  }).formatToParts(now)
  return Object.fromEntries(values.map((part) => [part.type, part.value])) as Record<string, string>
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
