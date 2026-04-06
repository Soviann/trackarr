export const colors = {
  bgPrimary: '#0D0D0D',
  bgCard: '#161616',
  bgSurface: '#1E1E1E',
  borderSubtle: '#1A1A1A',
  borderCard: '#222222',
  textPrimary: '#F0F0F0',
  textSecondary: '#E0E0E0',
  textMuted: '#D0D0D0',
  textDimmed: '#C0C0C0',

  accentAmber: '#E8A925',
  accentTeal: '#38BDB0',
  accentGreen: '#4CAF50',
  accentLavender: '#00F2FF',
  accentCoral: '#EB5757',
  accentBlue: '#5B9CF6',
  accentImdb: '#F5C518',
  accentAnilist: '#02A9FF',
} as const

export const accentWash = (hex: string) => `${hex}1F` // ~12% opacity

// Spacing scale
export const space = {
  xs: '4px',
  sm: '8px',
  md: '12px',
  lg: '16px',
  xl: '24px',
  xxl: '32px',
} as const

// Border radii
export const radius = {
  sm: '6px',
  md: '10px',
  lg: '16px',
  full: '9999px',
} as const

// Font sizes
export const fontSize = {
  xs: '11px',
  sm: '13px',
  md: '15px',
  lg: '18px',
  xl: '22px',
} as const
