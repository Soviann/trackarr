declare module 'node:fs' {
  export interface Dirent {
    name: string
    isDirectory(): boolean
  }
  export function readdirSync(path: string, options?: { withFileTypes: boolean }): Dirent[]
  export function readFileSync(path: string, encoding: string): string
}

declare module 'node:path' {
  export function join(...paths: string[]): string
  export function resolve(...paths: string[]): string
  export function dirname(path: string): string
}

declare const __dirname: string
