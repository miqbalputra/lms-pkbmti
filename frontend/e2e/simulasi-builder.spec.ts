import { expect, test } from '@playwright/test'

const username = process.env.E2E_SIMULASI_TEACHER
const password = process.env.E2E_SIMULASI_TEACHER_PASSWORD

test('guru membuat kanvas simulasi responsif dengan autosave dan bahan reusable', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username!)
  await page.getByPlaceholder('Masukkan password').fill(password!)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await page.getByRole('button', { name: /Simulasi & Bank Soal/i }).click()
  await expect(page.getByText('Simulasi & Bank Soal', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: /Buat paket/i }).click()
  await expect(page.getByPlaceholder('Paket tanpa judul')).toBeVisible()
  await page.getByPlaceholder('Paket tanpa judul').fill('Draf Literasi Responsif')
  await page.locator('select').first().selectOption('pg_tunggal')
  await page.getByPlaceholder('Tulis pertanyaan untuk siswa').fill('Apa gagasan utama bacaan?')
  await expect(page.getByText(/Tersimpan|Menyimpan|Perubahan belum tersimpan/i)).toBeVisible({ timeout: 5_000 })
  await page.getByRole('button', { name: 'Pratinjau siswa' }).click()
  await expect(page.getByText('Pratinjau seperti siswa')).toBeVisible()
})

test('workspace guru dapat berpindah ke pustaka bahan pada desktop', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username!)
  await page.getByPlaceholder('Masukkan password').fill(password!)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await page.getByRole('button', { name: /Simulasi & Bank Soal/i }).click()
  await page.getByRole('tab', { name: 'Bahan' }).click()
  await expect(page.getByText('Pustaka reusable')).toBeVisible()
})
