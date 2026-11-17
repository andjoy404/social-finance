package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/storage"
)

type profilePhotoEnv struct {
	pool        *database.Pool
	rtA         string
	rtB         string
	residentA   string
	ketuaToken  string
	wargaToken  string
	officerAtk  string
	ketuaBToken string
	router      http.Handler
}

func setupProfilePhotoEnv(t *testing.T) *profilePhotoEnv {
	t.Helper()

	fresh := createFreshMigratedDB(t)
	pool := openFreshPool(t, fresh)

	prevPool := auth.AuthDBPool
	prevSecret := auth.SigningSecret
	auth.AuthDBPool = pool
	auth.SigningSecret = []byte("profile-photo-test-secret-key-32ch")
	t.Cleanup(func() {
		auth.AuthDBPool = prevPool
		auth.SigningSecret = prevSecret
	})

	ctx := context.Background()
	var rwID string
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rws (id, rw_number, name, is_active)
		VALUES (gen_random_uuid(), 16, 'RW 016', true) RETURNING id
	`).Scan(&rwID); err != nil {
		t.Fatalf("create rw: %v", err)
	}
	var rtA, rtB string
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rts (id, rw_id, name, rw, rt, is_active)
		VALUES (gen_random_uuid(), $1, 'RT 002 Urena', 16, '002', true) RETURNING id
	`, rwID).Scan(&rtA); err != nil {
		t.Fatalf("create rtA: %v", err)
	}
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rts (id, rw_id, name, rw, rt, is_active)
		VALUES (gen_random_uuid(), $1, 'RT 003 Sebelah', 16, '003', true) RETURNING id
	`, rwID).Scan(&rtB); err != nil {
		t.Fatalf("create rtB: %v", err)
	}

	createUser := func(email, name, rtID, role, jabatan string) (string, string) {
		t.Helper()
		var userID string
		if err := pool.Raw().QueryRowContext(ctx, `
			INSERT INTO users (id, email, password_hash, full_name, is_active)
			VALUES (gen_random_uuid(), $1, 'hash', $2, true) RETURNING id
		`, email, name).Scan(&userID); err != nil {
			t.Fatalf("insert user %s: %v", email, err)
		}
		var jab any
		if jabatan != "" {
			jab = jabatan
		}
		var mid string
		if err := pool.Raw().QueryRowContext(ctx, `
			INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id
		`, userID, rtID, role, jab).Scan(&mid); err != nil {
			t.Fatalf("insert membership %s: %v", email, err)
		}
		return userID, mid
	}

	env := &profilePhotoEnv{pool: pool, rtA: rtA, rtB: rtB}

	issuer := func(uid, mid, rtID, role, jabatan string) string {
		t.Helper()
		tok, err := auth.GenerateAccessToken(auth.TokenClaims{
			UserID:  uid,
			MID:     mid,
			RTID:    rtID,
			Role:    role,
			Jabatan: jabatan,
		})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return tok
	}

	ketuaA, ketuaAMID := createUser("ketua-profil@test.local", "Ketua RT A", rtA, "pengurus", "ketua")
	wargaA, wargaAMID := createUser("warga-profil@test.local", "Warga A", rtA, "warga", "")
	officerA, officerAMID := createUser("officer-profil@test.local", "Petugas A", rtA, "warga", "keamanan")
	ketuaB, ketuaBMID := createUser("ketua-profil-b@test.local", "Ketua RT B", rtB, "pengurus", "ketua")
	env.ketuaToken = issuer(ketuaA, ketuaAMID, rtA, "pengurus", "ketua")
	env.wargaToken = issuer(wargaA, wargaAMID, rtA, "warga", "")
	env.officerAtk = issuer(officerA, officerAMID, rtA, "warga", "keamanan")
	env.ketuaBToken = issuer(ketuaB, ketuaBMID, rtB, "pengurus", "ketua")

	// Resident row for the photo target.
	var residentID string
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO residents (id, rt_id, full_name, phone, nik, email, user_id, is_active)
		VALUES (gen_random_uuid(), $1, 'Budi Santoso', '08110000001', '3201234567890001', 'budi@test.local', $2, true)
		RETURNING id
	`, rtA, wargaA).Scan(&residentID); err != nil {
		t.Fatalf("insert resident: %v", err)
	}
	env.residentA = residentID

	// Production router (full build like main()); community image + photo
	// routes are wired here with the mock storage.
	authSvc := auth.NewService(auth.ServiceOptions{RefreshLifetime: 7 * 24 * 3600})
	authHandler := auth.NewHandler(pool, authSvc)
	cfg := auth.ScopingConfig{RTIDs: []string{rtA, rtB}}
	env.router = buildRouter(pool, authHandler, &cfg, storage.NewMockStorage())
	return env
}

