import { forwardRef, type SelectHTMLAttributes } from 'react'
import { ChevronDown } from 'lucide-react'
import { cn } from '../../lib/utils'

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(({ className, children, ...props }, ref) => (
  <div className="relative">
    <select
      ref={ref}
      className={cn(
        'flex h-11 w-full appearance-none rounded-lg border border-gray-200 bg-white px-4 py-2.5 pr-9 text-sm text-gray-800 transition-all duration-150 focus:border-brand-300 focus:ring-3 focus:ring-brand-500/10 outline-none disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer dark:border-gray-800 dark:bg-gray-900 dark:text-white/90',
        className
      )}
      {...props}
    >
      {children}
    </select>
    <ChevronDown className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground/70" />
  </div>
))
Select.displayName = 'Select'

