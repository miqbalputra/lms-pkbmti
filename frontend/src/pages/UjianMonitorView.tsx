import { useEffect, useState } from 'react'
import { Monitor, RefreshCw, Download } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { EmptyState, PageToolbar } from '../components/ui/page'
import { Badge } from '../components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { downloadFile, request } from '../lib/api'
import { toast } from 'sonner'

type Peserta = Record<string, unknown> & { id: string }

export function UjianMonitorView({
  token,
}: {
  token: string
}) {
  const [ujians, setUjians] = useState<Peserta[]>([])
  const [selected, setSelected] = useState<string>('')
  const [pesertas, setPesertas] = useState<Peserta[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    request('/ujian', token)
      .then((d) => setUjians(Array.isArray(d) ? d : []))
      .catch(() => {})
  }, [token])

  const loadMonitor = (ujianId: string) => {
    setSelected(ujianId)
    setLoading(true)
    request(`/ujian-online/monitor/${ujianId}`, token)
      .then((d) => setPesertas(Array.isArray(d) ? d : []))
      .catch(() => setPesertas([]))
      .finally(() => setLoading(false))
  }

  const exportResults = async () => {
    if (!selected) return
    try {
      await downloadFile(`/ujian/${selected}/export`, token, 'hasil-ujian.csv')
      toast.success('Hasil ujian berhasil diunduh.')
    } catch (error) {
      toast.error(String((error as Error).message || 'Gagal mengekspor hasil ujian.'))
    }
  }

  const fmt = (v: unknown) => (v ? String(v).slice(0, 16).replace('T', ' ') : '-')

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Monitor Ujian Online"
        description="Pantau status pengerjaan ujian online siswa secara real-time."
        actions={
          selected ? (
            <div className="flex gap-2">
              <Button variant="outline" onClick={exportResults}>
                <Download className="h-4 w-4" /> Export CSV
              </Button>
              <Button variant="outline" onClick={() => loadMonitor(selected)}>
                <RefreshCw className="h-4 w-4" /> Refresh
              </Button>
            </div>
          ) : undefined
        }
      />

      <Card className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
        <div className="mb-4">
          <label className="text-sm font-semibold text-foreground">Pilih Ujian</label>
          <select
            className="mt-1 block w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            value={selected}
            onChange={(e) => loadMonitor(e.target.value)}
          >
            <option value="">-- Pilih ujian --</option>
            {ujians.map((u) => (
              <option key={u.id} value={u.id}>
                {String(u.judul)} — {String((u.mapel as Record<string, unknown>)?.namaMapel || '')}
              </option>
            ))}
          </select>
        </div>

        {loading && (
          <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
            <div className="h-4 w-4 rounded-full border-2 border-primary border-t-transparent animate-spin" />
            Memuat data monitoring...
          </div>
        )}

        {!loading && selected && pesertas.length === 0 && (
          <EmptyState title="Belum ada peserta" description="Belum ada siswa yang mengerjakan ujian ini." />
        )}

        {!loading && pesertas.length > 0 && (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>No</TableHead>
                  <TableHead>Nama Siswa</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Skor</TableHead>
                  <TableHead>Tab Switch</TableHead>
                  <TableHead>Mulai</TableHead>
                  <TableHead>Selesai</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pesertas.map((p, i) => {
                  const pd = p.pesertaDidik as Record<string, unknown> | undefined
                  return (
                    <TableRow key={p.id}>
                      <TableCell>{i + 1}</TableCell>
                      <TableCell className="font-medium">{String(pd?.nama || '-')}</TableCell>
                      <TableCell>
                        <Badge variant={p.status === 'selesai' ? 'default' : 'secondary'}>
                          {String(p.status)}
                        </Badge>
                      </TableCell>
                      <TableCell>{p.skor != null ? Number(p.skor).toFixed(1) : '-'}</TableCell>
                      <TableCell>{Number(p.tabSwitch || 0)}</TableCell>
                      <TableCell className="text-xs">{fmt(p.mulai)}</TableCell>
                      <TableCell className="text-xs">{fmt(p.selesai)}</TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}

        {!selected && !loading && (
          <EmptyState
            icon={<Monitor className="h-12 w-12 text-muted-foreground/40" />}
            title="Pilih ujian untuk dimonitor"
            description="Pilih ujian dari daftar di atas untuk melihat status pengerjaan siswa."
          />
        )}
      </Card>
    </div>
  )
}
