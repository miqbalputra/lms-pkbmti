import { expect, test } from '@playwright/test'

const username = process.env.E2E_SIMULASI_TEACHER
const password = process.env.E2E_SIMULASI_TEACHER_PASSWORD

async function openSimulasiWorkspace(page: import('@playwright/test').Page) {
  const workspace = page.getByRole('button', { name: /Simulasi & Bank Soal/i })
  if (await page.evaluate(() => window.innerWidth < 1024)) {
    await page.locator('header button[aria-label="Buka menu navigasi"]').click()
  } else if (!await workspace.isVisible()) {
    await page.locator('header button[aria-label="Lebarkan sidebar"]').click()
  }
  await workspace.click()
}

async function loginAsTeacher(page: import('@playwright/test').Page) {
  await page.goto('/')
  await page.getByPlaceholder('Masukkan username atau email').fill(username!)
  await page.getByPlaceholder('Masukkan password').fill(password!)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test('guru membuat kanvas simulasi responsif dengan autosave dan bahan reusable', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 375, height: 812 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await expect(page.getByText('Simulasi & Bank Soal', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: /Buat paket/i }).click()
  await expect(page.getByLabel('Judul paket simulasi')).toBeVisible()
  await page.getByLabel('Judul paket simulasi').fill('Draf Literasi Responsif')
  await page.getByLabel('Tema visual paket').selectOption('#166534')
  await page.getByLabel('Pesan setelah siswa mengirim').fill('Terima kasih, jawabanmu sudah tersimpan.')
  await page.getByRole('button', { name: 'Tambah bagian' }).first().click()
  await page.getByRole('textbox', { name: 'Nama bagian 1', exact: true }).fill('Bagian Literasi')
  await page.getByLabel('Tambah jenis pertanyaan').selectOption('pg_tunggal')
  await page.getByPlaceholder('Tulis pertanyaan untuk siswa').fill('Apa gagasan utama bacaan?')
  await page.getByRole('button', { name: 'Tambah bagian' }).first().click()
  await page.getByLabel('Bagian soal yang sedang dipilih').selectOption({ label: 'Bagian Literasi' })
  await expect(page.getByText('Alur berdasarkan jawaban')).toBeVisible()
  await page.getByText('Alur berdasarkan jawaban').click()
  await page.getByLabel('Arah untuk jawaban Pilihan A').selectOption({ label: 'Lanjut ke: Bagian 2' })
  await expect(page.getByLabel('Bagian soal yang sedang dipilih')).toHaveValue(/section-/)
  await expect(page.locator('header').getByText(/Tersimpan|Menyimpan|Perubahan belum tersimpan/i)).toBeVisible({ timeout: 5_000 })
  await page.getByRole('button', { name: 'Pratinjau siswa' }).click()
  await expect(page.getByText('Pratinjau seperti siswa').first()).toBeVisible()
})

test('workspace guru dapat berpindah ke pustaka bahan pada desktop', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 1440, height: 900 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await page.getByRole('tab', { name: 'Bahan' }).click()
  await expect(page.getByText('Pustaka reusable')).toBeVisible()
})

test('kanvas tutor nyaman di tablet tanpa scroll horizontal', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 768, height: 1024 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await page.getByRole('button', { name: /Buat paket/i }).click()
  await page.getByLabel('Tambah jenis pertanyaan').selectOption('pg_tunggal')

  const question = page.getByPlaceholder('Tulis pertanyaan untuk siswa')
  await expect(question).toBeVisible()
  const bounds = await question.boundingBox()
  expect(bounds?.x).toBeGreaterThanOrEqual(0)
  expect((bounds?.x || 0) + (bounds?.width || 0)).toBeLessThanOrEqual(768)
  expect(bounds?.height).toBeGreaterThanOrEqual(44)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(768)
})

test('guru dapat mengubah jenis soal dengan seret dan jatuhkan tanpa syntax', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 1440, height: 900 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await page.getByRole('tab', { name: 'Soal' }).click()
  await page.getByRole('button', { name: 'Buat soal' }).click()

  await page.getByText('Pilih lewat kartu seret').click()
  const typeCard = page.getByRole('button', { name: 'Seret jenis soal Benar / Salah' })
  await typeCard.dragTo(page.getByPlaceholder('Tulis pertanyaan untuk siswa'))

  await expect(typeCard).toHaveClass(/border-primary/)
  await expect(page.getByText('Pernyataan benar / salah')).toBeVisible()
  await expect(page.locator('textarea[placeholder*="konfigurasi" i], textarea[aria-label*="json" i]')).toHaveCount(0)
})

