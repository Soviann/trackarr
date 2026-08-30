#!/usr/bin/env node
import { spawnSync } from 'node:child_process'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const frontendRoot = path.resolve(__dirname, '..')

console.log('🔍 Auditing frontend codebase for hardcoded French & i18n parity...')

const result = spawnSync('npx', ['vitest', 'run', 'src/i18n/i18n-audit.test.ts'], {
  cwd: frontendRoot,
  stdio: 'inherit',
  env: { ...process.env, VITE_CONFIG_NATIVE_IGNORE_WARNING: 'true' },
})

if (result.status !== 0) {
  console.error('\n❌ i18n audit failed! Please resolve hardcoded French text/comments or translation key mismatches.')
  process.exit(result.status ?? 1)
}

console.log('✅ i18n audit passed: zero hardcoded French strings/comments and full dictionary parity.')
process.exit(0)
