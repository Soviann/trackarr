import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/preact'
import { LivePlaybackDrawer } from './LivePlaybackDrawer'
import * as api from '../api'
import type { ActivePlaybackSession } from '../types'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

describe('LivePlaybackDrawer', () => {
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
