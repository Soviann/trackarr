const BASE = '/api'

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

/**
 * Fetch wrapper for backend API calls.
 *
 * The `BASE` prefix `/api` is prepended automatically — pass paths like
 * `/titles/123`, NOT `/api/titles/123` (would produce `/api/api/...` and 404).
 * A dev-only `console.error` flags violations on first call.
 *
 * Use raw `fetch` with the full `/api/...` URL for pre-auth flows (config,
 * error reporting, OAuth init) — `apiFetch` redirects on 401, which would
 * loop those callers.
 */
export async function apiFetch<T = unknown>(path: string, options?: RequestInit): Promise<T> {
  if (import.meta.env.DEV) {
    if (!path.startsWith('/')) {
      console.error(`apiFetch: path must start with '/', got "${path}"`)
    } else if (path.startsWith('/api')) {
      console.error(`apiFetch: path must NOT start with '/api' (BASE adds it). Got "${path}" — will hit /api/api/... and 404.`)
    }
  }

  const isFormData = typeof FormData !== 'undefined' && options?.body instanceof FormData
  const headers = new Headers(options?.headers)
  if (!isFormData && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const res = await fetch(`${BASE}${path}`, {
    credentials: 'same-origin',
    ...options,
    headers,
  })

  if (res.status === 401) {
    if (window.location.pathname !== '/login') {
      window.location.href = '/login'
    }
    throw new ApiError(401, 'Unauthorized')
  }

  if (!res.ok) {
    const text = await res.text()
    throw new ApiError(res.status, text)
  }

  if (res.status === 204 || res.status === 202) return undefined as T

  return res.json()
}
