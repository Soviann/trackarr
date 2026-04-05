import clsx from 'clsx'
import s from './FilterBar.module.css'

export type FilterTab = 'all' | 'watching' | 'up_to_date' | 'completed' | 'dropped' | 'plan'

const tabs: { id: FilterTab; label: string }[] = [
  { id: 'watching', label: 'Watching' },
  { id: 'up_to_date', label: 'Up to date' },
  { id: 'completed', label: 'Completed' },
  { id: 'dropped', label: 'Dropped' },
  { id: 'plan', label: 'Plan' },
  { id: 'all', label: 'All' },
]

interface FilterBarProps {
  active: FilterTab
  onChange: (tab: FilterTab) => void
}

export function FilterBar({ active, onChange }: FilterBarProps) {
  return (
    <div className={s.bar}>
      {tabs.map((tab) => {
        const isActive = active === tab.id
        return (
          <button
            key={tab.id}
            onClick={() => onChange(tab.id)}
            className={clsx(s.tab, isActive && s.tabActive)}
          >
            <span className={clsx(s.label, isActive && s.labelActive)}>
              {tab.label}
            </span>
          </button>
        )
      })}
    </div>
  )
}
