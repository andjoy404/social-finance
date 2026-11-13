import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom'
import { ResidentAvatar } from './ResidentAvatar'

const { signMock } = vi.hoisted(() => {
  const signMock = vi.fn()
  return { signMock }
})

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof import('@/app/api')>('@/app/api')
  return {
    ...actual,
    apiSignCommunityImage: (...args: unknown[]) => signMock(...args),
    getSessionPair: vi.fn().mockReturnValue({ accessToken: 'valid-test-token' }),
  }
})

describe('ResidentAvatar thumbnail optimization', () => {
  beforeEach(() => {
    signMock.mockReset()
  })

  it('renders initials when no avatar is provided', () => {
    const { container } = render(<ResidentAvatar name="Ahmad Dahlan" />)
    expect(container.textContent).toBe('AD')
    expect(signMock).not.toHaveBeenCalled()
  })

  it('prefers thumbnail avatarThumbPath over original avatarPath', async () => {
    signMock.mockImplementation((_token: string, key: string) => {
      if (key === 'community/rt1/profile/photo_thumb.jpg') {
        return Promise.resolve({ url: 'https://example.test/photo_thumb.jpg', expires_at: 0 })
      }
      return Promise.resolve({ url: 'https://example.test/photo.jpg', expires_at: 0 })
    })

    render(
      <ResidentAvatar
        name="Ahmad Dahlan"
        avatarPath="community/rt1/profile/photo.jpg"
        avatarThumbPath="community/rt1/profile/photo_thumb.jpg"
      />,
    )

    await waitFor(() => {
      const img = screen.getByRole('button') as HTMLImageElement
      expect(img.src).toBe('https://example.test/photo_thumb.jpg')
    })

    // Verify sign was called with the thumbnail key first
    expect(signMock).toHaveBeenCalledWith('valid-test-token', 'community/rt1/profile/photo_thumb.jpg')
  })

  it('falls back to avatarPath when avatarThumbPath signing fails', async () => {
    signMock.mockImplementation((_token: string, key: string) => {
      if (key.includes('_thumb')) {
        return Promise.reject(new Error('Thumb not found'))
      }
      return Promise.resolve({ url: 'https://example.test/photo_original.jpg', expires_at: 0 })
    })

    render(
      <ResidentAvatar
        name="Ahmad Dahlan"
        avatarPath="community/rt1/profile/photo.jpg"
        avatarThumbPath="community/rt1/profile/photo_thumb.jpg"
      />,
    )

    await waitFor(() => {
      const img = screen.getByRole('button') as HTMLImageElement
      expect(img.src).toBe('https://example.test/photo_original.jpg')
    })
  })

  it('renders initials fallback when both thumb and original fail', async () => {
    signMock.mockRejectedValue(new Error('Network error'))

    const { container } = render(
      <ResidentAvatar
        name="Ahmad Dahlan"
        avatarPath="community/rt1/profile/photo.jpg"
        avatarThumbPath="community/rt1/profile/photo_thumb.jpg"
      />,
    )

    await waitFor(() => {
      expect(container.textContent).toBe('AD')
    })
  })
})
