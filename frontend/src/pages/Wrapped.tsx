import { useState, useEffect, useCallback, useRef } from 'preact/hooks'
import { route } from 'preact-router'
import { routeTo } from '../routes'
import { apiFetch } from '../api'
import { getCoverUrl, formatWatchtime } from '../utils'
import { useTranslation, type TranslationKey } from '../i18n'
import type { WrappedResponse, WrappedTitleItem, TitleType } from '../types'
import s from './Wrapped.module.css'

const TOTAL_SLIDES = 6
const SLIDE_DURATION_MS = 8000 // 8 seconds per slide auto-advance

interface WrappedProps {
  path?: string
  year?: string
}

export function Wrapped({ year: urlYear }: WrappedProps) {
  const { t } = useTranslation()
  const [data, setData] = useState<WrappedResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [currentSlide, setCurrentSlide] = useState(0)
  const [isManuallyPaused, setIsManuallyPaused] = useState(false)
  const [isHolding, setIsHolding] = useState(false)
  const [slideProgress, setSlideProgress] = useState(0)
  const [selectedCategoryFav, setSelectedCategoryFav] = useState<'movies' | 'series' | 'anime'>('movies')
  const [selectedCategoryRel, setSelectedCategoryRel] = useState<'movies' | 'series' | 'anime'>('movies')

  const isPaused = isManuallyPaused || isHolding
  const queryYear = urlYear || new URLSearchParams(window.location.search).get('year') || ''

  // Load Wrapped data
  const loadData = useCallback(() => {
    setLoading(true)
    setFetchError(null)
    const endpoint = queryYear ? `/stats/wrapped?year=${encodeURIComponent(queryYear)}` : '/stats/wrapped'
    apiFetch<WrappedResponse>(endpoint)
      .then((res) => {
        setData(res)
        setCurrentSlide(0)
        setSlideProgress(0)
      })
      .catch((err) => {
        console.error('Failed to load Wrapped stats:', err)
        setFetchError(err instanceof Error ? err.message : String(err))
      })
      .finally(() => {
        setLoading(false)
      })
  }, [queryYear])

  useEffect(() => {
    loadData()
  }, [loadData])

  // Slide navigation
  const nextSlide = useCallback(() => {
    setCurrentSlide((prev) => {
      if (prev < TOTAL_SLIDES - 1) {
        setSlideProgress(0)
        return prev + 1
      }
      return prev
    })
  }, [])

  const prevSlide = useCallback(() => {
    setCurrentSlide((prev) => {
      if (prev > 0) {
        setSlideProgress(0)
        return prev - 1
      }
      return 0
    })
  }, [])

  const goToSlide = (idx: number) => {
    setCurrentSlide(idx)
    setSlideProgress(0)
  }

  // Timer auto-advance (stops on the last slide to let the user review)
  useEffect(() => {
    if (loading || !data || isPaused || currentSlide >= TOTAL_SLIDES - 1) return

    const interval = 50
    const step = (interval / SLIDE_DURATION_MS) * 100

    const timer = setInterval(() => {
      setSlideProgress((prev) => {
        if (prev + step >= 100) {
          nextSlide()
          return 0
        }
        return prev + step
      })
    }, interval)

    return () => clearInterval(timer)
  }, [loading, data, isPaused, nextSlide, currentSlide])

  // Keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'ArrowRight') {
        e.preventDefault()
        nextSlide()
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault()
        prevSlide()
      } else if (e.key === ' ' || e.key === 'p' || e.key === 'P') {
        e.preventDefault()
        setIsManuallyPaused((prev) => !prev)
      } else if (e.key === 'Escape') {
        e.preventDefault()
        route(routeTo.stats())
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [nextSlide, prevSlide])

  const handleYearChange = (e: Event) => {
    const selected = (e.target as HTMLSelectElement).value
    route(routeTo.wrapped(parseInt(selected, 10)))
  }

  if (loading) {
    return (
      <div className={s.container}>
        <div className={s.topBar}>
          <div className={s.headerControls}>
            <div className={s.brand}>
              <span>Trackarr Wrapped</span>
            </div>
            <button className={s.controlBtn} onClick={() => route(routeTo.stats())} title={t('wrapped.backToStats')}>
              ✕
            </button>
          </div>
        </div>
        <div className={s.loadingState}>
          <div className={s.loadingSpinner}>✨</div>
          <h2 className={s.loadingTitle}>Trackarr Wrapped</h2>
          <p className={s.loadingSubtitle}>{t('wrapped.loadingMessage')}</p>
        </div>
      </div>
    )
  }

  if (fetchError || !data || data.overview.total_titles === 0) {
    const isErr = Boolean(fetchError)
    return (
      <div className={s.container}>
        <div className={s.topBar}>
          <div className={s.headerControls}>
            <div className={s.brand}>
              <span>Trackarr Wrapped</span>
              {data && <span className={s.yearBadge}>{data.year}</span>}
            </div>
            <button className={s.controlBtn} onClick={() => route(routeTo.stats())} title={t('wrapped.backToStats')}>
              ✕
            </button>
          </div>
        </div>
        <div className={s.emptyState}>
          <div className={s.emptyIcon}>{isErr ? '⚠️' : '🎬'}</div>
          <h2 className={s.emptyTitle}>
            {isErr ? t('wrapped.loadingError') : t('wrapped.noDataTitle', { year: data?.year || queryYear || new Date().getFullYear() })}
          </h2>
          <p className={s.emptyDesc}>
            {isErr ? fetchError : t('wrapped.noDataDesc')}
          </p>
          <div className={s.emptyActions}>
            {isErr && (
              <button className={s.replayBtn} onClick={loadData}>
                🔄 {t('wrapped.retry')}
              </button>
            )}
            <button className={s.exitBtn} onClick={() => route(routeTo.stats())}>
              {t('wrapped.backToStats')}
            </button>
          </div>
        </div>
      </div>
    )
  }

  // Dynamic backdrop cover based on slide
  let currentCoverUrl: string | null = null
  if (currentSlide === 1) {
    currentCoverUrl = data.top_favorites[selectedCategoryFav]?.[0]?.cover_url ?? null
  } else if (currentSlide === 2) {
    currentCoverUrl = data.top_releases[selectedCategoryRel]?.[0]?.cover_url ?? null
  } else if (currentSlide === 3) {
    currentCoverUrl = data.rewatch_champion?.title?.cover_url ?? null
  } else if (currentSlide === 0) {
    currentCoverUrl = data.top_favorites.movies[0]?.cover_url || data.top_favorites.series[0]?.cover_url || null
  }

  const backdropStyle = currentCoverUrl
    ? { backgroundImage: `url(${getCoverUrl(currentCoverUrl)})` }
    : undefined

  return (
    <div
      className={s.container}
      onMouseDown={() => setIsHolding(true)}
      onMouseUp={() => setIsHolding(false)}
      onTouchStart={() => setIsHolding(true)}
      onTouchEnd={() => setIsHolding(false)}
    >
      <div className={s.backdrop} style={backdropStyle} />
      <div className={s.gradientOverlay} />

      {/* Progress Bars & Controls */}
      <div className={s.topBar}>
        <div className={s.progressContainer}>
          {Array.from({ length: TOTAL_SLIDES }).map((_, idx) => {
            let width = '0%'
            if (idx < currentSlide) width = '100%'
            else if (idx === currentSlide) width = currentSlide === TOTAL_SLIDES - 1 ? '100%' : `${slideProgress}%`
            return (
              <button
                key={idx}
                type="button"
                data-testid={`progress-${idx}`}
                className={s.progressBarBg}
                onClick={(e) => {
                  e.stopPropagation()
                  goToSlide(idx)
                }}
              >
                <div className={s.progressBarFill} style={{ width }} />
              </button>
            )
          })}
        </div>

        <div className={s.headerControls}>
          <div className={s.brand}>
            <span>Trackarr Wrapped</span>
            <span className={s.yearBadge}>{data.year}</span>
          </div>

          <div className={s.actions}>
            {data.available_years.length > 1 && (
              <select
                className={s.yearSelect}
                value={data.year}
                onChange={handleYearChange}
                onClick={(e) => e.stopPropagation()}
              >
                {data.available_years.map((y) => (
                  <option key={y} value={y}>
                    {y}
                  </option>
                ))}
              </select>
            )}
            <button
              className={s.controlBtn}
              onClick={(e) => {
                e.stopPropagation()
                setIsManuallyPaused((prev) => !prev)
              }}
              title={isManuallyPaused ? t('wrapped.resumeStory') : t('wrapped.pauseStory')}
            >
              {isManuallyPaused ? '▶' : '⏸'}
            </button>
            <button
              className={s.controlBtn}
              onClick={(e) => {
                e.stopPropagation()
                route(routeTo.stats())
              }}
              title={t('wrapped.backToStats')}
            >
              ✕
            </button>
          </div>
        </div>
      </div>

      {/* Main Interactive Stage */}
      <div className={s.mainStage}>
        <button
          type="button"
          aria-label={t('wrapped.prevSlide')}
          data-testid="tap-left"
          className={s.tapZoneLeft}
          onClick={(e) => {
            e.stopPropagation()
            prevSlide()
          }}
        />
        <button
          type="button"
          aria-label={t('wrapped.nextSlide')}
          data-testid="tap-right"
          className={s.tapZoneRight}
          onClick={(e) => {
            e.stopPropagation()
            nextSlide()
          }}
        />

        {currentSlide === 0 && <SlideOverview data={data} t={t} />}
        {currentSlide === 1 && (
          <SlideFavorites
            topFavorites={data.top_favorites}
            category={selectedCategoryFav}
            onSelectCategory={setSelectedCategoryFav}
            year={data.year}
            t={t}
          />
        )}
        {currentSlide === 2 && (
          <SlideReleases
            topReleases={data.top_releases}
            category={selectedCategoryRel}
            onSelectCategory={setSelectedCategoryRel}
            year={data.year}
            t={t}
          />
        )}
        {currentSlide === 3 && <SlideRewatch rewatch={data.rewatch_champion} year={data.year} t={t} />}
        {currentSlide === 4 && (
          <SlideCastAndGenres
            actors={data.top_actors}
            directors={data.top_directors}
            genres={data.top_genres}
            year={data.year}
            t={t}
          />
        )}
        {currentSlide === 5 && (
          <SlidePersona
            persona={data.persona}
            year={data.year}
            createdAt={data.created_at}
            onReplay={() => goToSlide(0)}
            onExit={() => route(routeTo.stats())}
            t={t}
          />
        )}
      </div>
    </div>
  )
}

