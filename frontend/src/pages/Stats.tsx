import { useState, useEffect } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { formatWatchtime } from '../utils'
import { groupIntoRanges, formatRangeLabel } from '../utils/episodeRanges'
import type { StatsResponse, FunStat, ActivityEvent } from '../types'
import { routeTo } from '../routes'
import { ErrorBanner } from '../components/ErrorBanner'
import s from './Stats.module.css'

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
        Loading...
      </div>
    )
  }

  return (
    <div className={s.page}>
      <h1 className={s.pageTitle}>Stats</h1>

      <OverviewSection overview={data.overview} watchtimeMinutes={data.total_watch_minutes} />
      <GenreSection genres={data.genres ?? []} />
      <RatingsSection ratings={data.ratings} />
      <StreakSection streaks={data.streaks ?? { current: 0, best: 0 }} />
      {data.fun_stats.length > 0 && <FunStatsSection stats={data.fun_stats} />}
      <YearSection year={data.year_summary} />
      <ActivitySection />
    </div>
  )
}

function OverviewSection({ overview, watchtimeMinutes }: { overview: StatsResponse['overview']; watchtimeMinutes?: number }) {
  return (
    <section className={s.section}>
      <div className={s.statGrid}>
        <StatCard label="// TITLES TRACKED" value={overview.total_titles.toLocaleString('en-US')} />
        <StatCard label="// EPISODES WATCHED" value={overview.episodes_watched.toLocaleString('en-US')} />
        <StatCard label="// COMPLETED" value={`${Math.round(overview.completion_rate * 100)}%`} />
        <StatCard label="// AVG RATING" value={overview.average_rating > 0 ? overview.average_rating.toFixed(1) : '—'} />
        <StatCard label="// WATCH TIME" value={formatWatchtime(watchtimeMinutes) ?? '—'} wide />
      </div>
    </section>
  )
}

function StatCard({ label, value, wide }: { label: string; value: string; wide?: boolean }) {
  return (
    <div className={`${s.statCard}${wide ? ` ${s.statCardWide}` : ''}`}>
      <div className={s.statLabel}>{label}</div>
      <div className={s.statValue}>{value}</div>
    </div>
  )
}

function RatingsSection({ ratings }: { ratings: StatsResponse['ratings'] }) {
  const max = Math.max(...ratings.distribution, 1)

  return (
    <section className={s.section}>
      <SectionLabel>Ratings</SectionLabel>
      {[...ratings.distribution].reverse().map((count, i) => {
        const rating = 10 - i
        return (
          <div key={rating} className={s.barRow}>
            <span className={s.barLabel}>{rating}</span>
            <div className={s.barTrack}>
              <div
                className={s.barFill}
                style={{ width: count > 0 ? `${Math.max((count / max) * 100, 4)}%` : '0%' }}
              />
            </div>
            <span className={s.barValue}>{count > 0 ? count : ''}</span>
          </div>
        )
      })}
      {ratings.insight && (
        <div className={s.ratingInsight}>{ratings.insight}</div>
      )}
    </section>
  )
}

