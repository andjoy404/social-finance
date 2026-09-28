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

  const handleClose = () => {
    onClose()
  }

  const handleSelect = (type: string) => {
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
        <div className="dc-choice-container">
          <button
            type="button"
            onClick={() => handleSelect('warga')}
            className="dc-choice-card"
          >
            <span className="dc-choice-icon">
              <UserOutlined />
            </span>
            <div className="dc-choice-content">
              <div className="dc-choice-title">Warga</div>
              <div className="dc-choice-description">
                Tambah anggota keluarga ke rumah tangga
              </div>
            </div>
          </button>

          <button
            type="button"
            onClick={() => handleSelect('special')}
            className="dc-choice-card"
          >
            <span className="dc-choice-icon">
              <TeamOutlined />
            </span>
            <div className="dc-choice-content">
              <div className="dc-choice-title">Petugas</div>
              <div className="dc-choice-description">
                Keamanan / Kebersihan & Pembangunan (tanpa rumah tangga)
              </div>
            </div>
          </button>
        </div>
      </ModalBody>
    </Modal>
  )
}
