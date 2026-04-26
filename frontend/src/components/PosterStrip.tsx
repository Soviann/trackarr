import { PosterTile, type PosterTileItem } from './PosterTile'
import s from './PosterStrip.module.css'

interface Props {
  items: PosterTileItem[]
}

export function PosterStrip({ items }: Props) {
  return (
    <div className={s.strip}>
      {items.map(item => (
        <div key={item.id} className={s.slot}>
          <PosterTile item={item} />
        </div>
      ))}
    </div>
  )
}
