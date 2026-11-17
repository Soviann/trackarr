import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { LivePlaybackIndicator } from './LivePlaybackIndicator'
import type { ActivePlaybackSession } from '../types'

describe('LivePlaybackIndicator', () => {
  afterEach(() => {
    cleanup()
  })
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
    expect(screen.getByText('Now Playing')).not.toBeNull()
    expect(screen.queryByText(/S\d+E\d+/)).toBeNull()
  })

  it('renders movie session with player name and without episode numbers', () => {
    const movieSessionWithPlayer: ActivePlaybackSession = {
      source: 'plex',
      player: 'Apple TV',
      title_id: 203,
      title_name: 'Interstellar',
      title_type: 'movie',
      year: 2014,
      season_number: 0,
      episode_number: 0,
      started_at: '2026-10-24T12:00:00Z',
      updated_at: '2026-10-24T12:00:00Z',
    }

    render(<LivePlaybackIndicator session={movieSessionWithPlayer} onClick={vi.fn()} />)
    expect(screen.getByText('Interstellar')).not.toBeNull()
    expect(screen.getByText('Now playing on Apple TV')).not.toBeNull()
    expect(screen.queryByText(/S\d+E\d+/)).toBeNull()
  })
})
