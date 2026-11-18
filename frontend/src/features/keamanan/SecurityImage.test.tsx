import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import '@testing-library/jest-dom'
import * as SecurityApi from './securityApi'
import { SecurityImage, SecurityPhotoLightbox } from './SecurityImage'

vi.mock('./securityApi', async () => {
  const actual = await vi.importActual<typeof import('./securityApi')>('./securityApi')
  return {
    ...actual,
    apiGetPatrolPhotoBlob: vi.fn(),
    apiGetPaketPhotoBlob: vi.fn(),
  }
})

describe('SecurityImage & SecurityPhotoLightbox Tests', () => {
  const mockBlob = new Blob(['fake image bytes'], { type: 'image/jpeg' })

  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    vi.mocked(SecurityApi.apiGetPatrolPhotoBlob).mockResolvedValue(mockBlob)
    vi.mocked(SecurityApi.apiGetPaketPhotoBlob).mockResolvedValue(mockBlob)
  })

  it('loads blob from authenticated API and renders image', async () => {
    render(<SecurityImage photoId="photo-123" domain="patrol" alt="Patrol Gang Mawar" />)

    expect(screen.getByLabelText('Memuat foto...')).toBeInTheDocument()

    await waitFor(() => {
      expect(SecurityApi.apiGetPatrolPhotoBlob).toHaveBeenCalledWith('photo-123')
      const img = screen.getByRole('img', { name: 'Patrol Gang Mawar' })
      expect(img).toBeInTheDocument()
      expect(img.getAttribute('src')).toMatch(/^blob:/)
    })
  })

  it('renders fallback when image fails to load', async () => {
    vi.mocked(SecurityApi.apiGetPaketPhotoBlob).mockRejectedValue(new Error('404 Not Found'))

    render(<SecurityImage photoId="photo-invalid" domain="paket" alt="Paket Box" />)

    await waitFor(() => {
      expect(screen.getByText('Gagal memuat foto')).toBeInTheDocument()
    })
  })

  it('revokes blob URL on unmount to prevent memory leaks', async () => {
    const { unmount } = render(
      <SecurityImage photoId="photo-123" domain="patrol" alt="Test Leak" />
    )

    await waitFor(() => {
      expect(screen.getByRole('img', { name: 'Test Leak' })).toBeInTheDocument()
    })

    unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalled()
  })

  it('renders lightbox modal with navigation controls and closes on escape', () => {
    const photos = [
      { id: 'p1', title: 'Gang 1', location: 'Blok A' },
      { id: 'p2', title: 'Gang 2', location: 'Blok B' },
    ]
    const onClose = vi.fn()

    const { rerender } = render(
      <SecurityPhotoLightbox
        photos={photos}
        initialIndex={0}
        domain="patrol"
        open={true}
        onClose={onClose}
      />
    )

    expect(screen.getByRole('dialog', { name: 'Pratinjau Foto' })).toBeInTheDocument()
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.getByText('Gang 1')).toBeInTheDocument()
    expect(screen.getByText('Blok A')).toBeInTheDocument()

    // Next button
    const nextBtn = screen.getByLabelText('Foto Selanjutnya')
    fireEvent.click(nextBtn)
    expect(screen.getByText('2 / 2')).toBeInTheDocument()
    expect(screen.getByText('Gang 2')).toBeInTheDocument()

    // Escape closes lightbox
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()

    // When closed, renders null
    rerender(
      <SecurityPhotoLightbox
        photos={photos}
        initialIndex={0}
        domain="patrol"
        open={false}
        onClose={onClose}
      />
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
