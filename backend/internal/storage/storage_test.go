package storage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"

	stdwebp "golang.org/x/image/webp"
)

var (
	// Minimal 1x1 PNG bytes
	samplePNG = []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}

	// Minimal JPEG SOI/EOI
	sampleJPEG = []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x01, 0x00, 0x60, 0x00, 0x60, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x03, 0x02, 0x02, 0x03, 0x02, 0x02, 0x03, 0x03, 0x03, 0x03, 0x04,
		0xFF, 0xD9,
	}

	// Text file pretending to be image
	sampleText = []byte("Hello, this is just a plain text file, not an image!")
)

func TestMockStorage(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()

	t.Run("upload valid PNG succeeds", func(t *testing.T) {
		meta, err := store.UploadProof(ctx, "rt-123", "pay-456", "my_receipt.png", bytes.NewReader(samplePNG), int64(len(samplePNG)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.ContentType != "image/png" {
			t.Errorf("expected image/png, got %s", meta.ContentType)
		}
		if meta.Size != int64(len(samplePNG)) {
			t.Errorf("expected size %d, got %d", len(samplePNG), meta.Size)
		}

		// Retrieve
		reader, err := store.GetProof(ctx, meta.Key)
		if err != nil {
			t.Fatalf("failed to get proof: %v", err)
		}
		defer reader.Body.Close()

		data, err := io.ReadAll(reader.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if !bytes.Equal(data, samplePNG) {
			t.Errorf("retrieved bytes do not match uploaded bytes")
		}
	})

	t.Run("upload valid JPEG succeeds", func(t *testing.T) {
		meta, err := store.UploadProof(ctx, "rt-123", "pay-789", "receipt.jpg", bytes.NewReader(sampleJPEG), int64(len(sampleJPEG)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.ContentType != "image/jpeg" {
			t.Errorf("expected image/jpeg, got %s", meta.ContentType)
		}
	})

	t.Run("reject empty file", func(t *testing.T) {
		_, err := store.UploadProof(ctx, "rt-123", "pay-000", "empty.png", bytes.NewReader([]byte{}), 0)
		if !errors.Is(err, ErrEmptyFile) {
			t.Errorf("expected ErrEmptyFile, got %v", err)
		}
	})

	t.Run("reject invalid format", func(t *testing.T) {
		_, err := store.UploadProof(ctx, "rt-123", "pay-111", "evil.txt", bytes.NewReader(sampleText), int64(len(sampleText)))
		if !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("expected ErrInvalidFormat, got %v", err)
		}
	})

	t.Run("reject file exceeding size limit", func(t *testing.T) {
		smallStore := NewMockStorage()
		smallStore.MaxFileSize = 10 // 10 bytes max
		_, err := smallStore.UploadProof(ctx, "rt-123", "pay-222", "photo.png", bytes.NewReader(samplePNG), int64(len(samplePNG)))
		if !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("expected ErrFileTooLarge, got %v", err)
		}
	})

	t.Run("reject path traversal in GetProof", func(t *testing.T) {
		_, err := store.GetProof(ctx, "proofs/../../etc/passwd")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound for traversal, got %v", err)
		}

		_, err = store.GetProof(ctx, "arbitrary-key")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound for non-proofs prefix, got %v", err)
		}
	})

	t.Run("delete proof", func(t *testing.T) {
		meta, err := store.UploadProof(ctx, "rt-123", "pay-333", "del.png", bytes.NewReader(samplePNG), int64(len(samplePNG)))
		if err != nil {
			t.Fatalf("upload failed: %v", err)
		}

		err = store.DeleteProof(ctx, meta.Key)
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		_, err = store.GetProof(ctx, meta.Key)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound after deletion, got %v", err)
		}
	})
}

// --- 5 MiB boundary enforcement (shared by all upload endpoints) ---

func TestMockStorageUploadExactFiveMiBAccepted(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage() // MaxFileSize defaults to DefaultMaxFileSize (5 MiB)
	exact5MiB := append(append([]byte{}, samplePNG...), make([]byte, 5*1024*1024-len(samplePNG))...)
	if len(exact5MiB) != DefaultMaxFileSize {
		t.Fatalf("fixture size = %d, want %d", len(exact5MiB), DefaultMaxFileSize)
	}
	meta, err := store.UploadProof(ctx, "rt-1", "pay-5mib", "big.png", bytes.NewReader(exact5MiB), int64(len(exact5MiB)))
	if err != nil {
		t.Fatalf("exactly 5 MiB image must be accepted, got %v", err)
	}
	if meta.Size != DefaultMaxFileSize {
		t.Errorf("meta.Size = %d, want %d", meta.Size, DefaultMaxFileSize)
	}
	if meta.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want image/png", meta.ContentType)
	}
}

