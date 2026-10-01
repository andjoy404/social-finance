import { Routes, Route, Navigate } from 'react-router-dom'
import { KasList } from './KasList'
import { KasIncome } from './KasIncome'
import { KasExpense } from './KasExpense'
import { KasDetail } from './KasDetail'
import { KasReport } from './KasReport'

export function KasRoutes() {
  return (
    <Routes>
      <Route index element={<KasList />} />
      <Route path="baru" element={<KasIncome />} />
      <Route path=":id" element={<KasDetail />} />
      <Route path="laporan" element={<KasReport />} />
      <Route path="*" element={<Navigate to="/kas" replace />} />
    </Routes>
  )
}
