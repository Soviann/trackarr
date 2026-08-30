export type Locale = 'en' | 'fr'

export interface LocaleOption {
  id: Locale
  name: string
  nativeName: string
  flag: string
}

export const LOCALES: LocaleOption[] = [
  {
    id: 'en',
    name: 'English',
    nativeName: 'English',
    flag: '🇬🇧',
  },
  {
    id: 'fr',
    name: 'French',
    nativeName: 'Français', // i18n-ignore
    flag: '🇫🇷',
  },
]
