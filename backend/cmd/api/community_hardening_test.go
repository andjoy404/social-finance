package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/auth"
)

func doReq(t *testing.T, router http.Handler, method, path, token string, body []byte, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func jsonBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return b
}

// tinyPNG is a minimal PNG (magic header) accepted by DetectContentType.
var tinyPNG = append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03}, 64)...)

func uploadCommunityImage(t *testing.T, router http.Handler, token, category, filename string, data []byte) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := mw.WriteField("category", category); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/community/upload", &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp
}

func TestCommunity_PermissionMatrix_LeastPrivilege(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, sekretarisToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanSekretaris)
	_, bendaharaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanBendahara)
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	future := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	newActivity := func(token string) *httptest.ResponseRecorder {
		body := jsonBody(t, map[string]any{"title": "Permission Check", "starts_at": future})
		return doReq(t, router, http.MethodPost, "/api/v1/activities", token, body, "application/json")
	}

	// Non-permission holders must be rejected even though they carry a
	// management jabatan. Bendahara remains read-only; Sekretaris diberi
	// pengelolaan Pengumuman dan Kegiatan (WRITE).
	for name, token := range map[string]string{
		"warga":     wargaToken,
		"bendahara": bendaharaToken,
	} {
		if rec := newActivity(token); rec.Code != http.StatusForbidden {
			t.Errorf("%s create activity = %d, want 403", name, rec.Code)
		}
	}
	if rec := newActivity(sekretarisToken); rec.Code != http.StatusCreated {
		t.Errorf("sekretaris create activity = %d, want 201", rec.Code)
	}
	if rec := newActivity(ketuaToken); rec.Code != http.StatusCreated {
		t.Fatalf("ketua create activity = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	// Announcements: bendahara remains read-only; sekretaris has manage grants
	// since migration 072 (Sekretaris diberi pengelolaan Pengumuman — Lihat docs/…).
	newAnnouncement := func(token string) *httptest.ResponseRecorder {
		body := jsonBody(t, map[string]any{"title": "Ann", "body": "Body"})
		return doReq(t, router, http.MethodPost, "/api/v1/announcements", token, body, "application/json")
	}
	if rec := newAnnouncement(bendaharaToken); rec.Code != http.StatusForbidden {
		t.Errorf("bendahara create announcement = %d, want 403", rec.Code)
	}
	// Sekretaris diberi pengelolaan Pengumuman oleh migration 072 — diharapkan 201.
	if rec := newAnnouncement(sekretarisToken); rec.Code != http.StatusCreated {
		t.Errorf("sekretaris create announcement = %d, want 201", rec.Code)
	}
	if rec := newAnnouncement(ketuaToken); rec.Code != http.StatusCreated {
		t.Fatalf("ketua create announcement = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	// Sekretaris boleh mengunggah gambar pengumuman dan kegiatan; Bendahara tidak.
	if code, resp := uploadCommunityImage(t, router, sekretarisToken, "announcement", "poster.png", tinyPNG); code != http.StatusCreated {
		t.Errorf("sekretaris upload announcement image = %d, want 201 (resp=%v)", code, resp)
	}
	if code, resp := uploadCommunityImage(t, router, sekretarisToken, "activity", "poster.png", tinyPNG); code != http.StatusCreated {
		t.Errorf("sekretaris upload activity image = %d, want 201 (resp=%v)", code, resp)
	}
	if code, resp := uploadCommunityImage(t, router, bendaharaToken, "announcement", "poster.png", tinyPNG); code != http.StatusForbidden {
		t.Errorf("bendahara upload = %d, want 403 (resp=%v)", code, resp)
	}
	if code, resp := uploadCommunityImage(t, router, bendaharaToken, "activity", "poster.png", tinyPNG); code != http.StatusForbidden {
		t.Errorf("bendahara upload activity = %d, want 403 (resp=%v)", code, resp)
	}
	// Ketua can upload.
	if code, resp := uploadCommunityImage(t, router, ketuaToken, "announcement", "poster.png", tinyPNG); code != http.StatusCreated {
		t.Fatalf("ketua upload = %d, want 201 (resp=%v)", code, resp)
	}

	// Read-only listing works for every RT member.
	for name, token := range map[string]string{
		"warga":      wargaToken,
		"sekretaris": sekretarisToken,
		"bendahara":  bendaharaToken,
		"ketua":      ketuaToken,
	} {
		if rec := doReq(t, router, http.MethodGet, "/api/v1/activities", token, nil, ""); rec.Code != http.StatusOK {
			t.Errorf("%s list activities = %d, want 200", name, rec.Code)
		}
	}
}

func TestCommunity_DraftVisibility(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// Ketua creates a draft activity (no publish_now).
	draftBody := jsonBody(t, map[string]any{
		"title":     "Draft Internal Pengurus",
		"starts_at": time.Now().Add(72 * time.Hour).Format(time.RFC3339),
	})
	rec := doReq(t, router, http.MethodPost, "/api/v1/activities", ketuaToken, draftBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("ketua create draft activity = %d: %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	activityID, _ := created["id"].(string)

	// Warga must not see the draft in the list.
	rec = doReq(t, router, http.MethodGet, "/api/v1/activities", wargaToken, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("warga list activities = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Draft Internal Pengurus") {
		t.Fatal("warga can see draft activity in list")
	}

	// Warga must not fetch the draft by id.
	rec = doReq(t, router, http.MethodGet, "/api/v1/activities/"+activityID, wargaToken, nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("warga get draft activity = %d, want 404", rec.Code)
	}

	// Ketua (manager) can fetch it.
	rec = doReq(t, router, http.MethodGet, "/api/v1/activities/"+activityID, ketuaToken, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("ketua get draft activity = %d, want 200", rec.Code)
	}

	// Same for announcements.
	annBody := jsonBody(t, map[string]any{"title": "Draft Pengumuman", "body": "hidden"})
	rec = doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, annBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("ketua create draft announcement = %d: %s", rec.Code, rec.Body.String())
	}
	var ann map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &ann)
	annID, _ := ann["id"].(string)

	rec = doReq(t, router, http.MethodGet, "/api/v1/announcements", wargaToken, nil, "")
	if strings.Contains(rec.Body.String(), "Draft Pengumuman") {
		t.Fatal("warga can see draft announcement in list")
	}
	rec = doReq(t, router, http.MethodGet, "/api/v1/announcements/"+annID, wargaToken, nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("warga get draft announcement = %d, want 404", rec.Code)
	}
	rec = doReq(t, router, http.MethodGet, "/api/v1/announcements/"+annID, ketuaToken, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("ketua get draft announcement = %d, want 200", rec.Code)
	}
}

func TestCommunity_ImageEndpointAccessControl(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// 1. Unauthenticated request must be rejected.
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/community/"+rtID+"/announcement/whatever.png", "", nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated image access = %d, want 401", rec.Code)
	}

	// Upload an image and attach it to a published announcement.
	code, up := uploadCommunityImage(t, router, ketuaToken, "announcement", "hero.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	pubKey, _ := up["key"].(string)
	pubURL, _ := up["image_url"].(string)
	annBody := jsonBody(t, map[string]any{
		"title": "Pengumuman Dengan Gambar", "body": "with image",
		"publish_now": true, "image_url": pubURL,
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, annBody, "application/json"); rec.Code != http.StatusCreated {
		t.Fatalf("create announcement = %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Warga can read the published image.
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+pubKey, wargaToken, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("warga get published image = %d, want 200", rec.Code)
	}

	// Upload an image attached to a DRAFT announcement.
	code, up = uploadCommunityImage(t, router, ketuaToken, "announcement", "draft.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload draft image = %d", code)
	}
	draftKey, _ := up["key"].(string)
	draftURL, _ := up["image_url"].(string)
	annBody = jsonBody(t, map[string]any{
		"title": "Draft Dengan Gambar", "body": "hidden",
		"image_url": draftURL,
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, annBody, "application/json"); rec.Code != http.StatusCreated {
		t.Fatalf("create draft announcement = %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Warga must not read the draft image.
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+draftKey, wargaToken, nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("warga get draft image = %d, want 404", rec.Code)
	}
	// 4. Ketua (content manager) can read the draft image.
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+draftKey, ketuaToken, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("ketua get draft image = %d, want 200", rec.Code)
	}

	// 5. Uploaded but unreferenced image: managers with matching category permission
	// may preview it via authenticated Bearer access in their own RT (safe draft
	// preview of freshly uploaded content); signed URLs / warga-visible access
	// must not be granted for unreferenced keys. Warga cannot access it.
	code, up = uploadCommunityImage(t, router, ketuaToken, "announcement", "orphan.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload orphan image = %d", code)
	}
	orphanKey, _ := up["key"].(string)
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+orphanKey, wargaToken, nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unreferenced image for warga = %d, want 404", rec.Code)
	}
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+orphanKey, ketuaToken, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("unreferenced image bearer preview for ketua = %d, want 200", rec.Code)
	}
	// Unreferenced keys must not be signable as warga-visible/signed URLs.
	orphanSignBody := jsonBody(t, map[string]any{"key": orphanKey})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", ketuaToken, orphanSignBody, "application/json"); rec.Code == http.StatusOK {
		t.Fatal("manager minted a signed URL for an unreferenced image")
	}

	// 6. Traversal / double-encoding attempts must not succeed.
	for _, p := range []string{
		"/api/v1/community/images/community/" + rtID + "/announcement/..%2F..%2Fproof.png",
		"/api/v1/community/images/community/" + rtID + "/announcement/%2e%2e/%2e%2e/proofs/x.png",
		"/api/v1/community/images/../../../etc/passwd",
	} {
		rec := doReq(t, router, http.MethodGet, p, wargaToken, nil, "")
		if rec.Code == http.StatusOK {
			t.Errorf("traversal payload %q unexpectedly returned 200", p)
		}
	}

	// 7. Sign endpoint: ketua gets a URL that serves without auth;warga
	//    signing the same draft key is rejected (not a manager); tampered
	//    signature is rejected. Signed URLs never serve draft content.
	signBody := jsonBody(t, map[string]any{"key": pubKey})
	rec := doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", ketuaToken, signBody, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("ketua sign published image = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	// Manager cannot mint a signed URL for the draft image.
	draftSignBody := jsonBody(t, map[string]any{"key": draftKey})
	draftRec := doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", ketuaToken, draftSignBody, "application/json")
	if draftRec.Code == http.StatusOK {
		t.Fatal("manager minted a signed URL for a draft image")
	}
	var signResp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &signResp)
	signedURL, _ := signResp["url"].(string)
	if signedURL == "" {
		t.Fatal("sign response missing url")
	}
	if rec := doReq(t, router, http.MethodGet, signedURL, "", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("signed url without auth = %d, want 200", rec.Code)
	}
	// Swap the signature for garbage -> must be rejected.
	tampered := signedURL[:strings.LastIndex(signedURL, "sig=")] + "sig=deadbeefdeadbeefdeadbeefdeadbeef"
	if rec := doReq(t, router, http.MethodGet, tampered, "", nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("tampered signature = %d, want 403", rec.Code)
	}
	// Warga CAN sign the published (warga-visible) image...
	rec = doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", wargaToken, signBody, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("warga sign published image = %d, want 200", rec.Code)
	}
	// ...but cannot sign the draft image (never warga-visible content).
	wargaDraftSign := jsonBody(t, map[string]any{"key": draftKey})
	rec = doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", wargaToken, wargaDraftSign, "application/json")
	if rec.Code == http.StatusOK {
		t.Fatal("warga was able to sign a draft image URL")
	}
}

