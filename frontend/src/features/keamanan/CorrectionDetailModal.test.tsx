import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { CorrectionDetailModal } from './CorrectionDetailModal'
import type { AttendanceCorrection } from './securityApi'

describe('CorrectionDetailModal', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
  })

  afterEach(() => {
    cleanup()
    document.documentElement.removeAttribute('data-theme')
  })

  const sampleCorrection: AttendanceCorrection = {
    id: 'corr-1',
    rt_id: 'rt-1',
    attendance_record_id: 'att-1',
    schedule_id: 'sched-1',
    officer_user_id: 'usr-1',
    officer_name: 'Budi Keamanan',
    shift_name: 'Shift Pagi',
    shift_date: '2026-11-20',
    request_type: 'status_change',
    original_status: 'tidak_hadir',
    requested_status: 'hadir',
    reason: 'Lupa scan saat pergantian shift',
    status: 'pending',
    requested_by: 'usr-1',
    requested_by_name: 'Budi Keamanan',
    created_at: '2026-11-20T08:00:00Z',
  }

  it('uses dark-theme-safe CSS tokens without hardcoded white backgrounds on modal, Descriptions, and panels', () => {
    document.documentElement.setAttribute('data-theme', 'dark')

    render(
      <CorrectionDetailModal
        open={true}
        correction={sampleCorrection}
        onClose={vi.fn()}
        onRefresh={vi.fn()}
        canDecide={true}
        currentUserId="usr-admin"
      />
    )

    expect(screen.getByText('Detail Koreksi Absensi')).toBeDefined()
    expect(screen.getByText('Budi Keamanan')).toBeDefined()
    expect(screen.getByText('Alasan: Lupa scan saat pergantian shift')).toBeDefined()
    expect(screen.getByText('Menunggu Konfirmasi')).toBeDefined()
    expect(screen.getByRole('button', { name: /Setujui/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /Tolak/i })).toBeDefined()

    const modalRoot = document.querySelector('.sf-correction-detail-modal')
    expect(modalRoot).not.toBeNull()

    const modalContent = document.querySelector('.sf-correction-detail-modal .ant-modal-content') as HTMLElement | null
    expect(modalContent).not.toBeNull()
    expect(modalContent!.style.background).toContain('var(--sf-surface)')

    // Descriptions info panel must use theme tokens, never hardcoded white
    const descLabels = document.querySelectorAll('.sf-correction-detail-modal .ant-descriptions-item-label')
    const descContents = document.querySelectorAll('.sf-correction-detail-modal .ant-descriptions-item-content')
    expect(descLabels.length).toBeGreaterThanOrEqual(4)
    expect(descContents.length).toBeGreaterThanOrEqual(4)
    descLabels.forEach((el) => {
      expect((el as HTMLElement).style.background).toContain('var(--sf-surface-subtle)')
      expect((el as HTMLElement).style.color).toContain('var(--sf-text-muted)')
    })
    descContents.forEach((el) => {
      expect((el as HTMLElement).style.background).toContain('var(--sf-surface)')
      expect((el as HTMLElement).style.color).toContain('var(--sf-text)')
    })

    // Section panels must use theme tokens
    const statusPanel = screen.getByTestId('correction-status-panel')
    const submittedPanel = screen.getByTestId('correction-submitted-panel')
    const decisionPanel = screen.getByTestId('correction-decision-panel')
    expect(statusPanel.style.background).toContain('var(--sf-surface-subtle)')
    expect(submittedPanel.style.background).toContain('var(--sf-surface-subtle)')
    expect(decisionPanel.style.background).toContain('var(--sf-warning-soft)')

    // No hardcoded light/white hex colors inside modal portal
    const modalHtml = modalRoot!.innerHTML.toLowerCase()
    for (const forbidden of ['#ffffff', '#fff;', '#f5f5f5', '#fafafa', '#fffbe6', '#f6ffed', '#fff2f0', 'rgb(255, 255, 255)']) {
      expect(modalHtml).not.toContain(forbidden)
    }
  })

  it('preserves Light Theme readability and hides approval buttons on self-requested corrections', () => {
    document.documentElement.setAttribute('data-theme', 'light')

    render(
      <CorrectionDetailModal
        open={true}
        correction={sampleCorrection}
        onClose={vi.fn()}
        onRefresh={vi.fn()}
        canDecide={true}
        currentUserId="usr-1" // same as requested_by -> self-approval forbidden
      />
    )

    expect(screen.getByText('Detail Koreksi Absensi')).toBeDefined()
    expect(screen.getByText('Menunggu Persetujuan')).toBeDefined()
    expect(screen.getByText('Tidak Hadir')).toBeDefined()
    expect(screen.getByText('Hadir')).toBeDefined()
    expect(screen.queryByRole('button', { name: /Setujui/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /Tolak/i })).toBeNull()
    expect(screen.getByRole('button', { name: /Tutup/i })).toBeDefined()

    const modalContent = document.querySelector('.sf-correction-detail-modal .ant-modal-content') as HTMLElement | null
    expect(modalContent).not.toBeNull()
    expect(modalContent!.style.background).toContain('var(--sf-surface)')
  })

  it('renders approved decision status with semantic token styles', () => {
    document.documentElement.setAttribute('data-theme', 'dark')
    const approvedCorrection: AttendanceCorrection = {
      ...sampleCorrection,
      status: 'approved',
      decided_by_name: 'Ketua RT',
      decision_note: 'Disetujui setelah konfirmasi',
      decided_at: '2026-11-20T09:00:00Z',
    }

    render(
      <CorrectionDetailModal
        open={true}
        correction={approvedCorrection}
        onClose={vi.fn()}
        onRefresh={vi.fn()}
        canDecide={false}
      />
    )

    expect(screen.getByText('Disetujui')).toBeDefined()
    expect(screen.getByText('Peninjau: Ketua RT')).toBeDefined()
    expect(screen.getByText('Catatan: Disetujui setelah konfirmasi')).toBeDefined()
    expect(screen.getByTestId('correction-decision-panel').style.background).toContain('var(--sf-success-soft)')
    expect(document.querySelector('.sf-correction-detail-modal')!.innerHTML.toLowerCase()).not.toContain('#f6ffed')
  })

  it('renders rejected decision status with semantic token styles', () => {
    document.documentElement.setAttribute('data-theme', 'dark')
    const rejectedCorrection: AttendanceCorrection = {
      ...sampleCorrection,
      status: 'rejected',
      decided_by_name: 'Ketua RT',
      decision_note: 'Bukti tidak cukup',
      decided_at: '2026-11-20T09:00:00Z',
    }

    render(
      <CorrectionDetailModal
        open={true}
        correction={rejectedCorrection}
        onClose={vi.fn()}
        onRefresh={vi.fn()}
        canDecide={false}
      />
    )

    expect(screen.getByText('Ditolak')).toBeDefined()
    expect(screen.getByText('Peninjau: Ketua RT')).toBeDefined()
    expect(screen.getByText('Catatan: Bukti tidak cukup')).toBeDefined()
    expect(screen.getByTestId('correction-decision-panel').style.background).toContain('var(--sf-danger-soft)')
    expect(document.querySelector('.sf-correction-detail-modal')!.innerHTML.toLowerCase()).not.toContain('#fff2f0')
  })
})
