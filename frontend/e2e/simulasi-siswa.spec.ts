import { expect, test } from '@playwright/test'

// This intentionally uses an explicitly provisioned learner account instead
// of assuming a fixture exists in every deployment. Set both variables in an
// E2E environment after assigning a published simulation to that learner.
const username = process.env.E2E_SIMULASI_STUDENT
const password = process.env.E2E_SIMULASI_PASSWORD

async function expectMinimumTapTarget(locator: import('@playwright/test').Locator) {
  const box = await locator.boundingBox()
  expect(box?.width).toBeGreaterThanOrEqual(44)
  expect(box?.height).toBeGreaterThanOrEqual(44)
}

async function answerCurrentQuestion(page: import('@playwright/test').Page): Promise<'autosave' | 'upload' | false> {
  const truthRows = page.locator('table input[type="radio"][aria-label$=": Benar"]')
  if (await truthRows.count()) {
    let changed = false
    for (let index = 0; index < await truthRows.count(); index += 1) {
      const trueChoice = truthRows.nth(index)
      if (await trueChoice.isChecked()) continue
      const falseChoice = page.locator(`input[type="radio"][name="${await trueChoice.getAttribute('name')}"][aria-label$=": Salah"]`)
      if (await falseChoice.count() && await falseChoice.first().isChecked()) continue
      await trueChoice.check()
      changed = true
    }
    return changed ? 'autosave' : false
  }

  if (await page.getByText(/Pilih pernyataan di kiri/i).count()) {
    const leftItems = page.getByRole('button', { name: /^\d+\./ })
    let changed = false
    for (let index = 0; index < await leftItems.count(); index += 1) {
      const leftItem = leftItems.nth(index)
      if ((await leftItem.innerText()).includes('Sudah dipasangkan')) continue
      await leftItem.click()
      await page.getByRole('button', { name: /^Pilih pasangan/i }).first().click()
      changed = true
    }
    return changed ? 'autosave' : false
  }

  const matrixRows = page.locator('table tbody tr').filter({ has: page.locator('input[type="radio"], input[type="checkbox"]') })
  if (await matrixRows.count()) {
    let changed = false
    for (let index = 0; index < await matrixRows.count(); index += 1) {
      const row = matrixRows.nth(index)
      if (await row.locator('input:checked').count()) continue
      const option = row.locator('input[type="radio"], input[type="checkbox"]').first()
      if (await option.count()) {
        await option.check()
        changed = true
      }
    }
    return changed ? 'autosave' : false
  }

  const choices = page.locator('input[type="radio"], input[type="checkbox"]')
  for (let index = 0; index < await choices.count(); index += 1) {
    const choice = choices.nth(index)
    if (await choice.isVisible() && await choice.isEnabled() && !await choice.isChecked()) {
      await choice.check()
      return 'autosave'
    }
  }

  const scaleChoice = page.locator('[role="radiogroup"] [role="radio"][aria-checked="false"]').first()
  if (await scaleChoice.count() && await scaleChoice.isVisible()) {
    await scaleChoice.click()
    return 'autosave'
  }

  const orderMove = page.getByRole('button', { name: /Pindahkan .+ ke bawah/ }).first()
  if (await orderMove.count() && await orderMove.isEnabled()) {
    await orderMove.click()
    return 'autosave'
  }

  const dropdown = page.locator('select').first()
  if (await dropdown.count() && await dropdown.locator('option').count() > 1 && await dropdown.inputValue() === '') {
    await dropdown.selectOption({ index: 1 })
    return 'autosave'
  }

  const dateAnswer = page.locator('input[type="date"]').first()
  if (await dateAnswer.count() && await dateAnswer.isVisible() && !(await dateAnswer.inputValue())) {
    await dateAnswer.fill('2026-09-25')
    return 'autosave'
  }

  const timeAnswer = page.locator('input[type="time"]').first()
  if (await timeAnswer.count() && await timeAnswer.isVisible() && !(await timeAnswer.inputValue())) {
    await timeAnswer.fill('10:30')
    return 'autosave'
  }

  const dateTimeAnswer = page.locator('input[type="datetime-local"]').first()
  if (await dateTimeAnswer.count() && await dateTimeAnswer.isVisible() && !(await dateTimeAnswer.inputValue())) {
    await dateTimeAnswer.fill('2026-09-25T10:30')
    return 'upload'
  }

  const fileAnswer = page.locator('input[type="file"]').first()
  const uploadedTestFile = page.getByRole('button', { name: /jawaban-latihan\.pdf.*MB/ }).first()
  if (await uploadedTestFile.count()) return false
  if (await fileAnswer.count() && await fileAnswer.isEnabled()) {
    await fileAnswer.setInputFiles({
      name: 'jawaban-latihan.pdf',
      mimeType: 'application/pdf',
      buffer: Buffer.from('%PDF-1.4\n% E2E answer file\n%%EOF\n'),
    })
    await expect(page.getByRole('button', { name: /jawaban-latihan\.pdf.*MB/ }).first()).toBeVisible({ timeout: 10_000 })
    return 'autosave'
  }

  const textAnswer = page.locator('textarea, input[type="text"], input:not([type])').last()
  if (await textAnswer.count() && await textAnswer.isVisible() && !(await textAnswer.inputValue())) {
    await textAnswer.fill('Jawaban latihan yang cukup jelas.')
    return true
  }
  return false
}

async function expectAnswerSaved(page: import('@playwright/test').Page, type: 'autosave' | 'upload') {
  if (type === 'autosave') await page.waitForResponse((response) => response.url().includes('/jawaban/') && response.request().method() === 'PUT' && response.ok(), { timeout: 10_000 })
  await expect(page.getByText('Tersimpan', { exact: true })).toBeVisible({ timeout: 10_000 })
}