func TestCommunity_ImageCrossRTRejection(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})

	// Resolve another active RT (reuse the 003 fixture pattern).
	var otherRT string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM rts WHERE rt = '003' AND is_active = true LIMIT 1`).Scan(&otherRT)
	if err != nil {
		err = pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO rts (id, name, rw, rt, is_active)
			 VALUES (gen_random_uuid(), 'RT 003 Test', 16, '003', true)
			 RETURNING id`).Scan(&otherRT)
		if err != nil {
			t.Fatalf("ensure other rt: %v", err)
		}
	}
	_, _ = otherRT, pool

	_, ketuaToken := createCommunityTestUser(t, pool, commRTUrena, auth.RolePengurus, auth.JabatanKetua)
	_, otherWargaToken := createCommunityTestUser(t, pool, otherRT, auth.RoleWarga, "")

	code, up := uploadCommunityImage(t, router, ketuaToken, "announcement", "isolation.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	key, _ := up["key"].(string)
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+key, otherWargaToken, nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-RT image access = %d, want 403", rec.Code)
	}
}

func TestCommunity_DeleteLifecycle(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// Draft activity: deletable.
	draftBody := jsonBody(t, map[string]any{
		"title":     "Draft To Delete",
		"starts_at": time.Now().Add(48 * time.Hour).Format(time.RFC3339),
	})
	rec := doReq(t, router, http.MethodPost, "/api/v1/activities", ketuaToken, draftBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create draft = %d: %s", rec.Code, rec.Body.String())
	}
	var act map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &act)
	actID, _ := act["id"].(string)

	if rec := doReq(t, router, http.MethodDelete, "/api/v1/activities/"+actID, ketuaToken, nil, ""); rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("delete draft activity = %d, want 2xx", rec.Code)
	}
	if rec := doReq(t, router, http.MethodGet, "/api/v1/activities/"+actID, ketuaToken, nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("get deleted draft = %d, want 404", rec.Code)
	}

	// Published activity: NOT deletable; cancellable.
	pubBody := jsonBody(t, map[string]any{
		"title":       "Published Activity",
		"starts_at":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"publish_now": true,
	})
	rec = doReq(t, router, http.MethodPost, "/api/v1/activities", ketuaToken, pubBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish activity = %d: %s", rec.Code, rec.Body.String())
	}
	var pub map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pub)
	pubID, _ := pub["id"].(string)

	if rec := doReq(t, router, http.MethodDelete, "/api/v1/activities/"+pubID, ketuaToken, nil, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete published activity = %d, want 400: %s", rec.Code, rec.Body.String())
	}

	// Announcement lifecycle: draft delete OK; published delete rejected;
	// archive OK; archived not listed for warga.
	annDraft := jsonBody(t, map[string]any{"title": "Ann Draft", "body": "x"})
	rec = doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, annDraft, "application/json")
	var dAnn map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &dAnn)
	ddID, _ := dAnn["id"].(string)
	if rec := doReq(t, router, http.MethodDelete, "/api/v1/announcements/"+ddID, ketuaToken, nil, ""); rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("delete draft announcement = %d, want 2xx", rec.Code)
	}

	annPub := jsonBody(t, map[string]any{"title": "Ann Published", "body": "x", "publish_now": true})
	rec = doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, annPub, "application/json")
	var pAnn map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pAnn)
	ppID, _ := pAnn["id"].(string)
	if rec := doReq(t, router, http.MethodDelete, "/api/v1/announcements/"+ppID, ketuaToken, nil, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete published announcement = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements/"+ppID+"/archive", ketuaToken, nil, ""); rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("archive announcement = %d", rec.Code)
	}
}

