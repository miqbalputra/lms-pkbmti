import { readFileSync } from 'node:fs'
import { expect, test } from '@playwright/test'
import { saveFixtureGradeCP } from './report-grade-cp-fixture'

test('CP descriptions: 1/4/5 boundaries, server parity, and stored tutor text', async ({ page, request, context }) => {
  test.skip(process.env.E2E_RAPOR_ISOLATED !== '1', 'Requires a fresh TestReportBrowserServer in-memory fixture.')
  expect(await (await request.get('/__report-audit')).json()).toMatchObject({ isolated: true, storage: 'memory' })
  await context.addInitScript(() => localStorage.setItem('pkbmti-lms-install-prompt:last-dismissed-at', String(Date.now())))
  const cases = JSON.parse(readFileSync(new URL('../../backend/cmd/server/testdata/report_narrative_cases.json', import.meta.url), 'utf8'))
  const example = cases.find((row: { name: string }) => row.name === 'five CP selects two each')
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill('admin')
  await page.getByPlaceholder('Masukkan password').fill('Admin123')
  const loginResponse = page.waitForResponse(response => response.url().endsWith('/api/auth/login') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  const { accessToken } = await (await loginResponse).json()
  await expect(page).toHaveURL(/dashboard/)
  await page.getByRole('navigation', { name: 'Menu aplikasi', exact: true }).getByRole('link', { name: 'Modul Nilai', exact: true }).click()
  async function selectClass() {
    await page.getByLabel('Kelas rapor').selectOption({ label: 'Kelas 1 R' })
    await page.getByLabel('Mata pelajaran rapor').selectOption({ label: 'Bahasa Indonesia' })
  }
  await selectClass()
  const classId = await page.getByLabel('Kelas rapor').inputValue()
  const mapelId = await page.getByLabel('Mata pelajaran rapor').inputValue()
  async function save() {
    const response = page.waitForResponse(res => res.url().includes('/rapor-sumatif/grid?') && res.request().method() === 'PUT')
    await page.getByRole('button', { name: 'Simpan nilai', exact: true }).click()
    expect((await response).status()).toBe(200)
    await expect(page.getByText('Perubahan belum disimpan', { exact: true })).toHaveCount(0)
  }
  await saveFixtureGradeCP(request, accessToken, classId, mapelId, example.labels)
  await page.reload()
  await selectClass()
  for (const [i, value] of (example.values as number[]).entries()) await page.getByLabel(`Siswa A CP ${i + 1}`, { exact: true }).fill(String(value))
  await expect(page.getByLabel('Capaian Siswa A', { exact: true })).toHaveValue(example.strength)
  await expect(page.getByLabel('Penguatan Siswa A', { exact: true })).toHaveValue(example.growth)
  await save()
  const qs = `?kelasId=${classId}&semester=Ganjil&mapelId=${mapelId}`
  const headers = { Authorization: `Bearer ${accessToken}` }
  async function getGrid() {
    const response = await request.get('/api/rapor-sumatif/grid' + qs, { headers })
    expect(response.status()).toBe(200)
    return response.json()
  }
  let grid = await getGrid()
  const studentA = grid.students.find((student: { nama: string }) => student.nama === 'Siswa A')
  const docResponse = await request.get(`/api/rapor-sumatif/document/${studentA.id}${qs}`, { headers })
  expect(docResponse.status()).toBe(200)
  const subject = (await docResponse.json()).document.subjects.find((row: { mapelId: string }) => row.mapelId === mapelId)
  expect(subject).toMatchObject({ strength: example.strength, growth: example.growth, score: 80 })
  const custom = 'Deskripsi manual lama: Ananda sangat baik dalam membaca Indonesia, sesuai catatan tutor.'
  const customGrowth = 'Pendampingan lama tetap disimpan persis.'
  await page.getByLabel('Capaian Siswa A', { exact: true }).fill(custom)
  await page.getByLabel('Penguatan Siswa A', { exact: true }).fill(customGrowth)
  await save()
  await page.reload()
  await selectClass()
  await expect(page.getByLabel('Capaian Siswa A', { exact: true })).toHaveValue(custom)
  await page.getByLabel('Siswa A CP 5', { exact: true }).fill('100')
  await save()
  await expect(page.getByLabel('Capaian Siswa A', { exact: true })).toHaveValue(custom)
  await expect(page.getByLabel('Penguatan Siswa A', { exact: true })).toHaveValue(customGrowth)

  await page.getByRole('button', { name: 'Susun ulang deskripsi Siswa A', exact: true }).click()
  await expect(page.getByLabel('Capaian Siswa A', { exact: true })).toHaveValue('Ananda menunjukkan penguasaan yang baik dalam hal membaca huruf, suku kata, dan kata serta menggambar benda.')
  grid = await getGrid()
  expect(grid.narratives[0]).toMatchObject({ studentId: studentA.id, strength: custom, growth: customGrowth })
  // Discard the regenerated draft; the old stored text must still return.
  page.once('dialog', dialog => dialog.accept())
  await page.reload()
  await selectClass()
  await expect(page.getByLabel('Capaian Siswa A', { exact: true })).toHaveValue(custom)
  await expect(page.getByLabel('Penguatan Siswa A', { exact: true })).toHaveValue(customGrowth)

  // The fifth CP exists, but its empty score must not make four scored CPs use two per section.
  for (let i = 0; i < 4; i++) await page.getByLabel(`Siswa B CP ${i + 1}`, { exact: true }).fill(String(example.values[i]))
  await expect(page.getByLabel('Capaian Siswa B', { exact: true })).toHaveValue('Ananda menunjukkan penguasaan yang baik dalam hal membaca huruf, suku kata, dan kata.')
  await expect(page.getByLabel('Penguatan Siswa B', { exact: true })).toHaveValue('Perlu peningkatan dalam hal menceritakan pengalaman di Indonesia.')
  await page.getByLabel('Siswa B CP 5', { exact: true }).fill(String(example.values[4]))
  await expect(page.getByLabel('Capaian Siswa B', { exact: true })).toHaveValue(example.strength)
  await expect(page.getByLabel('Penguatan Siswa B', { exact: true })).toHaveValue(example.growth)
  for (let i = 1; i < 5; i++) await page.getByLabel(`Siswa B CP ${i + 1}`, { exact: true }).fill('')
  await expect(page.getByLabel('Capaian Siswa B', { exact: true })).toHaveValue('Ananda menunjukkan penguasaan yang baik dalam hal membaca huruf, suku kata, dan kata.')
  await expect(page.getByLabel('Penguatan Siswa B', { exact: true })).toHaveValue('')
  await save()
  grid = await getGrid()
  expect(grid.narratives).toHaveLength(1)
  expect(grid.narratives[0]).toMatchObject({ studentId: studentA.id, strength: custom, growth: customGrowth })
})
