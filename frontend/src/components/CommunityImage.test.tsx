import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, cleanup, act } from '@testing-library/react'
import '@testing-library/jest-dom'
import { CommunityImage } from '@/components/CommunityImage'

const {
  apiSignCommunityImage,
  apiGetCommunityImageBlob,
  getSessionPair,
} = vi.hoisted(() => ({
  apiSignCommunityImage: vi.fn(),
  apiGetCommunityImageBlob: vi.fn(),
  getSessionPair: vi.fn(),
}))

vi.mock('@/app/api', () => ({
  apiSignCommunityImage,
  apiGetCommunityImageBlob,
  getSessionPair,
}))

const { useAuth, canManageAnnouncements, canManageActivities } = vi.hoisted(() => ({
  useAuth: vi.fn(),
  canManageAnnouncements: vi.fn(),
  canManageActivities: vi.fn(),
}))

vi.mock('@/app/AuthContext', () => ({
  useAuth,
  canManageAnnouncements,
  canManageActivities,
}))

const draftSrc = '/api/v1/community/images/community/rt-1/announcement/draft.png'
const otherDraftSrc = '/api/v1/community/images/community/rt-1/announcement/draft-b.png'
const publishedSrc = '/api/v1/community/images/community/rt-1/announcement/live.png'

const managerUser = {
  id: 'u1', name: 'Manager', email: 'm@test', systemRole: null, role: 'pengurus', jabatan: 'ketua',
  rt: { id: 'rt-1', name: 'RT 002' },
}
const wargaUser = {
  id: 'u2', name: 'Warga', email: 'w@test', systemRole: null, role: 'warga', jabatan: null,
  rt: { id: 'rt-1', name: 'RT 002' },
}

function setAuth(user: any, canManage: boolean) {
  useAuth.mockReturnValue({ user })
  canManageAnnouncements.mockReturnValue(canManage)
  canManageActivities.mockReturnValue(canManage)
}

beforeEach(() => {
  vi.clearAllMocks()
  setAuth(managerUser, true)
  getSessionPair.mockReturnValue({ accessToken: 'tok', refreshToken: 'rtok' })
  URL.createObjectURL = vi.fn(() => 'blob:mock-object-url') as any
  URL.revokeObjectURL = vi.fn() as any
})

afterEach(() => {
  cleanup()
})

