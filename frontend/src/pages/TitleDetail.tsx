import { useState, useMemo } from 'preact/hooks'
import type { CSSProperties } from 'preact'
import clsx from 'clsx'
import { route } from 'preact-router'
import type { Title, TitleRelation } from '../types'
import { useApi } from '../hooks/useApi'
import { computeAniListUrl, getName, getAlternativeNames, languageLabel, getTypeLabel, getStatusLabel, formatMatchSource, formatDate, formatDateTime, formatRelativeTime, formatWatchtime, hexToRgba } from '../utils'
import { apiFetch } from '../api'
import { colors } from '../theme'
import { SeasonTab } from '../components/SeasonTab'
import { SeasonAniListStrip } from '../components/SeasonAniListStrip'
import { EpisodeRow } from '../components/EpisodeRow'
import { ActionDrawer } from '../components/ActionDrawer'
import { RatingPrompt } from '../components/RatingPrompt'
import { EditSheet } from '../components/EditSheet'
import { RematchSheet } from '../components/RematchSheet'
import { ArrPushSheet } from '../components/ArrPushSheet'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import { StatusBadge } from '../components/StatusBadge'
import { WatchProviderBadges } from '../components/WatchProviderBadges'
import { getMatchingProviders } from '../utils/providers'
import { ErrorBanner } from '../components/ErrorBanner'
import { coverBackground } from '../components/CoverPlaceholder'
import { CoverImage } from '../components/CoverImage'
import { TitleHistory } from '../components/TitleHistory'
import { SeasonSideStories } from '../components/SeasonSideStories'
import { FranchiseRelationsSection } from '../components/FranchiseRelationsSection'
import { BottomSheet } from '../components/BottomSheet'
import { PullToRefresh } from '../components/PullToRefresh'
import { NextEpisodeHero } from '../components/NextEpisodeHero'
import { PersonalNotesCard } from '../components/PersonalNotesCard'
import { routeTo } from '../routes'
import { useTitleStore } from '../store'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import { useTranslation } from '../i18n'
import { useUndo } from '../context/UndoContext'
import s from './TitleDetail.module.css'

function toggleEpisodeWatched(title: Title, episodeId: number): Title {
  return {
    ...title,
    seasons: title.seasons.map((s) => ({
      ...s,
      episodes: (s.episodes ?? []).map((ep) =>
        ep.id === episodeId ? { ...ep, watched: !ep.watched } : ep
      ),
    })),
  }
}

function formatSeriesStatus(st: string | null) {
  if (!st) return ''
  return st.charAt(0).toUpperCase() + st.slice(1).replace('_', ' ')
}

function formatRuntime(minutes: number): string {
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return h > 0 ? `${h}h ${m.toString().padStart(2, '0')}m` : `${m}m`
}

function parseJSON<T>(json: string | null): T | null {
  if (!json) return null
  try { return JSON.parse(json) } catch { return null }
}

