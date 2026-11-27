import { expect, type APIRequestContext } from '@playwright/test'

export async function saveFixtureGradeCP(request: APIRequestContext, adminToken: string, classId: string, mapelId: string, labels: string[]) {
  const headers = { Authorization: `Bearer ${adminToken}` }
  const state = await (await request.get(`/api/rapor-sumatif/period?kelasId=${classId}&semester=Ganjil`, { headers })).json()
  const login = await request.post('/api/auth/login', { data: { login: 'wali-rapor', password: 'Guru12345' } })
  expect(login.status()).toBe(200)
  const { accessToken, user } = await login.json()
  const { jenjang, tahunAjaranId } = state.class
  const year = `?tahunAjaranId=${tahunAjaranId}`
  const assignment = await request.put(`/api/rapor-sumatif/cp-assignments/${jenjang}${year}`, { headers, data: { userId: user.id } })
  expect(assignment.status(), await assignment.text()).toBe(200)
  const saved = await request.put(`/api/rapor-sumatif/cp-templates/${jenjang}${year}&semester=Ganjil&mapelId=${mapelId}`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    data: { revision: 0, cps: labels.map(label => ({ label })) },
  })
  expect(saved.status(), await saved.text()).toBe(200)
}
