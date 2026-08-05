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

export function EmptyState({ colSpan = 1, label = 'Belum ada data.', title, description, icon }: { colSpan?: number; label?: string; title?: string; description?: string; icon?: React.ReactNode }) {
  if (title || icon) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        {icon && <div className="mb-3">{icon}</div>}
        <h4 className="text-sm font-bold text-foreground">{title || label}</h4>
        {description && <p className="mt-1 text-xs text-muted-foreground">{description}</p>}
      </div>
    )
  }
  return (
    <tr>
      <td colSpan={colSpan} className="h-32 text-center text-sm text-muted-foreground font-medium">
        {label}
      </td>
    </tr>
  )
}

