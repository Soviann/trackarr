import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup, waitFor } from '@testing-library/preact'
import { PersonalNotesCard } from './PersonalNotesCard'
import * as api from '../api'

describe('PersonalNotesCard', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('renders initial notes', () => {
    const { getByDisplayValue } = render(
      <PersonalNotesCard titleId={42} initialNotes="My favorite season" />
    )
    expect(getByDisplayValue('My favorite season')).not.toBeNull()
  })

  it('auto-saves notes after debounce timeout', async () => {
    const spy = vi.spyOn(api, 'apiFetch').mockResolvedValue({ id: 42, personal_notes: 'New note' })
    const onSaved = vi.fn()

    const { getByPlaceholderText } = render(
      <PersonalNotesCard titleId={42} initialNotes="" onSaved={onSaved} />
    )

    const textarea = getByPlaceholderText(/Ajouter une note personnelle/) as HTMLTextAreaElement
    fireEvent.input(textarea, { target: { value: 'New note' } })

    expect(spy).not.toHaveBeenCalled()

    // Advance past 500ms debounce
    vi.advanceTimersByTime(550)

    expect(spy).toHaveBeenCalledWith('/titles/42', {
      method: 'PATCH',
      body: JSON.stringify({ personal_notes: 'New note' }),
    })
  })

  it('saves on blur', async () => {
    const spy = vi.spyOn(api, 'apiFetch').mockResolvedValue({ id: 42, personal_notes: 'Blur note' })

    const { getByPlaceholderText } = render(
      <PersonalNotesCard titleId={42} initialNotes="" />
    )

    const textarea = getByPlaceholderText(/Ajouter une note personnelle/) as HTMLTextAreaElement
    fireEvent.input(textarea, { target: { value: 'Blur note' } })
    fireEvent.blur(textarea)

    expect(spy).toHaveBeenCalledWith('/titles/42', {
      method: 'PATCH',
      body: JSON.stringify({ personal_notes: 'Blur note' }),
    })
  })
})
