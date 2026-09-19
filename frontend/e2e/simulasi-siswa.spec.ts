import { expect, test } from '@playwright/test'

// This intentionally uses an explicitly provisioned learner account instead
// of assuming a fixture exists in every deployment. Set both variables in an
// E2E environment after assigning a published simulation to that learner.
const username = process.env.E2E_SIMULASI_STUDENT
const password = process.env.E2E_SIMULASI_PASSWORD

test('siswa membaca instruksi, autosave, menandai, melanjutkan, dan mengirim simulasi', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_STUDENT dan E2E_SIMULASI_PASSWORD untuk menjalankan alur siswa.')
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username!)
  await page.getByPlaceholder('Masukkan password').fill(password!)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page.getByText('Simulasi ANBK/TKA SD', { exact: true })).toBeVisible()

  const enter = page.getByRole('button', { name: /Lihat instruksi|Lanjutkan/i }).first()
  await expect(enter).toBeEnabled()
  await enter.click()
  if (await page.getByRole('button', { name: /Mulai simulasi/i }).count()) {
    await page.getByRole('button', { name: /Mulai simulasi/i }).click()
  }
  await expect(page.getByText(/Soal 1 dari/i)).toBeVisible()
  await page.getByRole('button', { name: /Tandai/i }).click()
  await expect(page.getByText(/Ditandai/i)).toBeVisible()
  await page.reload()
  await expect(page.getByText('Simulasi ANBK/TKA SD', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: /Lanjutkan/i }).first().click()
  await expect(page.getByText(/Soal 1 dari/i)).toBeVisible()

  // Save the first selectable answer when the package begins with a choice
  // item. The assertion below is deliberately policy-neutral: a package may
  // hide scores and/or wait for manual essay grading.
  const option = page.locator('input[type="radio"]').first()
  if (await option.count()) await option.check()
  await expect(page.getByText(/Tersimpan|Menyimpan/i)).toBeVisible()
  page.once('dialog', (dialog) => dialog.accept())
  await page.getByRole('button', { name: /Selesai & kirim|Kirim jawaban/i }).click()
  await expect(page.getByText(/Simulasi selesai|menunggu penilaian/i)).toBeVisible()
})
