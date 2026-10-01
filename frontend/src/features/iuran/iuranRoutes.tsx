import { Route, Routes } from 'react-router-dom'
import { IuranList } from './IuranList'
import { IuranPayment } from './IuranPayment'
import { IuranArrears } from './IuranArrears'
import { IuranReport } from './IuranReport'
import { IuranDetail } from './IuranDetail'

export function IuranRoutes() {
  return (
    <Routes>
      <Route index element={<IuranList />} />
      <Route path=":id" element={<IuranDetail />} />
      <Route path=":id/pembayaran" element={<IuranPayment />} />
      <Route path="tunggakan" element={<IuranArrears />} />
      <Route path="laporan" element={<IuranReport />} />
    </Routes>
  )
}
