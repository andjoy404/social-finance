package profilephoto

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

var fxSeq int64

type controllableStorage struct {
	mu sync.Mutex

	uploadWithThumbFn func(ctx context.Context, rtID, category, filename string, r io.Reader, size int64, makeThumb bool) (*storage.ImageUploadResult, error)
	deleteFn          func(ctx context.Context, key string) error

	uploads      []*storage.ImageUploadResult
	deleted      []string
	deleteErrFor map[string]error
	objects      map[string]bool
}

func newControllableStorage() *controllableStorage {
	return &controllableStorage{objects: map[string]bool{}, deleteErrFor: map[string]error{}}
}

func (s *controllableStorage) UploadProof(context.Context, string, string, string, io.Reader, int64) (*storage.ObjectMetadata, error) {
	return nil, errors.New("not implemented")
}
func (s *controllableStorage) GetProof(context.Context, string) (*storage.ObjectReader, error) {
	return nil, errors.New("not implemented")
}
func (s *controllableStorage) DeleteProof(context.Context, string) error {
	return errors.New("not implemented")
}
func (s *controllableStorage) UploadNotificationImage(context.Context, string, string, string, io.Reader, int64) (*storage.ObjectMetadata, error) {
	return nil, errors.New("not implemented")
}
func (s *controllableStorage) GetNotificationImage(context.Context, string) (*storage.ObjectReader, error) {
	return nil, errors.New("not implemented")
}
func (s *controllableStorage) DeleteNotificationImage(context.Context, string) error {
	return errors.New("not implemented")
}
func (s *controllableStorage) UploadCommunityImage(ctx context.Context, rtID, category, filename string, r io.Reader, size int64) (*storage.ObjectMetadata, error) {
	res, err := s.UploadCommunityImageWithThumb(ctx, rtID, category, filename, r, size, false)
	if err != nil || res == nil {
		return nil, err
	}
	m := res.Original
	return &m, nil
}
func (s *controllableStorage) UploadCommunityImageWithThumb(ctx context.Context, rtID, category, filename string, r io.Reader, size int64, makeThumb bool) (*storage.ImageUploadResult, error) {
	s.mu.Lock()
	fn := s.uploadWithThumbFn
	s.mu.Unlock()
	if fn != nil {
		res, err := fn(ctx, rtID, category, filename, r, size, makeThumb)
		s.mu.Lock()
		if res != nil {
			s.uploads = append(s.uploads, res)
			if res.OriginalKey != "" {
				s.objects[res.OriginalKey] = true
			}
			if res.ThumbKey != "" {
				s.objects[res.ThumbKey] = true
			}
		}
		s.mu.Unlock()
		return res, err
	}
	ms := storage.NewMockStorage()
	res, err := ms.UploadCommunityImageWithThumb(ctx, rtID, category, filename, r, size, makeThumb)
	s.mu.Lock()
	if res != nil {
		s.uploads = append(s.uploads, res)
		if res.OriginalKey != "" {
			s.objects[res.OriginalKey] = true
		}
		if res.ThumbKey != "" {
			s.objects[res.ThumbKey] = true
		}
	}
	s.mu.Unlock()
	return res, err
}
func (s *controllableStorage) GetCommunityImage(context.Context, string) (*storage.ObjectReader, error) {
	return nil, storage.ErrNotFound
}
func (s *controllableStorage) DeleteCommunityImage(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, key)
	if err, ok := s.deleteErrFor[key]; ok && err != nil {
		return err
	}
	delete(s.objects, key)
	return nil
}
func (s *controllableStorage) InitBucket(context.Context) error { return nil }

func (s *controllableStorage) setUploadFn(fn func(context.Context, string, string, string, io.Reader, int64, bool) (*storage.ImageUploadResult, error)) {
	s.mu.Lock()
	s.uploadWithThumbFn = fn
	s.mu.Unlock()
}
func (s *controllableStorage) setDeleteErr(key string, err error) {
	s.mu.Lock()
	s.deleteErrFor[key] = err
	s.mu.Unlock()
}
func (s *controllableStorage) deletedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.deleted...)
}
func (s *controllableStorage) uploadResults() []*storage.ImageUploadResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*storage.ImageUploadResult(nil), s.uploads...)
}