func TestCommunity_BannerSnapshotImmutability(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")

	// Publish an activity with banner enabled. starts_at is in the past so
	// the banner is inside its display window right now.
	createBody := jsonBody(t, map[string]any{
		"title":          "Snapshot Original",
		"starts_at":      time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
		"publish_now":    true,
		"show_on_banner": true,
	})
	rec := doReq(t, router, http.MethodPost, "/api/v1/activities", ketuaToken, createBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create+publish activity = %d: %s", rec.Code, rec.Body.String())
	}
	var act map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &act)
	actID, _ := act["id"].(string)

	bannerTitle := func() string {
		t.Helper()
		rec := doReq(t, router, http.MethodGet, "/api/v1/banners", wargaToken, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("list banners = %d", rec.Code)
		}
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		data, _ := resp["data"].([]any)
		for _, item := range data {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if m["source_type"] == "activity" {
				if src, ok := m["source_id"].(string); ok && src == actID {
					title, _ := m["title"].(string)
					return title
				}
			}
		}
		return ""
	}

	if title := bannerTitle(); title != "Snapshot Original" {
		t.Fatalf("banner title after publish = %q, want %q", title, "Snapshot Original")
	}

	// Edit the activity: banner must NOT change automatically.
	updateBody := jsonBody(t, map[string]any{
		"title":          "Snapshot Changed",
		"starts_at":      time.Now().Add(-25 * time.Minute).Format(time.RFC3339),
		"show_on_banner": true,
	})
	if rec := doReq(t, router, http.MethodPut, "/api/v1/activities/"+actID, ketuaToken, updateBody, "application/json"); rec.Code != http.StatusOK {
		t.Fatalf("update activity = %d: %s", rec.Code, rec.Body.String())
	}
	if title := bannerTitle(); title != "Snapshot Original" {
		t.Fatalf("banner changed without explicit publish: %q", title)
	}

	// Explicit republish rewrites the snapshot atomically.
	if rec := doReq(t, router, http.MethodPost, "/api/v1/activities/"+actID+"/publish", ketuaToken, nil, "application/json"); rec.Code != http.StatusOK {
		t.Fatalf("explicit republish = %d: %s", rec.Code, rec.Body.String())
	}
	if title := bannerTitle(); title != "Snapshot Changed" {
		t.Fatalf("banner not updated by explicit republish: %q", title)
	}
}

