import { expect, test, type Page } from '@playwright/test'

test.beforeEach(async ({ request }) => {
  test.skip(process.env.E2E_MENU_ISOLATED !== '1', 'Requires TestMenuAuditServer: isolated database and server-side write barrier.')
  const res = await request.get('/__menu-audit')
  expect(await res.json()).toMatchObject({ isolated: true, storage: 'temporary', writes: 'blocked' })
})

async function login(page: Page, username: string) {
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username)
  await page.getByPlaceholder('Masukkan password').fill(username === 'admin' ? 'Admin123' : 'Guru12345')
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/dashboard/)
}

for (const username of ['admin', 'guru-audit', 'guru-mapel-audit']) {
  test(`${username}: search, keyboard, mobile drawer, and role-specific menus`, async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await login(page, username)
    const menu = page.getByRole('navigation', { name: 'Menu aplikasi', exact: true })
    await expect(menu.getByRole('link')).toHaveCount(username === 'admin' ? 54 : 28)
    if (username !== 'admin') {
      await expect(page.getByText('Guru / Tutor', { exact: true })).toBeVisible()
      await expect(menu.getByRole('link', { name: 'Backup & Restore', exact: true })).toHaveCount(0)
    }
    await expect(page.getByRole('region', { name: 'Akses cepat' })).toBeVisible()
    await page.getByLabel('Cari menu aplikasi', { exact: true }).fill('rapor')
    const result = page.getByRole('button', { name: 'Rapor', exact: true })
    await result.focus()
    await page.keyboard.press('Enter')
    await expect(page).toHaveURL(/\/rapor$/)
    await expect(menu.getByRole('link', { name: 'Rapor', exact: true })).toHaveAttribute('aria-current', 'page')
    await page.getByRole('button', { name: 'Ciutkan sidebar', exact: true }).click()
    await page.mouse.move(1000, 100)
    await expect(menu.getByRole('link', { name: 'Rapor', exact: true })).toHaveAccessibleName('Rapor')
    await page.setViewportSize({ width: 375, height: 812 })
    await expect(page.locator('#primary-navigation')).toHaveAttribute('inert', '')
    const trigger = page.getByRole('button', { name: 'Buka menu navigasi', exact: true })
    await trigger.click()
    const search = page.getByLabel('Cari menu pada navigasi', { exact: true })
    await expect(search).toBeFocused()
    await search.fill('absen')
    await expect(menu.getByRole('link', { name: 'Presensi Mingguan', exact: true })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.locator('#primary-navigation')).toHaveAttribute('inert', '')
    await expect(trigger).toBeFocused()
    await trigger.click()
    await search.fill('rapor')
    await menu.getByRole('link', { name: 'Rapor', exact: true }).click()
    await expect(page.locator('#primary-navigation')).toHaveAttribute('inert', '')
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  })
}

test('admin: parent portal opens once in a new tab and cloud backup stays idle when unconfigured', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await login(page, 'admin')
  const popupEvent = page.waitForEvent('popup')
  await page.getByRole('navigation', { name: 'Menu aplikasi', exact: true }).getByRole('link', { name: 'Portal Orang Tua', exact: true }).click()
  const popup = await popupEvent
  await expect(popup).toHaveURL(/\/orangtua$/)
  await expect(popup.locator('body')).toContainText(/Portal Orang Tua/)
  await expect(page).toHaveURL(/dashboard/)
  await popup.close()
  const archiveCalls: string[] = []
  page.on('request', req => { if (req.url().includes('/backup/r2/archives')) archiveCalls.push(req.url()) })
  await page.getByRole('navigation', { name: 'Menu aplikasi', exact: true }).getByRole('link', { name: 'Backup & Restore', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Backup & Restore', exact: true })).toBeVisible()
  await expect(page.getByText('R2 belum dikonfigurasi di secret server.', { exact: true })).toBeVisible()
  expect(archiveCalls).toEqual([])
})

test('subject teacher: competency access, import defaults, report guidance and restricted admin routes', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await login(page, 'guru-mapel-audit')
  const menu = page.getByRole('navigation', { name: 'Menu aplikasi', exact: true })
  const response = page.waitForResponse(res => res.url().endsWith('/api/kompetensi'))
  await menu.getByRole('link', { name: 'Kompetensi', exact: true }).click()
  expect((await response).status()).toBe(200)
  await menu.getByRole('link', { name: 'Import Terpusat', exact: true }).click()
  await expect(page.getByLabel('Tipe Import', { exact: true })).toHaveValue('nilai-kompetensi')
  await expect(page.getByText(/Belum ada kelas wali untuk akun ini/)).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Riwayat Import' })).toHaveCount(0)
  await menu.getByRole('link', { name: 'Pusat Laporan', exact: true }).click()
  await expect(page.getByText(/Belum ada kelas wali untuk akun ini/)).toBeVisible()
  await page.goto('/backup')
  await expect(page.getByText('Akses ini hanya tersedia untuk Admin.')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Backup & Restore', exact: true })).toHaveCount(0)
})

test('mobile: installation reminder never covers the sign-in form', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto('/')
  // The reminder normally appears after three seconds; a slow login must stay usable.
  await page.waitForTimeout(3500)
  await expect(page.getByRole('complementary', { name: 'Instal aplikasi PKBM Tunas Ilmu' })).toHaveCount(0)
  await page.getByPlaceholder('Masukkan username atau email').fill('guru-audit')
  await page.getByPlaceholder('Masukkan password').fill('Guru12345')
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/dashboard/)
})
