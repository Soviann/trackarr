import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent } from '@testing-library/preact'
import { ExternalLinks } from './ExternalLinks'

afterEach(() => cleanup())

describe('ExternalLinks', () => {
  it('renders nothing when no external IDs are provided', () => {
    const { container } = render(<ExternalLinks />)
    expect(container.firstChild).toBeNull()
  })

  it('renders TMDB movie link with strict attributes and stops click propagation', () => {
    const parentClick = vi.fn()
    const { getByText } = render(
      <div onClick={parentClick}>
        <ExternalLinks tmdbId={550} type="movie" />
      </div>
    )

    const link = getByText('TMDB ↗') as HTMLAnchorElement
    expect(link).toBeTruthy()
    expect(link.tagName).toBe('A')
    expect(link.href).toBe('https://www.themoviedb.org/movie/550')
    expect(link.target).toBe('_blank')
    expect(link.rel).toBe('noopener noreferrer')

    fireEvent.click(link)
    expect(parentClick).not.toHaveBeenCalled()
  })

  it('renders TMDB TV series link with correct route', () => {
    const { getByText } = render(<ExternalLinks tmdbId={1399} type="series" />)
    const link = getByText('TMDB ↗') as HTMLAnchorElement
    expect(link.href).toBe('https://www.themoviedb.org/tv/1399')
    expect(link.target).toBe('_blank')
    expect(link.rel).toBe('noopener noreferrer')
  })

  it('renders AniList link with correct URL and attributes', () => {
    const parentClick = vi.fn()
    const { getByText } = render(
      <div onClick={parentClick}>
        <ExternalLinks anilistId={21} />
      </div>
    )

    const link = getByText('AniList ↗') as HTMLAnchorElement
    expect(link).toBeTruthy()
    expect(link.href).toBe('https://anilist.co/anime/21')
    expect(link.target).toBe('_blank')
    expect(link.rel).toBe('noopener noreferrer')

    fireEvent.click(link)
    expect(parentClick).not.toHaveBeenCalled()
  })

  it('renders IMDb link with formatted tt prefix and stops propagation', () => {
    const parentClick = vi.fn()
    const { getByText } = render(
      <div onClick={parentClick}>
        <ExternalLinks imdbId="0137523" />
      </div>
    )

    const link = getByText('IMDb ↗') as HTMLAnchorElement
    expect(link).toBeTruthy()
    expect(link.href).toBe('https://www.imdb.com/title/tt0137523/')
    expect(link.target).toBe('_blank')
    expect(link.rel).toBe('noopener noreferrer')

    fireEvent.click(link)
    expect(parentClick).not.toHaveBeenCalled()
  })

  it('renders multiple badges together when multiple IDs are provided', () => {
    const { getByText } = render(
      <ExternalLinks tmdbId={550} imdbId="tt0137523" anilistId={21} type="movie" />
    )

    expect(getByText('TMDB ↗')).toBeTruthy()
    expect(getByText('AniList ↗')).toBeTruthy()
    expect(getByText('IMDb ↗')).toBeTruthy()
  })

  it('renders nothing when IDs are zero or empty/whitespace strings', () => {
    const { container } = render(
      <ExternalLinks tmdbId={0} anilistId={0} imdbId="   " />
    )
    expect(container.firstChild).toBeNull()
  })

  it('normalizes uppercase TT prefix in IMDb IDs without double-prefixing', () => {
    const { getByText } = render(<ExternalLinks imdbId="TT0137523" />)
    const link = getByText('IMDb ↗') as HTMLAnchorElement
    expect(link.href).toBe('https://www.imdb.com/title/tt0137523/')
  })
})