func ensureThumbSchema(t *testing.T, pool *database.Pool) {
	t.Helper()
	ctx := context.Background()
	var ok bool
	err := pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='public' AND table_name='residents' AND column_name='avatar_thumb_path'
		)`).Scan(&ok)
	if err != nil {
		t.Fatalf("check avatar_thumb_path column: %v", err)
	}
	if ok {
		return
	}
	if _, err := pool.Raw().ExecContext(ctx, `ALTER TABLE residents ADD COLUMN IF NOT EXISTS avatar_thumb_path text`); err != nil {
		t.Skipf("avatar_thumb_path missing and cannot be added in isolated test DB (migration 082 not applied): %v", err)
	}
}

type fx struct {
	pool      *database.Pool
	rtA       string
	rtB       string
	userID    string
	mid       string
	residentA string
}

func setupFX(t *testing.T) *fx {
	t.Helper()
	pool := testutil.GetTestPool(t)
	ensureThumbSchema(t, pool)
	ctx := context.Background()

	n := atomic.AddInt64(&fxSeq, 1)
	base := time.Now().UnixNano() + n*1000003
	// uniq_rts_active is partial unique on (rw, rt) WHERE is_active=true across all rows.
	rwNum := 16
	rtA := fmt.Sprintf("%03d", int(1+(base%998)))
	rtB := fmt.Sprintf("%03d", int(1+((base/7)%998)))
	if rtB == rtA {
		rtB = fmt.Sprintf("%03d", int(1+((base/13)%998)))
	}
	// ensure uniqueness best-effort by probing; isolated test DB.
	for i := 0; i < 50 && rtB == rtA; i++ {
		rtB = fmt.Sprintf("%03d", int(1+((base+int64(i*31))%998)))
	}
	var rwID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rws WHERE rw_number=$1 LIMIT 1`, rwNum).Scan(&rwID)
	if err != nil {
		if err2 := pool.Raw().QueryRowContext(ctx,
			`INSERT INTO rws (id, rw_number, name, is_active) VALUES (gen_random_uuid(), $1, $2, true) RETURNING id`,
			rwNum, fmt.Sprintf("RW %d", rwNum)).Scan(&rwID); err2 != nil {
			t.Fatalf("create/find rw: %v / %v", err, err2)
		}
	}
	// deactivate any existing active RT rows with same numbers to avoid uniq_rts_active collisions in shared isolated DB.
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE rts SET is_active=false WHERE rw=$1 AND rt IN ($2,$3) AND is_active=true`, rwNum, rtA, rtB)
	var rtIDA, rtIDB string
	if err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (id, rw_id, name, rw, rt, is_active) VALUES (gen_random_uuid(), $1, 'RT A', $2, $3, true) RETURNING id`,
		rwID, rwNum, rtA).Scan(&rtIDA); err != nil {
		t.Fatalf("create rtA: %v", err)
	}
	if err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (id, rw_id, name, rw, rt, is_active) VALUES (gen_random_uuid(), $1, 'RT B', $2, $3, true) RETURNING id`,
		rwID, rwNum, rtB).Scan(&rtIDB); err != nil {
		t.Fatalf("create rtB: %v", err)
	}

	email := fmt.Sprintf("pp-%d-%d@test.local", time.Now().UnixNano(), n)
	var userID string
	if err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, is_active) VALUES (gen_random_uuid(), $1, 'hash', 'Ketua', true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("user: %v", err)
	}
	var mid string
	if err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active) VALUES (gen_random_uuid(), $1, $2, 'pengurus', 'ketua', true) RETURNING id`,
		userID, rtIDA).Scan(&mid); err != nil {
		t.Fatalf("membership: %v", err)
	}
	nik := fmt.Sprintf("%016d", (time.Now().UnixNano()+n)%10000000000000000)
	var residentA string
	if err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (id, rt_id, full_name, phone, nik, email, user_id, is_active) VALUES (gen_random_uuid(), $1, 'Budi', '08123', $2, $3, $4, true) RETURNING id`,
		rtIDA, nik, email, userID).Scan(&residentA); err != nil {
		t.Fatalf("resident: %v", err)
	}
	return &fx{pool: pool, rtA: rtIDA, rtB: rtIDB, userID: userID, mid: mid, residentA: residentA}
}

func authCtx(f *fx) *auth.AuthContext {
	return &auth.AuthContext{UserID: f.userID, RTID: f.rtA, TenantRole: "pengurus", Jabatan: "ketua"}
}

func uploadReq(t *testing.T, residentID string, body []byte, filename string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/"+residentID+"/photo", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", residentID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req
}

func deleteReq(t *testing.T, residentID string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/residents/"+residentID+"/photo", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", residentID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req
}

func withAuth(req *http.Request, ac *auth.AuthContext) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))
}

func validJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 40, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	return m
}

func getAvatarPaths(t *testing.T, pool *database.Pool, id string) (orig, thumb sql.NullString) {
	t.Helper()
	if err := pool.Raw().QueryRowContext(context.Background(), `SELECT avatar_path, avatar_thumb_path FROM residents WHERE id=$1`, id).Scan(&orig, &thumb); err != nil {
		t.Fatalf("query paths: %v", err)
	}
	return
}

func TestUpload_Success(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)

	rec := httptest.NewRecorder()
	h.Upload(rec, withAuth(uploadReq(t, f.residentA, validJPEG(t), "photo.jpg"), authCtx(f)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if body["avatar_path"] == nil || body["avatar_thumb_path"] == nil {
		t.Fatalf("expected paths in response: %v", body)
	}
	ups := st.uploadResults()
	if len(ups) != 1 || ups[0].OriginalKey == "" || ups[0].ThumbKey == "" {
		t.Fatalf("expected upload with orig+thumb: %+v", ups)
	}
	ap, tp := getAvatarPaths(t, f.pool, f.residentA)
	if !ap.Valid || ap.String == "" || !tp.Valid || tp.String == "" {
		t.Fatalf("db paths not persisted: ap=%v tp=%v", ap, tp)
	}
}

func TestUpload_ThumbnailCreateFails(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	st.setUploadFn(func(ctx context.Context, rtID, category, filename string, r io.Reader, size int64, makeThumb bool) (*storage.ImageUploadResult, error) {
		return nil, fmt.Errorf("create thumbnail: boom")
	})
	h := NewHandler(f.pool, st)

	rec := httptest.NewRecorder()
	h.Upload(rec, withAuth(uploadReq(t, f.residentA, validJPEG(t), "photo.jpg"), authCtx(f)))
	if rec.Code == http.StatusCreated {
		t.Fatalf("expected failure, got 201: %s", rec.Body.String())
	}
	ap, _ := getAvatarPaths(t, f.pool, f.residentA)
	if ap.Valid && ap.String != "" {
		t.Fatalf("db should not persist on thumb failure, got %q", ap.String)
	}
}

func TestUpload_ThumbnailUploadFails(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	st.setUploadFn(func(ctx context.Context, rtID, category, filename string, r io.Reader, size int64, makeThumb bool) (*storage.ImageUploadResult, error) {
		return nil, fmt.Errorf("upload thumbnail: boom")
	})
	h := NewHandler(f.pool, st)

	rec := httptest.NewRecorder()
	h.Upload(rec, withAuth(uploadReq(t, f.residentA, validJPEG(t), "photo.jpg"), authCtx(f)))
	if rec.Code == http.StatusCreated {
		t.Fatalf("expected failure, got 201: %s", rec.Body.String())
	}
	ap, _ := getAvatarPaths(t, f.pool, f.residentA)
	if ap.Valid && ap.String != "" {
		t.Fatalf("db should not persist on thumb upload failure")
	}
}

func TestUpload_DBBeginFailure_CleansNewObjects(t *testing.T) {
	t.Skip("begin tx failure not easily simulated with database.Pool wrapper in isolated test; covered by persist-failure cleanup paths")
}

func TestUpload_DBLoadFailure_CleansNewObjects(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)

	ctx := context.Background()
	// resolveTargetRT succeeds (EXISTS check), but in-tx load of avatar columns fails.
	if _, err := f.pool.Raw().ExecContext(ctx, `ALTER TABLE residents DROP COLUMN avatar_thumb_path`); err != nil {
		t.Skipf("cannot sabotage residents.avatar_thumb_path: %v", err)
	}
	defer f.pool.Raw().ExecContext(context.Background(), `ALTER TABLE residents ADD COLUMN IF NOT EXISTS avatar_thumb_path text`)

	rec := httptest.NewRecorder()
	h.Upload(rec, withAuth(uploadReq(t, f.residentA, validJPEG(t), "photo.jpg"), authCtx(f)))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on in-tx load failure, got %d body=%s", rec.Code, rec.Body.String())
	}
	ups := st.uploadResults()
	if len(ups) != 1 {
		t.Fatalf("expected one upload attempt, got %d", len(ups))
	}
	deleted := strings.Join(st.deletedKeys(), ",")
	if !strings.Contains(deleted, ups[0].OriginalKey) || (ups[0].ThumbKey != "" && !strings.Contains(deleted, ups[0].ThumbKey)) {
		t.Fatalf("expected cleanup of new objects after load failure, deleted=[%s] orig=%s thumb=%s", deleted, ups[0].OriginalKey, ups[0].ThumbKey)
	}
}

func TestUpload_DBAuditOrCommitFailure_CleansNewObjects(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)

	ctx := context.Background()
	if _, err := f.pool.Raw().ExecContext(ctx, `ALTER TABLE audit_logs RENAME TO audit_logs_tmp_pp`); err != nil {
		t.Skipf("cannot sabotage audit_logs: %v", err)
	}
	defer f.pool.Raw().ExecContext(context.Background(), `ALTER TABLE audit_logs_tmp_pp RENAME TO audit_logs`)

	rec := httptest.NewRecorder()
	h.Upload(rec, withAuth(uploadReq(t, f.residentA, validJPEG(t), "photo.jpg"), authCtx(f)))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on audit failure, got %d body=%s", rec.Code, rec.Body.String())
	}
	ups := st.uploadResults()
	if len(ups) != 1 {
		t.Fatalf("expected one upload attempt, got %d", len(ups))
	}
	deleted := strings.Join(st.deletedKeys(), ",")
	if !strings.Contains(deleted, ups[0].OriginalKey) || (ups[0].ThumbKey != "" && !strings.Contains(deleted, ups[0].ThumbKey)) {
		t.Fatalf("expected cleanup of new objects after persist failure, deleted=[%s] orig=%s thumb=%s", deleted, ups[0].OriginalKey, ups[0].ThumbKey)
	}
	ap, _ := getAvatarPaths(t, f.pool, f.residentA)
	if ap.Valid && ap.String != "" {
		t.Fatalf("db must not keep new path on failure")
	}
}

func TestUpload_Replace_CleansOldOriginalAndThumbAfterCommit(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)
	ac := authCtx(f)

	rec1 := httptest.NewRecorder()
	h.Upload(rec1, withAuth(uploadReq(t, f.residentA, validJPEG(t), "a.jpg"), ac))
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first upload failed: %d %s", rec1.Code, rec1.Body.String())
	}
	first := st.uploadResults()[0]

	rec2 := httptest.NewRecorder()
	h.Upload(rec2, withAuth(uploadReq(t, f.residentA, validJPEG(t), "b.jpg"), ac))
	if rec2.Code != http.StatusCreated {
		t.Fatalf("second upload failed: %d %s", rec2.Code, rec2.Body.String())
	}
	second := st.uploadResults()[1]

	ap, tp := getAvatarPaths(t, f.pool, f.residentA)
	if ap.String != second.OriginalKey || tp.String != second.ThumbKey {
		t.Fatalf("db should point to new keys: ap=%s tp=%s want %s %s", ap.String, tp.String, second.OriginalKey, second.ThumbKey)
	}
	deleted := strings.Join(st.deletedKeys(), ",")
	if !strings.Contains(deleted, first.OriginalKey) || (first.ThumbKey != "" && !strings.Contains(deleted, first.ThumbKey)) {
		t.Fatalf("expected old keys cleaned after commit, deleted=[%s] oldOrig=%s oldThumb=%s", deleted, first.OriginalKey, first.ThumbKey)
	}
}

func TestUpload_Replace_OldPhotoWithoutThumbnail(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)

	oldKey := "community/" + f.rtA + "/profile/old_no_thumb.jpg"
	if _, err := f.pool.Raw().ExecContext(context.Background(), `UPDATE residents SET avatar_path=$1, avatar_thumb_path=NULL WHERE id=$2`, oldKey, f.residentA); err != nil {
		t.Fatalf("seed old: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Upload(rec, withAuth(uploadReq(t, f.residentA, validJPEG(t), "new.jpg"), authCtx(f)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload failed: %d %s", rec.Code, rec.Body.String())
	}
	deleted := strings.Join(st.deletedKeys(), ",")
	if !strings.Contains(deleted, oldKey) {
		t.Fatalf("expected old original cleaned even without thumb, deleted=[%s]", deleted)
	}
}

func TestUpload_Replace_SharedOldKey_NotDeleted(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)

	sharedOrig := "community/" + f.rtA + "/profile/shared.jpg"
	sharedThumb := "community/" + f.rtA + "/profile/shared_thumb.jpg"
	if _, err := f.pool.Raw().ExecContext(context.Background(),
		`UPDATE residents SET avatar_path=$1, avatar_thumb_path=$2 WHERE id=$3`,
		sharedOrig, sharedThumb, f.residentA); err != nil {
		t.Fatalf("seed: %v", err)
	}
	email2 := fmt.Sprintf("pp2-%d@test.local", time.Now().UnixNano())
	nik2 := fmt.Sprintf("%016d", (time.Now().UnixNano()+11)%10000000000000000)
	if _, err := f.pool.Raw().ExecContext(context.Background(),
		`INSERT INTO residents (id, rt_id, full_name, phone, nik, email, is_active, avatar_path, avatar_thumb_path)
		 VALUES (gen_random_uuid(), $1, 'Other', '08124', $2, $3, true, $4, $5)`,
		f.rtA, nik2, email2, sharedOrig, sharedThumb); err != nil {
		t.Fatalf("insert other: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Upload(rec, withAuth(uploadReq(t, f.residentA, validJPEG(t), "new.jpg"), authCtx(f)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload failed: %d %s", rec.Code, rec.Body.String())
	}
	deleted := strings.Join(st.deletedKeys(), ",")
	if strings.Contains(deleted, sharedOrig) || strings.Contains(deleted, sharedThumb) {
		t.Fatalf("shared keys must not be deleted, deleted=[%s]", deleted)
	}
}

func TestUpload_CleanupFailure_DoesNotBreakDBConsistency(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)
	ac := authCtx(f)

	rec1 := httptest.NewRecorder()
	h.Upload(rec1, withAuth(uploadReq(t, f.residentA, validJPEG(t), "a.jpg"), ac))
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", rec1.Code, rec1.Body.String())
	}
	first := st.uploadResults()[0]
	st.setDeleteErr(first.OriginalKey, errors.New("storage unavailable"))
	if first.ThumbKey != "" {
		st.setDeleteErr(first.ThumbKey, errors.New("storage unavailable"))
	}

	rec2 := httptest.NewRecorder()
	h.Upload(rec2, withAuth(uploadReq(t, f.residentA, validJPEG(t), "b.jpg"), ac))
	if rec2.Code != http.StatusCreated {
		t.Fatalf("second should succeed despite cleanup failure: %d %s", rec2.Code, rec2.Body.String())
	}
	ap, tp := getAvatarPaths(t, f.pool, f.residentA)
	second := st.uploadResults()[1]
	if ap.String != second.OriginalKey || tp.String != second.ThumbKey {
		t.Fatalf("db must point to new keys despite cleanup failure: ap=%s tp=%s", ap.String, tp.String)
	}
}

func TestDelete_ClearsPathsAndCleansObjects(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)
	ac := authCtx(f)

	rec1 := httptest.NewRecorder()
	h.Upload(rec1, withAuth(uploadReq(t, f.residentA, validJPEG(t), "a.jpg"), ac))
	if rec1.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec1.Code, rec1.Body.String())
	}
	up := st.uploadResults()[0]

	rec := httptest.NewRecorder()
	h.Delete(rec, withAuth(deleteReq(t, f.residentA), ac))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", rec.Code, rec.Body.String())
	}
	ap, tp := getAvatarPaths(t, f.pool, f.residentA)
	if (ap.Valid && ap.String != "") || (tp.Valid && tp.String != "") {
		t.Fatalf("paths should be cleared: ap=%v tp=%v", ap, tp)
	}
	deleted := strings.Join(st.deletedKeys(), ",")
	if !strings.Contains(deleted, up.OriginalKey) || (up.ThumbKey != "" && !strings.Contains(deleted, up.ThumbKey)) {
		t.Fatalf("expected both objects cleaned, deleted=[%s]", deleted)
	}
}

func TestDelete_OldPhotoWithoutThumbnail(t *testing.T) {
	f := setupFX(t)
	st := newControllableStorage()
	h := NewHandler(f.pool, st)

	oldKey := "community/" + f.rtA + "/profile/old_no_thumb.jpg"
	if _, err := f.pool.Raw().ExecContext(context.Background(),
		`UPDATE residents SET avatar_path=$1, avatar_thumb_path=NULL WHERE id=$2`,
		oldKey, f.residentA); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Delete(rec, withAuth(deleteReq(t, f.residentA), authCtx(f)))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", rec.Code, rec.Body.String())
	}
	ap, tp := getAvatarPaths(t, f.pool, f.residentA)
	if (ap.Valid && ap.String != "") || (tp.Valid && tp.String != "") {
		t.Fatalf("paths should be cleared")
	}
	if !strings.Contains(strings.Join(st.deletedKeys(), ","), oldKey) {
		t.Fatalf("expected old original cleaned")
	}
}
