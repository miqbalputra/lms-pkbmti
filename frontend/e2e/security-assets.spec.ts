import { expect, test } from '@playwright/test'

test.beforeEach(async ({ request, context }) => {
  test.skip(process.env.E2E_MENU_ISOLATED !== '1', 'Requires the isolated menu audit server.')
  const marker = await (await request.get('/__menu-audit')).json()
  expect(marker).toMatchObject({ isolated: true, writes: 'blocked', productionCSP: true })
  // Do not run real anti-bot challenges against external services during tests.
  await context.route('https://challenges.cloudflare.com/**', route => route.fulfill({ status: 200, contentType: 'application/javascript', body: '' }))
})

for (const [pathname, family] of [['/', 'Outfit'], ['/orangtua', 'Inter'], ['/__menu-audit/exam', 'Inter']]) {
  test(`${pathname}: local assets load under production CSP`, async ({ page }) => {
    const errors: string[] = []
    const assets: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    // Resource status errors are checked below, where the expected logged-out
    // refresh response can be distinguished from actual asset failures.
    page.on('console', message => { if (message.type() === 'error' && !message.text().startsWith('Failed to load resource:')) errors.push(message.text()) })
    page.on('response', response => {
      if (response.status() >= 400 && !response.url().endsWith('/api/auth/refresh')) errors.push(`${new URL(response.url()).pathname}: ${response.status()}`)
    })
    page.on('request', request => {
      if (['font', 'stylesheet', 'script'].includes(request.resourceType())) assets.push(request.url())
    })
    const response = await page.goto(pathname)
    expect(response?.headers()['content-security-policy']).toContain("font-src 'self';")
    await expect.poll(() => page.evaluate(async (name) => {
      await document.fonts.load(`600 16px "${name}"`)
      return Array.from(document.fonts).some(font => font.family.replace(/["']/g, '') === name && font.status === 'loaded')
    }, family)).toBe(true)
    expect(assets.some(url => url.includes('/vendor/'))).toBe(true)
    expect(assets.filter(url => /fonts\.googleapis|fonts\.gstatic|cdn\.jsdelivr/.test(url))).toEqual([])
    if (pathname === '/orangtua') {
      const chart = page.locator('script[src="/vendor/chart.js-4.5.1/chart.umd.min.js"]')
      await expect(chart).toHaveAttribute('integrity', /^sha384-/)
      expect(await page.evaluate(() => typeof (window as any).Chart)).toBe('function')
    }
    if (pathname === '/') await expect(page.getByRole('button', { name: 'Masuk', exact: true })).toBeVisible()
    expect(errors).toEqual([])
  })
}

test('saved dark theme works with production CSP', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('theme', 'dark'))
  await page.goto('/')
  await expect(page.locator('html')).toHaveClass(/dark/)
  await expect(page.getByRole('button', { name: 'Masuk', exact: true })).toBeVisible()
})