func TestMockStorageUploadFiveMiBPlusOneRejected(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	over5MiB := append(append([]byte{}, samplePNG...), make([]byte, 5*1024*1024-len(samplePNG)+1)...)
	if len(over5MiB) != DefaultMaxFileSize+1 {
		t.Fatalf("fixture size = %d, want %d", len(over5MiB), DefaultMaxFileSize+1)
	}
	_, err := store.UploadProof(ctx, "rt-1", "pay-5mib1", "bigger.png", bytes.NewReader(over5MiB), int64(len(over5MiB)))
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("5 MiB + 1 byte must be rejected with ErrFileTooLarge, got %v", err)
	}
}

func TestMockStorageUploadGifRejected(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	gif := []byte("GIF89a\x02\x00\x02\x00\x00\x00\x00;")
	_, err := store.UploadProof(ctx, "rt-1", "pay-gif", "image.gif", bytes.NewReader(gif), int64(len(gif)))
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("GIF must be rejected (not in allowed formats), got %v", err)
	}
}

func TestMockStorageUploadCommunityImageExactFiveMiBAccepted(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	exact5MiB := append(append([]byte{}, samplePNG...), make([]byte, 5*1024*1024-len(samplePNG))...)
	meta, err := store.UploadCommunityImage(ctx, "rt-1", "announcement", "big.png", bytes.NewReader(exact5MiB), int64(len(exact5MiB)))
	if err != nil {
		t.Fatalf("exactly 5 MiB community image must be accepted, got %v", err)
	}
	if meta.Size != DefaultMaxFileSize {
		t.Errorf("meta.Size = %d, want %d", meta.Size, DefaultMaxFileSize)
	}
}

func TestMockStorageUploadCommunityImageFiveMiBPlusOneRejected(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	over5MiB := append(append([]byte{}, samplePNG...), make([]byte, 5*1024*1024-len(samplePNG)+1)...)
	_, err := store.UploadCommunityImage(ctx, "rt-1", "announcement", "bigger.png", bytes.NewReader(over5MiB), int64(len(over5MiB)))
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("5 MiB + 1 byte community image must be rejected, got %v", err)
	}
}
func makeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func makeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 20, B: 10, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func mustReadWebPSample(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/tiny.webp")
	if err != nil {
		t.Fatalf("deterministic webp fixture missing/unreadable: %v", err)
	}
	return b
}

func decodeConfig(t *testing.T, b []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return cfg.Width, cfg.Height
}

func decodeImage(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode image: %v", err)
	}
	return img
}

func TestUploadCommunityImageWithThumb_JPEG(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	jpg := makeTestJPEG(t, 64, 32)
	res, err := store.UploadCommunityImageWithThumb(ctx, "rt-1", "profile", "photo.jpg", bytes.NewReader(jpg), int64(len(jpg)), true)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if res.OriginalKey == "" || res.ThumbKey == "" {
		t.Fatalf("expected keys, got orig=%q thumb=%q", res.OriginalKey, res.ThumbKey)
	}
	if !strings.HasSuffix(res.OriginalKey, ".jpg") {
		t.Errorf("original key ext: %s", res.OriginalKey)
	}
	if !strings.HasSuffix(res.ThumbKey, "_thumb.jpg") {
		t.Errorf("thumb key: %s", res.ThumbKey)
	}
	orig, err := store.GetCommunityImage(ctx, res.OriginalKey)
	if err != nil {
		t.Fatalf("get original: %v", err)
	}
	defer orig.Body.Close()
	origBytes, _ := io.ReadAll(orig.Body)
	if !bytes.Equal(origBytes, jpg) {
		t.Errorf("original bytes modified")
	}
	if orig.ContentType != "image/jpeg" {
		t.Errorf("orig content-type: %s", orig.ContentType)
	}
	thumb, err := store.GetCommunityImage(ctx, res.ThumbKey)
	if err != nil {
		t.Fatalf("get thumb: %v", err)
	}
	defer thumb.Body.Close()
	thumbBytes, _ := io.ReadAll(thumb.Body)
	if thumb.ContentType != "image/jpeg" {
		t.Errorf("thumb content-type: %s", thumb.ContentType)
	}
	tw, th := decodeConfig(t, thumbBytes)
	if tw != 200 || th != 200 {
		t.Errorf("thumb dims = %dx%d, want 200x200", tw, th)
	}
}

