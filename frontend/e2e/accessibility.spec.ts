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

async function expectVisibleControlsNamed(page: import('@playwright/test').Page) {
  const roleNames = ['button', 'link', 'textbox', 'combobox', 'checkbox', 'radio'] as const
  for (const role of roleNames) {
    const controls = await page.getByRole(role).all()
    for (const control of controls) {
      if (await control.isVisible()) await expect(control, `${role} harus memiliki nama aksesibel`).toHaveAccessibleName(/\S+/)
    }
  }
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

test('kontrol editor soal tutor memiliki nama yang terbaca teknologi bantu', async ({ page }) => {
  test.skip(!teacher || !teacherPassword, 'Set E2E_ACCESSIBILITY_TEACHER dan password untuk menjalankan audit aksesibilitas staf.')
  await page.setViewportSize({ width: 1440, height: 900 })
  await login(page, teacher!, teacherPassword!)
  const workspace = page.getByRole('navigation', { name: 'Menu aplikasi' }).getByRole('button', { name: 'Simulasi & Bank Soal', exact: true })
  await expect(workspace).toBeVisible()
  await workspace.click()
  await page.getByRole('tab', { name: 'Soal' }).click()
  await page.getByRole('button', { name: 'Buat soal' }).click()
  await expect(page.getByRole('textbox', { name: 'Tulis pertanyaan untuk siswa', exact: true })).toBeVisible()

  await expectVisibleControlsNamed(page)
})
