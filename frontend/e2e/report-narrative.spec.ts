import { readFileSync } from 'node:fs'
import { expect, test } from '@playwright/test'
import { automaticReportNarrative } from '../src/lib/report-narrative'

type Case = { name: string; labels: string[]; values: (number | null)[]; strength: string; growth: string }
const cases: Case[] = JSON.parse(readFileSync(new URL('../../backend/cmd/server/testdata/report_narrative_cases.json', import.meta.url), 'utf8'))

for (const fixture of cases) {
  test(`curriculum rule parity: ${fixture.name}`, () => {
    const cps = fixture.labels.map((label, index) => ({ id: String(index), label }))
    const scores = fixture.values.map((value, index) => ({ cpId: String(index), value }))
    const before = JSON.stringify({ cps, scores })
    expect(automaticReportNarrative(cps, scores)).toEqual({ strength: fixture.strength, growth: fixture.growth })
    expect(JSON.stringify({ cps, scores })).toBe(before)
  })
}
