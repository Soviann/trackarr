import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/preact'
import { CalendarMonthGrid } from './CalendarMonthGrid'
import type { CalendarEvent } from '../types'

const mockEvents: CalendarEvent[] = [
  {
    id: 'ep-1',
    title_id: 10,
    title_name: 'Frieren',
    type: 'series',
    is_anime: true,
    cover_url: '/covers/frieren.jpg',
    air_date: '2026-08-15',
    season_number: 1,
    episode_number: 5,
    status: 'watching',
  },
  {
    id: 'title-20-2026-08-20',
    title_id: 20,
    title_name: 'Dune 3',
    type: 'movie',
    is_anime: false,
    cover_url: '/covers/dune.jpg',
    air_date: '2026-08-20',
    status: 'plan_to_watch',
  },
]

describe('CalendarMonthGrid', () => {
  afterEach(() => cleanup())

  it('renders month navigation and weekday headers', () => {
    const { getByText } = render(<CalendarMonthGrid events={mockEvents} />)
    expect(getByText('Mon')).toBeTruthy()
    expect(getByText('Sun')).toBeTruthy()
    expect(getByText('Today')).toBeTruthy()
  })

  it('renders event pills on matching dates', () => {
    const { getByText } = render(<CalendarMonthGrid events={mockEvents} />)
    expect(getByText('Frieren')).toBeTruthy()
    expect(getByText('Dune 3')).toBeTruthy()
  })
})
