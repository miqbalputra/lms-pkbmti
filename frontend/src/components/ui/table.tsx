import { forwardRef, type HTMLAttributes, type TableHTMLAttributes, type TdHTMLAttributes, type ThHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export const Table = forwardRef<HTMLTableElement, TableHTMLAttributes<HTMLTableElement>>(({ className, ...props }, ref) => <div className='relative w-full overflow-x-auto overscroll-x-contain'><table ref={ref} className={cn('min-w-full caption-bottom text-sm', className)} {...props}/></div>)
Table.displayName = 'Table'
export const TableHeader = forwardRef<HTMLTableSectionElement, HTMLAttributes<HTMLTableSectionElement>>(({className,...props},ref)=><thead ref={ref} className={cn('bg-gray-50 [&_tr]:border-b border-gray-200 dark:bg-gray-900 dark:border-gray-800',className)} {...props}/>)
export const TableBody = forwardRef<HTMLTableSectionElement, HTMLAttributes<HTMLTableSectionElement>>(({className,...props},ref)=><tbody ref={ref} className={cn('[&_tr:last-child]:border-0 divide-y divide-gray-100 dark:divide-gray-800',className)} {...props}/>)
export const TableRow = forwardRef<HTMLTableRowElement, HTMLAttributes<HTMLTableRowElement>>(({className,...props},ref)=><tr ref={ref} className={cn('border-b border-gray-100 transition-colors hover:bg-gray-50 data-[state=selected]:bg-gray-100 dark:border-gray-800 dark:hover:bg-white/[0.03] dark:data-[state=selected]:bg-white/5',className)} {...props}/>)
export const TableHead = forwardRef<HTMLTableCellElement, ThHTMLAttributes<HTMLTableCellElement>>(({className,...props},ref)=><th ref={ref} className={cn('h-10 px-3 py-2 text-left align-middle text-theme-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400 sm:h-11 sm:px-4',className)} {...props}/>)
export const TableCell = forwardRef<HTMLTableCellElement, TdHTMLAttributes<HTMLTableCellElement>>(({className,...props},ref)=><td ref={ref} className={cn('px-3 py-3 align-middle text-gray-700 dark:text-gray-300 sm:p-4',className)} {...props}/>)
