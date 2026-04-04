import { useState, useRef, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import { colors } from '../theme'

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
    route(`/validate?q=${encodeURIComponent(query.trim())}`)
  }

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter') handleSubmit()
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: 'calc(100vh - 108px)' }}>
      {/* Empty state */}
      {!query.trim() && (
        <div style={{
          flex: 1,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}>
          <div style={{ textAlign: 'center', padding: '0 32px' }}>
            <div style={{
              width: '56px', height: '56px', borderRadius: '50%',
              background: colors.bgCard,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              margin: '0 auto 16px',
            }}>
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#2A2A2A" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10" /><line x1="12" y1="8" x2="12" y2="16" /><line x1="8" y1="12" x2="16" y2="12" />
              </svg>
            </div>
            <div style={{ fontSize: '13px', color: colors.textDimmed, lineHeight: '1.6' }}>
              Add a title by name or URL
            </div>
            <div style={{ fontSize: '11px', color: '#333', marginTop: '4px' }}>
              Paste an IMDb, TVDB, or AniList link
            </div>
          </div>
        </div>
      )}

      {/* URL detection hint */}
      {urlType && (
        <div style={{
          flex: 1,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}>
          <div style={{ textAlign: 'center', padding: '0 32px' }}>
            <div style={{
              fontSize: '11px',
              color: colors.accentGreen,
              background: `${colors.accentGreen}1F`,
              borderRadius: '8px',
              padding: '8px 16px',
              fontWeight: 500,
            }}>
              {urlType.toUpperCase()} URL detected
            </div>
          </div>
        </div>
      )}

      {/* Non-URL query */}
      {query.trim() && !urlType && (
        <div style={{
          flex: 1,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}>
          <div style={{ textAlign: 'center', padding: '0 32px', color: colors.textDimmed, fontSize: '13px' }}>
            Press Enter to search for "{query.trim()}"
          </div>
        </div>
      )}

      {/* Input */}
      <div style={{
        padding: '8px 16px',
        borderTop: `1px solid ${colors.borderSubtle}`,
        position: 'fixed',
        bottom: '72px',
        left: 0,
        right: 0,
        background: colors.bgPrimary,
        zIndex: 99,
      }}>
        <div style={{
          background: colors.bgSurface,
          borderRadius: '12px',
          padding: '10px 14px',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          border: `1.5px solid rgba(76,175,80,0.3)`,
        }}>
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
            style={{
              flex: 1,
              fontSize: '14px',
              color: colors.textPrimary,
              background: 'transparent',
              border: 'none',
              outline: 'none',
              fontFamily: 'inherit',
            }}
          />
          {query && (
            <button
              onClick={handleSubmit}
              style={{
                background: colors.accentGreen,
                border: 'none',
                borderRadius: '8px',
                padding: '6px 12px',
                cursor: 'pointer',
              }}
            >
              <span style={{ fontSize: '11px', fontWeight: 600, color: '#fff' }}>Go</span>
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
