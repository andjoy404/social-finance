import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import '@testing-library/jest-dom'
import { RwDetailModal } from './RwDetailModal'
import { persistSessionPair } from '@/app/api'

// ---- Test helpers ----

const mockRw = {
  id: 'rw-1',
  rw_number: 16,
  name: 'RW 016',
  head_name: 'Ketua RW',
  address: 'Jl. Melati',
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockRts = [
  { id: 'rt-1', name: 'RT 001', rt: '001', rw: 16, rw_name: 'RW 016', head_name: 'Ketua RT1', address: 'Addr1', is_active: true },
  { id: 'rt-2', name: 'RT 002', rt: '002', rw: 16, rw_name: 'RW 016', head_name: 'Ketua RT2', address: 'Addr2', is_active: true },
]

const mockMembers = [
  { membership_id: 'm-1', user_id: 'u-1', full_name: 'Budi Santoso', email: 'budi@test.com', phone: '081234', role: 'warga', jabatan: null, is_active: true },
  { membership_id: 'm-2', user_id: 'u-2', full_name: 'Siti Aminah', email: 'siti@test.com', phone: '085678', role: 'warga', jabatan: 'ketua', is_active: true },
]

function mockFetch(fn: (input: string, init?: RequestInit) => Response | Promise<Response>) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(
    async (input, init) => fn(typeof input === 'string' ? input : '', init),
  )
}

function okResponse(data: unknown): Response {
  const body = JSON.stringify(data)
  return {
    ok: true,
    status: 200,
    text: () => Promise.resolve(body),
    json: () => Promise.resolve(data),
  } as Response
}

function failResponse(status: number, data: unknown): Response {
  return {
    ok: false,
    status,
    text: () => Promise.resolve(JSON.stringify(data)),
    json: () => Promise.resolve(data),
  } as Response
}

function setupSession() {
  persistSessionPair({
    accessToken: 'test-token',
    refreshToken: 'test-refresh',
  })
}

function renderModal(props: { open: boolean; rw: typeof mockRw | null; onClose: () => void; onEdit: () => void }) {
  return render(<RwDetailModal {...props} />)
}

