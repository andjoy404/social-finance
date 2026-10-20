package rt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"social-finance/internal/auth"
)

func TestRWLifecycleAndRTRelation(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	setupEnv()
	token := superAdminToken(t)

	pool.Raw().Exec("DELETE FROM rts WHERE name LIKE 'RWTest RT%'")
	pool.Raw().Exec("DELETE FROM rws WHERE rw_number IN (901, 902)")

	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rts WHERE name LIKE 'RWTest RT%'")
		pool.Raw().Exec("DELETE FROM rws WHERE rw_number IN (901, 902)")
		pool.Close()
	})

	doReq := func(method, path, body, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 1. Non-superadmin cannot create RW
	authRTID := insertRT(t, pool, "RWTest Auth RT", 89, "RA01")
	wargaUserID, wargaMID := uuid.NewString(), uuid.NewString()
	insertAdminRTMembership(t, authRTID, wargaUserID, wargaMID, string(auth.RoleWarga))
	wargaClaims := auth.TokenClaims{UserID: wargaUserID, MID: wargaMID, RTID: authRTID, Role: string(auth.RoleWarga)}
	wargaClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))
	nonAdminTok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, wargaClaims).SignedString([]byte(testSigningKey))
	if rec := doReq("POST", "/api/v1/rws", `{"rw_number":901,"name":"RW 901"}`, nonAdminTok); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-superadmin RW create, got %d", rec.Code)
	}

	// 2. Create RW 901 and RW 902
	rec901 := doReq("POST", "/api/v1/rws", `{"rw_number":901,"name":"RW 901 Melati","head_name":"Ketua RW 901","address":"Jl. Melati"}`, token)
	if rec901.Code != http.StatusCreated {
		t.Fatalf("create RW 901 failed: %d %s", rec901.Code, rec901.Body.String())
	}
	var rw901 RW
	if err := json.Unmarshal(rec901.Body.Bytes(), &rw901); err != nil {
		t.Fatalf("unmarshal RW 901: %v", err)
	}
	if rw901.ID == "" || rw901.RWNumber != 901 || rw901.Name != "RW 901 Melati" {
		t.Fatalf("unexpected RW 901 payload: %+v", rw901)
	}

	// Duplicate active RW number should return 409 Conflict
	if recDup := doReq("POST", "/api/v1/rws", `{"rw_number":901,"name":"RW 901 Duplicate"}`, token); recDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate RW number, got %d: %s", recDup.Code, recDup.Body.String())
	}

	rec902 := doReq("POST", "/api/v1/rws", `{"rw_number":902,"name":"RW 902 Mawar"}`, token)
	if rec902.Code != http.StatusCreated {
		t.Fatalf("create RW 902 failed: %d %s", rec902.Code, rec902.Body.String())
	}
	var rw902 RW
	_ = json.Unmarshal(rec902.Body.Bytes(), &rw902)

	// 3. Create RT linked via rw_id to RW 901
	createRTBody := fmt.Sprintf(`{"name":"RWTest RT 001","rw_id":"%s","rt":"001","head_name":"Ketua RT 001"}`, rw901.ID)
	recRT := doReq("POST", "/api/v1/rts", createRTBody, token)
	if recRT.Code != http.StatusCreated {
		t.Fatalf("create RT with rw_id failed: %d %s", recRT.Code, recRT.Body.String())
	}
	var createdRT RT
	if err := json.Unmarshal(recRT.Body.Bytes(), &createdRT); err != nil {
		t.Fatalf("unmarshal RT: %v", err)
	}
	if createdRT.RW != 901 || createdRT.RWID == nil || *createdRT.RWID != rw901.ID {
		t.Fatalf("expected RT linked to RW 901 (%s), got RW=%d RWID=%v", rw901.ID, createdRT.RW, createdRT.RWID)
	}

	// 4. Filter RTs by rw_id
	recList := doReq("GET", "/api/v1/rts?rw_id="+rw901.ID, "", token)
	if recList.Code != http.StatusOK {
		t.Fatalf("list RTs by rw_id failed: %d %s", recList.Code, recList.Body.String())
	}
	var listResp struct {
		Data []RT `json:"data"`
	}
	_ = json.Unmarshal(recList.Body.Bytes(), &listResp)
	if len(listResp.Data) != 1 || listResp.Data[0].ID != createdRT.ID {
		t.Fatalf("expected 1 RT under RW 901, got %+v", listResp.Data)
	}

	// 5. Deactivating RW 901 while RT is active must fail with 409 Conflict
	if recDeact := doReq("PATCH", "/api/v1/rws/"+rw901.ID+"/deactivate", "", token); recDeact.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict deactivating RW with active RT, got %d: %s", recDeact.Code, recDeact.Body.String())
	}

	// 6. Move RT to RW 902 via PATCH /api/v1/rts/{id}
	updateRTBody := fmt.Sprintf(`{"rw_id":"%s"}`, rw902.ID)
	recUpdRT := doReq("PATCH", "/api/v1/rts/"+createdRT.ID, updateRTBody, token)
	if recUpdRT.Code != http.StatusOK {
		t.Fatalf("update RT rw_id failed: %d %s", recUpdRT.Code, recUpdRT.Body.String())
	}
	var updatedRT RT
	_ = json.Unmarshal(recUpdRT.Body.Bytes(), &updatedRT)
	if updatedRT.RW != 902 || updatedRT.RWID == nil || *updatedRT.RWID != rw902.ID {
		t.Fatalf("expected RT moved to RW 902, got RW=%d RWID=%v", updatedRT.RW, updatedRT.RWID)
	}

	// 7. Now RW 901 has 0 active RTs and can be deactivated
	if recDeact := doReq("PATCH", "/api/v1/rws/"+rw901.ID+"/deactivate", "", token); recDeact.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content deactivating RW 901, got %d: %s", recDeact.Code, recDeact.Body.String())
	}
}
