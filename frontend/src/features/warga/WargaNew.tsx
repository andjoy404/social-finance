import { DataChoiceModal } from './DataChoiceModal'
import { useNavigate } from 'react-router-dom'

export function WargaNew() {
  const navigate = useNavigate()

  const handleClose = () => {
    navigate('/warga', { replace: true })
  }

  return (
    <DataChoiceModal open onClose={handleClose} />
  )
}