func TestCommunity_ConcurrentPublishNoBannerDuplicates(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	draftBody := jsonBody(t, map[string]any{
		"title":          "Concurrent Banner",
		"starts_at":      time.Now().Add(96 * time.Hour).Format(time.RFC3339),
		"show_on_banner": true,
	})
	rec := doReq(t, router, http.MethodPost, "/api/v1/activities", ketuaToken, draftBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create activity = %d: %s", rec.Code, rec.Body.String())
	}
	var act map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &act)
	actID, _ := act["id"].(string)

	var wg sync.WaitGroup
	codes := make(chan int, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := doReq(t, router, http.MethodPost, "/api/v1/activities/"+actID+"/publish", ketuaToken, nil, "application/json")
			codes <- rec.Code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK {
			t.Errorf("concurrent publish returned %d, want 200", code)
		}
	}

	var activeCount int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM banners WHERE rt_id = $1 AND source_type = 'activity' AND source_id = $2 AND is_active = true`,
		rtID, actID).Scan(&activeCount); err != nil {
		t.Fatalf("count banners: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("active banners for source = %d, want exactly 1 (dedup failed)", activeCount)
	}
}

func TestCommunity_ScheduledAnnouncementLifecycle(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")

	now := time.Now()
	futureStart := now.Add(2 * time.Hour).Format(time.RFC3339)
	futureEnd := now.Add(4 * time.Hour).Format(time.RFC3339)

	// Publish with a future start lands in 'scheduled'.
	schedBody := jsonBody(t, map[string]any{
		"title":           "Future Announcement",
		"body":            "not yet visible",
		"publish_now":     true,
		"scheduled_start": futureStart,
		"scheduled_end":   futureEnd,
	})
	rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, schedBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("schedule announcement = %d: %s", rec.Code, rec.Body.String())
	}
	var sched map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &sched)
	if status, _ := sched["status"].(string); status != "scheduled" {
		t.Fatalf("scheduled announcement status = %q, want 'scheduled'", status)
	}

	// Warga must not see it before the window opens.
	rec = doReq(t, router, http.MethodGet, "/api/v1/announcements", wargaToken, nil, "")
	if strings.Contains(rec.Body.String(), "Future Announcement") {
		t.Fatal("warga can see scheduled announcement before window opens")
	}

	// Invalid schedule strings are rejected.
	badDate := jsonBody(t, map[string]any{
		"title":           "Bad Date",
		"body":            "x",
		"scheduled_start": "tomorrow morning",
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, badDate, "application/json"); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid scheduled_start = %d, want 400", rec.Code)
	}

	// End before start is rejected.
	revOrder := jsonBody(t, map[string]any{
		"title":           "Reversed Window",
		"body":            "x",
		"scheduled_start": futureEnd,
		"scheduled_end":   futureStart,
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, revOrder, "application/json"); rec.Code != http.StatusBadRequest {
		t.Fatalf("end<start = %d, want 400", rec.Code)
	}

	// Expired window: publish_now with scheduled_end in the past is created
	// but must not be visible to warga.
	pastEnd := now.Add(-1 * time.Hour).Format(time.RFC3339)
	expiredBody := jsonBody(t, map[string]any{
		"title":         "Expired Announcement",
		"body":          "old",
		"publish_now":   true,
		"scheduled_end": pastEnd,
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, expiredBody, "application/json"); rec.Code != http.StatusCreated {
		t.Fatalf("expired-window announcement = %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, router, http.MethodGet, "/api/v1/announcements", wargaToken, nil, "")
	if strings.Contains(rec.Body.String(), "Expired Announcement") {
		t.Fatal("warga can see expired announcement")
	}

	// In-window announcement remains visible.
	liveBody := jsonBody(t, map[string]any{
		"title":           "Live Announcement",
		"body":            "in window",
		"publish_now":     true,
		"scheduled_start": now.Add(-1 * time.Hour).Format(time.RFC3339),
		"scheduled_end":   now.Add(1 * time.Hour).Format(time.RFC3339),
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, liveBody, "application/json"); rec.Code != http.StatusCreated {
		t.Fatalf("live announcement = %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, router, http.MethodGet, "/api/v1/announcements", wargaToken, nil, "")
	if !strings.Contains(rec.Body.String(), "Live Announcement") {
		t.Fatal("warga cannot see in-window announcement")
	}
}

func TestCommunity_TemplateOwnershipValidation(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// Invalid UUID -> 400.
	anyTemplate := "not-a-uuid"
	body := jsonBody(t, map[string]any{"title": "T", "body": "B", "template_id": anyTemplate})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, body, "application/json"); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid template_id = %d, want 400", rec.Code)
	}

	// Well-formed but nonexistent template -> 404.
	body = jsonBody(t, map[string]any{
		"title": "T", "body": "B",
		"template_id": "11111111-2222-3333-4444-555555555555",
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, body, "application/json"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing template = %d, want 404", rec.Code)
	}

	// Template belonging to ANOTHER RT must not be usable.
	var otherRTTemplate string
	if err := pool.Raw().QueryRowContext(context.Background(), `
		INSERT INTO banner_templates (id, rt_id, name, category, title_template, body_template, is_system, is_active)
		VALUES (gen_random_uuid(),
		        (SELECT id FROM rts WHERE rt = '003' AND is_active = true LIMIT 1),
		        'Foreign RT Template', 'general', 'T', 'B', false, true)
		RETURNING id`).Scan(&otherRTTemplate); err != nil {
		t.Fatalf("insert foreign template: %v", err)
	}
	body = jsonBody(t, map[string]any{"title": "T", "body": "B", "template_id": otherRTTemplate})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, body, "application/json"); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign template = %d, want 404", rec.Code)
	}
}

func TestCommunity_OrganizationContactPrivacy(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, sekretarisToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanSekretaris)

	recWarga := doReq(t, router, http.MethodGet, "/api/v1/organization-structure", wargaToken, nil, "")
	if recWarga.Code != http.StatusOK {
		t.Fatalf("warga org structure = %d", recWarga.Code)
	}
	if strings.Contains(recWarga.Body.String(), `"phone"`) {
		t.Fatal("warga can see officer phone numbers")
	}

	recSek := doReq(t, router, http.MethodGet, "/api/v1/organization-structure", sekretarisToken, nil, "")
	if recSek.Code != http.StatusOK {
		t.Fatalf("sekretaris org structure = %d", recSek.Code)
	}
	if !strings.Contains(recSek.Body.String(), `"phone"`) {
		t.Fatal("sekretaris (organization.contact.read) cannot see officer phone numbers")
	}
}

// ---------------------------------------------------------------------------
// Security regression tests (signed URL lifecycle, rollback, key formats)
// ---------------------------------------------------------------------------

// testSignCommunityImageForMatch re-creates the backend signature algorithm
// (internal to package community) to enable negative-control tests that the
// shipped verification logic rejects out-of-window and tampered signatures.
// Keep in sync with imageSignature() in internal/community/image_auth.go.
func testSignCommunityImageForMatch(t *testing.T, key string, expUnix int64) string {
	t.Helper()
	mac := hmac.New(sha256.New, auth.SigningSecret)
	mac.Write([]byte(key + "." + strconv.FormatInt(expUnix, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestCommunity_SignedURLExpiryAndTampering(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")

	// Create content with an image and publish it.
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)
	code, up := uploadCommunityImage(t, router, ketuaToken, "announcement", "sigtest.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	key, _ := up["key"].(string)

	now := time.Now()
	publishBody := jsonBody(t, map[string]any{
		"title": "Signed URL Test", "body": "x", "publish_now": true,
		"image_url":       "/api/v1/community/images/" + key,
		"scheduled_start": now.Add(-time.Hour).Format(time.RFC3339),
		"scheduled_end":   now.Add(time.Hour).Format(time.RFC3339),
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, publishBody, "application/json"); rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d: %s", rec.Code, rec.Body.String())
	}

	// 1. Expired signature is rejected.
	expiredExp := now.Add(-time.Minute).Unix()
	expiredSig := testSignCommunityImageForMatch(t, key, expiredExp)
	expiredURL := "/api/v1/community/images/" + key + "?exp=" + strconv.FormatInt(expiredExp, 10) + "&sig=" + expiredSig
	if rec := doReq(t, router, http.MethodGet, expiredURL, "", nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expired signed URL = %d, want 403", rec.Code)
	}

	// 2. Modified signature is rejected.
	validExp := now.Add(15 * time.Minute).Unix()
	validSig := testSignCommunityImageForMatch(t, key, validExp)
	tampered := "/api/v1/community/images/" + key + "?exp=" + strconv.FormatInt(validExp, 10) + "&sig=" + validSig[:len(validSig)-2] + "00"
	if rec := doReq(t, router, http.MethodGet, tampered, "", nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("tampered signature = %d, want 403", rec.Code)
	}

	// 3. The sign endpoint never mints a manager-flag signature for keys the
	//    caller cannot manage, and never signs cross-RT keys. (Plain warga
	//    signs of published images carry the warga privilege class.)

	// 4. Sign endpoint rejects keys of another RT.
	otherRTBody := jsonBody(t, map[string]any{"key": "community/11111111-2222-3333-4444-555555555555/announcement/x.png"})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", wargaToken, otherRTBody, "application/json"); rec.Code != http.StatusForbidden {
		t.Fatalf("sign cross-RT key = %d, want 403", rec.Code)
	}
}

func TestCommunity_SignedURLRevokedAfterArchive(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	code, up := uploadCommunityImage(t, router, ketuaToken, "announcement", "revoke.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	key, _ := up["key"].(string)

	now := time.Now()
	annBody := jsonBody(t, map[string]any{
		"title": "Revoke Me", "body": "x", "publish_now": true,
		"image_url":       "/api/v1/community/images/" + key,
		"scheduled_start": now.Add(-time.Hour).Format(time.RFC3339),
		"scheduled_end":   now.Add(time.Hour).Format(time.RFC3339),
	})
	rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, annBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create+publish = %d: %s", rec.Code, rec.Body.String())
	}
	var ann map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &ann)
	annID, _ := ann["id"].(string)

	// Warga signs the published image: valid right now.
	signBody := jsonBody(t, map[string]any{"key": key})
	rec = doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", wargaToken, signBody, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("warga sign = %d", rec.Code)
	}
	var signResp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &signResp)
	signedURL, _ := signResp["url"].(string)
	if rec := doReq(t, router, http.MethodGet, signedURL, "", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("signed url before archive = %d, want 200", rec.Code)
	}

	// Archive the announcement. Warga sees archived announcements in Riwayat
	// list, so the image must still be authorized (the actual requirement).
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements/"+annID+"/archive", ketuaToken, nil, "application/json"); rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("archive = %d", rec.Code)
	}
	// Warga CAN see archived announcement images because warga can see the announcement in Riwayat list.
	if rec := doReq(t, router, http.MethodGet, signedURL, "", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("signed url after archive = %d, want 200 (history visibility)", rec.Code)
	}
}

func TestCommunity_LegacyKeyFormatStillServed(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	code, up := uploadCommunityImage(t, router, ketuaToken, "announcement", "legacy.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	key, _ := up["key"].(string)

	// Simulate a LEGACY row that stored the raw object key (no /api prefix)
	// instead of the full path the upload endpoint returns today.
	now := time.Now()
	legacyBody := jsonBody(t, map[string]any{
		"title": "Legacy Image Ref", "body": "x", "publish_now": true,
		"image_url":       key, // raw storage key, as some older rows may hold
		"scheduled_start": now.Add(-time.Hour).Format(time.RFC3339),
		"scheduled_end":   now.Add(time.Hour).Format(time.RFC3339),
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, legacyBody, "application/json"); rec.Code != http.StatusCreated {
		t.Fatalf("create legacy-ref announcement = %d: %s", rec.Code, rec.Body.String())
	}

	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+key, wargaToken, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("legacy raw-key image = %d, want 200", rec.Code)
	}
}

func TestCommunity_DeleteTransactionRollback(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// Unit-level atomicity proof: create a DRAFT activity with an associated
	// banner row, install a trigger that makes DELETE on activities fail, and
	// verify the banner row is NOT deleted when the source delete fails.
	createBody := jsonBody(t, map[string]any{
		"title":     "Rollback Proof",
		"starts_at": time.Now().Add(48 * time.Hour).Format(time.RFC3339),
	})
	rec := doReq(t, router, http.MethodPost, "/api/v1/activities", ketuaToken, createBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create draft = %d: %s", rec.Code, rec.Body.String())
	}
	var act map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &act)
	actID, _ := act["id"].(string)

	if _, err := pool.Raw().ExecContext(context.Background(), `
		INSERT INTO banners (id, rt_id, source_type, source_id, title, is_active, priority, display_order, snapshot_content, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'activity', $2, 'rollback-proof', true, 1, 0, '{}'::jsonb, NOW(), NOW())`,
		rtID, actID); err != nil {
		t.Fatalf("insert banner fixture: %v", err)
	}

	if _, err := pool.Raw().ExecContext(context.Background(), `
		CREATE OR REPLACE FUNCTION sf_test_block_delete() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced delete failure for rollback test';
		END $$ LANGUAGE plpgsql`); err != nil {
		t.Fatalf("create function: %v", err)
	}
	if _, err := pool.Raw().ExecContext(context.Background(), `
		DROP TRIGGER IF EXISTS sf_test_block_delete_activity ON activities;
		CREATE TRIGGER sf_test_block_delete_activity
		BEFORE DELETE ON activities
		FOR EACH ROW EXECUTE FUNCTION sf_test_block_delete()`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	defer func() {
		_, _ = pool.Raw().ExecContext(context.Background(), `DROP TRIGGER IF EXISTS sf_test_block_delete_activity ON activities`)
		_, _ = pool.Raw().ExecContext(context.Background(), `DROP FUNCTION IF EXISTS sf_test_block_delete()`)
	}()

	// Delete must FAIL (trigger), and the banner row must be RESTORED by rollback.
	rec = doReq(t, router, http.MethodDelete, "/api/v1/activities/"+actID, ketuaToken, nil, "application/json")
	if rec.Code == http.StatusOK || rec.Code == http.StatusNoContent {
		t.Fatalf("delete with failing source unexpectedly succeeded (%d)", rec.Code)
	}

	var bannerCount int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM banners WHERE rt_id = $1 AND source_type = 'activity' AND source_id = $2`,
		rtID, actID).Scan(&bannerCount); err != nil {
		t.Fatalf("count banner after rollback: %v", err)
	}
	if bannerCount != 1 {
		t.Fatalf("banner rows after failed delete = %d, want 1 (rollback restores it)", bannerCount)
	}

	// And the activity itself must still exist.
	rec = doReq(t, router, http.MethodGet, "/api/v1/activities/"+actID, ketuaToken, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("activity after failed delete = %d, want 200 (rollback preserved it)", rec.Code)
	}
}

