import { useState, useRef } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import s from './CollapsibleSection.module.css'

interface Props {
  title: string
  count?: number
  children: ComponentChildren
  onExpand?: () => void
}

export function CollapsibleSection({ title, count, children, onExpand }: Props) {
  const [open, setOpen] = useState(false)
  const didLoad = useRef(false)

  function toggle() {
    const next = !open
    setOpen(next)
    if (next && !didLoad.current) {
      didLoad.current = true
      onExpand?.()
    }
  }

  return (
    <div className={s.section}>
      <button className={s.header} onClick={toggle} aria-expanded={open}>
        <span className={s.title}>{title}</span>
        {count !== undefined && <span className={s.count}>{count}</span>}
        <span className={`${s.arrow} ${open ? s.open : ''}`}>›</span>
      </button>
      {open && <div className={s.body}>{children}</div>}
    </div>
  )
}
