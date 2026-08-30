import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/preact'
import { CalendarWeekTimeline } from './CalendarWeekTimeline'
import type { CalendarEvent } from '../types'

const mockEvents: CalendarEvent[] = [
  {
    id: 'ep-100',
    title_id: 10,
    title_name: 'Solo Leveling',
    type: 'series',
    is_anime: true,
    cover_url: '/covers/sololeveling.jpg',
    air_date: new Date().toISOString().slice(0, 10),
    season_number: 2,
    episode_number: 8,
    status: 'watching',
  },
]

describe('CalendarWeekTimeline', () => {
  afterEach(() => cleanup())

  it('renders week navigation and days', () => {
    const { getByText, getAllByText } = render(<CalendarWeekTimeline events={mockEvents} />)
    expect(getByText('Cette semaine')).toBeTruthy()
    expect(getAllByText('Solo Leveling').length).toBeGreaterThan(0)
  })
})
