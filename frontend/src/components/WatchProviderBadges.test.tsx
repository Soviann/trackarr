import { describe, it, expect, afterEach, beforeEach } from 'vitest'
import { render, cleanup } from '@testing-library/preact'
import { WatchProviderBadges } from './WatchProviderBadges'
import { setEnabledWatchProviders } from '../utils/providers'

describe('WatchProviderBadges', () => {
  beforeEach(() => {
    localStorage.clear()
    setEnabledWatchProviders('netflix,prime,disney,apple,max,canal,crunchyroll,paramount,adn')
  })

  afterEach(() => cleanup())

  it('renders nothing when no providers given', () => {
    const { container } = render(<WatchProviderBadges providers={[]} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders single matching provider badge', () => {
    const { getByText } = render(
      <WatchProviderBadges providers={[{ id: 119, name: 'Amazon Prime Video' }]} />
    )
    expect(getByText('prime')).toBeTruthy()
  })

  it('renders multiple badges in matching order', () => {
    const { getByText } = render(
      <WatchProviderBadges
        providers={[
          { id: 8, name: 'Netflix' },
          { id: 337, name: 'Disney+' },
          { id: 283, name: 'Crunchyroll' },
        ]}
      />
    )
    expect(getByText('netflix')).toBeTruthy()
    expect(getByText('disney+')).toBeTruthy()
    expect(getByText('crunchyroll')).toBeTruthy()
  })

  it('respects disabled providers', () => {
    setEnabledWatchProviders('netflix,disney')
    const { queryByText } = render(
      <WatchProviderBadges
        providers={[
          { id: 8, name: 'Netflix' },
          { id: 119, name: 'Amazon Prime Video' },
        ]}
      />
    )
    expect(queryByText('netflix')).not.toBeNull()
    expect(queryByText('prime')).toBeNull()
  })
})
