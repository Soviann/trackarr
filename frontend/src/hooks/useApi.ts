import { useState, useEffect, useCallback } from 'preact/hooks'
import { apiFetch } from '../api'

interface UseApiResult<T> {
  data: T | null
  error: string | null
  loading: boolean
  mutate: () => void
}

export function useApi<T>(path: string | null): UseApiResult<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(path !== null)

  const load = useCallback(() => {
    if (!path) return
    const controller = new AbortController()
    setLoading(true)
    setError(null)
    apiFetch<T>(path, { signal: controller.signal })
      .then(setData)
      .catch((e) => { if (e.name !== 'AbortError') setError(e.message) })
      .finally(() => setLoading(false))
    return controller
  }, [path])

  useEffect(() => {
    const controller = load()
    return () => controller?.abort()
  }, [load])

  return { data, error, loading, mutate: load }
}
