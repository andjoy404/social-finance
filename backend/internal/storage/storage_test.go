package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
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
