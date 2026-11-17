import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import '@testing-library/jest-dom'
import { ResidentPhotoManage } from './ResidentPhotoManage'

const { uploadMock, authState } = vi.hoisted(() => {
  const uploadMock = vi.fn()
  const authState: { user: any } = { user: null }
  return { uploadMock, authState }
})

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof import('@/app/api')>('@/app/api')
  return {
    ...actual,
    apiUploadResidentPhoto: (...args: unknown[]) => uploadMock(...args),
    apiDeleteResidentPhoto: vi.fn().mockResolvedValue({ status: 'ok' }),
    apiSignCommunityImage: vi.fn().mockResolvedValue({ url: 'https://example.test/img.jpg', expires_at: 0 }),
    getSessionPair: vi.fn().mockReturnValue({ accessToken: 'tok', refreshToken: 'x' }),
  }
})

vi.mock('@/app/AuthContext', async () => {
  const actual = await vi.importActual<typeof import('@/app/AuthContext')>('@/app/AuthContext')
  return {
    ...actual,
    useAuth: () => authState,
  }
})

const ketuaUser = {
  id: 'u1',
  systemRole: null,
  role: 'pengurus',
  jabatan: 'ketua',
  effectivePermissions: ['resident.profile_photo.manage'],
}

function findFileInput(container: HTMLElement): HTMLInputElement | null {
  return container.querySelector('input[type="file"]')
}

function hugeFile() {
  const bytes = new Uint8Array(5 * 1024 * 1024 + 1).fill(0x01)
  return new File([bytes], 'huge.jpg', { type: 'image/jpeg' })
}

describe('ResidentPhotoManage', () => {
  beforeEach(() => {
    uploadMock.mockReset()
    authState.user = ketuaUser
  })

  it('rejects files larger than 5MB and never sends upload', () => {
    const { container } = render(
      <ResidentPhotoManage
        residentId="r1"
        fullName="Budi Santoso"
        avatarPath={null}
        onChanged={() => {}}
      />,
    )
    const input = findFileInput(container)
    expect(input).not.toBeNull()
    fireEvent.change(input!, { target: { files: [hugeFile()] } })
    expect(uploadMock).not.toHaveBeenCalled()
  })

  it('renders nothing when user lacks resident.profile_photo.manage', () => {
    authState.user = { id: 'u2', systemRole: null, role: 'warga', jabatan: '', effectivePermissions: [] }
    const { container } = render(
      <ResidentPhotoManage
        residentId="r1"
        fullName="Budi Santoso"
        avatarPath={null}
        onChanged={() => {}}
      />,
    )
    expect(container.querySelector('button')).toBeNull()
  })
})
