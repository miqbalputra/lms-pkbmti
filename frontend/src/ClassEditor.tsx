import { useEffect, useState, type FormEvent } from 'react'
import { Alert, AlertDescription } from './components/ui/alert'
import { Button } from './components/ui/button'
import { FormCard } from './components/ui/page'
import { Label } from './components/ui/label'
import { Select } from './components/ui/select'
import { Input } from './components/ui/input'

type Row = Record<string, unknown> & { id: string }
const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

async function request(path: string, token: string, method = 'GET', body?: unknown) {
  const response = await fetch(apiBase + path, {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: body ? JSON.stringify(body) : undefined,
  })
  const result = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(result.error || 'Permintaan gagal')
  return result
}

function classTitle(row: Row) {
  return `Kelas ${String(row.jenjang || '')}${String(row.namaRombel || '')}`
}

export function ClassEditor({ classRow, token, close, saved }: { classRow: Row; token: string; close: () => void; saved: () => void }) {
  const [tutors, setTutors] = useState<Row[]>([])
  const [pokjars, setPokjars] = useState<Row[]>([])
  const [years, setYears] = useState<Row[]>([])
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    setLoading(true)
    void Promise.all([request('/tutor', token), request('/pokjar', token), request('/tahun-ajaran', token)])
      .then(([tutorRows, pokjarRows, yearRows]) => {
        setTutors(tutorRows as Row[])
        setPokjars(pokjarRows as Row[])
        setYears(yearRows as Row[])
      })
      .catch((error) => setMessage(error instanceof Error ? error.message : String(error)))
      .finally(() => setLoading(false))
  }, [token])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setMessage('')
    try {
      const form = Object.fromEntries(new FormData(event.currentTarget))
      await request(`/kelas/${classRow.id}`, token, 'PUT', {
        jenjang: Number(form.jenjang),
        namaRombel: String(form.namaRombel || ''),
        pokjarId: String(form.pokjarId || ''),
        tahunAjaranId: String(form.tahunAjaranId || ''),
        waliKelasId: String(form.waliKelasId || '') || null,
      })
      saved()
      close()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <FormCard title={`Edit data ${classTitle(classRow)}`} description="Isi nama rombel dengan A, B, C, atau kode rombel tanpa menulis ulang kata Kelas.">
      <form className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5" onSubmit={(event) => void submit(event)}>
        <div className="grid gap-2"><Label>Jenjang / kelas</Label><Select name="jenjang" defaultValue={String(classRow.jenjang || '')} disabled={loading}>{[1, 2, 3, 4, 5, 6].map((value) => <option key={value}>{value}</option>)}</Select></div>
        <div className="grid gap-2"><Label>Nama rombel</Label><Input name="namaRombel" defaultValue={String(classRow.namaRombel || '')} placeholder="A" required disabled={loading} /><span className="text-xs text-muted-foreground">Contoh: A → tampilan Kelas 1A</span></div>
        <div className="grid gap-2"><Label>Pokjar</Label><Select name="pokjarId" defaultValue={String(classRow.pokjarId || '')} required disabled={loading}><option value="">Pilih pokjar</option>{pokjars.map((row) => <option key={row.id} value={row.id}>{String(row.namaPokjar || '')}</option>)}</Select></div>
        <div className="grid gap-2"><Label>Tahun ajaran</Label><Select name="tahunAjaranId" defaultValue={String(classRow.tahunAjaranId || '')} required disabled={loading}><option value="">Pilih tahun ajaran</option>{years.map((row) => <option key={row.id} value={row.id}>{String(row.namaTahunAjaran || '')}</option>)}</Select></div>
        <div className="grid gap-2"><Label>Wali kelas</Label><Select name="waliKelasId" defaultValue={String(classRow.waliKelasId || '')} disabled={loading}><option value="">Belum ditetapkan</option>{tutors.map((row) => <option key={row.id} value={row.id}>{String(row.nama || '')}</option>)}</Select></div>
        <div className="flex gap-2 sm:col-span-2 lg:col-span-5"><Button disabled={loading || submitting}>{submitting ? 'Menyimpan...' : 'Simpan perubahan'}</Button><Button type="button" variant="outline" disabled={submitting} onClick={close}>Batal</Button></div>
        {message && <Alert className="sm:col-span-2 lg:col-span-5"><AlertDescription>{message}</AlertDescription></Alert>}
      </form>
    </FormCard>
  )
}
