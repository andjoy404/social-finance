import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { SpecialResidentCreate } from './SpecialResidentCreate'
import { SpecialResidentEdit } from './SpecialResidentEdit'
import { rtPositionLabel, positionVariant } from '@/features/rt/position-badge'
import * as api from '@/app/api'

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 'user-ketua-1',
      email: 'ketua@test.com',
      fullName: 'Ketua RT',
      role: 'pengurus',
      jabatan: 'ketua',
      systemRole: null,
      rtId: 'rt-1',
    },
  }),
}))

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof import('@/app/api')>('@/app/api')
  return {
    ...actual,
    getSessionPair: vi.fn(() => ({ accessToken: 'test-token', refreshToken: 'test-refresh' })),
    createSpecialResident: vi.fn(),
    getResident: vi.fn(),
    updateSpecialResident: vi.fn(),
  }
})

describe('Special Resident positions (Keamanan, Operasional, Kebersihan)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('maps position labels and badge variants for keamanan, operasional, and kebersihan', () => {
    expect(rtPositionLabel('keamanan')).toBe('Keamanan')
    expect(rtPositionLabel('operasional')).toBe('Operasional')
    expect(rtPositionLabel('kebersihan')).toBe('Kebersihan')

    expect(positionVariant('keamanan')).toBe('red')
    expect(positionVariant('operasional')).toBe('blue')
    expect(positionVariant('kebersihan')).toBe('green')
    expect(positionVariant('kebersihan_pembangunan')).toBe('default')
  })

  it('SpecialResidentCreate renders Keamanan, Operasional, and Kebersihan options and submits kebersihan & operasional', async () => {
    const createMock = vi.mocked(api.createSpecialResident).mockResolvedValue({
      id: 'res-new-1',
      rt_id: 'rt-1',
      household_id: null,
      full_name: 'Budi Kebersihan',
      nik: '3173000000000001',
      phone: '081234567890',
      email: 'budi.kebersihan@test.com',
      jabatan: 'kebersihan',
      is_active: true,
      created_at: '2026-11-05T00:00:00Z',
      updated_at: '2026-11-05T00:00:00Z',
    })

    render(
      <MemoryRouter initialEntries={['/warga/special/new']}>
        <Routes>
          <Route path="/warga/special/new" element={<SpecialResidentCreate />} />
          <Route path="/warga" element={<div>Daftar Warga</div>} />
        </Routes>
      </MemoryRouter>
    )

    const select = screen.getByLabelText(/Jenis Petugas/i) as HTMLSelectElement
    const options = Array.from(select.options).map((o) => ({ value: o.value, label: o.text }))
    expect(options).toEqual([
      { value: 'keamanan', label: 'Keamanan' },
      { value: 'operasional', label: 'Operasional' },
      { value: 'kebersihan', label: 'Kebersihan' },
    ])

    fireEvent.change(select, { target: { value: 'kebersihan' } })
    fireEvent.change(screen.getByLabelText(/Nama/i), { target: { value: 'Budi Kebersihan' } })
    fireEvent.change(screen.getByLabelText(/NIK/i), { target: { value: '3173000000000001' } })
    fireEvent.change(screen.getByLabelText(/Telepon/i), { target: { value: '081234567890' } })
    fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: 'budi.kebersihan@test.com' } })
    fireEvent.change(screen.getByLabelText(/Kata Sandi/i), { target: { value: 'password123' } })

    fireEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await waitFor(() => {
      expect(createMock).toHaveBeenCalledWith(
        'test-token',
        expect.objectContaining({
          jabatan: 'kebersihan',
          full_name: 'Budi Kebersihan',
          nik: '3173000000000001',
          phone: '081234567890',
          email: 'budi.kebersihan@test.com',
          password: 'password123',
          is_active: true,
        })
      )
    })
  })

  it('SpecialResidentEdit loads an operasional resident and allows updating to kebersihan', async () => {
    vi.mocked(api.getResident).mockResolvedValue({
      id: 'res-op-1',
      rt_id: 'rt-1',
      household_id: null,
      full_name: 'Andi Operasional',
      nik: '3173000000000002',
      phone: '081234567891',
      email: 'andi.op@test.com',
      jabatan: 'operasional',
      is_active: true,
      created_at: '2026-11-05T00:00:00Z',
      updated_at: '2026-11-05T00:00:00Z',
    })
    const updateMock = vi.mocked(api.updateSpecialResident).mockResolvedValue({
      id: 'res-op-1',
      rt_id: 'rt-1',
      household_id: null,
      full_name: 'Andi Operasional',
      nik: '3173000000000002',
      phone: '081234567891',
      email: 'andi.op@test.com',
      jabatan: 'kebersihan',
      is_active: true,
      created_at: '2026-11-05T00:00:00Z',
      updated_at: '2026-11-05T00:00:00Z',
    })

    render(
      <MemoryRouter initialEntries={['/warga/special/res-op-1/edit']}>
        <Routes>
          <Route path="/warga/special/:id/edit" element={<SpecialResidentEdit />} />
          <Route path="/warga" element={<div>Daftar Warga</div>} />
        </Routes>
      </MemoryRouter>
    )

    const select = (await screen.findByLabelText(/Jenis Petugas/i)) as HTMLSelectElement
    expect(select.value).toBe('operasional')

    const options = Array.from(select.options).map((o) => ({ value: o.value, label: o.text }))
    expect(options).toEqual([
      { value: 'keamanan', label: 'Keamanan' },
      { value: 'operasional', label: 'Operasional' },
      { value: 'kebersihan', label: 'Kebersihan' },
    ])

    fireEvent.change(select, { target: { value: 'kebersihan' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Perubahan' }))

    await waitFor(() => {
      expect(updateMock).toHaveBeenCalledWith(
        'test-token',
        'res-op-1',
        expect.objectContaining({
          jabatan: 'kebersihan',
        })
      )
    })
  })
})
