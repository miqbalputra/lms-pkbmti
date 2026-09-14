// Only allow navigations to explicit HTTP(S) URLs from data stored in the LMS.
// This protects old records as well as new input if a previously stored value
// contains a javascript:, data:, or malformed URL scheme.
export function safeExternalUrl(value: unknown): string {
  const raw = String(value ?? '').trim()
  if (!raw || raw.length > 2048) return ''
  try {
    const url = new URL(raw)
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.href : ''
  } catch {
    return ''
  }
}