func TestMakeThumbnailJPEG_DimensionsAndBehavior(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{
		{"small_square", 50, 50},
		{"small_portrait", 40, 80},
		{"small_landscape", 120, 60},
		{"large_square", 400, 400},
		{"large_portrait", 300, 600},
		{"large_landscape", 800, 400},
		{"tiny", 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, tc.w, tc.h))
			for y := 0; y < tc.h; y++ {
				for x := 0; x < tc.w; x++ {
					img.SetNRGBA(x, y, color.NRGBA{R: 30, G: 60, B: 200, A: 255})
				}
			}
			out, err := makeThumbnailJPEG(img)
			if err != nil {
				t.Fatalf("makeThumbnailJPEG: %v", err)
			}
			tw, th := decodeConfig(t, out)
			if tw != 200 || th != 200 {
				t.Fatalf("thumb dims = %dx%d, want 200x200", tw, th)
			}
		})
	}

	// Alpha must be composited to white (JPEG has no alpha).
	t.Run("alpha_composited_white", func(t *testing.T) {
		img := image.NewNRGBA(image.Rect(0, 0, 300, 300))
		for y := 0; y < 300; y++ {
			for x := 0; x < 300; x++ {
				img.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
			}
		}
		out, err := makeThumbnailJPEG(img)
		if err != nil {
			t.Fatalf("makeThumbnailJPEG: %v", err)
		}
		decoded := decodeImage(t, out)
		r, g, b, _ := decoded.At(100, 100).RGBA()
		if r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
			t.Fatalf("expected white background after alpha composite, got rgb=(%d,%d,%d)", r>>8, g>>8, b>>8)
		}
	})

	// Small image: not upscaled to blur/fill content; centered on white canvas.
	t.Run("small_not_upscaled_centered", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 50, 50))
		for y := 0; y < 50; y++ {
			for x := 0; x < 50; x++ {
				img.Set(x, y, color.RGBA{R: 10, G: 10, B: 200, A: 255})
			}
		}
		out, err := makeThumbnailJPEG(img)
		if err != nil {
			t.Fatalf("makeThumbnailJPEG: %v", err)
		}
		decoded := decodeImage(t, out)
		// Corner should be canvas (white), center should be image content (blue-ish).
		cr, cg, cb, _ := decoded.At(5, 5).RGBA()
		if cr>>8 < 240 || cg>>8 < 240 || cb>>8 < 240 {
			t.Fatalf("expected white canvas at corner for small image, got rgb=(%d,%d,%d)", cr>>8, cg>>8, cb>>8)
		}
		mr, _, mb, _ := decoded.At(100, 100).RGBA()
		if mb>>8 < 150 || mr>>8 > 120 {
			t.Fatalf("expected centered image content, got rgb=(%d,_,%d)", mr>>8, mb>>8)
		}
	})
}

func TestUploadCommunityImageWithThumb_PNG(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	pngData := makeTestPNG(t, 40, 60)
	res, err := store.UploadCommunityImageWithThumb(ctx, "rt-1", "profile", "photo.png", bytes.NewReader(pngData), int64(len(pngData)), true)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if !strings.HasSuffix(res.OriginalKey, ".png") {
		t.Errorf("original key ext: %s", res.OriginalKey)
	}
	if !strings.HasSuffix(res.ThumbKey, "_thumb.jpg") {
		t.Errorf("thumb key: %s", res.ThumbKey)
	}
	orig, _ := store.GetCommunityImage(ctx, res.OriginalKey)
	defer orig.Body.Close()
	origBytes, _ := io.ReadAll(orig.Body)
	if !bytes.Equal(origBytes, pngData) {
		t.Errorf("original png bytes modified")
	}
	if orig.ContentType != "image/png" {
		t.Errorf("orig content-type: %s", orig.ContentType)
	}
	thumb, _ := store.GetCommunityImage(ctx, res.ThumbKey)
	defer thumb.Body.Close()
	thumbBytes, _ := io.ReadAll(thumb.Body)
	if thumb.ContentType != "image/jpeg" {
		t.Errorf("thumb content-type: %s", thumb.ContentType)
	}
	tw, th := decodeConfig(t, thumbBytes)
	if tw != 200 || th != 200 {
		t.Errorf("thumb dims = %dx%d, want 200x200", tw, th)
	}
}