test('guru dapat mengurungkan dan mengulangi perubahan kanvas paket', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 1440, height: 900 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await page.getByRole('button', { name: /Buat paket/i }).click()

  const title = page.getByLabel('Judul paket simulasi')
  await title.fill('Draf undo dan redo')
  await page.getByRole('button', { name: 'Urungkan perubahan' }).click()
  await expect(title).toHaveValue('')
  await page.getByRole('button', { name: 'Ulangi perubahan' }).click()
  await expect(title).toHaveValue('Draf undo dan redo')

  await page.getByLabel('Tambah jenis pertanyaan').selectOption('pg_tunggal')
  await expect(page.getByText('Pertanyaan 1', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Urungkan perubahan' }).click()
  await expect(page.getByText('Pertanyaan 1', { exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Ulangi perubahan' }).click()
  await expect(page.getByText('Pertanyaan 1', { exact: true })).toBeVisible()
  await page.keyboard.press('Control+z')
  await expect(page.getByText('Pertanyaan 1', { exact: true })).toHaveCount(0)
  await page.keyboard.press('Control+y')
  await expect(page.getByText('Pertanyaan 1', { exact: true })).toBeVisible()
})

test('guru dapat membuat, mengurutkan, menugaskan, mempratinjau, dan menerbitkan paket dengan seluruh tipe soal', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  test.setTimeout(90_000)
  await page.setViewportSize({ width: 1440, height: 900 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await page.getByRole('button', { name: /Buat paket/i }).click()

  const packageTitle = `Paket lengkap E2E ${Date.now()}`
  await page.getByLabel('Judul paket simulasi').fill(packageTitle)

  const questionTypes: Array<[string, string]> = [
    ['pg_tunggal', 'pilihan ganda'],
    ['pg_kompleks', 'pilihan ganda kompleks'],
    ['benar_salah', 'benar salah'],
    ['menjodohkan', 'menjodohkan'],
    ['isian_singkat', 'jawaban singkat'],
    ['uraian', 'uraian'],
    ['dropdown', 'dropdown'],
    ['skala_linear', 'skala linear'],
    ['rating', 'rating'],
    ['kisi_pg', 'kisi pilihan tunggal'],
    ['kisi_checkbox', 'kisi kotak centang'],
    ['tanggal', 'tanggal'],
    ['waktu', 'waktu'],
    ['susun_urutan', 'susun urutan'],
    ['unggah_berkas', 'unggah berkas'],
  ]

  for (const [type, label] of questionTypes) {
    await page.getByLabel('Tambah jenis pertanyaan').selectOption(type)
    await page.getByPlaceholder('Tulis pertanyaan untuk siswa').fill(`Pertanyaan ${label}`)
  }

  const questionSidebar = page.locator('aside').filter({ has: page.getByText('Soal (15)', { exact: true }) })
  await expect(questionSidebar.getByRole('button', { name: /1\. Pertanyaan pilihan ganda/ })).toBeVisible()
  await page.getByRole('button', { name: 'Pindahkan soal 1 ke bawah' }).click()
  await expect(questionSidebar.getByRole('button', { name: /1\. Pertanyaan pilihan ganda kompleks/ })).toBeVisible()

  await page.getByLabel('Acak urutan soal').check()
  await page.getByRole('checkbox', { name: /Semua peserta aktif/ }).check()
  await expect(page.getByText('✓ Peserta ditugaskan', { exact: true })).toBeVisible()
  await expect(page.locator('header').getByText('Tersimpan', { exact: true })).toBeVisible({ timeout: 15_000 })

  const previewPromise = page.waitForEvent('popup')
  await page.getByRole('button', { name: 'Tab baru', exact: true }).click()
  const preview = await previewPromise
  await expect(preview.getByRole('heading', { name: packageTitle })).toBeVisible()
  await expect(preview.getByText('15 soal · 60 menit', { exact: true })).toBeVisible()
  for (const [, label] of questionTypes) await expect(preview.getByText(`Pertanyaan ${label}`, { exact: true })).toBeVisible()
  await preview.close()

  await page.getByRole('button', { name: 'Terbitkan', exact: true }).click()
  await expect(page.getByText('Paket diterbitkan dan soal dibekukan.', { exact: true })).toBeVisible({ timeout: 15_000 })
  const publishedRow = page.getByRole('row').filter({ hasText: packageTitle })
  await expect(publishedRow.getByText('terbit', { exact: true })).toBeVisible({ timeout: 15_000 })
})

test('panel kanvas mobile menjebak fokus, menutup dengan Escape, dan mengembalikan fokus', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 375, height: 812 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await page.getByRole('button', { name: /Buat paket/i }).click()

  const settingsButton = page.getByRole('button', { name: 'Pengaturan', exact: true })
  await settingsButton.click()
  const panel = page.getByRole('dialog', { name: 'Pengaturan paket' })
  await expect(panel).toBeVisible()
  await expect(panel.getByRole('button', { name: 'Tutup' })).toBeVisible()

  await page.keyboard.press('Escape')
  await expect(panel).toHaveCount(0)
  await expect(settingsButton).toBeFocused()
})

test('guru mengatur batas panjang jawaban melalui kontrol visual', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 1440, height: 900 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await page.getByRole('tab', { name: 'Soal' }).click()
  await page.getByRole('button', { name: 'Buat soal' }).click()
  await page.getByLabel('Jenis soal', { exact: true }).selectOption('isian_singkat')

  await expect(page.getByLabel('Minimal karakter', { exact: true })).toBeVisible()
  await page.getByLabel('Minimal karakter', { exact: true }).fill('5')
  await page.getByLabel('Maksimal karakter', { exact: true }).fill('80')
  await page.getByLabel('Pesan bila belum sesuai', { exact: true }).fill('Jelaskan jawaban dengan lebih lengkap.')
  await expect(page.getByText('Autosave tetap menerima tulisan sementara yang belum lengkap.')).toBeVisible()
  await expect(page.locator('textarea[placeholder*="konfigurasi" i], textarea[aria-label*="json" i]')).toHaveCount(0)
})

test('analitik menampilkan satu tren siswa dan riwayat nilai yang dapat dibuka', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan analitik staf.')
  await page.setViewportSize({ width: 1440, height: 900 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  await page.getByRole('tab', { name: 'Hasil' }).click()

  await expect(page.getByRole('heading', { name: 'Perkembangan per siswa' })).toHaveCount(1)
  await expect(page.getByRole('columnheader', { name: 'Tren 8 nilai terakhir' })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'Riwayat terbaru' })).toBeVisible()
})

test('bank soal Ujian Online memilih tipe secara drag and drop dan mengedit stimulus tabel visual', async ({ page }) => {
  test.skip(!username || !password, 'Set E2E_SIMULASI_TEACHER dan E2E_SIMULASI_TEACHER_PASSWORD untuk menjalankan alur guru.')
  await page.setViewportSize({ width: 1440, height: 900 })
  await loginAsTeacher(page)
  await openSimulasiWorkspace(page)
  const subjectsResponse = page.waitForResponse((response) => new URL(response.url()).pathname.endsWith('/api/mapel') && response.request().method() === 'GET')
  await page.getByRole('button', { name: 'Bank soal Ujian Online' }).click()
  const subjectOptions = await subjectsResponse
  expect(subjectOptions.status()).toBe(200)
  expect(Array.isArray(await subjectOptions.json())).toBe(true)
  await expect(page.getByRole('heading', { name: 'Bank Soal' })).toBeVisible()
  await page.getByRole('button', { name: /Tambah soal/i }).click()
  await page.getByText('Pilih atau seret jenis soal').click()

  const typeCard = page.getByRole('button', { name: 'Seret jenis soal Susun urutan' })
  await typeCard.dragTo(page.getByTestId('bank-question-type-dropzone'))

  await expect(page.getByLabel('Jenis soal', { exact: true })).toHaveValue('susun_urutan')
  await expect(page.locator('textarea[placeholder*="konfigurasi" i], textarea[aria-label*="json" i]')).toHaveCount(0)

  await page.getByLabel('Jenis stimulus baru').selectOption('table')
  await page.getByRole('button', { name: 'Tambah', exact: true }).last().click()

  await page.getByLabel('Stimulus 1, baris 1, kolom 1').fill('Hari')
  await page.getByLabel('Stimulus 1, baris 1, kolom 2').fill('Jumlah')
  await page.getByLabel('Stimulus 1, baris 2, kolom 1').fill('Senin')
  await page.getByLabel('Stimulus 1, baris 2, kolom 2').fill('12')
  await expect(page.locator('form aside').getByRole('table').getByText('Senin')).toBeVisible()
  await expect(page.locator('form aside').getByText('Stimulus pendukung', { exact: true })).toBeVisible()
})
