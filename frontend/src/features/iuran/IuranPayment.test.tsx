import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { IuranPayment } from './IuranPayment'
import * as apiModule from '@/app/api'

const mockBillResponse = {
  id: 'bill-123',
  due_id: 'due-1',
  due_name: 'Iuran Sampah & Keamanan',
  period: '2026-10',
  household_occupancy_id: 'occ-1',
  head_name: 'Budi Santoso',
  house_number: 'A-12',
  rt_code: '001',
  amount: '1000000',
  status: 'PENDING',
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
}

const mockPaymentsResponse = {
  data: [],
  meta: { total: 0, page: 1, limit: 10, total_pages: 0 },
}

describe('IuranPayment Regression Tests', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.spyOn(apiModule, 'getSessionPair').mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('1. Loading berubah menjadi form pembayaran saat request berhasil', async () => {
    vi.spyOn(apiModule, 'apiGetBill').mockResolvedValue(mockBillResponse as any)
    vi.spyOn(apiModule, 'apiListPayments').mockResolvedValue(mockPaymentsResponse as any)

    render(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={true} />
      </MemoryRouter>
    )

    // Initially displays loading
    expect(screen.getByText('Memuat data tagihan...')).toBeInTheDocument()

    // Transitions to payment form upon success
    await waitFor(() => {
      expect(screen.queryByText('Memuat data tagihan...')).not.toBeInTheDocument()
    })

    expect(screen.getByText('Pembayaran Iuran')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Budi Santoso')).toBeInTheDocument()
    // Nominal is formatted as 1.000.000
    const amountInput = screen.getByLabelText(/Nominal Pembayaran/i)
    expect(amountInput).toHaveValue('1.000.000')
  })

  it('2. Loading berubah menjadi pesan error saat request gagal', async () => {
    vi.spyOn(apiModule, 'apiGetBill').mockRejectedValue(new Error('Koneksi server terputus'))

    render(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={true} />
      </MemoryRouter>
    )

    expect(screen.getByText('Memuat data tagihan...')).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.queryByText('Memuat data tagihan...')).not.toBeInTheDocument()
    })

    expect(screen.getByText('Gagal Memuat Tagihan')).toBeInTheDocument()
    expect(screen.getByText('Koneksi server terputus')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Coba Lagi' })).toBeInTheDocument()
  })

  it('3. Loading berubah menjadi tampilan not-found ketika tagihan tidak tersedia', async () => {
    const notFoundErr = { code: 'not_found', message: 'Bill not found' }
    vi.spyOn(apiModule, 'apiGetBill').mockRejectedValue(notFoundErr)

    render(
      <MemoryRouter>
        <IuranPayment billId="bill-999" open={true} />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.queryByText('Memuat data tagihan...')).not.toBeInTheDocument()
    })

    expect(screen.getByText('Tagihan Tidak Ditemukan')).toBeInTheDocument()
    expect(screen.getByText('Tagihan iuran yang Anda buka tidak ditemukan.')).toBeInTheDocument()
  })

  it('4. Modal pembayaran dapat dibuka dan ditutup dengan benar', async () => {
    vi.spyOn(apiModule, 'apiGetBill').mockResolvedValue(mockBillResponse as any)
    vi.spyOn(apiModule, 'apiListPayments').mockResolvedValue(mockPaymentsResponse as any)

    const onClose = vi.fn()

    const { rerender } = render(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={true} onClose={onClose} />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Pembayaran Iuran')).toBeInTheDocument()
    })

    // Click Batal
    fireEvent.click(screen.getByRole('button', { name: 'Batal' }))
    expect(onClose).toHaveBeenCalledTimes(1)

    // When open=false, modal is unmounted / not rendered
    rerender(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={false} onClose={onClose} />
      </MemoryRouter>
    )
    expect(screen.queryByText('Pembayaran Iuran')).not.toBeInTheDocument()
  })

  it('5. Setelah mencoba kembali (Coba Lagi), UI dapat pulih dari error', async () => {
    const getBillMock = vi.spyOn(apiModule, 'apiGetBill')
      .mockRejectedValueOnce(new Error('Koneksi terputus'))
      .mockResolvedValueOnce(mockBillResponse as any)

    vi.spyOn(apiModule, 'apiListPayments').mockResolvedValue(mockPaymentsResponse as any)

    render(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={true} />
      </MemoryRouter>
    )

    // First attempt fails
    await waitFor(() => {
      expect(screen.getByText('Koneksi terputus')).toBeInTheDocument()
    })

    // Click Coba Lagi
    const retryBtn = screen.getByRole('button', { name: 'Coba Lagi' })
    fireEvent.click(retryBtn)

    // Recovers and displays payment form
    await waitFor(() => {
      expect(screen.getByText('Pembayaran Iuran')).toBeInTheDocument()
    })
    expect(getBillMock).toHaveBeenCalledTimes(2)
  })

  it('6. Format nominal dan payload API tetap benar (mengirim string numerik tanpa titik)', async () => {
    vi.spyOn(apiModule, 'apiGetBill').mockResolvedValue(mockBillResponse as any)
    vi.spyOn(apiModule, 'apiListPayments').mockResolvedValue(mockPaymentsResponse as any)
    const createPaymentMock = vi.spyOn(apiModule, 'apiCreatePayment').mockResolvedValue({
      id: 'pay-1',
      bill_id: 'bill-123',
      amount: '500000',
      method: 'CASH',
      status: 'PENDING',
      created_at: '2026-10-13T00:00:00Z',
    } as any)

    render(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={true} />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Pembayaran Iuran')).toBeInTheDocument()
    })

    const amountInput = screen.getByLabelText(/Nominal Pembayaran/i)

    // Change amount to 500.000
    fireEvent.change(amountInput, { target: { value: '500000' } })
    expect(amountInput).toHaveValue('500.000')

    // Select payment method Tunai
    const cashBtn = screen.getByRole('button', { name: 'Tunai' })
    fireEvent.click(cashBtn)

    // Submit payment
    const submitBtn = screen.getByRole('button', { name: 'Bayar' })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(createPaymentMock).toHaveBeenCalledTimes(1)
    })

    expect(createPaymentMock).toHaveBeenCalledWith('test-token', expect.objectContaining({
      bill_id: 'bill-123',
      amount: '500000', // Pure numeric string without Rp and without dots
      method: 'CASH',
    }))
  })

  it('7. Tombol Batal tampil dan dapat menutup modal saat proses loading', async () => {
    // Hang the promise to stay in loading state
    vi.spyOn(apiModule, 'apiGetBill').mockImplementation(() => new Promise(() => {}))
    const onClose = vi.fn()

    render(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={true} onClose={onClose} />
      </MemoryRouter>
    )

    expect(screen.getByText('Memuat data tagihan...')).toBeInTheDocument()
    const cancelBtn = screen.getByRole('button', { name: 'Batal' })
    expect(cancelBtn).toBeInTheDocument()

    fireEvent.click(cancelBtn)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('8. Kedua tombol (Bayar & Batal) tetap terlihat dan disabled saat submitting', async () => {
    vi.spyOn(apiModule, 'apiGetBill').mockResolvedValue(mockBillResponse as any)
    vi.spyOn(apiModule, 'apiListPayments').mockResolvedValue(mockPaymentsResponse as any)

    let resolvePayment: (val: any) => void = () => {}
    vi.spyOn(apiModule, 'apiCreatePayment').mockImplementation(
      () => new Promise((resolve) => { resolvePayment = resolve })
    )

    render(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={true} />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Pembayaran Iuran')).toBeInTheDocument()
    })

    // Pilih metode pembayaran
    fireEvent.click(screen.getByRole('button', { name: 'Tunai' }))

    // Klik Bayar
    const bayarBtn = screen.getByRole('button', { name: 'Bayar' })
    fireEvent.click(bayarBtn)

    // Saat submitting: tombol Bayar berubah menjadi 'Menyimpan...' dan disabled, tombol Batal disabled
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Menyimpan...' })).toBeInTheDocument()
    })

    const submittingBayarBtn = screen.getByRole('button', { name: 'Menyimpan...' })
    const batalBtn = screen.getByRole('button', { name: 'Batal' })

    expect(submittingBayarBtn).toBeDisabled()
    expect(batalBtn).toBeDisabled()
    // Keduanya tetap ada di dokumen (tidak hilang)
    expect(submittingBayarBtn).toBeInTheDocument()
    expect(batalBtn).toBeInTheDocument()

    // Selesaikan payment
    resolvePayment({
      id: 'pay-done',
      bill_id: 'bill-123',
      amount: '1000000',
      status: 'APPROVED',
      created_at: '2026-10-13T00:00:00Z',
    })
  })

  it('9. Validasi form: tombol Bayar memvalidasi nominal dan metode jika belum valid', async () => {
    vi.spyOn(apiModule, 'apiGetBill').mockResolvedValue(mockBillResponse as any)
    vi.spyOn(apiModule, 'apiListPayments').mockResolvedValue(mockPaymentsResponse as any)
    const createPaymentMock = vi.spyOn(apiModule, 'apiCreatePayment')

    render(
      <MemoryRouter>
        <IuranPayment billId="bill-123" open={true} />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Pembayaran Iuran')).toBeInTheDocument()
    })

    // Kosongkan nominal
    const amountInput = screen.getByLabelText(/Nominal Pembayaran/i)
    fireEvent.change(amountInput, { target: { value: '' } })

    // Klik Bayar tanpa memilih metode dan tanpa nominal
    const bayarBtn = screen.getByRole('button', { name: 'Bayar' })
    fireEvent.click(bayarBtn)

    // Muncul pesan validasi
    expect(screen.getByText('Nominal pembayaran harus diisi.')).toBeInTheDocument()
    expect(screen.getByText('Metode pembayaran harus dipilih.')).toBeInTheDocument()
    expect(createPaymentMock).not.toHaveBeenCalled()
  })

  it('10. State notFound dan error tetap memiliki tombol Tutup yang memanggil onClose', async () => {
    const onClose = vi.fn()
    vi.spyOn(apiModule, 'apiGetBill').mockRejectedValue({ code: 'not_found' })

    const { rerender } = render(
      <MemoryRouter>
        <IuranPayment billId="bill-999" open={true} onClose={onClose} />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Tagihan Tidak Ditemukan')).toBeInTheDocument()
    })

    const tutupBtn = screen.getByRole('button', { name: 'Tutup' })
    expect(tutupBtn).toBeInTheDocument()
    fireEvent.click(tutupBtn)
    expect(onClose).toHaveBeenCalledTimes(1)

    // Error state
    vi.spyOn(apiModule, 'apiGetBill').mockRejectedValue(new Error('Koneksi putus'))
    rerender(
      <MemoryRouter>
        <IuranPayment billId="bill-err" open={true} onClose={onClose} />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Gagal Memuat Tagihan')).toBeInTheDocument()
    })

    const tutupErrorBtn = screen.getByRole('button', { name: 'Tutup' })
    expect(tutupErrorBtn).toBeInTheDocument()
    fireEvent.click(tutupErrorBtn)
    expect(onClose).toHaveBeenCalledTimes(2)
  })
})
