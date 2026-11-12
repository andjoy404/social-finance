import { useCallback, useEffect, useId, useRef, useState } from 'react'
import { Button, Modal, Popconfirm, message } from 'antd'
import { CameraOutlined, DeleteOutlined } from '@ant-design/icons'
import { useAuth, hasPermission } from '@/app/AuthContext'
import { apiUploadResidentPhoto, apiDeleteResidentPhoto, getSessionPair } from '@/app/api'

type ResidentPhotoManageProps = {
  residentId: string
  fullName: string
  avatarPath?: string | null
  /** Called after a successful upload or delete with the new avatar path (or null when cleared). */
  onChanged: (newAvatarPath: string | null) => void
  /** Compact button styling to fit dense rows. */
  compact?: boolean
}

const MAX_BYTES = 5 * 1024 * 1024 // 5 MiB (backend enforces same limit and re-sniffs magic bytes)
const ACCEPT = '.jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp'

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
    revokePreview()
    const url = URL.createObjectURL(file)
    objectUrlRef.current = url
    setPendingFile(file)
    setPreviewUrl(url)
    setPreviewOpen(true)
  }

  const handleSave = async () => {
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
      onChanged(res.avatar_path)
      message.success('Foto tersimpan')
    } catch (err) {
      // Keep the old photo visible until the server confirms success.
      const text = err instanceof Error ? err.message : 'Gagal mengunggah foto.'
      message.error(text || 'Gagal mengunggah foto.')
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
      onChanged(null)
      message.success('Foto dihapus')
    } catch (err) {
      const text = err instanceof Error ? err.message : 'Gagal menghapus foto.'
      message.error(text || 'Gagal menghapus foto.')
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
      >
        Ubah Foto
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
