import { describe, it, expect, afterEach, beforeEach } from 'vitest'
import { render, cleanup } from '@testing-library/preact'
import { StatusBadge } from './StatusBadge'
import { setLocale } from '../i18n'

describe('StatusBadge', () => {
  beforeEach(() => {
    setLocale('en')
  })

  afterEach(() => cleanup())

  it('renders Watching badge when status is watching', () => {
    const { getByText } = render(<StatusBadge status="watching" />)
    const badge = getByText('Watching')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('watching')
  })

  it('renders Completed badge when status is completed', () => {
    const { getByText } = render(<StatusBadge status="completed" />)
    const badge = getByText('Completed')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('completed')
  })

  it('renders Dropped badge with dropped class when status is dropped', () => {
    const { getByText } = render(<StatusBadge status="dropped" />)
    const badge = getByText('Dropped')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('dropped')
  })

  it('renders Plan to Watch badge when status is plan_to_watch', () => {
    const { getByText } = render(<StatusBadge status="plan_to_watch" />)
    const badge = getByText('Plan to Watch')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('planToWatch')
  })

  it('renders Caught up when watching and caughtUp is true', () => {
    const { getByText } = render(<StatusBadge status="watching" caughtUp={true} />)
    const badge = getByText('Caught up')
    expect(badge).toBeTruthy()
    expect(badge.className).toContain('caughtUp')
  })

  it('ignores caughtUp flag when status is not watching', () => {
    const { getByText, queryByText } = render(<StatusBadge status="dropped" caughtUp={true} />)
    expect(getByText('Dropped')).toBeTruthy()
    expect(queryByText('Caught up')).toBeNull()
  })

  it('renders localized French labels when locale is fr', () => {
    setLocale('fr')
    const { getByText: getByTextPlan } = render(<StatusBadge status="plan_to_watch" />)
    expect(getByTextPlan('À voir')).toBeTruthy() // i18n-ignore

    cleanup()
    const { getByText: getByTextWatching } = render(<StatusBadge status="watching" caughtUp={true} />)
    expect(getByTextWatching('À jour')).toBeTruthy() // i18n-ignore
  })
})
