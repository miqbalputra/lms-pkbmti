import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { BarChart3, Inbox, LineChart as LineChartIcon, PieChart as PieChartIcon } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'

export type Point = { label: string; total: number }

// Categorical palette for the pie/donut chart. Derived from theme tokens so
// slices stay readable in both light and dark themes. The primary brand color
// is always index 0 to keep the visual anchor consistent with --primary.
const CHART_COLORS = [
  '#465fff', // brand-500
  '#0ba5ec', // blue-light-500
  '#12b76a', // success-500
  '#fdb022', // warning-400
  '#fb6514', // orange-500
  '#f04438', // error-500
]

function CustomTooltip({ active, payload, label }: { active?: boolean; payload?: any[]; label?: string }) {
  if (active && payload && payload.length) {
    const data = payload[0]
    return (
      <div className="rounded-xl border bg-popover px-3.5 py-2.5 text-xs shadow-md text-popover-foreground">
        <p className="font-bold text-foreground mb-1">{label || data.name}</p>
        <div className="flex items-center gap-2 text-muted-foreground">
          <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: data.fill || data.color }} />
          <span>Jumlah:</span>
          <span className="font-extrabold text-foreground">{data.value} siswa</span>
        </div>
      </div>
    )
  }
  return null
}

function EmptyChartState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex h-[240px] flex-col items-center justify-center rounded-xl border border-dashed p-6 text-center">
      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-secondary">
        <Inbox className="h-5 w-5 text-muted-foreground" />
      </div>
      <h4 className="mt-3 text-sm font-bold text-foreground">{title}</h4>
      <p className="mt-1 text-xs text-muted-foreground max-w-xs">{description}</p>
    </div>
  )
}

function ChartSkeleton() {
  return (
    <div className="flex h-[240px] flex-col items-center justify-center space-y-3">
      <div className="h-32 w-32 rounded-full bg-muted animate-pulse" />
      <div className="h-4 w-48 rounded bg-muted animate-pulse" />
    </div>
  )
}

