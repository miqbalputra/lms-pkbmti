import { useEffect, useState, type FormEvent } from 'react'
import { Alert, AlertDescription } from './components/ui/alert'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { PageToolbar } from './components/ui/page'
import { Select } from './components/ui/select'

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
export function ScheduleSettings({ token }: { token: string }) {
  const [day, setDay] = useState('Sabtu'), [time, setTime] = useState('00:05'), [message, setMessage] = useState('')
  useEffect(() => { void request('/settings/jadwal', token).then(data => { setDay(data.hariDefault); setTime(data.jamGenerate) }).catch(error => setMessage(String(error))) }, [token])
  async function save(event: FormEvent) { event.preventDefault(); try { await request('/settings/jadwal', token, 'PUT', { hariDefault: day, jamGenerate: time, zonaWaktu: 'Asia/Jakarta' }); setMessage('Pengaturan jadwal berhasil disimpan.') } catch (error) { setMessage(String(error)) } }
  return <div className='space-y-4'><PageToolbar title="Pengaturan Jadwal KBM" description="Scheduler berjalan dalam zona WIB dan membuat pertemuan untuk rombel aktif yang memiliki wali kelas."/><Card className='max-w-3xl'><CardHeader><CardTitle>Konfigurasi Scheduler</CardTitle><CardDescription>Atur hari dan jam pembuatan pertemuan otomatis.</CardDescription></CardHeader><CardContent className='space-y-5'><form className='grid gap-4 sm:grid-cols-3' onSubmit={save}><div className='grid gap-2'><Label>Hari default KBM</Label><Select value={day} onChange={event => setDay(event.target.value)}>{['Senin','Selasa','Rabu','Kamis','Jumat','Sabtu','Minggu'].map(value => <option key={value}>{value}</option>)}</Select></div><div className='grid gap-2'><Label>Jam generate WIB</Label><Input type='time' value={time} onChange={event => setTime(event.target.value)} required/></div><div className='grid gap-2'><Label>Zona waktu</Label><Input value='Asia/Jakarta' disabled/></div><div className='sm:col-span-3'><Button>Simpan pengaturan</Button></div></form>{message && <Alert><AlertDescription>{message}</AlertDescription></Alert>}</CardContent></Card></div>
}
async function request(path: string, token: string, method = 'GET', body?: unknown) { const response = await fetch(apiBase + path, { method, credentials: 'include', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }, body: body ? JSON.stringify(body) : undefined }); const result = await response.json().catch(() => ({})); if (!response.ok) throw new Error(result.error || 'Permintaan gagal'); return result }
