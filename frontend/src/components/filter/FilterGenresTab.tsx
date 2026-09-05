import { useState, useRef } from 'preact/hooks'
import type { RefObject } from 'preact'
import clsx from 'clsx'
import type { GenreCount, CountryCount } from '../../types'
import { countryLabel, isRealCountry } from '../../lib/country'
import type { FilterState, FilterActions } from './types'
import s from '../FilterDrawer.module.css'

interface FilterGenresTabProps {
  filter: FilterState
  actions: FilterActions
  genres: GenreCount[]
  countries: CountryCount[]
  genreDropdownRef?: RefObject<HTMLDivElement>
  countryDropdownRef?: RefObject<HTMLDivElement>
}

export function FilterGenresTab({
  filter,
  actions,
  genres,
  countries,
  genreDropdownRef: externalGenreDropdownRef,
  countryDropdownRef: externalCountryDropdownRef,
}: FilterGenresTabProps) {
  const [genreSearch, setGenreSearch] = useState('')
  const [genreDropdownOpen, setGenreDropdownOpen] = useState(false)
  const genreBlurTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const genreInputRef = useRef<HTMLInputElement>(null)
  const localGenreDropdownRef = useRef<HTMLDivElement>(null)
  const genreDropdownRef = externalGenreDropdownRef ?? localGenreDropdownRef

  const [countrySearch, setCountrySearch] = useState('')
  const [countryDropdownOpen, setCountryDropdownOpen] = useState(false)
  const countryBlurTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const countryInputRef = useRef<HTMLInputElement>(null)
  const localCountryDropdownRef = useRef<HTMLDivElement>(null)
  const countryDropdownRef = externalCountryDropdownRef ?? localCountryDropdownRef

  const selectedGenres = filter.selectedGenres
  const selectedCountries = filter.selectedCountries
  const genreOp = filter.genreOp

  return (
    <div className={s.tabPane}>
      {genres.length > 0 && (() => {
        const sortedGenres = [...genres].sort((a, b) => a.genre.localeCompare(b.genre))
        const filteredGenres = sortedGenres
          .filter(g => !selectedGenres.includes(g.genre))
          .filter(g => !genreSearch || g.genre.toLowerCase().includes(genreSearch.toLowerCase()))
        return (
          <>
            <div className={clsx(s.filterLabel, s.filterLabelFirst)}>Genres</div>
            <div className={s.genreOpRow}>
              <button
                type="button"
                className={clsx(s.opBtn, genreOp === 'OR' && s.opBtnActive)}
                onClick={() => actions.onGenreOpChange('OR')}
              >Any</button>
              <button
                type="button"
                className={clsx(s.opBtn, genreOp === 'AND' && s.opBtnActive)}
                onClick={() => actions.onGenreOpChange('AND')}
              >All</button>
              {selectedGenres.length > 0 && (
                <button
                  type="button"
                  className={s.clearGenresBtn}
                  onClick={() => selectedGenres.forEach(g => actions.onGenreToggle(g))}
                >Clear</button>
              )}
            </div>
            <div className={s.genreDropdownWrapper}>
              <div
                className={s.genreAutocomplete}
                onClick={() => genreInputRef.current?.focus()}
              >
                {selectedGenres.map(g => (
                  <span key={g} className={s.genreTag}>
                    {g}
                    <button
                      type="button"
                      className={s.genreTagRemove}
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={(e) => { e.stopPropagation(); actions.onGenreToggle(g) }}
                    >&times;</button>
                  </span>
                ))}
                <input
                  ref={genreInputRef}
                  type="text"
                  className={s.genreInput}
                  placeholder={selectedGenres.length === 0 ? 'Search genres…' : ''}
                  value={genreSearch}
                  onInput={(e) => setGenreSearch((e.target as HTMLInputElement).value)}
                  onFocus={() => {
                    if (genreBlurTimeout.current) clearTimeout(genreBlurTimeout.current)
                    setGenreDropdownOpen(true)
                  }}
                  onBlur={() => {
                    genreBlurTimeout.current = setTimeout(() => setGenreDropdownOpen(false), 150)
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Escape') {
                      setGenreDropdownOpen(false)
                      ;(e.target as HTMLInputElement).blur()
                    }
                  }}
                />
              </div>
              {genreDropdownOpen && filteredGenres.length > 0 && (
                <div ref={genreDropdownRef} className={s.genreDropdown}>
                  {filteredGenres.map(g => (
                    <div
                      key={g.genre}
                      className={s.genreDropdownItem}
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={() => {
                        actions.onGenreToggle(g.genre)
                        setGenreSearch('')
                        genreInputRef.current?.focus()
                      }}
                    >
                      <span>{g.genre}</span>
                      <span className={s.genreDropdownCount}>{g.count}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )
      })()}

      {countries.some(c => isRealCountry(c.country)) && (() => {
        const q = countrySearch.toLowerCase()
        const filteredCountries = countries
          .filter(c => isRealCountry(c.country))
          .filter(c => !selectedCountries.includes(c.country))
          .filter(c => !q || countryLabel(c.country).toLowerCase().includes(q) || c.country.toLowerCase().includes(q))
        return (
          <>
            <div className={s.filterLabel}>Country</div>
            {selectedCountries.length > 0 && (
              <div className={s.genreOpRow}>
                <button
                  type="button"
                  className={s.clearGenresBtn}
                  onClick={() => selectedCountries.forEach(c => actions.onCountryToggle(c))}
                >Clear</button>
              </div>
            )}
            <div className={s.genreDropdownWrapper}>
              <div
                className={s.genreAutocomplete}
                onClick={() => countryInputRef.current?.focus()}
              >
                {selectedCountries.map(c => (
                  <span key={c} className={s.genreTag}>
                    {countryLabel(c)}
                    <button
                      type="button"
                      className={s.genreTagRemove}
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={(e) => { e.stopPropagation(); actions.onCountryToggle(c) }}
                    >&times;</button>
                  </span>
                ))}
                <input
                  ref={countryInputRef}
                  type="text"
                  className={s.genreInput}
                  placeholder={selectedCountries.length === 0 ? 'Search countries…' : ''}
                  value={countrySearch}
                  onInput={(e) => setCountrySearch((e.target as HTMLInputElement).value)}
                  onFocus={() => {
                    if (countryBlurTimeout.current) clearTimeout(countryBlurTimeout.current)
                    setCountryDropdownOpen(true)
                  }}
                  onBlur={() => {
                    countryBlurTimeout.current = setTimeout(() => setCountryDropdownOpen(false), 150)
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Escape') {
                      setCountryDropdownOpen(false)
                      ;(e.target as HTMLInputElement).blur()
                    }
                  }}
                />
              </div>
              {countryDropdownOpen && filteredCountries.length > 0 && (
                <div ref={countryDropdownRef} className={clsx(s.genreDropdown, s.dropUp)}>
                  {filteredCountries.map(c => (
                    <div
                      key={c.country}
                      className={s.genreDropdownItem}
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={() => {
                        actions.onCountryToggle(c.country)
                        setCountrySearch('')
                        countryInputRef.current?.focus()
                      }}
                    >
                      <span>{countryLabel(c.country)}</span>
                      <span className={s.genreDropdownCount}>{c.count}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )
      })()}
    </div>
  )
}
