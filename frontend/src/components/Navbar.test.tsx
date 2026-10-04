import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest'
import { render, cleanup, fireEvent } from '@testing-library/preact'
import { Navbar } from './Navbar'
import { setLocale } from '../i18n'

describe('Navbar', () => {
  beforeEach(() => {
    setLocale('en')
  })

  afterEach(() => {
    cleanup()
  })

  it('renders all 5 main navigation tabs in English', () => {
    const onNavigate = vi.fn()
    const { getByRole, getByLabelText } = render(
      <Navbar currentPath="/" onNavigate={onNavigate} />
    )

    expect(getByLabelText('Collection')).toBeTruthy()
    expect(getByLabelText('Explore')).toBeTruthy()
    expect(getByLabelText('Calendar')).toBeTruthy()
    expect(getByLabelText('Stats')).toBeTruthy()
    expect(getByLabelText('Admin')).toBeTruthy()

    const collectionBtn = getByRole('button', { name: 'Collection' })
    expect(collectionBtn.getAttribute('aria-current')).toBe('page')
  })

  it('calls onNavigate with appropriate path when tab is clicked', () => {
    const onNavigate = vi.fn()
    const { getByRole } = render(
      <Navbar currentPath="/" onNavigate={onNavigate} />
    )

    fireEvent.click(getByRole('button', { name: 'Calendar' }))
    expect(onNavigate).toHaveBeenCalledWith('/coming-up')

    fireEvent.click(getByRole('button', { name: 'Explore' }))
    expect(onNavigate).toHaveBeenCalledWith('/search')
  })

  it('activates calendar tab for /coming-up and /releases', () => {
    const onNavigate = vi.fn()
    const { getByRole, rerender } = render(
      <Navbar currentPath="/coming-up" onNavigate={onNavigate} />
    )

    let calBtn = getByRole('button', { name: 'Calendar' })
    expect(calBtn.getAttribute('aria-current')).toBe('page')

    rerender(<Navbar currentPath="/releases" onNavigate={onNavigate} />)
    calBtn = getByRole('button', { name: 'Calendar' })
    expect(calBtn.getAttribute('aria-current')).toBe('page')
  })

  it('activates collection tab for / and /continue-watching', () => {
    const onNavigate = vi.fn()
    const { getByRole, rerender } = render(
      <Navbar currentPath="/" onNavigate={onNavigate} />
    )

    let colBtn = getByRole('button', { name: 'Collection' })
    expect(colBtn.getAttribute('aria-current')).toBe('page')

    rerender(<Navbar currentPath="/continue-watching" onNavigate={onNavigate} />)
    colBtn = getByRole('button', { name: 'Collection' })
    expect(colBtn.getAttribute('aria-current')).toBe('page')
  })

  it('renders above slot when provided', () => {
    const { getByText } = render(
      <Navbar
        currentPath="/"
        onNavigate={vi.fn()}
        above={<div data-testid="above-content">Above Slot</div>}
      />
    )

    expect(getByText('Above Slot')).toBeTruthy()
  })

  it('renders French labels when locale is switched to fr', () => {
    setLocale('fr')
    const { getByLabelText } = render(
      <Navbar currentPath="/" onNavigate={vi.fn()} />
    )

    expect(getByLabelText('Collection')).toBeTruthy()
    expect(getByLabelText('Explorer')).toBeTruthy()
    expect(getByLabelText('Calendrier')).toBeTruthy()
    expect(getByLabelText('Stats')).toBeTruthy()
    expect(getByLabelText('Admin')).toBeTruthy()
  })
})
