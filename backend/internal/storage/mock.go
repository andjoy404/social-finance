package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type MockStorage struct {
	mu          sync.RWMutex
	objects     map[string][]byte
	metadata    map[string]*ObjectMetadata
	MaxFileSize int64
	InitErr     error
	UploadErr   error
	GetErr      error
	DeleteErr   error
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		objects:     make(map[string][]byte),
		metadata:    make(map[string]*ObjectMetadata),
		MaxFileSize: DefaultMaxFileSize,
	}
}

func (m *MockStorage) InitBucket(ctx context.Context) error {
	if m.InitErr != nil {
		return m.InitErr
	}
	return nil
}

func (m *MockStorage) UploadProof(ctx context.Context, rtID, paymentID, filename string, r io.Reader, size int64) (*ObjectMetadata, error) {
	if m.UploadErr != nil {
		return nil, m.UploadErr
	}
	if size <= 0 {
		return nil, ErrEmptyFile
	}
	limit := m.MaxFileSize
	if limit <= 0 {
		limit = DefaultMaxFileSize
	}
	if size > limit {
		return nil, ErrFileTooLarge
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) == 0 {
		return nil, ErrEmptyFile
	}

	contentType := http.DetectContentType(data)
	if contentType == "application/octet-stream" && len(data) >= 12 {
		if string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			contentType = "image/webp"
		}
	}

	ext, ok := allowedMimeTypes[contentType]
	if !ok {
		return nil, ErrInvalidFormat
	}

	sanitizedName := filepath.Base(filename)
	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	now := time.Now()
	key := fmt.Sprintf("proofs/%s/%s_%d_%s%s", rtID, paymentID, now.UnixNano(), randomHex, ext)

	meta := &ObjectMetadata{
		Key:          key,
		OriginalName: sanitizedName,
		ContentType:  contentType,
		Size:         int64(len(data)),
		UploadedAt:   now,
	}

	m.mu.Lock()
	m.objects[key] = data
	m.metadata[key] = meta
	m.mu.Unlock()

	return meta, nil
}

func (m *MockStorage) GetProof(ctx context.Context, key string) (*ObjectReader, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "proofs/") {
		return nil, ErrNotFound
	}

	m.mu.RLock()
	data, ok := m.objects[key]
	meta := m.metadata[key]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrNotFound
	}

	ct := "application/octet-stream"
	if meta != nil && meta.ContentType != "" {
		ct = meta.ContentType
	}

	return &ObjectReader{
		Body:        io.NopCloser(bytes.NewReader(data)),
		ContentType: ct,
		Size:        int64(len(data)),
	}, nil
}

func (m *MockStorage) DeleteProof(ctx context.Context, key string) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "proofs/") {
		return ErrNotFound
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objects[key]; !ok {
		return ErrNotFound
	}
	delete(m.objects, key)
	delete(m.metadata, key)
	return nil
}