func TestUploadCommunityImageWithThumb_WebP(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	webpData := mustReadWebPSample(t)
	// verify decodable by registered decoder and process for real (no skip).
	if _, err := stdwebp.DecodeConfig(bytes.NewReader(webpData)); err != nil {
		t.Fatalf("webp fixture not decodable by registered decoder: %v", err)
	}
	if _, err := decodeAndValidateImage(webpData); err != nil {
		t.Fatalf("webp fixture must be processable: %v", err)
	}
	res, err := store.UploadCommunityImageWithThumb(ctx, "rt-1", "profile", "photo.webp", bytes.NewReader(webpData), int64(len(webpData)), true)
	if err != nil {
		t.Fatalf("upload webp: %v", err)
	}
	if !strings.HasSuffix(res.OriginalKey, ".webp") {
		t.Errorf("original key ext: %s", res.OriginalKey)
	}
	if !strings.HasSuffix(res.ThumbKey, "_thumb.jpg") {
		t.Errorf("thumb key: %s", res.ThumbKey)
	}
	orig, _ := store.GetCommunityImage(ctx, res.OriginalKey)
	defer orig.Body.Close()
	origBytes, _ := io.ReadAll(orig.Body)
	if !bytes.Equal(origBytes, webpData) {
		t.Errorf("original webp bytes modified")
	}
	if orig.ContentType != "image/webp" {
		t.Errorf("orig content-type: %s", orig.ContentType)
	}
	thumb, _ := store.GetCommunityImage(ctx, res.ThumbKey)
	defer thumb.Body.Close()
	thumbBytes, _ := io.ReadAll(thumb.Body)
	if thumb.ContentType != "image/jpeg" {
		t.Errorf("thumb content-type: %s", thumb.ContentType)
	}
	tw, th := decodeConfig(t, thumbBytes)
	if tw != 200 || th != 200 {
		t.Errorf("thumb dims = %dx%d, want 200x200", tw, th)
	}
}

func makeOversizePixelPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// leave default; encode produces a valid PNG whose header declares w,h
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestUploadCommunityImageWithThumb_RejectsOversizeDimensions(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	// Width exceeds MaxImageDimension (pre-decode config check)
	bigW := makeTestPNG(t, MaxImageDimension+1, 10)
	_, err := store.UploadCommunityImageWithThumb(ctx, "rt-1", "profile", "big.png", bytes.NewReader(bigW), int64(len(bigW)), true)
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected ErrImageTooLarge for width, got %v", err)
	}
	// Height exceeds
	bigH := makeTestPNG(t, 10, MaxImageDimension+1)
	_, err = store.UploadCommunityImageWithThumb(ctx, "rt-1", "profile", "big.png", bytes.NewReader(bigH), int64(len(bigH)), true)
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected ErrImageTooLarge for height, got %v", err)
	}
	// Pixel count exceeds while both dimensions are within limit (4000*3500 = 14M > 12M)
	bigPx := makeOversizePixelPNG(t, 4000, 3500)
	_, err = store.UploadCommunityImageWithThumb(ctx, "rt-1", "profile", "big.png", bytes.NewReader(bigPx), int64(len(bigPx)), true)
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected ErrImageTooLarge for pixels, got %v", err)
	}
}

func TestUploadCommunityImageWithThumb_RejectsTruncatedBody(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	jpg := makeTestJPEG(t, 20, 20)
	trunc := jpg[:len(jpg)/2]
	_, err := store.UploadCommunityImageWithThumb(ctx, "rt-1", "profile", "photo.jpg", bytes.NewReader(trunc), int64(len(jpg)), true)
	if err == nil {
		t.Fatalf("expected error for truncated body")
	}
}

func TestDecodeAndValidateImage_RegisteredDecoders(t *testing.T) {
	// Ensures JPEG/PNG/WebP decoders are registered in this binary.
	jpg := makeTestJPEG(t, 8, 8)
	if _, err := decodeAndValidateImage(jpg); err != nil {
		t.Fatalf("jpeg decode: %v", err)
	}
	pngData := makeTestPNG(t, 8, 8)
	if _, err := decodeAndValidateImage(pngData); err != nil {
		t.Fatalf("png decode: %v", err)
	}
	webpData := mustReadWebPSample(t)
	if _, err := decodeAndValidateImage(webpData); err != nil {
		t.Fatalf("webp decode: %v", err)
	}
}
