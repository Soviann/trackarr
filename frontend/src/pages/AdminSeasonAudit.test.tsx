import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { AdminSeasonAudit } from './AdminSeasonAudit'
import { apiFetch } from '../api'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('../components/CoverImage', () => ({
  CoverImage: ({ alt }: { alt?: string }) => <div data-testid="cover-image" aria-label={alt} />,
}))

describe('AdminSeasonAudit Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it('renders empty state when there are no proposals', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({ proposals: [] })

    render(<AdminSeasonAudit />)

    await waitFor(() => {
      expect(screen.getByText('No season conflicts found 🎉')).not.toBeNull()
    })
  })

  it('renders proposals with face-to-face cards, badges, and disabled Merge All when ambiguous', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      proposals: [
        {
          source_title_id: 1,
          source_name: 'Frieren Season 2',
          source_year: 2026,
          source_cover_url: '/covers/frieren2.jpg',
          source_seasons_count: 1,
          target_title_id: 2,
          target_name: 'Sousou no Frieren',
          target_year: 2023,
          target_cover_url: '/covers/frieren.jpg',
          target_seasons_count: 1,
          season_number: 2,
          shared_id: 'anilist:12345',
        },
        {
          source_title_id: 3,
          source_name: 'Attack on Titan Final Part 3',
          source_year: 2023,
          source_cover_url: null,
          source_seasons_count: 1,
          target_title_id: 4,
          target_name: 'Shingeki no Kyojin',
          target_year: 2013,
          target_cover_url: null,
          target_seasons_count: 4,
          season_number: 0,
          shared_id: 'imdb:tt2560140',
        },
      ],
    })

    render(<AdminSeasonAudit />)

    await waitFor(() => {
      expect(screen.getByText('Frieren Season 2')).not.toBeNull()
      expect(screen.getByText('Sousou no Frieren')).not.toBeNull()
    })

    // Badges
    expect(screen.getByText('Season 2 suggested')).not.toBeNull()
    expect(screen.getByText('Season to define')).not.toBeNull()
    expect(screen.getByText('anilist:12345')).not.toBeNull()
    expect(screen.getByText('imdb:tt2560140')).not.toBeNull()

    // Merge all button should be disabled because proposal 2 has season_number === 0
    const mergeAllBtn = screen.getByRole('button', { name: /Merge all/i })
    expect((mergeAllBtn as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText(/Disabled: some proposals require manual season assignment/i)).not.toBeNull()
  })

  it('opens BottomSheet drawer, allows changing season number and confirms merge', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      proposals: [
        {
          source_title_id: 3,
          source_name: 'Solo Leveling Season 2',
          source_year: 2025,
          source_cover_url: null,
          source_seasons_count: 1,
          target_title_id: 4,
          target_name: 'Solo Leveling',
          target_year: 2024,
          target_cover_url: null,
          target_seasons_count: 1,
          season_number: 0,
          shared_id: 'tvdb:414532',
        },
      ],
    })

    render(<AdminSeasonAudit />)

    await waitFor(() => {
      expect(screen.getByText('Solo Leveling Season 2')).not.toBeNull()
    })

    const mergeBtn = screen.getByRole('button', { name: 'Merge...' })
    fireEvent.click(mergeBtn)

    // Drawer should appear
    expect(screen.getByText('Merge season into series')).not.toBeNull()
    const seasonInput = screen.getByLabelText(/Integrate as season number/i) as HTMLInputElement
    // Should be prefilled with target_seasons_count + 1 = 2
    expect(seasonInput.value).toBe('2')

    // Change season to 3
    fireEvent.input(seasonInput, { target: { value: '3' } })
    expect(seasonInput.value).toBe('3')

    vi.mocked(apiFetch).mockResolvedValueOnce({ status: 'ok' })

    const confirmBtn = screen.getByRole('button', { name: 'Confirm merge' })
    fireEvent.click(confirmBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/season-audit/accept', {
        method: 'POST',
        body: JSON.stringify({
          source_title_id: 3,
          target_title_id: 4,
          season_number: 3,
        }),
      })
    })
  })

  it('calls dismiss endpoint when clicking Dismiss', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      proposals: [
        {
          source_title_id: 1,
          source_name: 'Source Show',
          source_year: 2024,
          source_cover_url: null,
          source_seasons_count: 1,
          target_title_id: 2,
          target_name: 'Target Show',
          target_year: 2023,
          target_cover_url: null,
          target_seasons_count: 1,
          season_number: 2,
          shared_id: 'imdb:tt1111111',
        },
      ],
    })

    render(<AdminSeasonAudit />)

    await waitFor(() => {
      expect(screen.getByText('Source Show')).not.toBeNull()
    })

    vi.mocked(apiFetch).mockResolvedValueOnce({ status: 'ok' })

    const dismissBtn = screen.getByRole('button', { name: 'Dismiss' })
    fireEvent.click(dismissBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/season-audit/dismiss', {
        method: 'POST',
        body: JSON.stringify({
          source_title_id: 1,
          target_title_id: 2,
        }),
      })
    })
  })
})
