import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { apiFetch, ApiError } from './api'

describe('apiFetch', () => {
  let fetchMock: ReturnType<typeof vi.fn>
  let errorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({}),
      text: async () => '',
    })
    vi.stubGlobal('fetch', fetchMock)
    errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    errorSpy.mockRestore()
  })

  it('prepends BASE /api to the path', async () => {
    await apiFetch('/titles/batch-status', { method: 'POST', body: '{}' })
    expect(fetchMock).toHaveBeenCalledOnce()
    const url = fetchMock.mock.calls[0][0]
    expect(url).toBe('/api/titles/batch-status')
  })

  it('warns in dev when path starts with /api (would produce /api/api/...)', async () => {
    await apiFetch('/api/titles/batch-status')
    expect(errorSpy).toHaveBeenCalled()
    const msg = errorSpy.mock.calls[0][0] as string
    expect(msg).toContain('must NOT start with')
    expect(msg).toContain('/api')
  })

  it('warns in dev when path does not start with /', async () => {
    await apiFetch('titles/batch-status')
    expect(errorSpy).toHaveBeenCalled()
    expect(errorSpy.mock.calls[0][0]).toContain("must start with '/'")
  })

  it('does not warn for valid paths', async () => {
    await apiFetch('/titles/123')
    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('throws ApiError on non-2xx response', async () => {
    fetchMock.mockResolvedValueOnce({ ok: false, status: 500, text: async () => 'boom' })
    await expect(apiFetch('/foo')).rejects.toBeInstanceOf(ApiError)
  })

  it('returns undefined for 204/202 (no body parsing)', async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, status: 204, json: async () => { throw new Error('should not parse') } })
    const result = await apiFetch('/foo')
    expect(result).toBeUndefined()
  })
})
