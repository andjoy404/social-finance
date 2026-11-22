import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  ApiError,
  apiGetNotificationImageBlob,
  persistSessionPair,
  clearSessionPair,
  resolveNotificationImageUrl,
} from '@/app/api'
import { NotificationImage } from './NotificationImage'

describe('resolveNotificationImageUrl', () => {
  it('preserves full /api/v1/notifications/images/... URL without duplicating /api/v1', () => {
    const backendUrl =
      '/api/v1/notifications/images/notifications/manual/43d1ed42-d982-4299-9b29-30080591fcf9/1795336397537925208_27e8ccabbb4dcc94a47b4717e295429c.png'
    expect(resolveNotificationImageUrl(backendUrl)).toBe(backendUrl)
  })

  it('prefixes /api/v1 when given a /notifications/images/... path', () => {
    expect(resolveNotificationImageUrl('/notifications/images/notifications/manual/rt-1/a.png')).toBe(
      '/api/v1/notifications/images/notifications/manual/rt-1/a.png',
    )
  })

  it('resolves raw storage key notifications/... to the image endpoint', () => {
    expect(resolveNotificationImageUrl('notifications/manual/rt-1/a.png')).toBe(
      '/api/v1/notifications/images/notifications/manual/rt-1/a.png',
    )
  })

  it('rejects empty or unsafe URLs', () => {
    expect(() => resolveNotificationImageUrl('')).toThrow(ApiError)
    expect(() => resolveNotificationImageUrl('javascript:alert(1)')).toThrow(ApiError)
  })
})

describe('apiGetNotificationImageBlob', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    clearSessionPair()
    sessionStorage.clear()
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
    clearSessionPair()
    sessionStorage.clear()
  })

  it('sends Authorization Bearer token and X-RT-ID to exact /api/v1/notifications/images/... URL', async () => {
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'test-refresh' })
    sessionStorage.setItem('sf_active_rt_id', '43d1ed42-d982-4299-9b29-30080591fcf9')

    const mockBlob = new Blob(['png-bytes'], { type: 'image/png' })
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      blob: () => Promise.resolve(mockBlob),
    })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    const targetUrl =
      '/api/v1/notifications/images/notifications/manual/43d1ed42-d982-4299-9b29-30080591fcf9/sample.png'
    const result = await apiGetNotificationImageBlob(targetUrl)

    expect(result).toBe(mockBlob)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [calledUrl, calledOpts] = fetchMock.mock.calls[0]
    expect(calledUrl).toBe(targetUrl)
    expect(calledUrl).not.toContain('/api/v1/api/v1/')
    expect(calledOpts.headers['Authorization']).toBe('Bearer test-jwt-token')
    expect(calledOpts.headers['X-RT-ID']).toBe('43d1ed42-d982-4299-9b29-30080591fcf9')
  })

  it('throws ApiError with status 403 when server rejects cross-tenant image access', async () => {
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'test-refresh' })

    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: () =>
        Promise.resolve(JSON.stringify({ error: { code: 'forbidden', message: 'cross-RT access denied' } })),
    })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    await expect(
      apiGetNotificationImageBlob('/api/v1/notifications/images/notifications/manual/other-rt/secret.png'),
    ).rejects.toMatchObject({
      code: 'forbidden',
      status: 403,
    })
  })
})