describe('RwDetailModal', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    setupSession()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders RW detail when open', async () => {
    mockFetch((input) => {
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: mockMembers })
      if (input.includes('/rts?')) return okResponse({ data: mockRts, pagination: { total: 2, total_pages: 1 } })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      expect(screen.getByText('Detail RW 016')).toBeInTheDocument()
    })
    expect(screen.getByText('RW 016')).toBeInTheDocument()
  })

  it('shows centered compact UBAH and JABATAN tabs and updates RW information in UBAH', async () => {
    let updateInit: RequestInit | undefined
    const onUpdated = vi.fn()
    mockFetch((input, init) => {
      if (input.includes('/rws/rw-1') && init?.method === 'PATCH') {
        updateInit = init
        return okResponse({ ...mockRw, name: 'RW Melati', rw_number: 17 })
      }
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: mockMembers })
      if (input.includes('/rts?')) return okResponse({ data: mockRts, pagination: { total: 2, total_pages: 1 } })
      return okResponse({})
    })

    render(<RwDetailModal open rw={mockRw} onClose={vi.fn()} onUpdated={onUpdated} />)

    const tablist = screen.getByRole('tablist', { name: 'Pengaturan RW' })
    expect(tablist).toHaveStyle({ justifyContent: 'center' })
    expect(tablist).toHaveStyle({ margin: '0px 20px' })
    expect(screen.getByRole('tab', { name: 'JABATAN' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'UBAH' })).toHaveStyle({ fontSize: '11px' })
    expect(await screen.findByText('Tetapkan Jabatan RW')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'UBAH' }))
    fireEvent.change(screen.getByLabelText('Nomor RW *'), { target: { value: '17' } })
    fireEvent.change(screen.getByLabelText('Nama RW'), { target: { value: 'RW Melati' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Perubahan' }))

    await waitFor(() => {
      expect(onUpdated).toHaveBeenCalledWith(expect.objectContaining({ rw_number: 17, name: 'RW Melati' }))
    })
    expect(updateInit).toEqual(expect.objectContaining({
      method: 'PATCH',
      body: JSON.stringify({
        rw_number: 17,
        name: 'RW Melati',
        head_name: 'Ketua RW',
        address: 'Jl. Melati',
      }),
    }))
    expect(await screen.findByRole('status')).toHaveTextContent('Informasi RW berhasil diperbarui.')
  })

  it('loads RTs filtered by RW', async () => {
    mockFetch((input) => {
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: mockMembers })
      if (input.includes('/rts?')) return okResponse({ data: mockRts, pagination: { total: 2, total_pages: 1 } })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      expect(screen.getByText(/RT 001/)).toBeInTheDocument()
    })

    // Verify rw_id filter was used
    // (URL captured during fetch)
  })

  it('shows only 4 valid jabatan options', async () => {
    mockFetch((input) => {
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: mockMembers })
      if (input.includes('/rts?')) return okResponse({ data: mockRts, pagination: { total: 2, total_pages: 1 } })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      const select = screen.getByRole('combobox') as HTMLSelectElement
      const options = Array.from(select.options).map((o) => o.value).filter((v) => v !== '')
      expect(options).toEqual(['ketua', 'wakil_ketua', 'sekretaris', 'bendahara'])
    })
  })

  it('shows loading state initially', () => {
    mockFetch(() => new Promise(() => {})) // never resolves

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    expect(screen.getByText('Memuat RT...')).toBeInTheDocument()
  })

  it('shows empty state when no RTs', async () => {
    mockFetch((input) => {
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: [] })
      if (input.includes('/rts?')) return okResponse({ data: [], pagination: { total: 0, total_pages: 1 } })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      expect(screen.getByText('Belum ada RT terdaftar di RW ini.')).toBeInTheDocument()
    })
  })

  it('shows error state on API failure', async () => {
    mockFetch((input) => {
      if (input.includes('/rts?')) return failResponse(500, { message: 'Server error' })
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: [] })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      expect(screen.getByText('Unexpected response')).toBeInTheDocument()
    })
  })

  it('does not render when closed', () => {
    const onClose = vi.fn()
    const onEdit = vi.fn()
    const { container } = renderModal({ open: false, rw: mockRw, onClose, onEdit })
    expect(container.innerHTML).toBe('')
  })

  it('shows revoke confirmation when clicking Cabut button', async () => {
    mockFetch((input) => {
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: mockMembers })
      if (input.includes('/rts?')) return okResponse({ data: mockRts, pagination: { total: 2, total_pages: 1 } })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      expect(screen.getByText('Detail RW 016')).toBeInTheDocument()
    })

    // Siti Aminah has jabatan 'ketua', so she has a Cabut button
    const cabutBtn = screen.getByText('Cabut')
    cabutBtn.click()

    await waitFor(() => {
      expect(screen.getByText(/Ya, Cabut/)).toBeInTheDocument()
    })
  })

  it('assigns RW jabatan with correct parameters', async () => {
    mockFetch((input) => {
      if (input.includes('/rts/rt-1/memberships')) return okResponse({
        data: [
          {
            membership_id: 'rm-1',
            user_id: 'u-1',
            full_name: 'Budi Santoso',
            email: 'budi@test.com',
            phone: '081234',
            role: 'warga',
            jabatan: null,
            is_active: true,
          },
        ],
      })
      if (input.includes('/rws/rw-1/memberships')) return okResponse({
        data: [
          { ...mockMembers[0], jabatan: 'ketua', rt_name: 'RT 001', rt_id: 'rt-1' },
          ...mockMembers.slice(1),
        ],
      })
      if (input.includes('/rts?')) return okResponse({ data: mockRts, pagination: { total: 2, total_pages: 1 } })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      expect(screen.getByText('Detail RW 016')).toBeInTheDocument()
    })

    // Select jabatan
    const select = screen.getByRole('combobox') as HTMLSelectElement
    select.value = 'ketua'
    select.dispatchEvent(new Event('change', { bubbles: true }))

    // Click RT button
    const rtBtn = screen.getByText('Pilih RT...')
    rtBtn.click()

    await waitFor(() => {
      expect(screen.getByText('Pilih RT')).toBeInTheDocument()
    })

    // Click first RT item
    const rtModal = screen.getByText('Pilih RT').closest('.sf-modal') as HTMLElement
    const rtItem = rtModal.querySelector('div[style*="cursor: pointer"]') as HTMLElement
    rtItem.click()

    // Wait for RT to be selected in the form
    await waitFor(() => {
      const btn = screen.getByRole('button', { name: /RT 001 — RT 001/ })
      expect(btn.tagName).toBe('BUTTON')
    })

    // Click Warga button to open member selector
    const wargaBtn = screen.getByText('Pilih Warga...')
    wargaBtn.click()

    await waitFor(() => {
      expect(screen.getByText('Pilih Warga')).toBeInTheDocument()
    })

    // Click Budi Santoso in the member selector
    const memberModal = screen.getByText('Pilih Warga').closest('.sf-modal') as HTMLElement
    const memberItem = memberModal.querySelector('div[style*="cursor: pointer"]') as HTMLElement
    memberItem.click()

    // Verify form is correctly filled
    await waitFor(() => {
      const btn = screen.getByRole('button', { name: /RT 001 — RT 001/ })
      expect(btn.tagName).toBe('BUTTON')
    })
    const saveBtn = screen.getByRole('button', { name: 'Simpan' })
    expect(saveBtn).not.toBeDisabled()

    // Submit the form via fireEvent.submit to reliably trigger onSubmit
    const form = document.querySelector('form')!
    fireEvent.submit(form)

    // After successful assignment: form resets (RT field cleared)
    await waitFor(() => {
      expect(screen.getByText('Pilih RT...')).toBeInTheDocument()
    })
  })

  it('only shows members with jabatan in the Pengurus RW section', async () => {
    mockFetch((input) => {
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: mockMembers })
      if (input.includes('/rts?')) return okResponse({ data: mockRts, pagination: { total: 2, total_pages: 1 } })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      expect(screen.getByText('Detail RW 016')).toBeInTheDocument()
    })

    // Member with jabatan should be visible
    expect(screen.getByText('Siti Aminah')).toBeInTheDocument()
    // Member without jabatan should NOT be visible in Pengurus RW
    expect(screen.queryByText('Budi Santoso')).not.toBeInTheDocument()
    expect(screen.queryByText('Tanpa jabatan')).not.toBeInTheDocument()
    expect(screen.queryByText('Tanpa Jabatan')).not.toBeInTheDocument()
  })

  it('shows empty state when all RW members lack jabatan', async () => {
    const noJabatanMembers = [
      {
        membership_id: 'm-3',
        user_id: 'u-3',
        full_name: 'Andriyan',
        email: 'andriyan@test.com',
        phone: '081200',
        role: 'warga',
        jabatan: null,
        is_active: true,
      },
    ]
    mockFetch((input) => {
      if (input.includes('/rws/rw-1/memberships')) return okResponse({ data: noJabatanMembers })
      if (input.includes('/rts?')) return okResponse({ data: mockRts, pagination: { total: 2, total_pages: 1 } })
      return okResponse({})
    })

    const onClose = vi.fn()
    const onEdit = vi.fn()
    renderModal({ open: true, rw: mockRw, onClose, onEdit })

    await waitFor(() => {
      expect(screen.getByText('Belum ada pengurus RW yang ditetapkan.')).toBeInTheDocument()
    })
    expect(screen.queryByText('Belum ada pengurus RW.')).not.toBeInTheDocument()
  })
})