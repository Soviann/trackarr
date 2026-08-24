// Country display helpers. Uses the browser-native Intl.DisplayNames for names
// (no bundled country table) and regional-indicator code points for flags.
const regionNames = new Intl.DisplayNames(['en'], { type: 'region' })

// Currently-assigned ISO 3166-1 alpha-2 codes. A regional-indicator pair only
// renders as a real flag glyph for these. Historic / exceptional codes that
// TMDB sometimes emits (e.g. SU = USSR, XC, YU) have no flag and no useful
// meaning to the user, so callers treat them as "not a country" and hide them
// rather than show a broken flag.
const VALID_ISO = new Set(
  ('AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ ' +
   'CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR ' +
   'GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP ' +
   'KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ ' +
   'NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW ' +
   'SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ ' +
   'UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW').split(' '),
)

// isRealCountry reports whether a code is a currently-assigned ISO 3166-1
// alpha-2 country (i.e. one with a real flag). Used to filter out the
// historic/non-standard origin codes some titles carry.
export function isRealCountry(iso: string): boolean {
  return VALID_ISO.has(iso.trim().toUpperCase())
}

export function countryFlag(iso: string): string {
  const code = iso.trim().toUpperCase()
  if (code.length !== 2 || !VALID_ISO.has(code)) return ''
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
