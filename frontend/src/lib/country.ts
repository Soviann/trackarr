// Country display helpers. Uses the browser-native Intl.DisplayNames for names
// (no bundled country table) and regional-indicator code points for flags.
const regionNames = new Intl.DisplayNames(['en'], { type: 'region' })

export function countryFlag(iso: string): string {
  const code = iso.trim().toUpperCase()
  if (code.length !== 2) return ''
  const A = 0x1f1e6
  return String.fromCodePoint(A + (code.charCodeAt(0) - 65), A + (code.charCodeAt(1) - 65))
}

export function countryName(iso: string): string {
  const code = iso.trim().toUpperCase()
  try {
    return regionNames.of(code) ?? code
  } catch {
    return code
  }
}

export function countryLabel(iso: string): string {
  const flag = countryFlag(iso)
  return flag ? `${flag} ${countryName(iso)}` : countryName(iso)
}
