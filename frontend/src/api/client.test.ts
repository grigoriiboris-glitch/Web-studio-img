import { afterEach, describe, expect, it, vi } from 'vitest'
import { authApi } from './client'

describe('authApi.register', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('posts registration credentials to the API and returns the session', async () => {
    const session = {
      user: { id: 'user-1', email: 'artist@example.com', name: 'Artist' },
      token: 'token-1',
      expires_at: '2026-10-07T00:00:00Z',
    }
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(session), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    await expect(authApi.register('artist@example.com', 'Artist', 'password123')).resolves.toEqual(session)

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/auth/register')
    expect(init?.method).toBe('POST')
    expect(init?.headers).toMatchObject({
      Accept: 'application/json',
      'Content-Type': 'application/json',
    })
    expect(JSON.parse(String(init?.body))).toEqual({
      email: 'artist@example.com',
      name: 'Artist',
      password: 'password123',
    })
  })

  it('does not hide a browser network failure as an authentication error', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'))

    await expect(authApi.register('artist@example.com', 'Artist', 'password123'))
      .rejects.toThrow('Failed to fetch')
  })
})