test('siswa membaca instruksi, autosave, menandai, melanjutkan, dan mengirim simulasi', async ({ page }) => {
  test.setTimeout(60_000)
  test.skip(!username || !password, 'Set E2E_SIMULASI_STUDENT dan E2E_SIMULASI_PASSWORD untuk menjalankan alur siswa.')
  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username!)
  await page.getByPlaceholder('Masukkan password').fill(password!)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page.getByRole('heading', { name: /Simulasi ANBK\s*\/\s*TKA SD/i })).toBeVisible()
  const dismissInstallPrompt = page.getByRole('button', { name: 'Nanti saja' })
  if (await dismissInstallPrompt.count()) await dismissInstallPrompt.click()

  const enter = page.getByRole('button', { name: /Lihat instruksi|Lanjutkan/i }).first()
  await expect(enter).toBeEnabled()
  await enter.click()
  const instructionConfirmation = page.getByText('Konfirmasi simulasi', { exact: true })
  const firstQuestion = page.getByText(/Soal 1 dari/i)
  await expect(instructionConfirmation.or(firstQuestion)).toBeVisible()
  if (await instructionConfirmation.isVisible() && await page.getByRole('button', { name: /Mulai simulasi/i }).count()) {
    await page.getByRole('button', { name: /Mulai simulasi/i }).click()
  }
  await expect(firstQuestion).toBeVisible()
  for (const name of ['Ukuran teks kecil', 'Ukuran teks sedang', 'Ukuran teks besar', 'Informasi soal', 'Daftar soal']) {
    await expectMinimumTapTarget(page.getByRole('button', { name }).first())
  }
  await expectMinimumTapTarget(page.getByRole('button', { name: /Ragu-ragu|Ditandai/ }).first())
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(375)
  const infoButton = page.getByRole('button', { name: 'Informasi soal' })
  await infoButton.click()
  const infoDialog = page.getByRole('dialog', { name: 'Informasi pengerjaan' })
  await expect(infoDialog).toBeVisible()
  await expectMinimumTapTarget(infoDialog.getByRole('button', { name: 'Tutup' }))
  expect(await infoDialog.evaluate((dialog) => dialog.contains(document.activeElement))).toBe(true)
  await page.keyboard.press('Escape')
  await expect(infoDialog).toHaveCount(0)
  await expect(infoButton).toBeFocused()
  const marked = page.getByRole('button', { name: 'Ditandai', exact: true })
  if (!await marked.count()) await page.getByRole('button', { name: 'Ragu-ragu', exact: true }).click()
  await expect(marked).toBeVisible()
  await page.reload()
  await expect(page.getByRole('heading', { name: /Simulasi ANBK\s*\/\s*TKA SD/i })).toBeVisible()
  await page.getByRole('button', { name: /Lanjutkan/i }).first().click()
  await expect(page.getByText(/Soal 1 dari/i)).toBeVisible()

  const firstAnswerChange = await answerCurrentQuestion(page)
  if (firstAnswerChange) await expectAnswerSaved(page, firstAnswerChange)
  await expect(page.getByText(/Tersimpan|Menyimpan/i)).toBeVisible({ timeout: 10_000 })
  await page.reload()
  await expect(page.getByRole('heading', { name: /Simulasi ANBK\s*\/\s*TKA SD/i })).toBeVisible()
  await page.getByRole('button', { name: /Lanjutkan/i }).first().click()
  await expect(page.getByText(/Soal 1 dari/i)).toBeVisible()
  const progressLabel = page.getByText(/^Soal \d+ dari \d+$/).first()
  const progressText = await progressLabel.textContent()
  const [, currentQuestion, totalQuestions] = progressText?.match(/Soal (\d+) dari (\d+)/) || []
  for (let questionNumber = Number(currentQuestion || 1); questionNumber <= Number(totalQuestions || 1); questionNumber += 1) {
    if (questionNumber > 1) {
      await page.getByRole('button', { name: /Berikutnya/i }).click()
      await expect(progressLabel).toContainText(`Soal ${questionNumber} dari`)
    }
    const answerChange = await answerCurrentQuestion(page)
    if (answerChange) await expectAnswerSaved(page, answerChange)
  }
  await page.getByRole('button', { name: /Kirim simulasi/i }).click()
  const reviewDialog = page.getByRole('dialog', { name: 'Periksa sebelum mengirim' })
  await expect(reviewDialog).toBeVisible()
  await expect(page.getByRole('dialog', { name: 'Periksa sebelum mengirim' })).toContainText(/0\s*Kosong/)
  const submitResponsePromise = page.waitForResponse((response) => response.url().includes('/kirim') && response.request().method() === 'POST')
  await page.getByRole('dialog', { name: 'Periksa sebelum mengirim' }).getByRole('button', { name: 'Ya, kirim simulasi' }).click()
  const submitResponse = await submitResponsePromise
  if (!submitResponse.ok()) throw new Error(`Kirim simulasi gagal (${submitResponse.status()}): ${await submitResponse.text()}`)
  await expect(page.getByRole('heading', { name: 'Simulasi selesai' })).toBeVisible()
  await page.getByRole('button', { name: /Kembali ke daftar/i }).click()
  const historyResult = page.getByRole('button', { name: /Lihat hasil · Percobaan/i }).first()
  await expect(historyResult).toBeVisible()
  await historyResult.click()
  await expect(page.getByRole('heading', { name: 'Simulasi selesai' })).toBeVisible()
})
