import { useEffect, useState } from 'preact/hooks'
import type { AniListSearchResult, Title } from '../types'
import { apiFetch } from '../api'
import { getName } from '../utils'
import { BottomSheet } from './BottomSheet'
import s from './RematchSheet.module.css'

interface RematchSheetProps {
  open: boolean
  onClose: () => void
  title: Title
  seasonID?: number
  onDone: () => void
}

interface TMDBResult {
  id: number
  title: string
  year: number
  poster_url: string | null
}

function extractAniListID(input: string): string {
  const trimmed = input.trim()
  const match = trimmed.match(/anilist\.co\/anime\/(\d+)/i)
  if (match) return match[1]
  return trimmed
}

export function RematchSheet({ open, onClose, title, seasonID, onDone }: RematchSheetProps) {
  const season = seasonID != null ? (title.seasons ?? []).find((s) => s.id === seasonID) : undefined
  const [query, setQuery] = useState(getName(title))
  const [mediaType, setMediaType] = useState<'movie' | 'tv'>(title.type === 'movie' ? 'movie' : 'tv')
  const [results, setResults] = useState<TMDBResult[]>([])
  const [hasSearched, setHasSearched] = useState(false)
  const [searching, setSearching] = useState(false)
  const [saving, setSaving] = useState(false)
  const [showManual, setShowManual] = useState(false)
  const [manualTmdb, setManualTmdb] = useState('')
  const [manualImdb, setManualImdb] = useState('')
  const [manualAnilist, setManualAnilist] = useState('')
  const [manualTvdb, setManualTvdb] = useState('')
  const [autoFill, setAutoFill] = useState(false)
  const [seasonAniListID, setSeasonAniListID] = useState('')
  const [anilistQuery, setAnilistQuery] = useState(getName(title))
  const [anilistResults, setAnilistResults] = useState<AniListSearchResult[]>([])
  const [searchingAniList, setSearchingAniList] = useState(false)
  const [hasSearchedAniList, setHasSearchedAniList] = useState(false)
  const [showManualSeason, setShowManualSeason] = useState(false)

  // For a series the on-screen AniList link is driven by a season, not
  // the title row — so the manual editor edits that season's mapping (prefer
  // S1, else the first season). Movies use the title row.
  const anilistSeason =
    title.type !== 'movie'
      ? ((title.seasons ?? []).find((sn) => sn.season_number === 1) ?? (title.seasons ?? [])[0])
      : undefined

  // Resynchronize season state whenever the season changes or sheet opens.
  useEffect(() => {
    setSeasonAniListID('')
  }, [seasonID])

  // Prefill fields and perform initial search each time the sheet opens
  useEffect(() => {
    if (!open) return
    if (seasonID != null) {
      const initialQuery = getName(title)
      setAnilistQuery(initialQuery)
      setSeasonAniListID('')
      setShowManualSeason(false)
      doAniListSearch(initialQuery)
      return
    }
    setManualTmdb(title.tmdb_id != null ? String(title.tmdb_id) : '')
    setManualImdb(title.imdb_id ?? '')
    setManualTvdb(title.tvdb_id != null ? String(title.tvdb_id) : '')
    const titleAniList = title.anilist_id != null ? String(title.anilist_id) : ''
    // For a series, prefer the season mapping but fall back to the title
    // row, so an existing (invisible) title-level ID surfaces and can migrate to
    // the season on save.
    setManualAnilist(anilistSeason ? (anilistSeason.anilist_id ?? titleAniList) : titleAniList)
    setAutoFill(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, title.id, seasonID])

  const doAniListSearch = async (q: string) => {
    if (!q.trim()) {
      setAnilistResults([])
      return
    }
    setSearchingAniList(true)
    setHasSearchedAniList(true)
    try {
      const data = await apiFetch<AniListSearchResult[]>(`/anilist/search?query=${encodeURIComponent(q.trim())}`)
      setAnilistResults(data)
    } catch {
      setAnilistResults([])
    } finally {
      setSearchingAniList(false)
    }
  }

  const handleSelectAniListResult = async (res: AniListSearchResult) => {
    if (seasonID == null) return
    setSaving(true)
    try {
      await apiFetch(`/titles/${title.id}/seasons/${seasonID}/anilist`, {
        method: 'POST',
        body: JSON.stringify({ anilist_id: String(res.id) }),
      })
      onDone()
      onClose()
    } catch (err) {
      console.error('Failed to link AniList anime:', err)
    } finally {
      setSaving(false)
    }
  }

  const doSearch = async (q: string, type: 'movie' | 'tv') => {
    if (!q.trim()) {
      setResults([])
      return
    }
    setSearching(true)
    setHasSearched(true)
    try {
      const data = await apiFetch<TMDBResult[]>(`/tmdb/search?query=${encodeURIComponent(q)}&type=${type}`)
      setResults(data)
    } catch {
      setResults([])
    } finally {
      setSearching(false)
    }
  }

  const handleSubmit = () => doSearch(query, mediaType)

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleSubmit()
    }
  }

  const handleTypeChange = (type: 'movie' | 'tv') => {
    setMediaType(type)
    doSearch(query, type)
  }

  const handleSelect = async (result: TMDBResult) => {
    setSaving(true)
    try {
      await apiFetch(`/titles/${title.id}/rematch`, {
        method: 'POST',
        body: JSON.stringify({ tmdb_id: result.id }),
      })
      onDone()
      onClose()
    } finally {
      setSaving(false)
    }
  }

  const handleManualSave = async () => {
    // Authoritative snapshot: every field is sent, empty = clear the ID. The
    // server locks what's filled in; auto_fill lets it back-fill the blanks.
    setSaving(true)
    try {
      await apiFetch(`/titles/${title.id}/external-ids`, {
        method: 'PUT',
        body: JSON.stringify({
          tmdb_id: manualTmdb.trim(),
          imdb_id: manualImdb.trim(),
          anilist_id: extractAniListID(manualAnilist),
          tvdb_id: manualTvdb.trim(),
          anilist_season_id: anilistSeason ? anilistSeason.id : null,
          auto_fill: autoFill,
        }),
      })
      onDone()
      onClose()
    } finally {
      setSaving(false)
    }
  }

  const handleAddPart = async () => {
    const cleanID = extractAniListID(seasonAniListID)
    if (seasonID == null || !cleanID) return
    setSaving(true)
    try {
      await apiFetch(`/titles/${title.id}/seasons/${seasonID}/anilist`, {
        method: 'POST',
        body: JSON.stringify({ anilist_id: cleanID }),
      })
      setSeasonAniListID('')
      onDone()
      onClose()
    } catch (err) {
      console.error('Failed to add AniList part:', err)
    } finally {
      setSaving(false)
    }
  }

  const handleReorder = async (index: number, direction: -1 | 1) => {
    if (seasonID == null) return
    const parts = season?.anilist_parts ?? []
    const target = index + direction
    if (target < 0 || target >= parts.length) return
    const ids = parts.map((p) => p.external_id)
    ;[ids[index], ids[target]] = [ids[target], ids[index]]
    setSaving(true)
    try {
      await apiFetch(`/titles/${title.id}/seasons/${seasonID}/anilist/order`, {
        method: 'PUT',
        body: JSON.stringify({ ordered_ids: ids }),
      })
      onDone()
    } catch (err) {
      console.error('Failed to reorder AniList parts:', err)
    } finally {
      setSaving(false)
    }
  }

  const handleRemovePart = async (externalID: string) => {
    if (seasonID == null) return
    setSaving(true)
    try {
      await apiFetch(`/titles/${title.id}/seasons/${seasonID}/anilist/${encodeURIComponent(externalID)}`, {
        method: 'DELETE',
      })
      onDone()
    } catch (err) {
      console.error('Failed to remove AniList part:', err)
    } finally {
      setSaving(false)
    }
  }

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel={seasonID != null ? 'Link AniList season' : 'Rematch title'}>
      <div className={s.content}>
        {seasonID != null ? (
          <>
            <div className={s.status}>AniList for S{season?.season_number ?? '?'}</div>

            {/* Search row */}
            <div className={s.searchRow}>
              <input
                type="text"
                value={anilistQuery}
                onInput={(e) => setAnilistQuery((e.target as HTMLInputElement).value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    doAniListSearch(anilistQuery)
                  }
                }}
                placeholder="Search AniList..."
                className={s.searchInput}
                autoFocus
              />
              <button
                onClick={() => doAniListSearch(anilistQuery)}
                disabled={searchingAniList}
                className={s.searchBtn}
                aria-label="Search AniList"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="11" cy="11" r="8" />
                  <line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
              </button>
            </div>

            {/* Browser search shortcut */}
            <div className={s.externalSearchRow}>
              <span className={s.externalSearchPrompt}>Or search directly on AniList:</span>
              <a
                href={`https://anilist.co/search/anime?search=${encodeURIComponent(anilistQuery.trim() || getName(title))}`}
                target="_blank"
                rel="noopener noreferrer"
                className={s.externalSearchLink}
              >
                Search on AniList.co ↗
              </a>
            </div>

            {/* Results */}
            <div className={s.results}>
              {searchingAniList && <div className={s.status}>Searching AniList...</div>}
              {!searchingAniList && anilistResults.length === 0 && hasSearchedAniList && (
                <div className={s.status}>No AniList results</div>
              )}
              {anilistResults.map((r) => (
                <button
                  key={r.id}
                  onClick={() => handleSelectAniListResult(r)}
                  disabled={saving}
                  className={s.resultCard}
                >
                  <div className={s.poster}>
                    {r.poster_url ? (
                      <img src={r.poster_url} alt={r.title} loading="lazy" />
                    ) : (
                      <div className={s.noPoster}>?</div>
                    )}
                  </div>
                  <div className={s.resultInfo}>
                    <div className={s.resultTitle}>{r.title}</div>
                    {r.romaji_title && r.romaji_title !== r.title && (
                      <div className={s.resultRomaji}>{r.romaji_title}</div>
                    )}
                    <div className={s.resultMeta}>
                      {r.format && <span className={s.formatBadge}>{r.format}</span>}
                      {r.year != null && <span>{r.year}</span>}
                      {r.episodes != null && <span>{r.episodes} eps</span>}
                      <span className={s.resultID}>#{r.id}</span>
                    </div>
                  </div>
                </button>
              ))}
            </div>

            {/* Current mapped parts section if any exist */}
            {(season?.anilist_parts ?? []).length > 0 && (
              <div className={s.currentPartsSection}>
                <div className={s.sectionHeader}>Current parts ({season?.anilist_parts?.length})</div>
                {(season?.anilist_parts ?? []).map((p, i) => {
                  const parts = season?.anilist_parts ?? []
                  return (
                    <div key={p.external_id} className={s.partManageRow}>
                      <span>Part {i + 1}: {p.external_id}{p.score != null ? ` · ${p.score}%` : ''}</span>
                      <span className={s.partManageActions}>
                        {parts.length > 1 && (
                          <span className={s.reorderControls}>
                            <button onClick={() => handleReorder(i, -1)} disabled={saving || i === 0} aria-label={`Move part ${i + 1} up`}>▲</button>
                            <button onClick={() => handleReorder(i, 1)} disabled={saving || i === parts.length - 1} aria-label={`Move part ${i + 1} down`}>▼</button>
                          </span>
                        )}
                        <button onClick={() => handleRemovePart(p.external_id)} disabled={saving} className={s.removeMapping}>
                          Remove
                        </button>
                      </span>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Manual ID toggle */}
            <button onClick={() => setShowManualSeason(!showManualSeason)} className={s.manualToggle}>
              {showManualSeason ? '▾' : '▸'} Manual AniList ID
            </button>

            {showManualSeason && (
              <div className={s.manualSection}>
                <label className={s.fieldLabel}>
                  Add AniList ID or URL
                  <input
                    type="text"
                    value={seasonAniListID}
                    onInput={(e) => setSeasonAniListID((e.target as HTMLInputElement).value)}
                    className={s.fieldInput}
                    placeholder="e.g. 26 or https://anilist.co/anime/26"
                  />
                </label>
                <div className={s.manualActions}>
                  <button onClick={handleAddPart} disabled={saving || !seasonAniListID.trim()} className={s.saveButton}>
                    <span className={s.saveButtonLabel}>{saving ? 'Saving...' : 'Add'}</span>
                  </button>
                </div>
              </div>
            )}
          </>
        ) : ( /* title-mode unchanged */
          /* Title-mode: existing TMDB search + manual IDs UI */
          <>
            {/* Search bar */}
            <div className={s.searchRow}>
              <input
                type="text"
                value={query}
                onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
                onKeyDown={handleKeyDown}
                placeholder="Search TMDB..."
                className={s.searchInput}
                autoFocus
              />
              <button onClick={handleSubmit} disabled={searching} className={s.searchBtn} aria-label="Search">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="11" cy="11" r="8" />
                  <line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
              </button>
            </div>

            {/* Type toggle */}
            <div className={s.typeToggle}>
              {(['movie', 'tv'] as const).map((t) => (
                <button
                  key={t}
                  onClick={() => handleTypeChange(t)}
                  className={`${s.typeBtn} ${mediaType === t ? s.active : ''}`}
                >
                  {t === 'movie' ? 'Movie' : 'TV'}
                </button>
              ))}
            </div>

            {/* Results grid */}
            <div className={s.results}>
              {searching && <div className={s.status}>Searching...</div>}
              {!searching && results.length === 0 && hasSearched && (
                <div className={s.status}>No results</div>
              )}
              {results.map((r) => (
                <button
                  key={r.id}
                  onClick={() => handleSelect(r)}
                  disabled={saving}
                  className={s.resultCard}
                >
                  <div className={s.poster}>
                    {r.poster_url ? (
                      <img src={r.poster_url} alt={r.title} loading="lazy" />
                    ) : (
                      <div className={s.noPoster}>?</div>
                    )}
                  </div>
                  <div className={s.resultInfo}>
                    <div className={s.resultTitle}>{r.title}</div>
                    {r.year > 0 && <div className={s.resultYear}>{r.year}</div>}
                  </div>
                </button>
              ))}
            </div>

            {/* Manual IDs section */}
            <button onClick={() => setShowManual(!showManual)} className={s.manualToggle}>
              {showManual ? '▾' : '▸'} Manual IDs
            </button>

            {showManual && (
              <div className={s.manualSection}>
                <label className={s.fieldLabel}>
                  TMDB ID
                  <input type="text" value={manualTmdb} onInput={(e) => setManualTmdb((e.target as HTMLInputElement).value)} className={s.fieldInput} placeholder="e.g. 550" />
                </label>
                <label className={s.fieldLabel}>
                  IMDB ID
                  <input type="text" value={manualImdb} onInput={(e) => setManualImdb((e.target as HTMLInputElement).value)} className={s.fieldInput} placeholder="e.g. tt0137523" />
                </label>
                <label className={s.fieldLabel}>
                  {anilistSeason ? `AniList ID (S${anilistSeason.season_number})` : 'AniList ID'}
                  <input type="text" value={manualAnilist} onInput={(e) => setManualAnilist((e.target as HTMLInputElement).value)} className={s.fieldInput} placeholder="e.g. 21" />
                </label>
                <label className={s.fieldLabel}>
                  TVDB ID
                  <input type="text" value={manualTvdb} onInput={(e) => setManualTvdb((e.target as HTMLInputElement).value)} className={s.fieldInput} placeholder="e.g. 81189" />
                </label>
                <label className={s.autoFillRow}>
                  <input type="checkbox" checked={autoFill} onChange={(e) => setAutoFill((e.target as HTMLInputElement).checked)} />
                  <span>Auto-find the other IDs</span>
                </label>
                <p className={s.autoFillHint}>
                  Leave blank to remove an ID. Tick the box to let matching fill the empty ones.
                </p>
                <button onClick={handleManualSave} disabled={saving} className={s.saveButton}>
                  <span className={s.saveButtonLabel}>{saving ? 'Saving...' : 'Save'}</span>
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </BottomSheet>
  )
}
