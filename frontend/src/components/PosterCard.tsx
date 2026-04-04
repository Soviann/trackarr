import { route } from 'preact-router'
import type { Title } from '../types'
import { colors } from '../theme'
import { CoverPlaceholder, coverBackground } from './CoverPlaceholder'

interface PosterCardProps {
  title: Title
}

export function PosterCard({ title }: PosterCardProps) {
  const name = (title.names ?? []).find((n) => n.is_primary)?.name ?? 'Untitled'

  return (
    <div
      onClick={() => route(`/title/${title.id}`)}
      style={{
        borderRadius: '8px',
        overflow: 'hidden',
        cursor: 'pointer',
      }}
    >
      <div style={{
        aspectRatio: '2/3',
        background: coverBackground(title.cover_url, title.type),
        display: 'flex',
        alignItems: 'flex-end',
        position: 'relative',
      }}>
        {!title.cover_url && <CoverPlaceholder type={title.type} />}
        <div style={{
          width: '100%',
          padding: '6px',
          background: 'linear-gradient(transparent, rgba(0,0,0,0.85))',
        }}>
          <div style={{
            fontSize: '10px',
            fontWeight: 600,
            color: '#fff',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {name}
          </div>
        </div>
      </div>
    </div>
  )
}
