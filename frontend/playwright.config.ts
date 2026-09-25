import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  // The application deliberately rate-limits repeated sign-ins. Running several
  // browser specs against the same seeded accounts in parallel can lock those
  // shared fixtures and make otherwise independent accessibility checks flaky.
  workers: 1,
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  reporter: [['html', { outputFolder: 'playwright-report', open: 'never' }], ['list']],
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://127.0.0.1:8080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
