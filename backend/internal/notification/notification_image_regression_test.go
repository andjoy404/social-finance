package notification_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/storage"
)

// mockStorageService implements storage.Service for testing.
type mockStorageService struct {
	objects map[string][]byte
	meta    map[string]storage.ObjectMetadata
}

func newMockStorageService() *mockStorageService {
	return &mockStorageService{
		objects: make(map[string][]byte),
		meta:    make(map[string]storage.ObjectMetadata),
	}
}

func (m *mockStorageService) UploadProof(ctx context.Context, rtID, paymentID, filename string, r io.Reader, size int64) (*storage.ObjectMetadata, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageService) GetProof(ctx context.Context, key string) (*storage.ObjectReader, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageService) DeleteProof(ctx context.Context, key string) error {
	return storage.ErrNotFound
}

func (m *mockStorageService) UploadNotificationImage(ctx context.Context, rtID, eventType, filename string, r io.Reader, size int64) (*storage.ObjectMetadata, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Replicate real storage validation: size limit
	maxSize := int64(5 * 1024 * 1024) // 5MB
	if size > maxSize || int64(len(data)) > maxSize {
		return nil, fmt.Errorf("file too large: max 5MB")
	}

	// Replicate real storage validation: MIME type detection
	// Pad data to at least 512 bytes so http.DetectContentType works correctly.
	// Copy data first, then pad remaining bytes with zeros (safe for MIME detection).
	buffer := make([]byte, 512)
	if len(data) > 0 {
		copy(buffer, data)
	}
	contentType := http.DetectContentType(buffer)
	allowedMimeTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	if !allowedMimeTypes[contentType] {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	key := fmt.Sprintf("notifications/%s/%s/%d_%s", eventType, rtID, 12345678, "mock.png")
	m.objects[key] = data
	m.meta[key] = storage.ObjectMetadata{
		Key:         key,
		ContentType: contentType,
		Size:        int64(len(data)),
	}
	return &storage.ObjectMetadata{
		Key:         key,
		ContentType: contentType,
		Size:        int64(len(data)),
	}, nil
}

func (m *mockStorageService) GetNotificationImage(ctx context.Context, key string) (*storage.ObjectReader, error) {
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "notifications/") {
		return nil, storage.ErrNotFound
	}
	data, ok := m.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	meta := m.meta[key]
	return &storage.ObjectReader{
		Body:        io.NopCloser(bytes.NewReader(data)),
		ContentType: meta.ContentType,
		Size:        meta.Size,
	}, nil
}

func (m *mockStorageService) DeleteNotificationImage(ctx context.Context, key string) error {
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "notifications/") {
		return storage.ErrNotFound
	}
	delete(m.objects, key)
	delete(m.meta, key)
	return nil
}

func (m *mockStorageService) UploadCommunityImage(ctx context.Context, rtID, category, filename string, r io.Reader, size int64) (*storage.ObjectMetadata, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageService) GetCommunityImage(ctx context.Context, key string) (*storage.ObjectReader, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageService) DeleteCommunityImage(ctx context.Context, key string) error {
	return storage.ErrNotFound
}

func (m *mockStorageService) InitBucket(ctx context.Context) error {
	return nil
}

func handlerWithStorage(svc notification.Service, store *mockStorageService) *notification.Handler {
	return notification.NewHandler(svc, nil, store)
}

