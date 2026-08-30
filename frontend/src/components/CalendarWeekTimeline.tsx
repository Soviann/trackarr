import { useState, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import { routeTo } from '../routes'
import type { CalendarEvent } from '../types'
import { useTranslation } from '../i18n'
import { WatchProviderBadges } from './WatchProviderBadges'
import s from './CalendarWeekTimeline.module.css'

interface Props {
  events: CalendarEvent[]
}

function formatYMD(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function getMonday(d: Date): Date {
  const date = new Date(d)
  const day = date.getDay()
  const diff = date.getDate() - day + (day === 0 ? -6 : 1) // adjust when day is sunday
  date.setDate(diff)
  date.setHours(0, 0, 0, 0)
  return date
}

export function CalendarWeekTimeline({ events }: Props) {
  const { t, locale } = useTranslation()
  const activeLocaleTag = locale === 'fr' ? 'fr-FR' : 'en-US'

  const today = useMemo(() => new Date(), [])
  const todayStr = useMemo(() => formatYMD(today), [today])

  const [weekStart, setWeekStart] = useState<Date>(() => getMonday(new Date()))

  const eventsByDate = useMemo(() => {
    const map = new Map<string, CalendarEvent[]>()
    for (const ev of events) {
      const list = map.get(ev.air_date) || []
      list.push(ev)
      map.set(ev.air_date, list)
    }
    return map
  }, [events])

  const weekDays = useMemo(() => {
    const days: { date: Date; dateStr: string }[] = []
    for (let i = 0; i < 7; i++) {
      const d = new Date(weekStart)
      d.setDate(d.getDate() + i)
      days.push({ date: d, dateStr: formatYMD(d) })
    }
    return days
  }, [weekStart])

  const prevWeek = () => {
    const d = new Date(weekStart)
    d.setDate(d.getDate() - 7)
    setWeekStart(d)
  }

  const nextWeek = () => {
    const d = new Date(weekStart)
    d.setDate(d.getDate() + 7)
    setWeekStart(d)
  }

  const goToToday = () => {
    setWeekStart(getMonday(new Date()))
  }

  const weekEnd = weekDays[6].date
  const weekLabel = `${weekDays[0].date.toLocaleDateString(activeLocaleTag, { day: 'numeric', month: 'short' })} – ${weekEnd.toLocaleDateString(activeLocaleTag, { day: 'numeric', month: 'short', year: 'numeric' })}`

  return (
    <div className={s.container}>
      {/* Week Header Navigation */}
      <div className={s.weekNav}>
        <div className={s.navTitle}>
          <span>{weekLabel}</span>
        </div>
        <div className={s.navControls}>
          <button onClick={goToToday} className={s.todayBtn}>
            {t('calendar.thisWeek')}
          </button>
          <button onClick={prevWeek} aria-label={t('calendar.prevWeek')} className={s.navBtn}>
            ‹
          </button>
          <button onClick={nextWeek} aria-label={t('calendar.nextWeek')} className={s.navBtn}>
            ›
          </button>
        </div>
      </div>

      {/* 7 Days Columns */}
      <div className={s.daysGrid}>
        {weekDays.map(({ date, dateStr }) => {
          const dayEvents = eventsByDate.get(dateStr) || []
          const isToday = dateStr === todayStr

          return (
            <div
              key={dateStr}
              className={`${s.dayColumn}${isToday ? ` ${s.todayColumn}` : ''}`}
            >
              <div className={s.dayHeader}>
                <div className={s.dayDate}>
                  <span className={s.dayWeekday}>
                    {date.toLocaleDateString(activeLocaleTag, { weekday: 'long' })}
                  </span>
                  <span className={s.dayNum}>
                    {date.toLocaleDateString(activeLocaleTag, { day: 'numeric', month: 'short' })}
                  </span>
                </div>
                {isToday && <span className={s.todayBadge}>{t('calendar.today')}</span>}
              </div>

              <div className={s.dayEvents}>
                {dayEvents.length === 0 ? (
                  <div className={s.emptyDay}>{t('calendar.emptyDay')}</div>
                ) : (
                  dayEvents.map((ev) => {
                    const epTag =
                      ev.season_number != null && ev.episode_number != null
                        ? `S${String(ev.season_number).padStart(2, '0')}E${String(ev.episode_number).padStart(2, '0')}`
                        : ev.next_air_episode || null

                    return (
                      <a
                        key={ev.id}
                        href={routeTo.title(ev.title_id)}
                        onClick={(e) => {
                          e.preventDefault()
                          route(routeTo.title(ev.title_id))
                        }}
                        className={s.eventCard}
                      >
                        {ev.cover_url ? (
                          <img
                            src={ev.cover_url}
                            alt={ev.title_name}
                            className={s.poster}
                            loading="lazy"
                          />
                        ) : (
                          <div className={s.poster} />
                        )}
                        <div className={s.cardBody}>
                          <div className={s.titleName}>{ev.title_name}</div>
                          <div className={s.epBadgeRow}>
                            {epTag && <span className={s.epBadge}>{epTag}</span>}
                            <span className={s.typeBadge}>
                              {ev.is_anime ? 'Anime' : ev.type === 'movie' ? t('franchise.movies') : t('franchise.series')}
                            </span>
                          </div>
                          {ev.episode_name && (
                            <div className={s.epName}>{ev.episode_name}</div>
                          )}
                          {ev.watch_providers && ev.watch_providers.length > 0 && (
                            <WatchProviderBadges providers={ev.watch_providers} />
                          )}
                        </div>
                      </a>
                    )
                  })
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
