import { useEffect, useState } from 'react'
import { BarChart3, Monitor, ExternalLink } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { PageToolbar } from '../components/ui/page'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { request } from '../lib/api'
import { AssessmentCollaboratorManager } from '../components/AssessmentCollaborators'

type UjianRow = Record<string, unknown> & { id: string }

export function UjianOnlineView({ token }: { token: string }) {
  const [ujians, setUjians] = useState<UjianRow[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    request('/ujian', token)
      .then((d) => setUjians(Array.isArray(d) ? d.filter((u: any) => u.aksesKode) : []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [token])

  return (
    <div className="space-y-4">
      <PageToolbar
        title="Ujian Online"
        description="Daftar ujian online yang memiliki kode akses. Siswa mengerjakan via halaman publik."
        actions={
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={() => window.location.assign('/simulasi?tab=hasil')}>
              <BarChart3 className="h-4 w-4" /> Analitik & laporan gabungan
            </Button>
            <Button variant="outline" onClick={() => window.open('/ujian', '_blank')}>
              <ExternalLink className="h-4 w-4" /> Buka Halaman Siswa
            </Button>
          </div>
        }
      />

      <Card className="rounded-2xl border border-border bg-card p-6 shadow-2xs">
        {loading ? (
          <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
            <div className="h-4 w-4 rounded-full border-2 border-primary border-t-transparent animate-spin" />
            Memuat data ujian...
          </div>
        ) : ujians.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-center">
            <Monitor className="h-12 w-12 text-muted-foreground/40 mb-3" />
            <h4 className="text-sm font-bold text-foreground">Belum ada ujian online</h4>
            <p className="mt-1 text-xs text-muted-foreground">Buat ujian dengan kode akses di menu Ujian (Luring).</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>No</TableHead>
                  <TableHead>Judul</TableHead>
                  <TableHead>Mata Pelajaran</TableHead>
                  <TableHead>Kode Akses</TableHead>
                  <TableHead>Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {ujians.map((u, i) => {
                  const mapel = u.mapel as Record<string, unknown> | undefined
                  return (
                    <TableRow key={u.id}>
                      <TableCell>{i + 1}</TableCell>
                      <TableCell className="font-medium">{String(u.judul || '-')}</TableCell>
                      <TableCell>{String(mapel?.namaMapel || '-')}</TableCell>
                      <TableCell>
                        <code className="rounded bg-muted px-2 py-0.5 text-xs font-mono">{String(u.aksesKode)}</code>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap justify-end gap-2">
                          <AssessmentCollaboratorManager token={token} module="ujian_online" assessmentId={u.id} />
                          <Button size="sm" className="min-h-11" variant="outline" onClick={() => window.open('/ujian', '_blank')}>
                            🎓 Halaman Siswa
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </Card>
    </div>
  )
}