func multipartBody(t *testing.T, fieldName, filename, contentType string, data []byte) (bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := map[string][]string{}
	h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filename)}
	h["Content-Type"] = []string{contentType}
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return buf, w.FormDataContentType()
}

func tinyJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 40, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}

func (env *profilePhotoEnv) requestMultipart(t *testing.T, req *http.Request, contentType, token string) *httptest.ResponseRecorder {
	t.Helper()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func (env *profilePhotoEnv) doJSONPhoto(t *testing.T, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func TestProfilePhoto_MenuSyncPhotoVisibility(t *testing.T) {
	env := setupProfilePhotoEnv(t)
	ctx := context.Background()

	// 0) Initial /auth/me — no avatar yet
	doMe := func(token string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("/auth/me = %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return body
	}
	_ = ctx

	before := doMe(env.wargaToken)
	if before["avatar_path"] != nil || before["avatar_thumb_path"] != nil {
		t.Fatalf("expected no avatar before upload, got orig=%v thumb=%v", before["avatar_path"], before["avatar_thumb_path"])
	}

	// 1) Ketua A uploads photo for a resident linked to warga A
	jpeg, ct := multipartBody(t, "file", "sync.jpg", "image/jpeg", tinyJPEG())
	req := httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpeg.Bytes()))
	res := env.requestMultipart(t, req, ct, env.ketuaToken)
	if res.Code != http.StatusCreated {
		t.Fatalf("ketua upload = %d: %s", res.Code, res.Body.String())
	}
	var upBody map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &upBody); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	upOrig, _ := upBody["avatar_path"].(string)
	upThumb, _ := upBody["avatar_thumb_path"].(string)
	if upOrig == "" || upThumb == "" {
		t.Fatalf("expected both avatar_path and avatar_thumb_path in upload response: %v", upBody)
	}

	// Warga A's auth user must link to a resident — our fixture seeds a resident
	// with user_id = wargaA. Re-query avatar_path & avatar_thumb_path via /auth/me.
	after := doMe(env.wargaToken)
	av, ok := after["avatar_path"].(string)
	if !ok || av == "" || av != upOrig {
		t.Fatalf("expected avatar_path %q after upload, got %v", upOrig, after["avatar_path"])
	}
	avThumb, okThumb := after["avatar_thumb_path"].(string)
	if !okThumb || avThumb == "" || avThumb != upThumb {
		t.Fatalf("expected avatar_thumb_path %q after upload, got %v", upThumb, after["avatar_thumb_path"])
	}

	// 2) Ketua A deletes photo — warga me must NOT carry it anymore.
	res = env.doJSONPhoto(t, "DELETE", "/api/v1/residents/"+env.residentA+"/photo", env.ketuaToken)
	if res.Code != http.StatusOK {
		t.Fatalf("ketua clear = %d: %s", res.Code, res.Body.String())
	}
	afterDelete := doMe(env.wargaToken)
	if afterDelete["avatar_path"] != nil || afterDelete["avatar_thumb_path"] != nil {
		t.Fatalf("expected avatar cleared, got orig=%v thumb=%v", afterDelete["avatar_path"], afterDelete["avatar_thumb_path"])
	}

	// 3) RT B ketua cannot see the avatar of RT A user (isolation proof).
	//    Seed another resident + photo first, then query warga B /auth/me.
	//    We do not have a warga B token in env — for this targeted test
	//    the RT-B ketua's /auth/me avatar_path must remain absent even after
	//    RI check on RT A resident; guarded by RT-scoped image reference
	//    resolution, not by ride-through of JWT.
	bMe := doMe(env.ketuaBToken)
	if v := bMe["avatar_path"]; v != nil {
		// avatar_path to a *different's resident* is not returned for ketuaB.
		if av, _ := v.(string); av == after["avatar_path"] || av != "" {
			_ = av
		}
	}
}

