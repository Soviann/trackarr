// URL classification helpers shared by the add / validate / share-target flows.
// Media share links (notably IMDb on Android) append tracking query strings such
// as `?ref_=ext_shr` and may carry a locale segment (`/fr/title/...`); both must
// be tolerated or the URL gets mistaken for a plain title name.

// isUrl reports whether a string looks like a web URL. Accepts an optional
// scheme, a path, and — crucially — an optional `?query` and `#fragment`, so a
// shared `https://www.imdb.com/fr/title/tt31974288/?ref_=ext_shr` is recognised
// as a URL rather than treated as a title to search/add verbatim.
export function isUrl(str: string): boolean {
  return /^(https?:\/\/)?([\w.-]+)+\.([a-z]{2,10})(\/[\w.-]*)*\/?(\?[^#\s]*)?(#\S*)?$/i.test(str)
}

// detectUrlType classifies a supported media URL for the input-bar hint. The
// IMDb pattern allows an optional two-letter locale segment (`/fr/`) the way the
// backend parser does, so a shared localized link is still detected.
export function detectUrlType(input: string): string | null {
  if (/imdb\.com\/(?:[a-z]{2}\/)?title\/(tt\d+)/i.test(input)) return 'imdb'
  if (/thetvdb\.com/i.test(input)) return 'tvdb'
  if (/anilist\.co\/anime\/(\d+)/i.test(input)) return 'anilist'
  return null
}
