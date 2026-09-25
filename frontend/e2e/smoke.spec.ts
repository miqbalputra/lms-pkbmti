import { expect, type Page, test } from '@playwright/test'

function backendBaseURL() {
  const configuredBackend = process.env.E2E_BACKEND_BASE_URL
  if (configuredBackend) return configuredBackend
  const configuredApp = process.env.E2E_BASE_URL
  if (configuredApp && !/localhost:5173|127\.0\.0\.1:5173/.test(configuredApp)) return configuredApp
  return 'http://127.0.0.1:8080'
}

async function login(page: Page, username: string, password: string) {
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username)
  await page.getByPlaceholder('Masukkan password').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test('admin dapat masuk dan membuka dashboard Backup R2', async ({ page }) => {
  await login(page, 'admin', process.env.E2E_ADMIN_PASSWORD || 'CiAdminPassword2026!')
  await page.getByRole('button', { name: 'Backup & Restore', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Backup & Restore' })).toBeVisible()
  await expect(page.getByText('Backup Penuh Cloudflare R2', { exact: true })).toBeVisible()
})

test('guru dan kepala sekolah tidak dapat membuka restore R2 admin', async ({ browser }) => {
  for (const account of [['guru1', 'Guru1234'], ['kepala', 'Kepala123']] as const) {
    const context = await browser.newContext()
    const page = await context.newPage()
    await login(page, account[0], account[1])
    await page.evaluate(() => {
      window.history.pushState({}, '', '/backup')
      window.dispatchEvent(new PopStateEvent('popstate'))
    })
    await expect(page.getByText('Akses ini hanya tersedia untuk Admin.')).toBeVisible()
    await context.close()
  }
})

test('portal orang tua dan ujian online publik dapat dibuka', async ({ page }) => {
  const backend = backendBaseURL()
  await page.goto(`${backend}/orangtua`)
  await expect(page.locator('body')).toContainText(/Orang Tua|Portal/i)
  await page.goto(`${backend}/ujian`)
  await expect(page.locator('body')).toContainText(/Ujian/i)
})

test('form Ujian Online terbaca di desktop dan tetap di dalam layar ponsel', async ({ page }) => {
  const backend = backendBaseURL()
  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto(`${backend}/ujian`)
  const mobile = await page.evaluate(() => {
    const card = document.querySelector('#loginCard')!
    const button = document.querySelector('#cekBtn')!
    return {
      viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
      cardLeft: card.getBoundingClientRect().left,
      cardRight: card.getBoundingClientRect().right,
      buttonLeft: button.getBoundingClientRect().left,
      buttonRight: button.getBoundingClientRect().right,
    }
  })
  expect(mobile.documentWidth).toBeLessThanOrEqual(mobile.viewportWidth)
  expect(mobile.buttonLeft).toBeGreaterThanOrEqual(mobile.cardLeft)
  expect(mobile.buttonRight).toBeLessThanOrEqual(mobile.cardRight + 1)

  await page.setViewportSize({ width: 768, height: 1024 })
  const tablet = await page.evaluate(() => {
    const card = document.querySelector('#loginCard')!
    return {
      viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
      cardLeft: card.getBoundingClientRect().left,
      cardRight: card.getBoundingClientRect().right,
    }
  })
  expect(tablet.documentWidth).toBeLessThanOrEqual(tablet.viewportWidth)
  expect(tablet.cardLeft).toBeGreaterThanOrEqual(0)
  expect(tablet.cardRight).toBeLessThanOrEqual(tablet.viewportWidth)

  await page.setViewportSize({ width: 1440, height: 900 })
  const desktopFormWidth = await page.locator('#loginCard .login-form-panel').evaluate((form) => form.getBoundingClientRect().width)
  expect(desktopFormWidth).toBeGreaterThanOrEqual(580)
})

test('kontrol login Ujian Online tetap mudah disentuh pada layar 320px', async ({ page }) => {
  const backend = backendBaseURL()
  await page.setViewportSize({ width: 320, height: 740 })
  await page.goto(`${backend}/ujian`)
  const controls = await page.locator('#loginCard .input, #loginCard .btn').evaluateAll((elements) => elements.map((element) => element.getBoundingClientRect().height))
  expect(controls.length).toBeGreaterThan(0)
  expect(controls.every((height) => height >= 44)).toBe(true)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320)
})

test('dialog Ujian Online mengunci fokus keyboard dan mengembalikannya saat ditutup', async ({ page }) => {
  const backend = backendBaseURL()
  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto(`${backend}/ujian`)
  // Expose the exam workspace without creating an attempt; this test targets
  // the public page's standalone dialog behavior only.
  await page.evaluate(() => document.getElementById('examCard')?.classList.remove('hidden'))

  const launcher = page.locator('#examCard [data-action="open-info"]')
  await launcher.focus()
  await page.keyboard.press('Enter')
  const dialog = page.getByRole('dialog', { name: 'Informasi pengerjaan' })
  await expect(dialog).toBeVisible()
  const closeButton = dialog.getByRole('button', { name: 'Tutup' })
  await expect(closeButton).toBeFocused()

  await page.keyboard.press('Tab')
  await expect(closeButton).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
  await expect(launcher).toBeFocused()
})
