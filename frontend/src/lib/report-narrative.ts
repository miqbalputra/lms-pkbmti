type CP = { id: string; label: string }
type Score = { cpId: string; value: number | null }

export const reportStrengthOpening = 'Ananda menunjukkan penguasaan yang baik dalam hal '
export const reportGrowthOpening = 'Perlu peningkatan dalam hal '

export function lowercaseCPStart(label: string) {
  const [first, ...rest] = Array.from(label)
  return first ? first.toLowerCase() + rest.join('') : ''
}

function cpClause(label: string) {
  return lowercaseCPStart(label.trim().replace(/\.+$/, '').trim())
}

// Keep this selection in parity with reportSelectNarrativeCPs on the server.
// Only generated text is normalized; saved narratives are handled by the caller.
export function automaticReportNarrative(cps: CP[], scores: Score[]) {
  const values = new Map(scores.map(score => [score.cpId, score.value]))
  const ranked = cps.map((cp, order) => ({ label: cpClause(cp.label), order, value: values.get(cp.id) }))
    .filter((cp): cp is { label: string; order: number; value: number } => cp.value != null && cp.label !== '')
    .sort((a, b) => b.value - a.value || a.order - b.order)
  const count = ranked.length > 4 ? 2 : 1
  const top = ranked.slice(0, count)
  const bottom = ranked.slice(count).sort((a, b) => a.value - b.value || a.order - b.order).slice(0, count)
  const sentence = (opening: string, items: typeof ranked) => items.length ? opening + items.map(cp => cp.label).join(' serta ') + '.' : ''
  return { strength: sentence(reportStrengthOpening, top), growth: sentence(reportGrowthOpening, bottom) }
}
