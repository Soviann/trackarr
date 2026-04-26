// Vault palette — mirrors tokens.css. Prefer CSS variables; this file is
// for components that need colors as JS values (inline SVG strokes, Navbar,
// occasional inline styles).
export const colors = {
  bg: '#1a1714',
  bgElev: '#25201a',
  bgElev2: '#312a22',

  ink: '#ebe5d9',
  inkDim: '#a09484',
  inkMute: '#6a5e50',

  border: '#2c2620',
  borderStrong: '#3a3127',

  accent: '#d4ad7a',
  accentDim: '#7a6044',

  brandImdb: '#F5C518',
  brandTmdb: '#01b4e4',
  brandAnilist: '#02A9FF',

  statusOk: '#4CAF50',
  statusWarn: '#E8A925',
  statusCrit: '#EB5757',
} as const

export const accentWash = (hex: string) => `${hex}1A` // ~10% opacity

export const space = {
  xs: '4px', sm: '8px', md: '12px', lg: '16px', xl: '24px', xxl: '32px',
} as const

export const radius = {
  card: '14px',
  poster: '10px',
  pill: '999px',
  drawer: '22px',
  button: '12px',
} as const

export const fontSize = {
  xs: '11px', sm: '13px', md: '15px', lg: '18px', xl: '22px',
} as const
