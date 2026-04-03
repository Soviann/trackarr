import { colors, accentWash } from '../theme'

export type FilterTab = 'all' | 'watching' | 'up_to_date' | 'completed' | 'dropped' | 'plan'

const tabs: { id: FilterTab; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'watching', label: 'Watching' },
  { id: 'up_to_date', label: 'Up to date' },
  { id: 'completed', label: 'Completed' },
  { id: 'dropped', label: 'Dropped' },
  { id: 'plan', label: 'Plan' },
]

interface FilterBarProps {
  active: FilterTab
  onChange: (tab: FilterTab) => void
}

export function FilterBar({ active, onChange }: FilterBarProps) {
  return (
    <div style={{
      display: 'flex',
      borderTop: `1px solid ${colors.borderSubtle}`,
      background: colors.bgPrimary,
      overflowX: 'auto',
    }}>
      {tabs.map((tab) => {
        const isActive = active === tab.id
        return (
          <button
            key={tab.id}
            onClick={() => onChange(tab.id)}
            style={{
              flex: 1,
              padding: '8px 12px',
              textAlign: 'center',
              background: isActive ? accentWash(colors.accentBlue) : 'transparent',
              borderTop: `2px solid ${isActive ? colors.accentBlue : 'transparent'}`,
              border: 'none',
              borderTopStyle: 'solid',
              borderTopWidth: '2px',
              borderTopColor: isActive ? colors.accentBlue : 'transparent',
              cursor: 'pointer',
              WebkitTapHighlightColor: 'transparent',
            }}
          >
            <span style={{
              fontSize: '10px',
              fontWeight: isActive ? 600 : 400,
              color: isActive ? colors.accentBlue : colors.textMuted,
            }}>
              {tab.label}
            </span>
          </button>
        )
      })}
    </div>
  )
}
