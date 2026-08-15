import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/preact'
import { TypeBadge } from './TypeBadge'

afterEach(() => cleanup())

describe('TypeBadge', () => {
  it('renders movie type badge without Arr indicator by default', () => {
    const { getByLabelText } = render(<TypeBadge type="movie" />)
    const badge = getByLabelText('Movie')
    expect(badge).toBeTruthy()
    expect(badge.getAttribute('title')).toBeNull()
  })

  it('renders series type badge without Arr indicator by default', () => {
    const { getByLabelText } = render(<TypeBadge type="series" />)
    const badge = getByLabelText('Series')
    expect(badge).toBeTruthy()
    expect(badge.getAttribute('title')).toBeNull()
  })

  it('renders Radarr indicator when radarrId is provided for movie', () => {
    const { getByLabelText } = render(<TypeBadge type="movie" radarrId={123} />)
    const badge = getByLabelText('Movie (Présent sur Radarr)')
    expect(badge).toBeTruthy()
    expect(badge.getAttribute('title')).toBe('Présent sur Radarr')
  })

  it('renders Sonarr indicator when sonarrId is provided for series', () => {
    const { getByLabelText } = render(<TypeBadge type="series" sonarrId={456} />)
    const badge = getByLabelText('Series (Présent sur Sonarr)')
    expect(badge).toBeTruthy()
    expect(badge.getAttribute('title')).toBe('Présent sur Sonarr')
  })

  it('ignores sonarrId on movie type badge', () => {
    const { getByLabelText, queryByLabelText } = render(<TypeBadge type="movie" sonarrId={456} />)
    expect(getByLabelText('Movie')).toBeTruthy()
    expect(queryByLabelText('Movie (Présent sur Sonarr)')).toBeNull()
  })
})
