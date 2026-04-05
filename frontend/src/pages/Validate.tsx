import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title, TitleStatus } from '../types'
import { colors } from '../theme'
import { useApi } from '../hooks/useApi'
import { getName } from '../utils'
import { StatusBadge } from '../components/StatusBadge'
import { apiFetch } from '../api'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'

export function Validate({ path }: { path?: string }) {
  const params = new URLSearchParams(window.location.search)
  const query = params.get('q') ?? ''
  const searchPath = query ? `/titles?search=${encodeURIComponent(query)}` : null
  const { data: results, loading } = useApi<Title[]>(searchPath)
  const [adding, setAdding] = useState(false)
  const [selectedStatus, setSelectedStatus] = useState<TitleStatus>('plan_to_watch')

  const handleAdd = async () => {
    if (adding) return
    setAdding(true)
    try {
      const created = await apiFetch<Title>('/titles', {
        method: 'POST',
        body: JSON.stringify({
          type: 'series',
          year: new Date().getFullYear(),
          status: selectedStatus,
          match_status: 'unconfirmed',
          names: [{ name: query, language: 'en', is_primary: true }],
        }),
      })
      route(`/title/${created.id}`)
    } finally {
      setAdding(false)
    }
  }

  return (
    <div style={{ padding: '16px', minHeight: '100vh' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '20px' }}>
        <div
          onClick={() => history.back()}
          style={{
            width: '32px', height: '32px', borderRadius: '50%',
            background: colors.bgCard,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
          }}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
        </div>
        <div style={{ fontSize: '17px', fontWeight: 600, color: colors.textPrimary }}>
          Validating: {query}
        </div>
      </div>

      {loading && (
        <div style={{ textAlign: 'center', padding: '40px 0', color: colors.textSecondary }}>
          <div style={{
            width: '40px', height: '40px', borderRadius: '50%',
            border: `3px solid ${colors.bgSurface}`,
            borderTopColor: colors.accentAmber,
            margin: '0 auto 12px',
            animation: 'spin 1s linear infinite',
          }} />
          Matching...
        </div>
      )}

      {/* Existing results */}
      {results && results.length > 0 && (
        <div style={{ marginBottom: '20px' }}>
          <div style={{ fontSize: '11px', color: colors.textSecondary, fontWeight: 600, marginBottom: '8px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
            Already in library
          </div>
          {results.map((t) => (
            <div
              key={t.id}
              onClick={() => route(`/title/${t.id}`)}
              style={{
                display: 'flex', gap: '12px', alignItems: 'center',
                background: colors.bgCard, borderRadius: '10px',
                padding: '10px 12px', border: `1px solid ${colors.borderCard}`,
                cursor: 'pointer', marginBottom: '8px',
              }}
            >
              <div style={{
                width: '42px', height: '60px', borderRadius: '6px', flexShrink: 0,
                background: coverBackground(t.cover_url, t.type),
                position: 'relative', overflow: 'hidden',
              }}>
                {!t.cover_url && <CoverPlaceholder type={t.type} iconSize="18px" />}
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '13px', fontWeight: 600, color: colors.textPrimary }}>{getName(t)}</span>
                  <StatusBadge status={t.status} />
                </div>
                <div style={{ fontSize: '10px', color: colors.textSecondary, marginTop: '3px' }}>
                  {t.type} · {t.year}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add new */}
      {results && (
        <div style={{
          background: colors.bgCard,
          borderRadius: '12px',
          padding: '16px',
          border: `1px solid ${colors.borderCard}`,
        }}>
          <div style={{ fontSize: '13px', fontWeight: 600, color: colors.textPrimary, marginBottom: '12px' }}>
            Add as new title
          </div>

          {/* Status picker */}
          <div style={{ display: 'flex', gap: '8px', marginBottom: '16px', flexWrap: 'wrap' }}>
            {(['watching', 'plan_to_watch', 'completed'] as TitleStatus[]).map((s) => (
              <button
                key={s}
                onClick={() => setSelectedStatus(s)}
                style={{
                  padding: '8px 14px',
                  borderRadius: '8px',
                  border: `1px solid ${selectedStatus === s ? colors.accentAmber : colors.borderCard}`,
                  background: selectedStatus === s ? `${colors.accentAmber}1F` : colors.bgSurface,
                  color: selectedStatus === s ? colors.accentAmber : colors.textSecondary,
                  fontSize: '12px',
                  fontWeight: selectedStatus === s ? 600 : 400,
                  cursor: 'pointer',
                }}
              >
                {s === 'plan_to_watch' ? 'Plan to watch' : s.charAt(0).toUpperCase() + s.slice(1)}
              </button>
            ))}
          </div>

          <button
            onClick={handleAdd}
            disabled={adding}
            style={{
              width: '100%',
              background: colors.accentGreen,
              borderRadius: '12px',
              padding: '13px',
              border: 'none',
              cursor: adding ? 'default' : 'pointer',
              opacity: adding ? 0.6 : 1,
            }}
          >
            <span style={{ fontSize: '13px', fontWeight: 700, color: '#fff' }}>
              {adding ? 'Adding...' : 'Add to library'}
            </span>
          </button>
        </div>
      )}
    </div>
  )
}
