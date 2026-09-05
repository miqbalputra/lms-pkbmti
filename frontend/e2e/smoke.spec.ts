import { expect, type Page, test } from '@playwright/test'

async function login(page: Page, username: string, password: string) {
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username)
  await page.getByPlaceholder('Masukkan password').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test('admin dapat masuk dan membuka dashboard Backup R2', async ({ page }) => {
  await login(page, 'admin', process.env.E2E_ADMIN_PASSWORD || 'CiAdminPassword2026!')
  await page.goto('/backup')
  await expect(page.getByText('Backup & Restore', { exact: true })).toBeVisible()
  await expect(page.getByText('Backup Penuh Cloudflare R2', { exact: true })).toBeVisible()
})

test('guru dan kepala sekolah tidak dapat membuka restore R2 admin', async ({ browser }) => {
  for (const account of [['guru1', 'Guru1234'], ['kepala', 'Kepala123']] as const) {
    const context = await browser.newContext()
    const page = await context.newPage()
    await login(page, account[0], account[1])
    await page.goto('/backup')
    await expect(page.getByText('Akses ini hanya tersedia untuk Admin.')).toBeVisible()
    await context.close()
  }
})

test('portal orang tua dan ujian online publik dapat dibuka', async ({ page }) => {
  const backend = process.env.E2E_BACKEND_BASE_URL || process.env.E2E_BASE_URL || 'http://127.0.0.1:8080'
  await page.goto(`${backend}/orangtua`)
  await expect(page.locator('body')).toContainText(/Orang Tua|Portal/i)
  await page.goto(`${backend}/ujian`)
  await expect(page.locator('body')).toContainText(/Ujian/i)
})
