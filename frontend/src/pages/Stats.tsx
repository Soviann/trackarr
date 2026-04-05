import { colors } from '../theme'
import { useApi } from '../hooks/useApi'
import type { StatsResponse, FunStat } from '../types'
import s from './Stats.module.css'

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
  flame: '\u{1F525}',
  heart: '\u2764\uFE0F',
  zap: '\u26A1',
  moon: '\u{1F319}',
  sun: '\u2600\uFE0F',
  tv: '\u{1F4FA}',
  'bar-chart': '\u{1F4CA}',
  calendar: '\u{1F4C5}',
  skull: '\u{1F480}',
  clock: '\u23F3',
  trophy: '\u{1F3C6}',
}

export function Stats({ path }: { path?: string }) {
  const { data, loading } = useApi<StatsResponse>('/stats')

  if (loading || !data) {
    return (
      <div className={s.loading}>
        Chargement...
      </div>
    )
  }

  return (
    <div className={s.page}>
      <h1 className={s.pageTitle}>Stats</h1>

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
    { value: overview.episodes_watched.toLocaleString('fr-FR'), label: '\u00C9PISODES VUS' },
    { value: `${Math.round(overview.completion_rate * 100)}%`, label: 'COMPL\u00C9T\u00C9S' },
    { value: overview.average_rating > 0 ? overview.average_rating.toFixed(1) : '\u2014', label: 'NOTE MOYENNE' },
  ]

  return (
    <section className={s.section}>
      <div className={s.overviewGrid}>
        {cards.map((card) => (
          <div key={card.label} className={s.card}>
            <div className={s.overviewValue}>{card.value}</div>
            <div className={s.overviewLabel}>{card.label}</div>
          </div>
        ))}
      </div>
      <div className={s.overviewFooter}>
        {overview.total_movies} films · {overview.total_series} séries · {overview.total_anime} anime
      </div>
    </section>
  )
}

function RatingsSection({ ratings }: { ratings: StatsResponse['ratings'] }) {
  const max = Math.max(...ratings.distribution, 1)

  return (
    <section className={s.section}>
      <SectionLabel>Notes</SectionLabel>
      <div className={s.ratingsCard}>
        {[...ratings.distribution].reverse().map((count, i) => {
          const rating = 10 - i
          return (
            <div key={rating} className={s.ratingRow}>
              <span className={s.ratingLabel}>{rating}</span>
              <div className={s.ratingTrack}>
                <div
                  className={s.ratingBar}
                  style={{ width: count > 0 ? `${Math.max((count / max) * 100, 4)}%` : '0%' }}
                />
              </div>
              <span className={s.ratingCount}>{count > 0 ? count : ''}</span>
            </div>
          )
        })}
      </div>
      {ratings.insight && (
        <div className={s.ratingInsight}>{ratings.insight}</div>
      )}
    </section>
  )
}

function BreakdownSection({ breakdown }: { breakdown: StatsResponse['breakdown'] }) {
  return (
    <section className={s.section}>
      <SectionLabel>Bibliothèque</SectionLabel>
      <div className={s.breakdownGrid}>
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
    <div className={s.donutCard}>
      <div className={s.donutTitle}>{title}</div>
      <div className={s.donutRing} style={{ background: gradient }}>
        <div className={s.donutCenter}>{total}</div>
      </div>
      <div className={s.donutLegend}>
        {entries.map(([key, count]) => (
          <div key={key} className={s.legendItem}>
            <span className={s.legendDot} style={{ background: colorMap[key] || colors.textMuted }} />
            <span className={s.legendLabel}>{labelMap[key] || key}</span>
            <span className={s.legendCount}>{count}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function FunStatsSection({ stats }: { stats: FunStat[] }) {
  return (
    <section className={s.section}>
      <SectionLabel>Le savais-tu ?</SectionLabel>
      <div className={s.funStatList}>
        {stats.map((stat) => (
          <div key={stat.id} className={s.funStatCard}>
            <span className={s.funStatIcon}>{funStatIcons[stat.icon] || '\u{1F4CC}'}</span>
            <div className={s.funStatBody}>
              <div className={s.funStatTitle}>{stat.title}</div>
              <div className={s.funStatValue}>{stat.value}</div>
              <div className={s.funStatDetail}>{stat.detail}</div>
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
    { value: year.episodes_watched, label: '\u00C9pisodes' },
    { value: year.completions, label: 'Terminés' },
  ]

  return (
    <section className={s.sectionLast}>
      <SectionLabel>{`${currentYear} en chiffres`}</SectionLabel>
      <div className={s.yearGrid}>
        {cards.map((card) => (
          <div key={card.label} className={s.yearCard}>
            <div className={s.yearValue}>{card.value}</div>
            <div className={s.yearLabel}>{card.label}</div>
          </div>
        ))}
      </div>
    </section>
  )
}

function SectionLabel({ children }: { children: string }) {
  return <div className={s.sectionLabel}>{children}</div>
}
