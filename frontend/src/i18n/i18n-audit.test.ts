import { describe, it, expect } from 'vitest'
import * as path from 'node:path'
import {
  scanAllSourceFiles,
  checkDictionaryParity,
  scanSourceFile,
} from './checker'

describe('i18n hardcoded text & comments audit', () => {
  it('detects French JSX text, French attributes, French literals and French comments in test samples', () => {
    const badCode = `
      // Ce commentaire est en français
      export function BadComponent() {
        return (
          <div title="Détails du média">
            <p>Ajouter à la liste</p>
            <input placeholder="Rechercher..." />
          </div>
        )
      }
    `
    const issues = scanSourceFile('/src/components/BadComponent.tsx', badCode)
    expect(issues.length).toBeGreaterThanOrEqual(3)
    expect(issues.some((i) => i.kind === 'accent' || i.kind === 'french-word')).toBe(true)
  })

  it('respects inline i18n-ignore annotations', () => {
    const ignoredCode = `
      // Timothée Chalamet // i18n-ignore
      export function IgnoredComponent() {
        return (
          <div>
            <p>Timothée Chalamet</p> {/* i18n-ignore */}
          </div>
        )
      }
    `
    const issues = scanSourceFile('/src/components/Ignored.tsx', ignoredCode)
    expect(issues).toEqual([])
  })

  it('finds no hardcoded French strings or comments across all frontend source files', () => {
    const srcDir = path.resolve(__dirname, '..')
    const issues = scanAllSourceFiles(srcDir)

    if (issues.length > 0) {
      const formatted = issues
        .map(
          (i) =>
            `\n  ❌ [${i.kind}] ${i.filePath}:${i.line}:${i.col}\n     ${i.message}\n     Snippet: "${i.snippet}"`
        )
        .join('')
      expect(issues, `Found ${issues.length} hardcoded French issues:${formatted}`).toEqual([])
    } else {
      expect(issues).toEqual([])
    }
  })
})

describe('i18n translation dictionary parity', () => {
  it('has exact key and parameter parity between English and French dictionaries', () => {
    const issues = checkDictionaryParity()

    if (issues.length > 0) {
      const formatted = issues
        .map((i) => `\n  ❌ [${i.kind}] ${i.message}`)
        .join('')
      expect(issues, `Found ${issues.length} dictionary discrepancies:${formatted}`).toEqual([])
    } else {
      expect(issues).toEqual([])
    }
  })
})
