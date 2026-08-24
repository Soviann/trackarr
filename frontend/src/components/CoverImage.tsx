import { useState, useEffect } from 'preact/hooks'
import type { TitleType } from '../types'
import { getCoverUrl } from '../utils'
import { CoverPlaceholder } from './CoverPlaceholder'

export interface CoverImageProps {
  coverUrl?: string | null
  type: TitleType
  is_anime?: boolean
  alt?: string
  className?: string
  iconSize?: string
  loading?: 'lazy' | 'eager'
  decoding?: 'async' | 'auto' | 'sync'
  onClick?: (e: MouseEvent) => void
}

export function CoverImage({
  coverUrl,
  type,
  is_anime,
  alt = '',
  className,
  iconSize,
  loading = 'lazy',
  decoding = 'async',
  onClick,
}: CoverImageProps) {
  const [imgError, setImgError] = useState(false)

  useEffect(() => {
    setImgError(false)
  }, [coverUrl])

  const resolvedUrl = getCoverUrl(coverUrl)

  if (resolvedUrl && !imgError) {
    return (
      <img
        src={resolvedUrl}
        alt={alt}
        className={className}
        loading={loading}
        decoding={decoding}
        onClick={onClick}
        onError={() => setImgError(true)}
      />
    )
  }

  return (
    <div
      className={className}
      onClick={onClick}
      style={{ position: 'relative', overflow: 'hidden' }}
    >
      <CoverPlaceholder type={type} is_anime={is_anime} iconSize={iconSize} />
    </div>
  )
}
