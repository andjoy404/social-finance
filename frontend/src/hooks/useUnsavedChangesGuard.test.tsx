import { useState, useRef } from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { isSnapshotDirty, useUnsavedChangesGuard } from './useUnsavedChangesGuard'
import { UnsavedChangesProvider, useUnsavedChangesContext } from '@/contexts/UnsavedChangesContext'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { AnnouncementModal } from '@/features/pengumuman/AnnouncementModal'
import { ActivityModal } from '@/features/kegiatan/ActivityModal'

// Mock BannerStyleEditor lightly so AnnouncementModal/ActivityModal tests don't depend on canvas/cropper
vi.mock('@/components/BannerStyleEditor', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/components/BannerStyleEditor')>()
  return {
    ...actual,
    BannerStyleEditor: () => <div data-testid="banner-style-editor-stub" />,
  }
})

function TestFormHarness({
  onExit,
  onSaveResult = 'success',
}: {
  onExit: () => void
  onSaveResult?: 'success' | 'fail' | 'pending'
}) {
  const initialRef = useRef({ name: '', checked: false })
  const [name, setName] = useState('')
  const [checked, setChecked] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const ctx = useUnsavedChangesContext()

  const isDirty = isSnapshotDirty({ name, checked }, initialRef.current)
  const { confirmAction, exitAfterSave, confirmModal } = useUnsavedChangesGuard({
    isDirty,
    isSaving: saving,
  })

  const handleSave = async () => {
    setSaving(true)
    setSaveError(null)
    if (onSaveResult === 'pending') {
      return
    }
    if (onSaveResult === 'fail') {
      setSaving(false)
      setSaveError('Gagal menyimpan')
      return
    }
    setSaving(false)
    exitAfterSave(onExit)
  }

  return (
    <div>
      <input
        aria-label="Nama"
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <input
        aria-label="Aktif"
        type="checkbox"
        checked={checked}
        onChange={(e) => setChecked(e.target.checked)}
      />
      {saveError && <div role="alert">{saveError}</div>}
      <button type="button" onClick={() => void confirmAction(onExit)}>
        Batal
      </button>
      <button type="button" onClick={() => void handleSave()}>
        Simpan
      </button>
      <button
        type="button"
        onClick={() => {
          void ctx?.confirmNavigation(onExit)
        }}
      >
        Navigasi Sidebar
      </button>
      {confirmModal}
    </div>
  )
}

function TestModalHarness({
  onClosed,
  isSaving = false,
}: {
  onClosed: () => void
  isSaving?: boolean
}) {
  const [open, setOpen] = useState(true)
  const [value, setValue] = useState('')
  const isDirty = isSnapshotDirty({ value }, { value: '' })

  return (
    <>
      <Modal
        open={open}
        onClose={() => {
          setOpen(false)
          onClosed()
        }}
        isDirty={isDirty}
        isSaving={isSaving}
      >
        <ModalHeader title="Form Modal Uji" onClose={() => { setOpen(false); onClosed() }} />
        <ModalBody>
          <input
            aria-label="Catatan"
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
        </ModalBody>
        <ModalFooter>
          <button
            type="button"
            onClick={() => {
              setOpen(false)
              onClosed()
            }}
          >
            Direct Close
          </button>
        </ModalFooter>
      </Modal>
    </>
  )
}

