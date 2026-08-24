import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/preact'
import { Setup } from './Setup'

describe('Setup Page (Onboarding Wizard)', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('renders onboarding form and handles account creation with recovery key', async () => {
    globalThis.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/api/config')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ setup_required: true, google_client_id: '' }),
        })
      }
      if (url.includes('/api/auth/setup')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ recovery_key: 'TRCK-TEST-RECOVERY-KEY-1234' }),
        })
      }
      return Promise.reject(new Error('Unknown url: ' + url))
    })

    render(<Setup />)

    expect(screen.getByText('Welcome to Trackarr')).toBeDefined()
    expect(screen.getByText('First-Time Setup Wizard')).toBeDefined()

    const passwordInput = screen.getByLabelText(/^Password/i)
    const confirmInput = screen.getByLabelText(/^Confirm Password/i)
    const submitBtn = screen.getByText(/Create Account/i)

    fireEvent.input(passwordInput, { target: { value: 'password123' } })
    fireEvent.input(confirmInput, { target: { value: 'password123' } })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(screen.getByText(/Save Your Emergency Recovery Key/i)).toBeDefined()
      expect(screen.getByText('TRCK-TEST-RECOVERY-KEY-1234')).toBeDefined()
    })
  })

  it('displays validation error if passwords do not match', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ setup_required: true, google_client_id: '' }),
    })

    render(<Setup />)

    const passwordInput = screen.getByLabelText(/^Password/i)
    const confirmInput = screen.getByLabelText(/^Confirm Password/i)
    const submitBtn = screen.getByText(/Create Account/i)

    fireEvent.input(passwordInput, { target: { value: 'password123' } })
    fireEvent.input(confirmInput, { target: { value: 'password456' } })
    fireEvent.click(submitBtn)

    expect(screen.getByText('Passwords do not match')).toBeDefined()
  })
})
