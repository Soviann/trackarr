import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { ArrPushSheet } from './ArrPushSheet'
import { apiFetch } from '../api'
import type { Title } from '../types'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

const baseTitle: Title = {
  id: 42,
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
  tvdb_id: 12345,
  total_watch_minutes: 0,
  my_rating: null,
  status: 'dropped',
  series_status: 'ended',
  match_status: 'confirmed',
  original_title: 'Dropped Series',
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
  names: [{ id: 1, title_id: 42, name: 'Dropped Series', language: 'en', is_primary: true }],
  seasons: [],
}

describe('ArrPushSheet', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path === '/admin/arr') {
        return {
          sonarr_std_root_folder: '/tv',
          sonarr_std_quality_profile: '1',
        }
      }
      if (path.includes('/rootfolder')) {
        return [{ id: 1, path: '/tv' }]
      }
      if (path.includes('/qualityprofile')) {
        return [{ id: 1, name: 'HD - 1080p' }]
      }
      if (path.includes('/arr/title/')) {
        return { exists: false }
      }
      return {}
    })
  })

  afterEach(() => {
    cleanup()
  })

  it('renders "Send to Sonarr" when series was never deleted from Sonarr', async () => {
    render(<ArrPushSheet open={true} onClose={vi.fn()} title={baseTitle} />)

    await waitFor(() => {
      expect(screen.getByText('Send to Sonarr')).toBeTruthy()
    })
    expect(screen.queryByText(/previously deleted and excluded/i)).toBeNull()
  })

  it('renders notice and "Re-add to Sonarr" when series has sonarr_deleted_at', async () => {
    const deletedTitle: Title = {
      ...baseTitle,
      sonarr_deleted_at: '2026-09-25T10:00:00Z',
    }

    render(<ArrPushSheet open={true} onClose={vi.fn()} title={deletedTitle} />)

    await waitFor(() => {
      expect(screen.getByText('Re-add to Sonarr')).toBeTruthy()
    })
    expect(screen.getByText(/previously deleted and excluded from Sonarr/i)).toBeTruthy()
  })

  it('renders "Send to Radarr" for movies even if sonarr_deleted_at is set', async () => {
    const movieTitle: Title = {
      ...baseTitle,
      type: 'movie',
      tmdb_id: 999,
      sonarr_deleted_at: '2026-09-25T10:00:00Z',
    }

    render(<ArrPushSheet open={true} onClose={vi.fn()} title={movieTitle} />)

    await waitFor(() => {
      expect(screen.getByText('Send to Radarr')).toBeTruthy()
    })
    expect(screen.queryByText(/previously deleted and excluded/i)).toBeNull()
  })

  it('submits push request and invokes onSuccess when Re-add is clicked', async () => {
    const deletedTitle: Title = {
      ...baseTitle,
      sonarr_deleted_at: '2026-09-25T10:00:00Z',
    }
    const onSuccess = vi.fn()
    const onClose = vi.fn()

    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path === '/admin/arr') return { sonarr_std_root_folder: '/tv', sonarr_std_quality_profile: '1' }
      if (path.includes('/rootfolder')) return [{ id: 1, path: '/tv' }]
      if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD - 1080p' }]
      if (path.includes('/arr/title/')) return { exists: false }
      if (path === '/arr/push/42') return { status: 'created', arr_id: 99 }
      return {}
    })

    render(<ArrPushSheet open={true} onClose={onClose} title={deletedTitle} onSuccess={onSuccess} />)

    await waitFor(() => {
      expect(screen.getByText('Re-add to Sonarr')).toBeTruthy()
    })

    fireEvent.click(screen.getByText('Re-add to Sonarr'))

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/arr/push/42', expect.objectContaining({
        method: 'POST',
      }))
      expect(onSuccess).toHaveBeenCalledWith(99)
      expect(onClose).toHaveBeenCalledOnce()
    })
  })
})
