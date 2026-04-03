import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title } from '../types'
import { colors } from '../theme'
import { useApi } from '../hooks/useApi'
import { FilterBar, FilterTab } from '../components/FilterBar'
import { TitleCard } from '../components/TitleCard'
import { PosterCard } from '../components/PosterCard'

function isUpToDate(title: Title): boolean {
  if (title.type === 'movie') return false
  return title.seasons.every((s) =>
    s.episodes.length === 0 || s.episodes.every((e) => e.watched)
  )
}

function filterTitles(titles: Title[], tab: FilterTab) {
  if (tab === 'all') return titles
  if (tab === 'watching') return titles.filter((t) => t.status === 'watching' && !isUpToDate(t))
  if (tab === 'up_to_date') return titles.filter((t) => t.status === 'watching' && isUpToDate(t))
  if (tab === 'completed') return titles.filter((t) => t.status === 'completed')
  if (tab === 'dropped') return titles.filter((t) => t.status === 'dropped')
  if (tab === 'plan') return titles.filter((t) => t.status === 'plan_to_watch')
  return titles
}

function SectionHeader({ label, color }: { label: string; color: string }) {
  return (
    <div style={{
      fontSize: '10px',
      color,
      fontWeight: 600,
      textTransform: 'uppercase',
      letterSpacing: '0.5px',
      padding: '12px 16px 8px',
    }}>
      {label}
    </div>
  )
}

function PosterGrid({ titles }: { titles: Title[] }) {
  if (titles.length === 0) return null
  return (
    <div style={{
      padding: '0 16px 8px',
      display: 'grid',
      gridTemplateColumns: '1fr 1fr 1fr',
      gap: '8px',
    }}>
      {titles.map((t) => <PosterCard key={t.id} title={t} />)}
    </div>
  )
}

function TitleList({ titles, onUpdate }: { titles: Title[]; onUpdate: () => void }) {
  if (titles.length === 0) return null
  return (
    <div style={{ padding: '0 16px 6px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
      {titles.map((t) => <TitleCard key={t.id} title={t} onUpdate={onUpdate} />)}
    </div>
  )
}

interface MatchReviewBannerProps {
  count: number
  pendingCount: number
  unconfirmedCount: number
}

function MatchReviewBanner({ count, pendingCount, unconfirmedCount }: MatchReviewBannerProps) {
  if (count === 0) return null
  return (
    <div
      onClick={() => route('/match-review')}
      style={{
        padding: '0 16px 12px',
        cursor: 'pointer',
      }}
    >
      <div style={{
        background: 'rgba(235,87,87,0.08)',
        border: '1px solid rgba(235,87,87,0.2)',
        borderRadius: '10px',
        padding: '10px 12px',
        display: 'flex',
        alignItems: 'center',
        gap: '10px',
      }}>
        <div style={{
          width: '24px',
          height: '24px',
          borderRadius: '50%',
          background: colors.accentCoral,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
        }}>
          <span style={{ fontSize: '11px', fontWeight: 700, color: '#fff' }}>{count}</span>
        </div>
        <div style={{ flex: 1 }}>
          <span style={{ fontSize: '12px', color: colors.textPrimary, fontWeight: 500 }}>
            {count} title{count > 1 ? 's' : ''} need{count === 1 ? 's' : ''} review
          </span>
          <span style={{ fontSize: '10px', color: colors.textSecondary, marginLeft: '6px' }}>
            {pendingCount} pending · {unconfirmedCount} unconfirmed
          </span>
        </div>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke={colors.textMuted} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="9 18 15 12 9 6" />
        </svg>
      </div>
    </div>
  )
}

export function Library({ path }: { path?: string }) {
  const [tab, setTab] = useState<FilterTab>('all')
  const { data: titles, loading, mutate } = useApi<Title[]>('/titles')

  const allTitles = titles ?? []
  const pendingCount = allTitles.filter((t) => t.match_status === 'pending_review').length
  const unconfirmedCount = allTitles.filter((t) => t.match_status === 'unconfirmed').length
  const reviewCount = pendingCount + unconfirmedCount

  const filtered = filterTitles(allTitles, tab)

  const watching = filtered.filter((t) => t.status === 'watching' && !isUpToDate(t))
  const upToDate = filtered.filter((t) => t.status === 'watching' && isUpToDate(t))
  const completed = filtered.filter((t) => t.status === 'completed')
  const dropped = filtered.filter((t) => t.status === 'dropped')
  const planToWatch = filtered.filter((t) => t.status === 'plan_to_watch')

  const showSection = (items: unknown[]) => tab === 'all' ? items.length > 0 : true

  return (
    <div style={{ paddingBottom: '36px' }}>
      {/* Header */}
      <div style={{ padding: '16px 16px 10px' }}>
        <div style={{ fontSize: '20px', fontWeight: 700, color: '#fff' }}>Library</div>
      </div>

      {loading && (
        <div style={{ padding: '40px 16px', textAlign: 'center', color: colors.textSecondary }}>
          Loading...
        </div>
      )}

      {!loading && allTitles.length === 0 && (
        <div style={{ padding: '40px 16px', textAlign: 'center', color: colors.textSecondary }}>
          No titles yet. Add one with the + tab!
        </div>
      )}

      {!loading && allTitles.length > 0 && (
        <>
          <MatchReviewBanner count={reviewCount} pendingCount={pendingCount} unconfirmedCount={unconfirmedCount} />

          {tab === 'all' ? (
            <>
              {showSection(watching) && (
                <>
                  <SectionHeader label="Watching" color={colors.accentAmber} />
                  <TitleList titles={watching} onUpdate={mutate} />
                </>
              )}
              {showSection(upToDate) && (
                <>
                  <SectionHeader label="Up to date" color={colors.accentGreen} />
                  <TitleList titles={upToDate} onUpdate={mutate} />
                </>
              )}
              {showSection(completed) && (
                <>
                  <SectionHeader label="Completed" color={colors.textSecondary} />
                  <PosterGrid titles={completed} />
                </>
              )}
              {showSection(planToWatch) && (
                <>
                  <SectionHeader label="Plan to watch" color={colors.textSecondary} />
                  <PosterGrid titles={planToWatch} />
                </>
              )}
              {showSection(dropped) && (
                <>
                  <SectionHeader label="Dropped" color={colors.textSecondary} />
                  <PosterGrid titles={dropped} />
                </>
              )}
            </>
          ) : (
            <>
              {(tab === 'watching' || tab === 'up_to_date') ? (
                <TitleList titles={filtered} onUpdate={mutate} />
              ) : (
                <PosterGrid titles={filtered} />
              )}
            </>
          )}
        </>
      )}

      <FilterBar active={tab} onChange={setTab} />
    </div>
  )
}