describe('Unsaved Changes Navigation Guard Standard (Web)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('isSnapshotDirty', () => {
    it('returns false for identical primitives, objects, arrays, Dates, and Files', () => {
      const d1 = new Date('2026-05-01T10:00:00Z')
      const d2 = new Date('2026-05-01T10:00:00Z')
      const f1 = new File(['abc'], 'doc.pdf', { lastModified: 1000 })
      const f2 = new File(['abc'], 'doc.pdf', { lastModified: 1000 })

      expect(isSnapshotDirty({ a: 'x', b: true, d: d1, f: f1 }, { b: true, a: 'x', d: d2, f: f2 })).toBe(false)
    })

    it('returns true when changed and false again when reverted to initial snapshot', () => {
      const initial = { title: 'Awal', active: false, tags: ['a'] }
      expect(isSnapshotDirty({ title: 'Diubah', active: false, tags: ['a'] }, initial)).toBe(true)
      expect(isSnapshotDirty({ title: 'Awal', active: true, tags: ['a'] }, initial)).toBe(true)
      expect(isSnapshotDirty({ title: 'Awal', active: false, tags: ['a'] }, initial)).toBe(false)
    })
  })

  describe('Form lifecycle: clean, dirty, reverted, saving, save success & failure', () => {
    it('clean state exits immediately without showing confirmation dialog', () => {
      const onExit = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestFormHarness onExit={onExit} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      fireEvent.click(screen.getByText('Batal'))
      expect(onExit).toHaveBeenCalledTimes(1)
      expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
    })

    it('dirty state prompts standard confirmation; Tetap di Halaman preserves input; Ya, Batalkan exits', async () => {
      const onExit = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestFormHarness onExit={onExit} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      const input = screen.getByLabelText('Nama') as HTMLInputElement
      fireEvent.change(input, { target: { value: 'Perubahan Penting' } })

      // Click Batal -> confirmation dialog with exact standard copy
      fireEvent.click(screen.getByText('Batal'))
      expect(await screen.findByText('Apakah Anda yakin ingin membatalkan?')).toBeInTheDocument()
      expect(screen.getByText('Perubahan yang belum disimpan akan hilang.')).toBeInTheDocument()
      expect(screen.getByTestId('unsaved-stay-btn')).toHaveTextContent('Tetap di Halaman')
      expect(screen.getByTestId('unsaved-confirm-btn')).toHaveTextContent('Ya, Batalkan')
      expect(onExit).not.toHaveBeenCalled()

      // Stay preserves user input
      fireEvent.click(screen.getByTestId('unsaved-stay-btn'))
      await waitFor(() => {
        expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
      })
      expect(input.value).toBe('Perubahan Penting')
      expect(onExit).not.toHaveBeenCalled()

      // Sidebar navigation while still dirty triggers single dialog -> Ya, Batalkan proceeds
      fireEvent.click(screen.getByText('Navigasi Sidebar'))
      expect(await screen.findByText('Apakah Anda yakin ingin membatalkan?')).toBeInTheDocument()
      fireEvent.click(screen.getByTestId('unsaved-confirm-btn'))
      await waitFor(() => {
        expect(onExit).toHaveBeenCalledTimes(1)
      })
    })

    it('value changed then reverted back to initial snapshot becomes clean and exits immediately', () => {
      const onExit = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestFormHarness onExit={onExit} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      const input = screen.getByLabelText('Nama')
      fireEvent.change(input, { target: { value: 'Sementara' } })
      fireEvent.change(input, { target: { value: '' } })

      fireEvent.click(screen.getByText('Batal'))
      expect(onExit).toHaveBeenCalledTimes(1)
      expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
    })

    it('saving state blocks all exit paths silently without dialog', async () => {
      const onExit = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestFormHarness onExit={onExit} onSaveResult="pending" />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Menyimpan' } })
      fireEvent.click(screen.getByText('Simpan'))

      fireEvent.click(screen.getByText('Batal'))
      fireEvent.click(screen.getByText('Navigasi Sidebar'))

      expect(onExit).not.toHaveBeenCalled()
      expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
    })

    it('save success exits cleanly without cancel dialog via exitAfterSave', async () => {
      const onExit = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestFormHarness onExit={onExit} onSaveResult="success" />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Data Valid' } })
      fireEvent.click(screen.getByText('Simpan'))

      await waitFor(() => {
        expect(onExit).toHaveBeenCalledTimes(1)
      })
      expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
    })

    it('save failure preserves inputs and keeps guard active', async () => {
      const onExit = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestFormHarness onExit={onExit} onSaveResult="fail" />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      const input = screen.getByLabelText('Nama') as HTMLInputElement
      fireEvent.change(input, { target: { value: 'Data Gagal' } })
      fireEvent.click(screen.getByText('Simpan'))

      expect(await screen.findByRole('alert')).toHaveTextContent('Gagal menyimpan')
      expect(input.value).toBe('Data Gagal')
      expect(onExit).not.toHaveBeenCalled()

      // Guard must still protect unsaved input
      fireEvent.click(screen.getByText('Batal'))
      expect(await screen.findByText('Apakah Anda yakin ingin membatalkan?')).toBeInTheDocument()
    })

    it('deduplicates concurrent confirmation triggers so double dialogs never appear', async () => {
      const onExit = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestFormHarness onExit={onExit} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Dirty' } })
      fireEvent.click(screen.getByText('Batal'))
      fireEvent.click(screen.getByText('Navigasi Sidebar'))

      const dialogs = await screen.findAllByText('Apakah Anda yakin ingin membatalkan?')
      expect(dialogs).toHaveLength(1)
    })

    it('beforeunload calls preventDefault when dirty and does not when clean', () => {
      const onExit = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestFormHarness onExit={onExit} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      const cleanEvent = new Event('beforeunload', { cancelable: true })
      window.dispatchEvent(cleanEvent)
      expect(cleanEvent.defaultPrevented).toBe(false)

      fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Dirty' } })
      const dirtyEvent = new Event('beforeunload', { cancelable: true })
      window.dispatchEvent(dirtyEvent)
      expect(dirtyEvent.defaultPrevented).toBe(true)
    })
  })

  describe('Modal guard (Escape, backdrop, X button, saving, CancelConfirmModal Escape)', () => {
    it('closes immediately on Escape when clean, and prompts on Escape/backdrop/X when dirty', async () => {
      const onClosedClean = vi.fn()
      const { unmount } = render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestModalHarness onClosed={onClosedClean} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      fireEvent.keyDown(document, { key: 'Escape' })
      expect(onClosedClean).toHaveBeenCalledTimes(1)
      expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
      unmount()

      const onClosedDirty = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestModalHarness onClosed={onClosedDirty} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      const input = screen.getByLabelText('Catatan') as HTMLInputElement
      fireEvent.change(input, { target: { value: 'Catatan baru' } })

      // Escape triggers confirmation dialog
      fireEvent.keyDown(document, { key: 'Escape' })
      expect(await screen.findByText('Apakah Anda yakin ingin membatalkan?')).toBeInTheDocument()
      expect(onClosedDirty).not.toHaveBeenCalled()

      // Pressing Escape while CancelConfirmModal is open stays on page (does NOT discard)
      fireEvent.keyDown(window, { key: 'Escape' })
      await waitFor(() => {
        expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
      })
      expect(onClosedDirty).not.toHaveBeenCalled()
      expect(input.value).toBe('Catatan baru')

      // Backdrop click triggers confirmation dialog -> Ya, Batalkan closes modal
      const overlay = document.querySelector('.sf-modal-overlay')!
      fireEvent.click(overlay)
      expect(await screen.findByText('Apakah Anda yakin ingin membatalkan?')).toBeInTheDocument()
      fireEvent.click(screen.getByTestId('unsaved-confirm-btn'))
      await waitFor(() => {
        expect(onClosedDirty).toHaveBeenCalledTimes(1)
      })
    })

    it('blocks Escape, backdrop, and X close when isSaving is true without showing dialog', () => {
      const onClosed = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <TestModalHarness onClosed={onClosed} isSaving />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      fireEvent.change(screen.getByLabelText('Catatan'), { target: { value: 'Menyimpan' } })
      fireEvent.keyDown(document, { key: 'Escape' })
      const overlay = document.querySelector('.sf-modal-overlay')!
      fireEvent.click(overlay)
      const closeBtn = screen.getByLabelText('Tutup')
      fireEvent.click(closeBtn)

      expect(onClosed).not.toHaveBeenCalled()
      expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
    })
  })

  describe('Real modal regressions: AnnouncementModal & ActivityModal snapshot bug fix', () => {
    it('AnnouncementModal starts clean (no false dirty from styleConfig), prompts when dirty, and resets to clean on revert', async () => {
      const onClose = vi.fn()
      const { unmount } = render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <AnnouncementModal templates={[]} onClose={onClose} onSuccess={vi.fn()} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      // Freshly opened modal is CLEAN -> Batal closes immediately
      fireEvent.click(screen.getByRole('button', { name: 'Batal' }))
      expect(onClose).toHaveBeenCalledTimes(1)
      expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
      unmount()

      const onClose2 = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <AnnouncementModal templates={[]} onClose={onClose2} onSuccess={vi.fn()} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      const titleInput = screen.getByPlaceholderText(/Contoh: Kerja Bakti/i) as HTMLInputElement
      fireEvent.change(titleInput, { target: { value: 'Rapat RT' } })

      // Dirty -> Escape triggers confirmation
      fireEvent.keyDown(document, { key: 'Escape' })
      expect(await screen.findByText('Apakah Anda yakin ingin membatalkan?')).toBeInTheDocument()
      expect(onClose2).not.toHaveBeenCalled()

      // Stay preserves input
      fireEvent.click(screen.getByTestId('unsaved-stay-btn'))
      await waitFor(() => {
        expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
      })
      expect(titleInput.value).toBe('Rapat RT')

      // Revert back to empty -> clean -> Batal exits immediately
      fireEvent.change(titleInput, { target: { value: '' } })
      fireEvent.click(screen.getByRole('button', { name: 'Batal' }))
      expect(onClose2).toHaveBeenCalledTimes(1)
    })

    it('ActivityModal starts clean (no false dirty from styleConfig), prompts when dirty, and resets to clean on revert', async () => {
      const onClose = vi.fn()
      const { unmount } = render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <ActivityModal onClose={onClose} onSuccess={vi.fn()} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      // Freshly opened modal is CLEAN -> Batal closes immediately
      fireEvent.click(screen.getByRole('button', { name: 'Batal' }))
      expect(onClose).toHaveBeenCalledTimes(1)
      expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
      unmount()

      const onClose2 = vi.fn()
      render(
        <MemoryRouter>
          <UnsavedChangesProvider>
            <ActivityModal onClose={onClose2} onSuccess={vi.fn()} />
          </UnsavedChangesProvider>
        </MemoryRouter>,
      )

      const titleInput = screen.getByPlaceholderText(/Contoh: Kerja Bakti/i) as HTMLInputElement
      fireEvent.change(titleInput, { target: { value: 'Kerja Bakti Minggu' } })

      // Dirty -> Batal triggers confirmation
      fireEvent.click(screen.getByRole('button', { name: 'Batal' }))
      expect(await screen.findByText('Apakah Anda yakin ingin membatalkan?')).toBeInTheDocument()
      fireEvent.click(screen.getByTestId('unsaved-stay-btn'))
      await waitFor(() => {
        expect(screen.queryByText('Apakah Anda yakin ingin membatalkan?')).not.toBeInTheDocument()
      })
      expect(titleInput.value).toBe('Kerja Bakti Minggu')

      // Revert back to empty -> clean -> exits immediately
      fireEvent.change(titleInput, { target: { value: '' } })
      fireEvent.click(screen.getByRole('button', { name: 'Batal' }))
      expect(onClose2).toHaveBeenCalledTimes(1)
    })
  })
})
