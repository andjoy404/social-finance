import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom'
import { ResidentPhotoManage } from './ResidentPhotoManage'
import { ApiError } from '@/app/api'

const { uploadMock, authState, messageErrorMock, messageSuccessMock } = vi.hoisted(() => {
  const uploadMock = vi.fn()
  const authState: { user: any } = { user: null }
  const messageErrorMock = vi.fn()
  const messageSuccessMock = vi.fn()
  return { uploadMock, authState, messageErrorMock, messageSuccessMock }
})

vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd')
  return {
    ...actual,
    message: {
      ...actual.message,
      error: (...args: unknown[]) => messageErrorMock(...args),
      success: (...args: unknown[]) => messageSuccessMock(...args),
    },
  }
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

function tinyFile() {
  return new File([new Uint8Array([0x01, 0x02, 0x03])], 'photo.jpg', { type: 'image/jpeg' })
}

function hugeFile() {
  const bytes = new Uint8Array(5 * 1024 * 1024 + 1).fill(0x01)
  return new File([bytes], 'huge.jpg', { type: 'image/jpeg' })
}

function unsupportedFile() {
  return new File([new Uint8Array([0x01, 0x02, 0x03])], 'photo.gif', { type: 'image/gif' })
}

function renderManage(overrides: { user?: any } = {}) {
  if (overrides.user !== undefined) authState.user = overrides.user
  const onChanged = vi.fn()
  const utils = render(
    <ResidentPhotoManage
      residentId="r1"
      fullName="Budi Santoso"
      avatarPath={null}
      onChanged={onChanged}
    />,
  )
  return { ...utils, onChanged }
}

describe('ResidentPhotoManage', () => {
  beforeEach(() => {
    uploadMock.mockReset()
    messageErrorMock.mockReset()
    messageSuccessMock.mockReset()
    authState.user = ketuaUser
  })

  it('renders the upload button when user has resident.profile_photo.manage', () => {
    const { getByText } = renderManage()
    expect(getByText('Ubah Foto')).toBeInTheDocument()
  })

  it('renders nothing when user lacks resident.profile_photo.manage', () => {
    const { container } = renderManage({ user: { id: 'u2', systemRole: null, role: 'warga', jabatan: '', effectivePermissions: [] } })
    expect(container.querySelector('button')).toBeNull()
  })

  it('renders the upload button for super admin (permission bypass)', () => {
    const { getByText } = renderManage({
      user: { id: 'u3', systemRole: 'super_admin', role: 'super_admin', jabatan: 'ketua', effectivePermissions: [] },
    })
    expect(getByText('Ubah Foto')).toBeInTheDocument()
  })

  it('rejects files larger than 5MB and never sends upload', () => {
    const { container } = renderManage()
    const input = findFileInput(container)!
    fireEvent.change(input, { target: { files: [hugeFile()] } })
    expect(uploadMock).not.toHaveBeenCalled()
  })

  it('rejects unsupported image formats and never sends upload', () => {
    const { container } = renderManage()
    const input = findFileInput(container)!
    fireEvent.change(input, { target: { files: [unsupportedFile()] } })
    expect(uploadMock).not.toHaveBeenCalled()
  })

  it('opens the file picker when Ubah Foto is clicked', async () => {
    const { getByText } = renderManage()
    fireEvent.click(getByText('Ubah Foto'))
    // jsdom does not open native file dialogs; the hidden input must be the trigger target.
    expect(findFileInput(document.body as HTMLElement)).not.toBeNull()
  })

  it('shows preview after picking a file and saves via api on Simpan', async () => {
    uploadMock.mockResolvedValue({
      status: 'uploaded',
      avatar_path: 'community/rt1/profile/abc_photo.jpg',
      avatar_thumb_path: 'community/rt1/profile/abc_photo_thumb.jpg',
      key: 'community/rt1/profile/abc_photo.jpg',
    })
    const { getByText, findByText, container, onChanged } = renderManage()
    const input = findFileInput(container)!
    fireEvent.change(input, { target: { files: [tinyFile()] } })
    expect(await findByText('Preview Foto')).toBeInTheDocument()

    fireEvent.click(getByText('Simpan'))
    await waitFor(() => expect(uploadMock).toHaveBeenCalledTimes(1))
    expect(uploadMock).toHaveBeenCalledWith('tok', 'r1', expect.any(File))

    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(
      'community/rt1/profile/abc_photo.jpg',
      'community/rt1/profile/abc_photo_thumb.jpg',
    ))
  })

  it('shows friendly error and keeps previous state when upload fails', async () => {
    uploadMock.mockRejectedValue(new ApiError('validation_error', 'file size exceeds 5MB limit', undefined, 400))
    const { getByText, findByText, container, onChanged } = renderManage()
    const input = findFileInput(container)!
    fireEvent.change(input, { target: { files: [tinyFile()] } })
    expect(await findByText('Preview Foto')).toBeInTheDocument()

    fireEvent.click(getByText('Simpan'))
    await waitFor(() => expect(uploadMock).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(messageErrorMock).toHaveBeenCalledWith('File foto tidak valid (format atau ukuran).'))
    // Old photo state must not be replaced by a fake success.
    expect(onChanged).not.toHaveBeenCalled()
  })

  it('shows friendly error for expired session (401) and for network failures', async () => {
    uploadMock.mockRejectedValueOnce(new ApiError('auth_expired', 'Session expired', undefined, 401))
    const { getByText, findByText, container } = renderManage()
    const input = findFileInput(container)!
    fireEvent.change(input, { target: { files: [tinyFile()] } })
    expect(await findByText('Preview Foto')).toBeInTheDocument()
    fireEvent.click(getByText('Simpan'))
    await waitFor(() => expect(messageErrorMock).toHaveBeenCalledWith('Sesi berakhir, silakan masuk ulang.'))

    uploadMock.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    fireEvent.click(getByText('Simpan'))
    await waitFor(() => expect(messageErrorMock).toHaveBeenCalledWith('Jaringan bermasalah, gagal mengunggah foto. Silakan coba lagi.'))
  })

  it('prevents double submission while an upload is in flight', async () => {
    let resolveUpload: (v: unknown) => void = () => {}
    uploadMock.mockReturnValue(new Promise((res) => { resolveUpload = res }))
    const { getByText, findByText, container } = renderManage()
    const input = findFileInput(container)!
    fireEvent.change(input, { target: { files: [tinyFile()] } })
    expect(await findByText('Preview Foto')).toBeInTheDocument()

    fireEvent.click(getByText('Simpan'))
    await waitFor(() => expect(uploadMock).toHaveBeenCalledTimes(1))

    // Second click while saving must not trigger another upload.
    fireEvent.click(getByText('Simpan'))
    await new Promise((r) => setTimeout(r, 50))
    expect(uploadMock).toHaveBeenCalledTimes(1)

    resolveUpload({ status: 'uploaded', avatar_path: 'community/rt1/profile/abc.jpg', avatar_thumb_path: null, key: 'k' })
    await waitFor(() => expect(messageSuccessMock).toHaveBeenCalledWith('Foto tersimpan'))
  })
})
