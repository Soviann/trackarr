import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { ArrPushSheet } from './ArrPushSheet'
import { apiFetch } from '../api'
import type { Title } from '../types'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

const baseTitle: Title = {
  id: 42,
  type: 'series',
  is_anime: false,
  year: 2024,
  cover_url: null,
  accent_hex: null,
  imdb_id: null,
  simkl_id: null,
  simkl_slug: null,
  anilist_id: null,
  tmdb_id: null,
  tvdb_id: 12345,
  total_watch_minutes: 0,
  my_rating: null,
  status: 'dropped',
  series_status: 'ended',
  match_status: 'confirmed',
  original_title: 'Dropped Series',
  match_source: null,
  overview: null,
  genres: null,
  runtime: null,
  tmdb_rating: null,
  credits: null,
  anilist_rating: null,
  release_date: null,
  created_at: '',
  updated_at: '',
  names: [{ id: 1, title_id: 42, name: 'Dropped Series', language: 'en', is_primary: true }],
  seasons: [],
}

describe('ArrPushSheet', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path === '/admin/arr') {
        return {
          sonarr_std_root_folder: '/tv',
          sonarr_std_quality_profile: '1',
        }
      }
      if (path.includes('/rootfolder')) {
        return [{ id: 1, path: '/tv' }]
      }
      if (path.includes('/qualityprofile')) {
        return [{ id: 1, name: 'HD - 1080p' }]
      }
      if (path.includes('/arr/title/')) {
        return { exists: false }
      }
      return {}
    })
  })

  afterEach(() => {
    cleanup()
  })

  it('renders "Send to Sonarr" when series was never deleted from Sonarr', async () => {
    render(<ArrPushSheet open={true} onClose={vi.fn()} title={baseTitle} />)

    await waitFor(() => {
      expect(screen.getByText('Send to Sonarr')).toBeTruthy()
    })
    expect(screen.queryByText(/previously deleted and excluded/i)).toBeNull()
  })

  it('renders notice and "Re-add to Sonarr" when series has sonarr_deleted_at', async () => {
    const deletedTitle: Title = {
      ...baseTitle,
      sonarr_deleted_at: '2026-09-25T10:00:00Z',
    }

    render(<ArrPushSheet open={true} onClose={vi.fn()} title={deletedTitle} />)

    await waitFor(() => {
      expect(screen.getByText('Re-add to Sonarr')).toBeTruthy()
    })
    expect(screen.getByText(/previously deleted and excluded from Sonarr/i)).toBeTruthy()
  })

  it('renders "Send to Radarr" for movies even if sonarr_deleted_at is set', async () => {
    const movieTitle: Title = {
      ...baseTitle,
      type: 'movie',
      tmdb_id: 999,
      sonarr_deleted_at: '2026-09-25T10:00:00Z',
    }

    render(<ArrPushSheet open={true} onClose={vi.fn()} title={movieTitle} />)

    await waitFor(() => {
      expect(screen.getByText('Send to Radarr')).toBeTruthy()
    })
    expect(screen.queryByText(/previously deleted and excluded/i)).toBeNull()
  })

  it('submits push request and invokes onSuccess when Re-add is clicked', async () => {
    const deletedTitle: Title = {
      ...baseTitle,
      sonarr_deleted_at: '2026-09-25T10:00:00Z',
    }
    const onSuccess = vi.fn()
    const onClose = vi.fn()

    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path === '/admin/arr') return { sonarr_std_root_folder: '/tv', sonarr_std_quality_profile: '1' }
      if (path.includes('/rootfolder')) return [{ id: 1, path: '/tv' }]
      if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD - 1080p' }]
      if (path.includes('/arr/title/')) return { exists: false }
      if (path === '/arr/push/42') return { status: 'created', arr_id: 99 }
      return {}
    })

    render(<ArrPushSheet open={true} onClose={onClose} title={deletedTitle} onSuccess={onSuccess} />)

    await waitFor(() => {
      expect(screen.getByText('Re-add to Sonarr')).toBeTruthy()
    })

    fireEvent.click(screen.getByText('Re-add to Sonarr'))

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/arr/push/42', expect.objectContaining({
        method: 'POST',
      }))
      expect(onSuccess).toHaveBeenCalledWith(99)
      expect(onClose).toHaveBeenCalledOnce()
    })
  })

  it('triggers onUnlinked and shows stale entry notice when title was linked but no longer exists in Sonarr', async () => {
    const linkedTitle: Title = {
      ...baseTitle,
      sonarr_id: 99,
    }
    const onUnlinked = vi.fn()

    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path === '/admin/arr') return { sonarr_std_root_folder: '/tv', sonarr_std_quality_profile: '1' }
      if (path.includes('/rootfolder')) return [{ id: 1, path: '/tv' }]
      if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD - 1080p' }]
      if (path === '/arr/title/42') return { exists: false }
      return {}
    })

    render(<ArrPushSheet open={true} onClose={vi.fn()} title={linkedTitle} onUnlinked={onUnlinked} />)

    await waitFor(() => {
      expect(onUnlinked).toHaveBeenCalledOnce()
      expect(screen.getByText(/was not found in Sonarr/i)).toBeTruthy()
      expect(screen.getByText('Send to Sonarr')).toBeTruthy()
    })
  })

  it('renders "Update in Sonarr" and does not call onUnlinked when entry exists in Sonarr', async () => {
    const linkedTitle: Title = {
      ...baseTitle,
      sonarr_id: 99,
    }
    const onUnlinked = vi.fn()

    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path === '/admin/arr') return { sonarr_std_root_folder: '/tv', sonarr_std_quality_profile: '1' }
      if (path.includes('/rootfolder')) return [{ id: 1, path: '/tv' }]
      if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD - 1080p' }]
      if (path === '/arr/title/42') {
        return {
          exists: true,
          mon: true,
          root_folder: '/tv',
          quality_profile_id: 1,
        }
      }
      return {}
    })

    render(<ArrPushSheet open={true} onClose={vi.fn()} title={linkedTitle} onUnlinked={onUnlinked} />)

    await waitFor(() => {
      expect(screen.getByText('Update in Sonarr')).toBeTruthy()
    })
    expect(onUnlinked).not.toHaveBeenCalled()
    expect(screen.queryByText(/This entry no longer exists in Sonarr/i)).toBeNull()
    expect(screen.queryByRole('button', { name: /Delete from Radarr/i })).toBeNull()
  })

  describe('Radarr deletion flow', () => {
    const radarrMovie: Title = {
      ...baseTitle,
      id: 77,
      type: 'movie',
      tmdb_id: 550,
      tvdb_id: null,
      radarr_id: 321,
      downloaded_at: '2026-01-10T12:00:00Z',
      status: 'completed',
      my_rating: 9,
      original_title: 'Fight Club',
      names: [{ id: 1, title_id: 77, name: 'Fight Club', language: 'en', is_primary: true }],
    }

    const mockMovieApis = (exists: boolean, deleteImpl?: () => Promise<unknown>) => {
      vi.mocked(apiFetch).mockImplementation(async (path: string, init?: RequestInit) => {
        if (path === '/admin/arr') {
          return { radarr_std_root_folder: '/movies', radarr_std_quality_profile: '1' }
        }
        if (path.includes('/rootfolder')) {
          return [{ id: 1, path: '/movies' }]
        }
        if (path.includes('/qualityprofile')) {
          return [{ id: 1, name: 'HD - 1080p' }]
        }
        if (path === '/arr/title/77' && (!init || !init.method || init.method === 'GET')) {
          return exists
            ? {
                app: 'radarr',
                exists: true,
                arr_id: 321,
                monitored: true,
                quality_profile_id: 1,
                root_folder_path: '/movies',
                has_file: true,
              }
            : { app: 'radarr', exists: false }
        }
        if (path === '/arr/title/77' && init?.method === 'DELETE') {
          if (deleteImpl) return deleteImpl()
          return { status: 'ok' }
        }
        return {}
      })
    }

    it('shows "Delete from Radarr" for a Radarr-linked movie and completes deletion with default exclusion', async () => {
      mockMovieApis(true)
      const onDeleted = vi.fn()
      const onUnlinked = vi.fn()
      const onClose = vi.fn()

      render(
        <ArrPushSheet
          open={true}
          onClose={onClose}
          title={radarrMovie}
          onDeleted={onDeleted}
          onUnlinked={onUnlinked}
        />
      )

      const deleteBtn = await screen.findByRole('button', { name: /Delete from Radarr/i })
      expect(deleteBtn).toBeTruthy()
      fireEvent.click(deleteBtn)

      // Confirmation view opens in the same sheet with disk/NAS warning and exclusion checked by default
      expect(screen.getByText(/permanently delete its folder and video files from disk \/ NAS/i)).toBeTruthy()
      const exclusionCheckbox = screen.getByRole('checkbox') as HTMLInputElement
      expect(exclusionCheckbox.checked).toBe(true)

      const confirmBtn = screen.getByRole('button', { name: /Confirm permanent deletion/i })
      fireEvent.click(confirmBtn)

      await waitFor(() => {
        expect(apiFetch).toHaveBeenCalledWith(
          '/arr/title/77',
          expect.objectContaining({
            method: 'DELETE',
            body: JSON.stringify({ delete_files: true, add_import_exclusion: true }),
          })
        )
        expect(onDeleted).toHaveBeenCalledOnce()
        expect(onUnlinked).not.toHaveBeenCalled()
        expect(onClose).toHaveBeenCalledOnce()
      })
    })

    it('sends add_import_exclusion: false when unchecked and falls back to onUnlinked when onDeleted is omitted', async () => {
      mockMovieApis(true)
      const onUnlinked = vi.fn()
      const onClose = vi.fn()

      render(
        <ArrPushSheet
          open={true}
          onClose={onClose}
          title={radarrMovie}
          onUnlinked={onUnlinked}
        />
      )

      const deleteBtn = await screen.findByRole('button', { name: /Delete from Radarr/i })
      fireEvent.click(deleteBtn)

      const exclusionCheckbox = screen.getByRole('checkbox') as HTMLInputElement
      fireEvent.click(exclusionCheckbox)
      expect(exclusionCheckbox.checked).toBe(false)

      fireEvent.click(screen.getByRole('button', { name: /Confirm permanent deletion/i }))

      await waitFor(() => {
        expect(apiFetch).toHaveBeenCalledWith(
          '/arr/title/77',
          expect.objectContaining({
            method: 'DELETE',
            body: JSON.stringify({ delete_files: true, add_import_exclusion: false }),
          })
        )
        expect(onUnlinked).toHaveBeenCalledOnce()
        expect(onClose).toHaveBeenCalledOnce()
      })
    })

    it('does not show "Delete from Radarr" when movie does not exist in Radarr or arrDetails fails', async () => {
      mockMovieApis(false)
      render(<ArrPushSheet open={true} onClose={vi.fn()} title={radarrMovie} />)

      await waitFor(() => {
        expect(screen.getByText('Send to Radarr')).toBeTruthy()
      })
      expect(screen.queryByRole('button', { name: /Delete from Radarr/i })).toBeNull()

      cleanup()
      vi.mocked(apiFetch).mockImplementation(async (path: string) => {
        if (path === '/admin/arr') return {}
        if (path.includes('/rootfolder')) return [{ id: 1, path: '/movies' }]
        if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD' }]
        if (path === '/arr/title/77') throw new Error('Network failure')
        return {}
      })

      render(<ArrPushSheet open={true} onClose={vi.fn()} title={radarrMovie} />)
      await waitFor(() => {
        expect(screen.getByText('Send to Radarr')).toBeTruthy()
      })
      expect(screen.queryByRole('button', { name: /Delete from Radarr/i })).toBeNull()
    })

    it('returns to Radarr options view on Cancel without deleting', async () => {
      mockMovieApis(true)
      const onDeleted = vi.fn()
      const onClose = vi.fn()

      render(<ArrPushSheet open={true} onClose={onClose} title={radarrMovie} onDeleted={onDeleted} />)

      const deleteBtn = await screen.findByRole('button', { name: /Delete from Radarr/i })
      fireEvent.click(deleteBtn)

      expect(screen.getByRole('button', { name: /Confirm permanent deletion/i })).toBeTruthy()
      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

      await waitFor(() => {
        expect(screen.getByText('Update in Radarr')).toBeTruthy()
      })
      expect(screen.queryByRole('button', { name: /Confirm permanent deletion/i })).toBeNull()
      expect(onDeleted).not.toHaveBeenCalled()
      expect(onClose).not.toHaveBeenCalled()
      expect(vi.mocked(apiFetch).mock.calls.some(([, opts]) => opts?.method === 'DELETE')).toBe(false)
    })

    it('disables confirm and cancel buttons while delete is in flight and surfaces errorBanner on failure', async () => {
      let rejectDelete!: (err: Error) => void
      const deletePromise = new Promise((_, reject) => {
        rejectDelete = reject
      })
      mockMovieApis(true, () => deletePromise)

      const onDeleted = vi.fn()
      const onClose = vi.fn()

      render(<ArrPushSheet open={true} onClose={onClose} title={radarrMovie} onDeleted={onDeleted} />)

      const deleteBtn = await screen.findByRole('button', { name: /Delete from Radarr/i })
      fireEvent.click(deleteBtn)

      const confirmBtn = screen.getByRole('button', { name: /Confirm permanent deletion/i }) as HTMLButtonElement
      const cancelBtn = screen.getByRole('button', { name: 'Cancel' }) as HTMLButtonElement
      fireEvent.click(confirmBtn)

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Deleting/i })).toBeTruthy()
      })
      expect((screen.getByRole('button', { name: /Deleting/i }) as HTMLButtonElement).disabled).toBe(true)
      expect(cancelBtn.disabled).toBe(true)

      rejectDelete(new Error('Radarr offline'))

      await waitFor(() => {
        const alert = screen.getByRole('alert')
        expect(alert.textContent).toContain('Radarr offline')
        expect(alert.className).toContain('errorBanner')
      })
      expect(onDeleted).not.toHaveBeenCalled()
      expect(onClose).not.toHaveBeenCalled()
      expect((screen.getByRole('button', { name: /Confirm permanent deletion/i }) as HTMLButtonElement).disabled).toBe(false)

      // Returning via Cancel clears errorBanner
      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
      await waitFor(() => {
        expect(screen.queryByRole('alert')).toBeNull()
        expect(screen.getByText('Update in Radarr')).toBeTruthy()
      })
    })
  })
})