export function TitleDetail({ id }: { id?: string; path?: string }) {
  const { data: title, loading, error, mutate, setData } = useApi<Title>(id ? `/titles/${id}` : null)
  useScrollRestoration(`title-${id ?? ''}`, !loading && title !== null)
  const [activeSeason, setActiveSeason] = useState<number | null>(null)
  const [showRating, setShowRating] = useState(false)
  const [showEdit, setShowEdit] = useState(false)
  const [showRematch, setShowRematch] = useState(false)
  const [rematchSeasonID, setRematchSeasonID] = useState<number | null>(null)
  const [showArrPush, setShowArrPush] = useState(false)
  const [showSonarrReaddConfirm, setShowSonarrReaddConfirm] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [synopsisExpanded, setSynopsisExpanded] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const { showUndo } = useUndo()
  const invalidate = useTitleStore((st) => st.invalidate)

  const sortedSeasons = useMemo(
    () => [...(title?.seasons ?? [])].sort((a, b) => a.season_number - b.season_number),
    [title?.id, title?.seasons]
  )

  const { t } = useTranslation()

  if (loading || !title) {
    return (
      <div className={s.loading}>
        {error ? <ErrorBanner message={error} onRetry={mutate} /> : loading ? t('common.loading') : t('common.notFound')}
      </div>
    )
  }
  const name = getName(title)
  const altNames = getAlternativeNames(title)
  const typeLabel = getTypeLabel(title.type)
  const isSonarrDeleted = title.type === 'series' && title.sonarr_id == null && Boolean(title.sonarr_deleted_at)
  const current = sortedSeasons.find((ss) => ss.season_number === activeSeason)
    ?? sortedSeasons.find((ss) => (ss.episodes ?? []).some((e) => !e.watched))
    ?? sortedSeasons[sortedSeasons.length - 1]

  const currentEps = current?.episodes ?? []
  const watched = currentEps.filter((e) => e.watched).length
  const total = current?.total_episodes ?? currentEps.length
  const pct = total > 0 ? (watched / total) * 100 : 0

  const activeSeasonSideStories = useMemo(() => {
    if (!current || !title.relations) return []
    return title.relations.filter(
      (r) =>
        (r.season_id != null && r.season_id === current.id) ||
        (r.season_number != null && r.season_number === current.season_number)
    )
  }, [current?.id, current?.season_number, title.relations])

  const genres = title.genres
  const credits = parseJSON<{ name: string; role: string }[]>(title.credits)

  const isAllSeasonWatched = useMemo(() => {
    if (!current || !current.episodes || current.episodes.length === 0) return false
    return current.episodes.every((e) => e.watched)
  }, [current])

  const handleSeasonToggleAll = async () => {
    if (!current || !current.episodes || current.episodes.length === 0) return
    const targetWatched = !isAllSeasonWatched
    const epIds = current.episodes.map((e) => e.id)
    const prevTitle = title

    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        seasons: (prev.seasons ?? []).map((s) => {
          if (s.id !== current.id) return s
          return {
            ...s,
            episodes: (s.episodes ?? []).map((e) => ({ ...e, watched: targetWatched })),
          }
        }),
      }
    })

    try {
      const updated = await apiFetch<Title>(`/titles/${title.id}/episodes/batch-watch`, {
        method: 'POST',
        body: JSON.stringify({
          episode_ids: epIds,
          watched: targetWatched,
        }),
      })
      if (updated && updated.id) {
        setData(updated)
      } else {
        await mutate()
      }

      showUndo({
        message: targetWatched
          ? t('undo.seasonMarked', { season: `S${current.season_number}` })
          : t('undo.seasonUnmarked', { season: `S${current.season_number}` }),
        onUndo: async () => {
          setData(prevTitle)
          await apiFetch(`/titles/${title.id}/episodes/batch-watch`, {
            method: 'POST',
            body: JSON.stringify({
              episode_ids: epIds,
              watched: !targetWatched,
            }),
          })
          await mutate()
        },
      })
    } catch {
      setActionError(t('details.failedUpdateSeason'))
      mutate()
    }
  }

  const handleRefresh = async () => {
    try {
      const updated = await apiFetch<Title>(`/titles/${title.id}/refresh?sync=true`, { method: 'POST' })
      if (updated && updated.id) {
        setData(updated)
      } else {
        await mutate()
      }
    } catch {
      await mutate()
    }
  }

  const [desktopRefreshState, setDesktopRefreshState] = useState<'idle' | 'loading' | 'success' | 'error'>('idle')

  const handleDesktopRefresh = async () => {
    if (desktopRefreshState !== 'idle') return
    setDesktopRefreshState('loading')
    try {
      await handleRefresh()
      setDesktopRefreshState('success')
    } catch {
      setDesktopRefreshState('error')
    } finally {
      setTimeout(() => {
        setDesktopRefreshState('idle')
      }, 2000)
    }
  }

  // Pull-to-refresh: refetch the title so the spinner stays up until data lands.
  const handlePullRefresh = async () => {
    const updated = await apiFetch<Title>(`/titles/${title.id}`)
    setData(updated)
  }

  const handleEpisodeToggle = async (episodeId: number) => {
    const foundEp = title?.seasons?.flatMap((s) => s.episodes ?? []).find((e) => e.id === episodeId)
    const wasWatched = foundEp?.watched
    setData((prev) => prev ? toggleEpisodeWatched(prev, episodeId) : prev)
    try {
      const updated = await apiFetch<Title>(`/titles/${title.id}/episodes/${episodeId}`, { method: 'PATCH' })
      setData(updated)
      if (!wasWatched) {
        showUndo({
          message: t('undo.episodeMarked', { ep: `E${foundEp?.episode ?? episodeId}` }),
          onUndo: async () => {
            await handleEpisodeToggle(episodeId)
          },
        })
      }
    } catch (e) {
      setActionError(t('details.failedUpdateEpisode'))
      mutate()
    }
  }

  const handleToggleSideStoryWatched = async (rel: TitleRelation) => {
    if (!rel.matched_title_id) return
    const isCompleted = rel.matched_status === 'completed'
    const newStatus = isCompleted ? 'plan_to_watch' : 'completed'
    try {
      await apiFetch(`/titles/${rel.matched_title_id}`, {
        method: 'PATCH',
        body: JSON.stringify({ status: newStatus }),
      })
      setData((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          relations: (prev.relations ?? []).map((r) =>
            r.id === rel.id || r.external_id === rel.external_id
              ? { ...r, matched_status: newStatus }
              : r
          ),
        }
      })
    } catch (e) {
      setActionError(t('details.failedUpdateTitleStatus'))
    }
  }

  const handleSaveRating = async (rating: number) => {
    setShowRating(false)
    setData((prev) => prev ? { ...prev, my_rating: rating } : prev)
    try {
      const updated = await apiFetch<Title>(`/titles/${title.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ my_rating: rating }),
      })
      setData(updated)
    } catch (e) {
      setActionError(t('details.failedSaveRating'))
      mutate()
    }
  }

  const handleSaveEdit = async (updates: { type?: string; status?: string; is_anime?: boolean; delete_from_sonarr?: boolean }) => {
    setShowEdit(false)
    if (Object.keys(updates).length === 0) return
    try {
      const updated = await apiFetch<Title>(`/titles/${title.id}`, {
        method: 'PATCH',
        body: JSON.stringify(updates),
      })
      setData(updated)
    } catch (e) {
      setActionError(t('details.failedSaveChanges'))
      mutate()
    }
  }

  const handleArrPushSuccess = (arrId: number) => {
    setData((prev) => {
      if (!prev) return prev
      return prev.type === 'movie'
        ? { ...prev, radarr_id: arrId }
        : { ...prev, sonarr_id: arrId, sonarr_deleted_at: null }
    })
  }

  // Deleting removes the whole title (seasons, episodes, history). We can't stay
  // on a page whose subject no longer exists, so route back to the library and
  // invalidate its cache so the deleted title drops out of the list.
  const handleDelete = async () => {
    setShowDeleteConfirm(false)
    const targetId = title.id
    const targetName = getName(title)
    route(routeTo.home())
    showUndo({
      message: t('undo.titleDeleted', { title: targetName }),
      onUndo: () => {
        route(routeTo.title(targetId))
      },
      onExpire: async () => {
        try {
          await apiFetch(`/titles/${targetId}`, { method: 'DELETE' })
          invalidate()
        } catch (e) {
          console.error('Failed to delete title:', e)
        }
      },
    })
  }

  // Build meta line
  const metaParts = [typeLabel, String(title.year)]
  if (title.runtime) metaParts.push(formatRuntime(title.runtime))
  if (title.series_status) metaParts.push(formatSeriesStatus(title.series_status))

  const coverBg = coverBackground(title.cover_url, title.type)

  const pageStyle = {
    '--cover-bg': coverBg,
    ...(title.accent_hex && {
      '--cover-accent': title.accent_hex,
      '--cover-accent-wash': hexToRgba(title.accent_hex, 0.10),
    }),
  } as CSSProperties

  return (
    <PullToRefresh
      onRefresh={handlePullRefresh}
      disabled={drawerOpen || showRating || showEdit || showRematch || showHistory || showArrPush}
    >
    <div className={s.page} style={pageStyle}>
      {actionError && <ErrorBanner message={actionError} onRetry={() => setActionError(null)} />}

      {/* Atmospheric backdrop for wide/desktop view */}
      <div className={s.backdrop} aria-hidden="true" />

      {/* Hero — spacer on mobile, holds back button */}
      <div className={s.hero}>
        <button onClick={() => history.back()} aria-label={t('common.back')} className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
      </div>

      {/* Layout Grid / Container */}
      <div className={s.layoutContainer}>
        {/* Left column (Desktop: sticky sidebar, Mobile: cards flow underneath) */}
        <aside className={s.sidebarCol}>
          {/* Desktop authentic 2:3 vertical poster */}
          <div className={s.desktopPosterWrap}>
            <CoverImage
              coverUrl={title.cover_url}
              type={title.type}
              is_anime={title.is_anime}
              className={s.desktopPoster}
              iconSize="48px"
            />
          </div>

          {/* Desktop Quick Action Toolbar */}
          <div className={s.desktopActionToolbar}>
            <div className={s.desktopActionRow}>
              <button
                type="button"
                className={clsx(s.desktopActionBtn, s.desktopActionBtnPrimary)}
                onClick={() => setShowEdit(true)}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                </svg>
                {t('common.edit')}
              </button>
              <button
                type="button"
                className={clsx(s.desktopActionBtn, desktopRefreshState === 'loading' && s.desktopActionBtnDisabled)}
                onClick={handleDesktopRefresh}
                disabled={desktopRefreshState !== 'idle'}
              >
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  className={clsx(desktopRefreshState === 'loading' && s.spinIcon)}
                >
                  <polyline points="23 4 23 10 17 10" />
                  <polyline points="1 20 1 14 7 14" />
                  <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
                </svg>
                {desktopRefreshState === 'loading' ? '...' : desktopRefreshState === 'success' ? t('common.refreshDone') : desktopRefreshState === 'error' ? t('common.refreshFailed') : t('common.refresh')}
              </button>
            </div>

            <div className={s.desktopActionRow}>
              {isSonarrDeleted ? (
                <button
                  type="button"
                  onClick={() => setShowSonarrReaddConfirm(true)}
                  className={clsx(s.desktopActionBtn, s.desktopActionBtnArr)}
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="1 4 1 10 7 10" />
                    <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10" />
                  </svg>
                  {t('details.readdToSonarr')}
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => setShowArrPush(true)}
                  className={clsx(s.desktopActionBtn, s.desktopActionBtnArr)}
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    {title.radarr_id != null || title.sonarr_id != null ? (
                      <>
                        <circle cx="12" cy="12" r="3" />
                        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                      </>
                    ) : (
                      <>
                        <line x1="22" y1="2" x2="11" y2="13" />
                        <polygon points="22 2 15 22 11 13 2 9 22 2" />
                      </>
                    )}
                  </svg>
                  {title.radarr_id != null || title.sonarr_id != null
                    ? t('details.manageInArr', { app: title.type === 'movie' ? 'Radarr' : 'Sonarr' })
                    : t('details.sendToArr', { app: title.type === 'movie' ? 'Radarr' : 'Sonarr' })}
                </button>
              )}
            </div>

            <div className={s.desktopActionRow}>
              <button
                type="button"
                className={s.desktopActionBtn}
                onClick={() => setShowRematch(true)}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                  <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
                </svg>
                {t('common.rematch')}
              </button>
              <button
                type="button"
                className={s.desktopActionBtn}
                onClick={() => route(`/search?mergeSourceId=${title.id}&mergeSourceName=${encodeURIComponent(name)}`)}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M8 7v10M12 12l4 4m0-8l-4 4" />
                </svg>
                {t('common.merge')}
              </button>
            </div>

            <div className={s.desktopActionRow}>
              <button
                type="button"
                className={clsx(s.desktopActionBtn, s.desktopActionBtnDanger)}
                onClick={() => setShowDeleteConfirm(true)}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="3 6 5 6 21 6" />
                  <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                </svg>
                {t('common.delete')}
              </button>
            </div>
          </div>

          {/* Details card (fiche technique) */}
          <div className={s.card}>
            <div className={s.cardLabel}>{t('details.mediaDetails')}</div>
            <div className={s.detailRow}>
              <span className={s.detailKey}>{t('details.added')}</span>
              <span className={s.detailVal}>{formatDate(title.created_at)}</span>
            </div>
            {title.last_watched_at && (
              <div className={s.detailRow}>
                <span className={s.detailKey}>{t('details.lastWatched')}</span>
                <span className={s.detailVal}>{formatDate(title.last_watched_at)}</span>
              </div>
            )}
            {formatWatchtime(title.total_watch_minutes) && (
              <div className={s.detailRow}>
                <span className={s.detailKey}>{t('details.watchTime')}</span>
                <span className={s.detailVal}>{formatWatchtime(title.total_watch_minutes)}</span>
              </div>
            )}
            <div className={s.detailRow}>
              <span className={s.detailKey}>{t('details.lastRefreshed')}</span>
              <span
                className={s.detailVal}
                title={title.last_refreshed_at ? formatDateTime(title.last_refreshed_at) : undefined}
              >
                {title.last_refreshed_at ? formatRelativeTime(title.last_refreshed_at) : t('details.never')}
              </span>
            </div>
            {title.match_source && (
              <div className={s.detailRow}>
                <span className={s.detailKey}>{t('details.match')}</span>
                <span className={s.detailVal}>{formatMatchSource(title.match_source)}</span>
              </div>
            )}
            {(title.imdb_id || (title.tmdb_id != null && title.tmdb_id > 0) || (title.tvdb_id != null && title.tvdb_id > 0) || computeAniListUrl(title)) && (
              <div className={s.detailRow}>
                <span className={s.detailKey}>{t('details.sources')}</span>
                <div className={s.externalLinksWrap}>
                  {title.imdb_id && (
                    <a
                      href={`https://www.imdb.com/title/${title.imdb_id}/`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={`${s.extLinkBadge} ${s.extLinkImdb}`}
                    >
                      IMDb
                    </a>
                  )}
                  {title.tmdb_id != null && title.tmdb_id > 0 && (
                    <a
                      href={`https://www.themoviedb.org/${title.type === 'movie' ? 'movie' : 'tv'}/${title.tmdb_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={`${s.extLinkBadge} ${s.extLinkTmdb}`}
                    >
                      TMDB
                    </a>
                  )}
                  {title.tvdb_id != null && title.tvdb_id > 0 && (
                    <a
                      href={`https://thetvdb.com/dereferrer/${title.type === 'movie' ? 'movie' : 'series'}/${title.tvdb_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={`${s.extLinkBadge} ${s.extLinkTvdb}`}
                    >
                      TVDB
                    </a>
                  )}
                  {computeAniListUrl(title) && (
                    <a
                      href={computeAniListUrl(title)!}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={`${s.extLinkBadge} ${s.extLinkAnilist}`}
                    >
                      AniList
                    </a>
                  )}
                </div>
              </div>
            )}
            {getMatchingProviders(title.watch_providers).length > 0 && (
              <div className={s.detailRow}>
                <span className={s.detailKey}>{t('details.platforms')}</span>
                <div className={s.externalLinksWrap}>
                  <WatchProviderBadges providers={title.watch_providers} />
                </div>
              </div>
            )}
            <div className={clsx(s.detailRow, s.mobileOnlyArrRow)}>
              <span className={s.detailKey}>{title.type === 'movie' ? 'Radarr' : 'Sonarr'}</span>
              <span className={s.detailVal}>
                {isSonarrDeleted ? (
                  <div className={s.arrDeletedGroup}>
                    <span className={s.sonarrDeletedBadge}>
                      {t('details.sonarrDeletedBadge')}
                    </span>
                    <button
                      type="button"
                      onClick={() => setShowSonarrReaddConfirm(true)}
                      className={`${s.arrAddBtn} ${s.sonarrReaddBtn}`}
                    >
                      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <polyline points="1 4 1 10 7 10" />
                        <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10" />
                      </svg>
                      {t('details.readdToSonarr')}
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => setShowArrPush(true)}
                    className={`${s.arrAddBtn} ${title.type === 'movie' ? s.radarrBtn : s.sonarrBtn}`}
                  >
                    {title.radarr_id != null || title.sonarr_id != null ? (
                      <>
                        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                          <circle cx="12" cy="12" r="3" />
                          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                        </svg>
                        {t('details.manageInArr', { app: title.type === 'movie' ? 'Radarr' : 'Sonarr' })}
                      </>
                    ) : (
                      <>
                        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                          <line x1="22" y1="2" x2="11" y2="13" />
                          <polygon points="22 2 15 22 11 13 2 9 22 2" />
                        </svg>
                        {t('details.sendToArr', { app: title.type === 'movie' ? 'Radarr' : 'Sonarr' })}
                      </>
                    )}
                  </button>
                )}
              </span>
            </div>
            {title.original_title && title.original_title !== name && (
              <div className={s.detailRow}>
                <span className={s.detailKey}>{t('details.originalTitle')}</span>
                <span className={s.detailVal}>{title.original_title}</span>
              </div>
            )}
            {altNames.length > 0 && (
              <div className={s.altNames}>
                <div className={s.altNamesLabel}>{t('details.altNames')}</div>
                {altNames.map((alt) => {
                  const lang = languageLabel(alt.language)
                  return (
                    <div key={`${alt.language}-${alt.name}`} className={s.altNameRow}>
                      <span className={s.altNameFlag} title={lang.label}>{lang.flag}</span>
                      <span className={s.altNameText}>{alt.name}</span>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* Personal Notes */}
          <PersonalNotesCard
            titleId={title.id}
            initialNotes={title.personal_notes}
            onSaved={(notes) => setData((prev) => (prev ? { ...prev, personal_notes: notes } : prev))}
          />
        </aside>

        {/* Right column (Desktop: identity header, hero, ratings, synopsis, seasons & episodes, cast, franchise) */}
        <div className={s.mainCol}>
          {/* Identity zone */}
          <div className={s.identity}>
            {/* Tablet-only authentic 2:3 poster (640px - 1023px) */}
            <div className={s.tabletPosterWrap}>
              <CoverImage
                coverUrl={title.cover_url}
                type={title.type}
                is_anime={title.is_anime}
                className={s.tabletPoster}
                iconSize="36px"
              />
            </div>

            <div className={s.identityInfo}>
              <div className={s.identityTitle}>{name}</div>
              <div className={s.identityMeta}>{metaParts.join(' · ')}</div>
              {genres && genres.length > 0 && (
                <div className={s.genrePills}>
                  {genres.map((g) => <span key={g} className={s.genrePill}>{g}</span>)}
                </div>
              )}
              <div style={{ marginTop: '12px' }}>
                <StatusBadge status={title.status} caughtUp={title.caught_up} />
              </div>
            </div>
          </div>

          {/* Hero & Rating Action Row */}
          <div className={s.actionRow}>
            {/* Next Episode Hero Button & Binge Estimator */}
            <NextEpisodeHero
              title={title}
              onEpisodeToggle={handleEpisodeToggle}
              onStatusChange={(status) => handleSaveEdit({ status })}
            />

            {/* Ratings card */}
            <div className={clsx(s.card, s.ratingsCard)}>
              {title.my_rating != null && title.my_rating > 0 ? (
                /* State B: Already Rated (Clean display + Edit action) */
                <div className={s.ratingHeaderRow}>
                  <div className={s.ratedScoreGroup}>
                    <span className={s.statLabelTerminal}>{t('details.myRating')}</span>
                    <span className={s.myRating}>{title.my_rating}</span>
                    <span className={s.myRatingSuffix}>/10</span>
                  </div>
                  <div className={s.ratedActionsGroup}>
                    <div className={s.extRatings}>
                      {title.tmdb_rating != null && (
                        <div className={s.extItem}>
                          <div className={`${s.extScore} ${s.tmdbColor}`}>{title.tmdb_rating.toFixed(1)}</div>
                          <div className={s.extSource}>TMDB</div>
                        </div>
                      )}
                      {title.anilist_rating != null && (
                        <div className={s.extItem}>
                          <div className={`${s.extScore} ${s.anilistColor}`}>{title.anilist_rating}%</div>
                          <div className={s.extSource}>AniList</div>
                        </div>
                      )}
                    </div>
                    <button
                      type="button"
                      className={s.btnEditRating}
                      onClick={() => setShowRating(true)}
                    >
                      {t('details.editRating')}
                    </button>
                  </div>
                </div>
              ) : (
                /* State A: Unrated (Direct 1-to-10 1-tap strip) */
                <div>
                  <div className={s.ratingHeaderRow} style={{ marginBottom: '8px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span className={s.statLabelTerminal}>{t('details.rateThisTitle')}</span>
                      <span className={s.noRating}>{t('details.notRated')}</span>
                    </div>
                    <div className={s.extRatings}>
                      {title.tmdb_rating != null && (
                        <div className={s.extItem}>
                          <div className={`${s.extScore} ${s.tmdbColor}`}>{title.tmdb_rating.toFixed(1)}</div>
                          <div className={s.extSource}>TMDB</div>
                        </div>
                      )}
                      {title.anilist_rating != null && (
                        <div className={s.extItem}>
                          <div className={`${s.extScore} ${s.anilistColor}`}>{title.anilist_rating}%</div>
                          <div className={s.extSource}>AniList</div>
                        </div>
                      )}
                    </div>
                  </div>
                  <div className={s.rateStrip}>
                    {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((val) => (
                      <button
                        key={val}
                        type="button"
                        className={s.rateBtn}
                        onClick={() => handleSaveRating(val)}
                        aria-label={t('ratingPrompt.rateValueAria', { val })}
                      >
                        {val}
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Synopsis card */}
          {title.overview && (
            <div className={s.card}>
              <div className={s.cardLabel}>{t('details.overview')}</div>
              <div className={`${s.synopsisText} ${!synopsisExpanded ? s.synopsisClamped : ''}`}>
                {title.overview}
              </div>
              <button className={s.synopsisToggle} onClick={() => setSynopsisExpanded(!synopsisExpanded)}>
                {synopsisExpanded ? t('details.showLess') : t('details.showMore')}
              </button>
            </div>
          )}

          {/* Hub Saisons & Épisodes (Elevated on desktop right after synopsis) */}
          {/* Progress bar (series/anime) */}
          {current && title.type !== 'movie' && (
            <div className={s.progressWrap}>
              <div className={s.progressTrack}>
                <div className={s.progressBar} style={{ width: `${pct}%` }} />
              </div>
              <div className={s.progressHeaderRow}>
                <div className={s.progressLabel}>
                  {t('details.seasonProgress', { season: current.season_number, watched, total })}
                </div>
                <button
                  type="button"
                  className={clsx(s.seasonBatchToggle, isAllSeasonWatched && s.seasonBatchToggleWatched)}
                  onClick={handleSeasonToggleAll}
                  title={isAllSeasonWatched ? t('details.markSeasonUnwatched') : t('details.markSeasonWatched')}
                  aria-label={isAllSeasonWatched ? t('details.markSeasonUnwatched') : t('details.markSeasonWatched')}
                >
                  <span className={s.seasonBatchLabel}>
                    {isAllSeasonWatched ? t('details.markSeasonUnwatched') : t('details.markSeasonWatched')}
                  </span>
                  <span className={s.seasonBatchBox}>
                    {isAllSeasonWatched ? (
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="var(--accent)" stroke="none">
                        <path d="M20 6L9 17l-5-5 1.41-1.41L9 14.17 18.59 4.58z" />
                      </svg>
                    ) : (
                      <div className={s.seasonBatchEmpty} />
                    )}
                  </span>
                </button>
              </div>
            </div>
          )}

          {/* Season tabs */}
          {sortedSeasons.length > 1 && (
            <div className={s.seasonTabs}>
              {sortedSeasons.map((ss) => (
                <SeasonTab
                  key={ss.id}
                  season={ss}
                  active={ss.id === current?.id}
                  onClick={() => setActiveSeason(ss.season_number)}
                />
              ))}
            </div>
          )}

          {/* AniList strip for the active season */}
          {current && title.type !== 'movie' && title.is_anime && (
            <SeasonAniListStrip
              season={current}
              onEdit={() => setRematchSeasonID(current.id)}
            />
          )}

          {/* Episode list */}
          {current && (
            <div className={s.episodeList}>
              {[...(current.episodes ?? [])]
                .sort((a, b) => a.episode - b.episode)
                .map((ep) => (
                  <EpisodeRow key={ep.id} episode={ep} onToggle={handleEpisodeToggle} />
                ))}

              {/* Side stories for the active season */}
              {activeSeasonSideStories.length > 0 && (
                <SeasonSideStories
                  seasonNumber={current.season_number}
                  sideStories={activeSeasonSideStories}
                  onToggleWatched={handleToggleSideStoryWatched}
                />
              )}
            </div>
          )}

          {/* Cast & Crew card */}
          {credits && credits.length > 0 && (
            <div className={s.card}>
              <div className={s.cardLabel}>{t('details.castCrew')}</div>
              <div className={s.castList}>
                {credits.map((c) => (
                  <div key={`${c.name}-${c.role}`} className={s.castEntry}>
                    <button
                      type="button"
                      className={s.castPerson}
                      onClick={() => route(routeTo.person(c.name))}
                    >
                      {c.name}
                    </button>
                    <span className={s.castRole}>{c.role}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Franchise & Watch History Hub Bar */}
          <FranchiseRelationsSection
            relations={title.relations}
            onOpenHistory={() => setShowHistory(true)}
          />
        </div>
      </div>

      {/* Watch history bottom sheet drawer */}
      {showHistory && (
        <BottomSheet
          open={showHistory}
          onClose={() => setShowHistory(false)}
          ariaLabel={t('details.watchHistory')}
        >
          <TitleHistory titleId={title.id} onClose={() => setShowHistory(false)} />
        </BottomSheet>
      )}

      {/* Action drawer (mobile-only, hidden on desktop via CSS) */}
      <div className={s.mobileActionDrawerWrap}>
        <ActionDrawer
          title={title}
          onRate={() => setShowRating(true)}
          onEdit={() => setShowEdit(true)}
          onRematch={() => setShowRematch(true)}
          onMerge={() => route(`/search?mergeSourceId=${title.id}&mergeSourceName=${encodeURIComponent(name)}`)}
          onRefresh={handleRefresh}
          onDelete={() => setShowDeleteConfirm(true)}
          onOpenChange={setDrawerOpen}
        />
      </div>

      <ConfirmationDrawer
        open={showDeleteConfirm}
        onClose={() => setShowDeleteConfirm(false)}
        onConfirm={handleDelete}
        title={t('details.deleteConfirmTitle', { name })}
        description={t('details.deleteConfirmDesc')}
        confirmText={t('common.delete')}
        isDangerous
      />

      <ConfirmationDrawer
        open={showSonarrReaddConfirm}
        onClose={() => setShowSonarrReaddConfirm(false)}
        onConfirm={() => {
          setShowSonarrReaddConfirm(false)
          setShowArrPush(true)
        }}
        title={t('details.readdSonarrConfirmTitle')}
        description={t('details.readdSonarrConfirmDesc')}
        confirmText={t('details.readdSonarrConfirmAction')}
      />

      {/* Bottom sheets */}
      <ArrPushSheet
        open={showArrPush}
        onClose={() => setShowArrPush(false)}
        title={title}
        onSuccess={handleArrPushSuccess}
      />

      <RatingPrompt
        open={showRating}
        onClose={() => setShowRating(false)}
        titleName={name}
        initialRating={title.my_rating}
        hasImdb={!!title.imdb_id}
        onSave={handleSaveRating}
        onSaveAndImdb={(rating) => {
          handleSaveRating(rating)
          if (title.imdb_id) window.open(`https://www.imdb.com/title/${title.imdb_id}/`, '_blank', 'noopener,noreferrer')
        }}
      />

      <EditSheet
        open={showEdit}
        onClose={() => setShowEdit(false)}
        title={title}
        onSave={handleSaveEdit}
      />


      <RematchSheet
        open={showRematch || rematchSeasonID != null}
        onClose={() => { setShowRematch(false); setRematchSeasonID(null) }}
        title={title}
        seasonID={rematchSeasonID ?? undefined}
        onDone={() => { setRematchSeasonID(null); mutate() }}
      />
    </div>
    </PullToRefresh>
  )
}
