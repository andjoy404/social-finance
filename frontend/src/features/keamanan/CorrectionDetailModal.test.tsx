import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { CorrectionDetailModal } from './CorrectionDetailModal'
import * as securityApi from './securityApi'
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

  it('displays Catatan Koreksi row separating requester note and approver decision note, and handles empty notes gracefully', () => {
    document.documentElement.setAttribute('data-theme', 'dark')
    const longReason = 'HP petugas mati mendadak saat hujan deras sehingga tidak sempat scan QR pos pada shift malam hari.'
    const approvedWithEmptyDecisionNote: AttendanceCorrection = {
      ...sampleCorrection,
      reason: longReason,
      status: 'approved',
      decided_by_name: 'Humas RT',
      decision_note: null,
      decided_at: '2026-11-20T09:15:00Z',
    }

    render(
      <CorrectionDetailModal
        open={true}
        correction={approvedWithEmptyDecisionNote}
        onClose={vi.fn()}
        onRefresh={vi.fn()}
        canDecide={false}
      />
    )

    expect(screen.getByText('Catatan Koreksi')).toBeDefined()
    expect(screen.getByText('Catatan Pemohon')).toBeDefined()
    expect(screen.getByText('Catatan Keputusan / Verifikasi')).toBeDefined()
    const requesterNode = screen.getByTestId('correction-requester-note')
    const approverNode = screen.getByTestId('correction-approver-note')
    expect(requesterNode.textContent).toBe(longReason)
    expect(requesterNode.style.whiteSpace).toBe('pre-wrap')
    expect(approverNode.textContent).toBe('—')
  })

  it('submits typed decision note on Setujui and Tolak without stale closure bug, and handles API error safely', async () => {
    const decideSpy = vi.spyOn(securityApi, 'apiDecideSecurityCorrection')
    decideSpy.mockResolvedValueOnce({ ...sampleCorrection, status: 'approved', decision_note: 'Terverifikasi CCTV' })

    const onRefresh = vi.fn()
    const onClose = vi.fn()

    render(
      <CorrectionDetailModal
        open={true}
        correction={sampleCorrection}
        onClose={onClose}
        onRefresh={onRefresh}
        canDecide={true}
        currentUserId="usr-humas"
      />
    )

    // 1. Approve flow with typed decision note
    fireEvent.click(screen.getByRole('button', { name: /Setujui/i }))
    const approveTextarea = await screen.findByPlaceholderText('Catatan keputusan (opsional)')
    fireEvent.change(approveTextarea, { target: { value: 'Terverifikasi CCTV' } })
    const confirmButtons = screen.getAllByRole('button', { name: /Setujui/i })
    fireEvent.click(confirmButtons[confirmButtons.length - 1])

    await waitFor(() => {
      expect(decideSpy).toHaveBeenCalledWith('corr-1', true, 'Terverifikasi CCTV')
      expect(onRefresh).toHaveBeenCalledTimes(1)
      expect(onClose).toHaveBeenCalledTimes(1)
    })

    // 2. Reject flow with typed decision note
    decideSpy.mockResolvedValueOnce({ ...sampleCorrection, status: 'rejected', decision_note: 'Tidak hadir di pos' })
    cleanup()
    const onRefresh2 = vi.fn()
    const onClose2 = vi.fn()
    render(
      <CorrectionDetailModal
        open={true}
        correction={sampleCorrection}
        onClose={onClose2}
        onRefresh={onRefresh2}
        canDecide={true}
        currentUserId="usr-ketua"
      />
    )
    fireEvent.click(screen.getByRole('button', { name: /Tolak/i }))
    const rejectTextarea = await screen.findByPlaceholderText('Catatan keputusan (opsional)')
    fireEvent.change(rejectTextarea, { target: { value: 'Tidak hadir di pos' } })
    const rejectButtons = screen.getAllByRole('button', { name: /Tolak/i })
    fireEvent.click(rejectButtons[rejectButtons.length - 1])

    await waitFor(() => {
      expect(decideSpy).toHaveBeenCalledWith('corr-1', false, 'Tidak hadir di pos')
      expect(onRefresh2).toHaveBeenCalledTimes(1)
      expect(onClose2).toHaveBeenCalledTimes(1)
    })

    // 3. Error state keeps modal open and does not call onClose
    decideSpy.mockRejectedValueOnce(new Error('Koreksi sudah diputuskan sebelumnya'))
    cleanup()
    const onRefresh3 = vi.fn()
    const onClose3 = vi.fn()
    render(
      <CorrectionDetailModal
        open={true}
        correction={sampleCorrection}
        onClose={onClose3}
        onRefresh={onRefresh3}
        canDecide={true}
        currentUserId="usr-ketua"
      />
    )
    fireEvent.click(screen.getByRole('button', { name: /Setujui/i }))
    await screen.findByPlaceholderText('Catatan keputusan (opsional)')
    const confirmBtns3 = screen.getAllByRole('button', { name: /Setujui/i })
    fireEvent.click(confirmBtns3[confirmBtns3.length - 1])

    await waitFor(() => {
      expect(decideSpy).toHaveBeenCalledTimes(3)
    })
    expect(onClose3).not.toHaveBeenCalled()
    expect(onRefresh3).not.toHaveBeenCalled()
    decideSpy.mockRestore()
  })
})
