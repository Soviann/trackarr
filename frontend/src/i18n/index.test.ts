import { describe, it, expect, beforeEach } from 'vitest'
import { translate, setLocale, getLocale, getStoredLocale } from './index'

describe('i18n system', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('en')
  })

  it('translates simple keys in English and French', () => {
    expect(translate('en', 'nav.library')).toBe('Library')
    expect(translate('fr', 'nav.library')).toBe('Bibliothèque')

    expect(translate('en', 'status.watching')).toBe('Watching')
    expect(translate('fr', 'status.watching')).toBe('En cours')
  })

  it('interpolates parameters correctly', () => {
    expect(translate('en', 'details.openInArr', { app: 'Radarr' })).toBe('Open in Radarr')
    expect(translate('fr', 'details.openInArr', { app: 'Sonarr' })).toBe('Ouvrir dans Sonarr')
  })

  it('persists selected locale to localStorage', () => {
    setLocale('fr')
    expect(getLocale()).toBe('fr')
    expect(localStorage.getItem('trackarr_locale')).toBe('fr')
    expect(getStoredLocale()).toBe('fr')
  })
})
