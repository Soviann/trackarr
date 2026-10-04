import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { Sidebar } from './Sidebar'
import { ROUTE_PATHS } from '../routes'
import { setLocale } from '../i18n'

describe('Sidebar', () => {
  beforeEach(() => {
    setLocale('en')
  })

  afterEach(() => {
    cleanup()
  })
  it('renders all navigation tabs and allows navigating', () => {
    const onNavigate = vi.fn()
    const onToggle = vi.fn()
    const { getByRole } = render(
      <Sidebar
        currentPath={ROUTE_PATHS.home}
        onNavigate={onNavigate}
        collapsed={false}
        onToggleCollapse={onToggle}
      />
    )

    expect(getByRole('button', { name: 'Collection' })).toBeDefined()
    expect(getByRole('button', { name: 'Explore' })).toBeDefined()
    expect(getByRole('button', { name: 'Calendar' })).toBeDefined()
    expect(getByRole('button', { name: 'Stats' })).toBeDefined()
    expect(getByRole('button', { name: 'Admin' })).toBeDefined()

    fireEvent.click(getByRole('button', { name: 'Explore' }))
    expect(onNavigate).toHaveBeenCalledWith(ROUTE_PATHS.search)
  })

  it('marks active tab correctly based on currentPath', () => {
    const onNavigate = vi.fn()
    const onToggle = vi.fn()
    const { getByRole } = render(
      <Sidebar
        currentPath={ROUTE_PATHS.stats}
        onNavigate={onNavigate}
        collapsed={false}
        onToggleCollapse={onToggle}
      />
    )

    const statsBtn = getByRole('button', { name: 'Stats' })
    expect(statsBtn.getAttribute('aria-current')).toBe('page')

    const collectionBtn = getByRole('button', { name: 'Collection' })
    expect(collectionBtn.getAttribute('aria-current')).toBeNull()
  })

  it('handles collapsed mode and toggling', () => {
    const onNavigate = vi.fn()
    const onToggle = vi.fn()
    const { container, getByRole, rerender } = render(
      <Sidebar
        currentPath={ROUTE_PATHS.home}
        onNavigate={onNavigate}
        collapsed={true}
        onToggleCollapse={onToggle}
      />
    )

    const aside = container.querySelector('aside')
    expect(aside?.className).toContain('collapsed')

    const expandBtn = getByRole('button', { name: 'Expand sidebar' })
    fireEvent.click(expandBtn)
    expect(onToggle).toHaveBeenCalledTimes(1)

    rerender(
      <Sidebar
        currentPath={ROUTE_PATHS.home}
        onNavigate={onNavigate}
        collapsed={false}
        onToggleCollapse={onToggle}
      />
    )
    expect(aside?.className).not.toContain('collapsed')
  })
})
