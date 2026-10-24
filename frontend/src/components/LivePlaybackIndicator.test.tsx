import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { LivePlaybackIndicator } from './LivePlaybackIndicator'
import type { ActivePlaybackSession } from '../types'

describe('LivePlaybackIndicator', () => {
  const nominalSeriesSession: ActivePlaybackSession = {
    source: 'plex',
    player: 'Living Room TV',
    title_id: 101,
    title_name: 'Breaking Bad',
    title_type: 'series',
    year: 2008,
    season_number: 2,
    episode_number: 5,
    episode_name: 'Breakage',
    started_at: '2026-10-24T12:00:00Z',
    updated_at: '2026-10-24T12:00:00Z',
  }

  it('renders nominal series playback information', () => {
    const onClick = vi.fn()
    render(<LivePlaybackIndicator session={nominalSeriesSession} onClick={onClick} />)

    expect(screen.getByText('Breaking Bad')).not.toBeNull()
    expect(screen.getByText('plex')).not.toBeNull()
    expect(screen.getByText(/S2E5/)).not.toBeNull()

    fireEvent.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('renders boundary movie session without episode numbers', () => {
    const movieSession: ActivePlaybackSession = {
      source: 'jellyfin',
      title_id: 202,
      title_name: 'Inception',
      title_type: 'movie',
      year: 2010,
      started_at: '2026-10-24T12:00:00Z',
      updated_at: '2026-10-24T12:00:00Z',
    }

    render(<LivePlaybackIndicator session={movieSession} onClick={vi.fn()} />)
    expect(screen.getByText('Inception')).not.toBeNull()
    expect(screen.getByText('jellyfin')).not.toBeNull()
  })
})
