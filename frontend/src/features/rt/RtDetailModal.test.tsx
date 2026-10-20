import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import '@testing-library/jest-dom'
import { persistSessionPair } from '@/app/api'
import { RtDetailModal } from './RtDetailModal'

const mockRt = {
  id: 'rt-1',
  name: 'RT 001',
  rw: 16,
  rt: '001',
  rw_name: 'RW 016',
  address: 'Jl. Melati',
  head_name: 'Ketua RT',
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockMembers = [
  {
    membership_id: 'm-1',
    user_id: 'u-1',
    full_name: 'Budi Santoso',
    email: 'budi@test.com',
    phone: '081234',
    role: 'warga',
    jabatan: null,
    is_active: true,
  },
  {
    membership_id: 'm-2',
    user_id: 'u-2',
    full_name: 'Siti Aminah',
    email: 'siti@test.com',
    phone: '085678',
    role: 'warga',
    jabatan: 'ketua',
    is_active: true,
  },
]

const mockRw = {
  id: 'rw-16',
  rw_number: 16,
  name: 'RW 016',
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
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

function mockFetch(handler: (input: string, init?: RequestInit) => Response | Promise<Response>) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(
    async (input, init) => handler(typeof input === 'string' ? input : '', init),
  )
}

function renderModal(open = true) {
  return render(<RtDetailModal open={open} rt={mockRt} onClose={vi.fn()} />)
}

describe('RtDetailModal', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-token', refreshToken: 'test-refresh' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows UBAH and JABATAN tabs, keeping the current assignment content in JABATAN', async () => {
    mockFetch((input) => input.includes('/rws')
      ? okResponse({ data: [mockRw], pagination: { page: 1, page_size: 100, total: 1, total_pages: 1 } })
      : okResponse({ data: mockMembers }))
    renderModal()

    expect(screen.getByRole('tab', { name: 'JABATAN' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'UBAH' })).toHaveAttribute('aria-selected', 'false')
    expect(await screen.findByText('Tetapkan Jabatan RT')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'UBAH' }))
    expect(await screen.findByLabelText('Nama RT *')).toHaveValue('RT 001')
    expect(screen.getByLabelText('RW *')).toHaveValue('rw-16')
    expect(screen.queryByText('Tetapkan Jabatan RT')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'JABATAN' }))
    expect(screen.getByText('Tetapkan Jabatan RT')).toBeInTheDocument()
  })

  it('updates RT information through the existing RT API and preserves edits on failure', async () => {
    const onUpdated = vi.fn()
    let updateCalls = 0
    let updateInit: RequestInit | undefined
    mockFetch((input, init) => {
      if (input.includes('/rws')) {
        return okResponse({ data: [mockRw], pagination: { page: 1, page_size: 100, total: 1, total_pages: 1 } })
      }
      if (input.includes('/rts/rt-1') && init?.method === 'PATCH') {
        updateCalls += 1
        updateInit = init
        return updateCalls === 1
          ? failResponse(500, { code: 'unexpected', message: 'Update failed' })
          : okResponse({ ...mockRt, name: 'RT Melati', rw_id: 'rw-16' })
      }
      return okResponse({ data: mockMembers })
    })
    render(<RtDetailModal open rt={mockRt} onClose={vi.fn()} onUpdated={onUpdated} />)

    fireEvent.click(screen.getByRole('tab', { name: 'UBAH' }))
    const nameInput = await screen.findByLabelText('Nama RT *')
    fireEvent.change(nameInput, { target: { value: 'RT Melati' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Perubahan' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Update failed')
    expect(nameInput).toHaveValue('RT Melati')
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Perubahan' }))

    await waitFor(() => expect(onUpdated).toHaveBeenCalledWith(expect.objectContaining({ name: 'RT Melati' })))
    expect(updateInit).toEqual(expect.objectContaining({
      method: 'PATCH',
      body: JSON.stringify({
        name: 'RT Melati',
        rw_id: 'rw-16',
        rw: 16,
        rt: '001',
        address: 'Jl. Melati',
        head_name: 'Ketua RT',
      }),
    }))
    expect(await screen.findByRole('status')).toHaveTextContent('Informasi RT berhasil diperbarui.')
  })

  it('uses the staged assignment layout and only the available RT positions', async () => {
    mockFetch(() => okResponse({ data: mockMembers }))
    renderModal()

    expect(await screen.findByText('Tetapkan Jabatan RT')).toBeInTheDocument()
    expect(screen.getByLabelText('Jabatan')).toBeInTheDocument()
    expect(screen.getByText('Warga', { selector: 'label' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Pilih warga' })).toBeInTheDocument()
    const options = within(screen.getByLabelText('Jabatan')).getAllByRole('option')
    expect(options.slice(1).map((option) => (option as HTMLOptionElement).value)).toEqual([
      'ketua',
      'wakil_ketua',
      'sekretaris',
      'bendahara',
      'keamanan',
      'sosial',
      'kebersihan_pembangunan',
    ])
  })

  it('searches actual RT members and updates the assignment summary as choices change', async () => {
    mockFetch(() => okResponse({ data: mockMembers }))
    renderModal()

    await screen.findByText('Pengurus RT')
    fireEvent.change(screen.getByLabelText('Jabatan'), { target: { value: 'bendahara' } })
    fireEvent.click(screen.getByRole('button', { name: 'Pilih warga' }))

    const search = screen.getByRole('textbox', { name: 'Cari warga berdasarkan nama, email, telepon, atau ID' })
    fireEvent.change(search, { target: { value: '085678' } })
    expect(screen.queryByRole('button', { name: /Budi Santoso.*budi@test.com/ })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Siti Aminah.*siti@test.com/ }))

    const summary = document.querySelector('[aria-live="polite"]')
    expect(summary).toHaveTextContent('Bendahara')
    expect(summary).toHaveTextContent('siti@test.com')
    expect(summary).toHaveTextContent('085678')
    expect(screen.getByRole('button', { name: 'Ganti warga, Siti Aminah' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Ganti warga, Siti Aminah' }))
    fireEvent.click(screen.getByRole('button', { name: /Budi Santoso.*budi@test.com/ }))
    expect(summary).toHaveTextContent('budi@test.com')
    expect(summary).toHaveTextContent('081234')
  })

  it('sends RT assignment to the RT API with the selected user and jabatan', async () => {
    let assignmentInit: RequestInit | undefined
    const fetchMock = vi.fn((input: string, init?: RequestInit) => {
      if (input.includes('/rts/rt-1/memberships/u-1/jabatan')) {
        assignmentInit = init
        return Promise.resolve(okResponse({ ...mockMembers[0], jabatan: 'ketua' }))
      }
      return Promise.resolve(okResponse({ data: mockMembers }))
    })
    mockFetch(fetchMock)
    renderModal()

    await screen.findByText('Pengurus RT')
    fireEvent.change(screen.getByLabelText('Jabatan'), { target: { value: 'ketua' } })
    fireEvent.click(screen.getByRole('button', { name: 'Pilih warga' }))
    fireEvent.click(await screen.findByRole('button', { name: /Budi Santoso.*budi@test.com/ }))
    fireEvent.submit(document.querySelector('form')!)

    await waitFor(() => {
      expect(assignmentInit).toEqual(expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ jabatan: 'ketua' }),
      }))
    })
    expect(await screen.findByRole('status')).toHaveTextContent('Jabatan berhasil disimpan.')
  })

  it('keeps the selected jabatan and member when assignment fails', async () => {
    mockFetch((input) => {
      if (input.includes('/rts/rt-1/memberships/u-1/jabatan')) {
        return failResponse(500, { message: 'Assignment failed' })
      }
      return okResponse({ data: mockMembers })
    })
    renderModal()

    await screen.findByText('Pengurus RT')
    fireEvent.change(screen.getByLabelText('Jabatan'), { target: { value: 'ketua' } })
    fireEvent.click(screen.getByRole('button', { name: 'Pilih warga' }))
    fireEvent.click(await screen.findByRole('button', { name: /Budi Santoso.*budi@test.com/ }))
    fireEvent.submit(document.querySelector('form')!)

    expect(await screen.findByText('Unexpected response')).toBeInTheDocument()
    expect(screen.getByLabelText('Jabatan')).toHaveValue('ketua')
    expect(screen.getByRole('button', { name: 'Ganti warga, Budi Santoso' })).toBeInTheDocument()
  })

  it('shows a loading state and supports retry after the membership request fails', async () => {
    let calls = 0
    mockFetch(() => {
      calls += 1
      return calls === 1
        ? failResponse(500, { message: 'Server error' })
        : okResponse({ data: mockMembers })
    })
    renderModal()

    expect(await screen.findByText('Unexpected response')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Coba Lagi' }))
    // After retry, members with jabatan are displayed (Budi has no jabatan, so only Siti Aminah)
    expect(await screen.findByText('Siti Aminah')).toBeInTheDocument()
    expect(calls).toBe(2)
  })

  it('shows a loading state while RT members are being fetched', () => {
    mockFetch(() => new Promise<Response>(() => {}))
    renderModal()
    expect(screen.getByRole('status')).toHaveTextContent('Memuat pengurus...')
  })

  it('does not submit duplicate RT assignment requests while a save is pending', async () => {
    let resolveAssignment!: (response: Response) => void
    const fetchMock = vi.fn((input: string) => {
      if (input.includes('/rts/rt-1/memberships/u-1/jabatan')) {
        return new Promise<Response>((resolve) => { resolveAssignment = resolve })
      }
      return Promise.resolve(okResponse({ data: mockMembers }))
    })
    mockFetch(fetchMock)
    renderModal()

    await screen.findByText('Pengurus RT')
    fireEvent.change(screen.getByLabelText('Jabatan'), { target: { value: 'ketua' } })
    fireEvent.click(screen.getByRole('button', { name: 'Pilih warga' }))
    fireEvent.click(await screen.findByRole('button', { name: /Budi Santoso.*budi@test.com/ }))
    const form = document.querySelector('form')!
    fireEvent.submit(form)
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2))
    fireEvent.submit(form)
    expect(fetchMock.mock.calls.filter(([url]) => url.includes('/jabatan'))).toHaveLength(1)

    resolveAssignment(okResponse({ ...mockMembers[0], jabatan: 'ketua' }))
    await screen.findByText('Jabatan berhasil disimpan.')
  })

  it('requires confirmation before sending a jabatan revoke request', async () => {
    const fetchMock = vi.fn((input: string, init?: RequestInit) => {
      if (init?.method === 'DELETE') return Promise.resolve(okResponse({}))
      if (input.includes('/rts/rt-1/memberships')) return Promise.resolve(okResponse({ data: mockMembers }))
      return Promise.resolve(okResponse({}))
    })
    mockFetch(fetchMock)
    renderModal()

    await screen.findByText('Pengurus RT')
    fireEvent.click(screen.getByRole('button', { name: 'Cabut' }))
    expect(screen.getByText('Cabut jabatan Ketua dari Siti Aminah?')).toBeInTheDocument()
    expect(fetchMock.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(false)

    fireEvent.click(screen.getByRole('button', { name: 'Ya, Cabut' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(true)
    })
    expect(await screen.findByRole('status')).toHaveTextContent('Jabatan berhasil dicabut.')
  })

  it('does not make assignment requests when the modal is closed', () => {
    const fetchMock = vi.fn()
    mockFetch(fetchMock)
    renderModal(false)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('only shows members with jabatan in the Pengurus RT section', async () => {
    mockFetch(() => okResponse({ data: mockMembers }))
    renderModal()

    // Member with jabatan should be visible
    expect(await screen.findByText('Siti Aminah')).toBeInTheDocument()
    // Member without jabatan should NOT be visible in Pengurus RT
    expect(screen.queryByText('Budi Santoso')).not.toBeInTheDocument()
    expect(screen.queryByText('Tanpa jabatan')).not.toBeInTheDocument()
    // Badge should use correct color variant for ketua
    expect(screen.queryByText('Tanpa Jabatan')).not.toBeInTheDocument()
  })

  it('shows empty state when all members lack jabatan', async () => {
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
    mockFetch(() => okResponse({ data: noJabatanMembers }))
    renderModal()

    expect(await screen.findByText('Belum ada pengurus RT yang ditetapkan.')).toBeInTheDocument()
    expect(screen.queryByText('Belum ada pengurus RT.')).not.toBeInTheDocument()
  })

  it('shows all members (including those without jabatan) in the warga selector', async () => {
    mockFetch(() => okResponse({ data: mockMembers }))
    renderModal()

    await screen.findByText('Pengurus RT')
    // Open the warga selector
    fireEvent.click(screen.getByRole('button', { name: 'Pilih warga' }))

    // Both members should be selectable, including the one without jabatan
    expect(await screen.findByRole('button', { name: /Budi Santoso.*budi@test.com/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Tanpa jabatan/ })).not.toBeInTheDocument()
  })
})
