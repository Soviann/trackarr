import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { AdminNotifications } from './AdminNotifications'
import { apiFetch } from '../api'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('AdminNotifications Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    setLocale('en')
  })

  afterEach(() => {
    cleanup()
  })

  const mockNotifPrefs = {
    notif_rating_prompt: true,
    notif_dead_task: false,
    notif_series_ended: true,
  }

  const mockSysSettings = {
    vapid_public_key: 'BMockPublicKey1234567890abcdef',
    vapid_subject: 'mailto:admin@trackarr.local',
    vapid_configured: true,
  }

  it('renders VAPID settings and notification triggers on nominal load', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/admin/notifications') return mockNotifPrefs
      if (url === '/admin/system-settings') return mockSysSettings
      return {}
    })

    render(<AdminNotifications />)

    await waitFor(() => {
      expect(screen.getByText('Notifications')).toBeTruthy()
    })

    expect(screen.getByText(/Web Push Notifications \(VAPID\)/)).toBeTruthy()
    expect(screen.getByText('Auto-configured')).toBeTruthy()
    expect(screen.getByDisplayValue('BMockPublicKey1234567890abcdef')).toBeTruthy()
    expect(screen.getByDisplayValue('mailto:admin@trackarr.local')).toBeTruthy()
    expect(screen.getByText(/Notification Triggers/)).toBeTruthy()
    expect(screen.getByText('Rating reminder')).toBeTruthy()
    expect(screen.getByText('Failed task')).toBeTruthy()
    expect(screen.getByText('Series ended')).toBeTruthy()
  })

  it('toggles a notification preference and calls PUT /admin/notifications', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/admin/notifications') return mockNotifPrefs
      if (url === '/admin/system-settings') return mockSysSettings
      return {}
    })

    render(<AdminNotifications />)

    await waitFor(() => {
      expect(screen.getByText('Failed task')).toBeTruthy()
    })

    const switchBtn = screen.getByRole('switch', { name: /Failed task/i })
    expect(switchBtn.getAttribute('aria-checked')).toBe('false')

    fireEvent.click(switchBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/notifications', {
        method: 'PUT',
        body: JSON.stringify({ notif_dead_task: true }),
      })
    })
  })

  it('regenerates VAPID keys upon clicking Regenerate Keys', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/admin/notifications') return mockNotifPrefs
      if (url === '/admin/system-settings') return mockSysSettings
      if (url === '/admin/system-settings/vapid/generate') {
        return {
          ok: true,
          vapid_public_key: 'BNewGeneratedKey999999',
          vapid_subject: 'mailto:admin@trackarr.local',
          message: 'New keys generated',
        }
      }
      return {}
    })

    render(<AdminNotifications />)

    await waitFor(() => {
      expect(screen.getByText('⚡ Regenerate Keys')).toBeTruthy()
    })

    const regenBtn = screen.getByText('⚡ Regenerate Keys')
    fireEvent.click(regenBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/system-settings/vapid/generate', expect.any(Object))
    })

    await waitFor(() => {
      expect(screen.getByDisplayValue('BNewGeneratedKey999999')).toBeTruthy()
      expect(screen.getByText('New keys generated')).toBeTruthy()
    })
  })

  it('saves VAPID subject and settings on Save Settings click', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/admin/notifications') return mockNotifPrefs
      if (url === '/admin/system-settings') return mockSysSettings
      return {}
    })

    render(<AdminNotifications />)

    await waitFor(() => {
      expect(screen.getByDisplayValue('mailto:admin@trackarr.local')).toBeTruthy()
    })

    const subjectInput = screen.getByDisplayValue('mailto:admin@trackarr.local')
    fireEvent.input(subjectInput, { target: { value: 'mailto:newcontact@example.com' } })

    const saveBtn = screen.getByRole('button', { name: /Save Settings/i })
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/system-settings', {
        method: 'PUT',
        body: JSON.stringify({
          vapid_public_key: 'BMockPublicKey1234567890abcdef',
          vapid_subject: 'mailto:newcontact@example.com',
        }),
      })
    })

    await waitFor(() => {
      expect(screen.getByText('Notification settings saved successfully!')).toBeTruthy()
    })
  })

  it('displays error message when saving VAPID settings fails', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string, opts?: { method?: string }) => {
      if (url === '/admin/notifications') return mockNotifPrefs
      if (url === '/admin/system-settings' && !opts?.method) return mockSysSettings
      if (url === '/admin/system-settings' && opts?.method === 'PUT') {
        throw new Error('Network connection failed')
      }
      return {}
    })

    render(<AdminNotifications />)

    await waitFor(() => {
      expect(screen.getByDisplayValue('mailto:admin@trackarr.local')).toBeTruthy()
    })

    const saveBtn = screen.getByRole('button', { name: /Save Settings/i })
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(screen.getByText('Network connection failed')).toBeTruthy()
    })
  })
})