function FunStatsSection({ stats }: { stats: FunStat[] }) {
  return (
    <section className={s.section}>
      <SectionLabel>Did you know?</SectionLabel>
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

function GenreSection({ genres }: { genres: StatsResponse['genres'] }) {
  if (genres.length === 0) return null
  const max = Math.max(...genres.map(g => g.count), 1)
  return (
    <section className={s.section}>
      <SectionLabel>Top Genres</SectionLabel>
      {genres.map(g => (
        <div key={g.genre} className={s.barRow}>
          <span className={s.barLabel}>{g.genre}</span>
          <div className={s.barTrack}>
            <div
              className={s.barFill}
              style={{ width: `${(g.count / max) * 100}%` }}
            />
          </div>
          <span className={s.barValue}>{g.count}</span>
        </div>
      ))}
    </section>
  )
}

function StreakSection({ streaks }: { streaks: StatsResponse['streaks'] }) {
  if (streaks.current === 0 && streaks.best === 0) return null
  return (
    <section className={s.section}>
      <div className={s.streakRow}>
        <div className={s.streakCard}>
          <div className={s.streakValue}>🔥 {streaks.current}d</div>
          <div className={s.streakLabel}>Current streak</div>
        </div>
        <div className={s.streakCard}>
          <div className={s.streakValue}>🏆 {streaks.best}d</div>
          <div className={s.streakLabel}>Best streak</div>
        </div>
      </div>
    </section>
  )
}

function ActivitySection() {
  const [events, setEvents] = useState<ActivityEvent[]>([])
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  const [error, setError] = useState('')
  const LIMIT = 50

  const loadMore = async () => {
    if (loading || !hasMore) return
    const off = offset
    setLoading(true)
    setError('')
    try {
      const data = await apiFetch<ActivityEvent[]>(
        `/stats/activity?limit=${LIMIT}&offset=${off}`
      )
      setEvents(prev => [...prev, ...data])
      setOffset(off + LIMIT)
      setHasMore(data.length === LIMIT)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load activity')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadMore() }, [])

  // Group events by calendar date, then by title+season into episode ranges
  const grouped = groupByDate(events)

  return (
    <section className={s.sectionLast}>
      <SectionLabel>Recent activity</SectionLabel>
      {Object.entries(grouped).map(([date, evts]) => {
        const rows = groupActivityEvents(evts)
        return (
          <div key={date}>
            <div className={s.activityDateHeader}>{formatDateHeader(date)}</div>
            {rows.map((row) => (
              <a key={row.key} href={routeTo.title(row.titleId)} className={s.activityRow}>
                {row.coverUrl
                  ? <img className={s.activityThumb} src={`/api/covers/${row.coverUrl}`} alt="" role="presentation" />
                  : <div className={s.activityThumbPlaceholder} />}
                <div className={s.activityInfo}>
                  <span className={s.activityTitle}>{row.titleName}</span>
                  <span className={s.activitySub}>{row.subLabel}</span>
                </div>
                <span className={`${s.activityBadge} ${s[`badge_${row.isCompletion ? 'done' : row.titleType}`]}`}>
                  {row.isCompletion ? 'Completed' : row.titleType === 'movie' ? 'Movie' : 'Episode'}
                </span>
              </a>
            ))}
          </div>
        )
      })}
      {error && <ErrorBanner message={error} onRetry={loadMore} onDismiss={() => setError('')} />}
      {hasMore && (
        <button className={s.loadMoreBtn} onClick={loadMore} disabled={loading}>
          {loading ? 'Loading…' : 'Load more'}
        </button>
      )}
    </section>
  )
}

interface ActivityRow {
  key: string
  titleId: number
  titleName: string
  coverUrl: string | null
  titleType: string
  subLabel: string
  isCompletion: boolean
  watchedAt: string
}

function groupActivityEvents(evts: ActivityEvent[]): ActivityRow[] {
  // Sub-group by (title_id, season_number)
  const buckets = new Map<string, ActivityEvent[]>()
  for (const ev of evts) {
    const key = `${ev.title_id}-${ev.season_number ?? 'null'}`
    const list = buckets.get(key) ?? []
    list.push(ev)
    buckets.set(key, list)
  }

  const rows: ActivityRow[] = []
  for (const group of buckets.values()) {
    // Sort by episode_number ASC for range detection
    group.sort((a, b) => (a.episode_number ?? 0) - (b.episode_number ?? 0))
    const ranges = groupIntoRanges(group)
    const rep = group[0]

    for (const range of ranges) {
      const label = formatRangeLabel(range)
      const isSingle = range.items.length === 1
      const subLabel = rep.title_type === 'movie'
        ? 'Movie'
        : isSingle && range.episodeName
          ? `${label} — ${range.episodeName}`
          : label

      rows.push({
        key: `${rep.title_id}-${range.seasonNumber}-${range.startEp}-${range.endEp}`,
        titleId: rep.title_id,
        titleName: rep.title_name,
        coverUrl: rep.cover_url,
        titleType: rep.title_type,
        subLabel,
        isCompletion: range.items.some(e => e.is_completion),
        watchedAt: range.items.reduce((max, e) =>
          e.watched_at > max ? e.watched_at : max, range.items[0].watched_at),
      })
    }
  }

  // Re-sort by watched_at DESC
  rows.sort((a, b) => b.watchedAt.localeCompare(a.watchedAt))
  return rows
}

function groupByDate(evts: ActivityEvent[]): Record<string, ActivityEvent[]> {
  return evts.reduce((acc, ev) => {
    const date = ev.watched_at.split('T')[0].split(' ')[0]
    if (!acc[date]) acc[date] = []
    acc[date].push(ev)
    return acc
  }, {} as Record<string, ActivityEvent[]>)
}

function formatDateHeader(dateStr: string): string {
  const today = new Date().toISOString().split('T')[0]
  const yesterday = new Date(Date.now() - 86_400_000).toISOString().split('T')[0]
  if (dateStr === today) return 'Today'
  if (dateStr === yesterday) return 'Yesterday'
  return new Date(dateStr).toLocaleDateString('en-US', { day: 'numeric', month: 'long' })
}

function YearSection({ year }: { year: StatsResponse['year_summary'] }) {
  const currentYear = new Date().getFullYear()
  const cards = [
    { value: year.titles_added, label: 'Added' },
    { value: year.episodes_watched, label: 'Episodes' },
    { value: year.completions, label: 'Completed' },
  ]

  return (
    <section className={s.section}>
      <SectionLabel>{`${currentYear} in numbers`}</SectionLabel>
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
  return <h2 className={s.sectionHeader}>{`// ${children.toUpperCase()}`}</h2>
}
