import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { AdminAuth } from './AdminAuth'
import { apiFetch } from '../api'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('AdminAuth Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    setLocale('en')
  })

  afterEach(() => {
    cleanup()
  })

  const mockAuthSettings = {
    auth_mode: 'hybrid' as const,
    has_password: true,
    has_google: true,
    username: 'admin',
  }

  const mockApiKeys = [
    {
      id: 1,
      name: 'MCP Agent',
      key_prefix: 'trck_live_ab',
      scopes: ['library:read', 'library:write'] as const,
      created_at: '2026-10-09T10:00:00Z',
      last_used_at: '2026-10-09T11:00:00Z',
      revoked_at: null,
    },
    {
      id: 2,
      name: 'Old Key',
      key_prefix: 'trck_live_cd',
      scopes: ['library:read'] as const,
      created_at: '2026-10-08T10:00:00Z',
      last_used_at: null,
      revoked_at: '2026-10-09T09:00:00Z',
    },
  ]

  // 1. Nominal: Renders settings, displays API keys list
  it('renders auth settings and API keys list on load', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/admin/auth-settings') return mockAuthSettings
      if (url === '/admin/api-keys') return mockApiKeys
      return {}
    })

    render(<AdminAuth />)

    await waitFor(() => {
      expect(screen.getByText('API Keys & Integrations')).toBeTruthy()
    })

    expect(screen.getByText('MCP Agent')).toBeTruthy()
    expect(screen.getByText('trck_live_ab…')).toBeTruthy()
    expect(screen.getByText('Old Key')).toBeTruthy()
    expect(screen.getByText('Revoked')).toBeTruthy()
  })

  // 2. Boundary: Opens key creation form and generates new token
  it('opens create key drawer, submits, and displays token once', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string, opts?: RequestInit) => {
      if (url === '/admin/auth-settings') return mockAuthSettings
      if (url === '/admin/api-keys') {
        if (opts?.method === 'POST') {
          return {
            key: {
              id: 3,
              name: 'Home Assistant',
              key_prefix: 'trck_live_ef',
              scopes: ['library:read'],
              created_at: '2026-10-09T12:00:00Z',
            },
            token: 'trck_live_ef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
          }
        }
        return mockApiKeys
      }
      return {}
    })

    render(<AdminAuth />)

    await waitFor(() => {
      expect(screen.getByText('Create API Key')).toBeTruthy()
    })

    // Click "Create API Key"
    fireEvent.click(screen.getByText('Create API Key'))

    // Form appears
    const input = screen.getByPlaceholderText('e.g. MCP Server, Home Assistant, Backup Script')
    expect(input).toBeTruthy()
    fireEvent.input(input, { target: { value: 'Home Assistant' } })

    // Click Generate Token
    fireEvent.click(screen.getByText('Generate Token'))

    await waitFor(() => {
      expect(screen.getByText('🔑 API Key Created')).toBeTruthy()
      expect(screen.getByText('trck_live_ef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef')).toBeTruthy()
    })
  })

  // 3. Error / Revoke confirmation
  it('handles revoking a key with confirmation dialog', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    vi.mocked(apiFetch).mockImplementation(async (url: string, opts?: RequestInit) => {
      if (url === '/admin/auth-settings') return mockAuthSettings
      if (url === '/admin/api-keys') return mockApiKeys
      if (url === '/admin/api-keys/1/revoke' && opts?.method === 'POST') return {}
      return {}
    })

    render(<AdminAuth />)

    await waitFor(() => {
      expect(screen.getByText('Revoke')).toBeTruthy()
    })

    fireEvent.click(screen.getByText('Revoke'))

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/api-keys/1/revoke', { method: 'POST' })
    })
  })
})
