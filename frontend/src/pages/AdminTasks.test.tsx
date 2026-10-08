import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { AdminTasks } from './AdminTasks'
import { apiFetch } from '../api'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('AdminTasks Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(apiFetch).mockResolvedValue({ tasks: [], total: 0 })
  })

  afterEach(() => {
    cleanup()
  })

  it('renders tasks with AdminHeader, badges, and action buttons', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tasks: [
        {
          id: 1,
          task_type: 'enrichment',
          payload: JSON.stringify({ title_id: 10, title_name: 'Attack on Titan' }),
          status: 'dead',
          attempts: 5,
          max_attempts: 5,
          day: 1,
          last_error: 'TMDB API timeout',
          run_at: new Date().toISOString(),
          created_at: new Date().toISOString(),
        },
        {
          id: 2,
          task_type: 'refresh',
          payload: JSON.stringify({ title_id: 20, title_name: 'Frieren' }),
          status: 'pending',
          attempts: 1,
          max_attempts: 3,
          day: 1,
          last_error: null,
          run_at: new Date().toISOString(),
          created_at: new Date().toISOString(),
        },
      ],
      total: 2,
    })

    render(<AdminTasks />)

    await waitFor(() => {
      expect(screen.getByText('Background Tasks')).not.toBeNull()
      expect(screen.getByText('(2)')).not.toBeNull()
      expect(screen.getByText('Attack on Titan')).not.toBeNull()
      expect(screen.getByText('Frieren')).not.toBeNull()
    })

    expect(screen.getByText('TMDB API timeout')).not.toBeNull()
    expect(screen.getByText('Failed')).not.toBeNull()
    expect(screen.getByText('Pending')).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Retry' })).not.toBeNull()
    expect(screen.getAllByRole('button', { name: 'Delete' }).length).toBeGreaterThan(0)
  })

  it('renders empty state when there are no tasks matching filter', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tasks: [],
      total: 0,
    })

    render(<AdminTasks />)

    await waitFor(() => {
      expect(screen.getByText('No tasks match this filter')).not.toBeNull()
    })
  })

  it('handles retry and delete operations', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tasks: [
        {
          id: 42,
          task_type: 'enrichment',
          payload: JSON.stringify({ title_id: 42, title_name: 'Jujutsu Kaisen' }),
          status: 'dead',
          attempts: 5,
          max_attempts: 5,
          day: 1,
          last_error: 'Server error 500',
          run_at: new Date().toISOString(),
          created_at: new Date().toISOString(),
        },
      ],
      total: 1,
    })

    render(<AdminTasks />)

    await waitFor(() => {
      expect(screen.getByText('Jujutsu Kaisen')).not.toBeNull()
    })

    // Click retry
    vi.mocked(apiFetch).mockResolvedValueOnce({ success: true })
    const retryBtn = screen.getByRole('button', { name: 'Retry' })
    fireEvent.click(retryBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/tasks/42/retry', { method: 'POST' })
    })
  })

  it('enables select mode and performs batch delete confirmation', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tasks: [
        {
          id: 101,
          task_type: 'cover_fetch',
          payload: JSON.stringify({ title_id: 101, title_name: 'Chainsaw Man' }),
          status: 'pending',
          attempts: 0,
          max_attempts: 3,
          day: 1,
          last_error: null,
          run_at: new Date().toISOString(),
          created_at: new Date().toISOString(),
        },
      ],
      total: 1,
    })

    render(<AdminTasks />)

    await waitFor(() => {
      expect(screen.getByText('Chainsaw Man')).not.toBeNull()
    })

    // Toggle select mode
    const selectBtn = screen.getByRole('button', { name: 'Select' })
    fireEvent.click(selectBtn)

    // Select the task
    const taskCard = screen.getByText('Chainsaw Man').closest('.taskCard')
    expect(taskCard).not.toBeNull()
    if (taskCard) fireEvent.click(taskCard)

    // Sticky action bar should appear
    await waitFor(() => {
      expect(screen.getByText('1 selected')).not.toBeNull()
    })

    const batchDeleteBtn = screen.getByRole('button', { name: 'Delete' })
    fireEvent.click(batchDeleteBtn)

    // Confirmation drawer appears
    await waitFor(() => {
      expect(screen.getByText('Delete 1 tasks?')).not.toBeNull()
    })
  })

  it('opens confirmation modal and deletes a single task', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tasks: [
        {
          id: 42,
          task_type: 'enrichment',
          payload: JSON.stringify({ title_id: 42, title_name: 'Jujutsu Kaisen' }),
          status: 'dead',
          attempts: 5,
          max_attempts: 5,
          day: 1,
          last_error: 'Server error 500',
          run_at: new Date().toISOString(),
          created_at: new Date().toISOString(),
        },
      ],
      total: 1,
    })

    render(<AdminTasks />)

    await waitFor(() => {
      expect(screen.getByText('Jujutsu Kaisen')).not.toBeNull()
    })

    const deleteBtn = screen.getByRole('button', { name: 'Delete' })
    fireEvent.click(deleteBtn)

    await waitFor(() => {
      expect(screen.getByText('Delete this task?')).not.toBeNull()
    })

    vi.mocked(apiFetch).mockResolvedValueOnce({ success: true })
    const allDeleteBtns = screen.getAllByRole('button', { name: 'Delete' })
    const confirmBtn = allDeleteBtns[allDeleteBtns.length - 1]
    fireEvent.click(confirmBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/tasks/42', { method: 'DELETE' })
    })
  })
})