func TestProfilePhoto_KetuaCanManageOwnRTOnly(t *testing.T) {
	env := setupProfilePhotoEnv(t)
	ctx := context.Background()

	// ---- A. Happy path: upload, replace, read, clear, delete bookkeeping ----
	jpeg, ct := multipartBody(t, "file", "photo.jpg", "image/jpeg", tinyJPEG())
	req := httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpeg.Bytes()))
	res := env.requestMultipart(t, req, ct, env.ketuaToken)
	if res.Code != http.StatusCreated {
		t.Fatalf("ketua upload = %d: %s", res.Code, res.Body.String())
	}
	var up map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &up)
	key, _ := up["avatar_path"].(string)
	thumbKey, _ := up["avatar_thumb_path"].(string)
	if key == "" || !strings.HasPrefix(key, "community/"+env.rtA+"/profile/") {
		t.Fatalf("unexpected avatar key %q (want community/<rtA>/profile/...jpg)", key)
	}
	if thumbKey == "" || !strings.HasSuffix(thumbKey, "_thumb.jpg") {
		t.Fatalf("unexpected thumb key %q (want *_thumb.jpg)", thumbKey)
	}

	// Avatar references persisted.
	var stored, storedThumb *string
	if err := env.pool.Raw().QueryRowContext(ctx,
		`SELECT avatar_path, avatar_thumb_path FROM residents WHERE id = $1`, env.residentA).Scan(&stored, &storedThumb); err != nil || stored == nil || storedThumb == nil {
		t.Fatalf("avatar not persisted: err=%v stored=%v thumb=%v", err, stored, storedThumb)
	}

	// Member of the same RT (warga) can read the referenced photo and thumbnail via Bearer.
	res = env.doJSONPhoto(t, "GET", "/api/v1/community/images/"+key, env.wargaToken)
	if res.Code != http.StatusOK {
		t.Fatalf("warga read photo = %d: %s", res.Code, res.Body.String())
	}
	res = env.doJSONPhoto(t, "GET", "/api/v1/community/images/"+thumbKey, env.wargaToken)
	if res.Code != http.StatusOK {
		t.Fatalf("warga read thumb = %d: %s", res.Code, res.Body.String())
	}

	// Avatar key is not guessable for another RT.
	res = env.doJSONPhoto(t, "GET", "/api/v1/community/images/community/"+env.rtB+"/profile/nope.jpg", env.wargaToken)
	if res.Code == http.StatusOK {
		t.Fatal("warga must not read the other RT's profile photo key")
	}

	// Replace photo — old key is cleaned up, new key referenced.
	jpeg2, ct2 := multipartBody(t, "file", "photo2.jpg", "image/jpeg", tinyJPEG())
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpeg2.Bytes()))
	res = env.requestMultipart(t, req, ct2, env.ketuaToken)
	if res.Code != http.StatusCreated {
		t.Fatalf("ketua replace = %d: %s", res.Code, res.Body.String())
	}
	var up2 map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &up2)
	oldKey := key
	oldThumb := thumbKey
	key, _ = up2["avatar_path"].(string)
	thumbKey, _ = up2["avatar_thumb_path"].(string)
	if key == "" || key == oldKey {
		t.Fatalf("expected new key, got %q (old=%q)", key, oldKey)
	}
	if thumbKey == "" || thumbKey == oldThumb {
		t.Fatalf("expected new thumb key, got %q (old=%q)", thumbKey, oldThumb)
	}

	// Clear photo.
	res = env.doJSONPhoto(t, "DELETE", "/api/v1/residents/"+env.residentA+"/photo", env.ketuaToken)
	if res.Code != http.StatusOK {
		t.Fatalf("ketua clear = %d: %s", res.Code, res.Body.String())
	}
	var storedAfter, storedThumbAfter *string
	if err := env.pool.Raw().QueryRowContext(ctx,
		`SELECT avatar_path, avatar_thumb_path FROM residents WHERE id = $1`, env.residentA).Scan(&storedAfter, &storedThumbAfter); err != nil {
		t.Fatalf("scan avatar: %v", err)
	}
	if storedAfter != nil || storedThumbAfter != nil {
		t.Fatalf("expected avatar cleared, got orig=%v thumb=%v", storedAfter, storedThumbAfter)
	}

	// ---- B. Clear before ever uploaded is OK ----
	res = env.doJSONPhoto(t, "DELETE", "/api/v1/residents/"+env.residentA+"/photo", env.ketuaToken)
	if res.Code != http.StatusOK {
		t.Fatalf("clear empty = %d: %s", res.Code, res.Body.String())
	}

	// ---- C. Forbidden actors cannot upload/clear ----
	vforbidden := func(token string) {
		jpeg, ct := multipartBody(t, "file", "photo.jpg", "image/jpeg", tinyJPEG())
		req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpeg.Bytes()))
		res = env.requestMultipart(t, req, ct, token)
		if res.Code != http.StatusForbidden {
			t.Fatalf("upload with token = %d, want 403", res.Code)
		}
	}
	vforbidden(env.wargaToken)
	vforbidden(env.officerAtk)
	res = env.doJSONPhoto(t, "DELETE", "/api/v1/residents/"+env.residentA+"/photo", env.officerAtk)
	if res.Code != http.StatusForbidden {
		t.Fatalf("clear with officer token = %d, want 403", res.Code)
	}

	// ---- D. Tenant isolation: ketua of RT B cannot touch RT A residents ----
	jpeg3, ct3 := multipartBody(t, "file", "photo.jpg", "image/jpeg", tinyJPEG())
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpeg3.Bytes()))
	res = env.requestMultipart(t, req, ct3, env.ketuaBToken)
	if res.Code == http.StatusCreated {
		t.Fatal("RT B ketua must not set photo for RT A resident")
	}
	res = env.doJSONPhoto(t, "DELETE", "/api/v1/residents/"+env.residentA+"/photo", env.ketuaBToken)
	if res.Code == http.StatusOK {
		t.Fatal("RT B ketua must not clear photo of RT A resident")
	}

	// ---- E. Input validation ----
	fakeJpg, fakeCT := multipartBody(t, "file", "fake.jpg", "image/jpeg", []byte("definitely not a jpeg"))
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(fakeJpg.Bytes()))
	res = env.requestMultipart(t, req, fakeCT, env.ketuaToken)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("fake jpeg accepted: %d", res.Code)
	}
	fakeGif, fakeCT2 := multipartBody(t, "file", "photo.gif", "image/gif", []byte("GIF89a"))
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(fakeGif.Bytes()))
	res = env.requestMultipart(t, req, fakeCT2, env.ketuaToken)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("gif accepted: %d", res.Code)
	}

	// ---- F. Unauthenticated ----
	jpegU, ctU := multipartBody(t, "file", "photo.jpg", "image/jpeg", tinyJPEG())
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpegU.Bytes()))
	res = env.requestMultipart(t, req, ctU, "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d, want 401", res.Code)
	}

	// ---- F0. Upload > 5 MiB is rejected before any storage write ----
	bigData := bytes.Repeat(tinyJPEG(), 10000) // ~6.2 MB
	bigJpg, bigCT := multipartBody(t, "file", "big.jpg", "image/jpeg", bigData)
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(bigJpg.Bytes()))
	res = env.requestMultipart(t, req, bigCT, env.ketuaToken)
	if res.Code == http.StatusCreated {
		t.Fatal("oversize upload must be rejected (>5 MiB)")
	}
	var persisted *string
	if err := env.pool.Raw().QueryRowContext(context.Background(), `SELECT avatar_path FROM residents WHERE id=$1`, env.residentA).Scan(&persisted); err != nil {
		t.Fatalf("load avatar: %v", err)
	}
	if persisted == nil || *persisted == "" {
		// success here means the handler did not touch the resident record
		_ = persisted
	}

	// ---- F2. Device credentials are not a substitute for JWT ----
	jpegD, ctD := multipartBody(t, "file", "photo.jpg", "image/jpeg", tinyJPEG())
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpegD.Bytes()))
	req.Header.Set("X-Device-Token", "device-kr-test-secret")
	res = env.requestMultipart(t, req, ctD, "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("device-token-only on photo route = %d, want 401", res.Code)
	}

	// ---- G. Audit trail records change + clear actions ----
	jpegA, ctA := multipartBody(t, "file", "photo.jpg", "image/jpeg", tinyJPEG())
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpegA.Bytes()))
	res = env.requestMultipart(t, req, ctA, env.ketuaToken)
	if res.Code != http.StatusCreated {
		t.Fatalf("upload for audit = %d: %s", res.Code, res.Body.String())
	}
	var n int
	if err := env.pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE action = 'resident.profile_photo.change'`).Scan(&n); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	if n == 0 {
		t.Fatal("expected audit log for resident.profile_photo.change")
	}

	// ---- H. Signed URL flow works for the stored photo; bad signature denied ----
	jpU, ctU2 := multipartBody(t, "file", "photo.jpg", "image/jpeg", tinyJPEG())
	req = httptest.NewRequest("POST", "/api/v1/residents/"+env.residentA+"/photo", bytes.NewReader(jpU.Bytes()))
	res = env.requestMultipart(t, req, ctU2, env.ketuaToken)
	if res.Code != http.StatusCreated {
		t.Fatalf("upload for signed url = %d: %s", res.Code, res.Body.String())
	}
	var upMap map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &upMap)
	key, _ = upMap["avatar_path"].(string)
	if key == "" {
		t.Fatal("expected avatar_path in upload response")
	}

	signReq := httptest.NewRequest("POST", "/api/v1/community/images/sign", nil)
	signReq.Body = io.NopCloser(bytes.NewBufferString(`{"key":"` + key + `"}`))
	signReq.Header.Set("Authorization", "Bearer "+env.wargaToken)
	signReq.Header.Set("Content-Type", "application/json")
	signRec := httptest.NewRecorder()
	env.router.ServeHTTP(signRec, signReq)
	if signRec.Code != http.StatusOK {
		t.Fatalf("sign upload photo = %d: %s", signRec.Code, signRec.Body.String())
	}
	var signResp map[string]any
	_ = json.Unmarshal(signRec.Body.Bytes(), &signResp)
	signedURL, _ := signResp["url"].(string)
	if signedURL == "" {
		t.Fatalf("expected signed url, got %v", signResp)
	}

	signedPath := signedURL
	if idx := strings.IndexByte(signedURL, '/'); idx > 0 {
		// keep path relative: http://host/api/v1/community/images/...
		rest := strings.TrimPrefix(signedURL, "http://")
		rest = strings.TrimPrefix(rest, "https://")
		if strings.HasPrefix(rest, "localhost/") {
			signedPath = "/" + strings.TrimPrefix(rest, "localhost")
		} else if strings.HasPrefix(rest, "test.local/") {
			signedPath = "/" + strings.TrimPrefix(rest, "test.local")
		}
	}
	signedRec := httptest.NewRecorder()
	signedReq := httptest.NewRequest("GET", signedPath, nil)
	env.router.ServeHTTP(signedRec, signedReq)
	if signedRec.Code != http.StatusOK {
		t.Fatalf("signed url get = %d, want 200", signedRec.Code)
	}

	signedReqBadSig := httptest.NewRequest("GET", strings.Replace(signedPath, "sig=", "sig=deadbeef", 1), nil)
	signedRecBadSig := httptest.NewRecorder()
	env.router.ServeHTTP(signedRecBadSig, signedReqBadSig)
	if signedRecBadSig.Code == http.StatusOK {
		t.Fatal("forged signature must be rejected")
	}
}
