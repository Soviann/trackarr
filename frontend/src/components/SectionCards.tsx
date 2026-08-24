import s from './SectionCards.module.css'
import { getCoverUrl } from '../utils'

export interface CardItemProps {
  label: string
  subText?: string
  posters?: { id?: number; cover_url: string | null }[]
  variant?: 'posters' | 'accent'
  onClick: () => void
  loading?: boolean
}

function SectionCard({ label, subText, posters, variant = 'posters', onClick, loading }: CardItemProps) {
  const slices = (posters ?? []).slice(0, 3)

  return (
    <button
      type="button"
      className={s.card}
      onClick={onClick}
      aria-busy={loading || undefined}
    >
      {/* Background layer */}
      <div className={s.bgLayer}>
        {variant === 'accent' ? (
          <>
            <div className={s.accentBg} />
            <svg
              className={s.accentWatermark}
              width="54"
              height="54"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.5"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <circle cx="12" cy="12" r="10" />
              <line x1="2" y1="12" x2="22" y2="12" />
              <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
            </svg>
          </>
        ) : (
          <>
            <div className={s.postersGrid}>
              {slices.map((p, idx) => {
                const cover = getCoverUrl(p.cover_url)
                return (
                  <div
                    key={p.id ?? idx}
                    className={s.posterSlice}
                    style={{
                      backgroundImage: cover ? `url(${cover})` : undefined,
                    }}
                  />
                )
              })}
            </div>
            <div className={s.bgOverlay} />
          </>
        )}
      </div>

      {/* Content */}
      <div className={s.content}>
        <div className={s.labelRow}>
          <span className={s.label}>{label}</span>
          <svg className={s.chev} width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="9 18 15 12 9 6" />
          </svg>
        </div>
        {loading ? (
          <div className={s.subSkeleton} aria-hidden="true" />
        ) : (
          <span className={s.sub}>{subText}</span>
        )}
      </div>
    </button>
  )
}

interface SectionCardsProps {
  cards: CardItemProps[]
}

export function SectionCards({ cards }: SectionCardsProps) {
  if (cards.length === 0) return null
  return (
    <div
      className={s.grid}
      style={{ gridTemplateColumns: `repeat(${cards.length}, 1fr)` }}
    >
      {cards.map((c) => (
        <SectionCard key={c.label} {...c} />
      ))}
    </div>
  )
}