function getWatchtimeEquivalent(
  minutes: number,
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
): string | null {
  if (!minutes || minutes < 60) return null

  const minutesPerDay = 60 * 24
  const minutesPerMonth = minutesPerDay * 30.4375
  const minutesPerYear = minutesPerDay * 365.25

  if (minutes >= minutesPerYear) {
    const years = (minutes / minutesPerYear).toFixed(1)
    const days = Math.round(minutes / minutesPerDay)
    return t('wrapped.watchTimeEquivalentYears', { years, days })
  }
  if (minutes >= minutesPerMonth) {
    const months = (minutes / minutesPerMonth).toFixed(1)
    const days = Math.round(minutes / minutesPerDay)
    return t('wrapped.watchTimeEquivalentMonths', { months, days })
  }
  if (minutes >= minutesPerDay) {
    const days = (minutes / minutesPerDay).toFixed(1)
    return t('wrapped.watchTimeEquivalentDays', { days })
  }
  const hours = Math.round(minutes / 60)
  return t('wrapped.watchTimeEquivalentHours', { hours })
}

// Slide 1: Overview
function SlideOverview({
  data,
  t,
}: {
  data: WrappedResponse
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  const watchTimeStr = formatWatchtime(data.total_watch_minutes) || '—'
  const equivalentStr = getWatchtimeEquivalent(data.total_watch_minutes, t)
  return (
    <div className={s.slideContent}>
      <div className={s.slideHeader}>
        <h1 className={s.slideTitle}>{t('wrapped.slide1Title')}</h1>
        <p className={s.slideSubtitle}>{t('wrapped.slide1Subtitle', { year: data.year })}</p>
      </div>

      <div className={s.overviewGrid}>
        <div className={s.overviewHeroCard}>
          <div className={s.overviewHeroValue}>{watchTimeStr}</div>
          {equivalentStr && <div className={s.overviewHeroEquivalent}>⚡ {equivalentStr}</div>}
          <div className={s.overviewHeroLabel}>{t('wrapped.totalWatchTime')}</div>
        </div>

        <div className={s.overviewMiniCard}>
          <div className={s.overviewMiniValue}>{data.overview.total_titles.toLocaleString()}</div>
          <div className={s.overviewMiniLabel}>{t('wrapped.titlesWatched')}</div>
        </div>

        <div className={s.overviewMiniCard}>
          <div className={s.overviewMiniValue}>{data.overview.episodes_watched.toLocaleString()}</div>
          <div className={s.overviewMiniLabel}>{t('wrapped.episodesWatched')}</div>
        </div>

        <div className={s.overviewMiniCard}>
          <div className={s.overviewMiniValue}>
            {data.overview.average_rating > 0 ? `★ ${data.overview.average_rating.toFixed(1)}` : '—'}
          </div>
          <div className={s.overviewMiniLabel}>{t('wrapped.averageRating')}</div>
        </div>

        <div className={s.overviewMiniCard}>
          <div className={s.overviewMiniValue}>{`${Math.round(data.overview.completion_rate * 100)}%`}</div>
          <div className={s.overviewMiniLabel}>{t('wrapped.completionRate')}</div>
        </div>
      </div>
    </div>
  )
}

// Slide 2: Top Favorites
function SlideFavorites({
  topFavorites,
  category,
  onSelectCategory,
  year,
  t,
}: {
  topFavorites: WrappedResponse['top_favorites']
  category: 'movies' | 'series' | 'anime'
  onSelectCategory: (c: 'movies' | 'series' | 'anime') => void
  year: number
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  const titles = topFavorites[category] || []

  return (
    <div className={s.slideContent}>
      <div className={s.slideHeader}>
        <h1 className={s.slideTitle}>{t('wrapped.slide2Title')}</h1>
        <p className={s.slideSubtitle}>{t('wrapped.slide2Subtitle', { year })}</p>
      </div>

      <div className={s.categoryTabs} onClick={(e) => e.stopPropagation()}>
        <button
          className={`${s.categoryTab} ${category === 'movies' ? s.categoryTabActive : ''}`}
          onClick={() => onSelectCategory('movies')}
        >
          🎬 {t('wrapped.categoryMovies')}
        </button>
        <button
          className={`${s.categoryTab} ${category === 'series' ? s.categoryTabActive : ''}`}
          onClick={() => onSelectCategory('series')}
        >
          📺 {t('wrapped.categorySeries')}
        </button>
        <button
          className={`${s.categoryTab} ${category === 'anime' ? s.categoryTabActive : ''}`}
          onClick={() => onSelectCategory('anime')}
        >
          ⛩️ {t('wrapped.categoryAnime')}
        </button>
      </div>

      <div className={s.titlesList}>
        {titles.length === 0 ? (
          <p className={s.emptyDesc}>{t('wrapped.noFavorites')}</p>
        ) : (
          titles.map((item, idx) => <TitleRowItem key={item.id} item={item} rank={idx + 1} t={t} />)
        )}
      </div>
    </div>
  )
}

// Slide 3: Top Releases
function SlideReleases({
  topReleases,
  category,
  onSelectCategory,
  year,
  t,
}: {
  topReleases: WrappedResponse['top_releases']
  category: 'movies' | 'series' | 'anime'
  onSelectCategory: (c: 'movies' | 'series' | 'anime') => void
  year: number
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  const titles = topReleases[category] || []

  return (
    <div className={s.slideContent}>
      <div className={s.slideHeader}>
        <h1 className={s.slideTitle}>{t('wrapped.slide3Title', { year })}</h1>
        <p className={s.slideSubtitle}>{t('wrapped.slide3Subtitle')}</p>
      </div>

      <div className={s.categoryTabs} onClick={(e) => e.stopPropagation()}>
        <button
          className={`${s.categoryTab} ${category === 'movies' ? s.categoryTabActive : ''}`}
          onClick={() => onSelectCategory('movies')}
        >
          🎬 {t('wrapped.categoryMovies')}
        </button>
        <button
          className={`${s.categoryTab} ${category === 'series' ? s.categoryTabActive : ''}`}
          onClick={() => onSelectCategory('series')}
        >
          📺 {t('wrapped.categorySeries')}
        </button>
        <button
          className={`${s.categoryTab} ${category === 'anime' ? s.categoryTabActive : ''}`}
          onClick={() => onSelectCategory('anime')}
        >
          ⛩️ {t('wrapped.categoryAnime')}
        </button>
      </div>

      <div className={s.titlesList}>
        {titles.length === 0 ? (
          <p className={s.emptyDesc}>{t('wrapped.noReleases', { year })}</p>
        ) : (
          titles.map((item, idx) => <TitleRowItem key={item.id} item={item} rank={idx + 1} t={t} />)
        )}
      </div>
    </div>
  )
}

function TitleRowItem({ item, rank, t }: { item: WrappedTitleItem; rank: number; t: (key: TranslationKey, params?: Record<string, string | number>) => string }) {
  const rankClass = rank === 1 ? s.rank1 : rank === 2 ? s.rank2 : s.rank3
  const cover = getCoverUrl(item.cover_url)
  const isMovie = item.type === 'movie'
  const countLabel = item.watch_count > 1
    ? (isMovie ? t('wrapped.movieWatches', { count: item.watch_count }) : t('wrapped.seriesEpisodesWatched', { count: item.watch_count }))
    : ''

  return (
    <div className={s.titleRowCard}>
      <div className={`${s.rankBadge} ${rankClass}`}>#{rank}</div>
      {cover && <img src={cover} alt={item.title} className={s.posterThumb} />}
      <div className={s.titleRowInfo}>
        <div className={s.titleRowName}>{item.title}</div>
        <div className={s.titleRowMeta}>
          {item.year ? `${item.year}` : ''}
          {countLabel ? ` • ${countLabel}` : ''}
        </div>
      </div>
      {item.my_rating != null && <div className={s.ratingPill}>★ {item.my_rating}</div>}
    </div>
  )
}

// Slide 4: Rewatch Champion
function SlideRewatch({
  rewatch,
  year,
  t,
}: {
  rewatch?: WrappedResponse['rewatch_champion']
  year: number
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  if (!rewatch) {
    return (
      <div className={s.slideContent}>
        <div className={s.slideHeader}>
          <h1 className={s.slideTitle}>{t('wrapped.slide4Title')}</h1>
          <p className={s.slideSubtitle}>{t('wrapped.slide4Subtitle')}</p>
        </div>
        <p className={s.emptyDesc}>{t('wrapped.noFavorites')}</p>
      </div>
    )
  }

  const cover = getCoverUrl(rewatch.title.cover_url)
  const isMovie = rewatch.is_movie
  const mainBadgeLabel = isMovie
    ? t('wrapped.totalPlaysMovie', { count: rewatch.total_plays, year })
    : rewatch.total_plays > 1
      ? t('wrapped.seriesCycles', { count: rewatch.total_plays, year })
      : t('wrapped.seriesCyclesSingle', { year })

  const hasEpDetail = !isMovie && rewatch.total_episodes != null && rewatch.distinct_episodes != null && rewatch.total_episodes > 0

  return (
    <div className={s.slideContent}>
      <div className={s.slideHeader}>
        <h1 className={s.slideTitle}>{t('wrapped.slide4Title')}</h1>
        <p className={s.slideSubtitle}>{t('wrapped.slide4Subtitle')}</p>
      </div>

      <div className={s.rewatchHero}>
        {cover && <img src={cover} alt={rewatch.title.title} className={s.rewatchCover} />}
        <div className={s.rewatchTitle}>{rewatch.title.title}</div>
        <div className={s.rewatchBadge}>
          {mainBadgeLabel}
        </div>
        {hasEpDetail && (
          <div className={s.rewatchSubBadge}>
            🔁 {t('wrapped.seriesEpisodesDetail', {
              plays: rewatch.total_episodes!,
              distinct: rewatch.distinct_episodes!,
            })}
          </div>
        )}
        <p className={s.slideSubtitle}>
          {isMovie ? t('wrapped.movieRewatch') : t('wrapped.seriesRewatch')}
        </p>
      </div>
    </div>
  )
}

// Slide 5: Cast, Crew & Genres
function SlideCastAndGenres({
  actors,
  directors,
  genres,
  year,
  t,
}: {
  actors: WrappedResponse['top_actors']
  directors: WrappedResponse['top_directors']
  genres: WrappedResponse['top_genres']
  year: number
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  return (
    <div className={s.slideContent}>
      <div className={s.slideHeader}>
        <h1 className={s.slideTitle}>{t('wrapped.slide5Title')}</h1>
        <p className={s.slideSubtitle}>{t('wrapped.slide5Subtitle', { year })}</p>
      </div>

      <div className={s.castGrid}>
        {/* Actors */}
        <div className={s.castSection}>
          <div className={s.castSectionTitle}>🌟 {t('wrapped.favoriteActors')}</div>
          <div className={s.castList}>
            {actors.slice(0, 4).map((a) => (
              <div key={a.name} className={s.castItem}>
                <span className={s.castName}>{a.name}</span>
                <span className={s.castCount}>{a.count} titles</span>
              </div>
            ))}
          </div>
        </div>

        {/* Directors */}
        <div className={s.castSection}>
          <div className={s.castSectionTitle}>🎬 {t('wrapped.favoriteDirectors')}</div>
          <div className={s.castList}>
            {directors.slice(0, 4).map((d) => (
              <div key={d.name} className={s.castItem}>
                <span className={s.castName}>{d.name}</span>
                <span className={s.castCount}>{d.count} titles</span>
              </div>
            ))}
          </div>
        </div>

        {/* Top Genres */}
        {genres.length > 0 && (
          <div className={s.genresSection}>
            <div className={s.castSectionTitle}>🏷️ {t('wrapped.favoriteGenres')}</div>
            <div className={s.genrePills}>
              {genres.map((g) => (
                <span key={g.genre} className={s.genrePill}>
                  {g.genre} ({g.count})
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// Slide 6: Gemini AI Persona
function SlidePersona({
  persona,
  year,
  createdAt,
  onReplay,
  onExit,
  t,
}: {
  persona: WrappedResponse['persona']
  year: number
  createdAt?: string | null
  onReplay: () => void
  onExit: () => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  return (
    <div className={s.slideContent}>
      <div className={s.slideHeader}>
        <h1 className={s.slideTitle}>{t('wrapped.slide6Title')}</h1>
        <p className={s.slideSubtitle}>{t('wrapped.slide6Subtitle', { year })}</p>
      </div>

      <div className={s.personaCard}>
        <div className={s.personaIcon}>🔮</div>
        <h2 className={s.personaArchetype}>{persona.title}</h2>

        {persona.badges && persona.badges.length > 0 && (
          <div className={s.personaBadges}>
            {persona.badges.map((b) => (
              <span key={b} className={s.personaBadgeItem}>
                {b}
              </span>
            ))}
          </div>
        )}

        {persona.quote && <blockquote className={s.personaQuote}>« {persona.quote} »</blockquote>}

        {persona.summary && <p className={s.personaSummary}>{persona.summary}</p>}

        {persona.fun_facts && persona.fun_facts.length > 0 && (
          <div className={s.funFactsBox}>
            <div className={s.funFactsTitle}>💡 {t('wrapped.funFactsTitle')}</div>
            <ul className={s.funFactsList}>
              {persona.fun_facts.map((fact: string, i: number) => (
                <li key={i}>{fact}</li>
              ))}
            </ul>
          </div>
        )}

        <div className={s.finalActions} onClick={(e) => e.stopPropagation()}>
          <button className={s.replayBtn} onClick={onReplay}>
            🔄 {t('wrapped.replay')}
          </button>
          <button className={s.exitBtn} onClick={onExit}>
            ✨ {t('wrapped.backToStats')}
          </button>
        </div>

        {createdAt && (
          <div className={s.archivedDateLabel}>
            {t('wrapped.archivedOn', { date: new Date(createdAt).toLocaleDateString() })}
          </div>
        )}
      </div>
    </div>
  )
}
