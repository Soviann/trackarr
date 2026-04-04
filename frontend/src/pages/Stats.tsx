import { colors, accentWash } from '../theme'
import { useApi } from '../hooks/useApi'
import type { StatsResponse, FunStat } from '../types'

const lavender = colors.accentLavender

const statusColors: Record<string, string> = {
  watching: colors.accentAmber,
  completed: colors.accentGreen,
  dropped: colors.accentCoral,
  plan_to_watch: colors.textDimmed,
}

const typeColors: Record<string, string> = {
  movie: colors.accentBlue,
  series: colors.accentTeal,
  anime: colors.accentLavender,
}

const statusLabels: Record<string, string> = {
  watching: 'En cours',
  completed: 'Terminés',
  dropped: 'Abandonnés',
  plan_to_watch: 'À voir',
}

const typeLabels: Record<string, string> = {
  movie: 'Films',
  series: 'Séries',
  anime: 'Anime',
}

const funStatIcons: Record<string, string> = {
  flame: '🔥',
  heart: '❤️',
  zap: '⚡',
  moon: '🌙',
  sun: '☀️',
  tv: '📺',
  'bar-chart': '📊',
  calendar: '📅',
  skull: '💀',
  clock: '⏳',
  trophy: '🏆',
}

export function Stats({ path }: { path?: string }) {
  const { data, loading } = useApi<StatsResponse>('/stats')

  if (loading || !data) {
    return (
      <div style={{ padding: '20px', color: colors.textSecondary }}>
        Chargement...
      </div>
    )
  }

  return (
    <div style={{ padding: '16px 0 36px' }}>
      <h1 style={{ fontSize: '20px', fontWeight: 700, color: colors.textPrimary, padding: '0 16px 16px' }}>
        Stats
      </h1>

      <OverviewSection overview={data.overview} />
      <RatingsSection ratings={data.ratings} />
      <BreakdownSection breakdown={data.breakdown} />
      {data.fun_stats.length > 0 && <FunStatsSection stats={data.fun_stats} />}
      <YearSection year={data.year_summary} />
    </div>
  )
}

function OverviewSection({ overview }: { overview: StatsResponse['overview'] }) {
  const cards = [
    { value: overview.total_titles.toLocaleString('fr-FR'), label: 'TITRES SUIVIS' },
    { value: overview.episodes_watched.toLocaleString('fr-FR'), label: 'ÉPISODES VUS' },
    { value: `${Math.round(overview.completion_rate * 100)}%`, label: 'COMPLÉTÉS' },
    { value: overview.average_rating > 0 ? overview.average_rating.toFixed(1) : '—', label: 'NOTE MOYENNE' },
  ]

  return (
    <section style={{ padding: '0 16px 24px' }}>
      <div style={{
        display: 'grid',
        gridTemplateColumns: '1fr 1fr',
        gap: '10px',
      }}>
        {cards.map((card) => (
          <div key={card.label} style={{
            background: colors.bgCard,
            border: `1px solid ${colors.borderCard}`,
            borderRadius: '12px',
            padding: '16px',
            textAlign: 'center',
          }}>
            <div style={{ fontSize: '32px', fontWeight: 700, color: colors.textPrimary }}>
              {card.value}
            </div>
            <div style={{
              fontSize: '10px',
              color: lavender,
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.5px',
              marginTop: '4px',
            }}>
              {card.label}
            </div>
          </div>
        ))}
      </div>
      <div style={{
        textAlign: 'center',
        color: colors.textMuted,
        fontSize: '12px',
        marginTop: '10px',
      }}>
        {overview.total_movies} films · {overview.total_series} séries · {overview.total_anime} anime
      </div>
    </section>
  )
}

function RatingsSection({ ratings }: { ratings: StatsResponse['ratings'] }) {
  const max = Math.max(...ratings.distribution, 1)

  return (
    <section style={{ padding: '0 16px 24px' }}>
      <SectionLabel>Notes</SectionLabel>
      <div style={{
        background: colors.bgCard,
        border: `1px solid ${colors.borderCard}`,
        borderRadius: '12px',
        padding: '14px 16px',
      }}>
        {[...ratings.distribution].reverse().map((count, i) => {
          const rating = 10 - i
          return (
          <div key={rating} style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            marginBottom: i < 9 ? '6px' : 0,
          }}>
            <span style={{ fontSize: '12px', color: colors.textSecondary, width: '18px', textAlign: 'right', flexShrink: 0 }}>
              {rating}
            </span>
            <div style={{ flex: 1, height: '16px', borderRadius: '4px', background: colors.bgSurface, overflow: 'hidden' }}>
              <div style={{
                height: '100%',
                width: count > 0 ? `${Math.max((count / max) * 100, 4)}%` : '0%',
                background: lavender,
                borderRadius: '4px',
                transition: 'width 0.3s ease',
              }} />
            </div>
            <span style={{ fontSize: '11px', color: colors.textMuted, width: '22px', textAlign: 'right', flexShrink: 0 }}>
              {count > 0 ? count : ''}
            </span>
          </div>
          )
        })}
      </div>
      {ratings.insight && (
        <div style={{
          color: colors.textSecondary,
          fontSize: '12px',
          marginTop: '10px',
          textAlign: 'center',
          fontStyle: 'italic',
        }}>
          {ratings.insight}
        </div>
      )}
    </section>
  )
}

