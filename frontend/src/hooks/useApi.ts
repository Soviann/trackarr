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
    setLoading(true)
    setError(null)
    apiFetch<T>(path)
      .then(setData)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [path])

  useEffect(() => { load() }, [load])

  return { data, error, loading, mutate: load }
}
