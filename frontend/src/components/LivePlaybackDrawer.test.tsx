import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/preact'
import { LivePlaybackDrawer } from './LivePlaybackDrawer'
import * as api from '../api'
import type { ActivePlaybackSession } from '../types'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

describe('LivePlaybackDrawer', () => {
  afterEach(() => {
    cleanup()
  })
  const mockSession: ActivePlaybackSession = {
    source: 'plex',
    player: 'Living Room TV',
    title_id: 101,
    title_name: 'Breaking Bad',
    title_type: 'series',
    year: 2008,
    season_number: 1,
    episode_number: 2,
    overview: 'Walt and Jesse attempt to dispose of two bodies.',
    started_at: '2026-10-24T12:00:00Z',
    updated_at: '2026-10-24T12:00:00Z',
  }

  it('renders active playback session details when open', () => {
    render(
      <LivePlaybackDrawer
        session={mockSession}
        open={true}
        onClose={vi.fn()}
        onDismiss={vi.fn()}
      />
    )

    expect(screen.getByText(/Breaking Bad/)).not.toBeNull()
    expect(screen.getByText(/Walt and Jesse/)).not.toBeNull()
    expect(screen.getByText(/S1E2/)).not.toBeNull()
    expect(screen.getByText(/Mark Watched/)).not.toBeNull()
  })

  it('triggers mark watched on button click', async () => {
    const apiFetchMock = vi.mocked(api.apiFetch)
    apiFetchMock.mockResolvedValueOnce({
      id: 101,
      seasons: [
        {
          id: 1,
          season_number: 1,
          episodes: [{ id: 501, season_id: 1, episode: 2, watched: false }],
        },
      ],
    })
    apiFetchMock.mockResolvedValueOnce({})

    const onWatchedSuccess = vi.fn()
    const { container } = render(
      <LivePlaybackDrawer
        session={mockSession}
        open={true}
        onClose={vi.fn()}
        onDismiss={vi.fn()}
        onWatchedSuccess={onWatchedSuccess}
      />
    )

    const btn = container.querySelector('button[class*="markWatchedBtn"]') as HTMLButtonElement
    expect(btn).not.toBeNull()
    fireEvent.click(btn)

    await waitFor(() => {
      expect(apiFetchMock).toHaveBeenCalledWith('/titles/101')
      expect(apiFetchMock).toHaveBeenCalledWith('/titles/101/episodes/501', { method: 'PATCH' })
      expect(onWatchedSuccess).toHaveBeenCalledOnce()
    })
  })

  it('renders movie session without episode details or +1 prefix and marks movie completed', async () => {
    const apiFetchMock = vi.mocked(api.apiFetch)
    apiFetchMock.mockResolvedValueOnce({})

    const movieSession: ActivePlaybackSession = {
      source: 'jellyfin',
      player: 'Apple TV',
      title_id: 202,
      title_name: 'Inception',
      title_type: 'movie',
      year: 2010,
      overview: 'A thief who steals corporate secrets through dream-sharing technology.',
      started_at: '2026-10-24T12:00:00Z',
      updated_at: '2026-10-24T12:00:00Z',
    }

    const onWatchedSuccess = vi.fn()
    const { container } = render(
      <LivePlaybackDrawer
        session={movieSession}
        open={true}
        onClose={vi.fn()}
        onDismiss={vi.fn()}
        onWatchedSuccess={onWatchedSuccess}
      />
    )

    expect(screen.getByText('Inception (2010)')).not.toBeNull()
    expect(screen.queryByText(/S\d+E\d+/)).toBeNull()

    const btn = container.querySelector('button[class*="markWatchedBtn"]') as HTMLButtonElement
    expect(btn).not.toBeNull()
    expect(btn.textContent).toBe('Mark Watched')
    expect(btn.textContent).not.toContain('+1')

    fireEvent.click(btn)

    await waitFor(() => {
      expect(apiFetchMock).toHaveBeenCalledWith('/titles/202', {
        method: 'PATCH',
        body: JSON.stringify({ status: 'completed' }),
      })
      expect(onWatchedSuccess).toHaveBeenCalledOnce()
    })
  })

  it('returns null when session is null', () => {
    const { container } = render(
      <LivePlaybackDrawer
        session={null}
        open={true}
        onClose={vi.fn()}
        onDismiss={vi.fn()}
      />
    )
    expect(container.firstChild).toBeNull()
  })
})