function BreakdownSection({ breakdown }: { breakdown: StatsResponse['breakdown'] }) {
  return (
    <section style={{ padding: '0 16px 24px' }}>
      <SectionLabel>Bibliothèque</SectionLabel>
      <div style={{
        display: 'grid',
        gridTemplateColumns: '1fr 1fr',
        gap: '10px',
      }}>
        <DonutChart
          data={breakdown.by_status}
          colorMap={statusColors}
          labelMap={statusLabels}
          title="Par statut"
        />
        <DonutChart
          data={breakdown.by_type}
          colorMap={typeColors}
          labelMap={typeLabels}
          title="Par type"
        />
      </div>
    </section>
  )
}

function DonutChart({
  data,
  colorMap,
  labelMap,
  title,
}: {
  data: Record<string, number>
  colorMap: Record<string, string>
  labelMap: Record<string, string>
  title: string
}) {
  const entries = Object.entries(data).filter(([, v]) => v > 0)
  const total = entries.reduce((sum, [, v]) => sum + v, 0)

  if (total === 0) return null

  // Build conic-gradient stops
  let angle = 0
  const stops: string[] = []
  for (const [key, count] of entries) {
    const color = colorMap[key] || colors.textMuted
    const deg = (count / total) * 360
    stops.push(`${color} ${angle}deg ${angle + deg}deg`)
    angle += deg
  }

  const gradient = `conic-gradient(${stops.join(', ')})`

  return (
    <div style={{
      background: colors.bgCard,
      border: `1px solid ${colors.borderCard}`,
      borderRadius: '12px',
      padding: '14px',
      textAlign: 'center',
    }}>
      <div style={{ fontSize: '11px', color: colors.textSecondary, marginBottom: '10px', fontWeight: 600 }}>
        {title}
      </div>
      <div style={{
        width: '100px',
        height: '100px',
        borderRadius: '50%',
        background: gradient,
        margin: '0 auto',
        position: 'relative',
      }}>
        <div style={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: '56px',
          height: '56px',
          borderRadius: '50%',
          background: colors.bgCard,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '18px',
          fontWeight: 700,
          color: colors.textPrimary,
        }}>
          {total}
        </div>
      </div>
      <div style={{ marginTop: '10px', display: 'flex', flexDirection: 'column', gap: '4px' }}>
        {entries.map(([key, count]) => (
          <div key={key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '6px', fontSize: '11px' }}>
            <span style={{
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: colorMap[key] || colors.textMuted,
              flexShrink: 0,
            }} />
            <span style={{ color: colors.textSecondary }}>{labelMap[key] || key}</span>
            <span style={{ color: colors.textMuted }}>{count}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function FunStatsSection({ stats }: { stats: FunStat[] }) {
  return (
    <section style={{ padding: '0 16px 24px' }}>
      <SectionLabel>Le savais-tu ?</SectionLabel>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {stats.map((stat) => (
          <div key={stat.id} style={{
            background: colors.bgCard,
            border: `1px solid ${colors.borderCard}`,
            borderRadius: '12px',
            padding: '14px 16px',
            display: 'flex',
            gap: '12px',
            alignItems: 'flex-start',
          }}>
            <span style={{ fontSize: '22px', flexShrink: 0, lineHeight: 1 }}>
              {funStatIcons[stat.icon] || '📌'}
            </span>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: '12px', color: colors.textSecondary, marginBottom: '2px' }}>
                {stat.title}
              </div>
              <div style={{ fontSize: '18px', fontWeight: 700, color: colors.textPrimary }}>
                {stat.value}
              </div>
              <div style={{ fontSize: '11px', color: colors.textMuted, marginTop: '2px' }}>
                {stat.detail}
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function YearSection({ year }: { year: StatsResponse['year_summary'] }) {
  const currentYear = new Date().getFullYear()
  const cards = [
    { value: year.titles_added, label: 'Ajoutés' },
    { value: year.episodes_watched, label: 'Épisodes' },
    { value: year.completions, label: 'Terminés' },
  ]

  return (
    <section style={{ padding: '0 16px 0' }}>
      <SectionLabel>{currentYear} en chiffres</SectionLabel>
      <div style={{
        display: 'grid',
        gridTemplateColumns: '1fr 1fr 1fr',
        gap: '8px',
      }}>
        {cards.map((card) => (
          <div key={card.label} style={{
            background: colors.bgCard,
            border: `1px solid ${colors.borderCard}`,
            borderRadius: '12px',
            padding: '12px',
            textAlign: 'center',
          }}>
            <div style={{ fontSize: '22px', fontWeight: 700, color: colors.textPrimary }}>
              {card.value}
            </div>
            <div style={{
              fontSize: '10px',
              color: colors.textSecondary,
              marginTop: '2px',
            }}>
              {card.label}
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function SectionLabel({ children }: { children: string }) {
  return (
    <div style={{
      fontSize: '10px',
      color: lavender,
      fontWeight: 600,
      textTransform: 'uppercase',
      letterSpacing: '0.5px',
      marginBottom: '10px',
    }}>
      {children}
    </div>
  )
}
