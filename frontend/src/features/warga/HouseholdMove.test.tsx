import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'
import { HouseholdMove } from './HouseholdMove'
import { persistSessionPair } from '@/app/api'
import * as AuthModule from '@/app/AuthContext'
import { MemoryRouter, Routes, Route } from 'react-router-dom'

function renderHouseholdMove(householdId: string = 'hh-1') {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user: {
      id: 'user-1',
      name: 'Test User',
      email: 'test@example.com',
      systemRole: null,
      role: 'pengurus',
      rt: { id: 'rt-1', name: 'RT 001' },
    },
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={[`/warga/${householdId}/pindah`]}>
        <Routes>
          <Route path="/warga/:id/pindah" element={<HouseholdMove />} />
          <Route path="/warga/:id" element={<div>HOUSEHOLD_MOVE_DESTINATION</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  }
}

function mockFetch(fn: (input: string, init?: RequestInit) => Response | Promise<Response>) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(
    async (input, init) => fn(typeof input === 'string' ? input : '', init),
  )
}

function okResponse<R>(data: R): Response {
  return new Response(JSON.stringify(data), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function failResponse(status: number, data: unknown): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('W4.3 — HouseholdMove form rendering', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows description explaining move is a new occupancy period', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse({
          id: 'hh-1', rt_id: 'rt-1', house_number: '001',
          head_name: 'Test', address: 'Jl. Test', phone: null,
          nik: null, occupancy_status: 'OWNER',
          is_active: true, created_at: '', updated_at: '',
        })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdMove()

    await waitFor(() => {
      expect(screen.getByText(/membuat periode hunian baru/i)).toBeInTheDocument()
    })
  })

  it('navigates to detail on successful move', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'POST' && input.includes('/api/v1/households/hh-1/move')) {
        return okResponse({ id: 'hh-1', rt_id: 'rt-1', house_number: '002', head_name: 'Test', address: 'Jl. Test', phone: null, nik: null, occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      if (input.includes('/api/v1/households/hh-1') && init?.method === 'GET') {
        return okResponse({ id: 'hh-1', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: 'Jl. Test', phone: null, nik: null, occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdMove()

    const dateInput = await screen.findByLabelText(/Tanggal Mulai Hunian Baru\s*\*/i)
    const submitBtn = screen.getByRole('button', { name: /Pindahkan/i })

    await userEvent.clear(dateInput)
    await userEvent.type(dateInput, '2026-07-15')

    await userEvent.click(submitBtn)

    await waitFor(() => {
      expect(screen.getByText(/HOUSEHOLD_MOVE_DESTINATION/)).toBeInTheDocument()
    })
  })
})
