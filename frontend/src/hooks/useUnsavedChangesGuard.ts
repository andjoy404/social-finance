import { createElement, useCallback, useEffect, useId, useRef, useState } from 'react'
import { CancelConfirmModal } from '@/components/CancelConfirmModal'
import { useUnsavedChangesContext } from '@/contexts/UnsavedChangesContext'

function normalizeValue(val: unknown): unknown {
  if (val === null || val === undefined) return val ?? null
  if (typeof val === 'string' || typeof val === 'number' || typeof val === 'boolean') {
    return val
  }
  if (val instanceof Date) {
    return val.getTime()
  }
  if (typeof File !== 'undefined' && val instanceof File) {
    return { __file: true, name: val.name, size: val.size, lastModified: val.lastModified }
  }
  if (Array.isArray(val)) {
    return val.map(normalizeValue)
  }
  if (typeof val === 'object') {
    const record = val as Record<string, unknown>
    const sortedKeys = Object.keys(record).sort()
    const out: Record<string, unknown> = {}
    for (const k of sortedKeys) {
      out[k] = normalizeValue(record[k])
    }
    return out
  }
  return String(val)
}

/**
 * Compares the current form values against an initial snapshot.
 * Returns `false` when values match (including when changed and then reverted
 * back to the initial snapshot).
 */
export function isSnapshotDirty(current: unknown, initial: unknown): boolean {
  return JSON.stringify(normalizeValue(current)) !== JSON.stringify(normalizeValue(initial))
}

export interface UseUnsavedChangesGuardOptions {
  /** True when form values differ from the initial snapshot. */
  isDirty: boolean
  /** True while save/upload is in progress; blocks all exits without dialog. */
  isSaving?: boolean
  /** Set to false for modals when closed so they don't block global navigation. */
  active?: boolean
}

export function useUnsavedChangesGuard({
  isDirty,
  isSaving = false,
  active = true,
}: UseUnsavedChangesGuardOptions) {
  const ctx = useUnsavedChangesContext()
  const formId = useId()

  const dirtyRef = useRef(isDirty)
  const savingRef = useRef(isSaving)
  const activeRef = useRef(active)
  const exitedAfterSaveRef = useRef(false)

  dirtyRef.current = isDirty
  savingRef.current = isSaving
  activeRef.current = active

  // Reset post-save flag whenever form re-activates
  useEffect(() => {
    if (active) {
      exitedAfterSaveRef.current = false
    }
  }, [active])

  // Local fallback confirmation dialog when rendered outside UnsavedChangesProvider
  const [localOpen, setLocalOpen] = useState(false)
  const localResolveRef = useRef<((confirmed: boolean) => void) | null>(null)
  const localPromiseRef = useRef<Promise<boolean> | null>(null)

  const resolveLocal = useCallback((confirmed: boolean) => {
    setLocalOpen(false)
    const fn = localResolveRef.current
    localResolveRef.current = null
    localPromiseRef.current = null
    fn?.(confirmed)
  }, [])

  const requestConfirm = useCallback((): Promise<boolean> => {
    if (ctx) {
      return ctx.requestConfirm()
    }
    if (localPromiseRef.current) {
      return localPromiseRef.current
    }
    const promise = new Promise<boolean>((resolve) => {
      localResolveRef.current = resolve
    })
    localPromiseRef.current = promise
    setLocalOpen(true)
    return promise
  }, [ctx])

  // Register form with global context when active
  useEffect(() => {
    if (!ctx || !active) return
    return ctx.registerForm(formId, {
      isDirty: () => activeRef.current && !exitedAfterSaveRef.current && dirtyRef.current,
      isSaving: () => activeRef.current && !exitedAfterSaveRef.current && savingRef.current,
    })
  }, [ctx, formId, active])

  // Notify global provider when dirty/saving changes (syncs popstate sentinel)
  useEffect(() => {
    if (ctx && active) {
      ctx.notifyStateChanged()
    }
  }, [ctx, active, isDirty, isSaving])

  // Standalone beforeunload fallback when no provider wraps the component
  useEffect(() => {
    if (ctx || !active) return
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (!exitedAfterSaveRef.current && (dirtyRef.current || savingRef.current)) {
        e.preventDefault()
        e.returnValue = ''
      }
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [ctx, active])

  /**
   * Guards any exit/cancel/tab-switch action:
   * - If saving: blocks silently.
   * - If clean: executes [action] immediately without dialog.
   * - If dirty: shows the single confirmation dialog and executes [action] only on confirm.
   */
  const confirmAction = useCallback(
    async (action: () => void): Promise<boolean> => {
      if (savingRef.current) return false
      if (!dirtyRef.current || exitedAfterSaveRef.current) {
        action()
        return true
      }
      const confirmed = await requestConfirm()
      if (!confirmed) return false
      action()
      return true
    },
    [requestConfirm],
  )

  /**
   * Official exit path after a form saves successfully.
   * Clears guard state immediately and executes [action] without showing any confirmation dialog.
   */
  const exitAfterSave = useCallback(
    (action: () => void) => {
      exitedAfterSaveRef.current = true
      ctx?.markApprovedNavigation()
      action()
    },
    [ctx],
  )

  const confirmModal = ctx
    ? null
    : createElement(CancelConfirmModal, {
        open: localOpen,
        onStay: () => resolveLocal(false),
        onConfirmCancel: () => resolveLocal(true),
      })

  return {
    confirmAction,
    guardedClose: confirmAction,
    exitAfterSave,
    confirmModal,
  }
}

/**
 * Convenience helper for Ant Design `<Modal>` components so Escape, backdrop
 * click, X close button, and Batal button share the exact same unsaved-changes guard.
 */
export function useAntdModalGuard({
  open,
  isDirty,
  isSaving = false,
  onClose,
}: {
  open: boolean
  isDirty: boolean
  isSaving?: boolean
  onClose: () => void
}) {
  const { guardedClose, exitAfterSave, confirmModal } = useUnsavedChangesGuard({
    active: open,
    isDirty,
    isSaving,
  })

  const handleCancel = useCallback(() => {
    void guardedClose(onClose)
  }, [guardedClose, onClose])

  return {
    onCancel: handleCancel,
    keyboard: !isSaving,
    maskClosable: !isSaving,
    closable: !isSaving,
    exitAfterSave,
    confirmModal,
  }
}