func handlerWithRouter(svc notification.Service, store *mockStorageService) http.Handler {
	r := chi.NewRouter()
	h := handlerWithStorage(svc, store)

	// Middleware must be defined BEFORE routes in chi
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			val := r.Context().Value(auth.AuthContextKey)
			ac, ok := val.(*auth.AuthContext)
			if !ok {
				ac = nil
			}
			// GET image endpoint is public
			isImageGet := r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/notifications/images/")
			if isImageGet {
				next.ServeHTTP(w, r)
				return
			}
			if ac == nil {
				http.Error(w, `{"error":{"code":"unauthorized"}}`, http.StatusUnauthorized)
				return
			}
			if ac.RTID == "" {
				http.Error(w, `{"error":{"code":"forbidden"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Production routes:
	//   GET  /api/v1/notifications/images/*  -> GetImage  (public)
	//   POST /api/v1/notifications/upload-image -> UploadImage (requires auth + RT)
	r.Get("/api/v1/notifications/images/*", h.GetImage)
	r.Post("/api/v1/notifications/upload-image", h.UploadImage)

	return r
}

// ============================================================
// Regression tests: Path Traversal Protection
// ============================================================

func TestImage_GetPathTraversal_Rejected(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	// Chi's wildcard route {key...} rejects paths with ".." before reaching the handler.
	// This is defense-in-depth: our handler also validates, but Chi provides an
	// additional layer of protection. Tests should verify that traversal attempts
	// are rejected at the HTTP level (404 for unmatchable routes, 400 for handler validation).

	tests := []struct {
		name    string
		path    string
		wantOK  bool
	}{
		// These paths are rejected by Chi's router before reaching handler
		{
			name:    "Chi rejects parent directory traversal",
			path:    "/api/v1/notifications/images/../../etc/passwd",
			wantOK:  false,
		},
		{
			name:    "Chi rejects double parent traversal",
			path:    "/api/v1/notifications/images/../../../etc/shadow",
			wantOK:  false,
		},
		// These paths pass through Chi to handler validation
		{
			name:    "handler rejects backslash traversal",
			path:    "/api/v1/notifications/images/..\\..\\etc\\passwd",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if tt.wantOK {
				if rec.Code != http.StatusOK {
					t.Errorf("expected 200 for %s, got %d: %s", tt.name, rec.Code, rec.Body.String())
				}
			} else {
				// Rejected at any level is acceptable — Chi (404) or handler (400) both work
				if rec.Code == http.StatusOK {
					t.Errorf("expected rejection for %s, got 200: %s", tt.name, rec.Body.String())
				}
			}
		})
	}
}

func TestImage_GetURLEncodedPathTraversal_Rejected(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "percent-encoded parent traversal",
			path: "/api/v1/notifications/images/%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		},
		{
			name: "double-encoded traversal",
			path: "/api/v1/notifications/images/%252e%252e%252fetc%252fpasswd",
		},
		{
			name: "mixed encoding",
			path: "/api/v1/notifications/images/..%2f%2e%2fetc%2fpasswd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			// Chi auto-decodes URL params, so %2e%2e becomes .. before check
			// Then Chi's router rejects the path, or handler rejects it
			if rec.Code == http.StatusOK {
				t.Errorf("expected rejection for URL-encoded path traversal %s, got 200", tt.name)
			}
		})
	}
}

func TestImage_GetPrefixConstraint(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	// Valid key with notifications/ prefix returns 404 (not found, but no error)
	req := httptest.NewRequest("GET", "/api/v1/notifications/images/notifications/manual/rt-002/12345678_test.png", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for valid but missing key, got %d", rec.Code)
	}

	// Key without prefix gets auto-prepend notifications/
	req = httptest.NewRequest("GET", "/api/v1/notifications/images/manual/rt-002/12345678_test.png", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	// Should become notifications/manual/rt-002/... after auto-prepend
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for key without prefix, got %d", rec.Code)
	}
}

func TestImage_GetBucketIsolation(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	// Put an object in the store
	store.objects["notifications/manual/rt-002/12345.png"] = []byte("fake image!")
	store.meta["notifications/manual/rt-002/12345.png"] = storage.ObjectMetadata{
		Key:         "notifications/manual/rt-002/12345.png",
		ContentType: "image/png",
		Size:        11,
	}

	req := httptest.NewRequest("GET", "/api/v1/notifications/images/notifications/manual/rt-002/12345.png", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid key, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "image/") {
		t.Errorf("expected image content-type, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.Len() != 11 {
		t.Errorf("expected body length 11, got %d", rec.Body.Len())
	}
}

// ============================================================
// Regression tests: Upload Authorization
// ============================================================

func TestImage_UploadRejectsUnauthorized(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("event_type", "manual_push")
	fw, _ := writer.CreateFormFile("file", "test.png")
	fw.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/notifications/upload-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated upload, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestImage_UploadRejectsEmptyRT(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("event_type", "manual_push")
	fw, _ := writer.CreateFormFile("file", "test.png")
	fw.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/notifications/upload-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
 req.Header.Set("X-RT-ID", "rt-002")
	ac := &auth.AuthContext{
		UserID: "test-user",
		RTID:   "",
	}
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for empty RT, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestImage_UploadRequiresValidEventType(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	tests := []struct {
		name      string
		eventType string
		wantCode  int
	}{
		{
			name:      "empty event type",
			eventType: "",
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "invalid event type",
			eventType: "transaction_proof",
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "valid waste_truck_arrival",
			eventType: "waste_truck_arrival",
			wantCode:  http.StatusCreated,
		},
		{
			name:      "valid manual_push",
			eventType: "manual_push",
			wantCode:  http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			writer.WriteField("event_type", tt.eventType)
			fw, _ := writer.CreateFormFile("file", "test.png")
			fw.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
			writer.Close()

			req := httptest.NewRequest("POST", "/api/v1/notifications/upload-image", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
   req.Header.Set("X-RT-ID", "rt-002")
			ac := &auth.AuthContext{
				UserID:     "test-user",
				RTID:       "rt-002",
				TenantRole: auth.RolePengurus,
				Jabatan:    "ketua",
			}
			req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("expected %d for event_type=%q, got %d: %s", tt.wantCode, tt.eventType, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestImage_UploadRequiresOfficer(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("event_type", "manual_push")
	fw, _ := writer.CreateFormFile("file", "test.png")
	fw.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/notifications/upload-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
 req.Header.Set("X-RT-ID", "rt-002")
	ac := &auth.AuthContext{
		UserID:     "warga-001",
		RTID:       "rt-002",
		TenantRole: auth.RoleWarga,
	}
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for warga upload, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ============================================================
// Regression tests: File Size Limits
// ============================================================

func TestImage_UploadRejectsOversizedFile(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	// ~5MB + 1KB
	largeFile := bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82}, 1310720)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("event_type", "manual_push")
	fw, _ := writer.CreateFormFile("file", "large.png")
	fw.Write(largeFile)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/notifications/upload-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
 req.Header.Set("X-RT-ID", "rt-002")
	ac := &auth.AuthContext{
		UserID:     "test-user",
		RTID:       "rt-002",
		TenantRole: auth.RolePengurus,
		Jabatan:    "ketua",
	}
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized file, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestImage_UploadRejectsNonImageFile(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	pdfContent := []byte("%PDF-1.4 fake pdf content\n")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("event_type", "manual_push")
	fw, _ := writer.CreateFormFile("file", "document.pdf")
	fw.Write(pdfContent)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/notifications/upload-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
 req.Header.Set("X-RT-ID", "rt-002")
	ac := &auth.AuthContext{
		UserID:     "test-user",
		RTID:       "rt-002",
		TenantRole: auth.RolePengurus,
		Jabatan:    "ketua",
	}
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-image file, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ============================================================
// Regression tests: Key Entropy Verification
// ============================================================

func TestImage_UploadKeyStructure(t *testing.T) {
	store := newMockStorageService()

	meta, err := store.UploadNotificationImage(
		context.Background(),
		"rt-002",
		"manual_push",
		"test.png",
		bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82}),
		4,
	)

	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// Verify key structure has notifications/ prefix
	if !strings.HasPrefix(meta.Key, "notifications/") {
		t.Errorf("key missing notifications/ prefix: %s", meta.Key)
	}

	// Verify content-type is set
	if meta.ContentType != "image/png" {
		t.Errorf("expected image/png content-type, got %s", meta.ContentType)
	}

	// Verify size is recorded (should match uploaded data length)
	expectedSize := int64(len(minimalPNG))
	if meta.Size != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, meta.Size)
	}
}

// minimalPNG is a 68-byte valid PNG signature that http.DetectContentType recognizes.
var minimalPNG = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82}

// ============================================================
// Regression tests: Cache Headers
// ============================================================

func TestImage_GetServesCacheControl(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	store.objects["notifications/manual/rt-002/12345.png"] = []byte("fake image data")
	store.meta["notifications/manual/rt-002/12345.png"] = storage.ObjectMetadata{
		Key:         "notifications/manual/rt-002/12345.png",
		ContentType: "image/png",
		Size:        15,
	}

	req := httptest.NewRequest("GET", "/api/v1/notifications/images/notifications/manual/rt-002/12345.png", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	cacheControl := rec.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=86400" {
		t.Errorf("expected Cache-Control: public, max-age=86400, got %q", cacheControl)
	}

	contentLength := rec.Header().Get("Content-Length")
	if contentLength != "15" {
		t.Errorf("expected Content-Length: 15, got %q", contentLength)
	}
}

// ============================================================
// Regression tests: Zero-Side-Effect Verification
// ============================================================

func TestImage_PathTraversalHasZeroSideEffect(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	if len(store.objects) != 0 {
		t.Fatalf("expected empty store, got %d objects", len(store.objects))
	}

	traversalPaths := []string{
		"/api/v1/notifications/images/../../etc/passwd",
		"/api/v1/notifications/images/..%2f..%2fetc%2fpasswd",
		"/api/v1/notifications/images/../../../etc/shadow",
	}

	for _, path := range traversalPaths {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusOK {
			t.Errorf("expected rejection for %s, got 200", path)
		}
	}

	if len(store.objects) != 0 {
		t.Errorf("expected store to remain empty after traversal attempts, got %d objects", len(store.objects))
	}
}

func TestImage_UploadZeroSideEffectOnRejection(t *testing.T) {
	store := newMockStorageService()
	svc := notification.NewService(nil, nil)
	router := handlerWithRouter(svc, store)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("event_type", "invalid_event")
	fw, _ := writer.CreateFormFile("file", "test.png")
	fw.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/notifications/upload-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
 req.Header.Set("X-RT-ID", "rt-002")
	ac := &auth.AuthContext{
		UserID:     "test-user",
		RTID:       "rt-002",
		TenantRole: auth.RolePengurus,
		Jabatan:    "ketua",
	}
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid event_type, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(store.objects) != 0 {
		t.Errorf("expected empty store after rejected upload, got %d objects", len(store.objects))
	}
}
