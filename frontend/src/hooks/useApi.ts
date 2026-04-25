import { useState, useEffect, useCallback, useRef } from 'preact/hooks'
import { apiFetch } from '../api'

interface UseApiResult<T> {
  data: T | null
  error: string | null
  loading: boolean
  mutate: () => void
  setData: (next: T | ((prev: T | null) => T)) => void
}

export function useApi<T>(path: string | null): UseApiResult<T> {
  const [data, setDataState] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(path !== null)
  const hasDataRef = useRef(false)

  const setData = useCallback((next: T | ((prev: T | null) => T)) => {
    setDataState((prev) => {
      const v = typeof next === 'function' ? (next as (p: T | null) => T)(prev) : next
      hasDataRef.current = v != null
      return v
    })
  }, [])

  const load = useCallback(() => {
    if (!path) return
    const controller = new AbortController()
    if (!hasDataRef.current) setLoading(true)
    setError(null)
    apiFetch<T>(path, { signal: controller.signal })
      .then((d) => {
        hasDataRef.current = d != null
        setDataState(d)
      })
      .catch((e) => { if (e.name !== 'AbortError') setError(e.message) })
      .finally(() => setLoading(false))
    return controller
  }, [path])

  useEffect(() => {
    hasDataRef.current = false
    setDataState(null)
    const controller = load()
    return () => controller?.abort()
  }, [load])

  return { data, error, loading, mutate: load, setData }
}
