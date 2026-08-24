import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/preact'
import { PrimeBadge } from './PrimeBadge'
import { isOnPrime } from '../utils/providers'

afterEach(() => cleanup())

describe('isOnPrime', () => {
  it('is true when an Amazon Prime Video id is present', () => {
    expect(isOnPrime([{ id: 119, name: 'Amazon Prime Video' }])).toBe(true)
  })
  it('ignores the rent/buy storefront (id 10) and empty input', () => {
    expect(isOnPrime([{ id: 10, name: 'Amazon Video' }])).toBe(false)
    expect(isOnPrime([])).toBe(false)
    expect(isOnPrime(undefined)).toBe(false)
  })
})

describe('PrimeBadge', () => {
  it('renders the prime label', () => {
    const { getByText } = render(<PrimeBadge />)
    expect(getByText('prime')).toBeTruthy()
  })
})
