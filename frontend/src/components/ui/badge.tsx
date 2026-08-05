import { cva, type VariantProps } from 'class-variance-authority'
import type { HTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-brand-50 text-brand-700 dark:bg-brand-500/[0.12] dark:text-brand-400',
        brand: 'border-transparent bg-brand-500 text-white',
        secondary: 'border-transparent bg-gray-100 text-gray-700 dark:bg-white/5 dark:text-gray-300',
        outline: 'border-gray-200 text-gray-700 dark:border-gray-700 dark:text-gray-300',
        destructive: 'border-transparent bg-error-50 text-error-700 dark:bg-error-500/15 dark:text-error-400',
        success: 'border-transparent bg-success-50 text-success-700 dark:bg-success-500/15 dark:text-success-400',
        warning: 'border-transparent bg-warning-50 text-warning-700 dark:bg-warning-500/15 dark:text-warning-400',
      },
    },
    defaultVariants: { variant: 'default' },
  }
)

export function Badge({
  className,
  variant,
  ...props
}: HTMLAttributes<HTMLDivElement> & VariantProps<typeof badgeVariants>) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}