import { Outlet } from 'react-router-dom'
import { ResidentList } from './ResidentList'

export function WargaLayout() {
  return (
    <div style={{ position: 'relative', flex: 1 }}>
      <ResidentList />
      <Outlet />
    </div>
  )
}
