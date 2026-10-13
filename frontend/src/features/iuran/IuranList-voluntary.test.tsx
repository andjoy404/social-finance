import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { IuranList } from './IuranList';
import * as api from '../../app/api';

vi.mock('../../app/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../app/api')>();
  return {
    ...actual,
    getSessionPair: vi.fn(),
    apiListBills: vi.fn(),
    apiListDues: vi.fn(),
    apiListPayments: vi.fn(),
    apiGetBill: vi.fn(),
  };
});

describe('IuranList - Voluntary Rp0 Dues', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    });
  });

  it('renders voluntary Rp0 due in the list with Sukarela badge and nominal Rp0', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-1',
        rt_id: 'rt-1',
        due_id: 'due-mandatory',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '50000.00',
        status: 'unpaid',
        head_name: 'Budi Santoso',
        due_name: 'Iuran Sampah',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    const mockDues: api.ApiDue[] = [
      {
        id: 'due-voluntary-1',
        rt_id: 'rt-1',
        rt_code: '001',
        name: 'Kerja Bakti',
        amount: '0.00',
        period_type: 'one_time',
        payment_rule: 'partial',
        is_voluntary: true,
        is_active: true,
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
      {
        id: 'due-mandatory',
        rt_id: 'rt-1',
        name: 'Iuran Sampah',
        amount: '50000.00',
        period_type: 'monthly',
        payment_rule: 'partial',
        is_voluntary: false,
        is_active: true,
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    });
    (api.apiListDues as any).mockResolvedValue(mockDues);
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    render(<IuranList />);

    // Wait for data to load
    await waitFor(() => {
      expect(screen.getByText('Kerja Bakti')).toBeDefined();
    });

    // Check presence of "Semua Warga (Sukarela)"
    expect(screen.getByText('Semua Warga (Sukarela)')).toBeDefined();

    // Check badge status "Sukarela"
    expect(screen.getByText('Sukarela')).toBeDefined();

    // Check Rp0 is rendered
    expect(screen.getAllByText('Rp0').length).toBeGreaterThan(0);
  });

  it('voluntary Rp0 period column does NOT display "undefined —"', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-1',
        rt_id: 'rt-1',
        due_id: 'due-mandatory',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '50000.00',
        status: 'unpaid',
        head_name: 'Budi Santoso',
        due_name: 'Iuran Sampah',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    const mockDues: api.ApiDue[] = [
      {
        id: 'due-voluntary-1',
        rt_id: 'rt-1',
        rt_code: '001',
        name: 'Kerja Bakti',
        amount: '0.00',
        period_type: 'one_time',
        payment_rule: 'partial',
        is_voluntary: true,
        is_active: true,
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    });
    (api.apiListDues as any).mockResolvedValue(mockDues);
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    render(<IuranList />);

    await waitFor(() => {
      expect(screen.getByText('Kerja Bakti')).toBeDefined();
    });

    // Verify that "undefined —" does NOT appear anywhere in the table
    const body = document.body.textContent;
    expect(body).not.toContain('undefined');
    expect(body).not.toContain('undefined —');
  });

  it('voluntary Rp0 displays correct RT from rt_code', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-1',
        rt_id: 'rt-1',
        due_id: 'due-mandatory',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '50000.00',
        status: 'unpaid',
        head_name: 'Budi Santoso',
        due_name: 'Iuran Sampah',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
        rt_code: '001',
      },
    ];

    const mockDues: api.ApiDue[] = [
      {
        id: 'due-voluntary-1',
        rt_id: 'rt-1',
        rt_code: '001',
        name: 'Kerja Bakti',
        amount: '0.00',
        period_type: 'one_time',
        payment_rule: 'partial',
        is_voluntary: true,
        is_active: true,
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    });
    (api.apiListDues as any).mockResolvedValue(mockDues);
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    render(<IuranList />);

    await waitFor(() => {
      expect(screen.getByText('Kerja Bakti')).toBeDefined();
    });

    // Voluntary item should show "RT 001" (not "Semua RT" or UUID)
    // Both bill and voluntary row show RT 001, so check at least 2 occurrences
    expect(screen.getAllByText('RT 001').length).toBeGreaterThanOrEqual(2);
  });

  it('regolar bills still display correctly formatted periode (e.g. "Okt 2026")', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-okt',
        rt_id: 'rt-1',
        due_id: 'due-mandatory',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '50000.00',
        status: 'unpaid',
        head_name: 'Budi Santoso',
        due_name: 'Iuran Sampah',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
        rt_code: '001',
      },
      {
        id: 'bill-nov',
        rt_id: 'rt-1',
        due_id: 'due-mandatory',
        household_occupancy_id: 'occ-1',
        period: '2026-11',
        due_date: '2026-11-30',
        amount: '50000.00',
        status: 'paid',
        head_name: 'Siti Aminah',
        due_name: 'Iuran Keamanan',
        created_at: '2026-11-01T00:00:00Z',
        updated_at: '2026-11-01T00:00:00Z',
        rt_code: '002',
      },
    ];

    const mockDues: api.ApiDue[] = [
      {
        id: 'due-mandatory',
        rt_id: 'rt-1',
        name: 'Iuran Sampah',
        amount: '50000.00',
        period_type: 'monthly',
        payment_rule: 'partial',
        is_voluntary: false,
        is_active: true,
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 2, total_pages: 1 },
    });
    (api.apiListDues as any).mockResolvedValue(mockDues);
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    render(<IuranList />);

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined();
    });

    // Regular bills should still show formatted period
    expect(screen.getByText('Okt 2026')).toBeDefined();
    expect(screen.getByText('Nov 2026')).toBeDefined();
  });

  it('clicking voluntary row opens DueDetailModal with correct due data', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-1',
        rt_id: 'rt-1',
        due_id: 'due-mandatory',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '50000.00',
        status: 'unpaid',
        head_name: 'Budi Santoso',
        due_name: 'Iuran Sampah',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    const mockDues: api.ApiDue[] = [
      {
        id: 'due-voluntary-1',
        rt_id: 'rt-1',
        rt_code: '001',
        name: 'Kerja Bakti',
        amount: '0.00',
        period_type: 'one_time',
        payment_rule: 'partial',
        is_voluntary: true,
        is_active: true,
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    });
    (api.apiListDues as any).mockResolvedValue(mockDues);
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    render(<IuranList />);

    // Wait for data to load
    await waitFor(() => {
      expect(screen.getByText('Kerja Bakti')).toBeDefined();
    });

    // Find the voluntary row (contains "Semua Warga (Sukarela)")
    const rows = document.querySelectorAll('tbody tr');
    const voluntaryRow = Array.from(rows).find(row =>
      row.textContent?.includes('Semua Warga (Sukarela)')
    );

    expect(voluntaryRow).toBeDefined();

    // Click the voluntary row
    voluntaryRow?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    // DueDetailModal should appear with due data
    // Modal contains aria-label "Detail Definisi Iuran Sukarela"
    await waitFor(() => {
      const modal = screen.getByRole('dialog');
      expect(modal).toBeDefined();
    });

    // Verify modal content — query inside dialog to avoid ambiguity with table rows
    await waitFor(() => {
      const modal = screen.getByRole('dialog');
      // Modal should contain the due name "Kerja Bakti"
      expect(modal.textContent).toContain('Kerja Bakti');
      // Modal should show "Sukarela" status
      expect(modal.textContent).toContain('Sukarela');
      // Modal should show Rp0
      expect(modal.textContent).toContain('Rp0');
      // Modal should show period type
      expect(modal.textContent).toContain('Sekali Waktu');
      // Modal should show RT code
      expect(modal.textContent).toContain('RT 001');
    });

    // Click close button (it's an <a> link with aria-label="Tutup", not a <button>)
    const closeButton = screen.getByRole('link', { name: 'Tutup' });
    (closeButton as HTMLElement).click();

    // Modal should be closed
    await waitFor(() => {
      expect(screen.queryByLabelText('Detail Definisi Iuran Sukarela')).toBeNull();
    });
  });
});
