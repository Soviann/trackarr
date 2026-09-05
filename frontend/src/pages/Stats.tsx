import { useState, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { formatHumanWatchtime, formatWatchtime, getCoverUrl } from '../utils'
import { groupIntoRanges, formatRangeLabel } from '../utils/episodeRanges'
import type { StatsResponse, FunStat, ActivityEvent, PersonStat, WrappedArchiveItem } from '../types'
import { routeTo } from '../routes'
import { ErrorBanner } from '../components/ErrorBanner'
import { PersonFilmographyDrawer } from '../components/PersonFilmographyDrawer'
import { useTranslation, type TranslationKey } from '../i18n'
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

export function Stats({ path: _path }: { path?: string }) {
  const { t, locale } = useTranslation()
  const [timeframe, setTimeframe] = useState<'all' | 'year' | '30d'>('all')
  const [selectedYear, setSelectedYear] = useState<number>(new Date().getFullYear())
  const [mediaType, setMediaType] = useState<'all' | 'movie' | 'series' | 'anime'>('all')

  const queryParams = new URLSearchParams()
  queryParams.set('timeframe', timeframe)
  if (timeframe === 'year') {
    queryParams.set('year', String(selectedYear))
  }
  if (mediaType !== 'all') {
    queryParams.set('media_type', mediaType)
  }
  const statsUrl = `/stats?${queryParams.toString()}`

  const { data, loading } = useApi<StatsResponse>(statsUrl)
  const { data: archives } = useApi<WrappedArchiveItem[]>('/stats/wrapped/archives')
  const [selectedPerson, setSelectedPerson] = useState<{ name: string; role: 'actor' | 'director' } | null>(null)

  if (loading || !data) {
    return (
      <div className={s.loading}>
        {t('common.loading')}
      </div>
    )
  }

  const availableYears = data.available_years && data.available_years.length > 0
    ? data.available_years
    : [new Date().getFullYear()]

  return (
    <div className={s.page}>
      <h1 className={s.pageTitle}>{t('stats.title')}</h1>

      <div className={s.wrappedBanner} onClick={() => route(routeTo.wrapped())}>
        <div className={s.wrappedBannerContent}>
          <div className={s.wrappedBannerIcon}>✨</div>
          <div>
            <div className={s.wrappedBannerTitle}>
              {t('wrapped.bannerTitle', { year: new Date().getFullYear() })}
            </div>
            <div className={s.wrappedBannerSubtitle}>
              {t('wrapped.bannerSubtitle')}
            </div>
          </div>
        </div>
        <button
          className={s.wrappedBannerBtn}
          onClick={(e) => {
            e.stopPropagation()
            route(routeTo.wrapped())
          }}
        >
          {t('wrapped.bannerButton')} →
        </button>
      </div>

      {archives && archives.length > 0 && (
        <WrappedArchivesSection archives={archives} t={t} />
      )}

      <div className={s.statsFiltersBar}>
        <div className={s.filterPillsRow}>
          <button
            type="button"
            className={`${s.pillOpt} ${timeframe === 'all' ? s.pillOptSelected : ''}`}
            onClick={() => setTimeframe('all')}
          >
            {t('stats.allHistory')}
          </button>
          <select
            id="stats-year-filter"
            name="year"
            aria-label={t('releases.filterByYear')}
            className={`${s.yearSelectPill} ${timeframe === 'year' ? s.yearSelectPillSelected : ''}`}
            value={selectedYear}
            onChange={(e) => {
              const yr = Number((e.currentTarget as HTMLSelectElement).value)
              setSelectedYear(yr)
              setTimeframe('year')
            }}
            onInput={(e) => {
              const yr = Number((e.currentTarget as HTMLSelectElement).value)
              setSelectedYear(yr)
              setTimeframe('year')
            }}
          >
            {availableYears.map((yr) => (
              <option key={yr} value={yr}>
                {yr}
              </option>
            ))}
          </select>
          <button
            type="button"
            className={`${s.pillOpt} ${timeframe === '30d' ? s.pillOptSelected : ''}`}
            onClick={() => setTimeframe('30d')}
          >
            {t('stats.last30Days')}
          </button>
        </div>

        <div className={s.filterPillsRow}>
          <button
            type="button"
            className={`${s.pillOpt} ${mediaType === 'all' ? s.pillOptSelected : ''}`}
            onClick={() => setMediaType('all')}
          >
            {t('stats.allMedia')}
          </button>
          <button
            type="button"
            className={`${s.pillOpt} ${mediaType === 'movie' ? s.pillOptSelected : ''}`}
            onClick={() => setMediaType('movie')}
          >
            {t('stats.movies')}
          </button>
          <button
            type="button"
            className={`${s.pillOpt} ${mediaType === 'series' ? s.pillOptSelected : ''}`}
            onClick={() => setMediaType('series')}
          >
            {t('stats.series')}
          </button>
          <button
            type="button"
            className={`${s.pillOpt} ${mediaType === 'anime' ? s.pillOptSelected : ''}`}
            onClick={() => setMediaType('anime')}
          >
            {t('stats.anime')}
          </button>
        </div>
      </div>

      <OverviewSection
        overview={data.overview}
        watchtimeMinutes={data.total_watch_minutes}
        locale={locale}
        t={t}
      />
      <GenreSection genres={data.genres ?? []} t={t} />
      <TopActorsSection
        actors={data.top_actors ?? []}
        onSelectPerson={(name) => setSelectedPerson({ name, role: 'actor' })}
        t={t}
      />
      <TopDirectorsSection
        directors={data.top_directors ?? []}
        onSelectPerson={(name) => setSelectedPerson({ name, role: 'director' })}
        t={t}
      />
      <RatingsSection ratings={data.ratings} t={t} />
      <StreakSection streaks={data.streaks ?? { current: 0, best: 0 }} t={t} />
      {data.fun_stats.length > 0 && <FunStatsSection stats={data.fun_stats} t={t} />}
      <YearSection year={data.year_summary} t={t} />
      <ActivitySection t={t} locale={locale} />

      <PersonFilmographyDrawer
        open={selectedPerson !== null}
        personName={selectedPerson?.name ?? null}
        role={selectedPerson?.role}
        onClose={() => setSelectedPerson(null)}
      />
    </div>
  )
}

function OverviewSection({
  overview,
  watchtimeMinutes,
  locale,
  t,
}: {
  overview: StatsResponse['overview']
  watchtimeMinutes?: number
  locale: string
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  const totalHours = Math.floor((watchtimeMinutes ?? 0) / 60).toLocaleString(locale === 'fr' ? 'fr-FR' : 'en-US')
  const totalEpisodes = overview.episodes_watched.toLocaleString(locale === 'fr' ? 'fr-FR' : 'en-US')
  const humanWatchtime = formatHumanWatchtime(watchtimeMinutes, locale)

  return (
    <section className={s.section}>
      <div className={s.statGrid}>
        <div className={s.statHeroCard}>
          <div className={s.statLabel}>{t('stats.watchTimeCumulative')}</div>
          <div className={s.statHumanTime}>{humanWatchtime}</div>
          <div className={s.statRawSub}>
            {t('stats.watchTimeSubtitle', {
              hours: totalHours,
              episodes: totalEpisodes,
            })}
          </div>
        </div>

        <StatCard label={t('stats.titlesTracked')} value={overview.total_titles.toLocaleString(locale === 'fr' ? 'fr-FR' : 'en-US')} />
        <StatCard label={t('stats.episodesWatched')} value={totalEpisodes} />
        <StatCard label={t('stats.completed')} value={`${Math.round(overview.completion_rate * 100)}%`} />
        <StatCard label={t('stats.avgRating')} value={overview.average_rating > 0 ? overview.average_rating.toFixed(1) : '—'} />
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

function RatingsSection({
  ratings,
  t,
}: {
  ratings: StatsResponse['ratings']
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  const max = Math.max(...ratings.distribution, 1)

  return (
    <section className={s.section}>
      <SectionLabel>{t('stats.ratings')}</SectionLabel>
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

function FunStatsSection({
  stats,
  t,
}: {
  stats: FunStat[]
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  return (
    <section className={s.section}>
      <SectionLabel>{t('stats.didYouKnow')}</SectionLabel>
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

function GenreSection({
  genres,
  t,
}: {
  genres: StatsResponse['genres']
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  if (genres.length === 0) return null
  const max = Math.max(...genres.map((g) => g.count), 1)
  return (
    <section className={s.section}>
      <SectionLabel>{t('stats.topGenres')}</SectionLabel>
      {genres.map((g) => (
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

function TopActorsSection({
  actors,
  onSelectPerson,
  t,
}: {
  actors: PersonStat[]
  onSelectPerson: (name: string) => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  if (actors.length === 0) return null
  const max = Math.max(...actors.map((a) => a.count), 1)
  return (
    <section className={s.section}>
      <SectionLabel>{t('stats.topActors')}</SectionLabel>
      {actors.map((a) => (
        <button
          key={a.name}
          type="button"
          className={s.personRow}
          onClick={() => onSelectPerson(a.name)}
          title={t('stats.actorCount', { count: a.count, plural: a.count === 1 ? '' : 's' })}
        >
          <span className={s.personLabel}>{a.name}</span>
          <div className={s.barTrack}>
            <div
              className={s.barFill}
              style={{ width: `${(a.count / max) * 100}%` }}
            />
          </div>
          <span className={s.barValue}>{a.count}</span>
        </button>
      ))}
    </section>
  )
}

function TopDirectorsSection({
  directors,
  onSelectPerson,
  t,
}: {
  directors: PersonStat[]
  onSelectPerson: (name: string) => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  if (directors.length === 0) return null
  const max = Math.max(...directors.map((d) => d.count), 1)
  return (
    <section className={s.section}>
      <SectionLabel>{t('stats.topDirectors')}</SectionLabel>
      {directors.map((d) => (
        <button
          key={d.name}
          type="button"
          className={s.personRow}
          onClick={() => onSelectPerson(d.name)}
          title={t('stats.directorCount', { count: d.count, plural: d.count === 1 ? '' : 's' })}
        >
          <span className={s.personLabel}>{d.name}</span>
          <div className={s.barTrack}>
            <div
              className={s.barFill}
              style={{ width: `${(d.count / max) * 100}%` }}
            />
          </div>
          <span className={s.barValue}>{d.count}</span>
        </button>
      ))}
    </section>
  )
}

function StreakSection({
  streaks,
  t,
}: {
  streaks: StatsResponse['streaks']
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  if (streaks.current === 0 && streaks.best === 0) return null
  return (
    <section className={s.section}>
      <div className={s.streakRow}>
        <div className={s.streakCard}>
          <div className={s.streakValue}>🔥 {streaks.current}d</div>
          <div className={s.streakLabel}>{t('stats.currentStreak')}</div>
        </div>
        <div className={s.streakCard}>
          <div className={s.streakValue}>🏆 {streaks.best}d</div>
          <div className={s.streakLabel}>{t('stats.bestStreak')}</div>
        </div>
      </div>
    </section>
  )
}

function ActivitySection({
  t,
  locale,
}: {
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  locale: string
}) {
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
      setEvents((prev) => [...prev, ...data])
      setOffset(off + LIMIT)
      setHasMore(data.length === LIMIT)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load activity')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadMore()
  }, [])

  // Group events by calendar date, then by title+season into episode ranges
  const grouped = groupByDate(events)

  return (
    <section className={s.sectionLast}>
      <SectionLabel>{t('stats.recentActivity')}</SectionLabel>
      {Object.entries(grouped).map(([date, evts]) => {
        const rows = groupActivityEvents(evts, t)
        return (
          <div key={date}>
            <div className={s.activityDateHeader}>{formatDateHeader(date, locale, t)}</div>
            {rows.map((row) => {
              const coverUrl = getCoverUrl(row.coverUrl)
              return (
                <a key={row.key} href={routeTo.title(row.titleId)} className={s.activityRow}>
                  {coverUrl ? (
                    <img className={s.activityThumb} src={coverUrl} alt="" role="presentation" loading="lazy" />
                  ) : (
                    <div className={s.activityThumbPlaceholder} />
                  )}
                  <div className={s.activityInfo}>
                    <span className={s.activityTitle}>{row.titleName}</span>
                    <span className={s.activitySub}>{row.subLabel}</span>
                  </div>
                  <span className={`${s.activityBadge} ${s[`badge_${row.isCompletion ? 'done' : row.titleType}`]}`}>
                    {row.isCompletion ? t('stats.completedBadge') : row.titleType === 'movie' ? t('stats.movie') : t('stats.episode')}
                  </span>
                </a>
              )
            })}
          </div>
        )
      })}
      {error && <ErrorBanner message={error} onRetry={loadMore} onDismiss={() => setError('')} />}
      {hasMore && (
        <button className={s.loadMoreBtn} onClick={loadMore} disabled={loading}>
          {loading ? t('common.loading') : t('stats.loadMore')}
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

function groupActivityEvents(
  evts: ActivityEvent[],
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
): ActivityRow[] {
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
      const subLabel =
        rep.title_type === 'movie'
          ? t('stats.movie')
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
        isCompletion: range.items.some((e) => e.is_completion),
        watchedAt: range.items.reduce(
          (max, e) => (e.watched_at > max ? e.watched_at : max),
          range.items[0].watched_at
        ),
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

function formatDateHeader(
  dateStr: string,
  locale: string,
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
): string {
  const today = new Date().toISOString().split('T')[0]
  const yesterday = new Date(Date.now() - 86_400_000).toISOString().split('T')[0]
  if (dateStr === today) return t('stats.today')
  if (dateStr === yesterday) return t('stats.yesterday')
  return new Date(dateStr).toLocaleDateString(locale === 'fr' ? 'fr-FR' : 'en-US', {
    day: 'numeric',
    month: 'long',
  })
}

function YearSection({
  year,
  t,
}: {
  year: StatsResponse['year_summary']
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  const currentYear = new Date().getFullYear()
  const cards = [
    { value: year.titles_added, label: t('stats.added') },
    { value: year.episodes_watched, label: t('stats.episodes') },
    { value: year.completions, label: t('stats.completed') },
  ]

  return (
    <section className={s.section}>
      <SectionLabel>{t('stats.yearInNumbers', { year: currentYear })}</SectionLabel>
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
  const text = children.startsWith('//') ? children : `// ${children.toUpperCase()}`
  return <h2 className={s.sectionHeader}>{text}</h2>
}

function WrappedArchivesSection({
  archives,
  t,
}: {
  archives: WrappedArchiveItem[]
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  return (
    <section className={s.section}>
      <SectionLabel>{t('wrapped.archivesSectionTitle')}</SectionLabel>
      <p className={s.archivesSubtitle}>{t('wrapped.archivesSectionSubtitle')}</p>
      <div className={s.archivesGrid}>
        {archives.map((a) => {
          const cover = getCoverUrl(a.top_cover_url)
          const watchTimeStr = formatWatchtime(a.total_watch_minutes) || '—'
          return (
            <div
              key={a.year}
              className={s.archiveCard}
              onClick={() => route(routeTo.wrapped(a.year))}
            >
              {cover ? (
                <img src={cover} alt={a.persona_title} className={s.archiveCover} />
              ) : (
                <div className={s.archiveCoverPlaceholder}>🎬</div>
              )}
              <div className={s.archiveCardContent}>
                <div className={s.archiveYearBadge}>{a.year}</div>
                <div className={s.archivePersonaTitle}>{a.persona_title}</div>
                <div className={s.archiveMetaRow}>
                  <span>⏱️ {watchTimeStr}</span>
                  <span>🎬 {a.total_titles} titles</span>
                </div>
                {a.persona_badges && a.persona_badges.length > 0 && (
                  <div className={s.archiveBadges}>
                    {a.persona_badges.slice(0, 2).map((b) => (
                      <span key={b} className={s.archiveBadgeItem}>
                        {b}
                      </span>
                    ))}
                  </div>
                )}
                <button
                  className={s.archiveViewBtn}
                  onClick={(e) => {
                    e.stopPropagation()
                    route(routeTo.wrapped(a.year))
                  }}
                >
                  {t('wrapped.viewArchive')} →
                </button>
              </div>
            </div>
          )
        })}
      </div>
    </section>
  )
}
