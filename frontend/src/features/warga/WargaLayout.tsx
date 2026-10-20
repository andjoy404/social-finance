import { Outlet } from 'react-router-dom'
import { ResidentList } from './ResidentList'

export function WargaLayout() {
  return (
    <>
      <ResidentList />
      <Outlet />
    </>
  )
}