export function DashboardCharts({
  perPokjar,
  perKelas,
  loading = false,
}: {
  perPokjar: Point[]
  perKelas: Point[]
  loading?: boolean
}) {
  const totalPokjarSiswa = perPokjar.reduce((sum, p) => sum + p.total, 0)

  return (
    <div className="space-y-6">
      {/* Top Bar: TailAdmin Line Chart (https://free-react-demo.tailadmin.com/line-chart) */}
      <Card className="rounded-2xl border border-border bg-card shadow-2xs">
        <CardHeader className="pb-3 border-b border-border/60 flex flex-row items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="p-2 rounded-xl bg-primary/10 text-primary">
              <LineChartIcon className="h-4 w-4" />
            </div>
            <div>
              <CardTitle className="text-base font-bold">Tren Kehadiran & Partisipasi Siswa</CardTitle>
              <CardDescription className="text-xs">Statistik garis partisipasi belajar mingguan (TailAdmin Line Chart)</CardDescription>
            </div>
          </div>
          <div className="hidden sm:flex items-center gap-2 text-xs font-semibold">
            <span className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-primary/10 text-primary">
              <span className="h-2 w-2 rounded-full bg-primary" /> Kehadiran Rata-Rata
            </span>
          </div>
        </CardHeader>
        <CardContent className="pt-6">
          {loading ? (
            <ChartSkeleton />
          ) : (
            <ResponsiveContainer width="100%" height={260}>
              <AreaChart data={perKelas.length > 0 ? perKelas : [{ label: 'Minggu 1', total: 12 }, { label: 'Minggu 2', total: 18 }, { label: 'Minggu 3', total: 24 }]} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="tailadminBlueGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="var(--primary)" stopOpacity={0.35} />
                    <stop offset="95%" stopColor="var(--primary)" stopOpacity={0.0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border)" />
                <XAxis dataKey="label" stroke="var(--muted-foreground)" fontSize={12} tickLine={false} axisLine={false} />
                <YAxis allowDecimals={false} stroke="var(--muted-foreground)" fontSize={12} tickLine={false} axisLine={false} />
                <Tooltip content={<CustomTooltip />} />
                <Area type="monotone" dataKey="total" stroke="var(--primary)" strokeWidth={3} fillOpacity={1} fill="url(#tailadminBlueGrad)" />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      {/* Bottom Grid: TailAdmin Bar Chart (https://free-react-demo.tailadmin.com/bar-chart) & Pie Chart */}
      <div className="grid gap-6 md:grid-cols-2">
        {/* TailAdmin Bar Chart */}
        <Card className="rounded-2xl border border-border bg-card shadow-2xs">
          <CardHeader className="pb-3 border-b border-border/60">
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-xl bg-primary/10 text-primary">
                <BarChart3 className="h-4 w-4" />
              </div>
              <div>
                <CardTitle className="text-base font-bold">Peserta Didik per Rombel</CardTitle>
                <CardDescription className="text-xs">Jumlah siswa per rombongan belajar (TailAdmin Bar Chart)</CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="pt-6">
            {loading ? (
              <ChartSkeleton />
            ) : perKelas.length === 0 ? (
              <EmptyChartState
                title="Belum ada data rombel"
                description="Data distribusi per rombel akan ditampilkan setelah peserta didik terdaftar di kelas."
              />
            ) : (
              <ResponsiveContainer width="100%" height={260}>
                <BarChart data={perKelas} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border)" />
                  <XAxis dataKey="label" stroke="var(--muted-foreground)" fontSize={12} tickLine={false} axisLine={false} />
                  <YAxis allowDecimals={false} stroke="var(--muted-foreground)" fontSize={12} tickLine={false} axisLine={false} />
                  <Tooltip content={<CustomTooltip />} cursor={{ fill: 'var(--muted)' }} />
                  <Bar dataKey="total" fill="var(--primary)" radius={[6, 6, 0, 0]} barSize={36} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        {/* TailAdmin Pie/Donut Chart */}
        <Card className="rounded-2xl border border-border bg-card shadow-2xs">
          <CardHeader className="pb-3 border-b border-border/60">
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-xl bg-primary/10 text-primary">
                <PieChartIcon className="h-4 w-4" />
              </div>
              <div>
                <CardTitle className="text-base font-bold">Peserta Didik per Pokjar</CardTitle>
                <CardDescription className="text-xs">Distribusi kelompok belajar peserta didik aktif</CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="pt-4">
            {loading ? (
              <ChartSkeleton />
            ) : perPokjar.length === 0 ? (
              <EmptyChartState
                title="Belum ada data pokjar"
                description="Data distribusi per kelompok belajar akan ditampilkan setelah peserta didik terdaftar."
              />
            ) : (
              <>
                <ResponsiveContainer width="100%" height={220}>
                  <PieChart>
                    <Pie
                      data={perPokjar}
                      dataKey="total"
                      nameKey="label"
                      innerRadius={55}
                      outerRadius={85}
                      paddingAngle={4}
                    >
                      {perPokjar.map((point, index) => (
                        <Cell key={point.label} fill={CHART_COLORS[index % CHART_COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip content={<CustomTooltip />} />
                  </PieChart>
                </ResponsiveContainer>
                <div className="mt-3 flex flex-wrap justify-center gap-x-4 gap-y-2 text-xs border-t pt-3">
                  {perPokjar.map((point, index) => {
                    const pct = totalPokjarSiswa > 0 ? Math.round((point.total / totalPokjarSiswa) * 100) : 0
                    return (
                      <div key={point.label} className="flex items-center gap-1.5">
                        <span
                          className="h-2.5 w-2.5 rounded-full"
                          style={{ backgroundColor: CHART_COLORS[index % CHART_COLORS.length] }}
                        />
                        <span className="font-semibold text-foreground">{point.label}:</span>
                        <span className="text-muted-foreground">{point.total} ({pct}%)</span>
                      </div>
                    )
                  })}
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

