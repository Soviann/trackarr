import { useState, useEffect } from 'preact/hooks'
import { en, type TranslationSchema } from './locales/en'
import { fr } from './locales/fr'
import { Locale, LOCALES } from './types'

export * from './types'

const LOCALE_STORAGE_KEY = 'trackarr_locale'

const dictionaries: Record<Locale, TranslationSchema> = {
  en,
  fr,
}

export function getStoredLocale(): Locale {
  if (typeof window === 'undefined') return 'en'
  const stored = localStorage.getItem(LOCALE_STORAGE_KEY) as Locale
  if (stored && (stored === 'en' || stored === 'fr')) {
    return stored
  }
  if (typeof navigator !== 'undefined' && navigator.language?.startsWith('fr')) {
    return 'fr'
  }
  return 'en'
}

let currentLocale: Locale = getStoredLocale()
const listeners = new Set<() => void>()

export function setLocale(locale: Locale): void {
  currentLocale = locale
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale)
  }
  listeners.forEach((listener) => listener())
}

export function getLocale(): Locale {
  return currentLocale
}

type NestedKeyOf<ObjectType extends object> = {
  [Key in keyof ObjectType & (string | number)]: ObjectType[Key] extends object
    ? `${Key}.${NestedKeyOf<ObjectType[Key]>}`
    : `${Key}`
}[keyof ObjectType & (string | number)]

export type TranslationKey = NestedKeyOf<TranslationSchema>

export function translate(locale: Locale, key: TranslationKey, params?: Record<string, string | number>): string {
  const dict = dictionaries[locale] || dictionaries.en
  const keys = key.split('.')
  let current: any = dict

  for (const k of keys) {
    if (current && typeof current === 'object' && k in current) {
      current = current[k]
    } else {
      // Fallback to English
      let fallback: any = dictionaries.en
      for (const fk of keys) {
        if (fallback && typeof fallback === 'object' && fk in fallback) {
          fallback = fallback[fk]
        } else {
          return key
        }
      }
      current = fallback
      break
    }
  }

  if (typeof current !== 'string') {
    return key
  }

  if (!params) return current

  return current.replace(/\{(\w+)\}/g, (_, p) => (p in params ? String(params[p]) : `{${p}}`))
}

export function useTranslation() {
  const [locale, setLocal] = useState<Locale>(currentLocale)

  useEffect(() => {
    const handler = () => setLocal(currentLocale)
    listeners.add(handler)
    return () => {
      listeners.delete(handler)
    }
  }, [])

  const t = (key: TranslationKey, params?: Record<string, string | number>) => {
    return translate(locale, key, params)
  }

  return { t, locale, setLocale, locales: LOCALES }
}
