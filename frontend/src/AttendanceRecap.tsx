import { useEffect, useState } from 'react'
import { Download } from 'lucide-react'
import { Alert, AlertDescription } from './components/ui/alert'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Label } from './components/ui/label'
import { Select } from './components/ui/select'
import { EmptyState } from './components/ui/page'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './components/ui/table'

type Row = Record<string, unknown> & { id: string }
const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

export function AttendanceRecap({ token }: { token: string }) {
  const [classes, setClasses] = useState<Row[]>([])
  const [classID, setClassID] = useState('')
  const [semester, setSemester] = useState('Ganjil')
  const [rows, setRows] = useState<Row[]>([])
  const [message, setMessage] = useState('')
  useEffect(() => { void request('/kelas', token).then(setClasses).catch(e => setMessage(String(e))) }, [token])
  function load() { if (classID) void request(`/presensi/rekap?kelasId=${classID}&semester=${semester}`, token).then(result => setRows(result.rows)).catch(e => setMessage(String(e))) }
  async function pdf() { try { const response = await fetch(`${apiBase}/presensi/rekap/pdf?kelasId=${classID}&semester=${semester}`, { credentials: 'include', headers: { Authorization: `Bearer ${token}` } }); if (!response.ok) { setMessage('PDF gagal dibuat.'); return } const url = URL.createObjectURL(await response.blob()); const link = document.createElement('a'); link.href = url; link.download = 'rekap-presensi.pdf'; link.click(); URL.revokeObjectURL(url) } catch { setMessage('PDF gagal dibuat. Periksa koneksi jaringan Anda.') } }
  return <Card><CardHeader><CardTitle>Rekap kehadiran semester</CardTitle><CardDescription>Ringkasan Hadir, Sakit, Izin, dan Alpa per peserta didik.</CardDescription></CardHeader><CardContent className='space-y-4'><div className='flex flex-col gap-3 sm:flex-row sm:items-end'><div className='grid flex-1 gap-2'><Label>Rombel</Label><Select value={classID} onChange={e => setClassID(e.target.value)}><option value=''>Pilih rombel</option>{classes.map(row => <option key={row.id} value={row.id}>Kelas {String(row.jenjang)}{String(row.namaRombel)}</option>)}</Select></div><div className='grid gap-2'><Label>Semester</Label><Select value={semester} onChange={e => setSemester(e.target.value)}><option>Ganjil</option><option>Genap</option></Select></div><Button variant='outline' disabled={!classID} onClick={load}>Tampilkan</Button><Button variant='outline' disabled={!classID} onClick={() => void pdf()}><Download className='h-4 w-4'/>PDF</Button></div>{message && <Alert><AlertDescription>{message}</AlertDescription></Alert>}<Table><TableHeader><TableRow><TableHead>Peserta didik</TableHead><TableHead>Hadir</TableHead><TableHead>Sakit</TableHead><TableHead>Izin</TableHead><TableHead>Alpa</TableHead></TableRow></TableHeader><TableBody>{rows.map(row => <TableRow key={String(row.pesertaDidikId)}><TableCell><div className='font-medium'>{String(row.nama)}</div><div className='text-xs text-muted-foreground'>{String(row.nis)}</div></TableCell><TableCell>{String(row.hadir)}</TableCell><TableCell>{String(row.sakit)}</TableCell><TableCell>{String(row.izin)}</TableCell><TableCell>{String(row.alpa)}</TableCell></TableRow>)}{!rows.length && <EmptyState colSpan={5} label='Pilih rombel untuk melihat rekap.'/ >}</TableBody></Table></CardContent></Card>
}

async function request(path: string, token: string) { const response = await fetch(apiBase + path, { credentials: 'include', headers: { Authorization: `Bearer ${token}` } }); const result = await response.json().catch(() => ({})); if (!response.ok) throw new Error(result.error || 'Permintaan gagal'); return result }
