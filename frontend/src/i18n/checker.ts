import * as fs from 'node:fs'
import * as path from 'node:path'
import { en } from './locales/en'
import { fr } from './locales/fr'

export interface I18nIssue {
  filePath: string
  line: number
  col: number
  kind: 'accent' | 'french-word' | 'missing-key' | 'extra-key' | 'param-mismatch'
  message: string
  snippet: string
}

// French diacritics and typography regex
const FRENCH_ACCENT_REGEX = /[éèêëàâîïôùûüçÉÈÊËÀÂÎÏÔÙÛÜÇœæ«»’]/

// Common French indicator words (matched with word boundaries)
const FRENCH_WORDS_REGEX = /\b(dans|avec|sans|aucun|aucune|supprimer|modifier|fermer|annuler|sauvegarder|enregistrer|suivant|suivante|suivants|suivantes|precedent|precedente|precedents|precedentes|recherche|rechercher|ajouter|bienvenue|erreur|chargement|connexion|deconnexion|saison|saisons|telecharger|parametres|bibliotheque|historique|selectionner|reinitialiser|mot de passe|identifiant|aujourd'hui|hier)\b/i

const UI_ATTR_REGEX = /\b(placeholder|title|aria-label|aria-placeholder|aria-description|alt|label)=["']([^"']+)["']/g

function isIgnoredLine(lineContent: string): boolean {
  return lineContent.includes('i18n-ignore')
}

export function scanSourceFile(filePath: string, code: string): I18nIssue[] {
  const issues: I18nIssue[] = []
  const lines = code.split('\n')

  if (code.includes('/* i18n-ignore-file */') || code.includes('// i18n-ignore-file')) {
    return issues
  }

  let inBlockComment = false

  lines.forEach((lineText, idx) => {
    const lineNum = idx + 1
    if (isIgnoredLine(lineText)) return

    const trimmed = lineText.trim()

    // 1. Block comments /* ... */
    if (inBlockComment) {
      if (trimmed.includes('*/')) {
        inBlockComment = false
      }
      if (FRENCH_ACCENT_REGEX.test(trimmed)) {
        issues.push({
          filePath,
          line: lineNum,
          col: 1,
          kind: 'accent',
          message: 'Found French accent in block comment',
          snippet: trimmed,
        })
      } else if (FRENCH_WORDS_REGEX.test(trimmed)) {
        issues.push({
          filePath,
          line: lineNum,
          col: 1,
          kind: 'french-word',
          message: 'Found French keyword in block comment',
          snippet: trimmed,
        })
      }
      return
    }

    if (trimmed.startsWith('/*')) {
      if (!trimmed.includes('*/')) {
        inBlockComment = true
      }
      if (FRENCH_ACCENT_REGEX.test(trimmed)) {
        issues.push({
          filePath,
          line: lineNum,
          col: 1,
          kind: 'accent',
          message: 'Found French accent in comment',
          snippet: trimmed,
        })
      } else if (FRENCH_WORDS_REGEX.test(trimmed)) {
        issues.push({
          filePath,
          line: lineNum,
          col: 1,
          kind: 'french-word',
          message: 'Found French keyword in comment',
          snippet: trimmed,
        })
      }
      return
    }

    // 2. Single-line comment
    if (trimmed.startsWith('//')) {
      if (FRENCH_ACCENT_REGEX.test(trimmed)) {
        issues.push({
          filePath,
          line: lineNum,
          col: 1,
          kind: 'accent',
          message: 'Found French accent in comment',
          snippet: trimmed,
        })
      } else if (FRENCH_WORDS_REGEX.test(trimmed)) {
        issues.push({
          filePath,
          line: lineNum,
          col: 1,
          kind: 'french-word',
          message: 'Found French keyword in comment',
          snippet: trimmed,
        })
      }
      return
    }

    // Ignore import/export lines
    if (trimmed.startsWith('import ') || trimmed.startsWith('export * from') || trimmed.startsWith('export {')) {
      return
    }

    // 3. UI Attributes check
    let attrMatch: RegExpExecArray | null
    const attrRegex = new RegExp(UI_ATTR_REGEX.source, 'g')
    while ((attrMatch = attrRegex.exec(lineText)) !== null) {
      const attrName = attrMatch[1]
      const attrVal = attrMatch[2]
      if (FRENCH_ACCENT_REGEX.test(attrVal)) {
        issues.push({
          filePath,
          line: lineNum,
          col: attrMatch.index + 1,
          kind: 'accent',
          message: `JSX attribute '${attrName}' contains French accent`,
          snippet: attrMatch[0],
        })
      } else if (FRENCH_WORDS_REGEX.test(attrVal)) {
        issues.push({
          filePath,
          line: lineNum,
          col: attrMatch.index + 1,
          kind: 'french-word',
          message: `JSX attribute '${attrName}' contains hardcoded French keyword`,
          snippet: attrMatch[0],
        })
      }
    }

    // 4. JSX Text: >...<
    const jsxTextMatches = lineText.match(/>([^<>{}]*[a-zA-Z\u00C0-\u017F][^<>{}]*)</g)
    if (jsxTextMatches) {
      for (const m of jsxTextMatches) {
        const text = m.slice(1, -1).trim()
        if (text.length > 0) {
          if (FRENCH_ACCENT_REGEX.test(text)) {
            issues.push({
              filePath,
              line: lineNum,
              col: lineText.indexOf(m) + 1,
              kind: 'accent',
              message: 'JSX text contains French accent',
              snippet: text,
            })
          } else if (FRENCH_WORDS_REGEX.test(text)) {
            issues.push({
              filePath,
              line: lineNum,
              col: lineText.indexOf(m) + 1,
              kind: 'french-word',
              message: 'JSX text contains hardcoded French keyword',
              snippet: text,
            })
          }
        }
      }
    }

    // 5. String Literals
    const strMatches = lineText.match(/(['"`])((?:\\.|(?!\1)[^\\])*)\1/g)
    if (strMatches) {
      for (const rawStr of strMatches) {
        const quote = rawStr[0]
        const strContent = rawStr.slice(1, -1).trim()
        if (
          strContent.length === 0 ||
          strContent.startsWith('http://') ||
          strContent.startsWith('https://') ||
          strContent.startsWith('/') ||
          strContent.startsWith('./') ||
          strContent.startsWith('../') ||
          strContent.startsWith('data:') ||
          (strContent.startsWith('M') && strContent.includes(' ') && /[0-9]/.test(strContent))
        ) {
          continue
        }

        if (FRENCH_ACCENT_REGEX.test(strContent)) {
          // Avoid duplicate report if already captured by attribute
          if (!issues.some((i) => i.filePath === filePath && i.line === lineNum && i.kind === 'accent')) {
            issues.push({
              filePath,
              line: lineNum,
              col: lineText.indexOf(rawStr) + 1,
              kind: 'accent',
              message: 'String literal contains French accent',
              snippet: rawStr,
            })
          }
        } else if (FRENCH_WORDS_REGEX.test(strContent)) {
          if (!issues.some((i) => i.filePath === filePath && i.line === lineNum && i.kind === 'french-word')) {
            issues.push({
              filePath,
              line: lineNum,
              col: lineText.indexOf(rawStr) + 1,
              kind: 'french-word',
              message: 'String literal contains hardcoded French keyword',
              snippet: rawStr,
            })
          }
        }
      }
    }
  })

  return issues
}

export function getAllSourceFiles(dir: string): string[] {
  const files: string[] = []
  const entries = fs.readdirSync(dir, { withFileTypes: true })

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name !== 'node_modules' && entry.name !== 'dist') {
        files.push(...getAllSourceFiles(fullPath))
      }
    } else if (
      (entry.name.endsWith('.ts') || entry.name.endsWith('.tsx')) &&
      !entry.name.endsWith('.d.ts')
    ) {
      // Exclude the French dictionary file itself and the audit test file
      if (
        !fullPath.endsWith(path.join('i18n', 'locales', 'fr.ts')) &&
        !fullPath.endsWith(path.join('i18n', 'i18n-audit.test.ts'))
      ) {
        files.push(fullPath)
      }
    }
  }

  return files
}

export function scanAllSourceFiles(srcDir: string): I18nIssue[] {
  const files = getAllSourceFiles(srcDir)
  const allIssues: I18nIssue[] = []

  for (const file of files) {
    const code = fs.readFileSync(file, 'utf-8')
    const issues = scanSourceFile(file, code)
    allIssues.push(...issues)
  }

  return allIssues
}

export function checkDictionaryParity(): I18nIssue[] {
  const issues: I18nIssue[] = []

  function extractKeysAndParams(
    obj: Record<string, any>,
    prefix = ''
  ): Map<string, string[]> {
    const result = new Map<string, string[]>()
    for (const [key, value] of Object.entries(obj)) {
      const fullKey = prefix ? `${prefix}.${key}` : key
      if (value && typeof value === 'object' && !Array.isArray(value)) {
        const nested = extractKeysAndParams(value, fullKey)
        nested.forEach((params, k) => result.set(k, params))
      } else if (typeof value === 'string') {
        const params = Array.from(new Set(value.match(/\{(\w+)\}/g) || [])).sort()
        result.set(fullKey, params)
      }
    }
    return result
  }

  const enMap = extractKeysAndParams(en)
  const frMap = extractKeysAndParams(fr)

  // 1. Missing in French
  for (const [key, enParams] of enMap.entries()) {
    if (!frMap.has(key)) {
      issues.push({
        filePath: 'frontend/src/i18n/locales/fr.ts',
        line: 1,
        col: 1,
        kind: 'missing-key',
        message: `Missing translation key in French dictionary: '${key}'`,
        snippet: key,
      })
    } else {
      const frParams = frMap.get(key) || []
      if (JSON.stringify(enParams) !== JSON.stringify(frParams)) {
        issues.push({
          filePath: 'frontend/src/i18n/locales/fr.ts',
          line: 1,
          col: 1,
          kind: 'param-mismatch',
          message: `Interpolation parameter mismatch for key '${key}': EN expects ${JSON.stringify(
            enParams
          )}, FR has ${JSON.stringify(frParams)}`,
          snippet: key,
        })
      }
    }
  }

  // 2. Extra orphan keys in French
  for (const key of frMap.keys()) {
    if (!enMap.has(key)) {
      issues.push({
        filePath: 'frontend/src/i18n/locales/fr.ts',
        line: 1,
        col: 1,
        kind: 'extra-key',
        message: `Unknown/Orphan translation key in French dictionary: '${key}'`,
        snippet: key,
      })
    }
  }

  return issues
}
