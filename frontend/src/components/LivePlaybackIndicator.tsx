import { useTranslation } from '../i18n'
import type { ActivePlaybackSession } from '../types'
import s from './LivePlaybackIndicator.module.css'

interface LivePlaybackIndicatorProps {
  session: ActivePlaybackSession
  onClick: () => void
}

export function LivePlaybackIndicator({ session, onClick }: LivePlaybackIndicatorProps) {
  const { t } = useTranslation()

  let subText = session.player ? t('playback.nowPlayingOn', { player: session.player }) : t('playback.nowPlaying')
  if (session.title_type === 'series' && session.season_number != null && session.episode_number != null) {
    const se = t('playback.seasonEpisode', {
      season: session.season_number,
      episode: session.episode_number,
    })
    subText = `${se}${session.episode_name ? ` · ${session.episode_name}` : ''}`
  }

  return (
    <div
      className={s.indicator}
      onClick={onClick}
      role="button"
      tabIndex={0}
      aria-label={t('playback.openCompanion')}
    >
      <div className={s.left}>
        <span
          className={s.pulseDot}
          style={session.accent_hex ? { background: session.accent_hex, boxShadow: `0 0 8px ${session.accent_hex}99` } : undefined}
          aria-hidden="true"
        />
        <div className={s.info}>
          <span className={s.title}>{session.title_name}</span>
          <span className={s.sub}>{subText}</span>
        </div>
      </div>
      <span className={s.sourceBadge}>{session.source}</span>
    </div>
  )
}