describe('NotificationImage component', () => {
  const originalFetch = globalThis.fetch
  let createObjectURLSpy: ReturnType<typeof vi.spyOn>
  let revokeObjectURLSpy: ReturnType<typeof vi.spyOn>
  let objectUrlSeq = 0

  beforeEach(() => {
    objectUrlSeq = 0
    clearSessionPair()
    sessionStorage.clear()
    persistSessionPair({ accessToken: 'valid-bearer-token', refreshToken: 'valid-refresh' })
    createObjectURLSpy = vi.spyOn(URL, 'createObjectURL').mockImplementation(() => {
      objectUrlSeq += 1
      return `blob:http://localhost/notification-img-${objectUrlSeq}`
    })
    revokeObjectURLSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
    createObjectURLSpy.mockRestore()
    revokeObjectURLSpy.mockRestore()
    clearSessionPair()
    sessionStorage.clear()
  })

  it('fetches authenticated image with Bearer token, renders blob URL, and revokes on unmount', async () => {
    const mockBlob = new Blob(['image-data'], { type: 'image/png' })
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      blob: () => Promise.resolve(mockBlob),
    })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    const imageUrl =
      '/api/v1/notifications/images/notifications/manual/43d1ed42-d982-4299-9b29-30080591fcf9/img.png'
    const { unmount } = render(<NotificationImage src={imageUrl} alt="Preview Lampiran" />)

    const img = await screen.findByRole('img', { name: 'Preview Lampiran' })
    expect(img).toHaveAttribute('src', 'blob:http://localhost/notification-img-1')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][0]).toBe(imageUrl)
    expect(fetchMock.mock.calls[0][1].headers['Authorization']).toBe('Bearer valid-bearer-token')

    unmount()
    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:http://localhost/notification-img-1')
  })

  it('handles HTTP 401 cleanly without rendering a broken/successful image', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      text: () => Promise.resolve(JSON.stringify({ error: { code: 'unauthorized', message: 'Unauthorized' } })),
    })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    render(
      <NotificationImage
        src="/api/v1/notifications/images/notifications/manual/rt-1/img.png"
        alt="Preview Lampiran"
      />,
    )

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/sesi tidak valid/i)
    expect(screen.queryByRole('img', { name: 'Preview Lampiran' })).not.toBeInTheDocument()
  })

  it('handles HTTP 403 cleanly as forbidden error state without retry button or img element', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: () => Promise.resolve(JSON.stringify({ error: { code: 'forbidden', message: 'Forbidden' } })),
    })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    render(
      <NotificationImage
        src="/api/v1/notifications/images/notifications/manual/other-rt/img.png"
        alt="Preview Lampiran"
      />,
    )

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/akses gambar ditolak/i)
    expect(screen.queryByRole('img', { name: 'Preview Lampiran' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /muat ulang/i })).not.toBeInTheDocument()
  })

  it('prevents race conditions and revokes previous object URL when src changes', async () => {
    let resolveFirst!: (res: unknown) => void
    let resolveSecond!: (res: unknown) => void
    const signals: Array<AbortSignal | undefined> = []

    const fetchMock = vi.fn().mockImplementation((_url: string, opts?: RequestInit) => {
      signals.push(opts?.signal ?? undefined)
      if (signals.length === 1) {
        return new Promise((resolve) => {
          resolveFirst = resolve
        })
      }
      return new Promise((resolve) => {
        resolveSecond = resolve
      })
    })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    const firstUrl = '/api/v1/notifications/images/notifications/manual/rt-1/first.png'
    const secondUrl = '/api/v1/notifications/images/notifications/manual/rt-1/second.png'

    const { rerender, unmount } = render(<NotificationImage src={firstUrl} alt="Banner" />)
    expect(fetchMock).toHaveBeenCalledTimes(1)

    // Switch src before first request resolves
    rerender(<NotificationImage src={secondUrl} alt="Banner" />)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(signals[0]?.aborted).toBe(true)

    // Second request finishes first
    resolveSecond({
      ok: true,
      status: 200,
      blob: () => Promise.resolve(new Blob(['second'], { type: 'image/png' })),
    })

    const img = await screen.findByRole('img', { name: 'Banner' })
    expect(img).toHaveAttribute('src', 'blob:http://localhost/notification-img-1')

    // Stale first request resolves later — must be ignored and must not create/overwrite object URL
    resolveFirst({
      ok: true,
      status: 200,
      blob: () => Promise.resolve(new Blob(['first'], { type: 'image/png' })),
    })
    await Promise.resolve()
    await Promise.resolve()

    expect(createObjectURLSpy).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('img', { name: 'Banner' })).toHaveAttribute(
      'src',
      'blob:http://localhost/notification-img-1',
    )

    unmount()
    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:http://localhost/notification-img-1')
  })

  it('aborts in-flight fetch on unmount without leaking object URLs', async () => {
    let capturedSignal: AbortSignal | undefined
    const fetchMock = vi.fn().mockImplementation((_url: string, opts?: RequestInit) => {
      capturedSignal = opts?.signal ?? undefined
      return new Promise(() => {
        // stays pending
      })
    })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    const { unmount } = render(
      <NotificationImage
        src="/api/v1/notifications/images/notifications/manual/rt-1/pending.png"
        alt="Pending"
      />,
    )

    expect(fetchMock).toHaveBeenCalledTimes(1)
    unmount()

    expect(capturedSignal?.aborted).toBe(true)
    expect(createObjectURLSpy).not.toHaveBeenCalled()
  })

  it('supports retry after transient error', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.resolve(JSON.stringify({ error: { code: 'internal_error', message: 'Temporary error' } })),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        blob: () => Promise.resolve(new Blob(['ok-png'], { type: 'image/png' })),
      })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    render(
      <NotificationImage
        src="/api/v1/notifications/images/notifications/manual/rt-1/retry.png"
        alt="Retryable"
      />,
    )

    await screen.findByRole('alert')
    const retryBtn = screen.getByRole('button', { name: /muat ulang/i })
    fireEvent.click(retryBtn)

    const img = await screen.findByRole('img', { name: 'Retryable' })
    expect(img).toHaveAttribute('src', 'blob:http://localhost/notification-img-1')
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('renders local blob: preview directly without network request or revoking caller URL', async () => {
    const fetchMock = vi.fn()
    globalThis.fetch = fetchMock as unknown as typeof fetch

    const { unmount } = render(<NotificationImage src="blob:http://localhost/local-draft" alt="Draft" />)

    const img = await screen.findByRole('img', { name: 'Draft' })
    expect(img).toHaveAttribute('src', 'blob:http://localhost/local-draft')
    expect(fetchMock).not.toHaveBeenCalled()

    unmount()
    expect(revokeObjectURLSpy).not.toHaveBeenCalledWith('blob:http://localhost/local-draft')
  })
})
