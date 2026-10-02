import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/preact'
import { StatusBadge } from './StatusBadge'

afterEach(() => cleanup())

describe('StatusBadge', () => {
  it('renders WATCHING badge when status is watching', () => {
    const { getByText } = render(<StatusBadge status="watching" />)
    const badge = getByText('WATCHING')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('watching')
  })

  it('renders COMPLETED badge when status is completed', () => {
    const { getByText } = render(<StatusBadge status="completed" />)
    const badge = getByText('COMPLETED')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('completed')
  })

  it('renders DROPPED badge with dropped class when status is dropped', () => {
    const { getByText } = render(<StatusBadge status="dropped" />)
    const badge = getByText('DROPPED')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('dropped')
  })

  it('renders PLAN badge when status is plan_to_watch', () => {
    const { getByText } = render(<StatusBadge status="plan_to_watch" />)
    const badge = getByText('PLAN')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('planToWatch')
  })

  it('renders CAUGHT UP when watching and caughtUp is true', () => {
    const { getByText } = render(<StatusBadge status="watching" caughtUp={true} />)
    const badge = getByText('CAUGHT UP')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('caughtUp')
  })

  it('ignores caughtUp flag when status is not watching', () => {
    const { getByText, queryByText } = render(<StatusBadge status="dropped" caughtUp={true} />)
    expect(getByText('DROPPED')).toBeTruthy()
    expect(queryByText('CAUGHT UP')).toBeNull()
  })
})
