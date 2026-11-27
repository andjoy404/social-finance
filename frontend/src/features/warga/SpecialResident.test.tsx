import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { SpecialResidentCreate } from './SpecialResidentCreate'
import { SpecialResidentEdit } from './SpecialResidentEdit'
import { HouseholdEdit } from './HouseholdEdit'
import { rtPositionLabel, positionVariant } from '@/features/rt/position-badge'
import * as api from '@/app/api'

const ketuaUser = {
  id: 'user-ketua-1',
  email: 'ketua@test.com',
  fullName: 'Ketua RT',
  role: 'pengurus',
  jabatan: 'ketua' as string | null,
  systemRole: null as string | null,
  rtId: 'rt-1',
}

const superAdminUser = {
  id: 'user-sa-1',
  email: 'admin@local.com',
  fullName: 'Super Admin',
  role: 'pengurus',
  jabatan: null as string | null,
  systemRole: 'super_admin' as string | null,
  rtId: 'rt-2',
}

let activeMockUser = ketuaUser

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: activeMockUser,
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
    getHousehold: vi.fn(),
    createHousehold: vi.fn(),
  }
})

describe('Special Resident positions (Keamanan, Operasional, Kebersihan)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    activeMockUser = ketuaUser
  })

  it('maps position labels and badge variants for keamanan, operasional, and kebersihan', () => {
    expect(rtPositionLabel('keamanan')).toBe('Keamanan')
    expect(rtPositionLabel('operasional')).toBe('Operasional')
    expect(rtPositionLabel('kebersihan')).toBe('Kebersihan')

    expect(positionVariant('keamanan')).toBe('red')
    expect(positionVariant('operasional')).toBe('teal')
    expect(positionVariant('kebersihan')).toBe('chartreuse')
    expect(positionVariant('sosial')).toBe('pink')
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
      { value: 'kebersihan', label: 'Kebersihan' },
      { value: 'anggota_rw', label: 'Anggota RW' },
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
    await waitFor(() => expect(screen.getByDisplayValue('Andi Operasional')).toBeInTheDocument())
    expect(select.value).toBe('operasional')

    const options = Array.from(select.options).map((o) => ({ value: o.value, label: o.text }))
    expect(options).toEqual([
      { value: 'keamanan', label: 'Keamanan' },
      { value: 'kebersihan', label: 'Kebersihan' },
      { value: 'anggota_rw', label: 'Anggota RW' },
      { value: 'operasional', label: 'Operasional' },
    ])

    fireEvent.change(select, { target: { value: 'kebersihan' } })
    await waitFor(() => {
      expect(select.value).toBe('kebersihan')
    })
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

  it('SuperAdmin opens Detail Anggota RW without calling getHousehold or redirecting to HouseholdEdit', async () => {
    activeMockUser = superAdminUser
    vi.mocked(api.getResident).mockResolvedValue({
      id: '3ac8df2e-f5e5-44a6-9cf7-e61515ed17d9',
      rt_id: 'rt-001',
      rt_number: '1',
      rw: 16,
      rt_name: '001',
      household_id: null,
      full_name: 'Anggota RW RT 001',
      nik: null,
      phone: null,
      email: null,
      jabatan: 'anggota_rw',
      is_active: true,
      created_at: '2026-11-23T00:00:00Z',
      updated_at: '2026-11-23T00:00:00Z',
    })

    render(
      <MemoryRouter initialEntries={['/warga/special/3ac8df2e-f5e5-44a6-9cf7-e61515ed17d9']}>
        <Routes>
          <Route path="/warga/special/:id" element={<SpecialResidentEdit mode="detail" />} />
          <Route path="/warga/:id/edit" element={<HouseholdEdit />} />
          <Route path="/warga/:id" element={<div>ResidentDetailFallback</div>} />
          <Route path="/warga" element={<div>Daftar Warga</div>} />
        </Routes>
      </MemoryRouter>
    )

    expect(await screen.findByText('Detail Petugas')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Anggota RW RT 001')).toBeInTheDocument()
    expect((screen.getByLabelText(/Jenis Petugas/i) as HTMLSelectElement).value).toBe('anggota_rw')
    expect(screen.getByDisplayValue('RT 001 / RW 016 — 001')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Ubah Petugas' })).toBeInTheDocument()

    // Must never call getHousehold or render HouseholdEdit 404
    expect(api.getHousehold).not.toHaveBeenCalled()
    expect(screen.queryByText(/Rumah tangga tidak ditemukan/i)).not.toBeInTheDocument()
  })

  it('SuperAdmin edits Anggota RW non-household fields with redacted contact info and keeps household_id null', async () => {
    activeMockUser = superAdminUser
    vi.mocked(api.getResident).mockResolvedValue({
      id: '3ac8df2e-f5e5-44a6-9cf7-e61515ed17d9',
      rt_id: 'rt-001',
      rt_number: '1',
      rw: 16,
      rt_name: '001',
      household_id: null,
      full_name: 'Anggota RW RT 001',
      nik: null,
      phone: null,
      email: null,
      jabatan: 'anggota_rw',
      is_active: true,
      created_at: '2026-11-23T00:00:00Z',
      updated_at: '2026-11-23T00:00:00Z',
    })
    const updateMock = vi.mocked(api.updateSpecialResident).mockResolvedValue({
      id: '3ac8df2e-f5e5-44a6-9cf7-e61515ed17d9',
      rt_id: 'rt-001',
      household_id: null,
      full_name: 'Anggota RW RT 001 Updated',
      nik: null,
      phone: null,
      email: null,
      jabatan: 'anggota_rw',
      is_active: false,
      created_at: '2026-11-23T00:00:00Z',
      updated_at: '2026-11-23T00:00:00Z',
    })

    render(
      <MemoryRouter initialEntries={['/warga/special/3ac8df2e-f5e5-44a6-9cf7-e61515ed17d9/edit']}>
        <Routes>
          <Route path="/warga/special/:id/edit" element={<SpecialResidentEdit />} />
          <Route path="/warga/:id/edit" element={<HouseholdEdit />} />
          <Route path="/warga" element={<div>Daftar Warga</div>} />
        </Routes>
      </MemoryRouter>
    )

    const nameInput = (await screen.findByLabelText(/Nama Lengkap/i)) as HTMLInputElement
    expect(nameInput.value).toBe('Anggota RW RT 001')

    // Change non-household fields only (redacted NIK, phone, email remain empty)
    fireEvent.change(nameInput, { target: { value: 'Anggota RW RT 001 Updated' } })
    fireEvent.change(screen.getByLabelText(/Status/i), { target: { value: 'false' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Perubahan' }))

    await waitFor(() => {
      expect(updateMock).toHaveBeenCalledTimes(1)
    })

    const [tokenArg, idArg, payloadArg] = updateMock.mock.calls[0]
    expect(tokenArg).toBe('test-token')
    expect(idArg).toBe('3ac8df2e-f5e5-44a6-9cf7-e61515ed17d9')
    expect(payloadArg).toEqual({
      full_name: 'Anggota RW RT 001 Updated',
      jabatan: 'anggota_rw',
      is_active: false,
    })
    // Must never send household_id or dummy household creation or GET household
    expect((payloadArg as Record<string, unknown>).household_id).toBeUndefined()
    expect(api.getHousehold).not.toHaveBeenCalled()
    expect(api.createHousehold).not.toHaveBeenCalled()
  })

  it('redirects non-special household-less resident to ResidentDetail instead of HouseholdEdit', async () => {
    activeMockUser = superAdminUser
    vi.mocked(api.getResident).mockResolvedValue({
      id: 'res-nohh-1',
      rt_id: 'rt-001',
      household_id: null,
      full_name: 'Pengurus Tanpa Rumah',
      nik: null,
      phone: null,
      email: null,
      jabatan: 'sekretaris',
      is_active: true,
      created_at: '2026-11-23T00:00:00Z',
      updated_at: '2026-11-23T00:00:00Z',
    })

    render(
      <MemoryRouter initialEntries={['/warga/special/res-nohh-1']}>
        <Routes>
          <Route path="/warga/special/:id" element={<SpecialResidentEdit mode="detail" />} />
          <Route path="/warga/:id/edit" element={<HouseholdEdit />} />
          <Route path="/warga/:id" element={<div>ResidentDetailFallback</div>} />
        </Routes>
      </MemoryRouter>
    )

    expect(await screen.findByText('ResidentDetailFallback')).toBeInTheDocument()
    expect(api.getHousehold).not.toHaveBeenCalled()
  })

  it('redirects resident with household_id to HouseholdEdit using household_id and loads household normally', async () => {
    activeMockUser = superAdminUser
    vi.mocked(api.getResident).mockResolvedValue({
      id: 'res-with-hh-1',
      rt_id: 'rt-001',
      household_id: 'hh-real-1',
      full_name: 'Warga Biasa',
      nik: null,
      phone: null,
      email: null,
      jabatan: 'ketua',
      is_active: true,
      created_at: '2026-11-23T00:00:00Z',
      updated_at: '2026-11-23T00:00:00Z',
    })
    vi.mocked(api.getHousehold).mockResolvedValue({
      id: 'hh-real-1',
      rt_id: 'rt-001',
      house_number: 'A-12',
      head_name: 'Warga Biasa',
      address: 'Jl. Melati No. 12',
      phone: null,
      email: null,
      nik: null,
      occupancy_status: 'OWNER',
      is_active: true,
      start_date: '2026-01-01',
      end_date: null,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    })

    render(
      <MemoryRouter initialEntries={['/warga/special/res-with-hh-1/edit']}>
        <Routes>
          <Route path="/warga/special/:id/edit" element={<SpecialResidentEdit />} />
          <Route path="/warga/:id/edit" element={<HouseholdEdit />} />
        </Routes>
      </MemoryRouter>
    )

    expect(await screen.findByText('Edit Rumah Tangga')).toBeInTheDocument()
    expect(await screen.findByDisplayValue('A-12')).toBeInTheDocument()
    expect(api.getHousehold).toHaveBeenCalledWith('test-token', 'hh-real-1')
  })

  it('preserves genuine 404 household error in HouseholdEdit when household truly does not exist', async () => {
    activeMockUser = superAdminUser
    vi.mocked(api.getHousehold).mockRejectedValue(new api.ApiError('not_found', 'household not found'))

    render(
      <MemoryRouter initialEntries={['/warga/hh-missing-404/edit']}>
        <Routes>
          <Route path="/warga/:id/edit" element={<HouseholdEdit />} />
        </Routes>
      </MemoryRouter>
    )

    expect(await screen.findByText('Rumah tangga tidak ditemukan.')).toBeInTheDocument()
    expect(api.getHousehold).toHaveBeenCalledWith('test-token', 'hh-missing-404')
  })
})
