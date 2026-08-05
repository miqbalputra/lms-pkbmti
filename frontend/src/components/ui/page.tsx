import type { ReactNode } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './card'

export function PageToolbar({
  title,
  description,
  actions,
}: {
  title?: string
  description: string
  actions?: ReactNode
}) {
  return (
    <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        {title && <h2 className="text-2xl font-extrabold text-foreground tracking-tight">{title}</h2>}
        <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2.5">{actions}</div>}
    </div>
  )
}

export function FormCard({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <Card className="mb-6 rounded-2xl border border-border bg-card shadow-2xs">
      <CardHeader className="border-b border-border/60">
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent className="pt-6">{children}</CardContent>
    </Card>
  )
}

export function EmptyState({ colSpan = 1, label = 'Belum ada data.' }: { colSpan?: number; label?: string }) {
  return (
    <tr>
      <td colSpan={colSpan} className="h-32 text-center text-sm text-muted-foreground font-medium">
        {label}
      </td>
    </tr>
  )
}

