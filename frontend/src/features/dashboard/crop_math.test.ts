import { describe, it, expect } from 'vitest'

interface CropRect {
  x: number
  y: number
  width: number
  height: number
}

// Computes CSS percentages used in Web Dashboard.tsx
function computeWebCropCSS(crop: CropRect) {
  const cropW = Math.max(0.01, Math.min(1.0, crop.width))
  const cropH = Math.max(0.01, Math.min(1.0, crop.height))
  const cropX = Math.max(0.0, Math.min(1.0 - cropW, crop.x))
  const cropY = Math.max(0.0, Math.min(1.0 - cropH, crop.y))

  return {
    widthPct: (1 / cropW) * 100,
    heightPct: (1 / cropH) * 100,
    leftPct: -(cropX / cropW) * 100,
    topPct: -(cropY / cropH) * 100,
  }
}

// Simulates Flutter BannerCropImage pixel calculations
function computeFlutterPixels(cw: number, ch: number, crop: CropRect) {
  const cropW = Math.max(0.01, Math.min(1.0, crop.width))
  const cropH = Math.max(0.01, Math.min(1.0, crop.height))
  const cropX = Math.max(0.0, Math.min(1.0 - cropW, crop.x))
  const cropY = Math.max(0.0, Math.min(1.0 - cropH, crop.y))

  const imgW = cw / cropW
  const imgH = ch / cropH
  const left = -cropX * imgW
  const top = -cropY * imgH

  return { left, top, imgW, imgH }
}

describe('Web CSS & Flutter Layout Math Equivalency', () => {
  it('converts normalized crop coordinates to identical pixel mapping', () => {
    const resolutions = [
      { w: 1920, h: 1080 },
      { w: 1280, h: 720 },
      { w: 800, h: 450 },
      { w: 360, h: 202.5 },
    ]

    const testCrops: CropRect[] = [
      { x: 0.0, y: 0.0, width: 1.0, height: 1.0 },
      { x: 0.1, y: 0.2, width: 0.5, height: 0.5 },
      { x: 0.25, y: 0.15, width: 0.6, height: 0.6 },
      { x: 0.33, y: 0.44, width: 0.33, height: 0.33 },
    ]

    for (const res of resolutions) {
      for (const crop of testCrops) {
        const webCSS = computeWebCropCSS(crop)
        const flutter = computeFlutterPixels(res.w, res.h, crop)

        // Web pixel value = (percentage / 100) * containerDimension
        const webLeftPx = (webCSS.leftPct / 100) * res.w
        const webTopPx = (webCSS.topPct / 100) * res.h
        const webWidthPx = (webCSS.widthPct / 100) * res.w
        const webHeightPx = (webCSS.heightPct / 100) * res.h

        expect(flutter.left).toBeCloseTo(webLeftPx, 4)
        expect(flutter.top).toBeCloseTo(webTopPx, 4)
        expect(flutter.imgW).toBeCloseTo(webWidthPx, 4)
        expect(flutter.imgH).toBeCloseTo(webHeightPx, 4)
      }
    }
  })
})
