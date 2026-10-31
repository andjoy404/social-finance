/**
 * Shared clipboard utilities with consistent user feedback.
 * Used by Iuran, KAS, and Dashboard components.
 */

/** Result of a clipboard copy attempt. */
export interface ClipboardResult {
  success: boolean
  message: string
}

/**
 * Copy text to clipboard with error handling.
 * Returns a result object so the caller can show appropriate feedback.
 */
export async function copyToClipboard(text: string): Promise<ClipboardResult> {
  try {
    await navigator.clipboard.writeText(text)
    return { success: true, message: 'Berhasil disalin.' }
  } catch (e: unknown) {
    const message =
      e instanceof Error
        ? `Gagal menyalin: ${e.message}`
        : 'Gagal menyalin ke clipboard.'
    return { success: false, message }
  }
}

/**
 * Show a simple inline feedback toast near the copy button.
 * Uses a fixed-position toast that fades in/out.
 * Returns a cleanup function that can be called if the component unmounts.
 */
let activeToast: HTMLElement | null = null

export function showCopyFeedback(result: ClipboardResult, showSuccess: boolean): { remove: () => void } {
  // Remove any existing toast
  removeToast()

  const toast = document.createElement('div')
  toast.setAttribute('role', 'alert')
  toast.style.cssText = `
    position: fixed;
    top: 20px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 10000;
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    color: ${showSuccess ? '#ffffff' : '#ffffff'};
    background: ${showSuccess ? '#22c55e' : '#ef4444'};
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
    opacity: 0;
    transition: opacity 0.2s ease;
    pointer-events: none;
    max-width: 90vw;
    text-align: center;
  `
  toast.textContent = result.message
  document.body.appendChild(toast)
  activeToast = toast

  // Fade in
  requestAnimationFrame(() => {
    toast.style.opacity = '1'
  })

  // Auto-remove after 2.5s
  const timer = setTimeout(() => {
    toast.style.opacity = '0'
    setTimeout(() => {
      if (toast.parentNode) toast.parentNode.removeChild(toast)
    }, 200)
  }, 2500)

  return {
    remove: () => {
      clearTimeout(timer)
      removeToast()
    },
  }
}

function removeToast(): void {
  if (activeToast && activeToast.parentNode) {
    activeToast.parentNode.removeChild(activeToast)
    activeToast = null
  }
}
