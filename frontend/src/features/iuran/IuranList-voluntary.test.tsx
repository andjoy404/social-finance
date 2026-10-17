import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { IuranList } from './IuranList';
import * as api from '../../app/api';

vi.mock('../../app/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../app/api')>();
  return {
    ...actual,
    getSessionPair: vi.fn(),
    apiListBills: vi.fn(),
    apiListPayments: vi.fn(),
    apiGetBill: vi.fn(),
  };
});

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('IuranList - Voluntary Rp0 Bills', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    });
    (api.apiGetBill as any).mockResolvedValue(null);
  });

  it('renders individual Rp0 bills from API (not synthetic rows)', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-vol-1',
        rt_id: 'rt-1',
        due_id: 'due-voluntary-1',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '0.00',
        status: 'unpaid',
        head_name: 'Budi Santoso',
        house_number: '001',
        due_name: 'Kerja Bakti',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
        rt_code: '001',
      },
      {
        id: 'bill-vol-2',
        rt_id: 'rt-1',
        due_id: 'due-voluntary-1',
        household_occupancy_id: 'occ-2',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '0.00',
        status: 'unpaid',
        head_name: 'Siti Aminah',
        house_number: '002',
        due_name: 'Kerja Bakti',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
        rt_code: '001',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 2, total_pages: 1 },
    });
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    renderWithRouter(<IuranList />);

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined();
      expect(screen.getByText('Siti Aminah')).toBeDefined();
    });

    // Each household has its own bill row
    const rows = document.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);

    // No synthetic collective row
    expect(screen.queryByText('Semua Warga (Sukarela)')).toBeNull();
  });

  it('Rp0 bills display correct individual head_name and house_number', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-vol-1',
        rt_id: 'rt-1',
        due_id: 'due-voluntary-1',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '0.00',
        status: 'unpaid',
        head_name: 'Ahmad Hidayat',
        house_number: '007',
        due_name: 'Sumbangan RT',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
        rt_code: '007',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    });
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    renderWithRouter(<IuranList />);

    await waitFor(() => {
      expect(screen.getByText('Ahmad Hidayat')).toBeDefined();
    });

    expect(screen.getByText('007')).toBeDefined();
  });

  it('regular bills still display correctly formatted periode (e.g. "Okt 2026")', async () => {
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

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 2, total_pages: 1 },
    });
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    renderWithRouter(<IuranList />);

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined();
    });

    expect(screen.getByText('Okt 2026')).toBeDefined();
    expect(screen.getByText('Nov 2026')).toBeDefined();
  });

  it('clicking a bill row opens IuranDetail (not DueDetailModal)', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-vol-1',
        rt_id: 'rt-1',
        due_id: 'due-voluntary-1',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '0.00',
        status: 'unpaid',
        head_name: 'Budi Santoso',
        due_name: 'Kerja Bakti',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    });
    (api.apiListPayments as any).mockResolvedValue({ data: [] });
    (api.apiGetBill as any).mockResolvedValue(mockBills[0]);

    renderWithRouter(<IuranList />);

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined();
    });

    // Find the table row
    const rows = document.querySelectorAll('tbody tr');
    expect(rows.length).toBeGreaterThan(0);

    // Click the bill row
    rows[0]?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    // IuranDetail modal should appear (it uses dialog role in modal mode)
    await waitFor(() => {
      const modal = screen.getByRole('dialog');
      expect(modal).toBeDefined();
    });

    // Should show household name
    const modal = screen.getByRole('dialog');
    expect(modal.textContent).toContain('Budi Santoso');
  });

  it('mix of Rp0 voluntary and regular bills render correctly together', async () => {
    const mockBills: api.ApiBill[] = [
      {
        id: 'bill-regular',
        rt_id: 'rt-1',
        due_id: 'due-regular',
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
        id: 'bill-vol-1',
        rt_id: 'rt-1',
        due_id: 'due-voluntary',
        household_occupancy_id: 'occ-2',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '0.00',
        status: 'unpaid',
        head_name: 'Siti Aminah',
        due_name: 'Kerja Bakti',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
        rt_code: '001',
      },
    ];

    (api.apiListBills as any).mockResolvedValue({
      data: mockBills,
      pagination: { page: 1, per_page: 20, total: 2, total_pages: 1 },
    });
    (api.apiListPayments as any).mockResolvedValue({ data: [] });

    renderWithRouter(<IuranList />);

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined();
      expect(screen.getByText('Siti Aminah')).toBeDefined();
    });

    const rows = document.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);
  });
});
