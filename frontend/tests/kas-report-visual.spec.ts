import { test, expect } from '@playwright/test'

test.describe('Kas Report Visual Verification', () => {
  // Start the Vite dev server
  test.beforeAll(async ({}, testInfo) => {
    testInfo.setTimeout(60000)
  })
})
