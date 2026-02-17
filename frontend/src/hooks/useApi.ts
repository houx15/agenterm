import { useCallback, useEffect, useState } from 'react'
import { apiFetch } from '../api/client'

export function useApi<T>(path: string, enabled = true) {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState<boolean>(enabled)
  const [error, setError] = useState<string>('')

  const refetch = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const result = await apiFetch<T>(path)
      setData(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [path])

  useEffect(() => {
    if (enabled) {
      void refetch()
    }
  }, [enabled, refetch])

  return { data, loading, error, refetch }
}
