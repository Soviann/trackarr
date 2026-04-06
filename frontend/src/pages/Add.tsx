import { useState, useRef, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import { colors } from '../theme'
import s from './Add.module.css'

function detectUrlType(input: string): string | null {
  if (/imdb\.com\/title\/(tt\d+)/i.test(input)) return 'imdb'
  if (/thetvdb\.com/i.test(input)) return 'tvdb'
  if (/anilist\.co\/anime\/(\d+)/i.test(input)) return 'anilist'
  return null
}

export function Add({ path }: { path?: string }) {
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const urlType = query.trim() ? detectUrlType(query.trim()) : null

  const handleSubmit = () => {
    if (!query.trim()) return
    route(`/admin/validate?q=${encodeURIComponent(query.trim())}`)
  }

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter') handleSubmit()
  }

  return (
    <div className={s.page}>
      {/* Empty state */}
      {!query.trim() && (
        <div className={s.center}>
          <div className={s.centerContent}>
            <div className={s.emptyIcon}>
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#888" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10" /><line x1="12" y1="8" x2="12" y2="16" /><line x1="8" y1="12" x2="16" y2="12" />
              </svg>
            </div>
            <div className={s.emptyText}>
              Add a title by name or URL
            </div>
            <div className={s.emptyHint}>
              Paste an IMDb, TVDB, or AniList link
            </div>
          </div>
        </div>
      )}

      {/* URL detection hint */}
      {urlType && (
        <div className={s.center}>
          <div className={s.centerContent}>
            <div className={s.urlBadge}>
              {urlType.toUpperCase()} URL detected
            </div>
          </div>
        </div>
      )}

      {/* Non-URL query */}
      {query.trim() && !urlType && (
        <div className={s.center}>
          <div className={s.searchHint}>
            Press Enter to search for "{query.trim()}"
          </div>
        </div>
      )}

      {/* Input */}
      <div className={s.inputBar}>
        <div className={s.inputWrapper}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.accentGreen} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10" /><line x1="12" y1="8" x2="12" y2="16" /><line x1="8" y1="12" x2="16" y2="12" />
          </svg>
          <input
            ref={inputRef}
            type="text"
            name="add-title"
            id="add-title"
            autocomplete="off"
            value={query}
            onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
            onKeyDown={handleKeyDown}
            placeholder="Title name or URL..."
            className={s.input}
          />
          {query && (
            <button onClick={handleSubmit} className={s.goBtn}>
              <span className={s.goBtnText}>Go</span>
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
