import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, cleanup, waitFor } from '@testing-library/preact'
import { CalendarIcalModal } from './CalendarIcalModal'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '../api'

const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

describe('CalendarIcalModal', () => {
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('renders nothing when closed', () => {
    const { container } = render(<CalendarIcalModal isOpen={false} onClose={() => {}} />)
    expect(container.firstChild).toBeNull()
  })

  it('loads and renders subscription URLs when opened', async () => {
    mockApiFetch.mockResolvedValue({
      token: 'abcd1234efgh5678',
      feed_url: '/api/calendar.ics?token=abcd1234efgh5678',
      http_url: 'http://localhost:8080/api/calendar.ics?token=abcd1234efgh5678',
      webcal_url: 'webcal://localhost:8080/api/calendar.ics?token=abcd1234efgh5678',
    })

    const { getByText, getAllByText, container } = render(
      <CalendarIcalModal isOpen={true} onClose={() => {}} />
    )

    await waitFor(() => {
      expect(getByText('iCal Calendar Subscription')).toBeTruthy()
      expect(getAllByText(/Apple Calendar/).length).toBeGreaterThan(0)
      expect(getAllByText(/Google Calendar/).length).toBeGreaterThan(0)
    })

    const input = container.querySelector('input') as HTMLInputElement
    expect(input).not.toBeNull()
    expect(input.value).toBe('http://localhost:8080/api/calendar.ics?token=abcd1234efgh5678')
  })
})
