import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { Login } from './Login'
import { apiFetch } from '../api'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('Login Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it('renders only Google Sign-In in google-only mode', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        google_client_id: 'test-google-id',
        google_auth_enabled: true,
        password_auth_enabled: false,
        auth_mode: 'google',
        setup_required: false,
      }),
    } as any)

    render(<Login />)

    await waitFor(() => {
      expect(screen.getByText('Trackarr')).not.toBeNull()
    })

    // Password form must NOT be present
    expect(screen.queryByLabelText('Username')).toBeNull()
    expect(screen.queryByLabelText('Password')).toBeNull()
    expect(screen.queryByText('Forgot Password? (Emergency Key)')).toBeNull()
    expect(screen.queryByText('OR')).toBeNull()
  })

  it('renders local login form and recovery button in password-only mode', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        google_client_id: '',
        google_auth_enabled: false,
        password_auth_enabled: true,
        auth_mode: 'password',
        setup_required: false,
      }),
    } as any)

    render(<Login />)

    await waitFor(() => {
      expect(screen.getByLabelText('Username')).not.toBeNull()
    })

    expect(screen.getByLabelText('Password')).not.toBeNull()
    expect(screen.getByText('Sign In')).not.toBeNull()
    expect(screen.getByText('Forgot Password? (Emergency Key)')).not.toBeNull()
    expect(screen.queryByText('OR')).toBeNull()
  })

  it('renders both local login and Google in hybrid mode', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        google_client_id: 'test-google-id',
        google_auth_enabled: true,
        password_auth_enabled: true,
        auth_mode: 'hybrid',
        setup_required: false,
      }),
    } as any)

    render(<Login />)

    await waitFor(() => {
      expect(screen.getByLabelText('Username')).not.toBeNull()
    })

    expect(screen.getByLabelText('Password')).not.toBeNull()
    expect(screen.getByText('OR')).not.toBeNull()
    expect(screen.getByText('Forgot Password? (Emergency Key)')).not.toBeNull()
  })

  it('handles emergency password recovery and displays new key', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        google_client_id: '',
        google_auth_enabled: false,
        password_auth_enabled: true,
        auth_mode: 'password',
        setup_required: false,
      }),
    } as any)

    vi.mocked(apiFetch).mockResolvedValue({
      new_recovery_key: 'TRCK-9999-8888-7777',
    })

    render(<Login />)

    await waitFor(() => {
      expect(screen.getByText('Forgot Password? (Emergency Key)')).not.toBeNull()
    })

    fireEvent.click(screen.getByText('Forgot Password? (Emergency Key)'))

    expect(screen.getByLabelText('Emergency Recovery Key')).not.toBeNull()
    expect(screen.getByLabelText('New Password')).not.toBeNull()
    expect(screen.getByLabelText('Confirm Password')).not.toBeNull()

    fireEvent.input(screen.getByLabelText('Emergency Recovery Key'), {
      target: { value: 'TRCK-1234-5678-9012' },
    })
    fireEvent.input(screen.getByLabelText('New Password'), {
      target: { value: 'brandnewpass123' },
    })
    fireEvent.input(screen.getByLabelText('Confirm Password'), {
      target: { value: 'brandnewpass123' },
    })

    fireEvent.click(screen.getByText('Reset Password'))

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/auth/recover', {
        method: 'POST',
        body: JSON.stringify({
          recovery_key: 'TRCK-1234-5678-9012',
          new_password: 'brandnewpass123',
        }),
      })
      expect(screen.getByText('TRCK-9999-8888-7777')).not.toBeNull()
      expect(screen.getByText('Open Trackarr')).not.toBeNull()
    })
  })
})
