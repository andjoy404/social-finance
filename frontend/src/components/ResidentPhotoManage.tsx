import { useCallback, useEffect, useId, useRef, useState } from 'react'
import { Button, Modal, Popconfirm, message } from 'antd'
import { CameraOutlined, DeleteOutlined } from '@ant-design/icons'
import { useAuth, hasPermission } from '@/app/AuthContext'
import { apiUploadResidentPhoto, apiDeleteResidentPhoto, getSessionPair, ApiError } from '@/app/api'

type ResidentPhotoManageProps = {
  residentId: string
  fullName: string
  avatarPath?: string | null
  /** Called after a successful upload or delete with the new avatar path and optional thumbnail path (or null when cleared). */
  onChanged: (newAvatarPath: string | null, newThumbPath?: string | null) => void
  /** Compact button styling to fit dense rows. */
  compact?: boolean
}

const MAX_BYTES = 5 * 1024 * 1024 // 5 MiB (backend enforces same limit and re-sniffs magic bytes)
const ACCEPT = '.jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp'
/** Image types accepted by the backend (extension sanity check mirrors backend switch). */
const ACCEPTED_MIME_TYPES = new Set(['image/jpeg', 'image/png', 'image/webp'])
const ACCEPTED_EXTENSION_RE = /\.(jpe?g|png|webp)$/i

/** True when the picked file is an image format the backend accepts. */
function isAcceptedImageFile(file: File): boolean {
  if (ACCEPTED_MIME_TYPES.has(file.type)) return true
  return ACCEPTED_EXTENSION_RE.test(file.name)
}

/** Map any upload/delete failure to a user-friendly Indonesian message. */
function toUploadErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.status) {
      case 400:
        // Backend validation_error — invalid multipart, oversized or unsupported content.
        // Backend messages are English/internal; surface a user-friendly Indonesian text.
        return 'File foto tidak valid (format atau ukuran).'
      case 401:
        return 'Sesi berakhir, silakan masuk ulang.'
      case 403:
        return 'Anda tidak berwenang mengubah foto ini.'
      case 413:
        return 'File terlalu besar (maksimal 5 MB).'
      case 415:
        return 'Format file tidak didukung. Gunakan JPG, PNG, atau WebP.'
      case 500:
        return 'Gagal mengunggah foto. Silakan coba lagi.'
      default:
        return err.message || 'Gagal mengunggah foto.'
    }
  }
  // Network error / timeout (fetch rejects with TypeError on network failure, AbortError on timeout).
  const isAbort = typeof (err as { name?: string })?.name === 'string' && ((err as { name?: string }).name) === 'AbortError'
  if (isAbort) return 'Permintaan upload timeout. Silakan coba lagi.'
  if (err instanceof TypeError) return 'Jaringan bermasalah, gagal mengunggah foto. Silakan coba lagi.'
  return err instanceof Error && err.message ? err.message : 'Gagal mengunggah foto.'
}

/**
 * "Ubah Foto" / "Hapus Foto" actions for a Warga/Petugas profile photo.
 * Visibility is gated by the resident.profile_photo.manage capability —
 * the backend enforces the same rule on every request.
 */
