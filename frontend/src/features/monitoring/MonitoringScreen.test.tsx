import { fireEvent, render, screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import * as AuthModule from '@/app/AuthContext'
import type { AuthUser } from '@/app/AuthContext'
import * as api from '@/app/api'
import { MonitoringScreen } from './MonitoringScreen'

const monitoringStyles = readFileSync('src/features/monitoring/MonitoringScreen.css', 'utf8')

const cleaningUser: AuthUser = {
  id: 'cleaning-1',
  name: 'Petugas Kebersihan',
  email: 'cleaning@example.com',
  systemRole: null,
  role: 'warga',
  jabatan: 'kebersihan',
  rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'Urena' },
  rtCode: '002',
  rwName: 'RW Melati',
  rwNumber: 16,
}

const rows = [
  {
    household_occupancy_id: 'internal-occupancy-a',
    house_number: 'A-01',
    note: 'Jadwal kunjungan pagi',
  },
  {
    household_occupancy_id: 'internal-occupancy-b',
    house_number: 'B-07',
  },
]

describe('Monitoring print report', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockImplementation((media: string) => ({
        matches: false,
        media,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: cleaningUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
    vi.spyOn(api, 'getSessionPair').mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
    vi.spyOn(api, 'apiGetMonitoringHouseholds').mockResolvedValue({
      households: rows,
      total: 2,
    })
  })

  it('prints the current monitoring data in a report without internal identifiers', async () => {
    const print = vi.spyOn(window, 'print').mockImplementation(() => {})
    render(<MonitoringScreen />)

    const report = await screen.findByRole('region', { name: 'Laporan Monitoring Kebersihan' })
    expect(within(report).getByRole('heading', { name: 'LAPORAN MONITORING KEBERSIHAN' })).toBeInTheDocument()
    expect(report).toHaveTextContent('RT 002 — Urena | RW 016 — RW Melati')
    expect(report).toHaveTextContent('Total rumah yang memiliki tunggakan: 2')
    expect(report).toHaveTextContent('A-01')
    expect(report).toHaveTextContent('B-07')
    expect(report).toHaveTextContent('Jadwal kunjungan pagi')
    expect(report).not.toHaveTextContent('internal-occupancy')
    expect(report).not.toHaveTextContent(cleaningUser.rtId!)

    fireEvent.click(screen.getByRole('button', { name: /Cetak/ }))
    expect(print).toHaveBeenCalledOnce()
  })

  it('shows the empty monitoring state and zero total in the report', async () => {
    vi.mocked(api.apiGetMonitoringHouseholds).mockResolvedValue({
      households: [],
      total: 0,
    })
    render(<MonitoringScreen />)
    const report = await screen.findByRole('region', { name: 'Laporan Monitoring Kebersihan' })
    expect(report).toHaveTextContent('Total rumah yang memiliki tunggakan: 0')
    expect(report).toHaveTextContent('Tidak ada rumah dalam monitoring.')
  })

  it('uses A4 print CSS and hides application controls from the printed report', () => {
    expect(monitoringStyles).toContain('@media print')
    expect(monitoringStyles).toContain('size: A4 portrait')
    expect(monitoringStyles).toContain('.monitoring-screen-content')
    expect(monitoringStyles).toContain('.monitoring-actions')
    expect(monitoringStyles).toContain('display: table-header-group')
    expect(monitoringStyles).toContain('break-inside: avoid')
    expect(monitoringStyles).toContain('counter(page)')
  })
})
