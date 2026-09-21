import '@testing-library/jest-dom/vitest'

/**
 * Prevent AbortSignal.timeout from firing during tests.
 * jsdom doesn't natively support AbortSignal.timeout, so we polyfill it
 * with a long-lived signal that never aborts.
 *
 * We use Object.defineProperty (NOT vi.spyOn) so that tests calling
 * vi.restoreAllMocks() never restore the polyfill back to the real
 * implementation.
 */
const neverController = new AbortController()

if (typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
  Object.defineProperty(AbortSignal, 'timeout', {
    value: () => neverController.signal as AbortSignal,
    writable: true,
    configurable: true,
    enumerable: false,
  })
}
