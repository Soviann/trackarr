import { useMemo } from 'preact/hooks'
import type { WatchProvider } from '../types'
import { getMatchingProviders } from '../utils/providers'
import s from './WatchProviderBadges.module.css'

interface Props {
  providers?: WatchProvider[] | null
  enabledSet?: Set<string>
  className?: string
}

export function WatchProviderBadges({ providers, enabledSet, className }: Props) {
  const matching = useMemo(
    () => getMatchingProviders(providers, enabledSet),
    [providers, enabledSet]
  )

  if (matching.length === 0) return null

  return (
    <div className={`${s.container}${className ? ` ${className}` : ''}`}>
      {matching.map((p) => (
        <span
          key={p.id}
          className={s.badge}
          style={{
            backgroundColor: p.bg,
            color: p.color,
            border: p.border ?? 'none',
          }}
        >
          {p.shortName}
        </span>
      ))}
    </div>
  )
}
