import { forwardRef, type InputHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'
export const Checkbox=forwardRef<HTMLInputElement,InputHTMLAttributes<HTMLInputElement>>(({className,...props},ref)=><input ref={ref} type='checkbox' className={cn('h-4 w-4 rounded border border-primary accent-[var(--primary)]',className)} {...props}/>)
Checkbox.displayName='Checkbox'