describe('CommunityImage', () => {
  it('renders published images via signed URL for warga', async () => {
    setAuth(wargaUser, false)
    apiSignCommunityImage.mockResolvedValue({ url: 'https://cdn.example/live.png?exp=1&sig=abc', expires_at: 1 })

    render(<CommunityImage src={publishedSrc} alt="live" />)

    await waitFor(() => expect(screen.getByRole('img', { name: 'live' })).toHaveAttribute('src', 'https://cdn.example/live.png?exp=1&sig=abc'))
    expect(apiGetCommunityImageBlob).not.toHaveBeenCalled()
  })

  it('manager falls back to Bearer blob for draft images', async () => {
    apiSignCommunityImage.mockRejectedValue(new Error('not found'))
    apiGetCommunityImageBlob.mockResolvedValue({ blob: new Blob(['x'], { type: 'image/png' }), contentType: 'image/png' })

    render(<CommunityImage src={draftSrc} alt="draft" />)

    await waitFor(() => expect(screen.getByRole('img', { name: 'draft' })).toHaveAttribute('src', 'blob:mock-object-url'))
    expect(apiGetCommunityImageBlob).toHaveBeenCalledTimes(1)
    expect(apiGetCommunityImageBlob).toHaveBeenCalledWith('tok', draftSrc)
  })

  it('warga never calls the Bearer blob fallback and shows placeholder', async () => {
    setAuth(wargaUser, false)
    apiSignCommunityImage.mockRejectedValue(new Error('not found'))

    render(<CommunityImage src={draftSrc} alt="draft" />)

    await waitFor(() => expect(screen.getByRole('img', { name: 'draft' })).toBeInTheDocument())
    expect(apiGetCommunityImageBlob).not.toHaveBeenCalled()
  })

  it('object URL is revoked on unmount', async () => {
    apiSignCommunityImage.mockRejectedValue(new Error('not found'))
    apiGetCommunityImageBlob.mockResolvedValue({ blob: new Blob(['x']), contentType: 'image/png' })

    const { unmount } = render(<CommunityImage src={draftSrc} alt="draft" />)
    await waitFor(() => expect(screen.getByRole('img', { name: 'draft' })).toBeInTheDocument())
    unmount()

    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-object-url')
  })

  it('object URL is revoked when the image source changes', async () => {
    apiSignCommunityImage.mockRejectedValue(new Error('not found'))
    apiGetCommunityImageBlob.mockResolvedValue({ blob: new Blob(['x']), contentType: 'image/png' })

    const { rerender } = render(<CommunityImage src={draftSrc} alt="draft" />)
    await waitFor(() => expect(screen.getByRole('img', { name: 'draft' })).toBeInTheDocument())

    rerender(<CommunityImage src={otherDraftSrc} alt="draft-b" />)
    await waitFor(() => expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-object-url'))
  })

  it('canManage true -> false removes the private draft image', async () => {
    apiSignCommunityImage.mockRejectedValue(new Error('not found'))
    apiGetCommunityImageBlob.mockResolvedValue({ blob: new Blob(['x']), contentType: 'image/png' })

    const { rerender } = render(<CommunityImage src={draftSrc} alt="draft" />)
    await waitFor(() => expect(screen.getByRole('img', { name: 'draft' })).toBeInTheDocument())

    // Demote to warga: rerender with same src but warga auth
    setAuth(wargaUser, false)
    rerender(<CommunityImage src={draftSrc} alt="draft" />)

    await waitFor(() => expect(screen.getByRole('img', { name: 'draft' }).tagName.toLowerCase()).not.toBe('img'))
    // the placeholder carries the aria-label 'draft' but is a div, not an <img>
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-object-url')
  })

  it('stale async result does not overwrite a newer image', async () => {
    let resolveSignA: ((v: { url: string; expires_at: number }) => void) | null = null
    apiSignCommunityImage.mockImplementation((_token: string, key: string) => {
      if (key === publishedSrc) {
        return new Promise((resolve) => {
          resolveSignA = resolve
        }) as Promise<{ url: string; expires_at: number }>
      }
      return Promise.resolve({ url: 'https://cdn.example/new.png?sig=new', expires_at: 1 })
    })

    const { rerender } = render(<CommunityImage src={publishedSrc} alt="first" />)
    await waitFor(() => expect(apiSignCommunityImage).toHaveBeenCalledWith('tok', publishedSrc))

    // Switch src before the first request resolves.
    rerender(<CommunityImage src={otherDraftSrc} alt="second" />)

    // Complete the stale request now.
    await act(async () => {
      resolveSignA?.({ url: 'https://cdn.example/STALE.png?sig=stale', expires_at: 1 })
    })

    await waitFor(() => expect(screen.getByRole('img', { name: 'second' })).toHaveAttribute('src', 'https://cdn.example/new.png?sig=new'))
    expect(screen.getByRole('img', { name: 'second' })).not.toHaveAttribute('src', expect.stringContaining('STALE'))
  })

  it('unauthorized (non-ok) Bearer fallback does not leave stale draft visible', async () => {
    apiSignCommunityImage.mockRejectedValue(new Error('not found'))
    apiGetCommunityImageBlob.mockRejectedValue(new Error('forbidden'))

    render(<CommunityImage src={draftSrc} alt="draft" />)

    await waitFor(() => expect(screen.getByRole('img', { name: 'draft' }).tagName.toLowerCase()).not.toBe('img'))
  })
})
