import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { AdminHeader } from './AdminHeader'
import * as preactRouter from 'preact-router'

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('AdminHeader', () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('renders title and defaults to routeTo.admin() on back click', () => {
    const { getByRole, getByText } = render(<AdminHeader title="AniList" />)

    expect(getByText('AniList')).toBeTruthy()
    const backBtn = getByRole('button', { name: /back/i })
    fireEvent.click(backBtn)

    expect(preactRouter.route).toHaveBeenCalledWith('/admin')
  })

  it('navigates to backTo when provided', () => {
    const { getByRole } = render(<AdminHeader title="Subpage" backTo="/admin/settings" />)

    const backBtn = getByRole('button', { name: /back/i })
    fireEvent.click(backBtn)

    expect(preactRouter.route).toHaveBeenCalledWith('/admin/settings')
  })

  it('invokes custom onBack handler when provided', () => {
    const onBack = vi.fn()
    const { getByRole } = render(<AdminHeader title="Subpage" onBack={onBack} />)

    const backBtn = getByRole('button', { name: /back/i })
    fireEvent.click(backBtn)

    expect(onBack).toHaveBeenCalledTimes(1)
    expect(preactRouter.route).not.toHaveBeenCalled()
  })

  it('renders badge and action children slots', () => {
    const { getByText } = render(
      <AdminHeader
        title="Tasks"
        badge={<span data-testid="count">5</span>}
      >
        <button type="button">Save</button>
      </AdminHeader>
    )

    expect(getByText('Tasks')).toBeTruthy()
    expect(getByText('5')).toBeTruthy()
    expect(getByText('Save')).toBeTruthy()
  })
})
