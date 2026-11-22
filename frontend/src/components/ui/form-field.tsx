import { Children, cloneElement, isValidElement, useId, type ReactElement, type ReactNode } from 'react'
import { Input } from './input'
import { Select } from './select'
import { Label } from './label'

type ControlProps = { id?: string; 'aria-label'?: string; 'aria-labelledby'?: string }

// Connect the visual caption without changing form values or event handlers.
export function FormField({ label, children, labelClassName }: { label: string; children: ReactNode; labelClassName?: string }) {
  const generated = useId()
  const labelId = generated + '-label'
  let controlId: string | undefined
  const content = Children.map(children, child => {
    if (controlId || !isValidElement(child)) return child
    const native = typeof child.type === 'string' && ['input', 'select', 'textarea'].includes(child.type)
    if (!native && child.type !== Input && child.type !== Select) return child
    const element = child as ReactElement<ControlProps>
    controlId = element.props.id || generated
    return cloneElement(element, { id: controlId, 'aria-labelledby': element.props['aria-labelledby'] || (element.props['aria-label'] ? undefined : labelId) })
  })
  return <div className="grid gap-2"><Label id={labelId} htmlFor={controlId} className={labelClassName}>{label}</Label>{content}</div>
}
