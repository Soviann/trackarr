export const colors = {
  bgPrimary: '#0D0D0D',
  bgCard: '#161616',
  bgSurface: '#1E1E1E',
  borderSubtle: '#1A1A1A',
  borderCard: '#222222',
  textPrimary: '#F0F0F0',
  textSecondary: '#666666',
  textMuted: '#555555',
  textDimmed: '#444444',

  accentAmber: '#E8A925',
  accentTeal: '#38BDB0',
  accentGreen: '#4CAF50',
  accentLavender: '#9575CD',
  accentCoral: '#EB5757',
  accentBlue: '#5B9CF6',
  accentImdb: '#F5C518',
  accentAnilist: '#02A9FF',
} as const

export const accentWash = (hex: string) => `${hex}1F` // ~12% opacity
