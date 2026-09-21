import { expect, test } from '@playwright/test'

const teacher = process.env.E2E_ACCESSIBILITY_TEACHER
const teacherPassword = process.env.E2E_ACCESSIBILITY_TEACHER_PASSWORD
const student = process.env.E2E_ACCESSIBILITY_STUDENT
const studentPassword = process.env.E2E_ACCESSIBILITY_STUDENT_PASSWORD

async function login(page: import('@playwright/test').Page, username: string, password: string) {
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username)
  await page.getByPlaceholder('Masukkan password').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
}

test('staf dapat memperbesar teks dan tetap tidak overflow di mobile', async ({ page }) => {
  test.skip(!teacher || !teacherPassword, 'Set E2E_ACCESSIBILITY_TEACHER dan password untuk menjalankan audit aksesibilitas staf.')
  await page.setViewportSize({ width: 375, height: 812 })
  await login(page, teacher!, teacherPassword!)
  await page.getByLabel('Pengaturan aksesibilitas').click()
  await page.getByRole('button', { name: 'Ukuran teks 125 persen' }).click()
  await expect.poll(() => page.evaluate(() => document.documentElement.style.getPropertyValue('--app-font-scale'))).toBe('1.25')
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('siswa memiliki kontrol tampilan yang dapat dioperasikan keyboard', async ({ page }) => {
  test.skip(!student || !studentPassword, 'Set E2E_ACCESSIBILITY_STUDENT dan password untuk menjalankan audit aksesibilitas siswa.')
  await page.setViewportSize({ width: 375, height: 812 })
  await login(page, student!, studentPassword!)
  const control = page.getByLabel('Pengaturan tampilan aksesibilitas')
  await expect(control).toBeVisible()
  await control.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('button', { name: 'Kontras tinggi' })).toBeVisible()
})
