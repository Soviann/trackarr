import { describe, it, expect } from 'vitest'
import { getActiveTab } from './navItems'
import { ROUTE_PATHS } from '../routes'

describe('getActiveTab', () => {
  const cases: Array<{ path: string; expected: string; label: string }> = [
    { path: '/', expected: ROUTE_PATHS.home, label: 'home / collection' },
    { path: '/continue-watching', expected: ROUTE_PATHS.home, label: 'continue-watching maps to home' },
    { path: '/search', expected: ROUTE_PATHS.search, label: 'search / explore' },
    { path: '/search?q=batman', expected: ROUTE_PATHS.search, label: 'search with query param' },
    { path: '/add', expected: ROUTE_PATHS.search, label: 'add route maps to explore' },
    { path: '/add?url=https%3A%2F%2Fwww.imdb.com', expected: ROUTE_PATHS.search, label: 'add from share target maps to explore' },
    { path: '/admin/validate', expected: ROUTE_PATHS.search, label: 'adminValidate maps to explore' },
    { path: '/admin/validate?q=https%3A%2F%2Fwww.imdb.com%2Ftitle%2Ftt1234567', expected: ROUTE_PATHS.search, label: 'adminValidate with share url maps to explore' },
    { path: '/coming-up', expected: ROUTE_PATHS.comingUp, label: 'coming-up / calendar' },
    { path: '/releases', expected: ROUTE_PATHS.comingUp, label: 'releases maps to calendar' },
    { path: '/stats', expected: ROUTE_PATHS.stats, label: 'stats tab' },
    { path: '/admin', expected: ROUTE_PATHS.admin, label: 'admin dashboard' },
    { path: '/admin/settings', expected: ROUTE_PATHS.admin, label: 'admin settings' },
    { path: '/admin/tasks', expected: ROUTE_PATHS.admin, label: 'admin tasks' },
    { path: '/admin/auth', expected: ROUTE_PATHS.admin, label: 'admin auth' },
    { path: '/admin/notifications', expected: ROUTE_PATHS.admin, label: 'admin notifications' },
    { path: '/admin/season-audit', expected: ROUTE_PATHS.admin, label: 'admin season audit' },
  ]

  cases.forEach(({ path, expected, label }) => {
    it(`maps ${path} to ${expected} (${label})`, () => {
      expect(getActiveTab(path)).toBe(expected)
    })
  })
})
