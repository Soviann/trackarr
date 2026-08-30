import { useState, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import { routeTo } from '../routes'
import type { CalendarEvent } from '../types'
import { useTranslation } from '../i18n'
import { WatchProviderBadges } from './WatchProviderBadges'
import s from './CalendarMonthGrid.module.css'

interface Props {
  events: CalendarEvent[]
}

const WEEKDAY_INDEXES = [1, 2, 3, 4, 5, 6, 0] // Mon=1 ... Sun=0

function formatYMD(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function CalendarMonthGrid({ events }: Props) {
  const { t, locale } = useTranslation()
  const activeLocaleTag = locale === 'fr' ? 'fr-FR' : 'en-US'

  const weekdayNames = useMemo(() => {
    // Generate Mon-Sun short names dynamically from locale
    return WEEKDAY_INDEXES.map((dayIdx) => {
      // 2026-08-31 is a Monday
      const sample = new Date(2026, 7, 24 + (dayIdx === 0 ? 6 : dayIdx - 1))
      return sample.toLocaleDateString(activeLocaleTag, { weekday: 'short' })
    })
  }, [activeLocaleTag])

  const today = useMemo(() => new Date(), [])
  const todayStr = useMemo(() => formatYMD(today), [today])

  const [currentDate, setCurrentDate] = useState<Date>(() => {
    const d = new Date()
    d.setDate(1)
    return d
  })

  const [selectedDateStr, setSelectedDateStr] = useState<string>(todayStr)

  const eventsByDate = useMemo(() => {
    const map = new Map<string, CalendarEvent[]>()
    for (const ev of events) {
      const list = map.get(ev.air_date) || []
      list.push(ev)
      map.set(ev.air_date, list)
    }
    return map
  }, [events])

  const year = currentDate.getFullYear()
  const month = currentDate.getMonth()

  // Generate calendar days grid (padded to full weeks, Mon-Sun)
  const calendarCells = useMemo(() => {
    const firstDayOfMonth = new Date(year, month, 1)
    const lastDayOfMonth = new Date(year, month + 1, 0)

    // Day of week: 0 = Sun, 1 = Mon, ..., 6 = Sat -> transform to Mon=0 ... Sun=6
    let startDayOfWeek = firstDayOfMonth.getDay() - 1
    if (startDayOfWeek < 0) startDayOfWeek = 6

    const cells: { date: Date; dateStr: string; isCurrentMonth: boolean }[] = []

    // Previous month padding
    for (let i = startDayOfWeek; i > 0; i--) {
      const d = new Date(year, month, 1 - i)
      cells.push({ date: d, dateStr: formatYMD(d), isCurrentMonth: false })
    }

    // Current month days
    for (let day = 1; day <= lastDayOfMonth.getDate(); day++) {
      const d = new Date(year, month, day)
      cells.push({ date: d, dateStr: formatYMD(d), isCurrentMonth: true })
    }

    // Next month padding to complete 7-day row
    const remainder = cells.length % 7
    if (remainder > 0) {
      const needed = 7 - remainder
      for (let i = 1; i <= needed; i++) {
        const d = new Date(year, month + 1, i)
        cells.push({ date: d, dateStr: formatYMD(d), isCurrentMonth: false })
      }
    }

    return cells
  }, [year, month])

  const prevMonth = () => {
    setCurrentDate(new Date(year, month - 1, 1))
  }

  const nextMonth = () => {
    setCurrentDate(new Date(year, month + 1, 1))
  }

  const goToToday = () => {
    const d = new Date()
    d.setDate(1)
    setCurrentDate(d)
    setSelectedDateStr(todayStr)
  }

  const monthLabel = currentDate.toLocaleDateString(activeLocaleTag, {
    month: 'long',
    year: 'numeric',
  })

  const selectedEvents = eventsByDate.get(selectedDateStr) || []

  return (
    <div className={s.container}>
      {/* Month Header Navigation */}
      <div className={s.monthNav}>
        <div className={s.navTitle}>
          <span>{monthLabel}</span>
        </div>
        <div className={s.navControls}>
          <button onClick={goToToday} className={s.todayBtn}>
            {t('calendar.today')}
          </button>
          <button onClick={prevMonth} aria-label={t('calendar.prevMonth')} className={s.navBtn}>
            ‹
          </button>
          <button onClick={nextMonth} aria-label={t('calendar.nextMonth')} className={s.navBtn}>
            ›
          </button>
        </div>
      </div>

      {/* Grid */}
      <div className={s.gridWrapper}>
        <div className={s.weekDaysHeader}>
          {weekdayNames.map((name) => (
            <div key={name} className={s.weekDayName}>
              {name}
            </div>
          ))}
        </div>

        <div className={s.calendarGrid}>
          {calendarCells.map((cell) => {
            const dayEvents = eventsByDate.get(cell.dateStr) || []
            const isToday = cell.dateStr === todayStr
            const isSelected = cell.dateStr === selectedDateStr

            return (
              <div
                key={cell.dateStr}
                className={`${s.dayCell}${!cell.isCurrentMonth ? ` ${s.otherMonth}` : ''}${isToday ? ` ${s.todayCell}` : ''}${isSelected ? ` ${s.selectedCell}` : ''}`}
                onClick={() => setSelectedDateStr(cell.dateStr)}
              >
                <div className={s.dayHeader}>
                  <span className={`${s.dayNumber}${isToday ? ` ${s.todayNumber}` : ''}`}>
                    {cell.date.getDate()}
                  </span>
                  {dayEvents.length > 0 && (
                    <span className={s.eventCountBadge}>{dayEvents.length}</span>
                  )}
                </div>

                {/* Mobile indicators */}
                {dayEvents.length > 0 && (
                  <div className={s.mobileDots}>
                    {dayEvents.slice(0, 3).map((ev) => (
                      <span
                        key={ev.id}
                        className={`${s.dot}${ev.is_anime ? ` ${s.dotAnime}` : ev.type === 'movie' ? ` ${s.dotMovie}` : ` ${s.dotSeries}`}`}
                      />
                    ))}
                  </div>
                )}

                {/* Desktop event pills */}
                <div className={s.eventsList}>
                  {dayEvents.slice(0, 2).map((ev) => {
                    const epTag =
                      ev.season_number != null && ev.episode_number != null
                        ? `S${String(ev.season_number).padStart(2, '0')}E${String(ev.episode_number).padStart(2, '0')}`
                        : ev.next_air_episode || ''

                    return (
                      <div
                        key={ev.id}
                        className={`${s.eventPill}${ev.is_anime ? ` ${s.pillAnime}` : ev.type === 'movie' ? ` ${s.pillMovie}` : ` ${s.pillSeries}`}`}
                        onClick={(e) => {
                          e.stopPropagation()
                          route(routeTo.title(ev.title_id))
                        }}
                        title={`${ev.title_name}${epTag ? ` (${epTag})` : ''}`}
                      >
                        {ev.cover_url && (
                          <img
                            src={ev.cover_url}
                            alt=""
                            className={s.pillCover}
                            loading="lazy"
                          />
                        )}
                        <span className={s.pillTitle}>{ev.title_name}</span>
                        {epTag && <span className={s.pillEp}>{epTag}</span>}
                      </div>
                    )
                  })}
                  {dayEvents.length > 2 && (
                    <span className={s.moreEventsNote}>
                      {t('calendar.moreCount', { count: dayEvents.length - 2 })}
                    </span>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Selected Day Details Section */}
      {selectedEvents.length > 0 && (
        <div className={s.selectedDaySection}>
          <div className={s.selectedDayHeader}>
            <span className={s.selectedDayTitle}>
              {t('calendar.releasesFor')}{' '}
              {new Date(selectedDateStr + 'T00:00:00').toLocaleDateString(activeLocaleTag, {
                weekday: 'long',
                day: 'numeric',
                month: 'long',
              })}
            </span>
          </div>

          <div className={s.selectedCardsGrid}>
            {selectedEvents.map((ev) => {
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
                  className={s.releaseCard}
                >
                  {ev.cover_url ? (
                    <img
                      src={ev.cover_url}
                      alt={ev.title_name}
                      className={s.cardPoster}
                      loading="lazy"
                    />
                  ) : (
                    <div className={s.cardPoster} />
                  )}
                  <div className={s.cardDetails}>
                    <div className={s.cardTitle}>{ev.title_name}</div>
                    {epTag && <div className={s.cardEp}>{epTag}</div>}
                    {ev.episode_name && (
                      <div className={s.cardEpName}>{ev.episode_name}</div>
                    )}
                    {ev.watch_providers && ev.watch_providers.length > 0 && (
                      <WatchProviderBadges providers={ev.watch_providers} />
                    )}
                  </div>
                </a>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
