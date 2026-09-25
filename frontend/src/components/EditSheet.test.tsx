import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { EditSheet } from './EditSheet'
import type { Title } from '../types'

afterEach(() => cleanup())

const mockTitle: Title = {
  id: 1,
  type: 'series',
  is_anime: false,
  year: 2024,
  cover_url: null,
  accent_hex: null,
  imdb_id: null,
  simkl_id: null,
  simkl_slug: null,
  anilist_id: null,
  tmdb_id: null,
  tvdb_id: null,
  total_watch_minutes: 0,
  my_rating: null,
  status: 'watching',
  series_status: 'returning',
  match_status: 'confirmed',
  original_title: 'Test Title',
  match_source: null,
  overview: null,
  genres: null,
  runtime: null,
  tmdb_rating: null,
  credits: null,
  anilist_rating: null,
  release_date: null,
  created_at: '',
  updated_at: '',
  names: [],
  seasons: [],
}

describe('EditSheet', () => {
  it('renders correctly', () => {
    const { getByText } = render(
      <EditSheet open={true} onClose={vi.fn()} title={mockTitle} onSave={vi.fn()} />
    )
    expect(getByText('Type')).toBeTruthy()
    expect(getByText('Status')).toBeTruthy()
    expect(getByText('Save')).toBeTruthy()
    expect(getByText('Cancel')).toBeTruthy()
  })

  it('calls onClose when Cancel is clicked', () => {
    const onClose = vi.fn()
    const { getByText } = render(
      <EditSheet open={true} onClose={onClose} title={mockTitle} onSave={vi.fn()} />
    )
    fireEvent.click(getByText('Cancel'))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('calls onSave when Save is clicked', () => {
    const onSave = vi.fn()
    const { getByText } = render(
      <EditSheet open={true} onClose={vi.fn()} title={mockTitle} onSave={onSave} />
    )
    fireEvent.click(getByText('Save'))
    expect(onSave).toHaveBeenCalledOnce()
  })

  it('displays Sonarr checkbox checked by default when transitioning series to dropped', () => {
    const onSave = vi.fn()
    const { getByText, getByRole } = render(
      <EditSheet open={true} onClose={vi.fn()} title={mockTitle} onSave={onSave} />
    )
    // Initially watching -> no checkbox
    expect(() => getByRole('checkbox')).toThrow()

    // Click Dropped status
    fireEvent.click(getByText('Dropped'))

    // Checkbox appears and is checked
    const checkbox = getByRole('checkbox') as HTMLInputElement
    expect(checkbox).toBeTruthy()
    expect(checkbox.checked).toBe(true)

    // Save
    fireEvent.click(getByText('Save'))
    expect(onSave).toHaveBeenCalledWith({
      status: 'dropped',
      delete_from_sonarr: true,
    })
  })

  it('displays Sonarr checkbox unchecked by default for already dropped series', () => {
    const onSave = vi.fn()
    const droppedTitle: Title = { ...mockTitle, status: 'dropped' }
    const { getByRole, getByText } = render(
      <EditSheet open={true} onClose={vi.fn()} title={droppedTitle} onSave={onSave} />
    )

    // Checkbox appears immediately and is unchecked
    const checkbox = getByRole('checkbox') as HTMLInputElement
    expect(checkbox).toBeTruthy()
    expect(checkbox.checked).toBe(false)

    // Check it and save
    fireEvent.click(checkbox)
    expect(checkbox.checked).toBe(true)

    fireEvent.click(getByText('Save'))
    expect(onSave).toHaveBeenCalledWith({
      delete_from_sonarr: true,
    })
  })

  it('does not display Sonarr checkbox for movies even when dropped', () => {
    const movieTitle: Title = { ...mockTitle, type: 'movie', status: 'dropped' }
    const { queryByRole } = render(
      <EditSheet open={true} onClose={vi.fn()} title={movieTitle} onSave={vi.fn()} />
    )
    expect(queryByRole('checkbox')).toBeNull()
  })
})
