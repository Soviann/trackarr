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

  it('activates explore tab for share and validate routes instead of admin', () => {
    const onNavigate = vi.fn()
    const { getByRole, rerender } = render(
      <Sidebar
        currentPath="/admin/validate?q=https%3A%2F%2Fwww.imdb.com%2Ftitle%2Ftt1234567"
        onNavigate={onNavigate}
        collapsed={false}
        onToggleCollapse={vi.fn()}
      />
    )

    const exploreBtn = getByRole('button', { name: 'Explore' })
    expect(exploreBtn.getAttribute('aria-current')).toBe('page')

    const adminBtn = getByRole('button', { name: 'Admin' })
    expect(adminBtn.getAttribute('aria-current')).toBeNull()

    rerender(
      <Sidebar
        currentPath="/add?url=https%3A%2F%2Fwww.imdb.com"
        onNavigate={onNavigate}
        collapsed={false}
        onToggleCollapse={vi.fn()}
      />
    )
    expect(exploreBtn.getAttribute('aria-current')).toBe('page')
    expect(adminBtn.getAttribute('aria-current')).toBeNull()
  })

  it('activates admin tab for /admin and /admin subroutes', () => {
    const onNavigate = vi.fn()
    const { getByRole, rerender } = render(
      <Sidebar
        currentPath="/admin"
        onNavigate={onNavigate}
        collapsed={false}
        onToggleCollapse={vi.fn()}
      />
    )
    expect(getByRole('button', { name: 'Admin' }).getAttribute('aria-current')).toBe('page')

    rerender(
      <Sidebar
        currentPath="/admin/tasks"
        onNavigate={onNavigate}
        collapsed={false}
        onToggleCollapse={vi.fn()}
      />
    )
    expect(getByRole('button', { name: 'Admin' }).getAttribute('aria-current')).toBe('page')
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