export function ResidentPhotoManage({ residentId, fullName, avatarPath, onChanged, compact }: ResidentPhotoManageProps) {
  const { user } = useAuth()
  const canManage = hasPermission(user, 'resident.profile_photo.manage')

  const inputRef = useRef<HTMLInputElement | null>(null)
  const objectUrlRef = useRef<string | null>(null)
  const fileInputId = useId()
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const [pendingFile, setPendingFile] = useState<File | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [previewOpen, setPreviewOpen] = useState(false)

  const revokePreview = useCallback(() => {
    if (objectUrlRef.current) {
      URL.revokeObjectURL(objectUrlRef.current)
      objectUrlRef.current = null
    }
  }, [])

  useEffect(() => () => revokePreview(), [revokePreview])

  const handlePicked = (file: File | null) => {
    if (!file) return
    // Client-side guard — mirrors the backend's enforce limit.
    if (file.size > MAX_BYTES) {
      message.error('Ukuran maksimum foto profil adalah 5 MB.')
      return
    }
    // Client-side guard — mirrors the backend's accepted content types.
    if (!isAcceptedImageFile(file)) {
      message.error('Format foto tidak didukung. Gunakan JPG, PNG, atau WebP.')
      return
    }
    revokePreview()
    const url = URL.createObjectURL(file)
    objectUrlRef.current = url
    setPendingFile(file)
    setPreviewUrl(url)
    setPreviewOpen(true)
  }

  const handleSave = async () => {
    // Prevent double submission while an upload is already in flight.
    if (saving) return
    if (!pendingFile) return
    const session = getSessionPair()
    if (!session?.accessToken) {
      message.error('Sesi berakhir, silakan masuk ulang.')
      return
    }
    setSaving(true)
    try {
      const res = await apiUploadResidentPhoto(session.accessToken, residentId, pendingFile)
      setPreviewOpen(false)
      revokePreview()
      setPendingFile(null)
      onChanged(res.avatar_path, res.avatar_thumb_path ?? null)
      message.success('Foto tersimpan')
    } catch (err) {
      // Keep the old photo visible until the server confirms success.
      message.error(toUploadErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    const session = getSessionPair()
    if (!session?.accessToken) {
      message.error('Sesi berakhir, silakan masuk ulang.')
      return
    }
    setDeleting(true)
    try {
      await apiDeleteResidentPhoto(session.accessToken, residentId)
      onChanged(null, null)
      message.success('Foto dihapus')
    } catch (err) {
      message.error(toUploadErrorMessage(err))
    } finally {
      setDeleting(false)
    }
  }

  if (!canManage) return null

  const sizeProps = compact ? { size: 'small' as const } : {}

  return (
    // stopPropagation keeps row-level navigation (e.g. table rows) from firing
    <span
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <input
        id={fileInputId}
        ref={inputRef}
        type="file"
        accept={ACCEPT}
        style={{ display: 'none' }}
        onChange={(e) => handlePicked(e.target.files?.[0] ?? null)}
      />
      <Button
        {...sizeProps}
        icon={<CameraOutlined />}
        onClick={() => {
          if (inputRef.current) {
            inputRef.current.value = '' // allow re-picking the same file
            inputRef.current.click()
          }
        }}
        style={{ marginRight: 8 }}
      >
        Ubah
      </Button>
      {avatarPath ? (
        <Popconfirm
          title="Hapus foto?"
          description={`Foto ${fullName} akan dihapus.`}
          okText="Hapus"
          cancelText="Batal"
          onConfirm={() => {
            void handleDelete()
          }}
        >
          <Button {...sizeProps} danger icon={<DeleteOutlined />} loading={deleting}>
            Hapus
          </Button>
        </Popconfirm>
      ) : null}

      <Modal
        open={previewOpen}
        title="Preview Foto"
        onCancel={() => {
          setPreviewOpen(false)
          revokePreview()
          setPendingFile(null)
        }}
        footer={[
          <Button
            key="cancel"
            onClick={() => {
              setPreviewOpen(false)
              revokePreview()
              setPendingFile(null)
            }}
          >
            Batal
          </Button>,
          <Button key="save" type="primary" loading={saving} onClick={() => void handleSave()}>
            Simpan
          </Button>,
        ]}
        destroyOnHidden
      >
        {previewUrl ? (
          <div style={{ display: 'flex', justifyContent: 'center', padding: '8px 0' }}>
            <img
              src={previewUrl}
              alt={`${fullName} (preview)`}
              style={{ maxWidth: '100%', maxHeight: 320, borderRadius: 8, objectFit: 'contain' }}
            />
          </div>
        ) : null}
      </Modal>
    </span>
  )
}
