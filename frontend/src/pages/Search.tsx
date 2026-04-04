import { useState, useRef, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title } from '../types'
import { colors } from '../theme'
import { useApi } from '../hooks/useApi'
import { StatusBadge } from '../components/StatusBadge'

export function Search({ path }: { path?: string }) {
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const searchPath = query.trim() ? `/titles?search=${encodeURIComponent(query.trim())}` : null
  const { data: results, loading } = useApi<Title[]>(searchPath)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const getName = (t: Title) => (t.names ?? []).find((n) => n.is_primary)?.name ?? 'Untitled'
  const getTypeLabel = (t: Title) => t.type.charAt(0).toUpperCase() + t.type.slice(1)

  const getMetadata = (t: Title) => {
    const parts = [getTypeLabel(t), String(t.year)]
    const seasons = t.seasons ?? []
    if (t.type !== 'movie' && seasons.length > 0) {
      const s = seasons[seasons.length - 1]
      const eps = s.episodes ?? []
      const w = eps.filter((e) => e.watched).length
      const total = s.total_episodes ?? eps.length
      parts.push(`S${s.season_number} ${w}/${total}`)
    }
    if (t.my_rating) parts.push(`★ ${t.my_rating}`)
    return parts.join(' · ')
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: 'calc(100vh - 108px)' }}>
      {/* Results area */}
      <div style={{ flex: 1 }}>
        {!query.trim() && (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            minHeight: 'calc(100vh - 200px)',
          }}>
            <div style={{ textAlign: 'center', padding: '0 32px' }}>
              <div style={{
                width: '56px', height: '56px', borderRadius: '50%',
                background: colors.bgCard,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                margin: '0 auto 16px',
              }}>
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#2A2A2A" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
              </div>
              <div style={{ fontSize: '13px', color: colors.textDimmed, lineHeight: '1.6' }}>
                Search across your entire library
              </div>
              <div style={{ fontSize: '11px', color: '#333', marginTop: '4px' }}>All statuses, all types</div>
            </div>
          </div>
        )}

        {query.trim() && results && (
          <>
            <div style={{ padding: '16px 16px 8px' }}>
              <span style={{ fontSize: '10px', color: colors.textMuted }}>
                {results.length} result{results.length !== 1 ? 's' : ''} for "{query.trim()}"
              </span>
            </div>
            <div style={{ padding: '0 16px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {results.map((t) => (
                <div
                  key={t.id}
                  onClick={() => route(`/title/${t.id}`)}
                  style={{
                    display: 'flex',
                    gap: '12px',
                    alignItems: 'center',
                    background: colors.bgCard,
                    borderRadius: '10px',
                    padding: '10px 12px',
                    border: `1px solid ${colors.borderCard}`,
                    cursor: 'pointer',
                  }}
                >
                  <div style={{
                    width: '42px', height: '60px', borderRadius: '6px', flexShrink: 0,
                    background: t.cover_url
                      ? `url(/api/covers/${t.cover_url}) center/cover`
                      : `linear-gradient(135deg, ${colors.bgSurface}, ${colors.bgCard})`,
                  }} />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span style={{
                        fontSize: '13px', fontWeight: 600, color: colors.textPrimary,
                        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                      }}>
                        {getName(t)}
                      </span>
                      <StatusBadge status={t.status} />
                    </div>
                    <div style={{ fontSize: '10px', color: colors.textSecondary, marginTop: '3px' }}>
                      {getMetadata(t)}
                    </div>
                  </div>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#333" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="9 18 15 12 9 6" />
                  </svg>
                </div>
              ))}
            </div>
          </>
        )}

        {query.trim() && loading && (
          <div style={{ padding: '40px 16px', textAlign: 'center', color: colors.textSecondary }}>
            Searching...
          </div>
        )}
      </div>

      {/* Search input */}
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
          border: query ? 'none' : `1.5px solid rgba(56,189,176,0.3)`,
        }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.accentTeal} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <input
            ref={inputRef}
            type="text"
            value={query}
            onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
            placeholder="Search titles..."
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
            <svg
              onClick={() => setQuery('')}
              width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.textMuted}
              stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
              style={{ cursor: 'pointer', flexShrink: 0 }}
            >
              <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          )}
        </div>
      </div>
    </div>
  )
}