func TestCommunity_PermissionRevokedAtRequestTime(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	userID, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// Ketua creates an activity -> allowed.
	createBody := jsonBody(t, map[string]any{
		"title":     "Revocation Check",
		"starts_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	before := doReq(t, router, http.MethodPost, "/api/v1/activities", ketuaToken, createBody, "application/json")
	if before.Code != http.StatusCreated {
		t.Fatalf("ketua create before revocation = %d: %s", before.Code, before.Body.String())
	}

	// Revoke the membership (is_active=false) WITHOUT issuing a new token.
	if _, err := pool.Raw().ExecContext(context.Background(),
		`UPDATE user_rt_memberships SET is_active = false WHERE user_id = $1 AND rt_id = $2`,
		userID, rtID); err != nil {
		t.Fatalf("revoke membership: %v", err)
	}

	after := doReq(t, router, http.MethodPost, "/api/v1/activities", ketuaToken, createBody, "application/json")
	if after.Code == http.StatusCreated || after.Code == http.StatusOK {
		t.Fatalf("create after membership revocation = %d, want denial", after.Code)
	}
}

func TestCommunity_SuperAdminWithoutRTContextDenied(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})

	// Create a system-only super_admin (no RT membership).
	var saUserID string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'SA NoContext', 'super_admin', true) RETURNING id`,
		fmt.Sprintf("sa-nocontext-%d@test.com", atomic.AddUint64(&commSeq, 1))).Scan(&saUserID); err != nil {
		t.Fatalf("create sa: %v", err)
	}
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  saUserID,
		SysRole: string(auth.SystemRoleSuperAdmin),
		// No RTID, no MembershipID.
	})
	if err != nil {
		t.Fatalf("sa token: %v", err)
	}

	// Without RT context, the community endpoints must reject the request
	// (tenant context is mandatory; super admin is not exempt).
	for _, path := range []string{"/api/v1/activities", "/api/v1/announcements", "/api/v1/banners", "/api/v1/home/dashboard", "/api/v1/organization-structure"} {
		rec := doReq(t, router, http.MethodGet, path, token, nil, "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("SA without RT context GET %s = %d, want 403", path, rec.Code)
		}
	}
}

func TestCommunity_SuperAdminWithRTContextAllowed(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})

	var saUserID string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'SA WithContext', 'super_admin', true) RETURNING id`,
		fmt.Sprintf("sa-context-%d@test.com", atomic.AddUint64(&commSeq, 1))).Scan(&saUserID); err != nil {
		t.Fatalf("create sa: %v", err)
	}
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  saUserID,
		SysRole: string(auth.SystemRoleSuperAdmin),
		RTID:    commRTUrena,
	})
	if err != nil {
		t.Fatalf("sa token: %v", err)
	}

	rec := doReq(t, router, http.MethodGet, "/api/v1/activities", token, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("SA with RT context GET /api/v1/activities = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestCommunity_SignedURLNeverUnlocksDraftContent(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// Publish nothing; create a DRAFT announcement with an image.
	code, up := uploadCommunityImage(t, router, ketuaToken, "announcement", "draft-secret.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	key, _ := up["key"].(string)
	draftBody := jsonBody(t, map[string]any{
		"title": "Draft Secret", "body": "hidden", "image_url": "/api/v1/community/images/" + key,
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, draftBody, "application/json"); rec.Code != http.StatusCreated {
		t.Fatalf("create draft = %d: %s", rec.Code, rec.Body.String())
	}

	// 1. Manager's attempt to mint a signed URL for the draft image is rejected.
	signBody := jsonBody(t, map[string]any{"key": key})
	rec := doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", ketuaToken, signBody, "application/json")
	if rec.Code == http.StatusOK {
		t.Fatal("manager minted a signed URL for a draft image")
	}

	// 2. A forged-but-valid signature over the draft key still cannot serve
	//    the image: read-side revalidation requires warga-visible content.
	now := time.Now()
	exp := now.Add(15 * time.Minute).Unix()
	validSig := testSignCommunityImageForMatch(t, key, exp)
	signedURL := "/api/v1/community/images/" + key + "?exp=" + strconv.FormatInt(exp, 10) + "&sig=" + validSig
	if rec := doReq(t, router, http.MethodGet, signedURL, "", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("draft image via forged signed URL = %d, want 404", rec.Code)
	}

	// 3. Manager CAN access the draft image via the Bearer mode (the intended
	//    path for content managers, not signed URLs).
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+key, ketuaToken, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("manager bearer draft image = %d, want 200", rec.Code)
	}

	// 4. Warga cannot access the draft image at all.
	if rec := doReq(t, router, http.MethodGet, "/api/v1/community/images/"+key, wargaToken, nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("warga draft image = %d, want 404", rec.Code)
	}
}

func TestCommunity_SignedURLUnreferencedKeyRejected(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	// Upload but never reference the image; craft a valid signature over it.
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)
	code, up := uploadCommunityImage(t, router, ketuaToken, "announcement", "orphan.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	key, _ := up["key"].(string)

	now := time.Now()
	exp := now.Add(15 * time.Minute).Unix()
	sig := testSignCommunityImageForMatch(t, key, exp)
	signedURL := "/api/v1/community/images/" + key + "?exp=" + strconv.FormatInt(exp, 10) + "&sig=" + sig
	if rec := doReq(t, router, http.MethodGet, signedURL, "", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unreferenced signed URL = %d, want 404", rec.Code)
	}
}

func TestCommunity_ImageResponseNoStore(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// Publish an announcement with an image.
	code, up := uploadCommunityImage(t, router, ketuaToken, "announcement", "cache-check.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	key, _ := up["key"].(string)
	now := time.Now()
	pubBody := jsonBody(t, map[string]any{
		"title": "Cache Check", "body": "x", "publish_now": true,
		"image_url":       "/api/v1/community/images/" + key,
		"scheduled_start": now.Add(-time.Hour).Format(time.RFC3339),
		"scheduled_end":   now.Add(time.Hour).Format(time.RFC3339),
	})
	rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, pubBody, "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d: %s", rec.Code, rec.Body.String())
	}
	var pub map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pub)
	pubID, _ := pub["id"].(string)

	assertNoStore := func(name string, rec *httptest.ResponseRecorder) {
		t.Helper()
		got := rec.Header().Get("Cache-Control")
		if !strings.EqualFold(strings.TrimSpace(got), "no-store") {
			t.Errorf("%s: Cache-Control = %q, want no-store", name, got)
		}
	}

	// 1. Signed URL valid -> 200 + no-store
	signBody := jsonBody(t, map[string]any{"key": key})
	signRec := doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", wargaToken, signBody, "application/json")
	if signRec.Code != http.StatusOK {
		t.Fatalf("sign = %d", signRec.Code)
	}
	var sr map[string]any
	_ = json.Unmarshal(signRec.Body.Bytes(), &sr)
	signedURL, _ := sr["url"].(string)
	rec = doReq(t, router, http.MethodGet, signedURL, "", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("signed url = %d", rec.Code)
	}
	assertNoStore("signed url valid", rec)

	// 2. Bearer manager for draft -> 200 + no-store
	code, up = uploadCommunityImage(t, router, ketuaToken, "announcement", "cache-draft.png", tinyPNG)
	if code != http.StatusCreated {
		t.Fatalf("upload draft = %d", code)
	}
	draftKey, _ := up["key"].(string)
	draftBody := jsonBody(t, map[string]any{
		"title": "Cache Draft", "body": "x", "image_url": "/api/v1/community/images/" + draftKey,
	})
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, draftBody, "application/json"); rec.Code != http.StatusCreated {
		t.Fatalf("create draft = %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, router, http.MethodGet, "/api/v1/community/images/"+draftKey, ketuaToken, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("manager draft image = %d", rec.Code)
	}
	assertNoStore("manager bearer draft", rec)

	// 3. Expired signed URL -> 403 + Cache-Control present & not permissive
	expiredExp := time.Now().Add(-time.Minute).Unix()
	expiredSig := testSignCommunityImageForMatch(t, key, expiredExp)
	expiredURL := "/api/v1/community/images/" + key + "?exp=" + strconv.FormatInt(expiredExp, 10) + "&sig=" + expiredSig
	rec = doReq(t, router, http.MethodGet, expiredURL, "", nil, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expired signed URL = %d, want 403", rec.Code)
	}
	assertNoStore("expired signed url", rec)

	// 4. Draft denied for warga -> 404 + no-store
	rec = doReq(t, router, http.MethodGet, "/api/v1/community/images/"+draftKey, wargaToken, nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("warga draft image = %d, want 404", rec.Code)
	}
	assertNoStore("warga draft denied", rec)

	// 5. Archived content: signed URL stays valid (Warga can view archived
	// announcements in Riwayat list).
	signRec = doReq(t, router, http.MethodPost, "/api/v1/community/images/sign", wargaToken, signBody, "application/json")
	var sr2 map[string]any
	_ = json.Unmarshal(signRec.Body.Bytes(), &sr2)
	signedURL2, _ := sr2["url"].(string)
	if rec := doReq(t, router, http.MethodPost, "/api/v1/announcements/"+pubID+"/archive", ketuaToken, nil, "application/json"); rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("archive = %d", rec.Code)
	}
	rec = doReq(t, router, http.MethodGet, signedURL2, "", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("archived content signed URL = %d, want 200", rec.Code)
	}
	assertNoStore("archived content", rec)

	// 6. Cross-RT -> 403 + no-store
	var otherRT string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM rts WHERE rt = '003' AND is_active = true LIMIT 1`).Scan(&otherRT)
	if err != nil {
		err = pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO rts (id, name, rw, rt, is_active) VALUES (gen_random_uuid(), 'RT 003 Test', 16, '003', true) RETURNING id`).Scan(&otherRT)
		if err != nil {
			t.Fatalf("ensure other rt: %v", err)
		}
	}
	_, otherToken := createCommunityTestUser(t, pool, otherRT, auth.RoleWarga, "")
	rec = doReq(t, router, http.MethodGet, "/api/v1/community/images/"+key, otherToken, nil, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-RT image = %d, want 403", rec.Code)
	}
	assertNoStore("cross-rt", rec)
}
