import { useState, useCallback } from 'preact/hooks'
import { BottomSheet } from './BottomSheet'
import { CoverImage } from './CoverImage'
import { useTranslation } from '../i18n'
import { apiFetch } from '../api'
import { routeTo } from '../routes'
import type { ActivePlaybackSession, Title } from '../types'
import s from './LivePlaybackDrawer.module.css'

interface LivePlaybackDrawerProps {
  session: ActivePlaybackSession | null
  open: boolean
  onClose: () => void
  onDismiss: () => void
  onWatchedSuccess?: () => void
}

export function LivePlaybackDrawer({
  session,
  open,
  onClose,
  onDismiss,
  onWatchedSuccess,
}: LivePlaybackDrawerProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [marked, setMarked] = useState(false)

  const handleMarkWatched = useCallback(async () => {
    if (!session || loading || marked) return
    setLoading(true)
    try {
      if (session.title_type === 'movie') {
        await apiFetch(`/titles/${session.title_id}`, {
          method: 'PATCH',
          body: JSON.stringify({ status: 'completed' }),
        })
      } else {
        // Fetch full title to find the matching episode id
        const fullTitle = await apiFetch<Title>(`/titles/${session.title_id}`)
        let targetEpisodeId: number | null = null

        if (session.season_number != null && session.episode_number != null) {
          const ep = fullTitle.seasons
            ?.flatMap((season) => season.episodes ?? [])
            .find(
              (e) =>
                e.episode === session.episode_number &&
                fullTitle.seasons?.find((sn) => sn.id === e.season_id)?.season_number === session.season_number
            )
          if (ep) targetEpisodeId = ep.id
        }

        if (!targetEpisodeId) {
          const firstUnwatched = fullTitle.seasons
            ?.flatMap((season) => season.episodes ?? [])
            .find((e) => !e.watched)
          if (firstUnwatched) targetEpisodeId = firstUnwatched.id
        }

        if (targetEpisodeId) {
          await apiFetch(`/titles/${session.title_id}/episodes/${targetEpisodeId}`, {
            method: 'PATCH',
          })
        }
      }

      setMarked(true)
      onWatchedSuccess?.()
      setTimeout(() => {
        onClose()
      }, 1000)
    } catch {
      // Keep UI usable on network error
    } finally {
      setLoading(false)
    }
  }, [session, loading, marked, onClose, onWatchedSuccess])

  if (!session) return null

  const isSeries = session.title_type === 'series'
  const seasonNum = session.season_number
  const epNum = session.episode_number
  const seasonEpText =
    isSeries && seasonNum != null && epNum != null
      ? t('playback.seasonEpisode', {
          season: seasonNum,
          episode: epNum,
        })
      : null

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel={t('playback.nowPlaying')}>
      <div className={s.content}>
        <div className={s.header}>
          <CoverImage
            coverUrl={session.cover_url}
            type={session.title_type}
            alt={session.title_name}
            className={s.cover}
          />
          <div className={s.meta}>
            <span className={s.liveBadge}>{session.source} • LIVE</span>
            <a href={routeTo.title(session.title_id)} className={s.titleLink} onClick={onClose}>
              {session.title_name} ({session.year})
            </a>
            {seasonEpText && (
              <span className={s.subTitle}>
                {seasonEpText}
                {session.episode_name ? ` · ${session.episode_name}` : ''}
              </span>
            )}
            {session.player && (
              <span className={s.deviceInfo}>
                {t('playback.nowPlayingOn', { player: session.player })}
              </span>
            )}
          </div>
        </div>

        {session.overview && <p className={s.overview}>{session.overview}</p>}

        <div className={s.actions}>
          <button
            type="button"
            className={`${s.markWatchedBtn} ${marked ? s.successBtn : ''}`}
            onClick={handleMarkWatched}
            disabled={loading || marked}
            style={session.accent_hex && !marked ? { background: session.accent_hex } : undefined}
          >
            {marked
              ? `✓ ${t('playback.markedWatched')}`
              : loading
              ? t('playback.markingWatched')
              : `+1 ${t('playback.markWatched')}`}
          </button>
          <button
            type="button"
            className={s.dismissBtn}
            onClick={() => {
              onDismiss()
              onClose()
            }}
          >
            {t('playback.dismiss')}
          </button>
        </div>
      </div>
    </BottomSheet>
  )
}
