import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody } from '@/components/Modal'
import {
  UserOutlined,
  TeamOutlined,
} from '@ant-design/icons'

interface DataChoiceProps {
  open: boolean
  onClose: () => void
}

export function DataChoiceModal({ open, onClose }: DataChoiceProps) {
  const navigate = useNavigate()
  const [selected, setSelected] = useState<string | null>(null)

  const handleClose = () => {
    setSelected(null)
    onClose()
  }

  const handleSelect = (type: string) => {
    setSelected(type)
    if (type === 'warga') {
      navigate('/warga/household/baru')
    } else if (type === 'special') {
      navigate('/warga/special/baru')
    }
  }

  return (
    <Modal open={open} onClose={handleClose} width={480}>
      <ModalHeader
        title="Tambah Data"
        subtitle="Pilih jenis data yang akan ditambahkan"
        onClose={handleClose}
      />
      <ModalBody>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <button
            type="button"
            onClick={() => handleSelect('warga')}
            disabled={selected !== null}
            style={{
              width: '100%',
              padding: '16px',
              border: `2px solid ${selected === 'warga' ? 'var(--sf-accent)' : 'var(--sf-border)'}`,
              borderRadius: 'var(--radius-sm)',
              background: selected === 'warga' ? 'var(--sf-accent-soft)' : 'var(--sf-bg)',
              cursor: selected !== null ? 'default' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              transition: 'all 0.15s ease',
            }}
          >
            <span style={{
              fontSize: 20,
              color: 'var(--sf-accent)',
            }}>
              <UserOutlined />
            </span>
            <div style={{ textAlign: 'left' }}>
              <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 2 }}>Warga</div>
              <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                Tambah anggota keluarga ke rumah tangga
              </div>
            </div>
          </button>

          <button
            type="button"
            onClick={() => handleSelect('special')}
            disabled={selected !== null}
            style={{
              width: '100%',
              padding: '16px',
              border: `2px solid ${selected === 'special' ? 'var(--sf-accent)' : 'var(--sf-border)'}`,
              borderRadius: 'var(--radius-sm)',
              background: selected === 'special' ? 'var(--sf-accent-soft)' : 'var(--sf-bg)',
              cursor: selected !== null ? 'default' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              transition: 'all 0.15s ease',
            }}
          >
            <span style={{
              fontSize: 20,
              color: 'var(--sf-accent)',
            }}>
              <TeamOutlined />
            </span>
            <div style={{ textAlign: 'left' }}>
              <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 2 }}>Petugas Khusus</div>
              <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                Keamanan / Kebersihan & Pembangunan (tanpa rumah tangga)
              </div>
            </div>
          </button>
        </div>
      </ModalBody>
    </Modal>
  )
}
